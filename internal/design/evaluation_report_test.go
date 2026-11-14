package design

import (
	"encoding/json"
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
