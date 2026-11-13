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
	if got := FromKey("design/projects/1/abc.pdf", "x.pdf"); got != "x.pdf" {
		t.Errorf("old key: got %q", got)
	}
}
