package pdf

import (
	"context"
	"fmt"
	"net/mail"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/docname"
	"github.com/gamidoc/backend/internal/mailer"
	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/recommendation"
	"github.com/gamidoc/backend/internal/session"
	"github.com/gamidoc/backend/internal/storage/objectstore"
	"github.com/gamidoc/backend/internal/wizard"
)

var ErrInvalidNotifyEmail = fmt.Errorf("invalid notify email")
var ErrPDFNotFound = fmt.Errorf("pdf not found")

type ProjectRepository interface {
	FindByID(ctx context.Context, id string) (project.Project, error)
	UpdatePDFURL(ctx context.Context, projectID string, pdfURL string) (project.Project, error)
}

type SessionRepository interface {
	FindByID(ctx context.Context, id string) (session.Session, error)
	UpdatePDFURL(ctx context.Context, sessionID string, pdfURL string) (session.Session, error)
}

type RecommendationService interface {
	Recommend(status interface{}, forStep int) (recommendation.Result, error)
}

type ProjectRecommendationService interface {
	Recommend(ctx context.Context, userID string, projectID string, forStep int) (recommendation.Result, error)
}

type SessionRecommendationService interface {
	Recommend(ctx context.Context, sessionID string, forStep int) (recommendation.Result, error)
}

type Service struct {
	builder                *Builder
	generator              Generator
	htmlRenderer           HTMLRenderer
	store                  objectstore.ObjectStore
	mailer                 mailer.Mailer
	mailerFromEmail        string
	mailerFromName         string
	projects               ProjectRepository
	sessions               SessionRepository
	projectRecommendations ProjectRecommendationService
	sessionRecommendations SessionRecommendationService
}

func NewService(
	builder *Builder,
	generator Generator,
	store objectstore.ObjectStore,
	m mailer.Mailer,
	mailerFromEmail string,
	mailerFromName string,
	projects ProjectRepository,
	sessions SessionRepository,
	projectRecommendations ProjectRecommendationService,
	sessionRecommendations SessionRecommendationService,
) *Service {
	return &Service{
		builder:                builder,
		generator:              generator,
		store:                  store,
		mailer:                 m,
		mailerFromEmail:        mailerFromEmail,
		mailerFromName:         mailerFromName,
		projects:               projects,
		sessions:               sessions,
		projectRecommendations: projectRecommendations,
		sessionRecommendations: sessionRecommendations,
	}
}

func (s *Service) WithHTMLRenderer(renderer HTMLRenderer) *Service {
	s.htmlRenderer = renderer
	return s
}

func (s *Service) GenerateProjectPDF(ctx context.Context, userID string, projectID string, notifyEmail string) (Generated, error) {
	return s.GenerateProjectPDFWithOptions(ctx, userID, projectID, GenerateOptions{
		NotifyEmail: notifyEmail,
	})
}

func (s *Service) GenerateProjectPDFWithOptions(ctx context.Context, userID string, projectID string, options GenerateOptions) (Generated, error) {
	options.NotifyEmail = strings.TrimSpace(options.NotifyEmail)
	if options.NotifyEmail != "" {
		if _, err := mail.ParseAddress(options.NotifyEmail); err != nil {
			return Generated{}, ErrInvalidNotifyEmail
		}
	}

	item, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return Generated{}, err
	}

	if item.UserID != userID {
		return Generated{}, project.ErrForbiddenProject
	}

	if err := wizard.ValidateComplete(item.Wizard); err != nil {
		return Generated{}, err
	}

	methodResult, err := s.projectRecommendations.Recommend(ctx, userID, projectID, 2)
	if err != nil {
		return Generated{}, err
	}

	instrumentResult, err := s.projectRecommendations.Recommend(ctx, userID, projectID, 3)
	if err != nil {
		return Generated{}, err
	}

	data, err := s.builder.BuildFromProject(item, methodResult.Recommendations, instrumentResult.Recommendations)
	if err != nil {
		return Generated{}, err
	}

	bytes, err := s.renderPDF(ctx, data, options)
	if err != nil {
		return Generated{}, err
	}

	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = docname.DefaultTitle
	}
	key := filepath.ToSlash(fmt.Sprintf("projects/%s/%d/%s", projectID, time.Now().UnixNano(), docname.FileName(name, docname.EvaluationPlan)))
	url, err := s.store.Save(ctx, key, bytes)
	if err != nil {
		return Generated{}, err
	}

	if _, err := s.projects.UpdatePDFURL(ctx, projectID, url); err != nil {
		return Generated{}, err
	}

	emailDelivery := s.sendPDFReadyEmail(ctx, options.NotifyEmail, item.Name, url)

	return Generated{
		Key:   key,
		URL:   url,
		Email: emailDelivery,
	}, nil
}

func (s *Service) GenerateSessionPDF(ctx context.Context, sessionID string, notifyEmail string) (Generated, error) {
	return s.GenerateSessionPDFWithOptions(ctx, sessionID, GenerateOptions{
		NotifyEmail: notifyEmail,
	})
}

func (s *Service) GenerateSessionPDFWithOptions(ctx context.Context, sessionID string, options GenerateOptions) (Generated, error) {
	options.NotifyEmail = strings.TrimSpace(options.NotifyEmail)
	if options.NotifyEmail != "" {
		if _, err := mail.ParseAddress(options.NotifyEmail); err != nil {
			return Generated{}, ErrInvalidNotifyEmail
		}
	}

	item, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return Generated{}, err
	}

	if err := wizard.ValidateComplete(item.Wizard); err != nil {
		return Generated{}, err
	}

	methodResult, err := s.sessionRecommendations.Recommend(ctx, sessionID, 2)
	if err != nil {
		return Generated{}, err
	}

	instrumentResult, err := s.sessionRecommendations.Recommend(ctx, sessionID, 3)
	if err != nil {
		return Generated{}, err
	}

	data, err := s.builder.BuildFromSession(item, methodResult.Recommendations, instrumentResult.Recommendations)
	if err != nil {
		return Generated{}, err
	}

	bytes, err := s.renderPDF(ctx, data, options)
	if err != nil {
		return Generated{}, err
	}

	key := filepath.ToSlash(fmt.Sprintf("sessions/%s/%d/%s", sessionID, time.Now().UnixNano(), docname.FileName(docname.DefaultTitle, docname.EvaluationPlan)))
	url, err := s.store.Save(ctx, key, bytes)
	if err != nil {
		return Generated{}, err
	}

	if _, err := s.sessions.UpdatePDFURL(ctx, sessionID, url); err != nil {
		return Generated{}, err
	}

	emailDelivery := s.sendPDFReadyEmail(ctx, options.NotifyEmail, "Anonymous Evaluation Plan", url)

	return Generated{
		Key:   key,
		URL:   url,
		Email: emailDelivery,
	}, nil
}

func (s *Service) renderPDF(ctx context.Context, data PlanData, options GenerateOptions) ([]byte, error) {
	if options.Template == nil {
		return s.generator.Generate(data)
	}
	if s.htmlRenderer == nil {
		return nil, ErrPDFRendererUnavailable
	}

	html, err := RenderCustomHTML(data, *options.Template)
	if err != nil {
		return nil, err
	}

	return s.htmlRenderer.Render(ctx, HTMLDocument{HTML: html})
}

func (s *Service) Download(ctx context.Context, key string) ([]byte, error) {
	return s.store.Read(ctx, key)
}

// DownloadProjectPDF returns the project's evaluation plan and its download file name.
func (s *Service) DownloadProjectPDF(ctx context.Context, userID, projectID string) ([]byte, string, error) {
	item, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	if item.UserID != userID {
		return nil, "", project.ErrForbiddenProject
	}
	if item.PDFURL == nil {
		return nil, "", ErrPDFNotFound
	}
	key, ok := s.store.KeyFromURL(*item.PDFURL)
	if !ok {
		return nil, "", ErrPDFNotFound
	}
	data, err := s.store.Read(ctx, key)
	return data, docname.FromKey(key, docname.FileName(item.Name, docname.EvaluationPlan)), err
}

// RenameProjectPDF moves the project's evaluation plan to a new file name and returns
// the new file name.
func (s *Service) RenameProjectPDF(ctx context.Context, userID, projectID, name string) (string, error) {
	fileName, err := docname.Custom(name)
	if err != nil {
		return "", err
	}
	item, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return "", err
	}
	if item.UserID != userID {
		return "", project.ErrForbiddenProject
	}
	if item.PDFURL == nil {
		return "", ErrPDFNotFound
	}
	oldKey, ok := s.store.KeyFromURL(*item.PDFURL)
	if !ok {
		return "", ErrPDFNotFound
	}
	if path.Base(oldKey) == fileName {
		return fileName, nil
	}
	data, err := s.store.Read(ctx, oldKey)
	if err != nil {
		return "", ErrPDFNotFound
	}
	newKey := fmt.Sprintf("projects/%s/%d/%s", projectID, time.Now().UnixNano(), fileName)
	url, err := s.store.Save(ctx, newKey, data)
	if err != nil {
		return "", err
	}
	if _, err := s.projects.UpdatePDFURL(ctx, projectID, url); err != nil {
		_ = s.store.Delete(ctx, newKey)
		return "", err
	}
	_ = s.store.Delete(ctx, oldKey)
	return fileName, nil
}

// DeleteProjectPDF removes the project's evaluation plan (file and link).
func (s *Service) DeleteProjectPDF(ctx context.Context, userID, projectID string) error {
	item, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if item.UserID != userID {
		return project.ErrForbiddenProject
	}
	if item.PDFURL == nil {
		return ErrPDFNotFound
	}
	if key, ok := s.store.KeyFromURL(*item.PDFURL); ok {
		_ = s.store.Delete(ctx, key) // a file already gone must not block the deletion
	}
	_, err = s.projects.UpdatePDFURL(ctx, projectID, "")
	return err
}

// DownloadSessionPDF returns the session's evaluation plan and its download file name.
func (s *Service) DownloadSessionPDF(ctx context.Context, sessionID string) ([]byte, string, error) {
	item, err := s.sessions.FindByID(ctx, sessionID)
	if err != nil {
		return nil, "", err
	}
	if item.PDFURL == nil {
		return nil, "", ErrPDFNotFound
	}
	key, ok := s.store.KeyFromURL(*item.PDFURL)
	if !ok {
		return nil, "", ErrPDFNotFound
	}
	data, err := s.store.Read(ctx, key)
	return data, docname.FromKey(key, docname.FileName(docname.DefaultTitle, docname.EvaluationPlan)), err
}

func (s *Service) sendPDFReadyEmail(ctx context.Context, notifyEmail string, title string, pdfURL string) *EmailDelivery {
	notifyEmail = strings.TrimSpace(notifyEmail)
	if notifyEmail == "" || s.mailer == nil {
		return nil
	}

	textBody := buildPDFReadyTextBody(title, pdfURL)
	htmlBody := buildPDFReadyHTMLBody(title, pdfURL)

	result, err := s.mailer.Send(ctx, mailer.Message{
		FromEmail: s.mailerFromEmail,
		FromName:  s.mailerFromName,
		To:        []string{notifyEmail},
		Subject:   "Your GamiDoc evaluation plan is ready",
		Text:      textBody,
		HTML:      htmlBody,
	})

	delivery := &EmailDelivery{
		Requested: true,
		To:        notifyEmail,
		Provider:  result.Provider,
		Sent:      err == nil && result.Accepted,
		MessageID: result.ID,
	}

	if err != nil {
		msg := err.Error()
		delivery.Error = &msg
	}

	return delivery
}

func buildPDFReadyTextBody(title string, pdfURL string) string {
	return strings.Join([]string{
		"Your GamiDoc evaluation plan is ready.",
		"",
		"Title: " + title,
		"PDF: " + pdfURL,
	}, "\n")
}

func buildPDFReadyHTMLBody(title string, pdfURL string) string {
	return "<p>Your GamiDoc evaluation plan is ready.</p><p><strong>Title:</strong> " + escapeHTML(title) + "</p><p><strong>PDF:</strong> <a href=\"" + escapeHTML(pdfURL) + "\">Download evaluation plan</a></p>"
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
