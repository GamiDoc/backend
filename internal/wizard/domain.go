package wizard

import "encoding/json"

type Status struct {
	CurrentStep int                        `json:"currentStep"`
	IsComplete  bool                       `json:"isComplete"`
	Steps       map[string]json.RawMessage `json:"steps"`
	// AIFill: the last AI fill of the evaluation from the design (projects only).
	AIFill *AIFill `json:"aiFill,omitempty"`
}

func NewInitialStatus() Status {
	return Status{
		CurrentStep: 1,
		IsComplete:  false,
		Steps:       map[string]json.RawMessage{},
	}
}
