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

// BrevoMailer sends transactional emails with the Brevo HTTP API
// (https://developers.brevo.com/reference/sendtransacemail).
type BrevoMailer struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewBrevoMailer(apiKey string, baseURL string, client *http.Client) *BrevoMailer {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.brevo.com/v3"
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &BrevoMailer{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

type brevoAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func (m *BrevoMailer) Send(ctx context.Context, message Message) (SendResult, error) {
	payload := map[string]any{
		"sender":  brevoAddress{Email: message.FromEmail, Name: message.FromName},
		"subject": message.Subject,
	}
	to := make([]brevoAddress, 0, len(message.To))
	for _, address := range message.To {
		to = append(to, brevoAddress{Email: address})
	}
	payload["to"] = to
	if strings.TrimSpace(message.HTML) != "" {
		payload["htmlContent"] = message.HTML
	}
	if strings.TrimSpace(message.Text) != "" {
		payload["textContent"] = message.Text
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return SendResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/smtp/email", bytes.NewReader(body))
	if err != nil {
		return SendResult{}, err
	}
	req.Header.Set("api-key", m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		return SendResult{}, fmt.Errorf("brevo send failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		MessageID string `json:"messageId"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return SendResult{Provider: "brevo", Accepted: true, ID: parsed.MessageID}, nil
}
