package design

import (
	"reflect"
	"strings"
)

/* Final documents of a design, written by the AI from the sections and exported as PDF.

   Two formats, chosen by the user:
   - "report": a reasoned design report. For each choice: why it was made, how it shows up
     in the design and what to watch out for, with references to the literature.
   - "gdd": a Game Design Document. One card per game system (purpose, rules, values,
     feedback, risks), player journey, adaptation rules, risks and success metrics.

   The AI fills the structs below (one request per document, answer constrained by the JSON
   Schema generated from them); the glossary and the reference list are built by the
   backend, so the AI can only cite references from the curated catalogue. */

const (
	ReportFormatReport = "report"
	ReportFormatGDD    = "gdd"
)

func ValidReportFormat(format string) bool {
	return format == ReportFormatReport || format == ReportFormatGDD || format == ReportFormatEvaluation
}

// ReportFormatTitle is the human name of a format (buttons, history, PDF header).
func ReportFormatTitle(format string) string {
	switch format {
	case ReportFormatReport:
		return "Design report"
	case ReportFormatGDD:
		return "Game Design Document"
	case ReportFormatEvaluation:
		return "UX evaluation report"
	case ReportVersionStandard:
		return "Standard report"
	case ReportVersionEnhanced:
		return "AI-Enhanced report"
	default:
		return "Design document"
	}
}

// ── Shared parts ────────────────────────────────────────────────────────────

type DocLoopStep struct {
	Title string `json:"title" desc:"2-3 words, e.g. Action, Result, Choice / Weekly goal"`
	Text  string `json:"text" desc:"at most 12 words"`
}

type DocLoop struct {
	Steps  []DocLoopStep `json:"steps" desc:"exactly 3 steps"`
	Repeat string        `json:"repeat" desc:"what makes the loop start again, at most 10 words"`
}

type DocLoops struct {
	CoreLabel string  `json:"coreLabel" desc:"e.g. 'every trip', 'every session'"`
	Core      DocLoop `json:"core"`
	LongLabel string  `json:"longLabel" desc:"e.g. 'every week and every month'"`
	Long      DocLoop `json:"long"`
	Link      string  `json:"link" desc:"how the core loop feeds the long loop, at most 8 words"`
}

// ── Format "report" ─────────────────────────────────────────────────────────

type ReportGlance struct {
	ForWhom    string `json:"forWhom" desc:"who it is for, 1-2 sentences"`
	WhatWeWant string `json:"whatWeWant" desc:"the behaviour to encourage, 1-2 sentences"`
	Motivation string `json:"motivation" desc:"what motivates the users, 1-2 sentences"`
	HowWeKnow  string `json:"howWeKnow" desc:"how we will know it works, 1-2 sentences"`
	MainRisk   string `json:"mainRisk" desc:"the main design risk, 1-2 sentences"`
	Addressing string `json:"addressing" desc:"how the design addresses that risk, 1-2 sentences"`
}

type ReportFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ReportBlock explains one group of choices (e.g. Constraints, Retention anchors).
type ReportBlock struct {
	Title       string   `json:"title" desc:"plain-language title, e.g. 'What brings users back'"`
	Term        string   `json:"term" desc:"the GamiDoc field name, shown in brackets, e.g. 'Retention anchors'; empty if same as title"`
	Values      []string `json:"values" desc:"the options chosen in the design, exactly as entered"`
	Why         string   `json:"why" desc:"why these choices fit this project, 2-4 sentences"`
	InTheDesign []string `json:"inTheDesign" desc:"how each choice shows up concretely in the design, one item per choice"`
	Numbered    bool     `json:"numbered" desc:"true if inTheDesign is an ordered sequence of steps"`
	WatchOut    string   `json:"watchOut" desc:"a risk or caveat to keep in mind, 1-2 sentences; empty if none"`
	Refs        []int    `json:"refs" desc:"ids of references from the catalogue supporting why/watchOut; empty if none"`
}

type ReportFeedback struct {
	ID          string `json:"id" desc:"F1, F2, ..."`
	Moment      string `json:"moment" desc:"e.g. 'After every trip'"`
	Message     string `json:"message" desc:"example message shown to the user"`
	When        string `json:"when"`
	Type        string `json:"type"`
	Channel     string `json:"channel"`
	GameElement string `json:"gameElement"`
	WhyThisWay  string `json:"whyThisWay" desc:"1-2 sentences"`
	Technical   string `json:"technical" desc:"1 sentence; empty if not entered"`
}

type ReportSection struct {
	Number   int              `json:"number" desc:"the design section number, 1-6"`
	InShort  string           `json:"inShort" desc:"1-2 sentences summarising the section"`
	Facts    []ReportFact     `json:"facts" desc:"short label/value facts of the section (e.g. Domain, Aim, Users; levels of emotional dynamics)"`
	Blocks   []ReportBlock    `json:"blocks"`
	Feedback []ReportFeedback `json:"feedback" desc:"only in the feedback section: one card per feedback component; empty elsewhere"`
	Closing  string           `json:"closing" desc:"optional closing paragraph (e.g. how we will know it works); empty if none"`
}

type ReportCoherence struct {
	Outcome  string `json:"outcome" enum:"Reinforce each other|Keep an eye on|Not covered"`
	Choices  string `json:"choices" desc:"the choices involved, from different sections"`
	Handling string `json:"handling" desc:"how it is handled, 1 sentence"`
}

type ReportEvaluationLink struct {
	Choice  string `json:"choice" desc:"a risky design choice"`
	Goal    string `json:"goal" desc:"the evaluation goal that tests it, e.g. 'Motivation Spectrum'"`
	Measure string `json:"measure" desc:"how it can be measured, e.g. 'IMI, diary, interview'"`
}

type ReportDoc struct {
	Title       string                 `json:"title" desc:"the project name, e.g. GreenStep; invent a fitting one if none is given; never the Spark text and never the document type (not 'Design report', not 'Game Design Document')"`
	Subtitle    string                 `json:"subtitle" desc:"one sentence: what the system is and the behaviour it encourages"`
	Glance      ReportGlance           `json:"glance"`
	Loops       DocLoops               `json:"loops"`
	Sections    []ReportSection        `json:"sections" desc:"one per design section that has content, in order"`
	Coherence   []ReportCoherence      `json:"coherence" desc:"3-6 rows: where choices support each other, may conflict, or leave something uncovered"`
	EvalIntro   string                 `json:"evalIntro" desc:"1-2 sentences introducing the link to the evaluation"`
	Evaluation  []ReportEvaluationLink `json:"evaluation" desc:"3-6 rows"`
	Assumptions []string               `json:"assumptions" desc:"what the report assumes beyond the entered data; empty if nothing"`
}

// ── Format "gdd" ────────────────────────────────────────────────────────────

type GDDPillar struct {
	Title string `json:"title" desc:"3-5 words"`
	Text  string `json:"text" desc:"1-2 sentences"`
}

type GDDRow struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type GDDPlayerType struct {
	Type        string `json:"type" desc:"Hexad type(s), e.g. 'Achiever' or 'Free Spirit, Disruptor'"`
	MotivatedBy string `json:"motivatedBy"`
	ServedBy    string `json:"servedBy" desc:"the systems (by id) and features that serve this type, or 'Not served in v1'"`
}

type GDDLoopRow struct {
	Loop      string `json:"loop" enum:"Core loop|Long loop"`
	Steps     string `json:"steps" desc:"steps joined with →"`
	Frequency string `json:"frequency"`
}

type GDDSystem struct {
	ID          string   `json:"id" desc:"SYS-01, SYS-02, ..."`
	Name        string   `json:"name"`
	Purpose     string   `json:"purpose" desc:"1 sentence"`
	Rules       []string `json:"rules"`
	Values      []GDDRow `json:"values" desc:"parameters and values ONLY if entered in the design; empty otherwise"`
	Feedback    []string `json:"feedback"`
	Risks       string   `json:"risks" desc:"risks and safeguards, 1-3 sentences"`
	FromGamiDoc string   `json:"fromGamiDoc" desc:"the GamiDoc fields this card comes from, e.g. 'Game rules; Performance elements: Points'"`
}

type GDDReward struct {
	Reward string `json:"reward"`
	Type   string `json:"type" desc:"e.g. Information, Cosmetic, Real-world"`
	Where  string `json:"where" desc:"where it appears"`
}

type GDDJourneyStep struct {
	Moment     string `json:"moment" desc:"e.g. 'Install (day 0)', 'First week'"`
	PlayerDoes string `json:"playerDoes"`
	SystemDoes string `json:"systemDoes"`
	Risk       string `json:"risk" desc:"risk to watch"`
}

type GDDAdaptation struct {
	If            string `json:"if"`
	Then          string `json:"then"`
	PlayerControl string `json:"playerControl" desc:"e.g. 'Accept or decline'; '—' if none"`
}

type GDDFeedback struct {
	ID      string `json:"id" desc:"F1, F2, ..."`
	Moment  string `json:"moment"`
	Message string `json:"message" desc:"example message"`
	Channel string `json:"channel" desc:"channel and timing"`
	Systems string `json:"systems" desc:"system ids, e.g. 'SYS-01, SYS-04'"`
}

type GDDRisk struct {
	Risk       string `json:"risk"`
	Mitigation string `json:"mitigation"`
	Systems    string `json:"systems" desc:"system ids or '—'"`
}

type GDDMetric struct {
	Metric string `json:"metric"`
	Target string `json:"target" desc:"a target only if it follows from the design; otherwise 'To be defined'"`
	How    string `json:"how" desc:"how it is measured"`
}

type GDDRationale struct {
	Title string `json:"title" desc:"the choice, e.g. 'Team competition instead of individual ranking'"`
	Text  string `json:"text" desc:"1-2 sentences"`
	Refs  []int  `json:"refs" desc:"ids of references from the catalogue; empty if none"`
}

type GDDTerm struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
}

type GDDDoc struct {
	Title           string           `json:"title" desc:"the project name, e.g. GreenStep; invent a fitting one if none is given; never the Spark text and never the document type (not 'Design report', not 'Game Design Document')"`
	Subtitle        string           `json:"subtitle" desc:"one sentence: what the game is and the behaviour it encourages"`
	HighConcept     string           `json:"highConcept" desc:"2 sentences"`
	Domain          string           `json:"domain"`
	Players         string           `json:"players" desc:"at most 6 words"`
	TargetBehaviour string           `json:"targetBehaviour" desc:"at most 8 words"`
	Modality        string           `json:"modality" desc:"at most 6 words"`
	Pillars         []GDDPillar      `json:"pillars" desc:"exactly 3 design pillars"`
	Loops           DocLoops         `json:"loops"`
	OutOfScope      string           `json:"outOfScope" desc:"what is deliberately left out of v1, 1 sentence; empty if nothing"`
	PlayerAspects   []GDDRow         `json:"playerAspects" desc:"Who, Gaming experience, Context of use, Constraints"`
	PlayerTypes     []GDDPlayerType  `json:"playerTypes"`
	LoopRows        []GDDLoopRow     `json:"loopRows" desc:"the core loop and the long loop"`
	GlobalRules     []string         `json:"globalRules"`
	Systems         []GDDSystem      `json:"systems" desc:"one card per game system (mechanic, goal, challenge, level...), 3-8 cards"`
	Progression     string           `json:"progression" desc:"how progression and rewards work, 2-3 sentences"`
	Rewards         []GDDReward      `json:"rewards"`
	Journey         []GDDJourneyStep `json:"journey" desc:"4-7 moments from install to long-term use"`
	Adaptation      []GDDAdaptation  `json:"adaptation" desc:"adaptation rules from the personalization section; empty if none"`
	AdaptationNote  string           `json:"adaptationNote" desc:"1 sentence on how changes are applied; empty if none"`
	Feedback        []GDDFeedback    `json:"feedback"`
	Technical       []GDDRow         `json:"technical" desc:"area / requirement rows from the technical section; empty if not entered"`
	Risks           []GDDRisk        `json:"risks"`
	Metrics         []GDDMetric      `json:"metrics"`
	Rationale       []GDDRationale   `json:"rationale" desc:"3-6 main choices and why"`
	Terms           []GDDTerm        `json:"terms" desc:"project-specific terms used in this document (e.g. 'Active member'); empty if none"`
}

// ── JSON Schema from the structs ────────────────────────────────────────────

// docSchema builds the strict JSON Schema of a document struct: every property required,
// no additional properties; `desc` tags become descriptions, `enum` tags (a|b|c) enums.
func docSchema(value any) jsonSchema {
	return typeSchema(reflect.TypeOf(value))
}

func typeSchema(t reflect.Type) jsonSchema {
	switch t.Kind() {
	case reflect.Struct:
		object := strictObject()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			property := typeSchema(field.Type)
			if desc := field.Tag.Get("desc"); desc != "" {
				property["description"] = desc
			}
			if enum := field.Tag.Get("enum"); enum != "" {
				property["enum"] = strings.Split(enum, "|")
			}
			addProperty(object, name, property)
		}
		return object
	case reflect.Slice:
		return jsonSchema{"type": "array", "items": typeSchema(t.Elem())}
	case reflect.Int:
		return jsonSchema{"type": "integer"}
	case reflect.Bool:
		return jsonSchema{"type": "boolean"}
	default:
		return jsonSchema{"type": "string"}
	}
}
