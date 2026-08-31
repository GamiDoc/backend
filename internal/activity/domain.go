package activity

import (
	"context"
	"time"
)

const (
	EventAPIRequest              = "api_request"
	EventAPIRequestFailed        = "api_request_failed"
	EventServerError             = "server_error"
	EventUserRegistered          = "user_registered"
	EventUserLogin               = "user_login"
	EventUserLogout              = "user_logout"
	EventTokenRefreshed          = "token_refreshed"
	EventProjectCreated          = "project_created"
	EventProjectUpdated          = "project_updated"
	EventProjectDeleted          = "project_deleted"
	EventWizardStepSaved         = "wizard_step_saved"
	EventRecommendationRequested = "recommendation_requested"
	EventPDFGenerated            = "pdf_generated"
	EventPDFDownloaded           = "pdf_downloaded"
	EventSessionCreated          = "session_created"
	EventSessionConverted        = "session_converted"
	EventDesignSectionSaved      = "design_section_saved"
	EventDesignPathChosen        = "design_path_chosen"
	EventDesignPDFGenerated      = "design_pdf_generated"
	EventDesignImported          = "design_imported"
	EventFrontendPrefix          = "frontend_"
)

type Event struct {
	ID         string
	Type       string
	UserID     string
	SessionID  string
	ProjectID  string
	Method     string
	Path       string
	StatusCode int
	Duration   time.Duration
	Metadata   map[string]any
	CreatedAt  time.Time
}

type Recorder interface {
	Record(ctx context.Context, event Event) error
}

type Repository interface {
	Save(ctx context.Context, event Event) error
}

type NoopRecorder struct{}

func (NoopRecorder) Record(ctx context.Context, event Event) error {
	return nil
}
