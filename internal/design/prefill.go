package design

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gamidoc/backend/internal/ai"
)

/* AI prefill from the Spark.

   The Spark (free text) is turned into section content in two phases:
   1. Analysis: one call extracts a structured understanding of the project (domain, users,
      behaviour to encourage, goals, constraints) plus the assumptions the model makes and
      the gaps the Spark leaves open.
   2. Sections: one call per section, in order 1..SectionCount. Each call gets the Spark,
      the analysis and the content of the sections before it (consistency: mechanics follow
      the chosen game elements, feedback links the mechanics…). The answer is constrained by
      a JSON Schema built from the section schema (only valid options), then sanitized
      (options, conditions, list format) before it is stored.

   For every value it chooses the model also gives a short reason and a confidence, kept in
   Status.Prefill.Notes so the interface can explain the suggestion. */

const (
	PrefillStateRunning = "running"
	PrefillStateDone    = "done"
	PrefillStateFailed  = "failed"

	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

// PrefillState is the progress and outcome of the AI prefill started by the last Spark.
type PrefillState struct {
	JobID      string     `json:"jobId"`
	State      string     `json:"state"`
	Current    int        `json:"currentSection"` // 0 = analysing the Spark
	Completed  []int      `json:"completedSections,omitempty"`
	Skipped    []int      `json:"skippedSections,omitempty"` // already had content
	Failed     []int      `json:"failedSections,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	Analysis *ProjectAnalysis `json:"analysis,omitempty"`
	// Notes per section ("1".."6"): why each value was chosen.
	Notes map[string][]FieldNote `json:"notes,omitempty"`
}

// FieldNote explains one AI-chosen value. Field is the field key (for lists: the list key).
type FieldNote struct {
	Field      string `json:"field"`
	Reason     string `json:"reason"`
	Confidence string `json:"confidence"`
}

// ProjectAnalysis is the model's structured understanding of the Spark.
type ProjectAnalysis struct {
	Summary            string   `json:"summary"`
	Domain             string   `json:"domain"`
	TargetUsers        string   `json:"targetUsers"`
	EncouragedBehavior string   `json:"encouragedBehavior"`
	Goals              []string `json:"goals"`
	Constraints        []string `json:"constraints"`
	Assumptions        []string `json:"assumptions"`
	Gaps               []string `json:"gaps"`
	Language           string   `json:"language"`
}

type Prefiller struct {
	assistant ai.Assistant
}

func NewPrefiller(assistant ai.Assistant) *Prefiller {
	return &Prefiller{assistant: assistant}
}

const prefillSystemPrompt = `You are a senior gamification and game-experience designer helping a user fill in a gamification design document in GamiDoc.

The user described their project idea in free text (the "Spark"). Your job is to understand the project and fill the document the way an expert would: make the design choices that best serve the project's users and the behaviour it wants to encourage, and keep every choice consistent with the rest of the document.

Design knowledge to apply:
- Start from the domain, the users and the behaviour to encourage; every mechanic must serve that behaviour.
- User types follow the HEXAD model: Philanthropists (purpose, altruism), Socialisers (relatedness), Free Spirits (autonomy, exploration), Achievers (competence, mastery), Players (extrinsic rewards), Disruptors (change, testing the system).
- Game elements follow the taxonomy of Toda et al.: Performance (points, level, progression, status, achievement), Social (competition, cooperation, reputation, social pressure), Personal (novelty, objectives, puzzles, renovation, sensation), Fictional (narrative, storytelling), Ecological (chance, imposed choice, economy, rarity, time pressure).
- Prefer intrinsic motivation (autonomy, competence, relatedness, meaning) over pure extrinsic rewards, especially in education, health and work contexts; avoid manipulative or anxiety-inducing dynamics for vulnerable users unless the project clearly needs them.
- A core loop is the smallest repeated sequence of actions; a long loop gives it purpose over weeks or the whole experience.
- Feedback must be timely, clear and tied to the player's goals and to the mechanics that produce it.

Rules:
- Use only the options listed for each field, written exactly as listed. Use "Other" (and its text field) only when no listed option fits and the field allows it.
- Fill every required field. When the Spark states the answer, use it. When it does not, choose the option an expert would pick for this kind of project and users, and record it as an assumption.
- Fill optional fields when the Spark or the analysis gives material for them or when there is a clear best practice; otherwise leave them empty ("" or []).
- Fill a conditional field only if its condition holds with the values you chose; leave every other conditional field empty. For example "Game element categories" is a single choice: fill only the elements and the mechanics of the category you chose.
- Link only game mechanics that you actually defined (named "<Element> 1", "<Element> 2"…) or that already exist.
- Stay consistent with the sections already filled: never contradict them.
- Free-text answers: concrete and specific to this project, 1–4 sentences, no generic filler, written in the language of the Spark. Option values always stay exactly as listed (in English).
- For each value you set, add a note: the field key, a reason of at most 25 words, and a confidence: "high" if the Spark states it, "medium" if it is strongly implied, "low" if it is your expert assumption.`

// Analyze extracts the project understanding from the Spark.
func (p *Prefiller) Analyze(ctx context.Context, spark string) (ProjectAnalysis, error) {
	user := "Spark (the user's project idea):\n\"\"\"\n" + strings.TrimSpace(spark) + "\n\"\"\"\n\n" +
		"Analyse the project. Summarise it in 2–3 sentences; identify the domain, the target users, the behaviour the system should encourage, the goals and the constraints. " +
		"List the assumptions you will need to make to design it and the gaps (important information the Spark does not give). " +
		"Give the language the Spark is written in (e.g. \"English\", \"Italian\")."

	raw, err := p.assistant.CompleteJSON(ctx, ai.JSONRequest{
		Name:   "project_analysis",
		System: prefillSystemPrompt,
		User:   user,
		Schema: analysisSchema,
	})
	if err != nil {
		return ProjectAnalysis{}, err
	}

	var analysis ProjectAnalysis
	if err := json.Unmarshal(raw, &analysis); err != nil {
		return ProjectAnalysis{}, fmt.Errorf("unparseable project analysis: %w", err)
	}
	analysis.Summary = strings.TrimSpace(analysis.Summary)
	if analysis.Summary == "" && analysis.Domain == "" && analysis.TargetUsers == "" {
		return ProjectAnalysis{}, fmt.Errorf("empty project analysis")
	}
	return analysis, nil
}

var analysisSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["summary", "domain", "targetUsers", "encouragedBehavior", "goals", "constraints", "assumptions", "gaps", "language"],
  "properties": {
    "summary": {"type": "string"},
    "domain": {"type": "string"},
    "targetUsers": {"type": "string"},
    "encouragedBehavior": {"type": "string"},
    "goals": {"type": "array", "items": {"type": "string"}},
    "constraints": {"type": "array", "items": {"type": "string"}},
    "assumptions": {"type": "array", "items": {"type": "string"}},
    "gaps": {"type": "array", "items": {"type": "string"}},
    "language": {"type": "string"}
  }
}`)

// FillSection asks the model for the content of one section. previous holds the content of
// the sections already filled (by the user or by the AI), by section number. The result is
// sanitized: the content can be stored as is (nil if the model left everything empty).
func (p *Prefiller) FillSection(
	ctx context.Context,
	spark string,
	analysis ProjectAnalysis,
	previous map[int]json.RawMessage,
	number int,
) (SectionDraft, error) {
	section, ok := SchemaFor(number)
	if !ok {
		return SectionDraft{}, ErrInvalidSectionNumber
	}

	// Mechanics described in Gameful Core can be linked from the following sections.
	mechanics := mechanicNamesFrom(previous[4])

	schema, err := json.Marshal(sectionAnswerSchema(section, mechanics))
	if err != nil {
		return SectionDraft{}, err
	}

	raw, err := p.assistant.CompleteJSON(ctx, ai.JSONRequest{
		Name:   fmt.Sprintf("section_%d", number),
		System: prefillSystemPrompt,
		User:   sectionPrompt(spark, analysis, previous, section, mechanics),
		Schema: schema,
	})
	if err != nil {
		return SectionDraft{}, err
	}
	return sanitizeSectionAnswer(section, raw, mechanics)
}

// DraftAll analyses the Spark and drafts the given sections in ONE call (prefill mode
// "single"): the content is the same as with one call per section, but it costs a single
// request, which matters on free tiers with a daily request limit. existing holds the
// sections already filled, given as context. A section missing from the answer gets no draft.
func (p *Prefiller) DraftAll(
	ctx context.Context,
	spark string,
	existing map[int]json.RawMessage,
	numbers []int,
) (ProjectAnalysis, map[int]SectionDraft, error) {
	mechanics := mechanicNamesFrom(existing[4]) // only if Gameful Core is not drafted now

	var analysis jsonSchema
	if err := json.Unmarshal(analysisSchema, &analysis); err != nil {
		return ProjectAnalysis{}, nil, err
	}
	sections := strictObject()
	var b strings.Builder
	writeSpark(&b, spark)
	writePrevious(&b, existing, SectionCount+1)
	b.WriteString("First analyse the project (\"analysis\"): summarise it in 2-3 sentences; identify the domain, the target users, the behaviour to encourage, the goals and the constraints; list the assumptions you make and the gaps (important information the Spark does not give); give the language of the Spark.\n\n")
	b.WriteString("Then fill these sections (\"sections\", by number), in order. Later sections must stay consistent with what you wrote in the earlier ones: e.g. feedback links the game mechanics you defined in Gameful Core, which are named \"<Element> 1\", \"<Element> 2\"...\n")
	for _, number := range numbers {
		section, ok := SchemaFor(number)
		if !ok {
			return ProjectAnalysis{}, nil, ErrInvalidSectionNumber
		}
		addProperty(sections, SectionKey(number), sectionAnswerSchema(section, mechanics))
		fmt.Fprintf(&b, "\n## Section %d, \"%s\"\n%s\n", number, section.Title, section.Description)
		b.WriteString(describeSection(section, mechanics))
	}
	b.WriteString("\nFor each section, \"values\" holds the fields, \"lists\" the repeatable items, \"notes\" one note per value you set.")

	answer := strictObject()
	addProperty(answer, "analysis", analysis)
	addProperty(answer, "sections", sections)
	schema, err := json.Marshal(answer)
	if err != nil {
		return ProjectAnalysis{}, nil, err
	}

	raw, err := p.assistant.CompleteJSON(ctx, ai.JSONRequest{
		Name:   "design_draft",
		System: prefillSystemPrompt,
		User:   b.String(),
		Schema: schema,
	})
	if err != nil {
		return ProjectAnalysis{}, nil, err
	}

	var decoded struct {
		Analysis ProjectAnalysis            `json:"analysis"`
		Sections map[string]json.RawMessage `json:"sections"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ProjectAnalysis{}, nil, fmt.Errorf("unparseable design draft: %w", err)
	}

	drafts := map[int]SectionDraft{}
	for _, number := range numbers {
		section, _ := SchemaFor(number)
		rawSection, ok := decoded.Sections[SectionKey(number)]
		if !ok || len(rawSection) == 0 || string(rawSection) == "null" {
			continue
		}
		if number == 5 && len(existing[4]) == 0 && drafts[4].Content != nil {
			mechanics = mechanicNamesFrom(drafts[4].Content) // links to the mechanics just drafted
		}
		draft, err := sanitizeSectionAnswer(section, rawSection, mechanics)
		if err != nil {
			continue
		}
		drafts[number] = draft
	}
	return decoded.Analysis, drafts, nil
}

// ── Prompt ──────────────────────────────────────────────────────────────────

func sectionPrompt(spark string, analysis ProjectAnalysis, previous map[int]json.RawMessage, section SchemaSection, mechanics []string) string {
	var b strings.Builder

	writeSpark(&b, spark)
	if encoded, err := json.MarshalIndent(analysis, "", "  "); err == nil {
		b.WriteString("Project analysis:\n")
		b.Write(encoded)
		b.WriteString("\n\n")
	}
	writePrevious(&b, previous, section.Number)

	fmt.Fprintf(&b, "Now fill section %d, \"%s\".\n%s\n\n", section.Number, section.Title, section.Description)
	b.WriteString(describeSection(section, mechanics))
	b.WriteString("\nAnswer with the JSON object: \"values\" holds the fields, \"lists\" the repeatable items, \"notes\" one note per value you set.")
	return b.String()
}

func writeSpark(b *strings.Builder, spark string) {
	b.WriteString("Spark (the user's project idea):\n\"\"\"\n")
	b.WriteString(strings.TrimSpace(spark))
	b.WriteString("\n\"\"\"\n\n")
}

// writePrevious writes the content of the sections before `before` that are already filled.
func writePrevious(b *strings.Builder, previous map[int]json.RawMessage, before int) {
	wrote := false
	for number := 1; number < before && number <= SectionCount; number++ {
		content := previous[number]
		if len(content) == 0 {
			continue
		}
		if !wrote {
			b.WriteString("Sections already filled (keep them, stay consistent with them):\n")
			wrote = true
		}
		section, _ := SchemaFor(number)
		fmt.Fprintf(b, "## %d. %s\n", number, section.Title)
		for _, entry := range orderedEntries(content) {
			fmt.Fprintf(b, "- %s: %s\n", entry.key, entryText(entry.value))
		}
		b.WriteString("\n")
	}
}

func describeSection(section SchemaSection, mechanics []string) string {
	var b strings.Builder
	groups := map[string]SchemaGroup{}
	for _, group := range section.Groups {
		groups[group.ID] = group
	}

	currentGroup := ""
	for _, field := range section.Fields {
		if field.Group != currentGroup {
			currentGroup = field.Group
			if group, ok := groups[currentGroup]; ok {
				fmt.Fprintf(&b, "\n### %s\n", group.Title)
				if group.Description != "" {
					b.WriteString(group.Description + "\n")
				}
			}
		}
		if field.Display == "loop" {
			// Container of the loop questions: describe the loop, it has no value of its own.
			fmt.Fprintf(&b, "(%s: %s)\n", field.Badge, field.Description)
			continue
		}
		if !field.fillable() {
			continue
		}
		describeField(&b, field, "", mechanics)
	}
	return b.String()
}

func describeField(b *strings.Builder, field SchemaField, indent string, mechanics []string) {
	fmt.Fprintf(b, "%s- \"%s\" [%s]\n", indent, field.Key, fieldKind(field))
	if question := questionOf(field); question != "" {
		fmt.Fprintf(b, "%s  Question: %s\n", indent, question)
	}
	if field.Placeholder != "" {
		fmt.Fprintf(b, "%s  Example format: %s\n", indent, strings.ReplaceAll(field.Placeholder, "\n", " / "))
	}
	for _, condition := range field.ShowIf {
		fmt.Fprintf(b, "%s  Only if \"%s\" is/includes \"%s\"; otherwise leave it empty.\n", indent, condition.Key, condition.Includes)
	}
	if field.linksMechanics() {
		if len(mechanics) > 0 {
			fmt.Fprintf(b, "%s  Values: names of game mechanics from Gameful Core: %s\n", indent, strings.Join(mechanics, ", "))
		} else {
			fmt.Fprintf(b, "%s  Values: names of the mechanics described in this section (e.g. \"Points 1\").\n", indent)
		}
	}
	for _, option := range field.Options {
		line := option.Value
		if option.Description != "" {
			line += " — " + option.Description
		}
		if option.Info != "" {
			line += " (" + option.Info + ")"
		}
		fmt.Fprintf(b, "%s    • %s\n", indent, line)
	}
	if field.Other {
		fmt.Fprintf(b, "%s    • Other — then write it in \"%s\"\n", indent, OtherKey(field))
	}
	if field.Type == FieldTypeList {
		if field.Numbered {
			fmt.Fprintf(b, "%s  Items are named automatically \"%s 1\", \"%s 2\"…; each item has:\n", indent, field.ItemLabel, field.ItemLabel)
		} else {
			fmt.Fprintf(b, "%s  Each item has a short \"Name\" and:\n", indent)
		}
		for _, sub := range field.ItemFields {
			if sub.fillable() {
				describeField(b, sub, indent+"    ", mechanics)
			}
		}
	}
}

func fieldKind(field SchemaField) string {
	var parts []string
	switch field.Type {
	case FieldTypeSingle:
		parts = append(parts, "single choice")
	case FieldTypeMulti:
		parts = append(parts, "multiple choice")
	case FieldTypeList:
		parts = append(parts, "list of items")
	default:
		parts = append(parts, "free text")
	}
	if field.Required {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}
	return strings.Join(parts, ", ")
}

func questionOf(field SchemaField) string {
	var parts []string
	for _, text := range []string{field.Title, field.Label, field.Hint} {
		text = strings.TrimSpace(text)
		if text != "" && text != field.Key {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " — ")
}

func entryText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return strings.Join(list, ", ")
	}
	return string(raw)
}

// ── Answer schema ───────────────────────────────────────────────────────────

type jsonSchema = map[string]any

// sectionAnswerSchema is the JSON Schema of the model's answer for a section:
//
//	{ "values": {<field>: …}, "lists": {<list>: [ {item} ]}, "notes": [ {field, reason, confidence} ] }
//
// It is written for strict structured outputs: every object lists all its properties as
// required and forbids others; "not filled" is "" or [].
func sectionAnswerSchema(section SchemaSection, mechanics []string) jsonSchema {
	values := strictObject()
	lists := strictObject()
	var noteFields []string

	for _, field := range section.Fields {
		if !field.fillable() {
			continue
		}
		noteFields = append(noteFields, field.Key)
		if field.Type == FieldTypeList {
			addProperty(lists, field.Key, jsonSchema{"type": "array", "items": listItemSchema(field, mechanics)})
			continue
		}
		addValueProperties(values, field, mechanics)
	}

	answer := strictObject()
	addProperty(answer, "values", values)
	if len(lists["required"].([]string)) > 0 {
		addProperty(answer, "lists", lists)
	}
	addProperty(answer, "notes", jsonSchema{
		"type": "array",
		"items": jsonSchema{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"field", "reason", "confidence"},
			"properties": jsonSchema{
				"field":      jsonSchema{"type": "string", "enum": noteFields},
				"reason":     jsonSchema{"type": "string"},
				"confidence": jsonSchema{"type": "string", "enum": []string{ConfidenceLow, ConfidenceMedium, ConfidenceHigh}},
			},
		},
	})
	return answer
}

func listItemSchema(list SchemaField, mechanics []string) jsonSchema {
	item := strictObject()
	if !list.Numbered {
		addProperty(item, "Name", jsonSchema{"type": "string"})
	}
	for _, sub := range list.ItemFields {
		if sub.fillable() {
			addValueProperties(item, sub, mechanics)
		}
	}
	return item
}

func addValueProperties(object jsonSchema, field SchemaField, mechanics []string) {
	addProperty(object, field.Key, valueSchema(field, mechanics))
	if field.Other {
		addProperty(object, OtherKey(field), jsonSchema{"type": "string"})
	}
}

func valueSchema(field SchemaField, mechanics []string) jsonSchema {
	switch field.Type {
	case FieldTypeSingle:
		return jsonSchema{"type": "string", "enum": append(field.optionValues(), "")}
	case FieldTypeMulti:
		if field.linksMechanics() {
			items := jsonSchema{"type": "string"}
			if len(mechanics) > 0 {
				items["enum"] = mechanics
			}
			return jsonSchema{"type": "array", "items": items}
		}
		return jsonSchema{"type": "array", "items": jsonSchema{"type": "string", "enum": field.optionValues()}}
	default:
		return jsonSchema{"type": "string"}
	}
}

func strictObject() jsonSchema {
	return jsonSchema{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{},
		"properties":           jsonSchema{},
	}
}

func addProperty(object jsonSchema, key string, schema jsonSchema) {
	object["properties"].(jsonSchema)[key] = schema
	object["required"] = append(object["required"].([]string), key)
}
