package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gamidoc/backend/internal/activity"
	"github.com/gamidoc/backend/internal/token"
)

func TestRequestActivityEventExtractsUserAndProject(t *testing.T) {
	manager := token.NewManager("secret", time.Hour)
	tokenValue, err := manager.Generate("user-1")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project-1/generate-pdf", nil)
	req.Header.Set("Authorization", "Bearer "+tokenValue)

	event := requestActivityEvent(req, manager, http.StatusOK, 25*time.Millisecond)

	if event.Type != activity.EventPDFGenerated {
		t.Fatalf("expected %q, got %q", activity.EventPDFGenerated, event.Type)
	}
	if event.UserID != "user-1" {
		t.Fatalf("expected user-1, got %q", event.UserID)
	}
	if event.ProjectID != "project-1" {
		t.Fatalf("expected project-1, got %q", event.ProjectID)
	}
}

func TestRequestActivityEventExtractsSessionStep(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/session-1/wizard/step/3", nil)

	event := requestActivityEvent(req, nil, http.StatusOK, 10*time.Millisecond)

	if event.Type != activity.EventWizardStepSaved {
		t.Fatalf("expected %q, got %q", activity.EventWizardStepSaved, event.Type)
	}
	if event.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %q", event.SessionID)
	}
	if event.Metadata["step_number"] != "3" {
		t.Fatalf("expected step number 3, got %v", event.Metadata["step_number"])
	}
}

func TestRequestActivityEventClassifiesDesignRoutes(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPut, "/api/v1/sessions/session-1/design/section/3", activity.EventDesignSectionSaved},
		{http.MethodPost, "/api/v1/projects/project-1/design/path", activity.EventDesignPathChosen},
		{http.MethodPost, "/api/v1/sessions/session-1/design/generate-pdf", activity.EventDesignPDFGenerated},
		{http.MethodPost, "/api/v1/projects/project-1/design/import-session", activity.EventDesignImported},
		{http.MethodGet, "/api/v1/sessions/session-1/design/dashboard", activity.EventAPIRequest},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		event := requestActivityEvent(req, nil, http.StatusOK, 10*time.Millisecond)
		if event.Type != c.want {
			t.Fatalf("%s %s: expected %q, got %q", c.method, c.path, c.want, event.Type)
		}
	}
}

func TestRequestActivityEventMarksFailures(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)

	event := requestActivityEvent(req, nil, http.StatusInternalServerError, 10*time.Millisecond)

	if event.Type != activity.EventServerError {
		t.Fatalf("expected %q, got %q", activity.EventServerError, event.Type)
	}
}
