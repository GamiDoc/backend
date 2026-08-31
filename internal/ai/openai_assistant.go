package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type OpenAIAssistant struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAIAssistant(baseURL string, apiKey string, model string, client *http.Client) *OpenAIAssistant {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAIAssistant{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  client,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (a *OpenAIAssistant) complete(ctx context.Context, system string, user string) (string, error) {
	payload, err := json.Marshal(chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai provider returned status %d", resp.StatusCode)
	}

	var decoded chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("ai provider returned no choices")
	}

	answer := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("ai provider returned an empty completion")
	}

	return answer, nil
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

func (a *OpenAIAssistant) RecommendBranch(ctx context.Context, spark string) (string, error) {
	answer, err := a.complete(ctx,
		"The user wrote a free-form project idea. Answer with the single letter A if the text mostly concerns user experience, personas, or journeys; answer B if it mostly concerns game mechanics, rewards, or technology. Answer with A or B only.",
		spark,
	)
	if err != nil {
		return "", err
	}
	words := strings.FieldsFunc(strings.ToUpper(answer), func(r rune) bool {
		return r < 'A' || r > 'Z'
	})
	for _, word := range words {
		if word == "A" || word == "B" {
			return word, nil
		}
	}
	return "A", nil
}

func (a *OpenAIAssistant) Prefill(ctx context.Context, spark string, sections []Section) (map[int]json.RawMessage, error) {
	trimmed := strings.TrimSpace(spark)
	if trimmed == "" {
		return map[int]json.RawMessage{}, nil
	}

	var names []string
	for _, section := range sections {
		names = append(names, fmt.Sprintf("%d: %s", section.Number, section.Name))
	}
	system := fmt.Sprintf(
		"From the user's project idea, draft initial content for the sections of a gamification design document: %s. Reply with a JSON object whose keys are the section numbers as strings and whose values are objects with a single 'draft' string field. Include only sections the idea gives real material for. Reply with JSON only.",
		strings.Join(names, "; "),
	)

	answer, err := a.complete(ctx, system, trimmed)
	if err != nil {
		return nil, err
	}
	answer = strings.TrimPrefix(answer, "```json")
	answer = strings.Trim(answer, "` \n")

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answer), &decoded); err != nil {
		return nil, fmt.Errorf("ai provider returned unparseable prefill: %w", err)
	}

	result := make(map[int]json.RawMessage, len(decoded))
	for key, value := range decoded {
		number, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		result[number] = value
	}
	return result, nil
}

func (a *OpenAIAssistant) Enhance(ctx context.Context, sections []SectionText) ([]SectionText, error) {
	var filtered []SectionText
	var parts []string
	for _, section := range sections {
		if strings.TrimSpace(section.Text) == "" {
			continue
		}
		filtered = append(filtered, section)
		parts = append(parts, fmt.Sprintf("Section %d (%s):\n%s", section.Number, section.Name, section.Text))
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	answer, err := a.complete(ctx,
		"Rewrite the raw notes of a gamification design report into naturally readable prose, section by section. Preserve the meaning and facts; do not add new claims. Where content repeats across sections, keep it in the most fitting section and drop the repetitions elsewhere. Reply with a JSON object whose keys are the section numbers as strings and whose values are the prose strings. Reply with JSON only.",
		strings.Join(parts, "\n\n"),
	)
	if err != nil {
		return nil, err
	}
	answer = strings.TrimPrefix(answer, "```json")
	answer = strings.Trim(answer, "` \n")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(answer), &decoded); err != nil {
		return nil, fmt.Errorf("ai provider returned unparseable enhancement: %w", err)
	}

	result := make([]SectionText, 0, len(filtered))
	for _, section := range filtered {
		if prose, ok := decoded[strconv.Itoa(section.Number)]; ok && strings.TrimSpace(prose) != "" {
			section.Text = strings.TrimSpace(prose)
		}
		result = append(result, section)
	}
	return result, nil
}
