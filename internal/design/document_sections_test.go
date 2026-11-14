package design

import (
	"encoding/json"
	"testing"
)

func TestAddMissingSectionsKeepsTechnicalRequirements(t *testing.T) {
	status := NewInitialStatus()
	status.Sections["1"] = SectionState{Content: json.RawMessage(`{"Domain":["Education"]}`)}
	status.Sections["6"] = SectionState{Content: json.RawMessage(`{"Platform notes":"Web, mobile first"}`)}
	doc := &ReportDoc{Sections: []ReportSection{{Number: 1, InShort: "AI text"}}}

	addMissingSections(doc, status)

	if len(doc.Sections) != 2 || doc.Sections[0].Number != 1 || doc.Sections[1].Number != 6 {
		t.Fatalf("sections: %+v", doc.Sections)
	}
	if doc.Sections[0].InShort != "AI text" || len(doc.Sections[1].Facts) == 0 {
		t.Fatalf("unexpected content: %+v", doc.Sections)
	}
}
