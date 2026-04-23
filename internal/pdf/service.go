package pdf

import (
	"context"
	"fmt"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/yifen9/gamidoc-backend/internal/mailer"
	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/storage/objectstore"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

var ErrInvalidNotifyEmail = fmt.Errorf("invalid notify email")

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

func (s *Service) GenerateProjectPDF(ctx context.Context, userID string, projectID string, notifyEmail string) (Generated, error) {
	notifyEmail = strings.TrimSpace(notifyEmail)
	if notifyEmail != "" {
		if _, err := mail.ParseAddress(notifyEmail); err != nil {
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

	bytes, err := s.generator.Generate(data)
	if err != nil {
		return Generated{}, err
	}

	key := filepath.ToSlash(fmt.Sprintf("projects/%s/%d.pdf", projectID, time.Now().UnixNano()))
	url, err := s.store.Save(ctx, key, bytes)
	if err != nil {
		return Generated{}, err
	}

	if _, err := s.projects.UpdatePDFURL(ctx, projectID, url); err != nil {
		return Generated{}, err
	}

	emailDelivery := s.sendPDFReadyEmail(ctx, notifyEmail, item.Name, url)

	return Generated{
		Key:   key,
		URL:   url,
		Email: emailDelivery,
	}, nil
}

func (s *Service) GenerateSessionPDF(ctx context.Context, sessionID string, notifyEmail string) (Generated, error) {
	notifyEmail = strings.TrimSpace(notifyEmail)
	if notifyEmail != "" {
		if _, err := mail.ParseAddress(notifyEmail); err != nil {
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

	bytes, err := s.generator.Generate(data)
	if err != nil {
		return Generated{}, err
	}

	key := filepath.ToSlash(fmt.Sprintf("sessions/%s/%d.pdf", sessionID, time.Now().UnixNano()))
	url, err := s.store.Save(ctx, key, bytes)
	if err != nil {
		return Generated{}, err
	}

	if _, err := s.sessions.UpdatePDFURL(ctx, sessionID, url); err != nil {
		return Generated{}, err
	}

	emailDelivery := s.sendPDFReadyEmail(ctx, notifyEmail, "Anonymous Evaluation Plan", url)

	return Generated{
		Key:   key,
		URL:   url,
		Email: emailDelivery,
	}, nil
}

func (s *Service) Download(ctx context.Context, key string) ([]byte, error) {
	return s.store.Read(ctx, key)
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
