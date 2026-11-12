package design

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// SectionCount is the number of design sections (Flow 2.0). They are traversed
// in order 1..SectionCount on the first pass.
const SectionCount = 6

const (
	SectionStatusNotStarted = "not_started"
	SectionStatusInProgress = "in_progress"
	SectionStatusComplete   = "complete"
)

const (
	ReportVersionStandard = "standard"
	ReportVersionEnhanced = "enhanced"
)

var ErrInvalidSectionNumber = errors.New("invalid section number")
var ErrInvalidSectionData = errors.New("invalid section data")
var ErrSectionLocked = errors.New("section locked")
var ErrDashboardLocked = errors.New("dashboard locked")
var ErrInvalidReportVersion = errors.New("invalid report version")

var sectionNames = map[int]string{
	1: "Context",
	2: "Experience Timeline",
	3: "Personalization & Dynamics",
	4: "Gameful Core",
	5: "Feedback & Evaluation",
	6: "Technical Requirements",
}

var sectionDescriptions = map[int]string{
	1: "The domain, the users, and the constraints that shape the project.",
	2: "How the experience evolves over time: onboarding, updates, retention, drop-out risks, goals, and emotional dynamics.",
	3: "User types, personalization, and how difficulty adapts over time.",
	4: "Game rules and loops, game elements and mechanics, progression, modality, and timing.",
	5: "How the system communicates with players and how its effect is evaluated.",
	6: "Platforms, technology stack, performance, data, integrations, and technical risks.",
}

func SectionName(number int) string {
	return sectionNames[number]
}

func SectionDescription(number int) string {
	return sectionDescriptions[number]
}

type SectionState struct {
	Content  json.RawMessage `json:"content,omitempty"`
	Complete bool            `json:"complete"`
	Visited  bool            `json:"visited"`
}

type Status struct {
	Spark         string                  `json:"spark,omitempty"`
	Cursor        int                     `json:"cursor"`
	FirstPassDone bool                    `json:"firstPassDone"`
	Sections      map[string]SectionState `json:"sections"`
	Reports       []Report                `json:"reports,omitempty"`
	// Prefill: progress and explanations of the AI prefill started by the Spark.
	Prefill *PrefillState `json:"prefill,omitempty"`
	// ReportJob: the last generation of a final document (see report_job.go).
	ReportJob *ReportJobState `json:"reportJob,omitempty"`
}

func NewInitialStatus() Status {
	return Status{
		Cursor:   0,
		Sections: map[string]SectionState{},
	}
}

func (s Status) Section(number int) SectionState {
	return s.Sections[SectionKey(number)]
}

func (s Status) HasContent() bool {
	for _, state := range s.Sections {
		if len(state.Content) > 0 {
			return true
		}
	}
	return false
}

func SectionKey(number int) string {
	return strconv.Itoa(number)
}

func SectionStatus(state SectionState) string {
	if len(state.Content) == 0 {
		return SectionStatusNotStarted
	}
	if state.Complete {
		return SectionStatusComplete
	}
	return SectionStatusInProgress
}

func SectionPercent(state SectionState) int {
	switch SectionStatus(state) {
	case SectionStatusComplete:
		return 100
	case SectionStatusInProgress:
		return 50
	default:
		return 0
	}
}

type DashboardSection struct {
	SectionNumber int    `json:"sectionNumber"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	Percent       int    `json:"percent"`
}

type Dashboard struct {
	Sections       []DashboardSection `json:"sections"`
	OverallPercent int                `json:"overallPercent"`
	FirstPassDone  bool               `json:"firstPassDone"`
}

type Report struct {
	ID        string    `json:"reportId"`
	ProjectID string    `json:"-"`
	Version   string    `json:"version"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	// Missing: the file is no longer in the object store (e.g. lost with the local disk
	// of an earlier deploy). Set when listing, never stored.
	Missing bool `json:"missing,omitempty"`
}
