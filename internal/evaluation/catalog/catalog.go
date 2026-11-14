// Package catalog is the UX Evaluation catalog: evaluation goals, development stages,
// constraints, methods and instruments, exported from the frontend
// (frontend_new: npm run export-evaluation-catalog → catalog.json).
//
// The AI that fills in the evaluation must pick from these lists, and the reasoned
// evaluation report uses their descriptions.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode"
)

//go:embed catalog.json
var catalogJSON []byte

type Goal struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type Stage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Method struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Stages      []string `json:"stages"`
	Goals       []string `json:"goals"`
}

type Instrument struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Attributes   []string `json:"attributes"`
	Stages       []string `json:"stages"`
	Constraints  []string `json:"constraints"`
	ForMethodIDs []string `json:"forMethodIds"`
	CoversGoals  []string `json:"coversGoals"`
}

type Catalog struct {
	Goals            []Goal       `json:"goals"`
	Stages           []Stage      `json:"stages"`
	Participants     []string     `json:"participants"`
	Accessibility    []string     `json:"accessibility"`
	Time             []string     `json:"time"`
	ExtraConstraints []string     `json:"extraConstraints"`
	Methods          []Method     `json:"methods"`
	Instruments      []Instrument `json:"instruments"`
}

var (
	loadOnce sync.Once
	loaded   Catalog
	loadErr  error
)

// Get returns the embedded catalog (parsed once).
func Get() Catalog {
	loadOnce.Do(func() {
		loadErr = json.Unmarshal(catalogJSON, &loaded)
	})
	if loadErr != nil {
		panic(fmt.Sprintf("evaluation catalog: %v", loadErr))
	}
	return loaded
}

// key compares names ignoring case, spaces and punctuation ("Think-aloud testing" ==
// "think aloud testing").
func key(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Match returns the option of list that name refers to, as written in the catalog.
func Match(list []string, name string) (string, bool) {
	wanted := key(name)
	if wanted == "" {
		return "", false
	}
	for _, option := range list {
		if key(option) == wanted {
			return option, true
		}
	}
	return "", false
}

func (c Catalog) GoalNames() []string {
	names := make([]string, len(c.Goals))
	for i, goal := range c.Goals {
		names[i] = goal.Name
	}
	return names
}

func (c Catalog) StageNames() []string {
	names := make([]string, len(c.Stages))
	for i, stage := range c.Stages {
		names[i] = stage.Name
	}
	return names
}

func (c Catalog) MethodNames() []string {
	names := make([]string, len(c.Methods))
	for i, method := range c.Methods {
		names[i] = method.Name
	}
	return names
}

func (c Catalog) InstrumentNames() []string {
	names := make([]string, len(c.Instruments))
	for i, instrument := range c.Instruments {
		names[i] = instrument.Name
	}
	return names
}

func (c Catalog) Goal(name string) (Goal, bool) {
	for _, goal := range c.Goals {
		if key(goal.Name) == key(name) {
			return goal, true
		}
	}
	return Goal{}, false
}

func (c Catalog) Stage(name string) (Stage, bool) {
	for _, stage := range c.Stages {
		if key(stage.Name) == key(name) {
			return stage, true
		}
	}
	return Stage{}, false
}

func (c Catalog) Method(name string) (Method, bool) {
	for _, method := range c.Methods {
		if key(method.Name) == key(name) || method.ID == name {
			return method, true
		}
	}
	return Method{}, false
}

func (c Catalog) Instrument(name string) (Instrument, bool) {
	for _, instrument := range c.Instruments {
		if key(instrument.Name) == key(name) || instrument.ID == name {
			return instrument, true
		}
	}
	return Instrument{}, false
}

// Text describes the catalog for an AI prompt: every option with its description.
func (c Catalog) Text() string {
	var b strings.Builder
	b.WriteString("Evaluation goals (pick by exact name):\n")
	for _, goal := range c.Goals {
		fmt.Fprintf(&b, "- %s [%s]: %s\n", goal.Name, goal.Category, goal.Description)
	}
	b.WriteString("\nDevelopment stages:\n")
	for _, stage := range c.Stages {
		fmt.Fprintf(&b, "- %s: %s\n", stage.Name, stage.Description)
	}
	fmt.Fprintf(&b, "\nParticipants (sample size): %s\n", strings.Join(c.Participants, " | "))
	fmt.Fprintf(&b, "Participant access: %s\n", strings.Join(c.Accessibility, " | "))
	fmt.Fprintf(&b, "Study duration: %s\n", strings.Join(c.Time, " | "))
	fmt.Fprintf(&b, "Other constraints: %s\n", strings.Join(c.ExtraConstraints, " | "))
	b.WriteString("\nMethods:\n")
	for _, method := range c.Methods {
		fmt.Fprintf(&b, "- %s (%s): %s Stages: %s. Goals: %s.\n", method.Name, method.Type, method.Description,
			strings.Join(method.Stages, ", "), strings.Join(method.Goals, ", "))
	}
	b.WriteString("\nInstruments:\n")
	for _, instrument := range c.Instruments {
		var methods []string
		for _, id := range instrument.ForMethodIDs {
			if method, ok := c.Method(id); ok {
				methods = append(methods, method.Name)
			}
		}
		fmt.Fprintf(&b, "- %s (%s): %s For methods: %s. Covers goals: %s. Stages: %s.\n", instrument.Name, instrument.Type,
			instrument.Description, strings.Join(methods, ", "), strings.Join(instrument.CoversGoals, ", "),
			strings.Join(instrument.Stages, ", "))
	}
	return b.String()
}
