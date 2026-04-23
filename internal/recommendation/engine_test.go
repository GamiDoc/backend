package recommendation

import "testing"

func TestEngineRecommendMatchesRequiredGoals(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Usability & Playability"},
			Recommendations: []Recommendation{
				{ID: "think-aloud", Name: "Think-aloud testing"},
			},
		},
	})

	result := engine.Recommend(Input{
		ForStep:         2,
		EvaluationGoals: []string{"Usability & Playability"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result))
	}
}

func TestEngineRecommendMatchesRequiredMethods(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep:         3,
			RequiredMethods: []string{"surveys"},
			Recommendations: []Recommendation{
				{ID: "sus", Name: "SUS"},
			},
		},
	})

	result := engine.Recommend(Input{
		ForStep:         3,
		SelectedMethods: []string{"surveys"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result))
	}
}

func TestEngineRecommendMatchesProjectContext(t *testing.T) {
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

	result := engine.Recommend(Input{
		ForStep:          2,
		EvaluationGoals:  []string{"Usability & Playability"},
		ProjectType:      "Concept test",
		Participants:     "Limited set of participants",
		DevelopmentStage: "Concept idea",
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result))
	}

	if result[0].ID != "expert-review" {
		t.Fatalf("expected expert-review, got %q", result[0].ID)
	}
}

func TestEngineRecommendDeduplicates(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Usability & Playability"},
			Recommendations: []Recommendation{
				{ID: "surveys", Name: "Surveys"},
			},
		},
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Usability & Playability"},
			Recommendations: []Recommendation{
				{ID: "surveys", Name: "Surveys"},
			},
		},
	})

	result := engine.Recommend(Input{
		ForStep:         2,
		EvaluationGoals: []string{"Usability & Playability"},
	})

	if len(result) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result))
	}
}

func TestEngineRecommendSortsByPriorityThenName(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep: 2,
			Recommendations: []Recommendation{
				{ID: "b", Name: "Beta", Priority: "Engagement"},
				{ID: "c", Name: "Charlie", Priority: "Recommended"},
				{ID: "a", Name: "Alpha", Priority: "Recommended"},
			},
		},
	})

	result := engine.Recommend(Input{
		ForStep: 2,
	})

	if len(result) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(result))
	}

	if result[0].Name != "Alpha" {
		t.Fatalf("expected Alpha first, got %q", result[0].Name)
	}

	if result[1].Name != "Charlie" {
		t.Fatalf("expected Charlie second, got %q", result[1].Name)
	}

	if result[2].Name != "Beta" {
		t.Fatalf("expected Beta third, got %q", result[2].Name)
	}
}

func TestEngineRecommendReturnsEmptyWhenNoMatch(t *testing.T) {
	engine := NewEngine([]Rule{
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Guidance & Feedback"},
			Recommendations: []Recommendation{
				{ID: "heuristic-evaluation", Name: "Heuristic evaluation"},
			},
		},
	})

	result := engine.Recommend(Input{
		ForStep:         2,
		EvaluationGoals: []string{"Usability & Playability"},
	})

	if len(result) != 0 {
		t.Fatalf("expected 0 recommendation, got %d", len(result))
	}
}
