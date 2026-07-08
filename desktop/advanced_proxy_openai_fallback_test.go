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

func TestBuildOpenAIChatFallbackPlanPreservesCapturedOpencodeToolHistory(t *testing.T) {
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
	toolCallMessages := 0
	toolOutputMessages := 0
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		switch strings.TrimSpace(toStringValue(message["role"])) {
		case "assistant":
			if toolCalls, ok := message["tool_calls"].([]any); ok && len(toolCalls) > 0 {
				toolCallMessages++
				if got := strings.TrimSpace(toStringValue(message["reasoning_content"])); got == "" {
					t.Fatalf("expected Opencode/DeepSeek tool_call assistant to carry reasoning_content: %#v", message)
				}
			}
		case "tool":
			toolOutputMessages++
			if strings.TrimSpace(toStringValue(message["tool_call_id"])) == "" {
				t.Fatalf("expected tool output to retain tool_call_id: %#v", message)
			}
		}
	}
	if toolCallMessages == 0 || toolOutputMessages == 0 {
		t.Fatalf("expected captured tool history to stay structured, got %#v", messages)
	}
}

func TestBuildOpenAIChatFallbackPlanKeepsFailedToolOutputStructuredForRetry(t *testing.T) {
	raw := []byte(`{
		"model":"deepseek-v4-flash-free",
		"stream":true,
		"input":[
			{"type":"function_call","call_id":"call_00_wQPCP5MRh91PpU4zdKjO8653","name":"shell_command","arguments":"{\"command\":\"python \\\"$env:USERPROFILE\\\\gen_ggbond.py\\\"\",\"timeout_ms\":30000}"},
			{"type":"function_call_output","call_id":"call_00_wQPCP5MRh91PpU4zdKjO8653","output":"Exit code: 1\nGenerated 93768 voxels\nUnicodeEncodeError: 'gbk' codec can't encode character '\\u2705' in position 0"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"继续"}]}
		],
		"tools":[{"type":"function","name":"shell_command","description":"Run shell","parameters":{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}}],
		"tool_choice":"auto"
	}`)

	plan, err := buildOpenAIChatFallbackPlanFromResponses(raw, AdvancedProxyProvider{
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

	var body map[string]any
	if err := json.Unmarshal(plan.ChatBody, &body); err != nil {
		t.Fatalf("decode chat body failed: %v", err)
	}
	messages, _ := body["messages"].([]any)
	if len(messages) < 3 {
		t.Fatalf("expected structured tool history before retry prompt: %#v", messages)
	}
	assistant, _ := messages[0].(map[string]any)
	if assistant["role"] != "assistant" {
		t.Fatalf("expected assistant tool_call first, got %#v", messages)
	}
	toolCalls, ok := assistant["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected one preserved tool_call, got %#v", assistant)
	}
	if got := strings.TrimSpace(toStringValue(assistant["reasoning_content"])); got == "" {
		t.Fatalf("expected DeepSeek reasoning_content backfill: %#v", assistant)
	}
	toolMessage, _ := messages[1].(map[string]any)
	if toolMessage["role"] != "tool" || toolMessage["tool_call_id"] != "call_00_wQPCP5MRh91PpU4zdKjO8653" {
		t.Fatalf("expected matching tool output, got %#v", toolMessage)
	}
	userMessage, _ := messages[2].(map[string]any)
	if userMessage["role"] != "user" || !strings.Contains(toStringValue(userMessage["content"]), "继续") {
		t.Fatalf("expected retry prompt after tool output, got %#v", userMessage)
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

func TestTransformOpenAIChatStreamToResponsesParsesFoldedDataFrames(t *testing.T) {
	frames := []string{
		`data: {"id":"chatcmpl_folded","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"role":"assistant","content":"Preparing"}}]}`,
		`data: {"id":"chatcmpl_folded","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_00_kpHYWsRAIyHBr3rWexOg9288","type":"function","function":{"name":"shell_command","arguments":""}}]}}]}`,
		`data: {"id":"chatcmpl_folded","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\": \"python \\\"$env:USERPROFILE\\\\gen"}}]}}]}`,
		`data: {"id":"chatcmpl_folded","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"_ggbond.py\\\"\", \"timeout_ms\": 30000}"}}]}}]}`,
		`data: {"id":"chatcmpl_folded","model":"deepseek-v4-flash-free","created":123,"choices":[{"finish_reason":"tool_calls","delta":{}}],"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}}`,
		`data: [DONE]`,
		`data: {"choices":[],"cost":"0"}`,
	}
	stream := io.NopCloser(strings.NewReader(strings.Join(frames, " ")))

	body, err := io.ReadAll(transformOpenAIChatStreamToResponsesStream(stream, "deepseek-v4-flash-free"))
	if err != nil {
		t.Fatalf("read transformed stream failed: %v", err)
	}
	payload := string(body)
	for _, needle := range []string{
		`event: response.output_item.added`,
		`"call_id":"call_00_kpHYWsRAIyHBr3rWexOg9288"`,
		`"name":"shell_command"`,
		`event: response.function_call_arguments.done`,
		`gen_ggbond.py`,
		`timeout_ms`,
		`event: response.completed`,
		`data: [DONE]`,
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("expected %q in transformed folded stream:\n%s", needle, payload)
		}
	}
	if strings.Contains(payload, `event: response.incomplete`) {
		t.Fatalf("folded stream should complete instead of becoming incomplete:\n%s", payload)
	}
}

func TestTransformOpenAIChatStreamToResponsesMarksUnexpectedEOFIncomplete(t *testing.T) {
	stream := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl_cut","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"role":"assistant","content":"partial text"}}]}`,
		`data: {"id":"chatcmpl_cut","model":"deepseek-v4-flash-free","created":123,"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"shell_command","arguments":"{\"command\":\"pwd\"}"}}]}}]}`,
		``,
	}, "\n\n")))

	body, err := io.ReadAll(transformOpenAIChatStreamToResponsesStream(stream, "deepseek-v4-flash-free"))
	if err != nil {
		t.Fatalf("read transformed stream failed: %v", err)
	}
	payload := string(body)
	if !strings.Contains(payload, `event: response.incomplete`) {
		t.Fatalf("expected incomplete event for truncated chat stream:\n%s", payload)
	}
	if !strings.Contains(payload, `"status":"incomplete"`) {
		t.Fatalf("expected completed envelope to carry incomplete status:\n%s", payload)
	}
	if !strings.Contains(payload, `"reason":"stream_ended_without_done"`) {
		t.Fatalf("expected stream_ended_without_done reason:\n%s", payload)
	}
	if !strings.Contains(payload, `event: response.completed`) || !strings.Contains(payload, `data: [DONE]`) {
		t.Fatalf("expected stream to close with completed envelope and DONE marker:\n%s", payload)
	}
}

func TestTransformOpenAIChatStreamToResponsesConvertsReadErrorToIncomplete(t *testing.T) {
	stream := &failingReadCloser{
		chunks: [][]byte{
			[]byte("data: {\"id\":\"chatcmpl_err\",\"model\":\"deepseek-v4-flash-free\",\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"),
		},
		err: io.ErrUnexpectedEOF,
	}

	body, err := io.ReadAll(transformOpenAIChatStreamToResponsesStream(stream, "deepseek-v4-flash-free"))
	if err != nil {
		t.Fatalf("read transformed stream should not surface upstream read error: %v", err)
	}
	payload := string(body)
	if !strings.Contains(payload, `event: response.incomplete`) {
		t.Fatalf("expected incomplete event for read error:\n%s", payload)
	}
	if !strings.Contains(payload, `"reason":"stream_read_error"`) {
		t.Fatalf("expected stream_read_error reason:\n%s", payload)
	}
	if !strings.Contains(payload, `data: [DONE]`) {
		t.Fatalf("expected DONE marker after read error conversion:\n%s", payload)
	}
}

func TestConvertResponsesRequestToolChoiceToChat(t *testing.T) {
	// Test case 1: string values should pass through
	result := convertResponsesRequestToolChoiceToChat("required")
	if result != "required" {
		t.Fatalf("expected 'required', got %#v", result)
	}

	result = convertResponsesRequestToolChoiceToChat("auto")
	if result != "auto" {
		t.Fatalf("expected 'auto', got %#v", result)
	}

	result = convertResponsesRequestToolChoiceToChat("none")
	if result != "none" {
		t.Fatalf("expected 'none', got %#v", result)
	}

	// Test case 2: invalid string should return nil
	result = convertResponsesRequestToolChoiceToChat("invalid")
	if result != nil {
		t.Fatalf("expected nil for invalid string, got %#v", result)
	}

	// Test case 3: correct nested format {"type": "function", "function": {"name": "..."}}
	result = convertResponsesRequestToolChoiceToChat(map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "get_weather",
		},
	})
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resultMap["type"] != "function" {
		t.Fatalf("expected type 'function', got %#v", resultMap["type"])
	}
	functionMap, ok := resultMap["function"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'function' to be a map[string]any, got %T", resultMap["function"])
	}
	if functionMap["name"] != "get_weather" {
		t.Fatalf("expected function name 'get_weather', got %#v", functionMap["name"])
	}

	// Test case 4: legacy flat format {"type": "function", "name": "..."} should also work
	result = convertResponsesRequestToolChoiceToChat(map[string]any{
		"type": "function",
		"name": "search_database",
	})
	resultMap, ok = result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for legacy format, got %T", result)
	}
	if resultMap["type"] != "function" {
		t.Fatalf("expected type 'function', got %#v", resultMap["type"])
	}
	functionMap, ok = resultMap["function"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'function' to be a map[string]any, got %T", resultMap["function"])
	}
	if functionMap["name"] != "search_database" {
		t.Fatalf("expected function name 'search_database', got %#v", functionMap["name"])
	}

	// Test case 5: missing name should return nil
	result = convertResponsesRequestToolChoiceToChat(map[string]any{
		"type": "function",
	})
	if result != nil {
		t.Fatalf("expected nil for missing name, got %#v", result)
	}

	// Test case 6: wrong type should return nil
	result = convertResponsesRequestToolChoiceToChat(map[string]any{
		"type": "other",
		"name": "test",
	})
	if result != nil {
		t.Fatalf("expected nil for wrong type, got %#v", result)
	}
}
