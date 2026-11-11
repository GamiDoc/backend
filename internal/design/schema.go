package design

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// The section schema (fields, options, conditions) is defined in the frontend
// (frontend_new/src/data/designSections.js) and exported here with
// `npm run export-schema`. It drives the AI prefill of the sections from the Spark.
//
//go:embed schema/sections.json
var sectionSchemaJSON []byte

type SchemaOption struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	Info        string `json:"info,omitempty"`
}

// ShowIf: the field is shown only if field Key equals (single) or contains (multi) Includes.
type ShowIf struct {
	Key      string `json:"key"`
	Includes string `json:"includes"`
	Keep     bool   `json:"keep"`
}

type SchemaField struct {
	Key         string         `json:"key"`
	Label       string         `json:"label,omitempty"`
	Title       string         `json:"title,omitempty"`
	Hint        string         `json:"hint,omitempty"`
	Group       string         `json:"group,omitempty"`
	Type        string         `json:"type"`
	Display     string         `json:"display,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	ItemLabel   string         `json:"itemLabel,omitempty"`
	Badge       string         `json:"badge,omitempty"`
	Description string         `json:"description,omitempty"`
	PopupIntro  string         `json:"popupIntro,omitempty"`
	Loop        string         `json:"loop,omitempty"`
	Options     []SchemaOption `json:"options,omitempty"`
	Other       bool           `json:"other,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Numbered    bool           `json:"numbered,omitempty"`
	ShowIf      []ShowIf       `json:"showIf,omitempty"`
	ItemFields  []SchemaField  `json:"itemFields,omitempty"`
}

type SchemaGroup struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type SchemaSection struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Summary     string        `json:"summary"`
	Optional    bool          `json:"optional"`
	Groups      []SchemaGroup `json:"groups"`
	Fields      []SchemaField `json:"fields"`
}

const (
	FieldTypeSingle = "single"
	FieldTypeMulti  = "multi"
	FieldTypeText   = "text"
	FieldTypeList   = "list"

	// OtherOption is the value of the "Other" choice of fields with `other`; its free
	// text is saved under OtherKey(field).
	OtherOption = "Other"
)

var sectionSchema = mustParseSchema(sectionSchemaJSON)

func mustParseSchema(data []byte) []SchemaSection {
	sections, err := parseSchema(data)
	if err != nil {
		panic(err)
	}
	return sections
}

func parseSchema(data []byte) ([]SchemaSection, error) {
	var decoded struct {
		Sections []SchemaSection `json:"sections"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("design schema: %w", err)
	}
	if len(decoded.Sections) != SectionCount {
		return nil, fmt.Errorf("design schema: expected %d sections, got %d", SectionCount, len(decoded.Sections))
	}
	for i, section := range decoded.Sections {
		if section.Number != i+1 {
			return nil, fmt.Errorf("design schema: section %d has number %d", i+1, section.Number)
		}
	}
	return decoded.Sections, nil
}

// SchemaFor returns the schema of section number (1..SectionCount).
func SchemaFor(number int) (SchemaSection, bool) {
	if number < 1 || number > len(sectionSchema) {
		return SchemaSection{}, false
	}
	return sectionSchema[number-1], true
}

func OtherKey(field SchemaField) string {
	return field.Key + " · Other"
}

// ListItemKey is the key under which a field of the n-th (0-based) item of a list is saved.
func ListItemKey(list SchemaField, index int, sub string) string {
	return fmt.Sprintf("%s %d · %s", list.Key, index+1, sub)
}

// fillable reports whether the AI may fill the field: images and loop containers have no
// value of their own.
func (f SchemaField) fillable() bool {
	return f.Display != "image" && f.Display != "loop"
}

// linksMechanics: the field's values are names of game mechanics described in Gameful Core.
func (f SchemaField) linksMechanics() bool {
	return f.Display == "mechanics"
}

func (f SchemaField) optionValues() []string {
	values := make([]string, 0, len(f.Options)+1)
	for _, option := range f.Options {
		values = append(values, option.Value)
	}
	if f.Other && !containsString(values, OtherOption) {
		values = append(values, OtherOption)
	}
	return values
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
