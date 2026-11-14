package design

import (
	"context"
	"github.com/gamidoc/backend/internal/docname"
	"log/slog"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/storage/objectstore"
	"github.com/google/uuid"
)

// RenderImage is an uploaded image printed in a document.
type RenderImage struct {
	Label string
	Data  []byte
	Type  string // "PNG" or "JPG"
	Name  string // storage key
}

// ReportService produces the final documents of a design (see document.go) and keeps
// track of them.
type ReportService struct {
	assistant ai.Assistant
	renderer  HTMLRenderer // nil: plain PDF only
	store     objectstore.ObjectStore
	repo      ReportRepository
	logger    *slog.Logger
}

func NewReportService(assistant ai.Assistant, renderer HTMLRenderer, store objectstore.ObjectStore, repo ReportRepository) *ReportService {
	return &ReportService{
		assistant: assistant,
		renderer:  renderer,
		store:     store,
		repo:      repo,
		logger:    slog.Default(),
	}
}

// Generate writes the document in the given format (AI when available, otherwise the
// entered data only), prints it to PDF and stores it. kind is "sessions" or "projects".
func (r *ReportService) Generate(ctx context.Context, kind string, id string, status Status, format string) (Report, error) {
	if !ValidReportFormat(format) {
		return Report{}, ErrInvalidReportVersion
	}
	logger := r.logger.With("design", kind+":"+id, "format", format)

	var document any
	withAI := false
	if ai.SupportsJSON(r.assistant) {
		var thin *ReportDoc
		// A thin report (blocks without explanations) is asked again once.
		for attempt := 1; attempt <= 2 && document == nil; attempt++ {
			generated, err := GenerateDocument(ctx, r.assistant, format, status)
			if err != nil {
				logger.Error("document generation failed", "attempt", attempt, "error", err)
				break
			}
			if report, ok := generated.(*ReportDoc); ok {
				if completeness := reportCompleteness(report); completeness < 0.6 {
					logger.Warn("thin design report", "attempt", attempt, "completeness", completeness, "sample", firstBlockSample(report))
					thin = report
					continue
				}
			}
			document, withAI = generated, true
		}
		if document == nil && thin != nil {
			document, withAI = completeThinReport(thin, status), true
		}
	}
	if document == nil {
		document = fallbackDocument(format, status)
	}
	title := DesignTitle(status, "")
	setDocumentTitle(document, title)

	documentID := documentIDFor(id)
	images := map[int][]RenderImage{}
	for number := 1; number <= SectionCount; number++ {
		if list, _ := r.sectionImages(ctx, status.Section(number).Content); len(list) > 0 {
			images[number] = list
		}
	}

	data, err := r.render(ctx, document, format, status, documentID, images, withAI, logger)
	if err != nil {
		return Report{}, err
	}
	return r.saveDocument(ctx, kind, id, format, title, data)
}

func (r *ReportService) render(ctx context.Context, document any, format string, status Status, documentID string, images map[int][]RenderImage, withAI bool, logger *slog.Logger) ([]byte, error) {
	if r.renderer != nil {
		html, err := RenderDocumentHTML(document, format, status, documentID, images, withAI)
		if err != nil {
			return nil, err
		}
		data, err := r.renderer.RenderHTML(ctx, html)
		if err == nil {
			return data, nil
		}
		logger.Error("html to pdf failed, printing a plain pdf", "error", err)
	}
	return RenderDocumentPlainPDF(document, format, status, documentID)
}

func (r *ReportService) saveDocument(ctx context.Context, kind string, id string, format string, title string, data []byte) (Report, error) {
	reportID := uuid.NewString()
	key := "design/" + kind + "/" + id + "/" + reportID + "/" + DocumentFileName(title, format)
	url, err := r.store.Save(ctx, key, data)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		ID:        reportID,
		Version:   format,
		URL:       url,
		CreatedAt: time.Now().UTC(),
	}

	if kind == "projects" && r.repo != nil {
		report.ProjectID = id
		created, err := r.repo.Create(ctx, report)
		if err != nil {
			_ = r.store.Delete(ctx, key)
			return Report{}, err
		}
		return created, nil
	}

	return report, nil
}

// documentIDFor gives a short, stable document id for a design ("DES-1A2B3C").
func documentIDFor(designID string) string {
	compact := strings.ToUpper(strings.ReplaceAll(designID, "-", ""))
	if len(compact) > 6 {
		compact = compact[:6]
	}
	return "DES-" + compact
}

// MarkMissing flags the reports whose file is no longer in the object store. If the
// store cannot be checked, the report is left as available.
func (r *ReportService) MarkMissing(ctx context.Context, reports []Report) []Report {
	marked := make([]Report, len(reports))
	for i, report := range reports {
		marked[i] = report
		key, ok := r.store.KeyFromURL(report.URL)
		if !ok {
			continue
		}
		if exists, err := r.store.Exists(ctx, key); err == nil && !exists {
			marked[i].Missing = true
		}
	}
	return marked
}

func (r *ReportService) List(ctx context.Context, projectID string) ([]Report, error) {
	return r.repo.ListByProjectID(ctx, projectID)
}

// Rename moves the PDF of a report to a new file name (the download name) and returns the
// updated report.
func (r *ReportService) Rename(ctx context.Context, projectID string, reportID string, name string) (Report, error) {
	fileName, err := docname.Custom(name)
	if err != nil {
		return Report{}, err
	}
	if r.repo == nil {
		return Report{}, ErrReportNotFound
	}
	reports, err := r.repo.ListByProjectID(ctx, projectID)
	if err != nil {
		return Report{}, err
	}
	var current *Report
	for i := range reports {
		if reports[i].ID == reportID {
			current = &reports[i]
		}
	}
	if current == nil {
		return Report{}, ErrReportNotFound
	}
	oldKey, ok := r.store.KeyFromURL(current.URL)
	if !ok {
		return Report{}, ErrReportFileMissing
	}
	newKey := "design/projects/" + projectID + "/" + reportID + "/" + fileName
	if newKey == oldKey {
		return r.withFileName(*current), nil
	}
	data, err := r.store.Read(ctx, oldKey)
	if err != nil {
		return Report{}, ErrReportFileMissing
	}
	url, err := r.store.Save(ctx, newKey, data)
	if err != nil {
		return Report{}, err
	}
	updated, err := r.repo.UpdateURL(ctx, projectID, reportID, url)
	if err != nil {
		_ = r.store.Delete(ctx, newKey)
		return Report{}, err
	}
	if err := r.store.Delete(ctx, oldKey); err != nil {
		r.logger.Warn("renamed report: old file not removed", "report", reportID, "error", err)
	}
	return r.withFileName(updated), nil
}

// WithFileNames sets the file name of each report (see Report.FileName).
func (r *ReportService) WithFileNames(reports []Report) []Report {
	named := make([]Report, len(reports))
	for i, report := range reports {
		named[i] = r.withFileName(report)
	}
	return named
}

func (r *ReportService) withFileName(report Report) Report {
	if key, ok := r.store.KeyFromURL(report.URL); ok {
		report.FileName = docname.FromKey(key, "")
	}
	return report
}

// Delete removes a report of a project and its PDF file.
func (r *ReportService) Delete(ctx context.Context, projectID string, reportID string) error {
	if r.repo == nil {
		return ErrReportNotFound
	}
	deleted, err := r.repo.Delete(ctx, projectID, reportID)
	if err != nil {
		return err
	}
	if key, ok := r.store.KeyFromURL(deleted.URL); ok {
		if err := r.store.Delete(ctx, key); err != nil {
			r.logger.Warn("deleted report: file not removed", "report", reportID, "error", err)
		}
	}
	return nil
}

func (r *ReportService) Persist(ctx context.Context, projectID string, reports []Report) error {
	for _, report := range reports {
		report.ID = uuid.NewString()
		report.ProjectID = projectID
		if report.CreatedAt.IsZero() {
			report.CreatedAt = time.Now().UTC()
		}
		if _, err := r.repo.Create(ctx, report); err != nil {
			return err
		}
	}
	return nil
}
