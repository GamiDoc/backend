package design

import "testing"

func TestTidyDocumentDropsInternalKeysAsTerms(t *testing.T) {
	doc := &ReportDoc{Title: "GreenStep", Sections: []ReportSection{{Number: 6, Blocks: []ReportBlock{
		{Title: "Platforms and Target OS", Term: "platform"},
		{Title: "Frame rate", Term: "performance_session"},
		{Title: "What brings users back", Term: "Retention anchors"},
	}}}}
	tidyDocument(doc)
	blocks := doc.Sections[0].Blocks
	if blocks[0].Term != "" || blocks[1].Term != "" || blocks[2].Term != "Retention anchors" {
		t.Fatalf("terms: %q %q %q", blocks[0].Term, blocks[1].Term, blocks[2].Term)
	}
}
