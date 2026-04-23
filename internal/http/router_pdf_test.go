package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yifen9/gamidoc-backend/internal/pdf"
	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/storage/r2"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

func TestProjectPDFRoute(t *testing.T) {
	tokenValue := authToken()

	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	projectHandler := project.NewHandler(projectService)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	store := r2.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: projectHandler,
		PDFHandler:     pdfHandler,
	})

	createBody := `{"name":"My Project","description":"Test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+tokenValue)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created project.Project
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	save1 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`))
	save1.Header.Set("Content-Type", "application/json")
	save1.Header.Set("Authorization", "Bearer "+tokenValue)
	save1Rec := httptest.NewRecorder()
	router.ServeHTTP(save1Rec, save1)

	save2 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/2", strings.NewReader(`{"stepData":{"selectedMethods":["surveys"]}}`))
	save2.Header.Set("Content-Type", "application/json")
	save2.Header.Set("Authorization", "Bearer "+tokenValue)
	save2Rec := httptest.NewRecorder()
	router.ServeHTTP(save2Rec, save2)

	save3 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/3", strings.NewReader(`{"stepData":{"selectedInstruments":["USEQ-Like","SUS"]}}`))
	save3.Header.Set("Content-Type", "application/json")
	save3.Header.Set("Authorization", "Bearer "+tokenValue)
	save3Rec := httptest.NewRecorder()
	router.ServeHTTP(save3Rec, save3)

	save4 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/4", strings.NewReader(`{"stepData":{"nextSteps":["Prepare materials","Run evaluation"],"notes":"Start with a pilot run."}}`))
	save4.Header.Set("Content-Type", "application/json")
	save4.Header.Set("Authorization", "Bearer "+tokenValue)
	save4Rec := httptest.NewRecorder()
	router.ServeHTTP(save4Rec, save4)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+created.ID+"/generate-pdf", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestProjectPDFIncompleteWizardRoute(t *testing.T) {
	tokenValue := authToken()

	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	projectHandler := project.NewHandler(projectService)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	store := r2.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: projectHandler,
		PDFHandler:     pdfHandler,
	})

	createBody := `{"name":"My Project","description":"Test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+tokenValue)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created project.Project
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	save1 := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`))
	save1.Header.Set("Content-Type", "application/json")
	save1.Header.Set("Authorization", "Bearer "+tokenValue)
	save1Rec := httptest.NewRecorder()
	router.ServeHTTP(save1Rec, save1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+created.ID+"/generate-pdf", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSessionPDFRoute(t *testing.T) {
	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	sessionHandler := session.NewHandler(sessionService, projectService)

	store := r2.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: sessionHandler,
		PDFHandler:     pdfHandler,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	save1 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`))
	save1.Header.Set("Content-Type", "application/json")
	save1Rec := httptest.NewRecorder()
	router.ServeHTTP(save1Rec, save1)

	save2 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/2", strings.NewReader(`{"stepData":{"selectedMethods":["surveys"]}}`))
	save2.Header.Set("Content-Type", "application/json")
	save2Rec := httptest.NewRecorder()
	router.ServeHTTP(save2Rec, save2)

	save3 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/3", strings.NewReader(`{"stepData":{"selectedInstruments":["USEQ-Like","SUS"]}}`))
	save3.Header.Set("Content-Type", "application/json")
	save3Rec := httptest.NewRecorder()
	router.ServeHTTP(save3Rec, save3)

	save4 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/4", strings.NewReader(`{"stepData":{"nextSteps":["Prepare materials","Run evaluation"],"notes":"Start with a pilot run."}}`))
	save4.Header.Set("Content-Type", "application/json")
	save4Rec := httptest.NewRecorder()
	router.ServeHTTP(save4Rec, save4)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+created.ID+"/generate-pdf", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestSessionPDFIncompleteWizardRoute(t *testing.T) {
	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	sessionHandler := session.NewHandler(sessionService, projectService)

	store := r2.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: sessionHandler,
		PDFHandler:     pdfHandler,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	save1 := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`))
	save1.Header.Set("Content-Type", "application/json")
	save1Rec := httptest.NewRecorder()
	router.ServeHTTP(save1Rec, save1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+created.ID+"/generate-pdf", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestPDFDownloadMissingRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:     testLogger(),
		PDFHandler: testPDFHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/files/pdfs/missing/file.pdf", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestPDFDownloadRoute(t *testing.T) {
	root := tTempDir()
	store := r2.NewLocalStore(root, "/files/pdfs")

	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	_, err := store.Save(context.Background(), "projects/test/file.pdf", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	router := NewRouter(Dependencies{
		Logger:     testLogger(),
		PDFHandler: pdfHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/files/pdfs/projects/test/file.pdf", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("expected application/pdf, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestProjectPDFForbiddenRoute(t *testing.T) {
	projectRepo := &fakeProjectRepository{
		items: []project.Project{
			{
				ID:          "project-1",
				UserID:      "another-user",
				Name:        "My Project",
				Description: "Test",
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
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		byID: map[string]project.Project{},
	}
	projectRepo.byID["project-1"] = projectRepo.items[0]

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	projectHandler := project.NewHandler(projectService)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	store := r2.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	pdfService := pdf.NewService(
		builder,
		generator,
		store,
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	pdfHandler := pdf.NewHandler(pdfService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: projectHandler,
		PDFHandler:     pdfHandler,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project-1/generate-pdf", nil)
	req.Header.Set("Authorization", "Bearer "+authToken())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}
