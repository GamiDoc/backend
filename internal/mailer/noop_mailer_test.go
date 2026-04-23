package mailer

import (
	"context"
	"testing"
)

func TestNoopMailerSend(t *testing.T) {
	m := NewNoopMailer()

	result, err := m.Send(context.Background(), Message{
		FromEmail: "noreply@example.com",
		FromName:  "GamiDoc",
		To:        []string{"user@example.com"},
		Subject:   "Hello",
		Text:      "Hello",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Provider != "noop" {
		t.Fatalf("expected provider noop, got %q", result.Provider)
	}

	if result.Accepted {
		t.Fatal("expected accepted to be false")
	}
}
