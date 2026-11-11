package ai

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrEmptyText = errors.New("empty text")

// ErrUnsupported is returned by assistants that cannot perform an operation (e.g. the
// noop assistant has no model behind it to produce structured answers).
var ErrUnsupported = errors.New("operation not supported by the ai provider")

type Section struct {
	Number int
	Name   string
}

type SectionText struct {
	Number int
	Name   string
	Text   string
}

// JSONRequest asks the model for an answer that is a JSON value matching Schema
// (a JSON Schema document). Name identifies the schema towards the provider.
type JSONRequest struct {
	Name   string
	System string
	User   string
	Schema json.RawMessage
}

type Assistant interface {
	Rewrite(ctx context.Context, text string) (string, error)
	Chat(ctx context.Context, section Section, message string) (string, error)
	// CompleteJSON returns the model's answer as raw JSON. The answer is guaranteed to be
	// valid JSON, not to match the schema: callers must validate it.
	CompleteJSON(ctx context.Context, request JSONRequest) (json.RawMessage, error)
	Enhance(ctx context.Context, sections []SectionText) ([]SectionText, error)
}

// SupportsJSON reports whether the assistant can answer CompleteJSON (a real model is
// configured). Assistants that cannot implement `SupportsJSON() bool` returning false.
func SupportsJSON(assistant Assistant) bool {
	if reporter, ok := assistant.(interface{ SupportsJSON() bool }); ok {
		return reporter.SupportsJSON()
	}
	return assistant != nil
}
