package design

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gamidoc/backend/internal/ai"
)

// ── Fakes ───────────────────────────────────────────────────────────────────

type memoryStates struct {
	mu     sync.Mutex
	states map[string]Status
}

func newMemoryStates() *memoryStates {
	return &memoryStates{states: map[string]Status{}}
}

func (m *memoryStates) Get(ctx context.Context, id string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	status, ok := m.states[id]
	if !ok {
		return NewInitialStatus(), nil
	}
	// Deep copy through JSON, as a real store would.
	data, _ := json.Marshal(status)
	var copied Status
	_ = json.Unmarshal(data, &copied)
	return copied, nil
}

func (m *memoryStates) Save(ctx context.Context, id string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[id] = status
	return nil
}

// scriptedAssistant answers CompleteJSON by schema name.
type scriptedAssistant struct {
	ai.NoopAssistant
	mu       sync.Mutex
	answers  map[string]string
	requests []ai.JSONRequest
}

func (a *scriptedAssistant) SupportsJSON() bool { return true }

func (a *scriptedAssistant) CompleteJSON(ctx context.Context, request ai.JSONRequest) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	answer, ok := a.answers[request.Name]
	if !ok {
		return nil, errors.New("no scripted answer for " + request.Name)
	}
	return json.RawMessage(answer), nil
}

const testAnalysis = `{"summary":"A walking app for commuting students.","domain":"Health","targetUsers":"University students","encouragedBehavior":"Walk to campus","goals":["more steps"],"constraints":["low budget"],"assumptions":["smartphone"],"gaps":["platform"],"language":"English"}`

// ── Schema ──────────────────────────────────────────────────────────────────

func TestSchemaHasAllSections(t *testing.T) {
	for number := 1; number <= SectionCount; number++ {
		section, ok := SchemaFor(number)
		if !ok || section.Number != number || len(section.Fields) == 0 {
			t.Fatalf("section %d missing or empty", number)
		}
	}
	if _, ok := SchemaFor(SectionCount + 1); ok {
		t.Fatal("expected no section after the last one")
	}
}

// Strict structured outputs require every object to list all its properties as required
// and to forbid additional ones.
func TestSectionAnswerSchemaIsStrict(t *testing.T) {
	var check func(path string, node any)
	check = func(path string, node any) {
		switch value := node.(type) {
		case map[string]any:
			if value["type"] == "object" {
				properties, _ := value["properties"].(map[string]any)
				required, _ := value["required"].([]any)
				if value["additionalProperties"] != false {
					t.Fatalf("%s: additionalProperties must be false", path)
				}
				if len(required) != len(properties) {
					t.Fatalf("%s: %d required for %d properties", path, len(required), len(properties))
				}
				for _, key := range required {
					if _, ok := properties[key.(string)]; !ok {
						t.Fatalf("%s: required %q is not a property", path, key)
					}
				}
			}
			for key, child := range value {
				check(path+"/"+key, child)
			}
		case []any:
			for _, child := range value {
				check(path+"[]", child)
			}
		}
	}

	for number := 1; number <= SectionCount; number++ {
		section, _ := SchemaFor(number)
		data, err := json.Marshal(sectionAnswerSchema(section, []string{"Points 1"}))
		if err != nil {
			t.Fatal(err)
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		check("section"+SectionKey(number), decoded)
	}
}

func TestSectionPromptDescribesOptionsAndPreviousSections(t *testing.T) {
	section, _ := SchemaFor(2)
	prompt := sectionPrompt("a walking game", ProjectAnalysis{Summary: "walk more"},
		map[int]json.RawMessage{1: json.RawMessage(`{"Domain":["Health"],"Aim of the software":"Walk to campus"}`)},
		section, nil)

	for _, want := range []string{"a walking game", "walk more", "Domain: Health", "Aim of the software: Walk to campus", `"Retention anchors"`, "Ritual anchors"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt misses %q", want)
		}
	}
}

// ── Sanitizing ──────────────────────────────────────────────────────────────

func TestSanitizeKeepsValidOptionsInSchemaOrder(t *testing.T) {
	section, _ := SchemaFor(1)
	answer := `{
		"values": {
			"Constraints": ["lack of autonomy", "Technical difficulties", "Made up constraint"],
			"Aim of the software": "  Help students walk to campus.  ",
			"Domain": ["health", "Space tourism"],
			"Domain · Other": "",
			"Unknown field": "dropped"
		},
		"notes": [
			{"field": "Domain", "reason": "Stated in the spark", "confidence": "HIGH"},
			{"field": "Unknown field", "reason": "x", "confidence": "low"},
			{"field": "Domain", "reason": "duplicate", "confidence": "low"},
			{"field": "Aim of the software", "reason": "Implied", "confidence": "whatever"}
		]
	}`

	draft, err := sanitizeSectionAnswer(section, json.RawMessage(answer), nil)
	content, notes := draft.Content, draft.Notes
	if err != nil {
		t.Fatal(err)
	}

	entries := orderedEntries(content)
	var keys []string
	for _, entry := range entries {
		keys = append(keys, entry.key)
	}
	if strings.Join(keys, "|") != "Domain|Domain · Other|Aim of the software|Constraints" {
		t.Fatalf("unexpected keys/order: %v", keys)
	}

	var decoded map[string]any
	_ = json.Unmarshal(content, &decoded)
	domain := decoded["Domain"].([]any)
	if len(domain) != 2 || domain[0] != "Health" || domain[1] != OtherOption {
		t.Fatalf("unexpected Domain %v", domain)
	}
	if decoded["Domain · Other"] != "Space tourism" {
		t.Fatalf("unknown option should become the Other text, got %v", decoded["Domain · Other"])
	}
	if decoded["Aim of the software"] != "Help students walk to campus." {
		t.Fatalf("text not trimmed: %q", decoded["Aim of the software"])
	}
	constraints := decoded["Constraints"].([]any)
	for _, c := range constraints {
		if c == "Made up constraint" {
			t.Fatal("invented option kept on a field without Other")
		}
	}

	if len(notes) != 2 || notes[0].Confidence != ConfidenceHigh || notes[1].Confidence != ConfidenceMedium {
		t.Fatalf("unexpected notes %+v", notes)
	}
}

func TestSanitizeKeepsTabFieldsAndFlattensMechanics(t *testing.T) {
	section, _ := SchemaFor(4)
	answer := `{
		"values": {
			"Game rules": "Walk to earn points.",
			"Game element categories": "Performance",
			"Performance elements": "Points",
			"Social elements": "Competition",
			"Modality": "Single player"
		},
		"lists": {
			"Points mechanics": [
				{"Trigger": "Walk 1 km", "System response": "+10 points", "Goal supported": "Daily walking", "Linked mechanics": ["Points 2", "Ghost 9"]},
				{"Trigger": "", "System response": "", "Goal supported": "", "Linked mechanics": []},
				{"Trigger": "Weekly streak", "System response": "Bonus", "Goal supported": "Habit", "Linked mechanics": ["Points 1"]}
			],
			"Competition mechanics": [
				{"Trigger": "Hidden", "System response": "Hidden", "Goal supported": "Hidden", "Linked mechanics": []}
			]
		},
		"notes": [{"field": "Points mechanics", "reason": "Fits a walking habit", "confidence": "medium"}]
	}`

	draft, err := sanitizeSectionAnswer(section, json.RawMessage(answer), nil)
	content, notes := draft.Content, draft.Notes
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	_ = json.Unmarshal(content, &decoded)
	// Fields hidden only by a tab (showIf keep) are kept: the AI may fill every tab.
	if decoded["Social elements"] != "Competition" {
		t.Fatalf("tab field dropped: %v", decoded["Social elements"])
	}
	if decoded["Competition mechanics 1 · Trigger"] != "Hidden" {
		t.Fatalf("mechanics of another tab dropped: %v", decoded)
	}
	if decoded["Points mechanics 1 · Name"] != "Points 1" || decoded["Points mechanics 2 · Name"] != "Points 2" {
		t.Fatalf("numbered names not assigned (empty item must be skipped): %v", decoded)
	}
	links := decoded["Points mechanics 1 · Linked mechanics"].([]any)
	if len(links) != 1 || links[0] != "Points 2" {
		t.Fatalf("unknown mechanic links must be dropped, got %v", links)
	}
	if names := mechanicNamesFrom(content); strings.Join(names, ",") != "Points 1,Points 2,Competition 1" {
		t.Fatalf("unexpected mechanic names %v", names)
	}
	if len(notes) != 1 || notes[0].Field != "Points mechanics" {
		t.Fatalf("list note lost: %+v", notes)
	}
}

func TestSanitizeFeedbackLinksOnlyKnownMechanics(t *testing.T) {
	section, _ := SchemaFor(5)
	answer := `{
		"values": {},
		"lists": {"Feedback component": [{
			"Name": "Step celebration",
			"Process stage": ["After success"], "Process stage · Other": "",
			"Message": "Great walk!",
			"Typology": ["Praise / Encouragement"], "Typology · Other": "",
			"Content": "Confetti", "Timing": ["Immediate"], "Timing · Other": "", "Timing details": "",
			"Modality": ["Visual", "Haptic"],
			"Linked game mechanics": ["Points 1", "Nope"],
			"Presentation": "", "Implementation": ""
		}]},
		"notes": []
	}`

	draft, err := sanitizeSectionAnswer(section, json.RawMessage(answer), []string{"Points 1"})
	content := draft.Content
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(content, &decoded)
	if decoded["Feedback component 1 · Name"] != "Step celebration" {
		t.Fatalf("item name lost: %v", decoded)
	}
	links := decoded["Feedback component 1 · Linked game mechanics"].([]any)
	if len(links) != 1 || links[0] != "Points 1" {
		t.Fatalf("unexpected links %v", links)
	}
	if _, ok := decoded["Feedback component 1 · Presentation"]; ok {
		t.Fatal("empty item field kept")
	}
}

func TestSanitizeEmptyAnswer(t *testing.T) {
	section, _ := SchemaFor(6)
	draft, err := sanitizeSectionAnswer(section, json.RawMessage(`{"values":{},"lists":{},"notes":[]}`), nil)
	if err != nil || draft.Content != nil || len(draft.Notes) != 0 {
		t.Fatalf("expected nothing, got %+v %v", draft, err)
	}
	if _, err := sanitizeSectionAnswer(section, json.RawMessage(`[1,2]`), nil); err == nil {
		t.Fatal("expected an error for a non-object answer")
	}
}

// ── Background job ──────────────────────────────────────────────────────────

func runPrefillSync(t *testing.T, assistant ai.Assistant, states *memoryStates, initial Status) Status {
	t.Helper()
	service := NewService()
	locks := newOwnerLocks()
	runner := newPrefillRunner(NewPrefiller(assistant), service, locks)
	o := owner{kind: "session", id: "s1", states: states}

	prefill, ctx := runner.prepare(o)
	initial.Prefill = prefill
	if err := states.Save(context.Background(), o.id, initial); err != nil {
		t.Fatal(err)
	}
	runner.run(ctx, o, prefill.JobID, initial.Spark)

	status, _ := states.Get(context.Background(), o.id)
	return status
}

func TestPrefillJobFillsEmptySectionsAndKeepsUserContent(t *testing.T) {
	assistant := &scriptedAssistant{answers: map[string]string{
		"project_analysis": testAnalysis,
		"section_1":        `{"values":{"Domain":["Health"],"Aim of the software":"Walk more"},"notes":[{"field":"Domain","reason":"Health app","confidence":"high"}]}`,
		"section_3":        `{"values":{"User types":["Achiever"]},"notes":[]}`,
		"section_4": `{"values":{"Game element categories":"Performance","Performance elements":"Points"},
			"lists":{"Points mechanics":[{"Trigger":"Walk","System response":"+1","Goal supported":"Walk","Linked mechanics":[]}]},"notes":[]}`,
		"section_5": `{"values":{},"lists":{"Feedback component":[{"Name":"Cheer","Process stage":[],"Process stage · Other":"","Message":"Nice","Typology":[],"Typology · Other":"","Content":"","Timing":[],"Timing · Other":"","Timing details":"","Modality":[],"Linked game mechanics":["Points 1"],"Presentation":"","Implementation":""}]},"notes":[]}`,
		// section_6 has no answer: the call fails, the job goes on.
	}}

	states := newMemoryStates()
	initial := NewInitialStatus()
	initial.Spark = "A walking app for students"
	initial.Sections = map[string]SectionState{"2": {Content: json.RawMessage(`{"Update frequency":"Weekly"}`), Visited: true}}

	status := runPrefillSync(t, assistant, states, initial)

	if status.Prefill.State != PrefillStateDone {
		t.Fatalf("expected done, got %+v", status.Prefill)
	}
	if string(status.Section(2).Content) != `{"Update frequency":"Weekly"}` {
		t.Fatalf("user content overwritten: %s", status.Section(2).Content)
	}
	if !strings.Contains(string(status.Section(1).Content), "Walk more") {
		t.Fatalf("section 1 not filled: %s", status.Section(1).Content)
	}
	if !strings.Contains(string(status.Section(5).Content), `"Points 1"`) {
		t.Fatalf("feedback should link the mechanic created in section 4: %s", status.Section(5).Content)
	}
	if len(status.Section(6).Content) != 0 {
		t.Fatal("section 6 should be empty")
	}
	if got := status.Prefill; strings.Join(intsToStrings(got.Skipped), ",") != "2" || strings.Join(intsToStrings(got.Failed), ",") != "6" {
		t.Fatalf("unexpected skipped/failed %+v", got)
	}
	if notes := status.Prefill.Notes["1"]; len(notes) != 1 || notes[0].Field != "Domain" {
		t.Fatalf("notes not stored: %+v", status.Prefill.Notes)
	}
	if status.Prefill.Analysis == nil || status.Prefill.Analysis.TargetUsers != "University students" {
		t.Fatal("analysis not stored")
	}
	if status.Section(1).Complete {
		t.Fatal("prefill must not mark sections complete")
	}

	// Section 2 (user content) is given to the next sections as context.
	for _, request := range assistant.requests {
		if request.Name == "section_3" && !strings.Contains(request.User, "Update frequency: Weekly") {
			t.Fatal("user content not passed as context")
		}
	}
}

func TestPrefillJobFailsWhenAnalysisFails(t *testing.T) {
	status := runPrefillSync(t, &scriptedAssistant{answers: map[string]string{}}, newMemoryStates(), Status{Spark: "x"})
	if status.Prefill.State != PrefillStateFailed || status.Prefill.FinishedAt == nil {
		t.Fatalf("expected failed, got %+v", status.Prefill)
	}
	if status.HasContent() {
		t.Fatal("no content expected")
	}
}

func TestReplacedJobStopsWriting(t *testing.T) {
	states := newMemoryStates()
	runner := newPrefillRunner(NewPrefiller(&scriptedAssistant{}), NewService(), newOwnerLocks())
	o := owner{kind: "session", id: "s1", states: states}

	old, _ := runner.prepare(o)
	current, _ := runner.prepare(o)
	_ = states.Save(context.Background(), o.id, Status{Prefill: current})

	if runner.update(o, old.JobID, func(status *Status) { status.Spark = "overwritten" }) {
		t.Fatal("a replaced job must not write")
	}
	if runner.isRunning(o, old.JobID) || !runner.isRunning(o, current.JobID) {
		t.Fatal("only the newest job runs")
	}
}

func TestLivenessMarksOrphanJobsFailed(t *testing.T) {
	runner := newPrefillRunner(NewPrefiller(ai.NewNoopAssistant()), NewService(), newOwnerLocks())
	o := owner{kind: "project", id: "p1"}
	status := runner.withLiveness(o, Status{Prefill: &PrefillState{JobID: "gone", State: PrefillStateRunning}})
	if status.Prefill.State != PrefillStateFailed {
		t.Fatalf("expected failed, got %s", status.Prefill.State)
	}
}

// ── Handler ─────────────────────────────────────────────────────────────────

func TestSaveSparkWithoutAIDoesNotStartPrefill(t *testing.T) {
	states := newMemoryStates()
	handler := NewHandler(NewService(), ai.NewNoopAssistant(), nil, nil, states, nil, nil)
	o := owner{kind: "session", id: "s1", states: states}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/spark", strings.NewReader(`{"spark":"  a game  "}`))
	handler.saveSpark(recorder, request, o)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		DesignStatus   Status `json:"designStatus"`
		PrefillStarted bool   `json:"prefillStarted"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	if body.PrefillStarted || body.DesignStatus.Prefill != nil || body.DesignStatus.Spark != "a game" {
		t.Fatalf("unexpected response %s", recorder.Body.String())
	}
}

func intsToStrings(values []int) []string {
	var result []string
	for _, v := range values {
		result = append(result, SectionKey(v))
	}
	return result
}

func TestSanitizeReportsDroppedValuesAndMissingRequired(t *testing.T) {
	section, _ := SchemaFor(2)
	answer := `{"values":{"Retention anchors":["Ancore rituali"],"Goal types":[],"Invented":"x"},"lists":{},"notes":[]}`
	draft, err := sanitizeSectionAnswer(section, json.RawMessage(answer), nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(draft.Dropped, " | ")
	if !strings.Contains(joined, `Retention anchors: "Ancore rituali" (not an option)`) || !strings.Contains(joined, "Invented") {
		t.Fatalf("unexpected dropped %v", draft.Dropped)
	}
	if !containsString(draft.MissingRequired, "Retention anchors") || !containsString(draft.MissingRequired, "Goal types") {
		t.Fatalf("unexpected missing %v", draft.MissingRequired)
	}
}

func TestPrefillJobSingleModeUsesOneCall(t *testing.T) {
	assistant := &scriptedAssistant{answers: map[string]string{
		"design_draft": `{"analysis":` + testAnalysis + `,"sections":{
			"1":{"values":{"Domain":["Health"]},"notes":[{"field":"Domain","reason":"Health app","confidence":"high"}]},
			"4":{"values":{"Game element categories":"Performance","Performance elements":"Points"},
				"lists":{"Points mechanics":[{"Trigger":"Walk","System response":"+1","Goal supported":"Walk","Linked mechanics":[]}]},"notes":[]},
			"5":{"values":{},"lists":{"Feedback component":[{"Name":"Cheer","Process stage":[],"Process stage · Other":"","Message":"Nice","Typology":[],"Typology · Other":"","Content":"","Timing":[],"Timing · Other":"","Timing details":"","Modality":[],"Linked game mechanics":["Points 1"],"Presentation":"","Implementation":""}]},"notes":[]}
		}}`,
	}}
	service := NewService()
	runner := newPrefillRunner(NewPrefiller(assistant), service, newOwnerLocks())
	runner.mode = PrefillModeSingle
	states := newMemoryStates()
	o := owner{kind: "session", id: "s1", states: states}

	initial := NewInitialStatus()
	initial.Spark = "walk"
	initial.Sections = map[string]SectionState{"2": {Content: json.RawMessage(`{"Update frequency":"Weekly"}`)}}
	prefill, ctx := runner.prepare(o)
	initial.Prefill = prefill
	_ = states.Save(context.Background(), o.id, initial)
	runner.run(ctx, o, prefill.JobID, initial.Spark)

	status, _ := states.Get(context.Background(), o.id)
	if len(assistant.requests) != 1 {
		t.Fatalf("expected one AI call, got %d", len(assistant.requests))
	}
	if !strings.Contains(assistant.requests[0].User, "Update frequency: Weekly") {
		t.Fatal("existing section not given as context")
	}
	if status.Prefill.State != PrefillStateDone || status.Prefill.Analysis == nil {
		t.Fatalf("unexpected prefill %+v", status.Prefill)
	}
	if strings.Join(intsToStrings(status.Prefill.Completed), ",") != "1,4,5" ||
		strings.Join(intsToStrings(status.Prefill.Failed), ",") != "3,6" ||
		strings.Join(intsToStrings(status.Prefill.Skipped), ",") != "2" {
		t.Fatalf("unexpected completed/failed/skipped %+v", status.Prefill)
	}
	if !strings.Contains(string(status.Section(5).Content), `"Points 1"`) {
		t.Fatalf("feedback must link the mechanic drafted in the same call: %s", status.Section(5).Content)
	}
	if string(status.Section(2).Content) != `{"Update frequency":"Weekly"}` {
		t.Fatal("user content overwritten")
	}
}

func TestSingleModeEmptyDraftIsRetriedThenFails(t *testing.T) {
	assistant := &scriptedAssistant{answers: map[string]string{
		"design_draft": `{"analysis":` + testAnalysis + `,"sections":{"1":{"values":{"Domain":[],"Aim of the software":""},"notes":[]}}}`,
	}}
	runner := newPrefillRunner(NewPrefiller(assistant), NewService(), newOwnerLocks())
	runner.mode = PrefillModeSingle
	states := newMemoryStates()
	o := owner{kind: "session", id: "s1", states: states}

	initial := NewInitialStatus()
	initial.Spark = "walk"
	prefill, ctx := runner.prepare(o)
	initial.Prefill = prefill
	_ = states.Save(context.Background(), o.id, initial)
	runner.run(ctx, o, prefill.JobID, initial.Spark)

	status, _ := states.Get(context.Background(), o.id)
	if len(assistant.requests) != 2 {
		t.Fatalf("an empty draft must be retried once, got %d calls", len(assistant.requests))
	}
	if status.Prefill.State != PrefillStateFailed || len(status.Prefill.Completed) != 0 || len(status.Prefill.Failed) != SectionCount {
		t.Fatalf("empty sections must count as failed: %+v", status.Prefill)
	}
}
