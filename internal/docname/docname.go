// Package docname names the generated documents: "<project>_<document type>.pdf".
package docname

import (
	"path"
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

// FromKey is the download name of a stored PDF: the file name of its key when it was
// saved with FileName, else fallback (older files are stored as "<id>.pdf").
func FromKey(key string, fallback string) string {
	base := path.Base(key)
	if strings.Contains(base, "_") && strings.HasSuffix(base, ".pdf") {
		return base
	}
	return fallback
}
