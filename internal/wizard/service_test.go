package wizard

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSaveStepRejectsInvalidStepNumber(t *testing.T) {
	service := NewService()

	_, err := service.SaveStep(NewInitialStatus(), 0, json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidStepNumber) {
		t.Fatalf("expected ErrInvalidStepNumber, got %v", err)
	}
}

func TestSaveStepRejectsInvalidData(t *testing.T) {
	service := NewService()

	_, err := service.SaveStep(NewInitialStatus(), 1, json.RawMessage(`{"evaluationGoals":[]}`))
	if !errors.Is(err, ErrInvalidStepData) {
		t.Fatalf("expected ErrInvalidStepData, got %v", err)
	}
}

func TestSaveStepRejectsMissingStep1Fields(t *testing.T) {
	service := NewService()

	_, err := service.SaveStep(NewInitialStatus(), 1, json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"","participants":"Limited set of participants","developmentStage":"Concept idea"}`))
	if !errors.Is(err, ErrInvalidStepData) {
		t.Fatalf("expected ErrInvalidStepData, got %v", err)
	}
}

func TestSaveStepRejectsSkippedPrerequisite(t *testing.T) {
	service := NewService()

	_, err := service.SaveStep(NewInitialStatus(), 2, json.RawMessage(`{"selectedMethods":["surveys"]}`))
	if !errors.Is(err, ErrStepPrerequisiteNotMet) {
		t.Fatalf("expected ErrStepPrerequisiteNotMet, got %v", err)
	}
}

func TestSaveStepClearsFollowingSteps(t *testing.T) {
	service := NewService()

	status := NewInitialStatus()

	var err error
	status, err = service.SaveStep(status, 1, json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 2, json.RawMessage(`{"selectedMethods":["surveys"]}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 3, json.RawMessage(`{"selectedInstruments":["USEQ-Like"]}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 4, json.RawMessage(`{"nextSteps":["Run evaluation"],"notes":"Pilot first."}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 2, json.RawMessage(`{"selectedMethods":["think-aloud"]}`))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := status.Steps["3"]; ok {
		t.Fatal("expected step 3 to be cleared")
	}

	if _, ok := status.Steps["4"]; ok {
		t.Fatal("expected step 4 to be cleared")
	}

	if status.CurrentStep != 3 {
		t.Fatalf("expected current step 3, got %d", status.CurrentStep)
	}

	if status.IsComplete {
		t.Fatal("expected wizard to be incomplete")
	}
}

func TestSaveStepComputesCompletion(t *testing.T) {
	service := NewService()

	status := NewInitialStatus()

	var err error
	status, err = service.SaveStep(status, 1, json.RawMessage(`{"evaluationGoals":["Usability & Playability"],"projectType":"Concept test","participants":"Limited set of participants","developmentStage":"Concept idea"}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 2, json.RawMessage(`{"selectedMethods":["surveys"]}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 3, json.RawMessage(`{"selectedInstruments":["USEQ-Like","SUS"]}`))
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveStep(status, 4, json.RawMessage(`{"nextSteps":["Prepare materials","Run evaluation"],"notes":"Pilot first."}`))
	if err != nil {
		t.Fatal(err)
	}

	if !status.IsComplete {
		t.Fatal("expected wizard to be complete")
	}
}
