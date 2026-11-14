package design

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/evaluation/catalog"
	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/wizard"
	"github.com/google/uuid"
)

/* AI fill of the UX Evaluation from the design.

   POST /projects/{id}/design/evaluation/ai-fill: once the design has had a first pass, the
   AI reads it and fills in the four steps of the evaluation wizard (goals, stage,
   constraints, research questions, methods, instruments, next steps), picking only from
   the evaluation catalog, and explains each choice. It runs in the background like the
   design prefill: the progress and the explanations are in the project's
   wizardStatus.aiFill (GET /projects/{id}); the steps are saved when it is done. */

// EvaluationFillTimeout bounds one AI fill of the evaluation (one model call).
const EvaluationFillTimeout = 10 * time.Minute

// WizardStore saves the evaluation wizard of a project.
type WizardStore interface {
	FindByID(ctx context.Context, id string) (project.Project, error)
	UpdateWizard(ctx context.Context, projectID string, status wizard.Status) (project.Project, error)
}

var errEvaluationFillUnavailable = errors.New("evaluation ai fill not configured")

type evaluationFiller struct {
	assistant ai.Assistant
	store     WizardStore
	wizard    *wizard.Service
	logger    *slog.Logger
	timeout   time.Duration

	mu      sync.Mutex
	running map[string]runningJob // by project id
}

// WithEvaluationStore enables the AI fill of the evaluation (needs the projects' wizard).
func (h *Handler) WithEvaluationStore(store WizardStore) *Handler {
	h.evaluation = &evaluationFiller{
		assistant: h.assistant,
		store:     store,
		wizard:    wizard.NewService(),
		logger:    h.prefill.logger,
		timeout:   EvaluationFillTimeout,
		running:   map[string]runningJob{},
	}
	return h
}

func (f *evaluationFiller) isRunning(projectID, jobID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	job, ok := f.running[projectID]
	return ok && job.id == jobID
}

// fillEvaluation starts the AI fill of the evaluation of a project.
func (h *Handler) fillEvaluation(w http.ResponseWriter, r *http.Request, o owner) {
	f := h.evaluation
	if f == nil || !ai.SupportsJSON(h.assistant) {
		response.WriteError(w, http.StatusServiceUnavailable, "AI_UNAVAILABLE", "The AI is not available", nil)
		return
	}

	unlock := h.lock(o)
	defer unlock()

	design, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	if !design.FirstPassDone || !design.HasContent() {
		response.WriteError(w, http.StatusConflict, "DESIGN_NOT_READY", "Go through all the design sections first", nil)
		return
	}
	found, err := f.store.FindByID(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	if current := found.Wizard.AIFill; current != nil && current.State == wizard.AIFillRunning && f.isRunning(o.id, current.JobID) {
		response.WriteError(w, http.StatusConflict, "AI_FILL_RUNNING", "The AI is already filling in the evaluation", nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	job := &wizard.AIFill{JobID: uuid.NewString(), State: wizard.AIFillRunning, StartedAt: time.Now().UTC()}
	f.mu.Lock()
	if previous, ok := f.running[o.id]; ok {
		previous.cancel()
	}
	f.running[o.id] = runningJob{id: job.JobID, cancel: cancel}
	f.mu.Unlock()

	status := found.Wizard
	status.AIFill = job
	saved, err := f.store.UpdateWizard(r.Context(), o.id, status)
	if err != nil {
		f.release(o.id, job.JobID)
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	go f.run(ctx, o.id, found.Name, *job, design)

	response.WriteJSON(w, http.StatusAccepted, map[string]any{"wizardStatus": saved.Wizard})
}

func (f *evaluationFiller) release(projectID, jobID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if job, ok := f.running[projectID]; ok && job.id == jobID {
		job.cancel()
		delete(f.running, projectID)
	}
}

// run asks the AI for the evaluation, saves the four steps and the explanations.
func (f *evaluationFiller) run(ctx context.Context, projectID, projectName string, job wizard.AIFill, design Status) {
	defer f.release(projectID, job.JobID)

	answer, err := f.ask(ctx, design)
	var steps [4]json.RawMessage
	var notes map[string][]wizard.AINote
	if err == nil {
		steps, notes, err = answer.toSteps(projectName)
	}

	saveCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	found, getErr := f.store.FindByID(saveCtx, projectID)
	if getErr != nil || found.Wizard.AIFill == nil || found.Wizard.AIFill.JobID != job.JobID {
		return // replaced by a newer request, or the project is gone
	}
	status := found.Wizard
	now := time.Now().UTC()
	job.FinishedAt = &now
	if err == nil {
		for number, data := range steps {
			if status, err = f.wizard.SaveStep(status, number+1, data); err != nil {
				err = fmt.Errorf("step %d: %w", number+1, err)
				break
			}
		}
	}
	if err != nil {
		f.logger.Error("evaluation ai fill failed", "project", projectID, "error", err)
		status = found.Wizard // keep the steps as they were
		job.State = wizard.AIFillFailed
		job.Error = "the AI could not fill in the evaluation"
	} else {
		job.State = wizard.AIFillDone
		job.Summary = strings.TrimSpace(answer.Summary)
		job.Notes = notes
	}
	status.AIFill = &job
	if _, err := f.store.UpdateWizard(saveCtx, projectID, status); err != nil {
		f.logger.Error("evaluation ai fill: save failed", "project", projectID, "error", err)
	}
}

// ── The model's answer ───────────────────────────────────────────────────────

type evalChoice struct {
	Name   string `json:"name" desc:"the exact name from the catalog"`
	Reason string `json:"reason" desc:"1-2 sentences: why it fits THIS design (mention the design choices it relates to)"`
}

type evalValue struct {
	Value  string `json:"value" desc:"the exact option from the catalog"`
	Reason string `json:"reason" desc:"1-2 sentences: why, from the design"`
}

type evalResearch struct {
	Objective  string   `json:"objective" desc:"the research objective, 1 sentence; empty if a formal research specification is not useful"`
	Questions  []string `json:"questions" desc:"2-4 research questions (RQ), each answerable by the chosen methods"`
	Hypotheses []string `json:"hypotheses" desc:"testable hypotheses for the questions that have one, with a threshold when an instrument allows it (e.g. 'UMUX-Lite score of at least 68'); empty for exploratory questions"`
	Reason     string   `json:"reason" desc:"1-2 sentences: what the questions focus on and why"`
}

type evalFillAnswer struct {
	Summary          string       `json:"summary" desc:"2-3 sentences: what the design is and what the evaluation must find out, given its riskiest choices"`
	Description      string       `json:"description" desc:"1-2 sentences describing the gamified system to evaluate, for the 'Project description' field"`
	Goals            []evalChoice `json:"goals" desc:"3-5 evaluation goals, exact names from the catalog; include the goals that test the design's riskiest choices"`
	Stage            evalValue    `json:"stage" desc:"the development stage, inferred from the design (default 'High-fidelity prototype' when unclear)"`
	Participants     evalValue    `json:"participants" desc:"the sample size option"`
	Access           evalValue    `json:"access" desc:"how easy it is to reach the target users"`
	Duration         evalValue    `json:"duration" desc:"the study duration option; long enough to see the behaviour the design wants to change"`
	OtherConstraints []evalChoice `json:"otherConstraints" desc:"other constraints from the catalog that apply (e.g. Sensitive population for minors or patients); empty if none"`
	Research         evalResearch `json:"research"`
	Methods          []evalChoice `json:"methods" desc:"2-4 methods that together cover the goals and fit the stage, sample and duration"`
	Instruments      []evalChoice `json:"instruments" desc:"3-6 instruments for the chosen methods that cover every chosen goal"`
	NextSteps        []string     `json:"nextSteps" desc:"3-5 concrete next steps to prepare and run the study"`
}

const evaluationFillSystemPrompt = `You are a UX researcher planning the evaluation of a gamified system designed in GamiDoc.
You receive the design (made in GamiDoc's Design System) and the catalog of the evaluation wizard: evaluation goals, development stages, constraints, methods and instruments.
Fill in the evaluation the way an expert would for THIS design:
- Pick options ONLY from the catalog, writing their names exactly as listed.
- Choose goals that test what the design is betting on: its motivational strategy, its riskiest mechanics (e.g. competition, extrinsic rewards), the behaviour it wants to change.
- Choose methods that fit the development stage, the sample and the time; choose instruments that belong to the chosen methods and together cover every chosen goal.
- Ground every reason in the design data: name the design choices it relates to. Never invent facts about the project.
- Write the reasons, questions, hypotheses and next steps in the language of the design data; keep the catalog names in English as listed.`

func (f *evaluationFiller) ask(ctx context.Context, design Status) (evalFillAnswer, error) {
	schema, err := json.Marshal(evaluationFillSchema())
	if err != nil {
		return evalFillAnswer{}, err
	}
	user := designDataText(design) + "\n\n## Evaluation catalog\n" + catalog.Get().Text()
	system := evaluationFillSystemPrompt
	if language := documentLanguage(design); language != "" {
		rule := "LANGUAGE: write reasons, questions, hypotheses and next steps in " + language + "; catalog names stay as listed."
		system = rule + "\n\n" + system
		user += "\n\n" + rule
	}
	raw, err := f.assistant.CompleteJSON(ctx, ai.JSONRequest{
		Name:   "evaluation_fill",
		System: system,
		User:   user,
		Schema: schema,
	})
	if err != nil {
		return evalFillAnswer{}, err
	}
	var answer evalFillAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return evalFillAnswer{}, fmt.Errorf("unparseable answer: %w", err)
	}
	return answer, nil
}

// evaluationFillSchema is the answer schema with the catalog options as enums, so a model
// with structured outputs can only pick valid options.
func evaluationFillSchema() jsonSchema {
	c := catalog.Get()
	schema := docSchema(evalFillAnswer{})
	properties := schema["properties"].(jsonSchema)
	setEnum := func(field, sub string, options []string, array bool) {
		target := properties[field].(jsonSchema)
		if array {
			target = target["items"].(jsonSchema)
		}
		target["properties"].(jsonSchema)[sub].(jsonSchema)["enum"] = options
	}
	setEnum("goals", "name", c.GoalNames(), true)
	setEnum("stage", "value", c.StageNames(), false)
	setEnum("participants", "value", c.Participants, false)
	setEnum("access", "value", c.Accessibility, false)
	setEnum("duration", "value", c.Time, false)
	setEnum("otherConstraints", "name", c.ExtraConstraints, true)
	setEnum("methods", "name", c.MethodNames(), true)
	setEnum("instruments", "name", c.InstrumentNames(), true)
	return schema
}

// defaultNextSteps are used when the AI gives none (same as the wizard's defaults).
var defaultNextSteps = []string{
	"Conduct a pilot test to identify potential issues and refine instructions.",
	"Coordinate with participants and schedule your evaluation sessions.",
	"Observe user interactions and collect data.",
}

// toSteps turns the answer into the four wizard steps (only catalog options are kept)
// and the explanations by step.
func (a evalFillAnswer) toSteps(projectName string) ([4]json.RawMessage, map[string][]wizard.AINote, error) {
	c := catalog.Get()
	notes := map[string][]wizard.AINote{}
	note := func(step, field, value, reason string) {
		if reason = strings.TrimSpace(reason); reason != "" {
			notes[step] = append(notes[step], wizard.AINote{Field: field, Value: value, Reason: reason})
		}
	}
	pickAll := func(step, field string, choices []evalChoice, options []string) []string {
		var picked []string
		seen := map[string]bool{}
		for _, choice := range choices {
			name, ok := catalog.MatchLoose(options, choice.Name)
			if !ok || seen[name] {
				continue
			}
			seen[name] = true
			picked = append(picked, name)
			note(step, field, name, choice.Reason)
		}
		return picked
	}
	pickOne := func(step, field string, value evalValue, options []string) string {
		name, ok := catalog.MatchLoose(options, value.Value)
		if !ok {
			return ""
		}
		note(step, field, name, value.Reason)
		return name
	}

	goals := pickAll("1", "evaluationGoals", a.Goals, c.GoalNames())
	stage := pickOne("1", "developmentStage", a.Stage, c.StageNames())
	participants := pickOne("1", "participants", a.Participants, c.Participants)
	access := pickOne("1", "accessibility", a.Access, c.Accessibility)
	duration := pickOne("1", "time", a.Duration, c.Time)
	others := pickAll("1", "extraConstraints", a.OtherConstraints, c.ExtraConstraints)
	methods := pickAll("2", "selectedMethods", a.Methods, c.MethodNames())
	instruments := pickAll("3", "selectedInstruments", a.Instruments, c.InstrumentNames())

	// Required by the wizard: a missing stage or sample gets a default the user is told to check.
	if stage == "" {
		stage = "High-fidelity prototype"
		note("1", "developmentStage", stage, "Default choice: the design does not say how far the system is built. Check it.")
	}
	if participants == "" {
		participants = "10-30"
		note("1", "participants", participants, "Default choice: the design does not say how many people can take part. Check it.")
	}
	switch {
	case len(goals) == 0:
		return [4]json.RawMessage{}, nil, fmt.Errorf("no valid evaluation goal in %q", choiceNames(a.Goals))
	case len(methods) == 0 || len(instruments) == 0:
		return [4]json.RawMessage{}, nil, fmt.Errorf("no valid method or instrument in %q / %q", choiceNames(a.Methods), choiceNames(a.Instruments))
	}

	questions := nonEmptyItems(a.Research.Questions)
	hypotheses := nonEmptyItems(a.Research.Hypotheses)
	research := len(questions) > 0
	if research {
		note("1", "research", "", a.Research.Reason)
	}
	nextSteps := nonEmptyItems(a.NextSteps)
	if len(nextSteps) == 0 {
		nextSteps = defaultNextSteps
	}

	step1 := map[string]any{
		"projectName":       projectName,
		"projectType":       strings.TrimSpace(a.Description),
		"evaluationGoals":   goals,
		"participants":      participants,
		"developmentStage":  stage,
		"accessibility":     access,
		"time":              duration,
		"extraConstraints":  emptyIfNil(others),
		"researchEnabled":   research,
		"researchObjective": strings.TrimSpace(a.Research.Objective),
		"researchQuestions": emptyIfNil(questions),
		"hypotheses":        emptyIfNil(hypotheses),
	}
	var steps [4]json.RawMessage
	for i, data := range []any{
		step1,
		map[string]any{"selectedMethods": methods},
		map[string]any{"selectedInstruments": instruments, "customConstructs": []any{}},
		map[string]any{"nextSteps": nextSteps, "notes": ""},
	} {
		encoded, err := json.Marshal(data)
		if err != nil {
			return [4]json.RawMessage{}, nil, err
		}
		steps[i] = encoded
	}
	return steps, notes, nil
}

func nonEmptyItems(list []string) []string {
	var out []string
	for _, item := range list {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func emptyIfNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func choiceNames(choices []evalChoice) []string {
	names := make([]string, len(choices))
	for i, choice := range choices {
		names[i] = choice.Name
	}
	return names
}
