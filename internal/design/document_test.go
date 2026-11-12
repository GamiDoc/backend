package design

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/pdf"
	"github.com/gamidoc/backend/internal/storage/objectstore"
)

func sampleDesignStatus() Status {
	status := NewInitialStatus()
	status.Spark = "An app that turns the commute to university into a team challenge."
	status.Sections = map[string]SectionState{
		"1": {Content: json.RawMessage(`{"Domain":["Sustainability","Health"],"Aim of the software":"More active trips to campus","Occupation":"Students","Constraints":["Lack of autonomy"]}`)},
		"2": {Content: json.RawMessage(`{"Retention anchors":["Ritual anchors","Social anchors"]}`)},
		"4": {Content: json.RawMessage(`{"Game rules":"10 points per km on foot","Game element categories":"Performance","Performance elements":"Points","Points mechanics 1 · Name":"Points 1","Points mechanics 1 · Trigger":"A trip ends","Points mechanics 1 · System response":"+10 points per km","Points mechanics 1 · Goal supported":"Every trip counts","Modality":"Competitive / Cooperative"}`)},
	}
	return status
}

const sampleReportJSON = `{
  "title": "GreenStep", "subtitle": "A team challenge for active commuting.",
  "glance": {"forWhom": "Students", "whatWeWant": "3 active trips a week", "motivation": "Their team", "howWeKnow": "A 4-week study", "mainRisk": "Playing for points", "addressing": "Points mean CO2"},
  "loops": {"coreLabel": "every trip", "core": {"steps": [{"title": "Action", "text": "walk"}, {"title": "Result", "text": "points"}, {"title": "Choice", "text": "mode back"}], "repeat": "next trip"},
            "longLabel": "every week", "long": {"steps": [{"title": "Weekly goal", "text": "3 trips"}, {"title": "Level", "text": "monthly"}, {"title": "Return", "text": "Monday"}], "repeat": "every week"}, "link": "every trip counts"},
  "sections": [{"number": 2, "inShort": "Habits take weeks.", "facts": [], "blocks": [
      {"title": "What brings users back", "term": "Retention anchors", "values": ["Ritual anchors"], "why": "Habits form over weeks.", "inTheDesign": ["Daily streak"], "numbered": false, "watchOut": "Fear of loss.", "refs": [8, 99]}],
      "feedback": [], "closing": ""}],
  "coherence": [{"outcome": "Keep an eye on", "choices": "Points, Players", "handling": "Symbolic rewards"}],
  "evalIntro": "Each risky choice maps to a goal.", "evaluation": [{"choice": "Points", "goal": "Motivation Spectrum", "measure": "IMI"}],
  "assumptions": []
}`

const sampleGDDJSON = `{
  "title": "GreenStep", "subtitle": "Team commuting game", "highConcept": "Every trip earns points.",
  "domain": "Sustainability", "players": "Students 19-26", "targetBehaviour": "3+ active trips / week", "modality": "Team vs team",
  "pillars": [{"title": "Every trip counts", "text": "Immediate result."}, {"title": "Together", "text": "Teams only."}, {"title": "No guilt", "text": "Nothing lost."}],
  "loops": {"coreLabel": "", "core": {"steps": [], "repeat": ""}, "longLabel": "", "long": {"steps": [], "repeat": ""}, "link": ""},
  "outOfScope": "", "playerAspects": [{"name": "Who", "value": "Students"}], "playerTypes": [{"type": "Achiever", "motivatedBy": "Competence", "servedBy": "SYS-01"}],
  "loopRows": [], "globalRules": ["At most 2 trips a day"],
  "systems": [{"id": "SYS-01", "name": "Points", "purpose": "Value every trip.", "rules": ["Points per km"], "values": [{"name": "On foot", "value": "10 points / km"}], "feedback": ["F1"], "risks": "Chasing points.", "fromGamiDoc": "Game rules"}],
  "progression": "Cyclical.", "rewards": [], "journey": [], "adaptation": [], "adaptationNote": "", "feedback": [], "technical": [], "risks": [], "metrics": [],
  "rationale": [{"title": "Symbolic rewards only", "text": "Extrinsic rewards can undermine motivation.", "refs": [2]}],
  "terms": [{"term": "Active member", "meaning": "A player with a trip this week."}]
}`

func TestDocumentSchemasAreStrict(t *testing.T) {
	for _, doc := range []any{ReportDoc{}, GDDDoc{}} {
		data, _ := json.Marshal(docSchema(doc))
		var decoded any
		_ = json.Unmarshal(data, &decoded)
		var check func(path string, node any)
		check = func(path string, node any) {
			switch value := node.(type) {
			case map[string]any:
				if value["type"] == "object" {
					properties, _ := value["properties"].(map[string]any)
					required, _ := value["required"].([]any)
					if value["additionalProperties"] != false || len(required) != len(properties) {
						t.Fatalf("%s: not strict", path)
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
		check("doc", decoded)
	}
}

func TestGenerateDocumentWithAI(t *testing.T) {
	assistant := &scriptedAssistant{answers: map[string]string{
		"design_document_report": sampleReportJSON,
		"design_document_gdd":    sampleGDDJSON,
	}}
	status := sampleDesignStatus()

	report, err := GenerateDocument(context.Background(), assistant, ReportFormatReport, status)
	if err != nil {
		t.Fatal(err)
	}
	if doc := report.(*ReportDoc); doc.Title != "GreenStep" || len(doc.Sections) != 1 {
		t.Fatalf("unexpected report %+v", doc)
	}
	prompt := assistant.requests[0].User
	for _, want := range []string{"Retention anchors [choice]: Ritual anchors, Social anchors", "Reference catalogue", "8. habit formation"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt misses %q", want)
		}
	}

	gdd, err := GenerateDocument(context.Background(), assistant, ReportFormatGDD, status)
	if err != nil || len(gdd.(*GDDDoc).Systems) != 1 {
		t.Fatalf("unexpected gdd %+v %v", gdd, err)
	}

	empty := &scriptedAssistant{answers: map[string]string{"design_document_report": `{"title":"","sections":[],"glance":{}}`}}
	if _, err := GenerateDocument(context.Background(), empty, ReportFormatReport, status); err == nil {
		t.Fatal("an empty document must be an error")
	}
}

func TestRenderReportHTMLNumbersCitationsAndBuildsGlossary(t *testing.T) {
	var doc ReportDoc
	if err := json.Unmarshal([]byte(sampleReportJSON), &doc); err != nil {
		t.Fatal(err)
	}
	html, err := RenderDocumentHTML(&doc, ReportFormatReport, sampleDesignStatus(), "DES-1", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"GreenStep", "What brings users back", "(Retention anchors)", "Habits form over weeks. [1]",
		"Lally, P.",                              // catalogue id 8, printed as reference 1
		"Ritual anchors", "Habitual touchpoints", // glossary from the option description
		"Keep an eye on", "Motivation Spectrum", "DES-1",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report html misses %q", want)
		}
	}
	if strings.Contains(html, "[99]") || strings.Contains(html, "ZgotmplZ") {
		t.Fatal("unknown reference printed or unsafe value in template")
	}
}

func TestRenderGDDHTMLAndPlainPDF(t *testing.T) {
	var doc GDDDoc
	if err := json.Unmarshal([]byte(sampleGDDJSON), &doc); err != nil {
		t.Fatal(err)
	}
	html, err := RenderDocumentHTML(&doc, ReportFormatGDD, sampleDesignStatus(), "DES-1", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Game Design Document", "SYS-01", "10 points / km", "Every trip counts", "Active member", "Deci, E. L., Koestner", "[1]"} {
		if !strings.Contains(html, want) {
			t.Fatalf("gdd html misses %q", want)
		}
	}

	for _, format := range []string{ReportFormatReport, ReportFormatGDD} {
		pdf, err := RenderDocumentPlainPDF(fallbackDocument(format, sampleDesignStatus()), format, sampleDesignStatus(), "DES-1")
		if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatalf("plain pdf (%s) not built: %v", format, err)
		}
	}
}

func TestFallbackGDDTurnsMechanicsIntoSystems(t *testing.T) {
	doc := fallbackDocument(ReportFormatGDD, sampleDesignStatus()).(*GDDDoc)
	if len(doc.Systems) != 1 || doc.Systems[0].ID != "SYS-01" || doc.Systems[0].Name != "Points 1, Points" {
		t.Fatalf("unexpected systems %+v", doc.Systems)
	}
	if doc.Domain != "Sustainability, Health" || doc.Modality != "Competitive / Cooperative" {
		t.Fatalf("unexpected one-pager %+v", doc)
	}
}

func TestGeneratePDFRunsInTheBackground(t *testing.T) {
	store := objectstore.NewLocalStore(t.TempDir(), "/files/pdfs")
	reports := NewReportService(ai.NewNoopAssistant(), nil, store, nil)
	states := newMemoryStates()
	handler := NewHandler(NewService(), ai.NewNoopAssistant(), reports, nil, states, nil, nil)
	o := owner{kind: "sessions", id: "s1", states: states}

	status := sampleDesignStatus()
	status.FirstPassDone = true
	_ = states.Save(context.Background(), o.id, status)

	recorder := httptest.NewRecorder()
	handler.generatePDF(recorder, httptest.NewRequest(http.MethodPost, "/generate-pdf", strings.NewReader(`{"format":"gdd"}`)), o)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		current, _ := states.Get(context.Background(), o.id)
		if job := current.ReportJob; job != nil && job.State != PrefillStateRunning {
			if job.State != PrefillStateDone || job.Report == nil || job.Report.Version != ReportFormatGDD || len(current.Reports) != 1 {
				t.Fatalf("unexpected job %+v", job)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("document job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	bad := httptest.NewRecorder()
	handler.generatePDF(bad, httptest.NewRequest(http.MethodPost, "/generate-pdf", strings.NewReader(`{"format":"enhanced"}`)), o)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown format must be rejected, got %d", bad.Code)
	}
}

// TestPreviewDocumentsWithChrome writes the sample documents as PDF with a local browser,
// for a visual check: GAMIDOC_DOC_PREVIEW_DIR=<dir> go test -run Preview ./internal/design/
func TestPreviewDocumentsWithChrome(t *testing.T) {
	dir := os.Getenv("GAMIDOC_DOC_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set GAMIDOC_DOC_PREVIEW_DIR to write preview PDFs")
	}
	renderer := previewRenderer(t)
	for format, raw := range map[string]string{ReportFormatReport: sampleReportJSON, ReportFormatGDD: sampleGDDJSON} {
		if path := os.Getenv("GAMIDOC_DOC_PREVIEW_" + strings.ToUpper(format)); path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			raw = string(data)
		}
		var doc any = &ReportDoc{}
		if format == ReportFormatGDD {
			doc = &GDDDoc{}
		}
		if err := json.Unmarshal([]byte(raw), doc); err != nil {
			t.Fatal(err)
		}
		html, err := RenderDocumentHTML(doc, format, sampleDesignStatus(), "DES-PREVIEW", nil, true)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dir, format+".html"), []byte(html), 0o644)
		pdf, err := renderer.RenderHTML(context.Background(), html)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, format+".pdf"), pdf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func previewRenderer(t *testing.T) HTMLRenderer {
	t.Helper()
	chrome, ok := pdf.FindChrome(os.Getenv("PDF_CHROME_PATH"))
	if !ok {
		t.Skip("no local Chrome/Edge")
	}
	return pdf.NewChromeHTMLRenderer(chrome, time.Minute)
}

func TestThinReportIsRetriedThenCompletedWithTheData(t *testing.T) {
	thin := `{"title":"A very long title that is really the whole spark of the user written here","subtitle":"","glance":{"forWhom":"Students","whatWeWant":"","motivation":"","howWeKnow":"","mainRisk":"","addressing":""},
	  "loops":{"coreLabel":"","core":{"steps":[],"repeat":""},"longLabel":"","long":{"steps":[],"repeat":""},"link":""},
	  "sections":[{"number":1,"inShort":"","facts":[],"blocks":[{"title":"Constraints","term":"","values":[],"why":"","inTheDesign":[],"numbered":false,"watchOut":"","refs":[]}],"feedback":[],"closing":""}],
	  "coherence":[],"evalIntro":"","evaluation":[],"assumptions":[]}`
	assistant := &scriptedAssistant{answers: map[string]string{"design_document_report": thin}}
	store := objectstore.NewLocalStore(t.TempDir(), "/files/pdfs")
	reports := NewReportService(assistant, nil, store, nil)

	report, err := reports.Generate(context.Background(), "sessions", "s1", sampleDesignStatus(), ReportFormatReport)
	if err != nil || report.Version != ReportFormatReport {
		t.Fatalf("unexpected %+v %v", report, err)
	}
	if len(assistant.requests) != 2 {
		t.Fatalf("a thin report must be asked again once, got %d calls", len(assistant.requests))
	}

	var doc ReportDoc
	_ = json.Unmarshal([]byte(thin), &doc)
	tidyDocument(&doc)
	if doc.Title != "A very long title that is really the whole spark of the user written here" {
		t.Fatalf("the title must never be cut: %q", doc.Title)
	}
	for title, want := range map[string]string{
		"Documento di Game Design — Gioco per l'aderenza alla glicemia": "Gioco per l'aderenza alla glicemia",
		"GreenStep · Game Design Document":                              "GreenStep",
		"GamiDoc GDD: GreenStep":                                        "GreenStep",
		"Design report":                                                 "Design report",
		"GreenStep":                                                     "GreenStep",
	} {
		if got := stripFormatFromTitle(title); got != want {
			t.Fatalf("stripFormatFromTitle(%q) = %q, want %q", title, got, want)
		}
	}
	completed := completeThinReport(&doc, sampleDesignStatus())
	if len(completed.Sections) < 3 || len(completed.Sections[0].Blocks) != 0 || len(completed.Sections[0].Facts) == 0 {
		t.Fatalf("thin sections must get the entered data: %+v", completed.Sections)
	}
}

func TestTidyMovesInlineCitationsToRefs(t *testing.T) {
	doc := &ReportDoc{
		Loops: DocLoops{CoreLabel: "Core loop", LongLabel: "every week"},
		Sections: []ReportSection{{Number: 1, Feedback: []ReportFeedback{{ID: "F1"}}, Blocks: []ReportBlock{{
			Why:    "Habits form over weeks [8][16]. Users differ [3].",
			Values: []string{"Ritual anchors", strings.Repeat("long text ", 10)},
			Refs:   []int{3},
		}}}},
	}
	tidyDocument(doc)
	block := doc.Sections[0].Blocks[0]
	if block.Why != "Habits form over weeks. Users differ." {
		t.Fatalf("citations left in text: %q", block.Why)
	}
	if len(block.Refs) != 3 || !containsInt(block.Refs, 8) || !containsInt(block.Refs, 16) {
		t.Fatalf("unexpected refs %v", block.Refs)
	}
	if len(doc.Sections[0].Feedback) != 0 {
		t.Fatal("feedback cards outside the feedback section must be dropped")
	}
	if len(block.Values) != 2 || len(splitValues(block.Values, true)) != 1 || len(splitValues(block.Values, false)) != 1 {
		t.Fatalf("long values must be kept (as text, not chips): %v", block.Values)
	}
	if doc.Loops.CoreLabel != "" || doc.Loops.LongLabel != "every week" {
		t.Fatalf("unexpected tidy result %+v %+v", block.Values, doc.Loops)
	}
	if !strings.Contains(designDataText(sampleDesignStatus()), "- Retention anchors [choice]: Ritual anchors") {
		t.Fatal("field kinds missing from the prompt")
	}
}

func TestFooterSurvivesAnyTitle(t *testing.T) {
	var doc GDDDoc
	_ = json.Unmarshal([]byte(sampleGDDJSON), &doc)
	doc.Title = `Gioco "salute" — l'aderenza <alla> glicemia; {test}`
	html, err := RenderDocumentHTML(&doc, ReportFormatGDD, sampleDesignStatus(), "DES-1", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "ZgotmplZ") || !strings.Contains(html, `content: "GamiDoc\000020\0000B7\000020Game\000020Design`) {
		t.Fatalf("footer not escaped: %s", html[strings.Index(html, "@bottom-left"):][:200])
	}
}

func TestTitleThatIsOnlyTheDocumentTypeUsesTheSubtitle(t *testing.T) {
	title, subtitle := "Report di design", "Un gioco per la glicemia"
	fixTitle(&title, &subtitle)
	if title != "Un gioco per la glicemia" || subtitle != "" {
		t.Fatalf("unexpected %q / %q", title, subtitle)
	}
	title, subtitle = "GreenStep", "A team challenge"
	fixTitle(&title, &subtitle)
	if title != "GreenStep" || subtitle != "A team challenge" {
		t.Fatalf("a real title must be kept: %q / %q", title, subtitle)
	}
}

func TestMoreTitleVariantsAndLanguageRule(t *testing.T) {
	title, subtitle := "GamiDoc Final Document — Blood glucose daily measurement game", "GamiDoc final design document"
	fixTitle(&title, &subtitle)
	if title != "Blood glucose daily measurement game" || subtitle != "" {
		t.Fatalf("unexpected %q / %q", title, subtitle)
	}

	assistant := &scriptedAssistant{answers: map[string]string{"design_document_gdd": sampleGDDJSON}}
	status := sampleDesignStatus()
	status.Prefill = &PrefillState{Analysis: &ProjectAnalysis{Language: "Italian"}}
	if _, err := GenerateDocument(context.Background(), assistant, ReportFormatGDD, status); err != nil {
		t.Fatal(err)
	}
	request := assistant.requests[0]
	if !strings.HasPrefix(request.System, "LANGUAGE: write every text") || !strings.HasPrefix(request.User, "LANGUAGE:") || !strings.Contains(request.User, "in Italian") {
		t.Fatal("the language rule must open both messages")
	}
}

func TestLoopNumberingIsNotRepeated(t *testing.T) {
	loops := DocLoops{
		CoreLabel: "Core loop: azione breve e ripetibile",
		LongLabel: "every week",
		Core:      DocLoop{Steps: []DocLoopStep{{Title: "Step 1. Trigger"}, {Title: "2. Approach"}, {Title: "Passo 3: Act"}, {Title: "Weekly goal"}}},
	}
	tidyLoops(&loops)
	if loops.CoreLabel != "azione breve e ripetibile" || loops.LongLabel != "every week" {
		t.Fatalf("unexpected labels %q / %q", loops.CoreLabel, loops.LongLabel)
	}
	for i, want := range []string{"Trigger", "Approach", "Act", "Weekly goal"} {
		if got := loops.Core.Steps[i].Title; got != want {
			t.Fatalf("step %d: %q, want %q", i, got, want)
		}
	}
}
