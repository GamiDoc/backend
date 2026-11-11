package ai

import (
	"context"
	"encoding/json"
	"strings"
)

type NoopAssistant struct{}

func NewNoopAssistant() *NoopAssistant {
	return &NoopAssistant{}
}

var sectionGuidance = map[int]string{
	1: "Describe the context: the domain and aim of the software, who the users are and which behaviour you want to encourage, and the constraints they face.",
	2: "Describe the experience over time: onboarding, how often the system is updated, what keeps users coming back, the drop-out risks, the goal types, and the emotional dynamics.",
	3: "Describe personalization and dynamics: the user types you design for, what players or the system can personalize, and how difficulty adapts over time.",
	4: "Describe the gameful core: the game rules and loops, the game elements and mechanics, how players progress, and the modality and timing of play.",
	5: "Describe feedback and evaluation: when and how the system gives feedback to players, and how you will know the design is working.",
	6: "Describe the technical requirements: platforms, engine and stack, performance budgets, data and online features, integrations, and technical risks.",
}

func (a *NoopAssistant) Rewrite(ctx context.Context, text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ErrEmptyText
	}
	return strings.Join(strings.Fields(trimmed), " "), nil
}

func (a *NoopAssistant) Chat(ctx context.Context, section Section, message string) (string, error) {
	guidance, ok := sectionGuidance[section.Number]
	if !ok {
		guidance = "Fill in the section with the information most relevant to your project."
	}
	return guidance, nil
}

func (a *NoopAssistant) SupportsJSON() bool { return false }

// CompleteJSON is not available without a model: callers fall back (e.g. no prefill).
func (a *NoopAssistant) CompleteJSON(ctx context.Context, request JSONRequest) (json.RawMessage, error) {
	return nil, ErrUnsupported
}

func (a *NoopAssistant) Enhance(ctx context.Context, sections []SectionText) ([]SectionText, error) {
	seen := map[string]bool{}
	result := make([]SectionText, 0, len(sections))
	for _, section := range sections {
		var kept []string
		for _, line := range strings.Split(section.Text, "\n") {
			normalized := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
			if normalized == "" || seen[strings.ToLower(normalized)] {
				continue
			}
			seen[strings.ToLower(normalized)] = true
			kept = append(kept, normalized)
		}
		section.Text = strings.Join(kept, " ")
		result = append(result, section)
	}
	return result, nil
}
