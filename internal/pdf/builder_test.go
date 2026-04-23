package pdf

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

func TestBuilderBuildFromProjectUsesWizardStepData(t *testing.T) {
	builder := NewBuilder()

	item := project.Project{
		ID:        "project-1",
		Name:      "My Project",
		CreatedAt: time.Now(),
		Wizard: wizard.Status{
			CurrentStep: 4,
			IsComplete:  true,
			Steps: map[string]json.RawMessage{
				"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"]}`),
				"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
				"3": json.RawMessage(`{"selectedInstruments":["USEQ-Like","SUS"]}`),
				"4": json.RawMessage(`{"nextSteps":["Prepare materials","Run evaluation"]}`),
			},
		},
	}

	data, err := builder.BuildFromProject(item, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data.Title != "My Project" {
		t.Fatalf("expected title %q, got %q", "My Project", data.Title)
	}

	if len(data.EvaluationGoals) != 1 {
		t.Fatalf("expected 1 evaluation goal, got %d", len(data.EvaluationGoals))
	}

	if len(data.SelectedMethods) != 1 {
		t.Fatalf("expected 1 selected method, got %d", len(data.SelectedMethods))
	}

	if len(data.RecommendedInstruments) != 2 {
		t.Fatalf("expected 2 instruments, got %d", len(data.RecommendedInstruments))
	}

	if len(data.NextSteps) != 2 {
		t.Fatalf("expected 2 next steps, got %d", len(data.NextSteps))
	}
}

func TestBuilderBuildFromSessionFallsBackToRecommendations(t *testing.T) {
	builder := NewBuilder()

	item := session.Session{
		ID:        "session-1",
		CreatedAt: time.Now(),
		Wizard: wizard.Status{
			CurrentStep: 3,
			IsComplete:  false,
			Steps: map[string]json.RawMessage{
				"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"]}`),
				"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
			},
		},
	}

	data, err := builder.BuildFromSession(item, []recommendation.Recommendation{
		{ID: "useq-like", Name: "USEQ-Like"},
		{ID: "sus", Name: "SUS"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data.Title != "Anonymous Evaluation Plan" {
		t.Fatalf("expected title %q, got %q", "Anonymous Evaluation Plan", data.Title)
	}

	if len(data.RecommendedInstruments) != 2 {
		t.Fatalf("expected 2 instruments, got %d", len(data.RecommendedInstruments))
	}
}
