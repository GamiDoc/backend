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
				"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`),
				"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
				"3": json.RawMessage(`{"selectedInstruments":["USEQ-Like","SUS"]}`),
				"4": json.RawMessage(`{"nextSteps":["Prepare materials","Run evaluation"],"notes":"Schedule a pilot session first."}`),
			},
		},
	}

	methodRecs := []recommendation.Recommendation{
		{
			ID:          "surveys",
			Name:        "Surveys & Questionnaires",
			Description: "Collect structured user feedback",
			Priority:    "Recommended",
			Rationale:   "Useful for measuring perceived usability",
		},
	}

	instrumentRecs := []recommendation.Recommendation{
		{
			ID:          "useq-like",
			Name:        "USEQ-Like",
			Description: "Short usability questionnaire",
			Priority:    "Recommended",
			Rationale:   "Suitable for usability evaluation",
		},
		{
			ID:          "sus",
			Name:        "SUS",
			Description: "System Usability Scale",
			Priority:    "Engagement",
			Rationale:   "Widely used benchmark",
		},
	}

	data, err := builder.BuildFromProject(item, methodRecs, instrumentRecs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data.Title != "My Project" {
		t.Fatalf("expected title %q, got %q", "My Project", data.Title)
	}

	if data.ProjectType != "Concept test" {
		t.Fatalf("expected project type %q, got %q", "Concept test", data.ProjectType)
	}

	if data.Participants != "Limited set of participants" {
		t.Fatalf("expected participants %q, got %q", "Limited set of participants", data.Participants)
	}

	if data.DevelopmentStage != "Concept idea" {
		t.Fatalf("expected development stage %q, got %q", "Concept idea", data.DevelopmentStage)
	}

	if len(data.SelectedMethods) != 1 {
		t.Fatalf("expected 1 selected method, got %d", len(data.SelectedMethods))
	}

	if len(data.SelectedInstruments) != 2 {
		t.Fatalf("expected 2 instruments, got %d", len(data.SelectedInstruments))
	}

	if len(data.NextSteps) != 2 {
		t.Fatalf("expected 2 next steps, got %d", len(data.NextSteps))
	}

	if data.Notes != "Schedule a pilot session first." {
		t.Fatalf("expected notes to be set, got %q", data.Notes)
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
				"1": json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`),
				"2": json.RawMessage(`{"selectedMethods":["surveys"]}`),
			},
		},
	}

	methodRecs := []recommendation.Recommendation{
		{
			ID:          "surveys",
			Name:        "Surveys & Questionnaires",
			Description: "Collect structured user feedback",
			Priority:    "Recommended",
			Rationale:   "Useful for measuring perceived usability",
		},
	}

	instrumentRecs := []recommendation.Recommendation{
		{ID: "useq-like", Name: "USEQ-Like"},
		{ID: "sus", Name: "SUS"},
	}

	data, err := builder.BuildFromSession(item, methodRecs, instrumentRecs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data.Title != "Anonymous Evaluation Plan" {
		t.Fatalf("expected title %q, got %q", "Anonymous Evaluation Plan", data.Title)
	}

	if len(data.SelectedMethods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(data.SelectedMethods))
	}

	if len(data.SelectedInstruments) != 2 {
		t.Fatalf("expected 2 instruments, got %d", len(data.SelectedInstruments))
	}
}
