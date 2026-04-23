package session

import (
	"context"

	"github.com/gamidoc/backend/internal/wizard"
)

type Repository interface {
	Create(ctx context.Context, input Session) (Session, error)
	FindByID(ctx context.Context, id string) (Session, error)
	UpdateWizard(ctx context.Context, id string, status wizard.Status) (Session, error)
}
