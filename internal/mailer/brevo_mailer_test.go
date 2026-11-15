package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrevoMailerSend(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/smtp/email" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("api-key") != "key" {
			t.Errorf("missing api key header")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"<abc@brevo>"}`))
	}))
	defer server.Close()

	m := NewBrevoMailer("key", server.URL, server.Client())
	result, err := m.Send(context.Background(), Message{
		FromEmail: "noreply@example.com", FromName: "GamiDoc",
		To: []string{"user@example.com"}, Subject: "Hi", Text: "Hi", HTML: "<p>Hi</p>",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if !result.Accepted || result.ID != "<abc@brevo>" || result.Provider != "brevo" {
		t.Fatalf("unexpected result %+v", result)
	}
	if got["subject"] != "Hi" || got["htmlContent"] != "<p>Hi</p>" {
		t.Fatalf("unexpected payload %v", got)
	}
	to := got["to"].([]any)[0].(map[string]any)
	if to["email"] != "user@example.com" {
		t.Fatalf("unexpected recipient %v", to)
	}
}

func TestBrevoMailerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	if _, err := NewBrevoMailer("bad", server.URL, server.Client()).Send(context.Background(), Message{To: []string{"a@b.c"}}); err == nil {
		t.Fatal("expected an error")
	}
}
