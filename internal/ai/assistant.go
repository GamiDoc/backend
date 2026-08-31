package ai

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrEmptyText = errors.New("empty text")

type Section struct {
	Number int
	Name   string
}

type SectionText struct {
	Number int
	Name   string
	Text   string
}

type Assistant interface {
	Rewrite(ctx context.Context, text string) (string, error)
	Chat(ctx context.Context, section Section, message string) (string, error)
	RecommendBranch(ctx context.Context, spark string) (string, error)
	Prefill(ctx context.Context, spark string, sections []Section) (map[int]json.RawMessage, error)
	Enhance(ctx context.Context, sections []SectionText) ([]SectionText, error)
}
