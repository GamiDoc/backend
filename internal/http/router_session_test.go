package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

func TestCreateSessionRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: testSessionHandler(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestSaveSessionStepRoute(t *testing.T) {
	handler := testSessionHandler()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: handler,
	})

	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createRec.Code)
	}

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	body := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestSessionStepRejectsIncompleteStep1Route(t *testing.T) {
	handler := testSessionHandler()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: handler,
	})

	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSessionStepPrerequisiteRoute(t *testing.T) {
	handler := testSessionHandler()
	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: handler,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/2", strings.NewReader(`{"stepData":{"selectedMethods":["surveys"]}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRecommendSessionRoute(t *testing.T) {
	handler := testSessionHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: handler,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	saveBody := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+created.ID+"/wizard/recommendations", strings.NewReader(`{"forStep":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestConvertSessionRoute(t *testing.T) {
	tokenValue := authToken()
	sessionHandler := testSessionHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		SessionHandler: sessionHandler,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/create", nil)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	var created session.Session
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	saveBody := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/"+created.ID+"/wizard/step/1", strings.NewReader(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)

	convertBody := `{"name":"Converted Project","description":"Created from session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+created.ID+"/convert", strings.NewReader(convertBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestSessionNotFoundRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		SessionHandler: testSessionHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/missing", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestConvertMissingSessionRoute(t *testing.T) {
	tokenValue := authToken()

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		testRecommendationService(),
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		testRecommendationService(),
	)

	handler := session.NewHandler(sessionService, projectService)

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		SessionHandler: handler,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/missing/convert", strings.NewReader(`{"name":"Converted Project"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
