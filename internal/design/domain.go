package design

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

const SectionCount = 7

const (
	PathExperienceFirst = "A"
	PathMechanicsFirst  = "B"
)

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
var ErrInvalidPath = errors.New("invalid path")
var ErrPathAlreadyChosen = errors.New("path already chosen")
var ErrPathNotChosen = errors.New("path not chosen")
var ErrSectionLocked = errors.New("section locked")
var ErrDashboardLocked = errors.New("dashboard locked")
var ErrInvalidReportVersion = errors.New("invalid report version")

var sectionNames = map[int]string{
	1: "Context",
	2: "Experience Timeline",
	3: "Personification & Dynamics",
	4: "Gameful Core",
	5: "Technology",
	6: "Impacts & Benefits",
	7: "Evaluation & Feedback",
}

var sectionDescriptions = map[int]string{
	1: "The domain, target audience, and problem the gamified system addresses.",
	2: "How the experience unfolds over time, from first contact to long-term use.",
	3: "Personas or player types and the social dynamics between them.",
	4: "The central game mechanics, rules, and reward structures.",
	5: "Platforms, devices, integrations, and technical constraints.",
	6: "The intended behavioural, learning, or business outcomes.",
	7: "How the system's effect is measured and how feedback reaches users.",
}

func SectionName(number int) string {
	return sectionNames[number]
}

func SectionDescription(number int) string {
	return sectionDescriptions[number]
}

func Order(path string) []int {
	switch path {
	case PathExperienceFirst:
		return []int{1, 2, 3, 4, 5, 6, 7}
	case PathMechanicsFirst:
		return []int{1, 4, 5, 2, 3, 6, 7}
	default:
		return nil
	}
}

type SectionState struct {
	Content  json.RawMessage `json:"content,omitempty"`
	Complete bool            `json:"complete"`
	Visited  bool            `json:"visited"`
}

type Status struct {
	Spark         string                  `json:"spark,omitempty"`
	Path          string                  `json:"path,omitempty"`
	Cursor        int                     `json:"cursor"`
	FirstPassDone bool                    `json:"firstPassDone"`
	Sections      map[string]SectionState `json:"sections"`
	Reports       []Report                `json:"reports,omitempty"`
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
	Path           string             `json:"path,omitempty"`
}

type Report struct {
	ID        string    `json:"reportId"`
	ProjectID string    `json:"-"`
	Version   string    `json:"version"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}
