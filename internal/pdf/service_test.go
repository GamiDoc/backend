package pdf

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yifen9/gamidoc-backend/internal/mailer"
	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

type fakeObjectStore struct {
	items map[string][]byte
}

func (s *fakeObjectStore) Save(ctx context.Context, key string, data []byte) (string, error) {
	if s.items == nil {
		s.items = map[string][]byte{}
	}
	s.items[key] = data
	return "/files/pdfs/" + key, nil
}

func (s *fakeObjectStore) Read(ctx context.Context, key string) ([]byte, error) {
	value, ok := s.items[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return value, nil
}

type fakeMailer struct {
	sendErr error
	result  mailer.SendResult
	sent    []mailer.Message
}

func (m *fakeMailer) Send(ctx context.Context, message mailer.Message) (mailer.SendResult, error) {
	m.sent = append(m.sent, message)
	if m.sendErr != nil {
		return mailer.SendResult{Provider: "fake"}, m.sendErr
	}
	if m.result.Provider == "" {
		m.result.Provider = "fake"
	}
	return m.result, nil
}

type fakeProjectRecommendationService struct {
	result recommendation.Result
	err    error
}

func (s *fakeProjectRecommendationService) Recommend(ctx context.Context, userID string, projectID string, forStep int) (recommendation.Result, error) {
	if s.err != nil {
		return recommendation.Result{}, s.err
	}
	return s.result, nil
}

type fakeSessionRecommendationService struct {
	result recommendation.Result
	err    error
}

func (s *fakeSessionRecommendationService) Recommend(ctx context.Context, sessionID string, forStep int) (recommendation.Result, error) {
	if s.err != nil {
		return recommendation.Result{}, s.err
	}
	return s.result, nil
}

type fakeProjectPDFRepository struct {
	item project.Project
}

func (r *fakeProjectPDFRepository) FindByID(ctx context.Context, id string) (project.Project, error) {
	if r.item.ID == "" {
		return project.Project{}, project.ErrProjectNotFound
	}
	return r.item, nil
}

func (r *fakeProjectPDFRepository) UpdatePDFURL(ctx context.Context, projectID string, pdfURL string) (project.Project, error) {
	r.item.PDFURL = &pdfURL
	return r.item, nil
}

type fakeSessionPDFRepository struct {
	item session.Session
}

func (r *fakeSessionPDFRepository) FindByID(ctx context.Context, id string) (session.Session, error) {
	if r.item.ID == "" {
		return session.Session{}, session.ErrSessionNotFound
	}
	return r.item, nil
}

func (r *fakeSessionPDFRepository) UpdatePDFURL(ctx context.Context, sessionID string, pdfURL string) (session.Session, error) {
	r.item.PDFURL = &pdfURL
	return r.item, nil
}

func TestGenerateProjectPDFWithEmailSuccess(t *testing.T) {
	store := &fakeObjectStore{}
	m := &fakeMailer{
		result: mailer.SendResult{
			Provider: "resend",
			Accepted: true,
			ID:       "email_123",
		},
	}

	projectRepo := &fakeProjectPDFRepository{
		item: project.Project{
			ID:        "project-1",
			UserID:    "user-1",
			Name:      "My Project",
			CreatedAt: time.Now(),
			Wizard: wizard.Status{
				CurrentStep: 4,
				IsComplete:  true,
				Steps: map[string]json.RawMessage{
					"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`),
					"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
					"3": json.RawMessage(`{"selectedInstruments":["USEQ-Like"]}`),
					"4": json.RawMessage(`{"nextSteps":["Run evaluation"],"notes":"Pilot first."}`),
				},
			},
		},
	}

	sessionRepo := &fakeSessionPDFRepository{}
	rec := recommendation.Result{
		ForStep: 2,
		Recommendations: []recommendation.Recommendation{
			{ID: "surveys", Name: "Surveys & Questionnaires", Priority: "Recommended"},
		},
	}

	service := NewService(
		NewBuilder(),
		NewFPDFGenerator(),
		store,
		m,
		"noreply@example.com",
		"GamiDoc",
		projectRepo,
		sessionRepo,
		&fakeProjectRecommendationService{result: rec},
		&fakeSessionRecommendationService{},
	)

	result, err := service.GenerateProjectPDF(context.Background(), "user-1", "project-1", "user@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Email == nil {
		t.Fatal("expected email result")
	}

	if !result.Email.Sent {
		t.Fatal("expected email to be sent")
	}

	if result.Email.Provider != "resend" {
		t.Fatalf("expected resend provider, got %q", result.Email.Provider)
	}

	if len(m.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(m.sent))
	}
}

func TestGenerateProjectPDFWithEmailFailureDoesNotFailMainFlow(t *testing.T) {
	store := &fakeObjectStore{}
	m := &fakeMailer{
		sendErr: errors.New("mailer down"),
	}

	projectRepo := &fakeProjectPDFRepository{
		item: project.Project{
			ID:        "project-1",
			UserID:    "user-1",
			Name:      "My Project",
			CreatedAt: time.Now(),
			Wizard: wizard.Status{
				CurrentStep: 4,
				IsComplete:  true,
				Steps: map[string]json.RawMessage{
					"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`),
					"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
					"3": json.RawMessage(`{"selectedInstruments":["USEQ-Like"]}`),
					"4": json.RawMessage(`{"nextSteps":["Run evaluation"],"notes":"Pilot first."}`),
				},
			},
		},
	}

	sessionRepo := &fakeSessionPDFRepository{}
	rec := recommendation.Result{
		ForStep: 2,
		Recommendations: []recommendation.Recommendation{
			{ID: "surveys", Name: "Surveys & Questionnaires", Priority: "Recommended"},
		},
	}

	service := NewService(
		NewBuilder(),
		NewFPDFGenerator(),
		store,
		m,
		"noreply@example.com",
		"GamiDoc",
		projectRepo,
		sessionRepo,
		&fakeProjectRecommendationService{result: rec},
		&fakeSessionRecommendationService{},
	)

	result, err := service.GenerateProjectPDF(context.Background(), "user-1", "project-1", "user@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Email == nil {
		t.Fatal("expected email result")
	}

	if result.Email.Sent {
		t.Fatal("expected email to be marked as not sent")
	}

	if result.URL == "" {
		t.Fatal("expected pdf url to be set")
	}
}

func TestGenerateProjectPDFRejectsInvalidNotifyEmail(t *testing.T) {
	store := &fakeObjectStore{}
	m := &fakeMailer{}
	projectRepo := &fakeProjectPDFRepository{}
	sessionRepo := &fakeSessionPDFRepository{}

	service := NewService(
		NewBuilder(),
		NewFPDFGenerator(),
		store,
		m,
		"noreply@example.com",
		"GamiDoc",
		projectRepo,
		sessionRepo,
		&fakeProjectRecommendationService{},
		&fakeSessionRecommendationService{},
	)

	_, err := service.GenerateProjectPDF(context.Background(), "user-1", "project-1", "not-an-email")
	if !errors.Is(err, ErrInvalidNotifyEmail) {
		t.Fatalf("expected ErrInvalidNotifyEmail, got %v", err)
	}
}
