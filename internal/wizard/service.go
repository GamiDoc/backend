package wizard

import (
	"encoding/json"
	"errors"
	"strconv"
)

var ErrInvalidStepNumber = errors.New("invalid step number")
var ErrInvalidStepData = errors.New("invalid step data")

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SaveStep(current Status, stepNumber int, stepData json.RawMessage) (Status, error) {
	if stepNumber < 1 || stepNumber > 4 {
		return Status{}, ErrInvalidStepNumber
	}

	if err := ValidateStep(stepNumber, stepData); err != nil {
		return Status{}, err
	}

	if current.Steps == nil {
		current.Steps = map[string]json.RawMessage{}
	}

	if err := s.validatePrerequisites(current, stepNumber); err != nil {
		return Status{}, err
	}

	current.Steps[stepKey(stepNumber)] = stepData

	for step := stepNumber + 1; step <= 4; step++ {
		delete(current.Steps, stepKey(step))
	}

	current.CurrentStep = s.computeCurrentStep(current)
	current.IsComplete = s.computeIsComplete(current)

	return current, nil
}

func (s *Service) validatePrerequisites(status Status, stepNumber int) error {
	for step := 1; step < stepNumber; step++ {
		raw, ok := status.Steps[stepKey(step)]
		if !ok {
			return ErrStepPrerequisiteNotMet
		}
		if err := ValidateStep(step, raw); err != nil {
			return ErrStepPrerequisiteNotMet
		}
	}
	return nil
}

func (s *Service) computeCurrentStep(status Status) int {
	for step := 1; step <= 4; step++ {
		raw, ok := status.Steps[stepKey(step)]
		if !ok {
			return step
		}
		if err := ValidateStep(step, raw); err != nil {
			return step
		}
	}
	return 4
}

func (s *Service) computeIsComplete(status Status) bool {
	return ValidateComplete(status) == nil
}

func stepKey(step int) string {
	return strconv.Itoa(step)
}
