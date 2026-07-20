package main

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPruneGrokBuildResponsesSSEBodyDropsUnsupportedEvents(t *testing.T) {
	raw := []byte(`event: codex.rate_limits
data: {"type":"codex.rate_limits","rate_limits":{"allowed":true}}

event: response.metadata
data: {"type":"response.metadata","trace_id":"ignored"}

event: response.created
data: {"type":"response.created","response":{"id":"resp_1"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"hello"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_1","status":"completed"}}

data: [DONE]

`)

	filtered, dropped := pruneGrokBuildResponsesSSEBody(raw)
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}
	body := string(filtered)
	for _, unexpected := range []string{"codex.rate_limits", "response.metadata"} {
		if strings.Contains(body, unexpected) {
			t.Fatalf("filtered body still contains %q: %s", unexpected, body)
		}
	}
	for _, expected := range []string{"response.created", "response.output_text.delta", "response.completed", "data: [DONE]"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("filtered body is missing %q: %s", expected, body)
		}
	}
}

func TestPruneGrokBuildResponsesSSEBodyNormalizesRequiredFields(t *testing.T) {
	raw := []byte(`event: response.created
data: {"type":"response.created","response":{"status":"in_progress","output":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"hello","output_index":0}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}

`)

	filtered, dropped := pruneGrokBuildResponsesSSEBody(raw)
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	events, err := parseAdvancedProxySSEEvents(filtered)
	if err != nil {
		t.Fatalf("parse filtered events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3: %s", len(events), string(filtered))
	}

	for index, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			t.Fatalf("event %d is not JSON: %#v", index, event)
		}
		if _, ok := data["sequence_number"]; !ok {
			t.Fatalf("event %d missing sequence_number: %s", index, advancedProxySSEEventPayload(event))
		}
	}

	created, _ := advancedProxySSEJSONPayload(events[0])
	createdResponse := created["response"].(map[string]any)
	if strings.TrimSpace(createdResponse["id"].(string)) == "" {
		t.Fatalf("created response missing synthesized id: %#v", createdResponse)
	}

	delta, _ := advancedProxySSEJSONPayload(events[1])
	if strings.TrimSpace(delta["item_id"].(string)) == "" {
		t.Fatalf("delta event missing item_id: %#v", delta)
	}

	completed, _ := advancedProxySSEJSONPayload(events[2])
	completedResponse := completed["response"].(map[string]any)
	output := completedResponse["output"].([]any)
	message := output[0].(map[string]any)
	if strings.TrimSpace(message["id"].(string)) == "" {
		t.Fatalf("message missing synthesized id: %#v", message)
	}
	content := message["content"].([]any)
	textPart := content[0].(map[string]any)
	annotations, ok := textPart["annotations"].([]any)
	if !ok || len(annotations) != 0 {
		encoded, _ := json.Marshal(textPart)
		t.Fatalf("output_text annotations not normalized: %s", string(encoded))
	}
}

func TestPruneGrokBuildResponsesSSEBodyNormalizesNestedFunctionCallArguments(t *testing.T) {
	raw := []byte(`event: response.completed
data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Preparing tool call"},{"call_id":"call_nested","name":"write_file","input":{"path":"notes.txt","content":"ok"}}]},{"type":"function_call","call_id":"call_direct","name":"read_file"}]}}

`)

	filtered, dropped := pruneGrokBuildResponsesSSEBody(raw)
	if dropped != 0 {
		t.Fatalf("dropped = %d, want 0", dropped)
	}
	events, err := parseAdvancedProxySSEEvents(filtered)
	if err != nil {
		t.Fatalf("parse filtered events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1: %s", len(events), string(filtered))
	}
	completed, ok := advancedProxySSEJSONPayload(events[0])
	if !ok {
		t.Fatalf("completed event is not JSON: %#v", events[0])
	}
	response := completed["response"].(map[string]any)
	output := response["output"].([]any)
	message := output[0].(map[string]any)
	content := message["content"].([]any)
	nestedCall := content[1].(map[string]any)
	directCall := output[1].(map[string]any)

	for label, call := range map[string]map[string]any{
		"nested": nestedCall,
		"direct": directCall,
	} {
		if call["type"] != "function_call" {
			t.Fatalf("%s call type = %#v, want function_call", label, call["type"])
		}
		for _, field := range []string{"id", "call_id", "name", "arguments", "status"} {
			value, ok := call[field].(string)
			if !ok || strings.TrimSpace(value) == "" && field != "arguments" {
				t.Fatalf("%s call missing normalized %s: %#v", label, field, call)
			}
		}
	}
	if nestedCall["arguments"] != `{"content":"ok","path":"notes.txt"}` {
		t.Fatalf("nested call arguments = %#v", nestedCall["arguments"])
	}
	if directCall["arguments"] != "" {
		t.Fatalf("direct call arguments = %#v, want empty string", directCall["arguments"])
	}
}

func TestProxyOpenAIStreamDirectToClientPrunesGrokBuildResponsesOnly(t *testing.T) {
	raw := `event: codex.rate_limits
data: {"type":"codex.rate_limits"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"hello"}

`
	recorder := httptest.NewRecorder()
	recordContext := &advancedProxyStreamRequestRecordContext{
		AppType:        "grokbuild",
		ClientRoute:    "responses",
		ObservedFormat: "responses",
	}
	if err := proxyOpenAIStreamDirectToClientWithMetrics(recorder, io.NopCloser(strings.NewReader(raw)), recordContext); err != nil {
		t.Fatalf("proxy direct stream: %v", err)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "codex.rate_limits") {
		t.Fatalf("direct stream leaked unsupported event: %s", body)
	}
	if !strings.Contains(body, "response.output_text.delta") {
		t.Fatalf("direct stream removed supported event: %s", body)
	}

	nonGrokContext := &advancedProxyStreamRequestRecordContext{
		AppType:        "codex",
		ClientRoute:    "responses",
		ObservedFormat: "responses",
	}
	recorder = httptest.NewRecorder()
	if err := proxyOpenAIStreamDirectToClientWithMetrics(recorder, io.NopCloser(strings.NewReader(raw)), nonGrokContext); err != nil {
		t.Fatalf("proxy non-grok direct stream: %v", err)
	}
	if got := recorder.Body.String(); got != raw {
		t.Fatalf("non-grok stream changed:\n got: %q\nwant: %q", got, raw)
	}
}
