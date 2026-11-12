package design

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

//go:embed doctemplates/*
var docTemplateFiles embed.FS

// HTMLRenderer turns a complete HTML document into a PDF (Chromium via Gotenberg, or a
// local headless Chrome/Edge).
type HTMLRenderer interface {
	RenderHTML(ctx context.Context, html string) ([]byte, error)
}

var docTemplates = template.Must(template.New("docs").Funcs(template.FuncMap{
	"add": func(a, b int) int { return a + b },
	// Replaced per document (references are numbered in the order they are printed).
	"cite":         func([]int) string { return "" },
	"sectionTitle": func(int) string { return "" },
	"chips":        func(values []string) []string { return splitValues(values, true) },
	"longValues":   func(values []string) []string { return splitValues(values, false) },
}).ParseFS(docTemplateFiles, "doctemplates/*.html"))

var docCSS = func() template.CSS {
	data, err := docTemplateFiles.ReadFile("doctemplates/style.css")
	if err != nil {
		panic(err)
	}
	return template.CSS(data)
}()

type docImage struct {
	Label string
	Src   template.URL // data: URI
}

type docView struct {
	Doc         any
	FormatTitle string
	DocumentID  string
	Date        string
	Note        string
	FooterCSS   template.CSS // CSS string literal with the page footer text
	Spark       string
	CSS         template.CSS
	Contents    []string
	Glossary    []GlossaryGroup
	References  []DocReference
	Images      map[int][]docImage
	AllImages   []docImage
}

const aiNote = "Written with AI from the data entered in the GamiDoc Design System. Review it before use."
const plainNote = "Generated from the data entered in the GamiDoc Design System (without AI explanations)."

// RenderDocumentHTML renders a document (*ReportDoc or *GDDDoc) as a complete HTML page.
func RenderDocumentHTML(document any, format string, status Status, documentID string, images map[int][]RenderImage, withAI bool) (string, error) {
	view := docView{
		Doc:         document,
		FormatTitle: ReportFormatTitle(format),
		DocumentID:  documentID,
		Date:        time.Now().UTC().Format("2 January 2006"),
		Note:        aiNote,
		Spark:       strings.TrimSpace(status.Spark),
		CSS:         docCSS,
		Glossary:    buildGlossary(status),
		Images:      map[int][]docImage{},
	}
	if !withAI {
		view.Note = plainNote
	}
	for number, list := range images {
		for _, image := range list {
			mime := "image/png"
			if image.Type == "JPG" {
				mime = "image/jpeg"
			}
			converted := docImage{
				Label: image.Label,
				Src:   template.URL("data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image.Data)),
			}
			view.Images[number] = append(view.Images[number], converted)
			view.AllImages = append(view.AllImages, converted)
		}
	}

	var cited []int
	switch doc := document.(type) {
	case *ReportDoc:
		for _, section := range doc.Sections {
			for _, block := range section.Blocks {
				cited = append(cited, block.Refs...)
			}
		}
		view.Contents = []string{"The design at a glance"}
		for _, section := range doc.Sections {
			view.Contents = append(view.Contents, sectionTitle(section.Number))
		}
		if len(doc.Coherence) > 0 {
			view.Contents = append(view.Contents, "How the choices fit together")
		}
		if len(doc.Evaluation) > 0 {
			view.Contents = append(view.Contents, "Link to the evaluation")
		}
		if len(view.Glossary) > 0 {
			view.Contents = append(view.Contents, "Glossary")
		}
	case *GDDDoc:
		for _, rationale := range doc.Rationale {
			cited = append(cited, rationale.Refs...)
		}
		view.Contents = []string{"Design on one page"}
		for _, entry := range []struct {
			name    string
			present bool
		}{
			{"Players", len(doc.PlayerAspects)+len(doc.PlayerTypes) > 0},
			{"Core gameplay", len(doc.LoopRows)+len(doc.GlobalRules) > 0},
			{"Game systems", len(doc.Systems) > 0},
			{"Progression and rewards", doc.Progression != "" || len(doc.Rewards) > 0},
			{"Player journey", len(doc.Journey) > 0},
			{"Adaptation rules", len(doc.Adaptation) > 0},
			{"Feedback and screens", len(doc.Feedback) > 0},
			{"Technical requirements", len(doc.Technical) > 0},
			{"Risks", len(doc.Risks) > 0},
			{"Success metrics", len(doc.Metrics) > 0},
			{"Appendix A · Design rationale", len(doc.Rationale) > 0},
			{"Appendix B · Glossary and references", true},
		} {
			if entry.present {
				view.Contents = append(view.Contents, entry.name)
			}
		}
	default:
		return "", ErrInvalidReportVersion
	}
	references, numbers := citedReferences(cited)
	view.References = references
	if _, isReport := document.(*ReportDoc); isReport && len(references) > 0 {
		view.Contents = append(view.Contents, "References")
	}
	// The footer names the document, not the project: a long title would wrap over the page
	// number (the full title is on the cover).
	view.FooterCSS = cssString("GamiDoc · " + view.FormatTitle + " · " + documentID)

	tpl, err := docTemplates.Clone()
	if err != nil {
		return "", err
	}
	tpl.Funcs(template.FuncMap{
		"cite": func(ids []int) string {
			var parts []string
			for _, id := range ids {
				if number, ok := numbers[id]; ok {
					parts = append(parts, fmt.Sprint(number))
				}
			}
			if len(parts) == 0 {
				return ""
			}
			return " [" + strings.Join(parts, ", ") + "]"
		},
		"sectionTitle": sectionTitle,
	})

	var out bytes.Buffer
	name := "report"
	if format == ReportFormatGDD {
		name = "gdd"
	}
	if err := tpl.ExecuteTemplate(&out, name, view); err != nil {
		return "", err
	}
	return out.String(), nil
}

// sectionTitle is the reader-facing title of a design section in the report.
func sectionTitle(number int) string {
	switch number {
	case 1:
		return "Context"
	case 2:
		return "Experience over time"
	case 3:
		return "Personalisation"
	case 4:
		return "The game"
	case 5:
		return "Feedback and evaluation"
	case 6:
		return "Technical requirements"
	}
	return SectionName(number)
}

// ── Plain PDF when no HTML renderer is available ────────────────────────────

// RenderDocumentPlainPDF prints the document as a simple text PDF (headings, paragraphs,
// lists), used only when no Chromium-based renderer is configured or it fails.
func RenderDocumentPlainPDF(document any, format string, status Status, documentID string) ([]byte, error) {
	w := newPlainWriter()
	switch doc := document.(type) {
	case *ReportDoc:
		w.cover(doc.Title, ReportFormatTitle(format), doc.Subtitle, documentID)
		w.heading("The design at a glance")
		w.pair("Who it is for", doc.Glance.ForWhom)
		w.pair("What we want them to do", doc.Glance.WhatWeWant)
		w.pair("What motivates them", doc.Glance.Motivation)
		w.pair("How we will know it works", doc.Glance.HowWeKnow)
		w.pair("The main risk", doc.Glance.MainRisk)
		w.pair("How we address it", doc.Glance.Addressing)
		for _, section := range doc.Sections {
			w.heading(fmt.Sprintf("%d. %s", section.Number, sectionTitle(section.Number)))
			w.paragraph(section.InShort)
			for _, fact := range section.Facts {
				w.pair(fact.Label, fact.Value)
			}
			for _, block := range section.Blocks {
				w.subheading(block.Title)
				w.paragraph(strings.Join(block.Values, " · "))
				w.pair("Why", block.Why)
				w.list(block.InTheDesign)
				w.pair("Watch out", block.WatchOut)
			}
			for _, fb := range section.Feedback {
				w.subheading(fb.ID + " " + fb.Moment)
				w.paragraph(fb.Message)
				w.pair("When", fb.When)
				w.pair("Channel", fb.Channel)
				w.pair("Why this way", fb.WhyThisWay)
			}
			w.paragraph(section.Closing)
		}
		if len(doc.Coherence) > 0 {
			w.heading("How the choices fit together")
			for _, row := range doc.Coherence {
				w.pair(row.Outcome, row.Choices+" — "+row.Handling)
			}
		}
		if len(doc.Evaluation) > 0 {
			w.heading("Link to the evaluation")
			for _, row := range doc.Evaluation {
				w.pair(row.Choice, row.Goal+" — "+row.Measure)
			}
		}
	case *GDDDoc:
		w.cover(doc.Title, ReportFormatTitle(format), doc.Subtitle, documentID)
		w.heading("Design on one page")
		w.paragraph(doc.HighConcept)
		w.pair("Domain", doc.Domain)
		w.pair("Players", doc.Players)
		w.pair("Target behaviour", doc.TargetBehaviour)
		w.pair("Modality", doc.Modality)
		for _, pillar := range doc.Pillars {
			w.pair(pillar.Title, pillar.Text)
		}
		if len(doc.PlayerAspects) > 0 {
			w.heading("Players")
			for _, row := range doc.PlayerAspects {
				w.pair(row.Name, row.Value)
			}
		}
		if len(doc.GlobalRules) > 0 {
			w.heading("Global rules")
			w.list(doc.GlobalRules)
		}
		if len(doc.Systems) > 0 {
			w.heading("Game systems")
			for _, system := range doc.Systems {
				w.subheading(system.ID + " · " + system.Name)
				w.pair("Purpose", system.Purpose)
				w.list(system.Rules)
				for _, value := range system.Values {
					w.pair(value.Name, value.Value)
				}
				w.list(system.Feedback)
				w.pair("Risks and safeguards", system.Risks)
			}
		}
		if len(doc.Feedback) > 0 {
			w.heading("Feedback")
			for _, fb := range doc.Feedback {
				w.pair(fb.ID+" "+fb.Moment, fb.Message+" ("+fb.Channel+")")
			}
		}
		if len(doc.Technical) > 0 {
			w.heading("Technical requirements")
			for _, row := range doc.Technical {
				w.pair(row.Name, row.Value)
			}
		}
		if len(doc.Risks) > 0 {
			w.heading("Risks")
			for _, row := range doc.Risks {
				w.pair(row.Risk, row.Mitigation)
			}
		}
		if len(doc.Metrics) > 0 {
			w.heading("Success metrics")
			for _, row := range doc.Metrics {
				w.pair(row.Metric, row.Target+" — "+row.How)
			}
		}
	default:
		return nil, ErrInvalidReportVersion
	}
	return w.output()
}

type plainWriter struct {
	doc       *fpdf.Fpdf
	translate func(string) string
}

func newPlainWriter() *plainWriter {
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetMargins(18, 18, 18)
	doc.SetAutoPageBreak(true, 18)
	doc.AddPage()
	return &plainWriter{doc: doc, translate: doc.UnicodeTranslatorFromDescriptor("")}
}

func (w *plainWriter) text(value string) string {
	replacer := strings.NewReplacer("→", "->", "↺", "", "—", "-", "–", "-", "“", "\"", "”", "\"", "’", "'", "…", "...", "₂", "2", "·", "-")
	return w.translate(replacer.Replace(value))
}

func (w *plainWriter) cover(title, format, subtitle, id string) {
	w.doc.SetFont("Helvetica", "", 9)
	w.doc.MultiCell(0, 5, w.text("GAMIDOC · "+strings.ToUpper(format)+" · "+id), "", "L", false)
	w.doc.SetFont("Helvetica", "B", 18)
	w.doc.MultiCell(0, 9, w.text(title), "", "L", false)
	w.paragraph(subtitle)
	w.doc.SetFont("Helvetica", "I", 8)
	w.doc.MultiCell(0, 4.5, w.text(plainNote), "", "L", false)
	w.doc.Ln(4)
}

func (w *plainWriter) heading(value string) {
	w.doc.Ln(3)
	w.doc.SetFont("Helvetica", "B", 13)
	w.doc.MultiCell(0, 7, w.text(value), "", "L", false)
}

func (w *plainWriter) subheading(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	w.doc.SetFont("Helvetica", "B", 10.5)
	w.doc.MultiCell(0, 6, w.text(value), "", "L", false)
}

func (w *plainWriter) paragraph(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	w.doc.SetFont("Helvetica", "", 10)
	w.doc.MultiCell(0, 5, w.text(value), "", "L", false)
	w.doc.Ln(1)
}

func (w *plainWriter) pair(label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	w.doc.SetFont("Helvetica", "B", 10)
	w.doc.Write(5, w.text(label+": "))
	w.doc.SetFont("Helvetica", "", 10)
	w.doc.Write(5, w.text(value))
	w.doc.Ln(6)
}

func (w *plainWriter) list(items []string) {
	w.doc.SetFont("Helvetica", "", 10)
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			w.doc.MultiCell(0, 5, w.text("- "+item), "", "L", false)
		}
	}
}

func (w *plainWriter) output() ([]byte, error) {
	var buffer bytes.Buffer
	if err := w.doc.Output(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// cssString quotes text as a CSS string literal, escaping every character that is not a
// plain letter, digit or space (html/template refuses dynamic text in @page rules).
func cssString(text string) template.CSS {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch {
		// A space right after an escape would be eaten by the CSS parser: spaces are escaped too.
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\%06X", r)
		}
	}
	b.WriteByte('"')
	return template.CSS(b.String())
}

// splitValues separates option names (shown as chips) from longer free text (shown as
// paragraphs, never cut).
func splitValues(values []string, short bool) []string {
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if (len([]rune(value)) <= 60) == short {
			result = append(result, value)
		}
	}
	return result
}
