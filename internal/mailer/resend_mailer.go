package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ResendMailer struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewResendMailer(apiKey string, baseURL string, client *http.Client) *ResendMailer {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.resend.com"
	}
	if client == nil {
		client = http.DefaultClient
	}

	return &ResendMailer{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (m *ResendMailer) Send(ctx context.Context, message Message) (SendResult, error) {
	payload := map[string]any{
		"from":    formatFrom(message.FromEmail, message.FromName),
		"to":      message.To,
		"subject": message.Subject,
	}

	if strings.TrimSpace(message.Text) != "" {
		payload["text"] = message.Text
	}

	if strings.TrimSpace(message.HTML) != "" {
		payload["html"] = message.HTML
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SendResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return SendResult{}, err
	}

	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return SendResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return SendResult{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResult{}, fmt.Errorf("resend send failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		ID string `json:"id"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &parsed)
	}

	return SendResult{
		Provider: "resend",
		Accepted: true,
		ID:       parsed.ID,
	}, nil
}

func formatFrom(email string, name string) string {
	if strings.TrimSpace(name) == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}
