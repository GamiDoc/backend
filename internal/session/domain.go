package session

import (
	"time"

	"github.com/gamidoc/backend/internal/wizard"
)

type Session struct {
	ID        string        `json:"sessionId"`
	Wizard    wizard.Status `json:"wizardStatus"`
	CreatedAt time.Time     `json:"createdAt"`
	ExpiresAt time.Time     `json:"expiresAt"`
	PDFURL    *string       `json:"pdfUrl,omitempty"`
}

func NewInitialWizardStatus() wizard.Status {
	return wizard.NewInitialStatus()
}
