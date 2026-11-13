package design

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gamidoc/backend/internal/ai"
)

// ── References ──────────────────────────────────────────────────────────────

// DocReference is an entry of the curated reference catalogue. The AI cites references
// only by ID, so documents never contain invented sources.
type DocReference struct {
	ID    int
	Topic string // what it supports, given to the AI to choose
	Text  string // full citation, printed in the document
}

var referenceCatalogue = []DocReference{
	{1, "flow; balance between challenge and skill", "Csikszentmihalyi, M. (1990). Flow: The Psychology of Optimal Experience. Harper & Row."},
	{2, "extrinsic rewards can undermine intrinsic motivation", "Deci, E. L., Koestner, R., & Ryan, R. M. (1999). A meta-analytic review of experiments examining the effects of extrinsic rewards on intrinsic motivation. Psychological Bulletin, 125(6), 627–668. https://doi.org/10.1037/0033-2909.125.6.627"},
	{3, "self-determination theory: autonomy, competence, relatedness", "Deci, E. L., & Ryan, R. M. (2000). The \"what\" and \"why\" of goal pursuits: Human needs and the self-determination of behavior. Psychological Inquiry, 11(4), 227–268. https://doi.org/10.1207/S15327965PLI1104_01"},
	{4, "definition of gamification and game design elements", "Deterding, S., Dixon, D., Khaled, R., & Nacke, L. (2011). From game design elements to gamefulness: Defining \"gamification\". Proceedings of MindTrek 2011, 9–15. https://doi.org/10.1145/2181037.2181040"},
	{5, "personal data protection in the EU (GDPR)", "Regulation (EU) 2016/679 of the European Parliament and of the Council (General Data Protection Regulation). https://eur-lex.europa.eu/eli/reg/2016/679/oj"},
	{6, "does gamification work: empirical evidence", "Hamari, J., Koivisto, J., & Sarsa, H. (2014). Does gamification work? A literature review of empirical studies on gamification. Proceedings of HICSS 2014, 3025–3034. https://doi.org/10.1109/HICSS.2014.377"},
	{7, "effective feedback: where am I, where next", "Hattie, J., & Timperley, H. (2007). The power of feedback. Review of Educational Research, 77(1), 81–112. https://doi.org/10.3102/003465430298487"},
	{8, "habit formation takes weeks of repetition", "Lally, P., van Jaarsveld, C. H. M., Potts, H. W. W., & Wardle, J. (2010). How are habits formed: Modelling habit formation in the real world. European Journal of Social Psychology, 40(6), 998–1009. https://doi.org/10.1002/ejsp.674"},
	{9, "leaderboards: motivate the top, can discourage the rest", "Landers, R. N., Bauer, K. N., & Callan, R. C. (2017). Gamification of task performance with leaderboards: A goal setting experiment. Computers in Human Behavior, 71, 508–515. https://doi.org/10.1016/j.chb.2015.08.008"},
	{10, "specific, challenging goals with feedback (goal setting)", "Locke, E. A., & Latham, G. P. (2002). Building a practically useful theory of goal setting and task motivation. American Psychologist, 57(9), 705–717. https://doi.org/10.1037/0003-066X.57.9.705"},
	{11, "Hexad user types (book)", "Marczewski, A. (2015). Even Ninja Monkeys Like to Play: Gamification, Game Thinking and Motivational Design. CreateSpace."},
	{12, "taxonomy of game elements (performance, social, personal, fictional, ecological)", "Toda, A. M., Oliveira, W., Klock, A. C., Palomino, P. T., Pimenta, M., Gasparini, I., Shi, L., Bittencourt, I., Isotani, S., & Cristea, A. I. (2019). A taxonomy of game elements for gamification in educational contexts: Proposal and evaluation. Proceedings of ICALT 2019, 84–88. https://doi.org/10.1109/ICALT.2019.00028"},
	{13, "Hexad user types scale", "Tondello, G. F., Wehbe, R. R., Diamond, L., Busch, M., Marczewski, A., & Nacke, L. E. (2016). The gamification user types Hexad scale. Proceedings of CHI PLAY 2016, 229–243. https://doi.org/10.1145/2967934.2968082"},
	{14, "self-determination theory and intrinsic motivation", "Ryan, R. M., & Deci, E. L. (2000). Self-determination theory and the facilitation of intrinsic motivation, social development, and well-being. American Psychologist, 55(1), 68–78. https://doi.org/10.1037/0003-066X.55.1.68"},
	{15, "meaningful gamification (play, exposition, choice, information, engagement, reflection)", "Nicholson, S. (2015). A RECIPE for meaningful gamification. In T. Reiners & L. C. Wood (Eds.), Gamification in Education and Business (pp. 1–20). Springer. https://doi.org/10.1007/978-3-319-10208-5_1"},
	{16, "behaviour = motivation × ability × trigger", "Fogg, B. J. (2009). A behavior model for persuasive design. Proceedings of Persuasive 2009, Article 40. https://doi.org/10.1145/1541948.1541999"},
}

func referenceByID(id int) (DocReference, bool) {
	for _, reference := range referenceCatalogue {
		if reference.ID == id {
			return reference, true
		}
	}
	return DocReference{}, false
}

// ── Glossary ────────────────────────────────────────────────────────────────

type GlossaryGroup struct {
	Title string
	Terms []GDDTerm
}

// buildGlossary lists, grouped by field, the options chosen in the design that the schema
// explains (description or info), in schema order.
func buildGlossary(status Status) []GlossaryGroup {
	var groups []GlossaryGroup
	for number := 1; number <= SectionCount; number++ {
		section, _ := SchemaFor(number)
		content := status.Section(number).Content
		if len(content) == 0 {
			continue
		}
		var values map[string]json.RawMessage
		if json.Unmarshal(content, &values) != nil {
			continue
		}
		for _, field := range section.Fields {
			if field.Type != FieldTypeSingle && field.Type != FieldTypeMulti {
				continue
			}
			chosen := listOf(values[field.Key])
			var terms []GDDTerm
			for _, option := range field.Options {
				meaning := option.Description
				if meaning == "" {
					meaning = option.Info
				}
				if meaning != "" && containsString(chosen, option.Value) {
					terms = append(terms, GDDTerm{Term: option.Value, Meaning: meaning})
				}
			}
			if len(terms) > 0 {
				groups = append(groups, GlossaryGroup{Title: field.Key, Terms: terms})
			}
		}
	}
	return groups
}

// ── Generation ──────────────────────────────────────────────────────────────

const documentSystemPrompt = `You are a senior gamification designer writing the final document of a design made in GamiDoc, a tool that guides designers through six sections (Context, Experience Timeline, Personalization & Dynamics, Gameful Core, Feedback & Evaluation, Technical Requirements).

Rules:
- Ground everything in the design data you are given. Never invent numbers, values, names of screens, prices, dates or features that are not in the data or directly implied by it. If something is not in the data, leave it out (empty string or empty list) rather than making it up.
- Keep the options chosen in GamiDoc exactly as entered when you list them (e.g. "Ritual anchors", "Achiever").
- Explain choices with established design knowledge (self-determination theory, Hexad user types, the Toda et al. taxonomy of game elements, goal setting, feedback, flow, habit formation).
- Cite references ONLY by their id from the catalogue you are given, and only where they really support the sentence. Never write citations in the text yourself.
- Write clearly and concretely, for a reader who did not take part in the design. Short sentences, no filler.
- Write in the language of the design data (the language of the Spark and of the free-text fields).`

func formatInstructions(format string) string {
	if format == ReportFormatGDD {
		return `Write a Game Design Document organised for reference: the design on one page (high concept, three design pillars, core and long loop), the players and the Hexad types served, the core gameplay with global rules, one card per game system (purpose, rules, values, feedback, risks and safeguards, the GamiDoc fields it comes from), progression and rewards, the player journey, adaptation rules, feedback messages, technical requirements, risks with mitigations, success metrics and the design rationale.
Number systems SYS-01, SYS-02… and feedback F1, F2…, and use these ids to cross-reference them. Derive systems from the game rules, the game elements and their mechanics, goals, levels, progression, streaks and challenges in the data. Put in "values" only parameters actually entered.`
	}
	return `Write a reasoned design report: page one summarises the whole design (who it is for, what we want them to do, what motivates them, how we will know it works, the main risk and how we address it, the core and long loop). Then one section per design section with content: a short summary ("inShort"), key facts, and blocks that explain each group of choices — the options chosen, WHY they fit this project, HOW they show up IN THE DESIGN (one concrete item per choice) and what to WATCH OUT for. In the feedback section, one card per feedback component. Then how the choices fit together (reinforce each other / keep an eye on / not covered) and the link to the evaluation (which evaluation goal and measure tests each risky choice).

Each field in the data is marked [choice] (options picked from a list), [text] (free text) or [list item].
The blocks are the core of the report and must be complete. For EVERY [choice] field (or group of related [choice] fields) entered in a section (for example Constraints, Onboarding methods, Retention anchors, Drop-out risks, Goal types, User types, personalization, game elements, progression, modality and timing, platforms) write one block with:
- "values": the options chosen, exactly as entered;
- "why": 2-4 sentences explaining why these choices fit this project and its users;
- "inTheDesign": one concrete item per chosen option, saying how it shows up in this design;
- "watchOut": the main risk or caveat, when there is one.
A block with only a title is useless: never leave values, why and inTheDesign empty.
Do NOT make blocks for [text] fields: put short ones (aim, users, age, experience) in "facts" and use the others inside the explanations. Keep the report focused: at most 6 blocks per section.
Never write citation numbers like [3] in the text: put reference ids only in "refs".`
}

// designDataText renders the design (Spark, AI analysis, sections) as text for the prompt.
func designDataText(status Status) string {
	var b strings.Builder
	if spark := strings.TrimSpace(status.Spark); spark != "" {
		writeSpark(&b, spark)
	}
	if status.Prefill != nil && status.Prefill.Analysis != nil && status.Prefill.Analysis.Summary != "" {
		fmt.Fprintf(&b, "Project summary: %s\n\n", status.Prefill.Analysis.Summary)
	}
	b.WriteString("Design data entered in GamiDoc:\n")
	for number := 1; number <= SectionCount; number++ {
		section, _ := SchemaFor(number)
		fmt.Fprintf(&b, "\n## Section %d. %s\n", number, section.Title)
		entries := orderedEntries(status.Section(number).Content)
		if len(entries) == 0 {
			b.WriteString("(not filled in)\n")
			continue
		}
		kinds := fieldKinds(section)
		for _, entry := range entries {
			if isImageURL(entry.value) {
				continue
			}
			fmt.Fprintf(&b, "- %s [%s]: %s\n", entry.key, kindOf(kinds, entry.key), entryText(entry.value))
		}
	}
	return b.String()
}

// fieldKinds maps the field keys of a section to "choice" or "text".
func fieldKinds(section SchemaSection) map[string]string {
	kinds := map[string]string{}
	for _, field := range section.Fields {
		switch field.Type {
		case FieldTypeSingle, FieldTypeMulti:
			kinds[field.Key] = "choice"
		case FieldTypeList:
			kinds[field.Key] = "list"
		default:
			kinds[field.Key] = "text"
		}
		if field.Other {
			kinds[OtherKey(field)] = "text"
		}
	}
	return kinds
}

func kindOf(kinds map[string]string, key string) string {
	if kind, ok := kinds[key]; ok {
		return kind
	}
	for field, kind := range kinds {
		if kind == "list" && strings.HasPrefix(key, field+" ") {
			return "list item"
		}
	}
	return "text"
}

func isImageURL(raw json.RawMessage) bool {
	text := textOf(raw)
	return strings.HasPrefix(text, "/files/") || strings.HasPrefix(text, "http") && (strings.HasSuffix(text, ".png") || strings.HasSuffix(text, ".jpg") || strings.HasSuffix(text, ".jpeg"))
}

func referenceCatalogueText() string {
	var b strings.Builder
	b.WriteString("Reference catalogue (cite by id only):\n")
	for _, reference := range referenceCatalogue {
		fmt.Fprintf(&b, "%d. %s\n", reference.ID, reference.Topic)
	}
	return b.String()
}

// GenerateDocument asks the AI for the document in the given format (one request).
// The result is either *ReportDoc or *GDDDoc.
func GenerateDocument(ctx context.Context, assistant ai.Assistant, format string, status Status) (any, error) {
	var target any
	var schema jsonSchema
	switch format {
	case ReportFormatReport:
		target, schema = &ReportDoc{}, docSchema(ReportDoc{})
	case ReportFormatGDD:
		target, schema = &GDDDoc{}, docSchema(GDDDoc{})
	default:
		return nil, ErrInvalidReportVersion
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}

	system := documentSystemPrompt
	user := designDataText(status) + "\n" + referenceCatalogueText() + "\n" + formatInstructions(format)
	if language := documentLanguage(status); language != "" {
		// Free models drift to English (the option names are English): the language goes at
		// the top of both messages and again at the end.
		rule := "LANGUAGE: write every text of the document (title, subtitle, explanations, table cells) in " + language + ". Only the GamiDoc option names stay as entered."
		system = rule + "\n\n" + system
		user = rule + "\n\n" + user + "\n\n" + rule
	}
	raw, err := assistant.CompleteJSON(ctx, ai.JSONRequest{
		Name:   "design_document_" + format,
		System: system,
		User:   user,
		Schema: encoded,
	})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("unparseable document: %w", err)
	}
	if empty := documentIsEmpty(target); empty {
		return nil, fmt.Errorf("the AI returned an empty document")
	}
	tidyDocument(target)
	return target, nil
}

// documentLanguage is the language of the design: from the Spark analysis when available.
func documentLanguage(status Status) string {
	if status.Prefill != nil && status.Prefill.Analysis != nil {
		return strings.TrimSpace(status.Prefill.Analysis.Language)
	}
	return ""
}

// ErrThinDocument: the AI answered with a skeleton (e.g. report blocks with only a title).
var ErrThinDocument = fmt.Errorf("the AI returned a document without explanations")

// reportCompleteness is the share of report blocks that explain the choice (why + in the
// design); 0 if there are no blocks.
func reportCompleteness(doc *ReportDoc) float64 {
	total, explained := 0, 0
	for _, section := range doc.Sections {
		for _, block := range section.Blocks {
			total++
			if strings.TrimSpace(block.Why) != "" && len(block.InTheDesign) > 0 {
				explained++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(explained) / float64(total)
}

// tidyDocument fixes what the model tends to get wrong without cutting any text: citation
// numbers written in the text, the document type repeated in the title, feedback cards in
// the wrong section, loop labels that only repeat "core loop".
func tidyDocument(document any) {
	switch doc := document.(type) {
	case *ReportDoc:
		fixTitle(&doc.Title, &doc.Subtitle)
		tidyLoops(&doc.Loops)
		for i := range doc.Sections {
			for j := range doc.Sections[i].Blocks {
				block := &doc.Sections[i].Blocks[j]
				block.Why, block.Refs = extractCitations(block.Why, block.Refs)
				block.WatchOut, block.Refs = extractCitations(block.WatchOut, block.Refs)
			}
			doc.Sections[i].InShort, _ = extractCitations(doc.Sections[i].InShort, nil)
			if doc.Sections[i].Number != 5 {
				doc.Sections[i].Feedback = nil // feedback cards belong to Feedback & Evaluation only
			}
		}
	case *GDDDoc:
		tidyLoops(&doc.Loops)
		for i := range doc.Rationale {
			doc.Rationale[i].Text, doc.Rationale[i].Refs = extractCitations(doc.Rationale[i].Text, doc.Rationale[i].Refs)
		}
		fixTitle(&doc.Title, &doc.Subtitle)
	}
}

// formatName matches the names of a document type the model likes to put in the title, in
// English and Italian ("Game Design Document", "GDD", "Documento finale di design", …).
const formatName = `(?:(?:gamidoc\s+)?(?:(?:final\s+)?game\s+design\s+document|gdd|(?:final\s+)?design\s+(?:report|document)|documento\s+(?:finale\s+)?di\s+(?:game\s+)?design(?:\s+finale)?|report\s+(?:finale\s+)?di\s+design)|gamidoc\s+(?:final\s+)?(?:design\s+)?document|documento\s+finale(?:\s+gamidoc)?)`

// formatInTitle matches the document type at the start or end of the title ("Game Design
// Document — X", "X · GDD", "Report di design: X"): the cover already shows the type.
var formatInTitle = regexp.MustCompile(`(?i)^\s*` + formatName + `\s*[—–\-:·|]+\s*|\s*[—–\-:·|]+\s*` + formatName + `\s*$`)

// onlyFormat matches a title that is just the document type (e.g. "Report di design").
var onlyFormat = regexp.MustCompile(`(?i)^\s*` + formatName + `\s*$`)

// unusualSpaces are spaces the model sometimes writes (no-break, narrow, thin…), which
// regexp's \s does not match.
var unusualSpaces = strings.NewReplacer("\u00a0", " ", "\u202f", " ", "\u2009", " ", "\u2007", " ", "\u200a", " ")

func stripFormatFromTitle(title string) string {
	title = strings.TrimSpace(unusualSpaces.Replace(title))
	if stripped := strings.TrimSpace(formatInTitle.ReplaceAllString(title, "")); stripped != "" && stripped != title {
		runes := []rune(stripped)
		runes[0] = unicode.ToUpper(runes[0]) // "Documento di design — gioco …" -> "Gioco …"
		return string(runes)
	}
	return title
}

// fixTitle: removes the document type from the title; when the title is only the type,
// the subtitle (what the project is) becomes the title. Nothing else is cut.
func fixTitle(title, subtitle *string) {
	*title = stripFormatFromTitle(*title)
	if onlyFormat.MatchString(unusualSpaces.Replace(*subtitle)) {
		*subtitle = "" // a subtitle that only repeats the document type
	}
	if (*title == "" || onlyFormat.MatchString(*title)) && strings.TrimSpace(*subtitle) != "" {
		*title, *subtitle = strings.TrimSpace(*subtitle), ""
	}
}

func documentIsEmpty(document any) bool {
	switch doc := document.(type) {
	case *ReportDoc:
		return len(doc.Sections) == 0 && doc.Glance.ForWhom == ""
	case *GDDDoc:
		return len(doc.Systems) == 0 && doc.HighConcept == ""
	}
	return true
}

// ── Fallback without AI ─────────────────────────────────────────────────────

// fallbackDocument builds the document from the entered data only, when no AI is
// available or the AI fails: facts and choices, without explanations.
func fallbackDocument(format string, status Status) any {
	title := "Design project"
	subtitle := strings.TrimSpace(status.Spark)
	if len([]rune(subtitle)) > 220 {
		subtitle = string([]rune(subtitle)[:220]) + "…"
	}
	if aim := fieldText(status, 1, "Aim of the software"); aim != "" {
		subtitle = aim
	}

	if format == ReportFormatGDD {
		doc := &GDDDoc{
			Title:           title,
			Subtitle:        subtitle,
			Domain:          fieldText(status, 1, "Domain"),
			Players:         fieldText(status, 1, "Occupation"),
			TargetBehaviour: fieldText(status, 1, "Encouraged behavior"),
			Modality:        fieldText(status, 4, "Modality"),
		}
		for _, key := range []string{"Occupation", "Age range", "Game experience", "Constraints"} {
			if value := fieldText(status, 1, key); value != "" {
				doc.PlayerAspects = append(doc.PlayerAspects, GDDRow{Name: key, Value: value})
			}
		}
		if rules := fieldText(status, 4, "Game rules"); rules != "" {
			doc.GlobalRules = splitSentences(rules)
		}
		doc.Systems = fallbackSystems(status)
		for _, entry := range orderedEntries(status.Section(6).Content) {
			doc.Technical = append(doc.Technical, GDDRow{Name: entry.key, Value: entryText(entry.value)})
		}
		return doc
	}

	doc := &ReportDoc{Title: title, Subtitle: subtitle}
	doc.Glance.ForWhom = joinNonEmpty(fieldText(status, 1, "Occupation"), fieldText(status, 1, "Age range"))
	doc.Glance.WhatWeWant = fieldText(status, 1, "Encouraged behavior")
	for number := 1; number <= SectionCount; number++ {
		section := ReportSection{Number: number}
		for _, entry := range orderedEntries(status.Section(number).Content) {
			if isImageURL(entry.value) {
				continue
			}
			section.Facts = append(section.Facts, ReportFact{Label: entry.key, Value: entryText(entry.value)})
		}
		if len(section.Facts) > 0 {
			doc.Sections = append(doc.Sections, section)
		}
	}
	return doc
}

// fallbackSystems turns each game mechanic of Gameful Core into a system card.
func fallbackSystems(status Status) []GDDSystem {
	section, _ := SchemaFor(4)
	values := map[string]json.RawMessage{}
	_ = json.Unmarshal(status.Section(4).Content, &values)
	var systems []GDDSystem
	for _, field := range section.Fields {
		if field.Type != FieldTypeList {
			continue
		}
		for index := 0; ; index++ {
			name := textOf(values[ListItemKey(field, index, "Name")])
			trigger := textOf(values[ListItemKey(field, index, "Trigger")])
			response := textOf(values[ListItemKey(field, index, "System response")])
			goal := textOf(values[ListItemKey(field, index, "Goal supported")])
			if name == "" && trigger == "" && response == "" {
				break
			}
			system := GDDSystem{
				ID:          fmt.Sprintf("SYS-%02d", len(systems)+1),
				Name:        joinNonEmpty(name, field.ItemLabel),
				Purpose:     goal,
				FromGamiDoc: "Gameful Core: " + field.Key,
			}
			if trigger != "" {
				system.Rules = append(system.Rules, "Trigger: "+trigger)
			}
			if response != "" {
				system.Rules = append(system.Rules, "System response: "+response)
			}
			systems = append(systems, system)
		}
	}
	return systems
}

func fieldText(status Status, number int, key string) string {
	for _, entry := range orderedEntries(status.Section(number).Content) {
		if entry.key == key {
			return entryText(entry.value)
		}
	}
	return ""
}

func joinNonEmpty(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ", ")
}

func splitSentences(text string) []string {
	var result []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

// citedReferences returns, in catalogue order, the references cited anywhere in the
// document, and the printed number of each catalogue id.
func citedReferences(ids []int) ([]DocReference, map[int]int) {
	seen := map[int]bool{}
	for _, id := range ids {
		if _, ok := referenceByID(id); ok {
			seen[id] = true
		}
	}
	var keys []int
	for id := range seen {
		keys = append(keys, id)
	}
	sort.Ints(keys)
	numbers := map[int]int{}
	var references []DocReference
	for i, id := range keys {
		reference, _ := referenceByID(id)
		references = append(references, reference)
		numbers[id] = i + 1
	}
	return references, numbers
}

// completeThinReport keeps what the AI wrote well (summary, loops, explained blocks,
// coherence) and adds the entered data where its explanations are missing.
func completeThinReport(doc *ReportDoc, status Status) *ReportDoc {
	data := fallbackDocument(ReportFormatReport, status).(*ReportDoc)
	facts := map[int][]ReportFact{}
	for _, section := range data.Sections {
		facts[section.Number] = section.Facts
	}

	byNumber := map[int]ReportSection{}
	for _, section := range doc.Sections {
		var kept []ReportBlock
		for _, block := range section.Blocks {
			if strings.TrimSpace(block.Why) != "" || len(block.InTheDesign) > 0 {
				kept = append(kept, block)
			}
		}
		section.Blocks = kept
		if len(kept) == 0 && len(section.Feedback) == 0 {
			section.Facts = facts[section.Number]
		}
		byNumber[section.Number] = section
	}

	var sections []ReportSection
	for number := 1; number <= SectionCount; number++ {
		if section, ok := byNumber[number]; ok {
			sections = append(sections, section)
		} else if len(facts[number]) > 0 {
			sections = append(sections, ReportSection{Number: number, Facts: facts[number]})
		}
	}
	doc.Sections = sections
	if strings.TrimSpace(doc.Title) == "" {
		doc.Title = data.Title
	}
	return doc
}

// firstBlockSample is the first report block as JSON (diagnostics for thin reports).
func firstBlockSample(doc *ReportDoc) string {
	for _, section := range doc.Sections {
		if len(section.Blocks) > 0 {
			data, _ := json.Marshal(section.Blocks[0])
			if len(data) > 600 {
				data = data[:600]
			}
			return string(data)
		}
	}
	data, _ := json.Marshal(doc.Sections)
	if len(data) > 600 {
		data = data[:600]
	}
	return string(data)
}

var inlineCitation = regexp.MustCompile(`\s*\[(\d+(?:\s*[,;]\s*\d+)*)\]`)

// extractCitations removes citation numbers the model wrote in the text ("… habits [8]")
// and adds them to refs, which the template prints with the right numbering.
func extractCitations(text string, refs []int) (string, []int) {
	for _, match := range inlineCitation.FindAllStringSubmatch(text, -1) {
		for _, part := range regexp.MustCompile(`\d+`).FindAllString(match[1], -1) {
			id, _ := strconv.Atoi(part)
			if _, ok := referenceByID(id); ok && !containsInt(refs, id) {
				refs = append(refs, id)
			}
		}
	}
	text = inlineCitation.ReplaceAllString(text, "")
	return strings.TrimSpace(strings.ReplaceAll(text, " .", ".")), refs
}

func containsInt(values []int, value int) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// tidyLoops drops loop labels that only repeat "core loop" / "long loop".
func tidyLoops(loops *DocLoops) {
	for _, label := range []*string{&loops.CoreLabel, &loops.LongLabel} {
		*label = strings.TrimSpace(loopWord.ReplaceAllString(*label, ""))
	}
	for _, loop := range []*DocLoop{&loops.Core, &loops.Long} {
		for i := range loop.Steps {
			// The template numbers the steps: drop "Step 1." / "Passo 1:" / "1." written by the model.
			if title := strings.TrimSpace(stepNumber.ReplaceAllString(loop.Steps[i].Title, "")); title != "" {
				loop.Steps[i].Title = title
			}
		}
	}
}

// loopWord matches a loop label that starts by repeating "core loop" / "long loop".
var loopWord = regexp.MustCompile(`(?i)^\s*(?:core|long)?\s*loop\s*[:·—–\-]?\s*`)

// stepNumber matches a step number at the start of a step title.
var stepNumber = regexp.MustCompile(`(?i)^\s*(?:(?:step|passo|fase)\s*)?\d+\s*[.:)·—–\-]\s*`)
