package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeProvider(t *testing.T, reply string, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if captured != nil {
			_ = json.Unmarshal(body, captured)
		}
		encoded, _ := json.Marshal(reply)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":` + string(encoded) + `},"finish_reason":"stop"}]}`))
	}))
}

var testSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"string"}}}`)

func TestCompleteJSONSendsStrictSchema(t *testing.T) {
	var request map[string]any
	server := fakeProvider(t, `{"a":"b"}`, &request)
	defer server.Close()

	assistant := NewOpenAIAssistant(server.URL, "key", "model", "json_schema", server.Client())
	raw, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", System: "sys", User: "user", Schema: testSchema})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"a":"b"}` {
		t.Fatalf("unexpected answer %s", raw)
	}

	format := request["response_format"].(map[string]any)
	schema := format["json_schema"].(map[string]any)
	if format["type"] != "json_schema" || schema["name"] != "test" || schema["strict"] != true || schema["schema"] == nil {
		t.Fatalf("unexpected response_format %v", format)
	}
	if _, ok := request["provider"]; ok {
		t.Fatal("OpenRouter routing must not be sent to other APIs")
	}
}

func TestCompleteJSONWithoutStructuredOutputsPutsSchemaInPrompt(t *testing.T) {
	var request map[string]any
	server := fakeProvider(t, "Here you go:\n```json\n{\"a\":\"b\"}\n```", &request)
	defer server.Close()

	assistant := NewOpenAIAssistant(server.URL, "key", "model", "none", server.Client())
	raw, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", System: "sys", User: "user", Schema: testSchema})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"a":"b"}` {
		t.Fatalf("fences not stripped: %s", raw)
	}
	if _, ok := request["response_format"]; ok {
		t.Fatal("no response_format expected")
	}
	system := request["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(system, `"additionalProperties":false`) {
		t.Fatal("schema missing from the system prompt")
	}
}

func TestCompleteJSONRejectsInvalidJSON(t *testing.T) {
	server := fakeProvider(t, "not json at all", nil)
	defer server.Close()

	assistant := NewOpenAIAssistant(server.URL, "key", "model", "json_object", server.Client())
	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", Schema: testSchema}); err == nil {
		t.Fatal("expected an error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCompleteJSONOnOpenRouterRequiresParameters(t *testing.T) {
	var body map[string]any
	var title string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		title = r.Header.Get("X-Title")
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"{\"a\":\"b\"}"},"finish_reason":"stop"}]}`)),
		}, nil
	})}

	assistant := NewOpenAIAssistant("https://openrouter.ai/api/v1", "key", "vendor/model:free", "json_schema", client)
	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", Schema: testSchema}); err != nil {
		t.Fatal(err)
	}
	provider, ok := body["provider"].(map[string]any)
	if !ok || provider["require_parameters"] != true {
		t.Fatalf("expected provider.require_parameters, got %v", body["provider"])
	}
	if title != "GamiDoc" {
		t.Fatalf("expected X-Title header, got %q", title)
	}
}

func TestCompleteRetriesTransientErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"temporarily unavailable"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"a\":\"b\"}"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	assistant := NewOpenAIAssistant(server.URL, "key", "model", "json_schema", server.Client())
	assistant.retryDelays = []time.Duration{0, 0}
	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", Schema: testSchema}); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestCompleteDoesNotRetryClientErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	assistant := NewOpenAIAssistant(server.URL, "key", "model", "json_schema", server.Client())
	assistant.retryDelays = []time.Duration{0, 0}
	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", Schema: testSchema}); err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Fatalf("a 400 must not be retried, got %d calls", calls)
	}
}

func TestOpenRouterFallbackModels(t *testing.T) {
	var body map[string]any
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"{\"a\":\"b\"}"},"finish_reason":"stop"}]}`)),
		}, nil
	})}

	assistant := NewOpenAIAssistant("https://openrouter.ai/api/v1", "key", "main:free", "json_schema", client).
		WithFallbackModels([]string{" backup:free ", "", "main:free"})
	if _, err := assistant.CompleteJSON(context.Background(), JSONRequest{Name: "test", Schema: testSchema}); err != nil {
		t.Fatal(err)
	}
	models, _ := body["models"].([]any)
	if len(models) != 2 || models[0] != "main:free" || models[1] != "backup:free" {
		t.Fatalf("unexpected models %v", body["models"])
	}
}
