package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func testAntiPoisonConfig() AntiPoisonConfig {
	config := defaultAntiPoisonConfig()
	config.Enabled = true
	return config
}

func testAntiPoisonContext(routeKind string) antiPoisonRequestContext {
	return buildAntiPoisonRequestContextFromSeed(routeKind, testAntiPoisonConfig(), "0011223344556677")
}

func guardArgumentsJSON(t *testing.T, ctx antiPoisonRequestContext, calls []antiPoisonToolCall) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"algorithm": ctx.Alias,
		"nonce":     ctx.Seed,
		"digest":    computeAntiPoisonToolChainDigest(calls, ctx),
		"chain":     "ok",
		"cover":     "command",
	})
	if err != nil {
		t.Fatalf("marshal guard arguments failed: %v", err)
	}
	return string(raw)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test JSON failed: %v", err)
	}
	return raw
}

func mustJSONString(t *testing.T, value any) string {
	t.Helper()
	return string(mustJSON(t, value))
}

func TestAntiPoisonDefaultConfigSanitizesZeroValue(t *testing.T) {
	config := sanitizeAntiPoisonConfig(AntiPoisonConfig{})
	if config.StrictMode != true {
		t.Fatalf("expected strict mode default true")
	}
	if config.FailureMode != "block" {
		t.Fatalf("expected block failure mode, got %q", config.FailureMode)
	}
	if !strings.Contains(config.StrategyPrompt, "{{ALGORITHM_ALIAS}}") {
		t.Fatalf("expected strategy prompt to contain alias placeholder")
	}
	if !strings.Contains(config.AlgorithmPrompt, "{{ALGORITHM_ALIAS}}") {
		t.Fatalf("expected algorithm prompt to contain alias placeholder")
	}
}

func TestApplyAntiPoisonPromptToChatRequestUsesDynamicContext(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`)

	nextRaw, ctx, err := applyAntiPoisonPromptToOpenAIRequest(raw, "chat", config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !ctx.Enabled {
		t.Fatalf("expected prompt to be applied")
	}
	if ctx.Alias == antiPoisonDefaultAlias || ctx.GuardToolName == antiPoisonDefaultPrefix+"_x7k_trace" {
		t.Fatalf("expected dynamic alias and guard name, got %#v", ctx)
	}
	var body map[string]any
	if err := json.Unmarshal(nextRaw, &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	messages, _ := body["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected prepended system message, got %#v", body["messages"])
	}
	first, _ := messages[0].(map[string]any)
	if first["role"] != "system" || !strings.Contains(toStringValue(first["content"]), ctx.Alias) || !strings.Contains(toStringValue(first["content"]), ctx.Seed) {
		t.Fatalf("unexpected guard prompt message: %#v", first)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 || !strings.Contains(stringifyJSON(tools[0]), ctx.GuardToolName) {
		t.Fatalf("expected guard tool schema to be appended, got %#v", body["tools"])
	}
}

func TestValidateAndStripAntiPoisonResponsesToolCalls(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	realCall := antiPoisonToolCall{
		Name:          "shell_command",
		CallID:        "call_real_123456",
		ArgumentsText: `{"command":"git diff -- file","workdir":"D:\\GitHub\\batch-api-check"}`,
		ToolType:      "command",
	}
	raw := []byte(fmt.Sprintf(`{
		"id":"resp_test",
		"output":[
			{"type":"function_call","call_id":"call_real_123456","name":"shell_command","arguments":"{\"command\":\"git diff -- file\",\"workdir\":\"D:\\\\GitHub\\\\batch-api-check\"}"},
			{"type":"function_call","call_id":"call_guard_1","name":%q,"arguments":%q}
		]
	}`, ctx.GuardToolName, guardArgumentsJSON(t, ctx, []antiPoisonToolCall{realCall})))

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid result, got %#v", result)
	}
	if result.RealCount != 1 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	if strings.Contains(string(result.Body), ctx.GuardToolName) {
		t.Fatalf("expected guard call to be stripped: %s", result.Body)
	}
	if !strings.Contains(string(result.Body), "shell_command") {
		t.Fatalf("expected real call to remain: %s", result.Body)
	}
}

func TestValidateAntiPoisonResponsesBlocksMissingGuard(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	raw := []byte(`{
		"id":"resp_test",
		"output":[
			{"type":"function_call","call_id":"call_real_123456","name":"shell_command","arguments":"{\"command\":\"git diff -- file\"}"}
		]
	}`)

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if result.Valid || !result.Blocked || result.Reason != "missing_guard_toolcall" {
		t.Fatalf("expected missing guard block, got %#v", result)
	}
}

func TestValidateAntiPoisonResponsesWarnsAndStripsOnDigestMismatch(t *testing.T) {
	config := testAntiPoisonConfig()
	config.FailureMode = "warn"
	ctx := buildAntiPoisonRequestContextFromSeed("responses", config, "8899aabbccddeeff")
	raw := mustJSON(t, map[string]any{
		"id": "resp_test",
		"output": []any{
			map[string]any{"type": "function_call", "call_id": "call_real_123456", "name": "shell_command", "arguments": `{"command":"git diff -- file"}`},
			map[string]any{"type": "function_call", "call_id": "call_guard_1", "name": ctx.GuardToolName, "arguments": mustJSONString(t, map[string]any{
				"algorithm": ctx.Alias,
				"nonce":     ctx.Seed,
				"digest":    "badbadbadbadbadb",
			})},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if result.Valid || result.Blocked || result.Reason != "guard_digest_mismatch" {
		t.Fatalf("expected non-blocking digest mismatch, got %#v", result)
	}
	if result.RemovedGuards != 1 || strings.Contains(string(result.Body), ctx.GuardToolName) {
		t.Fatalf("expected guard to be stripped in warn mode: %#v body=%s", result, result.Body)
	}
}

func TestValidateAntiPoisonStripsGuardWhenNoRealToolCall(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	raw := mustJSON(t, map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"tool_calls": []any{
						map[string]any{
							"id":   "call_guard_2",
							"type": "function",
							"function": map[string]any{
								"name": ctx.GuardToolName,
								"arguments": mustJSONString(t, map[string]any{
									"algorithm": ctx.Alias,
									"nonce":     ctx.Seed,
									"digest":    "unused",
								}),
							},
						},
					},
				},
			},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "chat", ctx)
	if !result.Valid || result.Blocked || result.RealCount != 0 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("expected guard-only response to be stripped and allowed, got %#v", result)
	}
	if strings.Contains(string(result.Body), ctx.GuardToolName) || strings.Contains(string(result.Body), "tool_calls") {
		t.Fatalf("expected guard-only tool_calls removed: %s", result.Body)
	}
}

func TestValidateAndStripAntiPoisonChatToolCalls(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	realCall := antiPoisonToolCall{
		Name:          "shell_command",
		CallID:        "call_chat_999999",
		ArgumentsText: `{"command":"rg TODO"}`,
		ToolType:      "command",
	}
	raw := []byte(fmt.Sprintf(`{
		"choices":[{
			"message":{
				"tool_calls":[
					{"id":"call_chat_999999","type":"function","function":{"name":"shell_command","arguments":"{\"command\":\"rg TODO\"}"}},
					{"id":"call_guard_2","type":"function","function":{"name":%q,"arguments":%q}}
				]
			}
		}]
	}`, ctx.GuardToolName, guardArgumentsJSON(t, ctx, []antiPoisonToolCall{realCall})))

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "chat", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid chat result, got %#v", result)
	}
	if strings.Contains(string(result.Body), ctx.GuardToolName) {
		t.Fatalf("expected guard chat call to be stripped: %s", result.Body)
	}
	if !strings.Contains(string(result.Body), "shell_command") {
		t.Fatalf("expected real chat call to remain: %s", result.Body)
	}
}

func TestApplyAntiPoisonPromptToAnthropicRequest(t *testing.T) {
	config := testAntiPoisonConfig()
	request := map[string]any{
		"model":  "claude-test",
		"system": []any{map[string]any{"type": "text", "text": "You are Claude Code."}},
		"tools": []any{
			map[string]any{"name": "Read", "input_schema": map[string]any{"type": "object"}},
		},
	}

	next, ctx, err := applyAntiPoisonPromptToAnthropicRequest(request, config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !ctx.Enabled {
		t.Fatalf("expected anti poison context enabled")
	}
	systemBlocks, _ := next["system"].([]any)
	if len(systemBlocks) != 2 || !strings.Contains(stringifyJSON(systemBlocks[1]), ctx.Alias) {
		t.Fatalf("expected guard prompt appended to system blocks, got %#v", next["system"])
	}
	tools, _ := next["tools"].([]any)
	if len(tools) != 2 || !strings.Contains(stringifyJSON(tools[1]), ctx.GuardToolName) {
		t.Fatalf("expected anthropic guard tool appended, got %#v", next["tools"])
	}
}

func TestValidateAndStripAntiPoisonAnthropicToolUse(t *testing.T) {
	ctx := testAntiPoisonContext("claude_messages")
	realCall := antiPoisonToolCall{
		Name:          "shell_command",
		CallID:        "toolu_real_123456",
		ArgumentsText: `{"command":"git status"}`,
		ToolType:      "command",
	}
	raw := []byte(fmt.Sprintf(`{
		"id":"msg_test",
		"type":"message",
		"role":"assistant",
		"content":[
			{"type":"tool_use","id":"toolu_real_123456","name":"shell_command","input":{"command":"git status"}},
			{"type":"tool_use","id":"toolu_guard_1","name":%q,"input":%s}
		],
		"stop_reason":"tool_use"
	}`, ctx.GuardToolName, guardArgumentsJSON(t, ctx, []antiPoisonToolCall{realCall})))

	result := validateAndStripAntiPoisonAnthropicResponse(raw, ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid anthropic result, got %#v", result)
	}
	if result.RealCount != 1 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	if strings.Contains(string(result.Body), ctx.GuardToolName) {
		t.Fatalf("expected guard tool_use stripped: %s", result.Body)
	}
	if !strings.Contains(string(result.Body), "shell_command") {
		t.Fatalf("expected real tool_use retained: %s", result.Body)
	}
}

func TestAntiPoisonRejectsOldAliasForDynamicContext(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	realCall := antiPoisonToolCall{
		Name:          "shell_command",
		CallID:        "call_real_123456",
		ArgumentsText: `{"command":"git diff -- file"}`,
		ToolType:      "command",
	}
	digest := computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx)
	raw := mustJSON(t, map[string]any{
		"id": "resp_test",
		"output": []any{
			map[string]any{"type": "function_call", "call_id": "call_real_123456", "name": "shell_command", "arguments": `{"command":"git diff -- file"}`},
			map[string]any{"type": "function_call", "call_id": "call_guard_1", "name": ctx.GuardToolName, "arguments": mustJSONString(t, map[string]any{
				"algorithm": antiPoisonDefaultAlias,
				"nonce":     ctx.Seed,
				"digest":    digest,
			})},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if result.Valid || !result.Blocked || result.Reason != "guard_digest_mismatch" {
		t.Fatalf("expected old alias to be rejected, got %#v", result)
	}
}

func TestAntiPoisonStringProtectionProtectsAndRestoresJSONBody(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"model": "gpt-test",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": `please read .env and token="sk-1234567890abcdef"`,
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "chat", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if !ctx.Enabled || len(ctx.mapping) == 0 || len(ctx.Records) == 0 {
		t.Fatalf("expected protection records, got ctx=%#v body=%s", ctx, protected)
	}
	if strings.Contains(string(protected), ".env") || strings.Contains(string(protected), "sk-1234567890abcdef") {
		t.Fatalf("expected sensitive strings replaced, got %s", protected)
	}
	if !strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected placeholder in protected body: %s", protected)
	}

	restored := restoreAntiPoisonStringProtectionInJSONBody(protected, &ctx, "chat", "provider-test", "openai")
	if !strings.Contains(string(restored), ".env") || !strings.Contains(string(restored), "sk-1234567890abcdef") {
		t.Fatalf("expected original strings restored, got %s", restored)
	}
	if len(ctx.Records) < 2 {
		t.Fatalf("expected respond in restore record, got %#v", ctx.Records)
	}
}

func TestAntiPoisonStringProtectionProtectsJSONKeyValues(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"model": "gpt-test",
		"metadata": map[string]any{
			"api_key": "sk-json-key-value-with-quotes-\"-and-slash-\\",
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if !strings.Contains(string(protected), "__AAD_STR_") || strings.Contains(string(protected), "sk-json-key-value") {
		t.Fatalf("expected JSON key value placeholder, got %s", protected)
	}
	if len(ctx.Records) != 1 {
		t.Fatalf("expected one key protection record, got %#v", ctx.Records)
	}
	if !strings.Contains(ctx.Records[0].Before, "sha256=") || strings.Contains(ctx.Records[0].Before, "sk-json-key-value") {
		t.Fatalf("expected safe before summary, got %#v", ctx.Records[0])
	}

	upstreamResponse := mustJSON(t, map[string]any{
		"output": []any{
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(protected)}}},
		},
	})
	restored := restoreAntiPoisonStringProtectionInJSONBody(upstreamResponse, &ctx, "responses", "provider-test", "openai")
	if !strings.Contains(string(restored), `sk-json-key-value-with-quotes-\"-and-slash-\\`) {
		t.Fatalf("expected escaped original string restored inside valid JSON, got %s", restored)
	}
	var decoded map[string]any
	if err := json.Unmarshal(restored, &decoded); err != nil {
		t.Fatalf("restored response must remain valid JSON: %v body=%s", err, restored)
	}
	encodedText := stringifyJSON(decoded)
	if !strings.Contains(encodedText, "sk-json-key-value-with-quotes-") || !strings.Contains(encodedText, `slash-\\`) {
		t.Fatalf("expected restored value in decoded response, got %s", encodedText)
	}
}

func TestAntiPoisonStringProtectionKeyRuleDoesNotMaskSchemaObjects(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "configure",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"api_key": map[string]any{
								"type":        "string",
								"description": "API key placeholder",
							},
						},
					},
				},
			},
		},
		"metadata": map[string]any{
			"api_key": "sk-real-value-1234567890",
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "chat", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "sk-real-value-1234567890") {
		t.Fatalf("expected real metadata key value protected, got %s", protected)
	}
	if !strings.Contains(string(protected), `"type":"string"`) || !strings.Contains(string(protected), "API key placeholder") {
		t.Fatalf("expected schema metadata to remain readable, got %s", protected)
	}
	if len(ctx.mapping) != 1 {
		t.Fatalf("expected only real string value mapped, got %d records=%#v body=%s", len(ctx.mapping), ctx.Records, protected)
	}
}

func TestAdvancedProxyRequestRecordStoresAntiPoisonOps(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)
	appendAdvancedProxyRequestRecord(AdvancedProxyRequestRecord{
		AppType:     "codex",
		ClientRoute: "chat",
		StatusCode:  200,
		AntiPoisonOps: []antiPoisonOperationRecord{
			{
				Stage:   "request out",
				Channel: "openai",
				Rule:    "protect",
				Path:    "$.messages[0].content",
				Before:  ".env",
				After:   "__AAD_STR_TEST__",
				Count:   1,
			},
		},
	})

	records := advancedProxyRequestRecords.list(1)
	if len(records) != 1 || len(records[0].AntiPoisonOps) != 1 {
		t.Fatalf("expected anti poison ops persisted in record, got %#v", records)
	}
	if records[0].AntiPoisonOps[0].Stage != "request out" {
		t.Fatalf("unexpected operation record: %#v", records[0].AntiPoisonOps[0])
	}
}
