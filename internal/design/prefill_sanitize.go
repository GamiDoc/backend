package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

/* Sanitizing the model's answer for a section before it is stored.

   The stored content follows the frontend contract (designSections.js → valuesToContent):
   - keys are field keys, values are strings (single, text) or arrays of strings (multi);
   - the "Other" text of a field is saved under "<key> · Other", right after the field;
   - list items are flattened as "<list key> <n> · <item field>" ("<n> · Name" first);
   - empty and hidden (showIf) fields are left out; keys follow the schema order, since the
     PDF prints them in insertion order.
   Anything the model invents (options, keys, mechanics) is dropped. */

type sectionAnswer struct {
	Values map[string]json.RawMessage              `json:"values"`
	Lists  map[string][]map[string]json.RawMessage `json:"lists"`
	Notes  []FieldNote                             `json:"notes"`
}

// SectionDraft is the sanitized AI answer for one section.
type SectionDraft struct {
	Content json.RawMessage // nil if nothing usable
	Notes   []FieldNote
	// Dropped lists what the model answered but could not be stored ("key: value (reason)"),
	// MissingRequired the required fields left empty: diagnostics for the logs.
	Dropped         []string
	MissingRequired []string
}

// dropLog collects the values the sanitizer discards.
type dropLog struct{ entries []string }

func (d *dropLog) add(key string, value any, reason string) {
	if d == nil {
		return
	}
	encoded, _ := json.Marshal(value)
	text := string(encoded)
	if runes := []rune(text); len(runes) > 120 {
		text = string(runes[:120]) + "…"
	}
	d.entries = append(d.entries, fmt.Sprintf("%s: %s (%s)", key, text, reason))
}

func sanitizeSectionAnswer(section SchemaSection, raw json.RawMessage, mechanics []string) (SectionDraft, error) {
	var answer sectionAnswer
	if err := json.Unmarshal(raw, &answer); err != nil {
		return SectionDraft{}, fmt.Errorf("unparseable section answer: %w", err)
	}
	if answer.Values == nil {
		answer.Values = map[string]json.RawMessage{}
	}
	drops := &dropLog{}
	known := map[string]bool{}
	for _, field := range section.Fields {
		known[field.Key] = true
		known[OtherKey(field)] = true
	}
	for key, value := range answer.Values {
		if !known[key] && len(value) > 0 && string(value) != `""` && string(value) != "[]" && string(value) != "null" {
			drops.add(key, json.RawMessage(value), "unknown field")
		}
	}

	// Values first (conditions of the lists depend on them).
	values := map[string]any{}
	for _, field := range section.Fields {
		if !field.fillable() || field.Type == FieldTypeList {
			continue
		}
		if !isVisible(field, values) {
			if raw := answer.Values[field.Key]; len(listOf(raw)) > 0 {
				drops.add(field.Key, raw, "hidden by its condition")
			}
			continue
		}
		value, other := sanitizeValue(field, answer.Values[field.Key], answer.Values[OtherKey(field)], nil, drops.scoped(field.Key))
		if value == nil {
			continue
		}
		values[field.Key] = value
		if other != "" {
			values[OtherKey(field)] = other
		}
	}

	// Lists: first the items and their names, then the links between mechanics, which may
	// point to any mechanic of this section or of Gameful Core.
	type listItems struct {
		field SchemaField
		items []map[string]json.RawMessage
		names []string
	}
	var lists []listItems
	allowed := append([]string{}, mechanics...)
	for _, field := range section.Fields {
		if field.Type != FieldTypeList {
			continue
		}
		if !isVisible(field, values) {
			if len(answer.Lists[field.Key]) > 0 {
				drops.add(field.Key, fmt.Sprintf("%d items", len(answer.Lists[field.Key])), "hidden by its condition")
			}
			continue
		}
		raw := answer.Lists[field.Key]
		if raw == nil {
			raw = decodeItems(answer.Values[field.Key]) // tolerate lists put among the values
		}
		entry := listItems{field: field}
		for _, item := range raw {
			if !itemHasContent(field, item) {
				continue
			}
			name := fmt.Sprintf("%s %d", field.ItemLabel, len(entry.items)+1)
			if !field.Numbered {
				name = strings.TrimSpace(textOf(item["Name"]))
			}
			entry.items = append(entry.items, item)
			entry.names = append(entry.names, name)
			if field.Numbered && strings.HasSuffix(field.Key, " mechanics") {
				allowed = append(allowed, name)
			}
		}
		if len(entry.items) > 0 {
			lists = append(lists, entry)
		}
	}

	var content orderedContent
	keys := map[string]bool{}
	put := func(key string, value any) {
		content = append(content, contentEntry{key: key, value: value})
		keys[key] = true
	}

	listsByKey := map[string]listItems{}
	for _, entry := range lists {
		listsByKey[entry.field.Key] = entry
	}
	for _, field := range section.Fields {
		if field.Type == FieldTypeList {
			entry, ok := listsByKey[field.Key]
			if !ok {
				continue
			}
			for index, item := range entry.items {
				if entry.names[index] != "" {
					put(ListItemKey(field, index, "Name"), entry.names[index])
				}
				for _, sub := range field.ItemFields {
					if !sub.fillable() {
						continue
					}
					value, other := sanitizeValue(sub, item[sub.Key], item[OtherKey(sub)], allowed, drops.scoped(ListItemKey(field, index, sub.Key)))
					if value == nil {
						continue
					}
					put(ListItemKey(field, index, sub.Key), value)
					if other != "" {
						put(ListItemKey(field, index, OtherKey(sub)), other)
					}
				}
			}
			keys[field.Key] = true // notes may refer to the list as a whole
			continue
		}
		value, ok := values[field.Key]
		if !ok {
			continue
		}
		put(field.Key, value)
		if other, ok := values[OtherKey(field)]; ok {
			put(OtherKey(field), other)
		}
	}
	draft := SectionDraft{Notes: sanitizeNotes(answer.Notes, keys), Dropped: drops.entries}
	for _, field := range section.Fields {
		if field.Required && field.fillable() && !keys[field.Key] && isVisible(field, values) {
			draft.MissingRequired = append(draft.MissingRequired, field.Key)
		}
	}

	if len(content) == 0 {
		return draft, nil
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return SectionDraft{}, err
	}
	draft.Content = encoded
	return draft, nil
}

// scopedDrop records the discarded values of one field.
type scopedDrop struct {
	log *dropLog
	key string
}

func (d *dropLog) scoped(key string) scopedDrop {
	return scopedDrop{log: d, key: key}
}

func (d scopedDrop) add(value any, reason string) {
	d.log.add(d.key, value, reason)
}

// sanitizeValue returns the stored value of a field (string or []string; nil = not filled)
// and its "Other" text. allowedMechanics restricts fields that link game mechanics.
func sanitizeValue(field SchemaField, raw json.RawMessage, rawOther json.RawMessage, allowedMechanics []string, drop scopedDrop) (any, string) {
	otherText := strings.TrimSpace(textOf(rawOther))

	switch field.Type {
	case FieldTypeSingle:
		text := strings.TrimSpace(textOf(raw))
		if text == "" {
			return nil, ""
		}
		if option, ok := matchOption(field, text); ok {
			if option == OtherOption && field.Other {
				if otherText == "" {
					return nil, ""
				}
				return option, otherText
			}
			return option, ""
		}
		if field.Other {
			return OtherOption, text
		}
		drop.add(text, "not an option")
		return nil, ""

	case FieldTypeMulti:
		var selected []string
		var unknown []string
		for _, item := range listOf(raw) {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if field.linksMechanics() {
				switch {
				case !containsString(allowedMechanics, item):
					drop.add(item, "unknown mechanic")
				case !containsString(selected, item):
					selected = append(selected, item)
				}
				continue
			}
			option, ok := matchOption(field, item)
			switch {
			case ok && !containsString(selected, option):
				selected = append(selected, option)
			case !ok:
				unknown = append(unknown, item)
			}
		}
		if field.Other {
			if len(unknown) > 0 {
				otherText = strings.Join(append(nonEmpty(otherText), unknown...), "; ")
			}
			hasOther := containsString(selected, OtherOption)
			switch {
			case otherText != "" && !hasOther:
				selected = append(selected, OtherOption)
			case otherText == "" && hasOther:
				selected = removeString(selected, OtherOption)
			}
		} else {
			for _, item := range unknown {
				drop.add(item, "not an option")
			}
			otherText = ""
		}
		if len(selected) == 0 {
			return nil, ""
		}
		return selected, otherText

	default:
		text := strings.TrimSpace(textOf(raw))
		if text == "" {
			return nil, ""
		}
		return text, ""
	}
}

func matchOption(field SchemaField, value string) (string, bool) {
	for _, option := range field.optionValues() {
		if strings.EqualFold(strings.TrimSpace(option), value) {
			return option, true
		}
	}
	return "", false
}

// isVisible mirrors isFieldVisible of the frontend, except for conditions with Keep: those
// only pick the tab shown on screen (e.g. which game element is open), and the frontend
// keeps the values of hidden tabs, so the AI may fill every tab (mechanics of several
// elements and categories). Fields hidden by other conditions are never pre-filled.
func isVisible(field SchemaField, values map[string]any) bool {
	for _, condition := range field.ShowIf {
		if condition.Keep {
			continue
		}
		switch value := values[condition.Key].(type) {
		case string:
			if value != condition.Includes {
				return false
			}
		case []string:
			if !containsString(value, condition.Includes) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func itemHasContent(list SchemaField, item map[string]json.RawMessage) bool {
	for _, sub := range list.ItemFields {
		if !sub.fillable() || sub.linksMechanics() {
			continue
		}
		if value, _ := sanitizeValue(sub, item[sub.Key], item[OtherKey(sub)], nil, scopedDrop{}); value != nil {
			return true
		}
	}
	return false
}

func sanitizeNotes(notes []FieldNote, keys map[string]bool) []FieldNote {
	var result []FieldNote
	seen := map[string]bool{}
	for _, note := range notes {
		field := strings.TrimSuffix(strings.TrimSpace(note.Field), " · Other")
		reason := strings.TrimSpace(note.Reason)
		if !keys[field] || reason == "" || seen[field] {
			continue
		}
		seen[field] = true
		if runes := []rune(reason); len(runes) > 300 {
			reason = string(runes[:300]) + "…"
		}
		confidence := strings.ToLower(strings.TrimSpace(note.Confidence))
		if confidence != ConfidenceLow && confidence != ConfidenceHigh {
			confidence = ConfidenceMedium
		}
		result = append(result, FieldNote{Field: field, Reason: reason, Confidence: confidence})
	}
	return result
}

var mechanicNamePattern = regexp.MustCompile(`^.+ mechanics (\d+) · Name$`)

// mechanicNamesFrom lists the names of the game mechanics in Gameful Core content, in the
// order they are stored.
func mechanicNamesFrom(content json.RawMessage) []string {
	var names []string
	for _, entry := range orderedEntries(content) {
		if !mechanicNamePattern.MatchString(entry.key) {
			continue
		}
		if name := strings.TrimSpace(textOf(entry.value)); name != "" && !containsString(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// ── JSON helpers ────────────────────────────────────────────────────────────

type contentEntry struct {
	key   string
	value any
}

// orderedContent is a JSON object that keeps its keys in insertion order.
type orderedContent []contentEntry

func (c orderedContent) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, entry := range c {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(entry.key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(entry.value)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

type rawEntry struct {
	key   string
	value json.RawMessage
}

// orderedEntries returns the top-level keys of a JSON object in document order (nil if
// content is not an object).
func orderedEntries(content json.RawMessage) []rawEntry {
	if len(content) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil
	}
	var entries []rawEntry
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return entries
		}
		key, ok := token.(string)
		if !ok {
			return entries
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return entries
		}
		entries = append(entries, rawEntry{key: key, value: value})
	}
	return entries
}

func textOf(raw json.RawMessage) string {
	var text string
	if len(raw) == 0 || json.Unmarshal(raw, &text) != nil {
		return ""
	}
	return text
}

func listOf(raw json.RawMessage) []string {
	var list []string
	if len(raw) == 0 {
		return nil
	}
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	if text := textOf(raw); text != "" {
		return []string{text}
	}
	return nil
}

func decodeItems(raw json.RawMessage) []map[string]json.RawMessage {
	var items []map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &items) != nil {
		return nil
	}
	return items
}

func nonEmpty(text string) []string {
	if text == "" {
		return nil
	}
	return []string{text}
}

func removeString(values []string, value string) []string {
	result := values[:0:0]
	for _, v := range values {
		if v != value {
			result = append(result, v)
		}
	}
	return result
}

// sortedInts returns a sorted copy without duplicates.
func sortedInts(values []int) []int {
	seen := map[int]bool{}
	var result []int
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	sort.Ints(result)
	return result
}
