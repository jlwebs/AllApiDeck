package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestBuildOpenAIChatFallbackPlanPreservesResponsesReasoningContent(t *testing.T) {
	raw := []byte(`{
		"model":"deepseek-v4-flash-free",
		"stream":true,
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"first"}]},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"kept thinking"}],"encrypted_content":"cipher-state"},
			{"type":"function_call","call_id":"call_1","name":"shell_command","arguments":"{\"command\":\"pwd\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"next"}]}
		],
		"reasoning":{"effort":"high"}
	}`)

	plan, err := buildOpenAIChatFallbackPlanFromResponses(raw, AdvancedProxyProvider{
		Name:      "OpenAI-compatible",
		BaseURL:   "https://example.com/v1",
		APIKey:    "public",
		Model:     "gpt-5.5",
		Enabled:   true,
		APIFormat: "openai_responses",
	})
	if err != nil {
		t.Fatalf("build fallback plan failed: %v", err)
	}
	if !plan.SupportsChat {
		t.Fatalf("expected chat fallback support, blockers=%v", plan.Blockers)
	}

	var body map[string]any
	if err := json.Unmarshal(plan.ChatBody, &body); err != nil {
		t.Fatalf("decode chat body failed: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing: %#v", body["messages"])
	}
	var assistant map[string]any
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		if message["role"] == "assistant" {
			assistant = message
			break
		}
	}
	if assistant == nil {
		t.Fatalf("assistant tool-call message missing: %#v", messages)
	}
	if got := toStringValue(assistant["reasoning_content"]); got != "kept thinking\ncipher-state" {
		t.Fatalf("reasoning_content mismatch: %q", got)
	}
	if _, ok := assistant["tool_calls"].([]any); !ok {
		t.Fatalf("assistant tool_calls missing: %#v", assistant)
	}
	if got := toStringValue(body["reasoning_effort"]); got != "high" {
		t.Fatalf("reasoning_effort mismatch: %q", got)
	}
}

func TestBuildOpenAIChatFallbackPlanGroupsParallelToolCallsBeforeOutputs(t *testing.T) {
	raw := []byte(`{
		"model":"deepseek-v4-flash-free",
		"stream":true,
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"run checks"}]},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"need two commands"}]},
			{"type":"function_call","call_id":"call_1","name":"shell_command","arguments":"{\"command\":\"pwd\"}"},
			{"type":"function_call","call_id":"call_2","name":"shell_command","arguments":"{\"command\":\"ls\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"S:\\project"},
			{"type":"function_call_output","call_id":"call_2","output":"ok"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)

	plan, err := buildOpenAIChatFallbackPlanFromResponses(raw, AdvancedProxyProvider{
		Name:      "OpenAI-compatible",
		BaseURL:   "https://example.com/v1",
		APIKey:    "public",
		Model:     "gpt-5.5",
		Enabled:   true,
		APIFormat: "openai_responses",
	})
	if err != nil {
		t.Fatalf("build fallback plan failed: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(plan.ChatBody, &body); err != nil {
		t.Fatalf("decode chat body failed: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing: %#v", body["messages"])
	}
	if len(messages) != 5 {
		t.Fatalf("unexpected message count %d: %#v", len(messages), messages)
	}

	assistant, _ := messages[1].(map[string]any)
	if assistant["role"] != "assistant" {
		t.Fatalf("expected grouped assistant message at index 1: %#v", messages)
	}
	if got := toStringValue(assistant["reasoning_content"]); got != "need two commands" {
		t.Fatalf("reasoning_content mismatch: %q", got)
	}
	toolCalls, ok := assistant["tool_calls"].([]any)
	if !ok || len(toolCalls) != 2 {
		t.Fatalf("expected two grouped tool calls: %#v", assistant["tool_calls"])
	}

	firstTool, _ := messages[2].(map[string]any)
	secondTool, _ := messages[3].(map[string]any)
	if firstTool["role"] != "tool" || firstTool["tool_call_id"] != "call_1" {
		t.Fatalf("first tool output not directly after grouped call: %#v", firstTool)
	}
	if secondTool["role"] != "tool" || secondTool["tool_call_id"] != "call_2" {
		t.Fatalf("second tool output not directly after grouped call: %#v", secondTool)
	}
}

func TestBuildOpenAIChatFallbackPlanDropsToolCallsWithoutOutputs(t *testing.T) {
	raw := []byte(`{
		"model":"deepseek-v4-flash-free",
		"stream":true,
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"run checks"}]},
			{"type":"function_call","call_id":"call_1","name":"shell_command","arguments":"{\"command\":\"pwd\"}"},
			{"type":"function_call","call_id":"call_2","name":"shell_command","arguments":"{\"command\":\"ls\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}
		]
	}`)

	plan, err := buildOpenAIChatFallbackPlanFromResponses(raw, AdvancedProxyProvider{
		Name:      "OpenAI-compatible",
		BaseURL:   "https://example.com/v1",
		APIKey:    "public",
		Model:     "gpt-5.5",
		Enabled:   true,
		APIFormat: "openai_responses",
	})
	if err != nil {
		t.Fatalf("build fallback plan failed: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(plan.ChatBody, &body); err != nil {
		t.Fatalf("decode chat body failed: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok {
		t.Fatalf("messages missing: %#v", body["messages"])
	}
	assistant, _ := messages[1].(map[string]any)
	toolCalls, ok := assistant["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected only the answered tool call to be forwarded: %#v", assistant)
	}
	toolCall, _ := toolCalls[0].(map[string]any)
	if toolCall["id"] != "call_1" {
		t.Fatalf("unexpected forwarded call: %#v", toolCall)
	}
	toolOutput, _ := messages[2].(map[string]any)
	if toolOutput["role"] != "tool" || toolOutput["tool_call_id"] != "call_1" {
		t.Fatalf("expected matching tool output: %#v", toolOutput)
	}
	if !stringSliceContains(plan.Blockers, "tool_call_missing_output") {
		t.Fatalf("expected missing output blocker, got %v", plan.Blockers)
	}
}

func TestBuildOpenAIChatFallbackPlanFlattensCapturedOpencodeToolHistory(t *testing.T) {
	rawBody, err := os.ReadFile(filepath.Join("testdata", "advanced_proxy", "request_content.json"))
	if err != nil {
		t.Fatalf("read captured request fixture: %v", err)
	}

	plan, err := buildOpenAIChatFallbackPlanFromResponses(rawBody, AdvancedProxyProvider{
		Name:      "Opencode",
		BaseURL:   "https://opencode.ai/zen/v1",
		APIKey:    "public",
		Model:     "deepseek-v4-flash-free",
		Enabled:   true,
		APIFormat: "openai_responses",
	})
	if err != nil {
		t.Fatalf("build fallback plan failed: %v", err)
	}
	if !plan.SupportsChat {
		t.Fatalf("expected captured payload to support chat fallback, blockers=%v", plan.Blockers)
	}
	if plan.HostedWebSearch {
		t.Fatalf("expected Opencode/DeepSeek fallback to skip hosted web_search loop")
	}

	var body map[string]any
	if err := json.Unmarshal(plan.ChatBody, &body); err != nil {
		t.Fatalf("decode chat body failed: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatalf("messages missing: %#v", body["messages"])
	}
	flattenedToolContext := 0
	for index, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		if _, exists := message["tool_calls"]; exists {
			t.Fatalf("expected Opencode/DeepSeek fallback to flatten tool_calls at message[%d], got %#v", index, message)
		}
		if strings.TrimSpace(toStringValue(message["role"])) == "tool" {
			t.Fatalf("expected Opencode/DeepSeek fallback to avoid tool role at message[%d], got %#v", index, message)
		}
		if strings.Contains(toStringValue(message["content"]), "Tool call recorded for context.") {
			flattenedToolContext++
		}
	}
	if flattenedToolContext == 0 {
		t.Fatalf("expected captured tool calls to become text context, got %#v", messages)
	}
}

func TestTransformOpenAIChatStreamToResponsesPreservesReasoningContent(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl_reasoning","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"role":"assistant","reasoning_content":"kept "}}]}`,
		`data: {"id":"chatcmpl_reasoning","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"reasoning_content":"thinking","content":"ok"}}]}`,
		`data: {"id":"chatcmpl_reasoning","model":"deepseek-v4-flash-free","created":123,"choices":[{"finish_reason":"stop","delta":{}}]}`,
		`data: [DONE]`,
		``,
	}, "\n\n")))

	body, err := io.ReadAll(transformOpenAIChatStreamToResponsesStream(stream, "deepseek-v4-flash-free"))
	if err != nil {
		t.Fatalf("read transformed stream failed: %v", err)
	}
	payload := string(body)
	if !strings.Contains(payload, `"reasoning_content":"kept thinking"`) {
		t.Fatalf("reasoning_content missing from transformed stream:\n%s", payload)
	}
}
