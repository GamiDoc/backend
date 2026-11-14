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
	"github.com/gamidoc/backend/internal/recommendation"
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

// EvaluationRecommender gives the methods (step 2) and instruments (step 3) the wizard
// recommends for a set-up (the rules in rule/recommendations.json).
type EvaluationRecommender interface {
	Recommend(status wizard.Status, forStep int) (recommendation.Result, error)
}

type evaluationFiller struct {
	assistant   ai.Assistant
	store       WizardStore
	recommender EvaluationRecommender // nil: no recommendations given to the AI
	wizard      *wizard.Service
	logger      *slog.Logger
	timeout     time.Duration

	mu      sync.Mutex
	running map[string]runningJob // by project id
}

// WithEvaluationStore enables the AI fill of the evaluation (needs the projects' wizard and
// the wizard's recommendations, which the AI considers first).
func (h *Handler) WithEvaluationStore(store WizardStore, recommender EvaluationRecommender) *Handler {
	h.evaluation = &evaluationFiller{
		assistant:   h.assistant,
		recommender: recommender,
		store:       store,
		wizard:      wizard.NewService(),
		logger:      h.prefill.logger,
		timeout:     EvaluationFillTimeout,
		running:     map[string]runningJob{},
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

// evalCustomInstrument is a custom questionnaire, as in the wizard's "Custom questionnaire"
// (Define Custom Evaluation Construct): a construct name, what it measures, the goals.
type evalCustomInstrument struct {
	Name        string   `json:"name" desc:"the construct the questionnaire measures, e.g. 'Perceived pressure from the department leaderboard'"`
	Description string   `json:"description" desc:"what you want to measure, specific and measurable, 1-2 sentences"`
	Goals       []string `json:"goals" desc:"the chosen goals it covers, exact names"`
	Reason      string   `json:"reason" desc:"1-2 sentences: why this project needs it"`
}

type evalResearch struct {
	Objective  string   `json:"objective" desc:"the research objective, 1 sentence; empty if a formal research specification is not useful"`
	Questions  []string `json:"questions" desc:"2-4 research questions (RQ), each answerable by the chosen methods"`
	Hypotheses []string `json:"hypotheses" desc:"testable hypotheses for the questions that have one, with a threshold when an instrument allows it (e.g. 'UMUX-Lite score of at least 68'); empty for exploratory questions"`
	Reason     string   `json:"reason" desc:"1-2 sentences: what the questions focus on and why"`
}

type evalFillAnswer struct {
	Summary           string                 `json:"summary" desc:"2-3 sentences addressed to the user: what this evaluation will find out about their system and how. Constructive; never about missing information, the tool or its options"`
	Description       string                 `json:"description" desc:"1-2 sentences describing the gamified system to evaluate, for the 'Project description' field"`
	Goals             []evalChoice           `json:"goals" desc:"the evaluation goals that matter for this design (usually 3-5), exact names from the catalog; include the goals that test the design's riskiest choices"`
	Stage             evalValue              `json:"stage" desc:"the development stage, inferred from the design (default 'High-fidelity prototype' when unclear)"`
	Participants      evalValue              `json:"participants" desc:"the sample size option"`
	Access            evalValue              `json:"access" desc:"how easy it is to reach the target users"`
	Duration          evalValue              `json:"duration" desc:"the study duration option; long enough to see the behaviour the design wants to change"`
	OtherConstraints  []evalChoice           `json:"otherConstraints" desc:"other constraints from the catalog that apply (e.g. Sensitive population for minors or patients); empty if none"`
	Research          evalResearch           `json:"research"`
	Methods           []evalChoice           `json:"methods" desc:"every method the study needs (usually 2-4), from the whole catalog, that together cover the goals and fit the stage, sample and duration"`
	Instruments       []evalChoice           `json:"instruments" desc:"every instrument the study needs, from the whole catalog, for the chosen methods, so that every chosen goal is covered"`
	CustomInstruments []evalCustomInstrument `json:"customInstruments" desc:"custom questionnaires (one construct each), ONLY for chosen goals that no chosen catalog instrument covers; empty otherwise"`
	NextSteps         []string               `json:"nextSteps" desc:"3-5 concrete next steps to prepare and run the study"`
}

const evaluationFillSystemPrompt = `You are a UX researcher planning the evaluation of a gamified system designed in GamiDoc.
You receive the design (made in GamiDoc's Design System) and the catalog of the evaluation wizard: evaluation goals, development stages, constraints, methods and instruments.
Fill in the evaluation the way an expert would for THIS design. Always produce a complete, usable evaluation:
- Pick options ONLY from the catalog, writing their names exactly as listed.
- Choose goals that test what the design is betting on: its motivational strategy, its riskiest mechanics (e.g. competition, extrinsic rewards), the behaviour it wants to change.
- Choose methods that fit the development stage, the sample and the time; choose instruments that belong to the chosen methods and together cover every chosen goal. When a chosen goal is not covered by any catalog instrument, add a custom questionnaire for it (a construct to measure, as in the wizard's "Custom questionnaire").
- Ground every reason in the design data and name the design choices it relates to. When the design gives little detail, make the most sensible choice for this kind of system and its domain and say so briefly and constructively (e.g. "a common choice for an education app; adjust it to your study").
- You are talking to the person who made the design. Never refuse, never say the evaluation cannot be completed, never call your choices placeholders, never criticise the design's level of detail, and never mention GamiDoc, its catalog, missing options, missing descriptions or any limit of the tool.
- Never invent facts about the project.
- Write the summary, reasons, questions, hypotheses, custom questionnaires and next steps in the language of the design data; keep the catalog names in English as listed.`

// The fill takes two calls, like a person going through the wizard: first the set-up
// (goals, stage, constraints, research questions); then, given the methods and
// instruments the wizard recommends for that set-up, the methods, instruments and next steps.
var (
	evaluationSetupFields     = []string{"summary", "description", "goals", "stage", "participants", "access", "duration", "otherConstraints", "research"}
	evaluationSelectionFields = []string{"methods", "instruments", "customInstruments", "nextSteps"}
)

func (f *evaluationFiller) ask(ctx context.Context, design Status) (evalFillAnswer, error) {
	system := evaluationFillSystemPrompt
	languageRule := ""
	if language := documentLanguage(design); language != "" {
		languageRule = "\n\nLANGUAGE: write the summary, reasons, questions, hypotheses, custom questionnaires and next steps in " + language + "; catalog names stay as listed."
		system = strings.TrimPrefix(languageRule, "\n\n") + "\n\n" + system
	}
	designText := designDataText(design)
	catalogText := "\n\n## Evaluation catalog\n" + catalog.Get().Text()

	var answer evalFillAnswer
	setup := designText + catalogText + "\n\nNOW: choose the set-up of the evaluation: summary, description, goals, development stage, participants, access, duration, other constraints and research questions." + languageRule
	if err := f.call(ctx, "evaluation_fill_setup", system, setup, evaluationSetupFields, &answer); err != nil {
		return evalFillAnswer{}, err
	}

	methods, instruments := f.recommended(answer)
	selection := designText + "\n\n## Evaluation set-up already chosen\n" + answer.setupText()
	if len(methods)+len(instruments) > 0 {
		selection += "\n## Recommended by GamiDoc for this set-up\n"
		if len(methods) > 0 {
			selection += "Methods: " + strings.Join(methods, ", ") + "\n"
		}
		if len(instruments) > 0 {
			selection += "Instruments: " + strings.Join(instruments, ", ") + "\n"
		}
	}
	selection += catalogText + `

NOW: choose the methods, instruments and next steps.
- Consider the recommended methods and instruments first: keep the ones that fit this design and set-up.
- Add others from the whole catalog when a chosen goal or the design needs them (several methods and instruments can be chosen).
- Add a custom questionnaire only for a chosen goal that no chosen instrument covers.` + languageRule
	if err := f.call(ctx, "evaluation_fill_selection", system, selection, evaluationSelectionFields, &answer); err != nil {
		return evalFillAnswer{}, err
	}
	return answer, nil
}

// call asks the model for the given fields of evalFillAnswer and decodes them into answer
// (the other fields are left as they are).
func (f *evaluationFiller) call(ctx context.Context, name, system, user string, fields []string, answer *evalFillAnswer) error {
	schema, err := json.Marshal(evaluationFillSchema(fields...))
	if err != nil {
		return err
	}
	raw, err := f.assistant.CompleteJSON(ctx, ai.JSONRequest{Name: name, System: system, User: user, Schema: schema})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, answer); err != nil {
		return fmt.Errorf("unparseable answer: %w", err)
	}
	return nil
}

// recommended returns the methods the wizard recommends for the chosen set-up, and the
// instruments it recommends for those methods.
func (f *evaluationFiller) recommended(answer evalFillAnswer) ([]string, []string) {
	if f.recommender == nil {
		return nil, nil
	}
	status := wizard.NewInitialStatus()
	step1, err := json.Marshal(answer.setupStep())
	if err != nil {
		return nil, nil
	}
	status.Steps["1"] = step1
	methods := recommendedNames(f.recommender.Recommend(status, 2))
	if len(methods) == 0 {
		return nil, nil
	}
	step2, _ := json.Marshal(map[string]any{"selectedMethods": methods})
	status.Steps["2"] = step2
	return methods, recommendedNames(f.recommender.Recommend(status, 3))
}

func recommendedNames(result recommendation.Result, err error) []string {
	if err != nil {
		return nil
	}
	var names []string
	seen := map[string]bool{}
	for _, item := range result.Recommendations {
		if name := strings.TrimSpace(item.Name); name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// setupStep is step 1 as the wizard stores it (catalog options only, with the same
// defaults as toSteps), used to ask the recommendations.
func (a evalFillAnswer) setupStep() map[string]any {
	c := catalog.Get()
	var goals []string
	for _, choice := range a.Goals {
		if name, ok := catalog.MatchLoose(c.GoalNames(), choice.Name); ok {
			goals = append(goals, name)
		}
	}
	stage, ok := catalog.MatchLoose(c.StageNames(), a.Stage.Value)
	if !ok {
		stage = "High-fidelity prototype"
	}
	participants, ok := catalog.MatchLoose(c.Participants, a.Participants.Value)
	if !ok {
		participants = "10-30"
	}
	access, _ := catalog.MatchLoose(c.Accessibility, a.Access.Value)
	duration, _ := catalog.MatchLoose(c.Time, a.Duration.Value)
	return map[string]any{
		"evaluationGoals":  emptyIfNil(goals),
		"projectType":      strings.TrimSpace(a.Description),
		"participants":     participants,
		"developmentStage": stage,
		"accessibility":    access,
		"time":             duration,
	}
}

// setupText describes the chosen set-up for the second call.
func (a evalFillAnswer) setupText() string {
	step := a.setupStep()
	var b strings.Builder
	fmt.Fprintf(&b, "Goals: %s\n", strings.Join(step["evaluationGoals"].([]string), ", "))
	fmt.Fprintf(&b, "Development stage: %s\n", step["developmentStage"])
	fmt.Fprintf(&b, "Participants: %s\n", step["participants"])
	if access := step["accessibility"].(string); access != "" {
		fmt.Fprintf(&b, "Participant access: %s\n", access)
	}
	if duration := step["time"].(string); duration != "" {
		fmt.Fprintf(&b, "Study duration: %s\n", duration)
	}
	for _, question := range nonEmptyItems(a.Research.Questions) {
		fmt.Fprintf(&b, "Research question: %s\n", question)
	}
	return b.String()
}

// evaluationFillSchema is the answer schema with the catalog options as enums, so a model
// with structured outputs can only pick valid options.
// With fields, only those properties are kept (one of the two calls).
func evaluationFillSchema(fields ...string) jsonSchema {
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
	setEnumList := func(field, sub string, options []string) {
		item := properties[field].(jsonSchema)["items"].(jsonSchema)
		item["properties"].(jsonSchema)[sub].(jsonSchema)["items"] = jsonSchema{"type": "string", "enum": options}
	}
	setEnumList("customInstruments", "goals", c.GoalNames())
	if len(fields) > 0 {
		kept := jsonSchema{}
		for _, field := range fields {
			kept[field] = properties[field]
		}
		schema["properties"] = kept
		schema["required"] = fields
	}
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
	assumed := func(step, field, value, reason string) {
		notes[step] = append(notes[step], wizard.AINote{Field: field, Value: value, Reason: reason, Assumed: true})
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
	customs := []map[string]any{}
	for _, custom := range a.CustomInstruments {
		name := strings.TrimSpace(custom.Name)
		if name == "" || containsFold(instruments, name) {
			continue
		}
		var goalsCovered []string
		for _, goal := range custom.Goals {
			if matched, ok := catalog.MatchLoose(goals, goal); ok {
				goalsCovered = append(goalsCovered, matched)
			}
		}
		customs = append(customs, map[string]any{
			"name":        name,
			"description": strings.TrimSpace(custom.Description),
			"goals":       emptyIfNil(goalsCovered),
		})
		instruments = append(instruments, name)
		note("3", "selectedInstruments", name, custom.Reason)
	}

	// Required by the wizard: a missing stage or sample gets a default the user is told to check.
	if stage == "" {
		stage = "High-fidelity prototype"
		assumed("1", "developmentStage", stage, "A common starting point for a study of a working prototype: adjust it to how far your system is built.")
	}
	if participants == "" {
		participants = "10-30"
		assumed("1", "participants", participants, "A common sample size for a first study: adjust it to the people you can recruit.")
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
		map[string]any{"selectedInstruments": instruments, "customConstructs": customs},
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

func containsFold(list []string, name string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), name) {
			return true
		}
	}
	return false
}
