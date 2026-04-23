package pdf

import (
	"testing"
	"time"
)

func TestFPDFGeneratorGenerate(t *testing.T) {
	generator := NewFPDFGenerator()

	data, err := generator.Generate(PlanData{
		Title:            "Test Plan",
		Date:             time.Now(),
		EvaluationGoals:  []string{"Usability & Playability"},
		ProjectType:      "Concept test",
		Participants:     "Limited set of participants",
		DevelopmentStage: "Concept idea",
		SelectedMethods: []MethodEntry{
			{
				Name:        "Surveys & Questionnaires",
				Description: "Collect structured user feedback",
				Priority:    "Recommended",
				Rationale:   "Useful for early validation",
			},
		},
		SelectedInstruments: []InstrumentEntry{
			{
				Name:        "USEQ-Like",
				Description: "Short usability questionnaire",
				Priority:    "Recommended",
				Rationale:   "Suitable for perceived usability",
			},
			{
				Name:        "SUS",
				Description: "System Usability Scale",
				Priority:    "Engagement",
				Rationale:   "Widely used benchmark",
			},
		},
		NextSteps: []string{"Prepare materials"},
		Notes:     "Review participant availability before scheduling.",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty pdf bytes")
	}
}
