package pdf

import "time"

type MethodEntry struct {
	Name        string
	Description string
	Priority    string
	Rationale   string
}

type InstrumentEntry struct {
	Name        string
	Description string
	Priority    string
	Rationale   string
}

type PlanData struct {
	Title               string
	Date                time.Time
	EvaluationGoals     []string
	ProjectType         string
	Participants        string
	DevelopmentStage    string
	SelectedMethods     []MethodEntry
	SelectedInstruments []InstrumentEntry
	NextSteps           []string
	Notes               string
}

type Generated struct {
	Key string
	URL string
}
