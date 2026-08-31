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

var experienceTerms = []string{
	"experience", "journey", "timeline", "player", "user", "persona", "story", "narrative", "emotion", "motivation",
}

var mechanicsTerms = []string{
	"mechanic", "points", "score", "badge", "leaderboard", "reward", "challenge", "level", "technology", "platform", "app",
}

var sectionGuidance = map[int]string{
	1: "Describe the project context: the domain, the target audience, and the problem the gamified system should address.",
	2: "Describe the experience over time: how a user first meets the system, what a typical session looks like, and how the experience evolves.",
	3: "Describe the personification and dynamics: personas or player types, and the social or competitive dynamics between them.",
	4: "Describe the gameful core: the central mechanics, rules, and reward structures.",
	5: "Describe the technology: platforms, devices, integrations, and technical constraints.",
	6: "Describe the impacts and benefits: the intended behavioural, learning, or business outcomes.",
	7: "Describe evaluation and feedback: how the system's effect will be measured and how feedback loops reach the users.",
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

func (a *NoopAssistant) RecommendBranch(ctx context.Context, spark string) (string, error) {
	lowered := strings.ToLower(spark)
	experience := 0
	mechanics := 0
	for _, term := range experienceTerms {
		experience += strings.Count(lowered, term)
	}
	for _, term := range mechanicsTerms {
		mechanics += strings.Count(lowered, term)
	}
	if mechanics > experience {
		return "B", nil
	}
	return "A", nil
}

func (a *NoopAssistant) Prefill(ctx context.Context, spark string, sections []Section) (map[int]json.RawMessage, error) {
	trimmed := strings.TrimSpace(spark)
	if trimmed == "" {
		return map[int]json.RawMessage{}, nil
	}
	draft, err := json.Marshal(map[string]string{"draft": trimmed})
	if err != nil {
		return nil, err
	}
	return map[int]json.RawMessage{1: draft}, nil
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
