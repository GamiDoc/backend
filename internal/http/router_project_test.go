package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yifen9/gamidoc-backend/internal/project"
)

func TestCreateProjectRoute(t *testing.T) {
	tokenValue := authToken()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: testProjectHandler(),
	})

	body := `{"name":"My Project","description":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestSaveProjectStepRoute(t *testing.T) {
	tokenValue := authToken()
	handler := testProjectHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: handler,
	})

	createBody := `{"name":"My Project","description":"Test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+tokenValue)
	createRec := httptest.NewRecorder()

	router.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createRec.Code)
	}

	var created project.Project
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	body := `{"stepData":{"evaluationGoals":["Usability & Playability"]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestProjectStepPrerequisiteRoute(t *testing.T) {
	tokenValue := authToken()
	handler := testProjectHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: handler,
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

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/2", strings.NewReader(`{"stepData":{"selectedMethods":["surveys"]}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRecommendProjectRoute(t *testing.T) {
	tokenValue := authToken()
	handler := testProjectHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: handler,
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

	saveBody := `{"stepData":{"evaluationGoals":["Usability & Playability"]}}`
	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("Authorization", "Bearer "+tokenValue)
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+created.ID+"/wizard/recommendations", strings.NewReader(`{"forStep":2}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestProjectInvalidRecommendationStepRoute(t *testing.T) {
	tokenValue := authToken()
	handler := testProjectHandler()

	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: handler,
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+created.ID+"/wizard/recommendations", strings.NewReader(`{"forStep":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
