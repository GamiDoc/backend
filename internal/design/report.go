package design

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

type RenderSection struct {
	Number int
	Name   string
	Lines  []string
}

type ReportBuilder struct{}

func NewReportBuilder() *ReportBuilder {
	return &ReportBuilder{}
}

func (b *ReportBuilder) Build(title string, version string, spark string, sections []RenderSection) ([]byte, error) {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetTitle(title, false)
	doc.AddPage()

	doc.SetFont("Arial", "B", 16)
	doc.MultiCell(0, 8, title, "", "L", false)
	doc.Ln(2)

	doc.SetFont("Arial", "", 10)
	doc.Cell(0, 6, "Version: "+version)
	doc.Ln(6)
	doc.Cell(0, 6, "Date: "+time.Now().UTC().Format("2006-01-02 15:04:05"))
	doc.Ln(10)

	if strings.TrimSpace(spark) != "" {
		doc.SetFont("Arial", "B", 13)
		doc.Cell(0, 8, "The Spark")
		doc.Ln(8)
		doc.SetFont("Arial", "", 11)
		doc.MultiCell(0, 6, spark, "", "L", false)
		doc.Ln(4)
	}

	for _, section := range sections {
		doc.SetFont("Arial", "B", 13)
		doc.Cell(0, 8, fmt.Sprintf("%d. %s", section.Number, section.Name))
		doc.Ln(8)
		doc.SetFont("Arial", "", 11)
		if len(section.Lines) == 0 {
			doc.MultiCell(0, 6, "Not provided.", "", "L", false)
		}
		for _, line := range section.Lines {
			doc.MultiCell(0, 6, line, "", "L", false)
		}
		doc.Ln(4)
	}

	var buffer bytes.Buffer
	if err := doc.Output(&buffer); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func SectionLines(content json.RawMessage) []string {
	if len(content) == 0 {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return []string{string(content)}
	}

	if delim, ok := token.(json.Delim); ok && delim == '{' {
		var lines []string
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return []string{string(content)}
			}
			value, err := renderValue(decoder, true)
			if err != nil {
				return []string{string(content)}
			}
			lines = append(lines, fmt.Sprintf("%v: %s", keyToken, value))
		}
		return lines
	}

	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return []string{text}
	}

	return []string{string(content)}
}

func SectionPlainText(content json.RawMessage) string {
	return strings.Join(SectionLines(content), "\n")
}

func renderValue(decoder *json.Decoder, bareList bool) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}

	switch typed := token.(type) {
	case json.Delim:
		var parts []string
		if typed == '{' {
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return "", err
				}
				value, err := renderValue(decoder, false)
				if err != nil {
					return "", err
				}
				parts = append(parts, fmt.Sprintf("%v: %s", keyToken, value))
			}
			if _, err := decoder.Token(); err != nil {
				return "", err
			}
			return "{" + strings.Join(parts, ", ") + "}", nil
		}
		for decoder.More() {
			value, err := renderValue(decoder, false)
			if err != nil {
				return "", err
			}
			parts = append(parts, value)
		}
		if _, err := decoder.Token(); err != nil {
			return "", err
		}
		if bareList {
			return strings.Join(parts, ", "), nil
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case json.Number:
		return typed.String(), nil
	case string:
		return typed, nil
	case nil:
		return "", nil
	default:
		return fmt.Sprint(typed), nil
	}
}
