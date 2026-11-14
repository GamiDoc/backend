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
	if len(notes["1"]) == 0 || len(notes["2"]) != 2 || len(notes["3"]) != 1 {
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
	assistant := &scriptedAssistant{answers: map[string]string{"evaluation_fill": testEvaluationAnswer}}
	f := &evaluationFiller{assistant: assistant, store: store, wizard: wizard.NewService(), logger: slog.Default(),
		timeout: time.Minute, running: map[string]runningJob{}}

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
	if !strings.Contains(assistant.requests[0].User, "Evaluation catalog") {
		t.Fatal("prompt without the catalog")
	}
}

func TestEvaluationFillRunKeepsStepsOnFailure(t *testing.T) {
	store := &memoryWizards{project: project.Project{ID: "p1", Wizard: wizard.NewInitialStatus()}}
	assistant := &scriptedAssistant{answers: map[string]string{"evaluation_fill": `{"goals":[],"methods":[]}`}}
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
