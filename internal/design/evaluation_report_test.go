package design

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gamidoc/backend/internal/wizard"
)

func sampleEvaluationWizard(t *testing.T) *wizard.Status {
	t.Helper()
	var answer evalFillAnswer
	if err := json.Unmarshal([]byte(testEvaluationAnswer), &answer); err != nil {
		t.Fatal(err)
	}
	steps, _, err := answer.toSteps("GreenStep")
	if err != nil {
		t.Fatal(err)
	}
	status := wizard.NewInitialStatus()
	service := wizard.NewService()
	for i, data := range steps {
		if status, err = service.SaveStep(status, i+1, data); err != nil {
			t.Fatal(err)
		}
	}
	return &status
}

func TestEvaluationReportFallbackRenders(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)

	doc := fallbackDocument(ReportFormatEvaluation, status).(*EvalReportDoc)
	setDocumentTitle(doc, "GreenStep")
	if len(doc.Goals) != 2 || len(doc.Methods) != 2 || len(doc.Instrs) != 2 || doc.Goals[0].Meaning == "" || len(doc.Instrs[1].Covers) != 1 || !strings.Contains(doc.Instrs[1].What, "Custom questionnaire") {
		t.Fatalf("fallback: %+v", doc)
	}
	html, err := RenderDocumentHTML(doc, ReportFormatEvaluation, status, "DES-1", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"UX evaluation report", "GreenStep", "Plan ID", "High-fidelity prototype", "The instruments, explained", "G1"} {
		if !strings.Contains(html, want) {
			t.Errorf("html without %q", want)
		}
	}
	if _, err := RenderDocumentPlainPDF(doc, ReportFormatEvaluation, status, "DES-1"); err != nil {
		t.Fatal(err)
	}
	if DocumentFileName("GreenStep", ReportFormatEvaluation) != "GreenStep_evaluation-report.pdf" {
		t.Fatal("file name")
	}
}

func TestEvaluationPromptHasDataAndReferences(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)
	status.Title = "GreenStep"
	text := evaluationHeader(status)
	if !strings.Contains(text, "PROJECT NAME: GreenStep") {
		t.Error("prompt without the project name")
	}
	for _, want := range []string{"UX evaluation entered", "Diary Study", "Reference catalogue", "UMUX-Lite"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt without %q", want)
		}
	}
}

func TestSplitFieldLines(t *testing.T) {
	prose, facts := splitFieldLines([]string{
		"APP_NAME: Spark",
		"DOMAIN: Health\nGOAL: Keep memory trained.",
		"Obiettivo: mantenere allenata la memoria delle persone anziane con una sessione breve ogni giorno.",
		"The study checks whether the daily session is easy to complete.",
	})
	if len(facts) != 3 || facts[0].Label != "App name" || facts[1].Value != "Health" {
		t.Fatalf("facts: %+v", facts)
	}
	if len(prose) != 2 || !strings.HasPrefix(prose[0], "Obiettivo:") {
		t.Fatalf("prose: %q", prose)
	}
}

func TestEvaluationReportRejectsPlaceholders(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)
	assistant := &scriptedAssistant{answers: map[string]string{
		"evaluation_report_context": `{"subtitle":"","context":["I only have the setup."],"goals":[{"id":"G1","meaning":"placeholder","whyHere":"placeholder"}],"questions":[],"hypotheses":[]}`,
		"evaluation_report_methods": `{"methods":[],"instruments":[],"coverage":[],"nextSteps":["placeholder"]}`,
	}}
	_, err := GenerateDocument(context.Background(), assistant, ReportFormatEvaluation, status)
	if !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("expected ErrInvalidDocument, got %v", err)
	}
}

func TestEvaluationReportExplainsTheSkeletonByID(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)
	// Two goals, two methods, two instruments (one custom); the AI forgets the custom one.
	assistant := &scriptedAssistant{answers: map[string]string{
		"evaluation_report_context": `{"subtitle":"Evaluation of GreenStep.","context":["GreenStep rewards active trips."],
			"goals":[{"id":"G1","meaning":"Ease of use.","whyHere":"Logging must be quick."},{"id":"g2","meaning":"Quality of motivation.","whyHere":"Points may crowd out motivation."}],
			"questions":[{"id":"RQ1","reasoning":"Usability first.","exploratory":false,"hypotheses":["H1"]}],
			"hypotheses":[{"id":"H1","rationale":"68 is the SUS average.","refs":[102]}]}`,
		"evaluation_report_methods": `{"methods":[{"id":"M1","what":"Scales.","whyHere":"Fast.","limits":"Self-report."},{"id":"M2","what":"Daily notes.","whyHere":"Habits.","limits":"Drop-out."}],
			"instruments":[{"id":"I1","what":"Tasks.","whyHere":"Usability.","measures":"Completion.","howToRead":"Share of tasks done.","limits":"Lab."}],
			"coverage":["Each goal is seen twice."],"nextSteps":["Pilot with 2 students."]}`,
	}}
	generated, err := GenerateDocument(context.Background(), assistant, ReportFormatEvaluation, status)
	if err != nil {
		t.Fatal(err)
	}
	doc := generated.(*EvalReportDoc)
	if doc.Goals[1].WhyHere != "Points may crowd out motivation." || doc.Methods[1].WhyHere != "Habits." {
		t.Fatalf("AI text not merged: %+v %+v", doc.Goals, doc.Methods)
	}
	if len(doc.Instrs) != 2 || doc.Instrs[1].Name != "Perceived leaderboard pressure" || !strings.Contains(doc.Instrs[1].What, "Custom questionnaire") {
		t.Fatalf("instruments: %+v", doc.Instrs)
	}
	if len(doc.Questions) != 1 || len(doc.Questions[0].Hypotheses) != 1 || doc.Questions[0].Hypotheses[0].Rationale == "" {
		t.Fatalf("questions: %+v", doc.Questions)
	}
	if !strings.Contains(assistant.requests[1].User, "I2 Perceived leaderboard pressure") {
		t.Fatal("second call without the custom questionnaire")
	}
}

func TestEvaluationPlanIsBuiltWithoutAI(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)
	if !ValidReportFormat(ReportFormatEvaluationPlan) || ReportFormatTitle(ReportFormatEvaluationPlan) != "UX evaluation plan" {
		t.Fatal("format not registered")
	}
	if DocumentFileName("GreenStep", ReportFormatEvaluationPlan) != "GreenStep_evaluation-plan.pdf" {
		t.Fatal("file name")
	}
	doc := fallbackDocument(ReportFormatEvaluationPlan, status).(*EvalReportDoc)
	html, err := RenderDocumentHTML(doc, ReportFormatEvaluationPlan, status, "PLAN-1", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "UX evaluation plan") || !strings.Contains(html, "Built from the UX evaluation") {
		t.Fatal("plan cover or note missing")
	}
}

func TestCleanContextKeepsOnlyTheContext(t *testing.T) {
	got := cleanContext([]string{
		"# Progress test", "## Subtitle", "A concept-stage evaluation.",
		"## Project context", "Progress test is an education concept [4].", "The study has 10-30 participants.",
		"## Goals", "### G1 Usability & Efficiency", "Learners must reach tasks.",
	})
	if len(got) != 2 || got[0] != "Progress test is an education concept." {
		t.Fatalf("context: %q", got)
	}
	plain := cleanContext([]string{"First paragraph.", "Second paragraph."})
	if len(plain) != 2 {
		t.Fatalf("plain context: %q", plain)
	}
}
