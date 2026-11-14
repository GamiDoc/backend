package design

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/recommendation"
	"github.com/gamidoc/backend/internal/wizard"
)

const testEvaluationAnswer = `{
  "summary": "A team challenge for active commuting; the evaluation checks usability and whether points keep motivation intrinsic.",
  "description": "A mobile app that rewards students' active trips with points and a weekly team challenge.",
  "goals": [
    {"name": "usability & efficiency", "reason": "Logging a trip must take seconds."},
    {"name": "Motivation Spectrum", "reason": "Points may crowd out intrinsic motivation."},
    {"name": "Made-up goal", "reason": "dropped"},
    {"name": "Motivation Spectrum", "reason": "duplicate"}
  ],
  "stage": {"value": "high-fidelity prototype", "reason": "The loops are specified in detail."},
  "participants": {"value": "10-30", "reason": "A class-sized pilot."},
  "access": {"value": "Easy", "reason": "Students on campus."},
  "duration": {"value": "1 month", "reason": "Habits take weeks."},
  "otherConstraints": [],
  "research": {"objective": "Check usability and motivation.", "questions": ["Can students log a trip unaided?", " "], "hypotheses": ["UMUX-Lite of at least 68"], "reason": "Usability first, then motivation."},
  "methods": [{"name": "Surveys & Questionnaires", "reason": "Validated scales."}, {"name": "Diary Study", "reason": "Behaviour over weeks."}],
  "instruments": [{"name": "Task based protocol", "reason": "Structured tasks."}, {"name": "Unknown scale", "reason": "dropped"}],
  "customInstruments": [{"name": "Perceived leaderboard pressure", "description": "How much the department ranking makes students feel pushed.", "goals": ["motivation spectrum"], "reason": "No chosen instrument covers it."}],
  "nextSteps": ["Run a pilot with 2 students."]
}`

func TestEvaluationAnswerToSteps(t *testing.T) {
	var answer evalFillAnswer
	if err := json.Unmarshal([]byte(testEvaluationAnswer), &answer); err != nil {
		t.Fatal(err)
	}
	steps, notes, err := answer.toSteps("GreenStep")
	if err != nil {
		t.Fatal(err)
	}
	var step1 wizard.Step1Data
	if err := json.Unmarshal(steps[0], &step1); err != nil {
		t.Fatal(err)
	}
	if strings.Join(step1.EvaluationGoals, "|") != "Usability & Efficiency|Motivation Spectrum" {
		t.Fatalf("goals: %v", step1.EvaluationGoals)
	}
	if step1.DevelopmentStage != "High-fidelity prototype" || step1.Participants != "10-30" || !step1.ResearchEnabled || len(step1.ResearchQuestions) != 1 {
		t.Fatalf("step 1: %+v", step1)
	}
	for number, data := range steps {
		if err := wizard.ValidateStep(number+1, data); err != nil {
			t.Fatalf("step %d invalid: %v", number+1, err)
		}
	}
	var step3 wizard.Step3Data
	_ = json.Unmarshal(steps[2], &step3)
	if strings.Join(step3.SelectedInstruments, "|") != "Task based protocol|Perceived leaderboard pressure" || !strings.Contains(string(steps[2]), `"goals":["Motivation Spectrum"]`) {
		t.Fatalf("step 3: %s", steps[2])
	}
	if len(notes["1"]) == 0 || len(notes["2"]) != 2 || len(notes["3"]) != 2 {
		t.Fatalf("notes: %+v", notes)
	}
}

type memoryWizards struct {
	mu      sync.Mutex
	project project.Project
}

func (m *memoryWizards) FindByID(ctx context.Context, id string) (project.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.project, nil
}

func (m *memoryWizards) UpdateWizard(ctx context.Context, id string, status wizard.Status) (project.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.project.Wizard = status
	return m.project, nil
}

func TestEvaluationFillRunSavesStepsAndNotes(t *testing.T) {
	store := &memoryWizards{project: project.Project{ID: "p1", Name: "GreenStep", Wizard: wizard.NewInitialStatus()}}
	assistant := &scriptedAssistant{answers: map[string]string{"evaluation_fill_setup": testEvaluationAnswer, "evaluation_fill_selection": testEvaluationAnswer}}
	f := &evaluationFiller{assistant: assistant, store: store, wizard: wizard.NewService(), logger: slog.Default(),
		timeout: time.Minute, running: map[string]runningJob{}, recommender: fakeRecommender{}}

	job := wizard.AIFill{JobID: "j1", State: wizard.AIFillRunning, StartedAt: time.Now()}
	status := store.project.Wizard
	status.AIFill = &job
	store.project.Wizard = status

	f.run(context.Background(), "p1", "GreenStep", job, sampleDesignStatus())

	got := store.project.Wizard
	if got.AIFill == nil || got.AIFill.State != wizard.AIFillDone || got.AIFill.Summary == "" {
		t.Fatalf("ai fill: %+v", got.AIFill)
	}
	if !got.IsComplete || got.CurrentStep != 4 {
		t.Fatalf("wizard not complete: step %d complete %v", got.CurrentStep, got.IsComplete)
	}
	if len(assistant.requests) != 2 || !strings.Contains(assistant.requests[0].User, "Evaluation catalog") {
		t.Fatalf("expected two calls with the catalog, got %d", len(assistant.requests))
	}
	second := assistant.requests[1].User
	if !strings.Contains(second, "Recommended by GamiDoc") || !strings.Contains(second, "Methods: Surveys & Questionnaires") ||
		!strings.Contains(second, "Instruments: UMUX-Lite") || !strings.Contains(second, "Goals: Usability & Efficiency") {
		t.Fatalf("second prompt without set-up or recommendations:\n%s", second)
	}
}

func TestEvaluationFillRunKeepsStepsOnFailure(t *testing.T) {
	store := &memoryWizards{project: project.Project{ID: "p1", Wizard: wizard.NewInitialStatus()}}
	assistant := &scriptedAssistant{answers: map[string]string{"evaluation_fill_setup": `{"goals":[]}`, "evaluation_fill_selection": `{"methods":[]}`}}
	f := &evaluationFiller{assistant: assistant, store: store, wizard: wizard.NewService(), logger: slog.Default(),
		timeout: time.Minute, running: map[string]runningJob{}}
	job := wizard.AIFill{JobID: "j1", State: wizard.AIFillRunning}
	status := store.project.Wizard
	status.AIFill = &job
	store.project.Wizard = status

	f.run(context.Background(), "p1", "", job, sampleDesignStatus())

	got := store.project.Wizard
	if got.AIFill.State != wizard.AIFillFailed || len(got.Steps) != 0 {
		t.Fatalf("expected failure without steps: %+v %d", got.AIFill, len(got.Steps))
	}
}

func TestEvaluationFillSchemaHasCatalogEnums(t *testing.T) {
	encoded, err := json.Marshal(evaluationFillSchema())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"High-fidelity prototype"`, `"Think-aloud testing"`, `"UMUX-Lite"`, `"10-30"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("schema without %s", want)
		}
	}
}

func TestEvaluationAnswerDefaultsStageAndSample(t *testing.T) {
	var answer evalFillAnswer
	if err := json.Unmarshal([]byte(testEvaluationAnswer), &answer); err != nil {
		t.Fatal(err)
	}
	answer.Stage.Value = "it is a working beta"
	answer.Participants.Value = "about twenty"
	steps, notes, err := answer.toSteps("GreenStep")
	if err != nil {
		t.Fatal(err)
	}
	if wizard.ValidateStep(1, steps[0]) != nil {
		t.Fatal("step 1 invalid")
	}
	defaults := 0
	for _, note := range notes["1"] {
		if note.Assumed {
			defaults++
		}
	}
	if defaults != 2 {
		t.Fatalf("expected 2 default notes, got %d", defaults)
	}
}

// fakeRecommender recommends methods for step 2 and, once methods are chosen, instruments.
type fakeRecommender struct{}

func (fakeRecommender) Recommend(status wizard.Status, forStep int) (recommendation.Result, error) {
	if forStep == 2 {
		return recommendation.Result{ForStep: 2, Recommendations: []recommendation.Recommendation{{Name: "Surveys & Questionnaires"}}}, nil
	}
	if _, ok := status.Steps["2"]; !ok {
		return recommendation.Result{}, nil
	}
	return recommendation.Result{ForStep: 3, Recommendations: []recommendation.Recommendation{{Name: "UMUX-Lite"}, {Name: "UMUX-Lite"}}}, nil
}
