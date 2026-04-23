package project

import (
	"time"

	"github.com/gamidoc/backend/internal/wizard"
)

type Project struct {
	ID          string        `json:"projectId"`
	UserID      string        `json:"-"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Wizard      wizard.Status `json:"wizardStatus"`
	PDFURL      *string       `json:"pdfUrl"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

func NewInitialWizardStatus() wizard.Status {
	return wizard.NewInitialStatus()
}
