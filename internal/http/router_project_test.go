package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gamidoc/backend/internal/project"
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

	body := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestProjectStepRejectsIncompleteStep1Route(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+created.ID+"/wizard/step/1", strings.NewReader(`{"stepData":{"evaluationGoals":["Usability & Playability"]}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenValue)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
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

	saveBody := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
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

func TestProjectRecommendationUsesProjectContextRoute(t *testing.T) {
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

	saveBody := `{"stepData":{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}}`
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

	var body struct {
		ForStep         int `json:"forStep"`
		Recommendations []struct {
			ID string `json:"id"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, item := range body.Recommendations {
		if item.ID == "expert-review" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected expert-review recommendation to be present")
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
