package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/activity"
	"github.com/gamidoc/backend/internal/token"
)

func Activity(recorder activity.Recorder, manager *token.Manager, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if recorder == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(lrw, r)

			event := requestActivityEvent(r, manager, lrw.status, time.Since(start))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := recorder.Record(ctx, event); err != nil && logger != nil {
				logger.Warn(
					"activity_record_failed",
					"request_id", GetRequestID(r.Context()),
					"error", err.Error(),
				)
			}
		})
	}
}

func requestActivityEvent(r *http.Request, manager *token.Manager, status int, duration time.Duration) activity.Event {
	sessionID, projectID := pathResourceIDs(r.URL.Path)
	userID := bearerUserID(r, manager)

	metadata := map[string]any{
		"request_id":  GetRequestID(r.Context()),
		"remote_addr": r.RemoteAddr,
	}
	if sessionID != "" {
		metadata["actor_type"] = "anonymous_session"
	} else if userID != "" {
		metadata["actor_type"] = "authenticated_user"
	} else {
		metadata["actor_type"] = "anonymous"
	}
	if stepNumber := wizardStepNumber(r.URL.Path); stepNumber != "" {
		metadata["step_number"] = stepNumber
	}

	return activity.Event{
		Type:       requestEventType(r.Method, r.URL.Path, status),
		UserID:     userID,
		SessionID:  sessionID,
		ProjectID:  projectID,
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: status,
		Duration:   duration,
		Metadata:   metadata,
	}
}

func bearerUserID(r *http.Request, manager *token.Manager) string {
	if manager == nil {
		return ""
	}

	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
		return ""
	}

	claims, err := manager.Parse(parts[1])
	if err != nil {
		return ""
	}
	return claims.UserID
}

func pathResourceIDs(path string) (sessionID string, projectID string) {
	segments := pathSegments(path)
	for i := 0; i < len(segments)-1; i++ {
		switch segments[i] {
		case "sessions":
			sessionID = segments[i+1]
		case "projects":
			projectID = segments[i+1]
		}
	}
	return sessionID, projectID
}

func wizardStepNumber(path string) string {
	segments := pathSegments(path)
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == "step" {
			return segments[i+1]
		}
	}
	return ""
}

func requestEventType(method string, path string, status int) string {
	if status >= 500 {
		return activity.EventServerError
	}
	if status >= 400 {
		return activity.EventAPIRequestFailed
	}

	segments := pathSegments(path)
	if len(segments) < 3 || segments[0] != "api" || segments[1] != "v1" {
		return activity.EventAPIRequest
	}

	if containsSegment(segments, "design") {
		switch {
		case method == http.MethodPut && containsSegment(segments, "section"):
			return activity.EventDesignSectionSaved
		case method == http.MethodPost && lastSegment(segments) == "path":
			return activity.EventDesignPathChosen
		case method == http.MethodPost && lastSegment(segments) == "generate-pdf":
			return activity.EventDesignPDFGenerated
		case method == http.MethodPost && lastSegment(segments) == "import-session":
			return activity.EventDesignImported
		}
		return activity.EventAPIRequest
	}

	if len(segments) >= 4 && segments[2] == "auth" {
		switch {
		case method == http.MethodPost && segments[3] == "register":
			return activity.EventUserRegistered
		case method == http.MethodPost && segments[3] == "login":
			return activity.EventUserLogin
		case method == http.MethodPost && segments[3] == "refresh":
			return activity.EventTokenRefreshed
		case method == http.MethodPost && segments[3] == "logout":
			return activity.EventUserLogout
		}
	}

	if len(segments) >= 3 && segments[2] == "projects" {
		switch {
		case method == http.MethodPost && len(segments) == 3:
			return activity.EventProjectCreated
		case method == http.MethodPatch && len(segments) == 4:
			return activity.EventProjectUpdated
		case method == http.MethodDelete && len(segments) == 4:
			return activity.EventProjectDeleted
		case method == http.MethodPut && hasPathSuffix(segments, "wizard", "step"):
			return activity.EventWizardStepSaved
		case method == http.MethodPost && lastSegment(segments) == "recommendations":
			return activity.EventRecommendationRequested
		case method == http.MethodPost && lastSegment(segments) == "generate-pdf":
			return activity.EventPDFGenerated
		case method == http.MethodGet && lastSegment(segments) == "download-pdf":
			return activity.EventPDFDownloaded
		}
	}

	if len(segments) >= 3 && segments[2] == "sessions" {
		switch {
		case method == http.MethodPost && len(segments) == 4 && segments[3] == "create":
			return activity.EventSessionCreated
		case method == http.MethodPost && (lastSegment(segments) == "convert" || lastSegment(segments) == "convert-to-project"):
			return activity.EventSessionConverted
		case method == http.MethodPut && hasPathSuffix(segments, "wizard", "step"):
			return activity.EventWizardStepSaved
		case method == http.MethodPost && lastSegment(segments) == "recommendations":
			return activity.EventRecommendationRequested
		case method == http.MethodPost && lastSegment(segments) == "generate-pdf":
			return activity.EventPDFGenerated
		case method == http.MethodGet && lastSegment(segments) == "download-pdf":
			return activity.EventPDFDownloaded
		}
	}

	return activity.EventAPIRequest
}

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func containsSegment(segments []string, value string) bool {
	for _, segment := range segments {
		if segment == value {
			return true
		}
	}
	return false
}

func hasPathSuffix(segments []string, first string, second string) bool {
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == first && segments[i+1] == second {
			return true
		}
	}
	return false
}

func lastSegment(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	return segments[len(segments)-1]
}
