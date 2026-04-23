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
