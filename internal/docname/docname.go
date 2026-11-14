// Package docname names the generated documents: "<project>_<document type>.pdf".
package docname

import (
	"errors"
	"path"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// DefaultTitle names a project without a name.
const DefaultTitle = "Your project"

// Document types used in the file names.
const (
	DesignReport       = "design-report"
	GameDesignDocument = "game-design-document"
	EvaluationPlan     = "evaluation-plan"
	EvaluationReport   = "evaluation-report"
)

// FileName is "<Title>_<docType>.pdf", e.g. "GreenStep_design-report.pdf": ASCII letters,
// digits and dashes only, so it is safe in storage keys, URLs and download headers.
func FileName(title string, docType string) string {
	return Slug(title) + "_" + docType + ".pdf"
}

// Slug keeps letters and digits (accents removed) and turns the rest into single dashes;
// an empty result becomes "your-project".
func Slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(title) {
		if b.Len() >= 60 {
			break
		}
		switch {
		case unicode.Is(unicode.Mn, r):
			// accent of the previous letter
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "your-project"
	}
	return slug
}

// legacyName matches the names of files stored before the documents had names: a uuid
// ("<report id>.pdf") or a timestamp ("<unix nano>.pdf").
var legacyName = regexp.MustCompile(`^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9]+)\.pdf$`)

// FromKey is the download name of a stored PDF: the file name of its key, or fallback
// for files stored before the documents had names.
func FromKey(key string, fallback string) string {
	base := path.Base(key)
	if !strings.HasSuffix(base, ".pdf") || legacyName.MatchString(base) {
		return fallback
	}
	return base
}

// ErrInvalidName: the new name of a document has no usable character.
var ErrInvalidName = errors.New("invalid document name")

// Custom is the file name for a name typed by the user ("My report v2" -> "My-report-v2.pdf"):
// letters, digits, "_" and "-" are kept (accents removed), the rest becomes "-".
func Custom(name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name = name[:len(name)-4]
	}
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(name) {
		if b.Len() >= 100 {
			break
		}
		switch {
		case unicode.Is(unicode.Mn, r):
			// accent of the previous letter
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'):
			b.WriteRune(r)
			dash = r == '-'
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-_")
	if slug == "" {
		return "", ErrInvalidName
	}
	return slug + ".pdf", nil
}
