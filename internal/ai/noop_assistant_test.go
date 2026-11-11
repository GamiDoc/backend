package ai

import (
	"context"
	"errors"
	"testing"
)

func TestNoopRewrite(t *testing.T) {
	assistant := NewNoopAssistant()

	result, err := assistant.Rewrite(context.Background(), "  hello   world  ")
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Fatalf("unexpected rewrite %q", result)
	}

	if _, err := assistant.Rewrite(context.Background(), "   "); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("expected ErrEmptyText, got %v", err)
	}
}

func TestNoopCompleteJSONIsUnsupported(t *testing.T) {
	assistant := NewNoopAssistant()

	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "x"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestNoopChat(t *testing.T) {
	assistant := NewNoopAssistant()

	reply, err := assistant.Chat(context.Background(), Section{Number: 4, Name: "Gameful Core"}, "what goes here?")
	if err != nil {
		t.Fatal(err)
	}
	if reply == "" {
		t.Fatal("expected non-empty guidance")
	}
}

func TestNoopEnhanceDeduplicatesAcrossSections(t *testing.T) {
	assistant := NewNoopAssistant()

	result, err := assistant.Enhance(context.Background(), []SectionText{
		{Number: 1, Name: "Context", Text: "students commute daily\nwalking is healthy"},
		{Number: 6, Name: "Impacts & Benefits", Text: "walking is healthy\nless car traffic"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result))
	}
	if result[1].Text != "less car traffic" {
		t.Fatalf("expected duplicate line dropped, got %q", result[1].Text)
	}
}
