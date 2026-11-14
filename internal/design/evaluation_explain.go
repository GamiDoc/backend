package design

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gamidoc/backend/internal/ai"
)

/* How the UX evaluation report is written.

   The backend builds the whole document from the wizard (ids, names, catalog
   descriptions, which method each instrument belongs to, which goals it covers): the AI
   only writes the explanations, by id, in two smaller calls:
   1. the project context, the goals, the research questions and hypotheses;
   2. the methods, the instruments, how they complement each other, the next steps.
   An explanation the AI leaves out keeps the catalog text, so the document is never
   empty; an answer that explains almost nothing is asked again (ErrInvalidDocument). */

type evalGoalText struct {
	ID      string `json:"id" desc:"the goal id, e.g. G1"`
	Meaning string `json:"meaning" desc:"what the goal means for this kind of system, 2 sentences"`
	WhyHere string `json:"whyHere" desc:"why it matters for THIS project, 2-3 sentences tied to its design choices and users"`
}

type evalHypothesisText struct {
	ID        string `json:"id" desc:"the hypothesis id, e.g. H1"`
	Rationale string `json:"rationale" desc:"why this expectation or threshold, 1-2 sentences"`
	Refs      []int  `json:"refs" desc:"reference ids supporting the rationale; empty if none"`
}

type evalQuestionText struct {
	ID          string   `json:"id" desc:"the question id, e.g. RQ1"`
	Reasoning   string   `json:"reasoning" desc:"what the question is about, why it matters for this design and how the study answers it, 2-4 sentences"`
	Exploratory bool     `json:"exploratory" desc:"true when no hypothesis tests it"`
	Hypotheses  []string `json:"hypotheses" desc:"ids of the hypotheses that test this question; empty if exploratory"`
}

type evalContextAnswer struct {
	Subtitle   string               `json:"subtitle" desc:"1-2 sentences: the system evaluated and the aim of the evaluation"`
	Context    []string             `json:"context" desc:"ONLY the project context: 2-3 paragraphs of plain prose (no headings, no markdown, no lists, no 'KEY: value' lines) about the project, the design choices at stake and what the study wants to find out. Goals, questions and hypotheses go in their own fields"`
	Goals      []evalGoalText       `json:"goals" desc:"one entry for EVERY goal listed, by id"`
	Questions  []evalQuestionText   `json:"questions" desc:"one entry for every research question listed, by id; empty if none"`
	Hypotheses []evalHypothesisText `json:"hypotheses" desc:"one entry for every hypothesis listed, by id; empty if none"`
}

type evalMethodText struct {
	ID      string `json:"id" desc:"the method id, e.g. M1"`
	What    string `json:"what" desc:"what it is and how it runs in this study, 2 sentences"`
	WhyHere string `json:"whyHere" desc:"why it fits this project (stage, sample, time, goals, users), 2-3 sentences"`
	Limits  string `json:"limits" desc:"its main limit here and how it is compensated, 1-2 sentences"`
	Refs    []int  `json:"refs" desc:"reference ids; empty if none"`
}

type evalInstrumentText struct {
	ID        string `json:"id" desc:"the instrument id, e.g. I1"`
	What      string `json:"what" desc:"what it is (items, scale, subscales), 1-2 sentences; no numbers you are not sure of"`
	WhyHere   string `json:"whyHere" desc:"why it was chosen for this project, 2 sentences"`
	Measures  string `json:"measures" desc:"what it measures here, 1-2 sentences"`
	HowToRead string `json:"howToRead" desc:"how to read the results: scoring, reference values or thresholds when well established, 2-3 sentences"`
	Limits    string `json:"limits" desc:"its main limit here, 1 sentence"`
	Refs      []int  `json:"refs" desc:"reference ids for the instrument; empty if none in the catalogue"`
}

type evalMethodsAnswer struct {
	Methods     []evalMethodText     `json:"methods" desc:"one entry for EVERY method listed, by id"`
	Instruments []evalInstrumentText `json:"instruments" desc:"one entry for EVERY instrument listed, by id"`
	Coverage    []string             `json:"coverage" desc:"1-2 paragraphs: how the choices complement each other (each goal seen from more than one angle) and what remains fragile"`
	NextSteps   []string             `json:"nextSteps" desc:"the next steps listed, each rephrased into one clear, concrete sentence for this study"`
}

// evaluationSkeleton is the document built from the data, with the hypotheses listed apart
// (the AI tells which question each one tests).
func evaluationSkeleton(status Status) (*EvalReportDoc, []EvalHypothesis) {
	doc := fallbackEvaluation(status)
	var hypotheses []EvalHypothesis
	for i := range doc.Questions {
		hypotheses = append(hypotheses, doc.Questions[i].Hypotheses...)
		doc.Questions[i].Hypotheses = nil
	}
	return doc, hypotheses
}

func generateEvaluationDocument(ctx context.Context, assistant ai.Assistant, status Status) (*EvalReportDoc, error) {
	doc, hypotheses := evaluationSkeleton(status)
	system := evaluationSystemPrompt
	languageRule := ""
	if language := documentLanguage(status); language != "" {
		languageRule = "\n\nLANGUAGE: write every text in " + language + "; keep goal, method and instrument names as listed."
		system = strings.TrimPrefix(languageRule, "\n\n") + "\n\n" + system
	}
	header := evaluationHeader(status)

	// 1. Context, goals, questions and hypotheses.
	var first evalContextAnswer
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n## Explain these goals (by id)\n")
	for _, goal := range doc.Goals {
		fmt.Fprintf(&b, "%s %s [%s]: %s\n", goal.ID, goal.Name, goal.Category, goal.Meaning)
	}
	if len(doc.Questions) > 0 {
		b.WriteString("\n## Research questions (by id)\n")
		for _, question := range doc.Questions {
			fmt.Fprintf(&b, "%s %s\n", question.ID, question.Question)
		}
	}
	if len(hypotheses) > 0 {
		b.WriteString("\n## Hypotheses (by id)\n")
		for _, hypothesis := range hypotheses {
			fmt.Fprintf(&b, "%s %s\n", hypothesis.ID, hypothesis.Text)
		}
	}
	b.WriteString("\nNOW: write the subtitle, the project context, and the explanation of every goal, research question and hypothesis above, by id." + languageRule)
	if err := askEvaluationPart(ctx, assistant, "evaluation_report_context", system, b.String(), evalContextAnswer{}, &first); err != nil {
		return nil, err
	}

	// 2. Methods, instruments, coverage and next steps.
	var second evalMethodsAnswer
	b.Reset()
	b.WriteString(header)
	b.WriteString("\n## Goals\n")
	for _, goal := range doc.Goals {
		fmt.Fprintf(&b, "%s %s\n", goal.ID, goal.Name)
	}
	b.WriteString("\n## Explain these methods (by id)\n")
	for _, method := range doc.Methods {
		fmt.Fprintf(&b, "%s %s: %s Serves: %s.\n", method.ID, method.Name, method.What, strings.Join(method.Goals, ", "))
	}
	b.WriteString("\n## Explain these instruments (by id)\n")
	for _, instrument := range doc.Instrs {
		fmt.Fprintf(&b, "%s %s: %s Used in: %s. Covers: %s.\n", instrument.ID, instrument.Name, instrument.What,
			strings.Join(instrument.Methods, ", "), strings.Join(instrument.Covers, ", "))
	}
	if len(doc.NextSteps) > 0 {
		b.WriteString("\n## Next steps entered\n")
		for _, step := range doc.NextSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	}
	b.WriteString("\nNOW: explain every method and instrument above, by id, then how they complement each other, then the next steps." + languageRule)
	if err := askEvaluationPart(ctx, assistant, "evaluation_report_methods", system, b.String(), evalMethodsAnswer{}, &second); err != nil {
		return nil, err
	}

	explained := mergeEvaluationTexts(doc, hypotheses, first, second)
	total := len(doc.Goals) + len(doc.Methods) + len(doc.Instrs)
	if total > 0 && explained*2 < total {
		return nil, fmt.Errorf("%w: %d of %d choices explained", ErrInvalidDocument, explained, total)
	}
	doc.Context, doc.ContextFacts = splitFieldLines(doc.Context)
	return doc, nil
}

// evaluationHeader: the project, the design and the evaluation set-up, given to both calls.
func evaluationHeader(status Status) string {
	var b strings.Builder
	if title := strings.TrimSpace(status.Title); title != "" {
		fmt.Fprintf(&b, "PROJECT NAME: %s. Always call the system by this name (not by names that appear in the design text).\n\n", title)
	}
	if status.HasContent() || strings.TrimSpace(status.Spark) != "" {
		b.WriteString(designDataText(status))
		b.WriteString("\n")
	}
	b.WriteString(evaluationDataText(status.Evaluation))
	b.WriteString("\n")
	b.WriteString(evaluationReferencesText())
	return b.String()
}

func askEvaluationPart(ctx context.Context, assistant ai.Assistant, name, system, user string, shape any, target any) error {
	schema, err := json.Marshal(docSchema(shape))
	if err != nil {
		return err
	}
	raw, err := assistant.CompleteJSON(ctx, ai.JSONRequest{Name: name, System: system, User: user, Schema: schema})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("unparseable answer: %w", err)
	}
	return nil
}

// usable is a text the AI really wrote (not empty, not a placeholder).
func usable(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return lower != "" && lower != "tbd" && !strings.Contains(lower, "placeholder") && !strings.Contains(lower, "lorem ipsum")
}

// mergeEvaluationTexts puts the AI's explanations on the skeleton by id and returns how
// many goals, methods and instruments got an explanation.
func mergeEvaluationTexts(doc *EvalReportDoc, hypotheses []EvalHypothesis, first evalContextAnswer, second evalMethodsAnswer) int {
	// Citation numbers written in the text are dropped: sources are listed by their refs.
	set := func(target *string, text string) bool {
		if usable(text) {
			*target = strings.TrimSpace(contextCitation.ReplaceAllString(text, ""))
			return true
		}
		return false
	}
	explained := 0

	set(&doc.Subtitle, first.Subtitle)
	if context := cleanContext(first.Context); len(context) > 0 {
		doc.Context = context
	}

	goals := map[string]evalGoalText{}
	for _, goal := range first.Goals {
		goals[strings.ToUpper(strings.TrimSpace(goal.ID))] = goal
	}
	for i := range doc.Goals {
		if text, ok := goals[doc.Goals[i].ID]; ok {
			set(&doc.Goals[i].Meaning, text.Meaning)
			if set(&doc.Goals[i].WhyHere, text.WhyHere) {
				explained++
			}
		}
	}

	hypothesisTexts := map[string]evalHypothesisText{}
	for _, hypothesis := range first.Hypotheses {
		hypothesisTexts[strings.ToUpper(strings.TrimSpace(hypothesis.ID))] = hypothesis
	}
	byID := map[string]EvalHypothesis{}
	for _, hypothesis := range hypotheses {
		if text, ok := hypothesisTexts[hypothesis.ID]; ok {
			set(&hypothesis.Rationale, text.Rationale)
			hypothesis.Refs = text.Refs
		}
		byID[hypothesis.ID] = hypothesis
	}
	questions := map[string]evalQuestionText{}
	for _, question := range first.Questions {
		questions[strings.ToUpper(strings.TrimSpace(question.ID))] = question
	}
	assigned := map[string]bool{}
	for i := range doc.Questions {
		text, ok := questions[doc.Questions[i].ID]
		if !ok {
			continue
		}
		set(&doc.Questions[i].Reasoning, text.Reasoning)
		doc.Questions[i].Exploratory = text.Exploratory
		for _, id := range text.Hypotheses {
			id = strings.ToUpper(strings.TrimSpace(id))
			if hypothesis, ok := byID[id]; ok && !assigned[id] {
				doc.Questions[i].Hypotheses = append(doc.Questions[i].Hypotheses, hypothesis)
				assigned[id] = true
			}
		}
	}
	// Hypotheses no question claimed go under the first question that is not exploratory.
	for _, hypothesis := range hypotheses {
		if assigned[hypothesis.ID] || len(doc.Questions) == 0 {
			continue
		}
		target := 0
		for i, question := range doc.Questions {
			if !question.Exploratory {
				target = i
				break
			}
		}
		doc.Questions[target].Hypotheses = append(doc.Questions[target].Hypotheses, byID[hypothesis.ID])
		doc.Questions[target].Exploratory = false
	}

	methods := map[string]evalMethodText{}
	for _, method := range second.Methods {
		methods[strings.ToUpper(strings.TrimSpace(method.ID))] = method
	}
	for i := range doc.Methods {
		if text, ok := methods[doc.Methods[i].ID]; ok {
			set(&doc.Methods[i].What, text.What)
			set(&doc.Methods[i].Limits, text.Limits)
			doc.Methods[i].Refs = text.Refs
			if set(&doc.Methods[i].WhyHere, text.WhyHere) {
				explained++
			}
		}
	}

	instruments := map[string]evalInstrumentText{}
	for _, instrument := range second.Instruments {
		instruments[strings.ToUpper(strings.TrimSpace(instrument.ID))] = instrument
	}
	for i := range doc.Instrs {
		if text, ok := instruments[doc.Instrs[i].ID]; ok {
			set(&doc.Instrs[i].What, text.What)
			set(&doc.Instrs[i].Measures, text.Measures)
			set(&doc.Instrs[i].HowToRead, text.HowToRead)
			set(&doc.Instrs[i].Limits, text.Limits)
			doc.Instrs[i].Refs = text.Refs
			if set(&doc.Instrs[i].WhyHere, text.WhyHere) {
				explained++
			}
		}
	}

	var coverage []string
	for _, paragraph := range second.Coverage {
		if usable(paragraph) {
			coverage = append(coverage, strings.TrimSpace(paragraph))
		}
	}
	if len(coverage) > 0 {
		doc.Coverage = coverage
	}
	var steps []string
	for _, step := range second.NextSteps {
		if usable(step) {
			steps = append(steps, strings.TrimSpace(step))
		}
	}
	if len(steps) > 0 {
		doc.NextSteps = steps
	}
	return explained
}

// contextStop matches the headings of the other parts of the document, which some models
// write inside the context ("## Goals", "### G1 …", "## Metodi").
var contextStop = regexp.MustCompile(`(?i)goal|obiettiv|question|domand|hypothes|ipotes|method|metod|instrument|strument|coverage|complement|next step|prossim`)

// contextCitation matches citation numbers written in the text ("[3]", "[2, 7]").
var contextCitation = regexp.MustCompile(`\s*\[\d+(?:\s*,\s*\d+)*\]`)

// cleanContext keeps only the project context: the model sometimes writes the whole
// document as markdown in it. Headings are dropped, the subtitle part is skipped, it stops
// at the first heading of another part, and keeps at most 4 paragraphs.
func cleanContext(paragraphs []string) []string {
	var context []string
	skip := false
	for _, paragraph := range paragraphs {
		text := strings.TrimSpace(paragraph)
		if strings.HasPrefix(text, "#") {
			heading := strings.ToLower(strings.TrimLeft(text, "# "))
			if contextStop.MatchString(heading) && len(context) > 0 {
				break
			}
			skip = strings.Contains(heading, "subtitle") || strings.Contains(heading, "sottotitolo") || contextStop.MatchString(heading)
			continue
		}
		if skip || !usable(text) {
			continue
		}
		context = append(context, strings.TrimSpace(contextCitation.ReplaceAllString(text, "")))
		if len(context) == 4 {
			break
		}
	}
	return context
}
