package project

import (
	"context"

	"github.com/gamidoc/backend/internal/wizard"
)

type Repository interface {
	Create(ctx context.Context, input Project) (Project, error)
	ListByUserID(ctx context.Context, userID string) ([]Project, error)
	FindByID(ctx context.Context, id string) (Project, error)
	UpdateWizard(ctx context.Context, projectID string, status wizard.Status) (Project, error)
	UpdateInfo(ctx context.Context, projectID string, name string, description string) (Project, error)
	Delete(ctx context.Context, projectID string) error
}
