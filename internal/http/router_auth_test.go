package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:       testLogger(),
		TokenManager: testTokenManager(),
		AuthHandler:  testAuthHandler(),
	})

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestProtectedProjectRouteWithoutToken(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:         testLogger(),
		TokenManager:   testTokenManager(),
		ProjectHandler: testProjectHandler(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
