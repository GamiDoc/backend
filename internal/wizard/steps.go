package wizard

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrIncompleteWizard = errors.New("incomplete wizard")
var ErrStepPrerequisiteNotMet = errors.New("step prerequisite not met")

type Step1Data struct {
	EvaluationGoals []string `json:"evaluationGoals"`
}

type Step2Data struct {
	SelectedMethods []string `json:"selectedMethods"`
}

type Step3Data struct {
	SelectedInstruments []string `json:"selectedInstruments"`
}

type Step4Data struct {
	NextSteps []string `json:"nextSteps"`
}

func ValidateStep(stepNumber int, stepData json.RawMessage) error {
	if len(stepData) == 0 || !json.Valid(stepData) {
		return ErrInvalidStepData
	}

	switch stepNumber {
	case 1:
		var data Step1Data
		if err := json.Unmarshal(stepData, &data); err != nil {
			return ErrInvalidStepData
		}
		if !hasNonEmpty(data.EvaluationGoals) {
			return ErrInvalidStepData
		}
		return nil
	case 2:
		var data Step2Data
		if err := json.Unmarshal(stepData, &data); err != nil {
			return ErrInvalidStepData
		}
		if !hasNonEmpty(data.SelectedMethods) {
			return ErrInvalidStepData
		}
		return nil
	case 3:
		var data Step3Data
		if err := json.Unmarshal(stepData, &data); err != nil {
			return ErrInvalidStepData
		}
		if !hasNonEmpty(data.SelectedInstruments) {
			return ErrInvalidStepData
		}
		return nil
	case 4:
		var data Step4Data
		if err := json.Unmarshal(stepData, &data); err != nil {
			return ErrInvalidStepData
		}
		if !hasNonEmpty(data.NextSteps) {
			return ErrInvalidStepData
		}
		return nil
	default:
		return ErrInvalidStepNumber
	}
}

func ValidateComplete(status Status) error {
	for step := 1; step <= 4; step++ {
		raw, ok := status.Steps[stepKey(step)]
		if !ok {
			return ErrIncompleteWizard
		}
		if err := ValidateStep(step, raw); err != nil {
			return ErrIncompleteWizard
		}
	}
	return nil
}

func DecodeStep1(status Status) (Step1Data, bool) {
	raw, ok := status.Steps["1"]
	if !ok {
		return Step1Data{}, false
	}
	var data Step1Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return Step1Data{}, false
	}
	if !hasNonEmpty(data.EvaluationGoals) {
		return Step1Data{}, false
	}
	return data, true
}

func DecodeStep2(status Status) (Step2Data, bool) {
	raw, ok := status.Steps["2"]
	if !ok {
		return Step2Data{}, false
	}
	var data Step2Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return Step2Data{}, false
	}
	if !hasNonEmpty(data.SelectedMethods) {
		return Step2Data{}, false
	}
	return data, true
}

func DecodeStep3(status Status) (Step3Data, bool) {
	raw, ok := status.Steps["3"]
	if !ok {
		return Step3Data{}, false
	}
	var data Step3Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return Step3Data{}, false
	}
	if !hasNonEmpty(data.SelectedInstruments) {
		return Step3Data{}, false
	}
	return data, true
}

func DecodeStep4(status Status) (Step4Data, bool) {
	raw, ok := status.Steps["4"]
	if !ok {
		return Step4Data{}, false
	}
	var data Step4Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return Step4Data{}, false
	}
	if !hasNonEmpty(data.NextSteps) {
		return Step4Data{}, false
	}
	return data, true
}

func hasNonEmpty(items []string) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
}
