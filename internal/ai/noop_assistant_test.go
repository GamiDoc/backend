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

func TestNoopRecommendBranch(t *testing.T) {
	assistant := NewNoopAssistant()

	branch, err := assistant.RecommendBranch(context.Background(), "points, badges and a leaderboard on a mobile app")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "B" {
		t.Fatalf("expected B, got %s", branch)
	}

	branch, err = assistant.RecommendBranch(context.Background(), "the player journey and user experience over time")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "A" {
		t.Fatalf("expected A, got %s", branch)
	}
}

func TestNoopPrefill(t *testing.T) {
	assistant := NewNoopAssistant()

	prefill, err := assistant.Prefill(context.Background(), "a commuting game", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefill) != 1 {
		t.Fatalf("expected one prefilled section, got %d", len(prefill))
	}
	if _, ok := prefill[1]; !ok {
		t.Fatal("expected section 1 prefilled")
	}

	empty, err := assistant.Prefill(context.Background(), "   ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no prefill, got %d", len(empty))
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
