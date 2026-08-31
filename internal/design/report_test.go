package design

import (
	"encoding/json"
	"testing"
)

func TestSectionLinesPreservesNumbersAndOrder(t *testing.T) {
	content := json.RawMessage(`{"zeta":"first","budget":1500000,"alpha":{"y":1,"x":2},"tags":["b","a"],"big":9007199254740993}`)

	lines := SectionLines(content)

	expected := []string{
		"zeta: first",
		"budget: 1500000",
		"alpha: {y: 1, x: 2}",
		"tags: b, a",
		"big: 9007199254740993",
	}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Fatalf("line %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

func TestSectionLinesNestedArrays(t *testing.T) {
	lines := SectionLines(json.RawMessage(`{"grid":[[1,2],[3,4]],"flag":true,"none":null}`))

	expected := []string{
		"grid: [1, 2], [3, 4]",
		"flag: true",
		"none: ",
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Fatalf("line %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

func TestSectionLinesScalarForms(t *testing.T) {
	if lines := SectionLines(json.RawMessage(`"just text"`)); len(lines) != 1 || lines[0] != "just text" {
		t.Fatalf("unexpected string handling: %v", lines)
	}
	if lines := SectionLines(nil); lines != nil {
		t.Fatalf("expected nil for empty content, got %v", lines)
	}
}
