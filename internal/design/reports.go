package design

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/storage/objectstore"
	"github.com/google/uuid"
)

var ErrAssistant = errors.New("assistant error")

type GeneratedReports struct {
	Standard Report `json:"standard"`
	Enhanced Report `json:"enhanced"`
}

type ReportService struct {
	assistant ai.Assistant
	builder   *ReportBuilder
	store     objectstore.ObjectStore
	repo      ReportRepository
}

func NewReportService(assistant ai.Assistant, builder *ReportBuilder, store objectstore.ObjectStore, repo ReportRepository) *ReportService {
	return &ReportService{
		assistant: assistant,
		builder:   builder,
		store:     store,
		repo:      repo,
	}
}

func (r *ReportService) Generate(ctx context.Context, kind string, id string, status Status) (GeneratedReports, error) {
	standardSections := renderStandard(status)
	enhancedSections, err := r.renderEnhanced(ctx, status)
	if err != nil {
		return GeneratedReports{}, fmt.Errorf("%w: %v", ErrAssistant, err)
	}

	standard, err := r.produce(ctx, kind, id, status, ReportVersionStandard, standardSections)
	if err != nil {
		return GeneratedReports{}, err
	}

	enhanced, err := r.produce(ctx, kind, id, status, ReportVersionEnhanced, enhancedSections)
	if err != nil {
		return GeneratedReports{}, err
	}

	return GeneratedReports{Standard: standard, Enhanced: enhanced}, nil
}

func (r *ReportService) produce(ctx context.Context, kind string, id string, status Status, version string, sections []RenderSection) (Report, error) {
	data, err := r.builder.Build("Gamification Design Report", version, status.Spark, sections)
	if err != nil {
		return Report{}, err
	}

	reportID := uuid.NewString()
	key := "design/" + kind + "/" + id + "/" + reportID + ".pdf"
	url, err := r.store.Save(ctx, key, data)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		ID:        reportID,
		Version:   version,
		URL:       url,
		CreatedAt: time.Now().UTC(),
	}

	if kind == "projects" {
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

func (r *ReportService) List(ctx context.Context, projectID string) ([]Report, error) {
	return r.repo.ListByProjectID(ctx, projectID)
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

func renderStandard(status Status) []RenderSection {
	var sections []RenderSection
	for number := 1; number <= SectionCount; number++ {
		sections = append(sections, RenderSection{
			Number: number,
			Name:   SectionName(number),
			Lines:  SectionLines(status.Section(number).Content),
		})
	}
	return sections
}

func (r *ReportService) renderEnhanced(ctx context.Context, status Status) ([]RenderSection, error) {
	var inputs []ai.SectionText
	for number := 1; number <= SectionCount; number++ {
		text := SectionPlainText(status.Section(number).Content)
		if text == "" {
			continue
		}
		inputs = append(inputs, ai.SectionText{
			Number: number,
			Name:   SectionName(number),
			Text:   text,
		})
	}

	enhanced, err := r.assistant.Enhance(ctx, inputs)
	if err != nil {
		return nil, err
	}

	prose := make(map[int]string, len(enhanced))
	for _, section := range enhanced {
		prose[section.Number] = section.Text
	}

	var sections []RenderSection
	for number := 1; number <= SectionCount; number++ {
		var lines []string
		if text, ok := prose[number]; ok && text != "" {
			lines = []string{text}
		}
		sections = append(sections, RenderSection{
			Number: number,
			Name:   SectionName(number),
			Lines:  lines,
		})
	}
	return sections, nil
}
