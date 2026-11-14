package docname

import "testing"

func TestFileName(t *testing.T) {
	cases := map[string]string{
		"GreenStep":             "GreenStep_design-report.pdf",
		"  Città   verde! 2.0 ": "Citta-verde-2-0_design-report.pdf",
		"":                      "your-project_design-report.pdf",
		"日本":                    "your-project_design-report.pdf",
		"Your project":          "Your-project_design-report.pdf",
	}
	for title, want := range cases {
		if got := FileName(title, DesignReport); got != want {
			t.Errorf("FileName(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestFromKey(t *testing.T) {
	if got := FromKey("design/projects/1/abc/GreenStep_game-design-document.pdf", "x.pdf"); got != "GreenStep_game-design-document.pdf" {
		t.Errorf("got %q", got)
	}
	if got := FromKey("design/projects/1/0a80ca6c-06f9-45b0-becb-3dbe4906f26f.pdf", "x.pdf"); got != "x.pdf" {
		t.Errorf("old key: got %q", got)
	}
}

func TestFromKeyLegacyTimestamp(t *testing.T) {
	if got := FromKey("projects/1/1794591771648.pdf", "x.pdf"); got != "x.pdf" {
		t.Errorf("got %q", got)
	}
	if got := FromKey("projects/1/9/My-plan.pdf", "x.pdf"); got != "My-plan.pdf" {
		t.Errorf("renamed: got %q", got)
	}
}

func TestCustom(t *testing.T) {
	cases := map[string]string{
		"My report v2":        "My-report-v2.pdf",
		"prova_design-report": "prova_design-report.pdf",
		"Città verde.PDF":     "Citta-verde.pdf",
	}
	for name, want := range cases {
		if got, err := Custom(name); err != nil || got != want {
			t.Errorf("Custom(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := Custom("  ?? "); err != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}
