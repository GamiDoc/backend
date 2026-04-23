package recommendation

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gamidoc/backend/internal/wizard"
)

func TestLoadRulesFromFile(t *testing.T) {
	path := filepath.Join("..", "..", "rule", "recommendations.json")

	rules, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(rules) == 0 {
		t.Fatal("expected at least one rule")
	}
}

func TestRecommendStep2(t *testing.T) {
	engine := NewEngine(LoadDefaultRulesForTest())
	service := NewService(engine)

	step1, _ := json.Marshal(map[string]any{
		"evaluationGoals":  []string{"Usability & Playability"},
		"projectType":      "Concept test",
		"participants":     "Limited set of participants",
		"developmentStage": "Concept idea",
	})

	status := wizard.Status{
		CurrentStep: 2,
		IsComplete:  false,
		Steps: map[string]json.RawMessage{
			"1": step1,
		},
	}

	result, err := service.Recommend(status, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Recommendations) == 0 {
		t.Fatal("expected at least one recommendation")
	}
}

func TestRecommendStep3(t *testing.T) {
	engine := NewEngine(LoadDefaultRulesForTest())
	service := NewService(engine)

	step1, _ := json.Marshal(map[string]any{
		"evaluationGoals":  []string{"Usability & Playability"},
		"projectType":      "Concept test",
		"participants":     "Limited set of participants",
		"developmentStage": "Concept idea",
	})

	step2, _ := json.Marshal(map[string]any{
		"selectedMethods": []string{"surveys"},
	})

	status := wizard.Status{
		CurrentStep: 3,
		IsComplete:  false,
		Steps: map[string]json.RawMessage{
			"1": step1,
			"2": step2,
		},
	}

	result, err := service.Recommend(status, 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Recommendations) == 0 {
		t.Fatal("expected at least one recommendation")
	}
}

func TestRecommendStep2UsesProjectContext(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep:                   2,
			RequiredEvaluationGoals:   []string{"Usability & Playability"},
			RequiredProjectTypes:      []string{"Concept test"},
			RequiredParticipants:      []string{"Limited set of participants"},
			RequiredDevelopmentStages: []string{"Concept idea"},
			Recommendations: []Recommendation{
				{ID: "expert-review", Name: "Expert review"},
			},
		},
	})
	service := NewService(engine)

	step1, _ := json.Marshal(map[string]any{
		"evaluationGoals":  []string{"Usability & Playability"},
		"projectType":      "Concept test",
		"participants":     "Limited set of participants",
		"developmentStage": "Concept idea",
	})

	status := wizard.Status{
		CurrentStep: 2,
		IsComplete:  false,
		Steps: map[string]json.RawMessage{
			"1": step1,
		},
	}

	result, err := service.Recommend(status, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result.Recommendations))
	}

	if result.Recommendations[0].ID != "expert-review" {
		t.Fatalf("expected expert-review, got %q", result.Recommendations[0].ID)
	}
}

func LoadDefaultRulesForTest() []Rule {
	return []Rule{
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Usability & Playability"},
			RequiredProjectTypes:    []string{"Concept test"},
			Recommendations: []Recommendation{
				{
					ID: "think-aloud",
				},
			},
		},
		{
			ForStep:         3,
			RequiredMethods: []string{"surveys"},
			Recommendations: []Recommendation{
				{
					ID: "sus",
				},
			},
		},
	}
}
