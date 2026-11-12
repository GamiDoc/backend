package project

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gamidoc/backend/internal/recommendation"
	"github.com/gamidoc/backend/internal/wizard"
)

type fakeRepository struct {
	created Project
}

func (r *fakeRepository) Create(ctx context.Context, input Project) (Project, error) {
	input.CreatedAt = time.Now()
	input.UpdatedAt = input.CreatedAt
	r.created = input
	return input, nil
}

func (r *fakeRepository) ListByUserID(ctx context.Context, userID string, options ListOptions) (ListResult, error) {
	return ListResult{Limit: options.Limit, Offset: options.Offset}, nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id string) (Project, error) {
	return Project{}, ErrProjectNotFound
}

func (r *fakeRepository) UpdateWizard(ctx context.Context, projectID string, status wizard.Status) (Project, error) {
	return Project{}, nil
}

func (r *fakeRepository) UpdateInfo(ctx context.Context, projectID string, name string, description string) (Project, error) {
	return Project{}, nil
}

func (r *fakeRepository) Delete(ctx context.Context, projectID string) error {
	return nil
}

type fakeSessionProjectReader struct {
	source    SessionSource
	deletedID string
}

func (r *fakeSessionProjectReader) FindProjectSourceByID(ctx context.Context, sessionID string) (SessionSource, error) {
	return r.source, nil
}

func (r *fakeSessionProjectReader) Delete(ctx context.Context, sessionID string) error {
	r.deletedID = sessionID
	return nil
}

func TestCreateFromSessionCopiesPDFAndDeletesSession(t *testing.T) {
	pdfURL := "/files/pdfs/sessions/session-1/evaluation.pdf"
	step1, _ := json.Marshal(map[string]any{
		"evaluationGoals":  []string{"Usability & Playability"},
		"projectType":      "Concept test",
		"participants":     "Limited set of participants",
		"developmentStage": "Concept idea",
	})

	projectRepo := &fakeRepository{}
	sessionReader := &fakeSessionProjectReader{
		source: SessionSource{
			Wizard: wizard.Status{
				CurrentStep: 2,
				Steps: map[string]json.RawMessage{
					"1": step1,
				},
			},
			PDFURL: &pdfURL,
		},
	}

	service := NewService(
		projectRepo,
		sessionReader,
		wizard.NewService(),
		recommendation.NewService(recommendation.NewEngine(nil)),
	)

	created, err := service.CreateFromSession(context.Background(), "user-1", "session-1", ConvertInput{
		Name: "Converted",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if created.PDFURL == nil || *created.PDFURL != pdfURL {
		t.Fatalf("expected copied pdf url %q, got %v", pdfURL, created.PDFURL)
	}
	if sessionReader.deletedID != "session-1" {
		t.Fatalf("expected session to be deleted, got %q", sessionReader.deletedID)
	}
	if projectRepo.created.PDFURL == nil || *projectRepo.created.PDFURL != pdfURL {
		t.Fatalf("expected repository input to include pdf url")
	}
}

type fakePDFStore struct{ stored map[string]bool }

func (s fakePDFStore) KeyFromURL(url string) (string, bool) {
	const prefix = "/files/pdfs/"
	if len(url) <= len(prefix) || url[:len(prefix)] != prefix {
		return "", false
	}
	return url[len(prefix):], true
}

func (s fakePDFStore) Delete(ctx context.Context, key string) error { return nil }

func (s fakePDFStore) Exists(ctx context.Context, key string) (bool, error) {
	return s.stored[key], nil
}

func TestWithoutMissingPDFClearsURLOfLostFile(t *testing.T) {
	service := NewService(nil, nil, wizard.NewService(), recommendation.NewService(recommendation.NewEngine(nil))).
		WithPDFCleaner(fakePDFStore{stored: map[string]bool{"kept.pdf": true}})

	kept, lost := "/files/pdfs/kept.pdf", "/files/pdfs/lost.pdf"
	if got := service.WithoutMissingPDF(context.Background(), Project{PDFURL: &kept}); got.PDFURL == nil {
		t.Fatal("stored PDF hidden")
	}
	if got := service.WithoutMissingPDF(context.Background(), Project{PDFURL: &lost}); got.PDFURL != nil {
		t.Fatal("lost PDF still offered")
	}
}
