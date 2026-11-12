package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type OpenAIAssistant struct {
	baseURL        string
	apiKey         string
	model          string
	responseFormat string
	client         *http.Client
	// retryDelays: pauses before each retry of a transient failure (len = max retries).
	retryDelays []time.Duration
	// fallbackModels (OpenRouter only): used in order when the main model fails.
	fallbackModels []string
}

// WithFallbackModels sets the models OpenRouter falls back to when the main model fails
// (e.g. a free model whose provider is unavailable). Ignored by other APIs.
func (a *OpenAIAssistant) WithFallbackModels(models []string) *OpenAIAssistant {
	a.fallbackModels = nil
	for _, model := range models {
		if model = strings.TrimSpace(model); model != "" && model != a.model {
			a.fallbackModels = append(a.fallbackModels, model)
		}
	}
	return a
}

// NewOpenAIAssistant talks to any OpenAI-compatible chat completions API. responseFormat
// says how JSON answers are requested: "json_schema" (strict structured outputs, the
// default), "json_object" (JSON mode, schema in the prompt) or "none" (schema in the prompt).
func NewOpenAIAssistant(baseURL string, apiKey string, model string, responseFormat string, client *http.Client) *OpenAIAssistant {
	if client == nil {
		client = http.DefaultClient
	}
	if responseFormat == "" {
		responseFormat = "json_schema"
	}
	return &OpenAIAssistant{
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		model:          model,
		responseFormat: responseFormat,
		client:         client,
		retryDelays:    []time.Duration{3 * time.Second, 10 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchemaFormat struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type responseFormat struct {
	Type       string            `json:"type"`
	JSONSchema *jsonSchemaFormat `json:"json_schema,omitempty"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	// Provider and Models are OpenRouter-only (never sent to other APIs, which reject them).
	Provider *openRouterRouting `json:"provider,omitempty"`
	// Models: the model followed by fallbacks, tried in order when one fails.
	Models []string `json:"models,omitempty"`
}

// openRouterRouting: with require_parameters OpenRouter only routes to upstream providers
// that support every parameter sent (here response_format), instead of silently ignoring it.
type openRouterRouting struct {
	RequireParameters bool `json:"require_parameters"`
}

func (a *OpenAIAssistant) isOpenRouter() bool {
	return strings.Contains(a.baseURL, "openrouter.ai")
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

func (a *OpenAIAssistant) complete(ctx context.Context, system string, user string) (string, error) {
	return a.completeWithFormat(ctx, system, user, nil)
}

func (a *OpenAIAssistant) completeWithFormat(ctx context.Context, system string, user string, format *responseFormat) (string, error) {
	request := chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: format,
	}
	if format != nil && a.isOpenRouter() {
		request.Provider = &openRouterRouting{RequireParameters: true}
	}
	if len(a.fallbackModels) > 0 && a.isOpenRouter() {
		request.Models = append([]string{a.model}, a.fallbackModels...)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	// Transient provider errors (rate limits, overloaded or briefly unavailable upstreams,
	// frequent on free models) are retried after a short pause.
	for attempt := 0; ; attempt++ {
		answer, retryable, err := a.send(ctx, payload)
		if err == nil || !retryable || attempt >= len(a.retryDelays) {
			return answer, err
		}
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(a.retryDelays[attempt]):
		}
	}
}

// send performs one chat completion call; retryable reports whether a failure is transient.
func (a *OpenAIAssistant) send(ctx context.Context, payload []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	if a.isOpenRouter() {
		req.Header.Set("X-Title", "GamiDoc") // app name in the OpenRouter dashboard
	}

	resp, err := a.client.Do(req)
	if err != nil {
		// Connection problems are retried; a timeout is not (the next try would be as slow).
		var netErr net.Error
		timeout := errors.As(err, &netErr) && netErr.Timeout()
		return "", ctx.Err() == nil && !timeout, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("ai provider returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
		return "", retryableStatus(resp.StatusCode), err
	}

	var decoded chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", false, err
	}
	if len(decoded.Choices) == 0 {
		// OpenRouter reports some upstream failures as 200 without choices.
		return "", true, fmt.Errorf("ai provider returned no choices")
	}
	if decoded.Choices[0].FinishReason == "length" {
		return "", false, fmt.Errorf("ai provider truncated the answer (finish_reason=length)")
	}

	answer := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if answer == "" {
		return "", true, fmt.Errorf("ai provider returned an empty completion")
	}

	return answer, false, nil
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (a *OpenAIAssistant) CompleteJSON(ctx context.Context, request JSONRequest) (json.RawMessage, error) {
	system := request.System
	var format *responseFormat
	switch a.responseFormat {
	case "json_schema":
		format = &responseFormat{
			Type:       "json_schema",
			JSONSchema: &jsonSchemaFormat{Name: request.Name, Strict: true, Schema: request.Schema},
		}
	case "json_object":
		format = &responseFormat{Type: "json_object"}
		system += "\n\nReply with a single JSON object that matches this JSON Schema:\n" + string(request.Schema)
	default:
		system += "\n\nReply with a single JSON object, and nothing else, that matches this JSON Schema:\n" + string(request.Schema)
	}

	answer, err := a.completeWithFormat(ctx, system, request.User, format)
	if err != nil {
		return nil, err
	}
	raw := extractJSON(answer)
	if !json.Valid(raw) {
		return nil, fmt.Errorf("ai provider returned invalid JSON")
	}
	return raw, nil
}

// extractJSON strips Markdown code fences and any text around the outermost JSON object.
func extractJSON(answer string) json.RawMessage {
	trimmed := strings.TrimSpace(answer)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	first := strings.Index(trimmed, "{")
	last := strings.LastIndex(trimmed, "}")
	if first >= 0 && last > first {
		return json.RawMessage(trimmed[first : last+1])
	}
	return json.RawMessage(trimmed)
}

func (a *OpenAIAssistant) Rewrite(ctx context.Context, text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ErrEmptyText
	}
	return a.complete(ctx,
		"Rewrite the user's text into grammatically correct, professionally phrased prose. Preserve the meaning. Do not add new claims. Reply with the rewritten text only.",
		trimmed,
	)
}

func (a *OpenAIAssistant) Chat(ctx context.Context, section Section, message string) (string, error) {
	system := fmt.Sprintf(
		"You are the GamiDoc assistant. The user is filling in the '%s' section of a gamification design document. Give concise, concrete guidance for this section. Clarify domain terminology when asked.",
		section.Name,
	)
	return a.complete(ctx, system, message)
}
