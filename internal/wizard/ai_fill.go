package wizard

import "time"

// AI fill states (same values as the design AI prefill).
const (
	AIFillRunning = "running"
	AIFillDone    = "done"
	AIFillFailed  = "failed"
)

// AIFill is the last AI fill of the evaluation from the project's design: its progress
// and, once done, why the AI chose each value (shown next to the choices in the wizard).
type AIFill struct {
	JobID      string     `json:"jobId"`
	State      string     `json:"state"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// Summary: what the AI understood of the design and how it approached the evaluation.
	Summary string `json:"summary,omitempty"`
	// Notes by step ("1".."4").
	Notes map[string][]AINote `json:"notes,omitempty"`
}

// AINote explains one choice: Field is the step field ("evaluationGoals",
// "developmentStage", "selectedMethods"...), Value the option chosen ("" for the field
// as a whole).
type AINote struct {
	Field  string `json:"field"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason"`
}
