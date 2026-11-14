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
	text := evaluationUserText(status)
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
	assistant := &scriptedAssistant{answers: map[string]string{"design_document_evaluation": `{
		"title":"x","subtitle":"","context":["I only have the setup."],
		"goals":[{"id":"G1","name":"Usability & Efficiency","meaning":"placeholder: what the goal means","whyHere":""}],
		"methods":[{"id":"M1","name":"placeholder","what":"placeholder"}],
		"instruments":[{"id":"I1","name":"placeholder","what":"placeholder"}],"nextSteps":["placeholder"]}`}}
	_, err := GenerateDocument(context.Background(), assistant, ReportFormatEvaluation, status)
	if !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("expected ErrInvalidDocument, got %v", err)
	}
}

func TestEvaluationReportAlignsToTheWizard(t *testing.T) {
	status := sampleDesignStatus()
	status.Evaluation = sampleEvaluationWizard(t)
	// The AI explains both goals and both methods but forgets the custom questionnaire.
	assistant := &scriptedAssistant{answers: map[string]string{"design_document_evaluation": `{
		"title":"GreenStep","subtitle":"Evaluation of GreenStep.","context":["GreenStep rewards active trips."],
		"goals":[{"id":"G1","name":"Usability & Efficiency","meaning":"Ease of use.","whyHere":"Logging must be quick."},
		         {"id":"G2","name":"Motivation Spectrum","meaning":"Quality of motivation.","whyHere":"Points may crowd out motivation."}],
		"methods":[{"id":"M1","name":"Surveys & Questionnaires","what":"Scales.","whyHere":"Fast."},
		           {"id":"M2","name":"Diary Study","what":"Daily notes.","whyHere":"Habits."}],
		"instruments":[{"id":"I1","name":"Task based protocol","what":"Tasks.","whyHere":"Usability."}],
		"nextSteps":["Pilot with 2 students."]}`}}
	generated, err := GenerateDocument(context.Background(), assistant, ReportFormatEvaluation, status)
	if err != nil {
		t.Fatal(err)
	}
	doc := generated.(*EvalReportDoc)
	if len(doc.Instrs) != 2 || doc.Instrs[1].Name != "Perceived leaderboard pressure" || doc.Instrs[1].ID != "I2" {
		t.Fatalf("instruments: %+v", doc.Instrs)
	}
	if doc.Goals[1].WhyHere != "Points may crowd out motivation." {
		t.Fatalf("AI text lost: %+v", doc.Goals)
	}
}
