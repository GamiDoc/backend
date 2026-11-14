package design

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gamidoc/backend/internal/evaluation/catalog"
	"github.com/gamidoc/backend/internal/wizard"
)

/* Reasoned UX evaluation report ("evaluation" format).

   Written by the AI from the evaluation wizard of the project (goals, stage, constraints,
   research questions and hypotheses, methods, instruments, next steps) and, when there is
   one, from the project's design. Every goal, method and instrument is explained with the
   same outline: what it is, why it was chosen for this project, what it measures, how to
   read the results, limits, sources. Sources come only from the curated catalogues. The
   job and the file are handled like the design documents (Status.ReportJob, reports list).

   The evaluation data travels with the job in Status.Evaluation (never stored). */

const ReportFormatEvaluation = "evaluation"

type EvalGoal struct {
	ID       string `json:"id" desc:"G1, G2, ..."`
	Name     string `json:"name" desc:"the goal exactly as chosen"`
	Category string `json:"category" desc:"its category in the GamiDoc UX model, e.g. Pragmatic UX"`
	Meaning  string `json:"meaning" desc:"what the goal means, 1-2 sentences"`
	WhyHere  string `json:"whyHere" desc:"why it matters for THIS project, 1-2 sentences tied to the design"`
}

type EvalHypothesis struct {
	ID        string `json:"id" desc:"H1, H2, ... numbered across the whole document"`
	Text      string `json:"text" desc:"the hypothesis, with its threshold when entered"`
	Rationale string `json:"rationale" desc:"why this threshold or expectation, 1 sentence; empty if none"`
	Refs      []int  `json:"refs" desc:"reference ids supporting the rationale; empty if none"`
}

type EvalQuestion struct {
	ID          string           `json:"id" desc:"RQ1, RQ2, ..."`
	Question    string           `json:"question"`
	Exploratory bool             `json:"exploratory" desc:"true when the question has no hypothesis"`
	Reasoning   string           `json:"reasoning" desc:"what the question is about and how it will be answered, 2-3 sentences"`
	Hypotheses  []EvalHypothesis `json:"hypotheses"`
}

type EvalMethod struct {
	ID      string   `json:"id" desc:"M1, M2, ..."`
	Name    string   `json:"name" desc:"the method exactly as chosen"`
	Goals   []string `json:"goals" desc:"ids of the goals it serves, e.g. G1"`
	What    string   `json:"what" desc:"what it is and how it runs in this study, 1-2 sentences"`
	WhyHere string   `json:"whyHere" desc:"why it fits this project (stage, sample, time, goals), 1-2 sentences"`
	Limits  string   `json:"limits" desc:"its main limit here, 1 sentence"`
	Refs    []int    `json:"refs" desc:"reference ids; empty if none"`
}

type EvalInstrument struct {
	ID        string   `json:"id" desc:"I1, I2, ..."`
	Name      string   `json:"name" desc:"the instrument exactly as chosen"`
	Methods   []string `json:"methods" desc:"ids of the methods it is used in, e.g. M2"`
	Covers    []string `json:"covers" desc:"ids of the goals it covers fully"`
	Partial   []string `json:"partial" desc:"ids of the goals it covers only in part; empty if none"`
	What      string   `json:"what" desc:"what it is (items, scale, subscales), 1-2 sentences; no numbers you are not sure of"`
	WhyHere   string   `json:"whyHere" desc:"why it was chosen for this project, 1-2 sentences"`
	Measures  string   `json:"measures" desc:"what it measures, 1 sentence"`
	HowToRead string   `json:"howToRead" desc:"how to read the results (scoring, reference values or thresholds when well established), 1-3 sentences"`
	Limits    string   `json:"limits" desc:"its main limit here, 1 sentence"`
	Refs      []int    `json:"refs" desc:"reference ids for the instrument; empty if none in the catalogue"`
}

type EvalReportDoc struct {
	Title    string   `json:"title" desc:"the project name"`
	Subtitle string   `json:"subtitle" desc:"1-2 sentences: the system evaluated and the aim of the evaluation"`
	Context  []string `json:"context" desc:"2-3 paragraphs of flowing prose (never a list of fields or 'KEY: value' lines): the project, the design choices at stake and what the study wants to find out"`
	// ContextFacts: "KEY: value" lines the model wrote in Context, shown as a table.
	ContextFacts []ReportFact     `json:"-"`
	Goals        []EvalGoal       `json:"goals"`
	Questions    []EvalQuestion   `json:"questions" desc:"one per research question entered; empty if none"`
	Methods      []EvalMethod     `json:"methods"`
	Instrs       []EvalInstrument `json:"instruments"`
	Coverage     []string         `json:"coverage" desc:"1-2 paragraphs: how the choices complement each other (each goal seen from more than one angle) and what is left out or fragile"`
	NextSteps    []string         `json:"nextSteps" desc:"the next steps, as entered (rephrased only to be clear)"`
}

// Evaluation references: sources of the methods and instruments of the catalog. Only
// well-established original publications; instruments without one are left uncited.
var evaluationReferences = []DocReference{
	{101, "SUS System Usability Scale (original)", "Brooke, J. (1996). SUS: A \"quick and dirty\" usability scale. In P. W. Jordan, B. Thomas, B. A. Weerdmeester, & I. L. McClelland (Eds.), Usability Evaluation in Industry (pp. 189–194). Taylor & Francis."},
	{102, "usability benchmarks and statistics (SUS average 68)", "Sauro, J., & Lewis, J. R. (2016). Quantifying the User Experience: Practical Statistics for User Research (2nd ed.). Morgan Kaufmann."},
	{103, "UMUX-Lite", "Lewis, J. R., Utesch, B. S., & Maher, D. E. (2013). UMUX-LITE: When there's no time for the SUS. Proceedings of CHI 2013, 2099–2102. https://doi.org/10.1145/2470654.2481287"},
	{104, "AttrakDiff (pragmatic and hedonic quality)", "Hassenzahl, M., Burmester, M., & Koller, F. (2003). AttrakDiff: Ein Fragebogen zur Messung wahrgenommener hedonischer und pragmatischer Qualität. In Mensch & Computer 2003 (pp. 187–196). Vieweg+Teubner. https://doi.org/10.1007/978-3-322-80058-9_19"},
	{105, "UEQ User Experience Questionnaire", "Laugwitz, B., Held, T., & Schrepp, M. (2008). Construction and evaluation of a user experience questionnaire. In HCI and Usability for Education and Work (USAB 2008), LNCS 5298, 63–76. https://doi.org/10.1007/978-3-540-89350-9_6"},
	{106, "GEQ Game Experience Questionnaire", "IJsselsteijn, W. A., de Kort, Y. A. W., & Poels, K. (2013). The Game Experience Questionnaire. Technische Universiteit Eindhoven."},
	{107, "SAM Self-Assessment Manikin", "Bradley, M. M., & Lang, P. J. (1994). Measuring emotion: The Self-Assessment Manikin and the semantic differential. Journal of Behavior Therapy and Experimental Psychiatry, 25(1), 49–59. https://doi.org/10.1016/0005-7916(94)90063-9"},
	{108, "PANAS positive and negative affect", "Watson, D., Clark, L. A., & Tellegen, A. (1988). Development and validation of brief measures of positive and negative affect: The PANAS scales. Journal of Personality and Social Psychology, 54(6), 1063–1070. https://doi.org/10.1037/0022-3514.54.6.1063"},
	{109, "NASA-TLX workload", "Hart, S. G., & Staveland, L. E. (1988). Development of NASA-TLX (Task Load Index): Results of empirical and theoretical research. Advances in Psychology, 52, 139–183. https://doi.org/10.1016/S0166-4115(08)62386-9"},
	{110, "PENS player need satisfaction (competence, autonomy, relatedness)", "Ryan, R. M., Rigby, C. S., & Przybylski, A. (2006). The motivational pull of video games: A self-determination theory approach. Motivation and Emotion, 30(4), 344–360. https://doi.org/10.1007/s11031-006-9051-8"},
	{111, "IMI Intrinsic Motivation Inventory", "McAuley, E., Duncan, T., & Tammen, V. V. (1989). Psychometric properties of the Intrinsic Motivation Inventory in a competitive sport setting: A confirmatory factor analysis. Research Quarterly for Exercise and Sport, 60(1), 48–58. https://doi.org/10.1080/02701367.1989.10607413"},
	{112, "FSS-2 Flow State Scale", "Jackson, S. A., & Eklund, R. C. (2002). Assessing flow in physical activity: The Flow State Scale-2 and Dispositional Flow Scale-2. Journal of Sport and Exercise Psychology, 24(2), 133–150. https://doi.org/10.1123/jsep.24.2.133"},
	{113, "EGameFlow enjoyment of e-learning games", "Fu, F.-L., Su, R.-C., & Yu, S.-C. (2009). EGameFlow: A scale to measure learners' enjoyment of e-learning games. Computers & Education, 52(1), 101–112. https://doi.org/10.1016/j.compedu.2008.07.004"},
	{114, "UES User Engagement Scale (and short form)", "O'Brien, H. L., Cairns, P., & Hall, M. (2018). A practical approach to measuring user engagement with the refined user engagement scale (UES) and new UES short form. International Journal of Human-Computer Studies, 112, 28–39. https://doi.org/10.1016/j.ijhcs.2018.01.004"},
	{115, "PXI Player Experience Inventory (basis of miniPXI)", "Vanden Abeele, V., Spiel, K., Nacke, L., Johnson, D., & Gerling, K. (2020). Development and validation of the player experience inventory: A scale to measure player experiences at the level of functional and psychosocial consequences. International Journal of Human-Computer Studies, 135, 102370. https://doi.org/10.1016/j.ijhcs.2019.102370"},
	{116, "think-aloud / verbal protocols", "Ericsson, K. A., & Simon, H. A. (1993). Protocol Analysis: Verbal Reports as Data (Rev. ed.). MIT Press."},
	{117, "heuristic evaluation", "Nielsen, J., & Molich, R. (1990). Heuristic evaluation of user interfaces. Proceedings of CHI '90, 249–256. https://doi.org/10.1145/97243.97281"},
	{118, "heuristics for game playability", "Desurvire, H., Caplan, M., & Toth, J. A. (2004). Using heuristics to evaluate the playability of games. CHI '04 Extended Abstracts, 1509–1512. https://doi.org/10.1145/985921.986102"},
	{119, "diary studies", "Bolger, N., Davis, A., & Rafaeli, E. (2003). Diary methods: Capturing life as it is lived. Annual Review of Psychology, 54, 579–616. https://doi.org/10.1146/annurev.psych.54.101601.145030"},
	{120, "thematic analysis of qualitative data", "Braun, V., & Clarke, V. (2006). Using thematic analysis in psychology. Qualitative Research in Psychology, 3(2), 77–101. https://doi.org/10.1191/1478088706qp063oa"},
	{121, "semi-structured interview guides", "Kallio, H., Pietilä, A.-M., Johnson, M., & Kangasniemi, M. (2016). Systematic methodological review: Developing a framework for a qualitative semi-structured interview guide. Journal of Advanced Nursing, 72(12), 2954–2965. https://doi.org/10.1111/jan.13031"},
	{122, "focus groups", "Krueger, R. A., & Casey, M. A. (2015). Focus Groups: A Practical Guide for Applied Research (5th ed.). SAGE."},
}

func evaluationReferencesText() string {
	var b strings.Builder
	b.WriteString("Reference catalogue (cite by id only; leave refs empty when none fits):\n")
	for _, reference := range append(append([]DocReference{}, referenceCatalogue...), evaluationReferences...) {
		fmt.Fprintf(&b, "%d. %s\n", reference.ID, reference.Topic)
	}
	return b.String()
}

const evaluationSystemPrompt = `You are a UX researcher writing the reasoned evaluation plan of a gamified system, made with GamiDoc (a design system and an evaluation wizard).
Rules:
- Ground everything in the evaluation data and the design you are given. Keep goals, methods and instruments exactly as chosen; never add, drop or swap any.
- Explain every goal, method and instrument for THIS project: tie the reasons to the design choices (mechanics, users, behaviour to change, risks).
- For instruments, describe items, scales and scoring only when you are sure of them; never invent numbers. Use well-established reference values only (e.g. SUS average 68).
- Cite references ONLY by their id from the catalogue, and only where they really support the sentence. Never write citations in the text yourself.
- Write clearly for a reader who did not take part in the planning. Short sentences, no filler.
- Write in the language of the data (the language of the free-text fields and of the design).
- All the data you need is in the message. Write the finished document now: never ask for more data, never answer with a template, an outline or placeholders, never talk about yourself or the task.`

// evaluationUserText describes the evaluation (and the design, when there is one) for the AI.
func evaluationUserText(status Status) string {
	var b strings.Builder
	if status.HasContent() || strings.TrimSpace(status.Spark) != "" {
		b.WriteString(designDataText(status))
		b.WriteString("\n")
	}
	b.WriteString(evaluationDataText(status.Evaluation))
	b.WriteString("\n")
	b.WriteString(evaluationReferencesText())
	b.WriteString(`
Write the reasoned evaluation plan: the project and what the study wants to find out; each goal (what it means, why here); the research questions with their hypotheses and the reasoning that links them (when entered); each method (what it is, why here, limits); each instrument (what it is, why here, what it measures, how to read the results, limits, sources); how the choices complement each other and what is left out; the next steps.
Number goals G1…, questions RQ1…, hypotheses H1… (across the document), methods M1…, instruments I1…, and use these ids to cross-reference them.`)
	return b.String()
}

// customQuestionnaire is a custom construct of step 3 ("Custom questionnaire").
type customQuestionnaire struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Goals       []string `json:"goals"`
}

type evaluationData struct {
	step1       wizard.Step1Data
	methods     []string
	instruments []string
	customs     map[string]customQuestionnaire // by name
	nextSteps   []string
	notes       string
}

func decodeEvaluation(status *wizard.Status) evaluationData {
	var data evaluationData
	if status == nil {
		return data
	}
	_ = json.Unmarshal(status.Steps["1"], &data.step1)
	var step2 wizard.Step2Data
	_ = json.Unmarshal(status.Steps["2"], &step2)
	data.methods = step2.SelectedMethods
	var step3 struct {
		SelectedInstruments []string              `json:"selectedInstruments"`
		CustomConstructs    []customQuestionnaire `json:"customConstructs"`
	}
	_ = json.Unmarshal(status.Steps["3"], &step3)
	data.instruments = step3.SelectedInstruments
	data.customs = map[string]customQuestionnaire{}
	for _, custom := range step3.CustomConstructs {
		data.customs[custom.Name] = custom
	}
	var step4 wizard.Step4Data
	_ = json.Unmarshal(status.Steps["4"], &step4)
	data.nextSteps = step4.NextSteps
	data.notes = step4.Notes
	return data
}

func evaluationDataText(status *wizard.Status) string {
	data := decodeEvaluation(status)
	c := catalog.Get()
	var b strings.Builder
	b.WriteString("## UX evaluation entered in GamiDoc\n")
	if text := strings.TrimSpace(data.step1.ProjectType); text != "" {
		fmt.Fprintf(&b, "System evaluated: %s\n", text)
	}
	b.WriteString("Evaluation goals:\n")
	for _, name := range data.step1.EvaluationGoals {
		if goal, ok := c.Goal(name); ok {
			fmt.Fprintf(&b, "- %s [%s]: %s\n", goal.Name, goal.Category, goal.Description)
		} else {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	fmt.Fprintf(&b, "Development stage: %s\n", data.step1.DevelopmentStage)
	fmt.Fprintf(&b, "Participants: %s\n", data.step1.Participants)
	if data.step1.Accessibility != "" {
		fmt.Fprintf(&b, "Participant access: %s\n", data.step1.Accessibility)
	}
	if data.step1.Time != "" {
		fmt.Fprintf(&b, "Study duration: %s\n", data.step1.Time)
	}
	if len(data.step1.ExtraConstraints) > 0 {
		fmt.Fprintf(&b, "Other constraints: %s\n", strings.Join(data.step1.ExtraConstraints, ", "))
	}
	if data.step1.ResearchEnabled {
		if data.step1.ResearchObjective != "" {
			fmt.Fprintf(&b, "Research objective: %s\n", data.step1.ResearchObjective)
		}
		for i, question := range data.step1.ResearchQuestions {
			fmt.Fprintf(&b, "Research question %d: %s\n", i+1, question)
		}
		for i, hypothesis := range data.step1.Hypotheses {
			fmt.Fprintf(&b, "Hypothesis %d: %s\n", i+1, hypothesis)
		}
	}
	b.WriteString("Methods:\n")
	for _, name := range data.methods {
		if method, ok := c.Method(name); ok {
			fmt.Fprintf(&b, "- %s (%s): %s\n", method.Name, method.Type, method.Description)
		} else {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	b.WriteString("Instruments:\n")
	for _, name := range data.instruments {
		if instrument, ok := c.Instrument(name); ok {
			fmt.Fprintf(&b, "- %s (%s): %s Covers: %s.\n", instrument.Name, instrument.Type, instrument.Description, strings.Join(instrument.CoversGoals, ", "))
		} else if custom, ok := data.customs[name]; ok {
			fmt.Fprintf(&b, "- %s (custom questionnaire written for this study): %s Covers: %s.\n", custom.Name, custom.Description, strings.Join(custom.Goals, ", "))
		} else {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	if len(data.nextSteps) > 0 {
		b.WriteString("Next steps:\n")
		for _, step := range data.nextSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	}
	if strings.TrimSpace(data.notes) != "" {
		fmt.Fprintf(&b, "Notes: %s\n", data.notes)
	}
	return b.String()
}

// fallbackEvaluation builds the document from the entered data and the catalog
// descriptions only (no AI): what each goal, method and instrument is, without reasons.
func fallbackEvaluation(status Status) *EvalReportDoc {
	data := decodeEvaluation(status.Evaluation)
	c := catalog.Get()
	doc := &EvalReportDoc{Subtitle: strings.TrimSpace(data.step1.ProjectType), NextSteps: data.nextSteps}
	if objective := strings.TrimSpace(data.step1.ResearchObjective); objective != "" {
		doc.Context = []string{objective}
	}
	goalIDs := map[string]string{}
	for i, name := range data.step1.EvaluationGoals {
		id := fmt.Sprintf("G%d", i+1)
		goalIDs[name] = id
		goal := EvalGoal{ID: id, Name: name}
		if found, ok := c.Goal(name); ok {
			goal.Category, goal.Meaning = found.Category, found.Description
		}
		doc.Goals = append(doc.Goals, goal)
	}
	if data.step1.ResearchEnabled {
		for i, question := range data.step1.ResearchQuestions {
			doc.Questions = append(doc.Questions, EvalQuestion{ID: fmt.Sprintf("RQ%d", i+1), Question: question})
		}
		if len(doc.Questions) > 0 {
			for i, hypothesis := range data.step1.Hypotheses {
				doc.Questions[0].Hypotheses = append(doc.Questions[0].Hypotheses, EvalHypothesis{ID: fmt.Sprintf("H%d", i+1), Text: hypothesis})
			}
		}
	}
	methodIDs := map[string]string{}
	for i, name := range data.methods {
		id := fmt.Sprintf("M%d", i+1)
		method := EvalMethod{ID: id, Name: name}
		if found, ok := c.Method(name); ok {
			methodIDs[found.ID] = id
			method.What = found.Description
			for _, goal := range found.Goals {
				if goalID, ok := goalIDs[goal]; ok {
					method.Goals = append(method.Goals, goalID)
				}
			}
		}
		doc.Methods = append(doc.Methods, method)
	}
	for i, name := range data.instruments {
		instrument := EvalInstrument{ID: fmt.Sprintf("I%d", i+1), Name: name}
		if found, ok := c.Instrument(name); ok {
			instrument.What = found.Description
			for _, methodID := range found.ForMethodIDs {
				if id, ok := methodIDs[methodID]; ok {
					instrument.Methods = append(instrument.Methods, id)
				}
			}
			for _, goal := range found.CoversGoals {
				if goalID, ok := goalIDs[goal]; ok {
					instrument.Covers = append(instrument.Covers, goalID)
				}
			}
		} else if custom, ok := data.customs[name]; ok {
			instrument.What = "Custom questionnaire written for this study. " + custom.Description
			for _, goal := range custom.Goals {
				if goalID, ok := goalIDs[goal]; ok {
					instrument.Covers = append(instrument.Covers, goalID)
				}
			}
		}
		doc.Instrs = append(doc.Instrs, instrument)
	}
	return doc
}

func evaluationContents(doc *EvalReportDoc) []string {
	var contents []string
	for _, entry := range []struct {
		name    string
		present bool
	}{
		{"The project and what we want to find out", len(doc.Context)+len(doc.ContextFacts) > 0},
		{"The goals and what they mean", len(doc.Goals) > 0},
		{"Questions, hypotheses and the reasoning behind them", len(doc.Questions) > 0},
		{"The methods, explained", len(doc.Methods) > 0},
		{"The instruments, explained", len(doc.Instrs) > 0},
		{"How the choices complement each other", len(doc.Coverage) > 0},
		{"Next steps", len(doc.NextSteps) > 0},
	} {
		if entry.present {
			contents = append(contents, entry.name)
		}
	}
	return contents
}

func evaluationCited(doc *EvalReportDoc) []int {
	var cited []int
	for _, question := range doc.Questions {
		for _, hypothesis := range question.Hypotheses {
			cited = append(cited, hypothesis.Refs...)
		}
	}
	for _, method := range doc.Methods {
		cited = append(cited, method.Refs...)
	}
	for _, instrument := range doc.Instrs {
		cited = append(cited, instrument.Refs...)
	}
	return cited
}

func renderEvaluationPlain(w *plainWriter, doc *EvalReportDoc, format, documentID string) {
	w.cover(doc.Title, ReportFormatTitle(format), doc.Subtitle, documentID)
	if len(doc.Context) > 0 {
		w.heading("The project and what we want to find out")
		for _, paragraph := range doc.Context {
			w.paragraph(paragraph)
		}
	}
	if len(doc.Goals) > 0 {
		w.heading("The goals and what they mean")
		for _, goal := range doc.Goals {
			w.subheading(goal.ID + " " + goal.Name)
			w.paragraph(goal.Meaning)
			w.paragraph(goal.WhyHere)
		}
	}
	if len(doc.Questions) > 0 {
		w.heading("Questions, hypotheses and reasoning")
		for _, question := range doc.Questions {
			w.subheading(question.ID + " " + question.Question)
			w.paragraph(question.Reasoning)
			for _, hypothesis := range question.Hypotheses {
				w.pair(hypothesis.ID, hypothesis.Text)
			}
		}
	}
	if len(doc.Methods) > 0 {
		w.heading("The methods, explained")
		for _, method := range doc.Methods {
			w.subheading(method.ID + " " + method.Name)
			w.pair("What it is", method.What)
			w.pair("Why here", method.WhyHere)
			w.pair("Limits", method.Limits)
		}
	}
	if len(doc.Instrs) > 0 {
		w.heading("The instruments, explained")
		for _, instrument := range doc.Instrs {
			w.subheading(instrument.ID + " " + instrument.Name)
			w.pair("What it is", instrument.What)
			w.pair("Why here", instrument.WhyHere)
			w.pair("What it measures", instrument.Measures)
			w.pair("How to read the results", instrument.HowToRead)
			w.pair("Limits", instrument.Limits)
		}
	}
	if len(doc.Coverage) > 0 {
		w.heading("How the choices complement each other")
		for _, paragraph := range doc.Coverage {
			w.paragraph(paragraph)
		}
	}
	if len(doc.NextSteps) > 0 {
		w.heading("Next steps")
		w.list(doc.NextSteps)
	}
}

// evaluationCoverFacts are the plan details printed on the cover.
func evaluationCoverFacts(doc *EvalReportDoc, status *wizard.Status, documentID, date string) []ReportFact {
	data := decodeEvaluation(status)
	facts := []ReportFact{{Label: "Plan ID", Value: documentID}, {Label: "Date", Value: date}}
	add := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			facts = append(facts, ReportFact{Label: label, Value: value})
		}
	}
	add("Development stage", data.step1.DevelopmentStage)
	add("Participants", data.step1.Participants)
	add("Study duration", data.step1.Time)
	add("Scope", fmt.Sprintf("%d goals · %d methods · %d instruments", len(doc.Goals), len(doc.Methods), len(doc.Instrs)))
	return facts
}

// fieldLine matches a "KEY: value" line ("APP_NAME: Spark", "Development stage: …").
var fieldLine = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z_ /]{1,40}):\s+(\S.*)$`)

// splitFieldLines separates prose paragraphs from "KEY: value" lines, which some models
// write instead of paragraphs; the lines become label/value facts ("APP_NAME" → "App name").
// An entry is taken as fields when every line is "key: value" and either the keys are in
// capitals (APP_NAME) or there are several lines; otherwise it stays prose.
func splitFieldLines(paragraphs []string) ([]string, []ReportFact) {
	var prose []string
	var facts []ReportFact
	for _, paragraph := range paragraphs {
		text := strings.TrimSpace(paragraph)
		if text == "" {
			continue
		}
		lines := strings.Split(text, "\n")
		var found []ReportFact
		capitals := true
		for _, line := range lines {
			match := fieldLine.FindStringSubmatch(line)
			if match == nil {
				found = nil
				break
			}
			if strings.ToUpper(match[1]) != match[1] {
				capitals = false
			}
			found = append(found, ReportFact{Label: fieldLabel(match[1]), Value: strings.TrimSpace(match[2])})
		}
		if found == nil || (!capitals && len(found) < 2) {
			prose = append(prose, text)
			continue
		}
		facts = append(facts, found...)
	}
	return prose, facts
}

// fieldLabel turns "APP_NAME" or "app name" into "App name".
func fieldLabel(key string) string {
	label := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(key, "_", " ")))
	if label == "" {
		return key
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

// ErrInvalidDocument: the AI answered off-topic (a template, placeholders, other choices
// than the ones entered); the document is asked again, then built from the data.
var ErrInvalidDocument = errors.New("the AI returned an invalid document")

// alignEvaluation keeps the AI's document on the choices entered in the wizard: goals,
// methods and instruments follow the wizard (an entry the AI left out is added with the
// catalog description). An answer made of placeholders, or that misses most of the
// choices, is rejected.
func alignEvaluation(doc *EvalReportDoc, status Status) error {
	if containsPlaceholder(doc) {
		return fmt.Errorf("%w: placeholders", ErrInvalidDocument)
	}
	fallback := fallbackEvaluation(status)
	matched, total := 0, 0

	goals := make([]EvalGoal, 0, len(fallback.Goals))
	used := map[string]bool{}
	for _, want := range fallback.Goals {
		total++
		if found, ok := findByName(len(doc.Goals), func(i int) string { return doc.Goals[i].Name }, want.Name); ok {
			matched++
			goals = append(goals, doc.Goals[found])
			used[doc.Goals[found].ID] = true
			continue
		}
		goals = append(goals, want)
	}
	doc.Goals = renumberMissing(goals, used, "G", func(g *EvalGoal) *string { return &g.ID })

	methods := make([]EvalMethod, 0, len(fallback.Methods))
	used = map[string]bool{}
	for _, want := range fallback.Methods {
		total++
		if found, ok := findByName(len(doc.Methods), func(i int) string { return doc.Methods[i].Name }, want.Name); ok {
			matched++
			methods = append(methods, doc.Methods[found])
			used[doc.Methods[found].ID] = true
			continue
		}
		methods = append(methods, want)
	}
	doc.Methods = renumberMissing(methods, used, "M", func(m *EvalMethod) *string { return &m.ID })

	instruments := make([]EvalInstrument, 0, len(fallback.Instrs))
	used = map[string]bool{}
	for _, want := range fallback.Instrs {
		total++
		if found, ok := findByName(len(doc.Instrs), func(i int) string { return doc.Instrs[i].Name }, want.Name); ok {
			matched++
			instruments = append(instruments, doc.Instrs[found])
			used[doc.Instrs[found].ID] = true
			continue
		}
		instruments = append(instruments, want)
	}
	doc.Instrs = renumberMissing(instruments, used, "I", func(i *EvalInstrument) *string { return &i.ID })

	if total > 0 && matched*2 < total {
		return fmt.Errorf("%w: %d of %d choices", ErrInvalidDocument, matched, total)
	}
	if len(doc.NextSteps) == 0 {
		doc.NextSteps = fallback.NextSteps
	}
	return nil
}

func findByName(count int, name func(int) string, want string) (int, bool) {
	for i := 0; i < count; i++ {
		if _, ok := catalog.Match([]string{want}, name(i)); ok {
			return i, true
		}
	}
	return 0, false
}

// renumberMissing gives the entries taken from the data (not from the AI) an id not used
// by the AI's entries, so the cross-references of the AI's text stay valid.
func renumberMissing[T any](items []T, used map[string]bool, prefix string, id func(*T) *string) []T {
	next := 1
	for i := range items {
		current := id(&items[i])
		if used[*current] {
			continue
		}
		for used[fmt.Sprintf("%s%d", prefix, next)] {
			next++
		}
		*current = fmt.Sprintf("%s%d", prefix, next)
		used[*current] = true
	}
	return items
}

// containsPlaceholder reports a template answer ("placeholder", "TBD", "lorem ipsum").
func containsPlaceholder(doc *EvalReportDoc) bool {
	texts := append([]string{doc.Subtitle}, doc.Context...)
	for _, goal := range doc.Goals {
		texts = append(texts, goal.Name, goal.Meaning, goal.WhyHere)
	}
	for _, method := range doc.Methods {
		texts = append(texts, method.Name, method.What, method.WhyHere)
	}
	for _, instrument := range doc.Instrs {
		texts = append(texts, instrument.Name, instrument.What, instrument.WhyHere)
	}
	texts = append(texts, doc.NextSteps...)
	for _, text := range texts {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "placeholder") || strings.Contains(lower, "lorem ipsum") || strings.TrimSpace(lower) == "tbd" {
			return true
		}
	}
	return false
}
