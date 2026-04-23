package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSPreflightRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:             testLogger(),
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/ping", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected allow origin %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestBodyLimitRejectsLargeRequest(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:       testLogger(),
		AuthHandler:  testAuthHandler(),
		TokenManager: testTokenManager(),
		MaxBodyBytes: 16,
	})

	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
}
