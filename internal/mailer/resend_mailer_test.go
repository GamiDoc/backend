package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendMailerSend(t *testing.T) {
	var gotAuth string
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")

		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	defer server.Close()

	m := NewResendMailer("re_test_key", server.URL, server.Client())

	result, err := m.Send(context.Background(), Message{
		FromEmail: "noreply@example.com",
		FromName:  "GamiDoc",
		To:        []string{"user@example.com"},
		Subject:   "Your PDF is ready",
		Text:      "Hello",
		HTML:      "<p>Hello</p>",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if gotAuth != "Bearer re_test_key" {
		t.Fatalf("unexpected auth header %q", gotAuth)
	}

	if gotPayload["subject"] != "Your PDF is ready" {
		t.Fatalf("unexpected subject %v", gotPayload["subject"])
	}

	if result.Provider != "resend" {
		t.Fatalf("expected provider resend, got %q", result.Provider)
	}

	if !result.Accepted {
		t.Fatal("expected accepted to be true")
	}

	if result.ID != "email_123" {
		t.Fatalf("expected id email_123, got %q", result.ID)
	}
}

func TestResendMailerSendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	m := NewResendMailer("re_test_key", server.URL, server.Client())

	_, err := m.Send(context.Background(), Message{
		FromEmail: "noreply@example.com",
		To:        []string{"user@example.com"},
		Subject:   "Hello",
		Text:      "Hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
