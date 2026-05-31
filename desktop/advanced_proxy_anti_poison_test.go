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
	call := antiPoisonToolCall{}
	if len(calls) > 0 {
		call = calls[0]
	}
	toolType := strings.TrimSpace(call.ToolType)
	if toolType == "" {
		toolType = classifyAntiPoisonToolName(call.Name)
	}
	raw, err := json.Marshal(map[string]any{
		"algorithm": ctx.Alias,
		"nonce":     ctx.Seed,
		"digest":    computeAntiPoisonToolChainDigest(calls, ctx),
		"chain":     fmt.Sprintf("0|%s|%s", toolType, strings.TrimSpace(call.Name)),
		"cover":     canonicalAntiPoisonArgumentText(call.ArgumentsText),
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

func guardJSONBlock(t *testing.T, ctx antiPoisonRequestContext, call antiPoisonToolCall, digest string) string {
	t.Helper()
	toolName := strings.TrimSpace(call.Name)
	toolType := strings.TrimSpace(call.ToolType)
	if toolType == "" {
		toolType = classifyAntiPoisonToolName(toolName)
	}
	return antiPoisonGuardJSONOpenTag + mustJSONString(t, map[string]any{
		"name":      antiPoisonGuardToolNameForTool(ctx, toolName),
		"tool_name": toolName,
		"tool_type": toolType,
		"algorithm": ctx.Alias,
		"nonce":     ctx.Seed,
		"digest":    digest,
		"chain":     fmt.Sprintf("0|%s|%s", toolType, toolName),
		"cover":     canonicalAntiPoisonArgumentText(call.ArgumentsText),
	}) + antiPoisonGuardJSONCloseTag
}

func guardJSONBlockAtIndex(t *testing.T, ctx antiPoisonRequestContext, call antiPoisonToolCall, index int, digest string) string {
	t.Helper()
	toolName := strings.TrimSpace(call.Name)
	toolType := strings.TrimSpace(call.ToolType)
	if toolType == "" {
		toolType = classifyAntiPoisonToolName(toolName)
	}
	return antiPoisonGuardJSONOpenTag + mustJSONString(t, map[string]any{
		"name":      antiPoisonGuardToolNameForTool(ctx, toolName),
		"tool_name": toolName,
		"tool_type": toolType,
		"algorithm": ctx.Alias,
		"nonce":     ctx.Seed,
		"digest":    digest,
		"chain":     fmt.Sprintf("%d|%s|%s", index, toolType, toolName),
		"cover":     canonicalAntiPoisonArgumentText(call.ArgumentsText),
	}) + antiPoisonGuardJSONCloseTag
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
	raw := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"WebSearch","parameters":{"type":"object"}}}]}`)

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
	if first["role"] != "system" || !strings.Contains(toStringValue(first["content"]), ctx.GuardToolName) || !strings.Contains(toStringValue(first["content"]), "Do not include digest") {
		t.Fatalf("unexpected guard prompt message: %#v", first)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected no extra guard tool schema appended, got %#v", body["tools"])
	}
}

func TestApplyAntiPoisonPromptToResponsesRequestPrependsInstructions(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"model":        "gpt-test",
		"instructions": "USER_ORIGINAL_INSTRUCTIONS",
		"input":        "hi",
		"tools": []any{
			map[string]any{"type": "function", "name": "WebSearch", "parameters": map[string]any{"type": "object"}},
		},
	})

	nextRaw, ctx, err := applyAntiPoisonPromptToOpenAIRequest(raw, "responses", config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !ctx.Enabled {
		t.Fatalf("expected prompt to be applied")
	}
	var body map[string]any
	if err := json.Unmarshal(nextRaw, &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	instructions := toStringValue(body["instructions"])
	if !strings.HasPrefix(instructions, "<important_gateway_rules>\nIMPORTANT: AllApiDeck guard rules") {
		t.Fatalf("expected guard prompt to be prepended, got %q", instructions)
	}
	if !strings.Contains(instructions, "USER_ORIGINAL_INSTRUCTIONS") || strings.Index(instructions, ctx.GuardToolName) > strings.Index(instructions, "USER_ORIGINAL_INSTRUCTIONS") {
		t.Fatalf("expected guard instructions before ordinary instructions, got %q", instructions)
	}
	if !strings.Contains(instructions, antiPoisonGuardJSONOpenTag) || !strings.Contains(instructions, antiPoisonGuardToolNameForTool(ctx, "WebSearch")) {
		t.Fatalf("expected prompt to contain concrete guard json example, got %q", instructions)
	}
	if strings.Contains(instructions, "digest 只需填写") || strings.Contains(instructions, "chain 规则") || strings.Contains(instructions, "cover 规则") {
		t.Fatalf("expected minimal prompt without old complex rules, got %q", instructions)
	}
}

func TestApplyAntiPoisonPromptToResponsesRequestReplacesExistingGuardBlock(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"model":        "gpt-test",
		"instructions": "<important_gateway_rules>\nold guard\n</important_gateway_rules>\n\nUSER_ORIGINAL_INSTRUCTIONS",
		"input":        "hi",
	})

	nextRaw, ctx, err := applyAntiPoisonPromptToOpenAIRequest(raw, "responses", config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(nextRaw, &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	instructions := toStringValue(body["instructions"])
	if strings.Count(instructions, "<important_gateway_rules>") != 1 {
		t.Fatalf("expected exactly one guard block, got %q", instructions)
	}
	if strings.Contains(instructions, "old guard") {
		t.Fatalf("expected old guard block removed, got %q", instructions)
	}
	if !strings.Contains(instructions, ctx.GuardToolName) || !strings.Contains(instructions, "USER_ORIGINAL_INSTRUCTIONS") {
		t.Fatalf("expected new guard plus original instructions, got %q", instructions)
	}
}

func TestApplyAntiPoisonPromptToResponsesRequestSanitizesHistoricalGuardArtifacts(t *testing.T) {
	config := testAntiPoisonConfig()
	oldGuard := antiPoisonGuardJSONOpenTag + `{"name":"aad_guard_1f85d1c995_Read","tool_name":"Read"}` + antiPoisonGuardJSONCloseTag
	raw := mustJSON(t, map[string]any{
		"model":        "gpt-test",
		"instructions": "<important_gateway_rules>\nold guard aad_guard_1f85d1c995_Read\n</important_gateway_rules>\n\nUSER_ORIGINAL_INSTRUCTIONS",
		"input": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "output_text", "text": oldGuard + "\nAllApiDeck anti-poison validation failed: missing_guard_toolcall: naming rule `aad_guard_1f85d1c995_<original_tool_name>`"},
				},
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_old",
				"output":  "guard_coverage_mismatch aad_guard_579e08c748_Read should not remain",
			},
			map[string]any{"role": "user", "content": "read env.txt"},
		},
	})

	nextRaw, ctx, err := applyAntiPoisonPromptToOpenAIRequest(raw, "responses", config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(nextRaw, &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	encoded := stringifyJSON(body)
	if strings.Count(toStringValue(body["instructions"]), "<important_gateway_rules>") != 1 {
		t.Fatalf("expected exactly one current guard prompt, got %s", encoded)
	}
	if !strings.Contains(encoded, antiPoisonGuardToolNameForTool(ctx, "WebSearch")) || !strings.Contains(encoded, "USER_ORIGINAL_INSTRUCTIONS") {
		t.Fatalf("expected current guard prompt and original instructions, got %s", encoded)
	}
	for _, stale := range []string{"aad_guard_1f85d1c995", "aad_guard_579e08c748", "missing_guard_toolcall", "guard_coverage_mismatch", "AllApiDeck anti-poison validation failed"} {
		if strings.Contains(encoded, stale) {
			t.Fatalf("expected stale artifact %q removed, got %s", stale, encoded)
		}
	}
	if strings.Contains(encoded, oldGuard) || strings.Contains(stringifyJSON(body["input"]), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected historical guard json stripped from conversation history, got %s", encoded)
	}
}

func TestApplyAntiPoisonPromptToChatRequestSanitizesHistoricalGuardArtifacts(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"model": "gpt-test",
		"messages": []any{
			map[string]any{
				"role":    "assistant",
				"content": antiPoisonGuardJSONOpenTag + `{"name":"aad_guard_1f85d1c995_Read","tool_name":"Read"}` + antiPoisonGuardJSONCloseTag + "\nmissing_guard_toolcall naming rule `aad_guard_1f85d1c995_<original_tool_name>`",
			},
			map[string]any{"role": "user", "content": "read env.txt"},
		},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "Read", "parameters": map[string]any{"type": "object"}}}},
	})

	nextRaw, ctx, err := applyAntiPoisonPromptToOpenAIRequest(raw, "chat", config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(nextRaw, &body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	encoded := stringifyJSON(body)
	if !strings.Contains(encoded, antiPoisonGuardToolNameForTool(ctx, "WebSearch")) {
		t.Fatalf("expected current guard prompt, got %s", encoded)
	}
	for _, stale := range []string{"aad_guard_1f85d1c995", "missing_guard_toolcall"} {
		if strings.Contains(encoded, stale) {
			t.Fatalf("expected stale artifact %q removed, got %s", stale, encoded)
		}
	}
}

func TestAppendAntiPoisonAnthropicSystemPrependsPrompt(t *testing.T) {
	prompt := "<important_gateway_rules>\nIMPORTANT: AllApiDeck guard rules"

	asString := appendAntiPoisonAnthropicSystem("ordinary system", prompt)
	if got := toStringValue(asString); !strings.HasPrefix(got, prompt) || strings.Index(got, prompt) > strings.Index(got, "ordinary system") {
		t.Fatalf("expected string system prompt prepended, got %#v", asString)
	}

	asBlocks := appendAntiPoisonAnthropicSystem([]any{map[string]any{"type": "text", "text": "ordinary system"}}, prompt)
	blocks, _ := asBlocks.([]any)
	if len(blocks) != 2 {
		t.Fatalf("expected prepended system block, got %#v", asBlocks)
	}
	first, _ := blocks[0].(map[string]any)
	if toStringValue(first["text"]) != prompt {
		t.Fatalf("expected first block to be guard prompt, got %#v", asBlocks)
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
			{"type":"message","content":[{"type":"output_text","text":%q}]}
		]
	}`, "done "+guardJSONBlock(t, ctx, realCall, computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx))))

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid result, got %#v", result)
	}
	if result.RealCount != 1 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	if strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard json to be stripped: %s", result.Body)
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
	if result.Valid || !result.Blocked || !strings.Contains(result.Reason, "missing_guard_toolcall") {
		t.Fatalf("expected missing guard block, got %#v", result)
	}
}

func TestValidateAntiPoisonResponsesAllowsHostedWebSearchCallWithoutGuard(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	raw := mustJSON(t, map[string]any{
		"id": "resp_test",
		"output": []any{
			map[string]any{
				"type": "web_search_call",
				"id":   "ws_123",
				"action": map[string]any{
					"type":    "search",
					"queries": []any{"2026骞?鏈?6鏃?浠婃棩鏂伴椈"},
					"sources": []any{
						map[string]any{"type": "url", "url": "https://example.com/news"},
					},
				},
			},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked || result.RealCount != 0 || result.GuardCount != 0 {
		t.Fatalf("expected hosted web_search_call to bypass model guard requirement, got %#v", result)
	}
}

func TestValidateAntiPoisonResponsesIgnoresDigestMismatchInMinimalMode(t *testing.T) {
	config := testAntiPoisonConfig()
	config.FailureMode = "warn"
	ctx := buildAntiPoisonRequestContextFromSeed("responses", config, "8899aabbccddeeff")
	raw := mustJSON(t, map[string]any{
		"id": "resp_test",
		"output": []any{
			map[string]any{"type": "function_call", "call_id": "call_real_123456", "name": "shell_command", "arguments": `{"command":"git diff -- file"}`},
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "output_text", "text": guardJSONBlock(t, ctx, antiPoisonToolCall{Name: "shell_command", ArgumentsText: `{"command":"git diff -- file"}`, ToolType: "command"}, "badbadbadbadbadb")},
			}},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid result in minimal mode, got %#v", result)
	}
	if result.RemovedGuards != 1 || strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard to be stripped in warn mode: %#v body=%s", result, result.Body)
	}
}

func TestValidateAntiPoisonStripsGuardWhenNoRealToolCall(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	raw := mustJSON(t, map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": guardJSONBlock(t, ctx, antiPoisonToolCall{Name: "shell_command", ArgumentsText: `{"command":"unused"}`, ToolType: "command"}, "unusedunusedunused"),
				},
			},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "chat", ctx)
	if !result.Valid || result.Blocked || result.RealCount != 0 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("expected guard-only response to be stripped and allowed, got %#v", result)
	}
	if strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard-only guard json removed: %s", result.Body)
	}
}

func TestClassifyAntiPoisonToolNamePrefersNetworkForWebSearch(t *testing.T) {
	if got := classifyAntiPoisonToolName("WebSearch"); got != "network" {
		t.Fatalf("expected WebSearch => network, got %q", got)
	}
}

func TestExtractAntiPoisonGuardsFromTextParsesInlineGuardJSON(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	call := antiPoisonToolCall{
		Name:          "shell_command",
		ArgumentsText: `{"command":"git status"}`,
		ToolType:      "command",
	}
	text := "ok " + guardJSONBlock(t, ctx, call, "badbadbadbadbadb")
	extracted := extractAntiPoisonGuardsFromText(text, ctx)
	if extracted.GuardCount != 1 {
		t.Fatalf("expected one guard block, got %#v", extracted)
	}
	if strings.Contains(extracted.Text, antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected stripped text without guard tag, got %q", extracted.Text)
	}
	if extracted.Text != "ok" {
		t.Fatalf("expected remaining text to be ok, got %q", extracted.Text)
	}
}

func TestExtractAntiPoisonGuardsFromTextStripsGuardLikeNoiseButCountsOnlyCanonicalGuard(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	call := antiPoisonToolCall{
		Name:          "shell_command",
		ArgumentsText: `{"command":"git status"}`,
		ToolType:      "command",
	}
	pseudoGuard := `<a ad _guard _json >{" algorithm ":"bad","tool _name":"shell_command"}</ aad _guard _json >`
	text := "prefix " + pseudoGuard + " " + guardJSONBlock(t, ctx, call, "badbadbadbadbadb") + " suffix"
	extracted := extractAntiPoisonGuardsFromText(text, ctx)
	if extracted.GuardCount != 1 {
		t.Fatalf("expected one canonical guard block, got %#v", extracted)
	}
	if strings.Contains(strings.ToLower(extracted.Text), "guard") {
		t.Fatalf("expected guard-like noise stripped from remaining text, got %q", extracted.Text)
	}
	normalized := strings.Join(strings.Fields(extracted.Text), " ")
	if normalized != "prefix suffix" {
		t.Fatalf("expected remaining text preserved around stripped guards, got %q", extracted.Text)
	}
}

func TestStripAntiPoisonOpenAIChatStreamGuardEventsRemovesInlineGuardJSON(t *testing.T) {
	ctx := testAntiPoisonContext("chat")
	call := antiPoisonToolCall{
		Name:          "shell_command",
		ArgumentsText: `{"command":"git status"}`,
		ToolType:      "command",
	}
	guardText := "ok " + guardJSONBlock(t, ctx, call, "badbadbadbadbadb")
	eventPayload := mustJSONString(t, map[string]any{
		"id": "chatcmpl_guard",
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"content": guardText,
					"tool_calls": []any{
						map[string]any{
							"index": 0,
							"id":    "call_real_abcdef12",
							"type":  "function",
							"function": map[string]any{
								"name":      "shell_command",
								"arguments": `{"command":"git status"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	})
	raw := []byte(strings.Join([]string{
		`data: ` + eventPayload,
		"",
		`data: [DONE]`,
		"",
	}, "\n"))
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		t.Fatalf("parse sse events: %v", err)
	}
	sanitized := stripAntiPoisonOpenAIChatStreamGuardEvents(events, false, ctx)
	if strings.Contains(string(sanitized), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard json stripped from sanitized stream, got %s", sanitized)
	}
	if !strings.Contains(string(sanitized), `"shell_command"`) {
		t.Fatalf("expected real toolcall retained, got %s", sanitized)
	}
}

func TestSanitizeAntiPoisonOpenAIResponsesStreamBodyStripsGuardLikeNoise(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	realCall := antiPoisonToolCall{
		Name:          "shell_command",
		ArgumentsText: `{"command":"git status"}`,
		ToolType:      "command",
	}
	pseudoGuard := `<a ad _guard _json >{" algorithm ":"bad","tool _name":"shell_command"}</ aad _guard _json >`
	guardText := pseudoGuard + " " + guardJSONBlock(t, ctx, realCall, computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx))
	deltaPayload := mustJSONString(t, map[string]any{
		"type":          "response.output_text.delta",
		"item_id":       "msg_1",
		"content_index": 0,
		"delta":         guardText,
	})
	completedPayload := mustJSONString(t, map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": guardText,
						},
					},
				},
				map[string]any{
					"type":      "function_call",
					"id":        "fc_1",
					"call_id":   "call_1",
					"name":      "shell_command",
					"arguments": `{"command":"git status"}`,
				},
			},
		},
	})
	raw := []byte(strings.Join([]string{
		`event: response.output_text.delta`,
		`data: ` + deltaPayload,
		``,
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"shell_command","arguments":"{\"command\":\"git status\"}"}}`,
		``,
		`event: response.completed`,
		`data: ` + completedPayload,
		``,
	}, "\n"))

	sanitized, result, err := sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	if err != nil {
		t.Fatalf("sanitize responses stream failed: %v", err)
	}
	if result.Blocked || !result.Valid {
		t.Fatalf("expected valid sanitized stream, got %#v", result)
	}
	sanitizedText := string(sanitized)
	if strings.Contains(sanitizedText, antiPoisonGuardJSONTagName) {
		t.Fatalf("expected canonical guard stripped from stream, got %s", sanitizedText)
	}
	if strings.Contains(strings.ToLower(sanitizedText), "a ad _guard _json") {
		t.Fatalf("expected guard-like noise stripped from stream, got %s", sanitizedText)
	}
}

func TestSanitizeAntiPoisonOpenAIResponsesStreamBodyStripsSplitGuardJSON(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	realCall := antiPoisonToolCall{
		Name:          "Read",
		ArgumentsText: `{"file_path":"env.txt"}`,
		ToolType:      "read",
	}
	guardText := guardJSONBlock(t, ctx, realCall, computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx))
	parts := []string{guardText[:16], guardText[16:64], guardText[64:]}
	lines := make([]string, 0, 16)
	for _, part := range parts {
		payload := mustJSONString(t, map[string]any{
			"type":          "response.output_text.delta",
			"item_id":       "msg_split_guard",
			"content_index": 0,
			"delta":         part,
		})
		lines = append(lines, "event: response.output_text.delta", "data: "+payload, "")
	}
	lines = append(lines,
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"Read","arguments":"{\"file_path\":\"env.txt\"}"}}`,
		``,
	)
	raw := []byte(strings.Join(lines, "\n"))

	sanitized, result, err := sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	if err != nil {
		t.Fatalf("sanitize responses stream failed: %v", err)
	}
	if result.Blocked || !result.Valid {
		t.Fatalf("expected valid sanitized stream, got %#v", result)
	}
	if strings.Contains(string(sanitized), "aad_guard") || strings.Contains(string(sanitized), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected split guard stripped before client, got %s", sanitized)
	}
	if !strings.Contains(string(sanitized), `"Read"`) {
		t.Fatalf("expected real toolcall retained, got %s", sanitized)
	}
}

func TestSanitizeAntiPoisonOpenAIResponsesStreamIgnoresInstructionEchoGuardExamples(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	exampleCall := antiPoisonToolCall{
		Name:          "WebSearch",
		ArgumentsText: `{"allowed_domains":[],"blocked_domains":[],"query":"浠婃棩鏂伴椈"}`,
		ToolType:      "network",
	}
	realArguments := `{"allowed_domains":[],"blocked_domains":[],"query":"2026/05/26 浠婃棩鏂伴椈 涓浗 鍥介檯 绉戞妧 鐑偣"}`
	createdPayload := mustJSONString(t, map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":           "resp_stream_ignore_prompt_guard",
			"instructions": "prompt echo " + guardJSONBlock(t, ctx, exampleCall, "badbadbadbadbadb"),
		},
	})
	outputItemPayload := mustJSONString(t, map[string]any{
		"type": "response.output_item.added",
		"item": map[string]any{
			"type":      "function_call",
			"id":        "fc_1",
			"call_id":   "call_real_1",
			"name":      "WebSearch",
			"arguments": realArguments,
		},
	})
	completedPayload := mustJSONString(t, map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"status": "completed",
			"output": []any{
				map[string]any{
					"type":      "function_call",
					"id":        "fc_1",
					"call_id":   "call_real_1",
					"name":      "WebSearch",
					"arguments": realArguments,
				},
			},
		},
	})
	raw := []byte(strings.Join([]string{
		`event: response.created`,
		`data: ` + createdPayload,
		``,
		`event: response.output_item.added`,
		`data: ` + outputItemPayload,
		``,
		`event: response.completed`,
		`data: ` + completedPayload,
		``,
	}, "\n"))

	sanitized, result, err := sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	if err != nil {
		t.Fatalf("sanitize responses stream failed: %v", err)
	}
	if !result.Blocked {
		t.Fatalf("expected missing real guard to block, got %#v", result)
	}
	if !strings.Contains(result.Reason, "missing_guard_toolcall") {
		t.Fatalf("expected prompt-echo guards ignored and missing guard reason surfaced, got %#v", result)
	}
	if result.GuardCount != 0 {
		t.Fatalf("expected prompt-echo guard examples not counted as real guard coverage, got %#v", result)
	}
	if !strings.Contains(string(sanitized), antiPoisonGuardJSONTagName) {
		t.Fatalf("expected blocked path to preserve raw stream body for diagnostics, got %s", sanitized)
	}
}

func TestSanitizeAntiPoisonOpenAIResponsesStreamAllowsHostedWebSearchCallWithoutGuard(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	outputItemPayload := mustJSONString(t, map[string]any{
		"type": "response.output_item.added",
		"item": map[string]any{
			"type": "web_search_call",
			"id":   "ws_1",
			"action": map[string]any{
				"type":    "search",
				"queries": []any{"2026骞?鏈?6鏃?浠婃棩鏂伴椈"},
				"sources": []any{
					map[string]any{"type": "url", "url": "https://example.com/news"},
				},
			},
		},
	})
	completedPayload := mustJSONString(t, map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "web_search_call",
					"id":   "ws_1",
					"action": map[string]any{
						"type":    "search",
						"queries": []any{"2026骞?鏈?6鏃?浠婃棩鏂伴椈"},
						"sources": []any{
							map[string]any{"type": "url", "url": "https://example.com/news"},
						},
					},
				},
			},
		},
	})
	raw := []byte(strings.Join([]string{
		`event: response.output_item.added`,
		`data: ` + outputItemPayload,
		``,
		`event: response.completed`,
		`data: ` + completedPayload,
		``,
	}, "\n"))

	_, result, err := sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	if err != nil {
		t.Fatalf("sanitize responses stream failed: %v", err)
	}
	if !result.Valid || result.Blocked || result.RealCount != 0 || result.GuardCount != 0 {
		t.Fatalf("expected hosted web_search_call stream to bypass model guard requirement, got %#v", result)
	}
}

func TestSanitizeAntiPoisonOpenAIResponsesStreamDeduplicatesFunctionCallLifecycle(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	arguments := `{"allowed_domains":[],"blocked_domains":[],"query":"2026骞?鏈?6鏃?涓婅瘉鏂伴椈"}`
	addedPayload := mustJSONString(t, map[string]any{
		"type":    "response.output_item.added",
		"item_id": "fc_lifecycle_1",
		"item": map[string]any{
			"type":    "function_call",
			"id":      "fc_lifecycle_1",
			"call_id": "call_lifecycle_1",
			"name":    "WebSearch",
		},
	})
	argsDeltaPayload := mustJSONString(t, map[string]any{
		"type":    "response.function_call_arguments.delta",
		"call_id": "call_lifecycle_1",
		"name":    "WebSearch",
		"delta":   arguments,
	})
	completedPayload := mustJSONString(t, map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"status": "completed",
			"output": []any{
				map[string]any{
					"type":      "function_call",
					"id":        "fc_lifecycle_1",
					"call_id":   "call_lifecycle_1",
					"name":      "WebSearch",
					"arguments": arguments,
				},
			},
		},
	})
	raw := []byte(strings.Join([]string{
		`event: response.output_item.added`,
		`data: ` + addedPayload,
		``,
		`event: response.function_call_arguments.delta`,
		`data: ` + argsDeltaPayload,
		``,
		`event: response.completed`,
		`data: ` + completedPayload,
		``,
	}, "\n"))

	_, result, err := sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	if err != nil {
		t.Fatalf("sanitize responses stream failed: %v", err)
	}
	if !result.Blocked || !strings.Contains(result.Reason, "missing_guard_toolcall") {
		t.Fatalf("expected missing guard block, got %#v", result)
	}
	if result.RealCount != 1 {
		t.Fatalf("expected function_call lifecycle deduped to one real toolcall, got %#v", result)
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
				"content":%q,
				"tool_calls":[
					{"id":"call_chat_999999","type":"function","function":{"name":"shell_command","arguments":"{\"command\":\"rg TODO\"}"}}
				]
			}
		}]
	}`, "ok "+guardJSONBlock(t, ctx, realCall, computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx))))

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "chat", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid chat result, got %#v", result)
	}
	if strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard chat json to be stripped: %s", result.Body)
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
	if len(systemBlocks) != 2 || !strings.Contains(stringifyJSON(systemBlocks[0]), ctx.Prefix) {
		t.Fatalf("expected guard prompt prepended to system blocks, got %#v", next["system"])
	}
	tools, _ := next["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected anthropic tools unchanged, got %#v", next["tools"])
	}
}

func TestApplyAntiPoisonPromptToAnthropicRequestSanitizesHistoricalGuardArtifacts(t *testing.T) {
	config := testAntiPoisonConfig()
	request := map[string]any{
		"model":  "claude-test",
		"system": []any{map[string]any{"type": "text", "text": "<important_gateway_rules>\nold aad_guard_1f85d1c995_Read\n</important_gateway_rules>\n\nYou are Claude Code."}},
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": antiPoisonGuardJSONOpenTag + `{"name":"aad_guard_1f85d1c995_Read","tool_name":"Read"}` + antiPoisonGuardJSONCloseTag + "\nAllApiDeck anti-poison validation failed: guard_coverage_mismatch naming rule `aad_guard_1f85d1c995_<original_tool_name>`"},
				},
			},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "read env.txt"}}},
		},
		"tools": []any{map[string]any{"name": "Read", "input_schema": map[string]any{"type": "object"}}},
	}

	next, ctx, err := applyAntiPoisonPromptToAnthropicRequest(request, config)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	encoded := stringifyJSON(next)
	if !strings.Contains(encoded, antiPoisonGuardToolNameForTool(ctx, "WebSearch")) || !strings.Contains(encoded, "You are Claude Code.") {
		t.Fatalf("expected current guard prompt and original system, got %s", encoded)
	}
	for _, stale := range []string{"aad_guard_1f85d1c995", "guard_coverage_mismatch", "AllApiDeck anti-poison validation failed"} {
		if strings.Contains(encoded, stale) {
			t.Fatalf("expected stale artifact %q removed, got %s", stale, encoded)
		}
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
			{"type":"text","text":%q}
		],
		"stop_reason":"tool_use"
	}`, guardJSONBlock(t, ctx, realCall, computeAntiPoisonToolChainDigest([]antiPoisonToolCall{realCall}, ctx))))

	result := validateAndStripAntiPoisonAnthropicResponse(raw, ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected valid anthropic result, got %#v", result)
	}
	if result.RealCount != 1 || result.GuardCount != 1 || result.RemovedGuards != 1 {
		t.Fatalf("unexpected counts: %#v", result)
	}
	if strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected guard json stripped: %s", result.Body)
	}
	if !strings.Contains(string(result.Body), "shell_command") {
		t.Fatalf("expected real tool_use retained: %s", result.Body)
	}
}

func TestAntiPoisonIgnoresLegacyAliasFieldInMinimalMode(t *testing.T) {
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
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "output_text", "text": antiPoisonGuardJSONOpenTag + mustJSONString(t, map[string]any{
					"name":      antiPoisonGuardToolNameForTool(ctx, "shell_command"),
					"tool_name": "shell_command",
					"tool_type": "command",
					"algorithm": antiPoisonDefaultAlias,
					"nonce":     ctx.Seed,
					"digest":    digest,
				}) + antiPoisonGuardJSONCloseTag},
			}},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected legacy alias field to be ignored in minimal mode, got %#v", result)
	}
}

func TestAntiPoisonGuardMatchesNamespacedCodexToolName(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	raw := mustJSON(t, map[string]any{
		"id": "resp_test",
		"output": []any{
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "output_text", "text": antiPoisonGuardJSONOpenTag + mustJSONString(t, map[string]any{
					"name":      antiPoisonGuardToolNameForTool(ctx, "shell_command"),
					"tool_name": "functions.shell_command",
				}) + antiPoisonGuardJSONCloseTag},
			}},
			map[string]any{"type": "function_call", "call_id": "call_real_123456", "name": "shell_command", "arguments": `{"command":"Get-Location"}`},
		},
	})

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if !result.Valid || result.Blocked {
		t.Fatalf("expected namespaced Codex tool guard to match real tool, got %#v", result)
	}
	if strings.Contains(string(result.Body), antiPoisonGuardJSONOpenTag) || strings.Contains(string(result.Body), "functions.shell_command") {
		t.Fatalf("expected guard json stripped from response body, got %s", result.Body)
	}
}

func TestAntiPoisonMissingGuardReasonIsExplicit(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	raw := []byte(`{
		"id":"resp_test",
		"output":[
			{"type":"function_call","call_id":"call_real_123456","name":"shell_command","arguments":"{\"command\":\"pwd\"}"}
		]
	}`)

	result := validateAndStripAntiPoisonOpenAIResponse(raw, "responses", ctx)
	if result.Valid || !result.Blocked {
		t.Fatalf("expected missing guard block, got %#v", result)
	}
	if !strings.Contains(result.Reason, "missing_guard_toolcall") || !strings.Contains(result.Reason, antiPoisonGuardJSONOpenTag) || !strings.Contains(result.Reason, "name") || !strings.Contains(result.Reason, "tool_name") {
		t.Fatalf("expected explicit missing guard reason, got %q", result.Reason)
	}
}

func TestAntiPoisonDigestDoesNotDependOnRuntimeCallID(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	first := []antiPoisonToolCall{{
		Name:          "Bash",
		CallID:        "call_runtime_first",
		ArgumentsText: `{"command":"pwd","timeout":120000}`,
		ToolType:      "command",
	}}
	second := []antiPoisonToolCall{{
		Name:          "Bash",
		CallID:        "call_runtime_second",
		ArgumentsText: `{"timeout":120000,"command":"pwd"}`,
		ToolType:      "command",
	}}

	if got, want := computeAntiPoisonToolChainDigest(first, ctx), computeAntiPoisonToolChainDigest(second, ctx); got != want {
		t.Fatalf("expected digest independent from runtime call id and JSON key order, got %s want %s", got, want)
	}
}

func TestAntiPoisonPromptDoesNotRequireComplexGuardFields(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	prompt := buildAntiPoisonPrompt(ctx)
	for _, forbidden := range []string{
		"guard JSON 必须包含 name、tool_name、tool_type",
		"chain 规则",
		"cover 规则",
		"digest 只需填写",
		"algorithm=",
		"nonce=",
		"canonical_arguments",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("expected minimal guard prompt to omit %q, got %s", forbidden, prompt)
		}
	}
	if !strings.Contains(prompt, "Do not include digest, chain, cover, nonce, algorithm, or tool_type") {
		t.Fatalf("expected prompt to explicitly forbid complex guard fields, got %s", prompt)
	}
}
func TestAntiPoisonPromptRequiresExplicitFailureReason(t *testing.T) {
	ctx := testAntiPoisonContext("responses")
	prompt := buildAntiPoisonPrompt(ctx)
	if !strings.Contains(prompt, "guard generation failed for pending toolcall") || !strings.Contains(prompt, "emit no real toolcall") {
		t.Fatalf("expected prompt to require explicit failure reason, got %s", prompt)
	}
	if !strings.Contains(prompt, "<important_gateway_rules>") || !strings.Contains(prompt, "<gateway_contract>") {
		t.Fatalf("expected prompt to use structured gateway blocks, got %s", prompt)
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
	if strings.Contains(string(protected), "sk-1234567890abcdef") {
		t.Fatalf("expected secret value replaced, got %s", protected)
	}
	if !strings.Contains(string(protected), ".env") {
		t.Fatalf("expected plain file name mention retained, got %s", protected)
	}
	if !strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected placeholder in protected body: %s", protected)
	}
	if !strings.Contains(ctx.Records[0].Context, "please read") || !strings.Contains(ctx.Records[0].Context, "sk-1234567890abcdef") {
		t.Fatalf("expected protection record context to include payload excerpt, got %#v", ctx.Records[0])
	}

	restored := restoreAntiPoisonStringProtectionInJSONBody(protected, &ctx, "chat", "provider-test", "openai")
	if !strings.Contains(string(restored), ".env") || !strings.Contains(string(restored), "sk-1234567890abcdef") {
		t.Fatalf("expected original strings restored, got %s", restored)
	}
	if len(ctx.Records) < 2 {
		t.Fatalf("expected respond in restore record, got %#v", ctx.Records)
	}
	restoreRecord := ctx.Records[len(ctx.Records)-1]
	if !strings.Contains(restoreRecord.Before, "__AAD_STR_") || strings.Contains(restoreRecord.Before, "sk-1234567890abcdef") {
		t.Fatalf("expected restore before to show placeholder, got %#v", restoreRecord)
	}
	if !strings.Contains(restoreRecord.After, "sk-1234567890abcdef") || strings.Contains(restoreRecord.After, "restored for client") {
		t.Fatalf("expected restore after to show original value, got %#v", restoreRecord)
	}
}

func TestAntiPoisonStringProtectionDoesNotMaskPlainFileNamesInPromptText(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "please edit settings.json and .claude/settings.json but do not read secrets",
					},
				},
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected plain file names to remain visible, got %s", protected)
	}
	if len(ctx.Records) != 0 {
		t.Fatalf("expected no protection records for plain file names, got %#v", ctx.Records)
	}
}

func TestAntiPoisonStringProtectionDoesNotMaskPlainSensitiveFileNameMention(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "you may mention .env but do not read it",
					},
				},
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected plain .env mention to remain visible, got %s", protected)
	}
	if len(ctx.Records) != 0 {
		t.Fatalf("expected no protection records for plain .env mention, got %#v", ctx.Records)
	}
}

func TestAntiPoisonStringProtectionMasksUserMarkedAngleContent(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "login with <<passw0rd>> and mention .env only as a filename",
					},
				},
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "<<passw0rd>>") || !strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected user-marked angle content protected, got %s", protected)
	}
	if !strings.Contains(string(protected), ".env") {
		t.Fatalf("expected plain file mention retained, got %s", protected)
	}
	if len(ctx.Records) != 1 || ctx.Records[0].Before != "<<passw0rd>>" || !strings.Contains(ctx.Records[0].Rule, "双尖括号") {
		t.Fatalf("expected one user-marked protection record, got %#v", ctx.Records)
	}
	if !strings.Contains(ctx.Records[0].Context, "login with") || !strings.Contains(ctx.Records[0].Context, ".env") {
		t.Fatalf("expected protection record context to include surrounding user text, got %#v", ctx.Records[0])
	}

	var placeholder string
	for key := range ctx.mapping {
		placeholder = key
	}
	if placeholder == "" {
		t.Fatalf("expected placeholder mapping, got %#v", ctx.mapping)
	}
	upstreamResponse := mustJSON(t, map[string]any{
		"output": []any{
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": "received " + placeholder}}},
		},
	})
	restored := restoreAntiPoisonStringProtectionInJSONBody(upstreamResponse, &ctx, "responses", "provider-test", "openai")
	var decoded map[string]any
	if err := json.Unmarshal(restored, &decoded); err != nil {
		t.Fatalf("restored response must remain valid JSON: %v body=%s", err, restored)
	}
	text := decoded["output"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "<<passw0rd>>") || strings.Contains(text, placeholder) {
		t.Fatalf("expected user-marked content restored, got %s", text)
	}
}

func TestAntiPoisonStringProtectionDoesNotMaskSingleAngleUserContextTag(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"input": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": "<environment_context>\n  <cwd>C:\\repo</cwd>\n</environment_context>",
					},
				},
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected single-angle context tags to remain visible, got %s", protected)
	}
	if len(ctx.Records) != 0 {
		t.Fatalf("expected no records for single-angle context tags, got %#v", ctx.Records)
	}
}

func TestAntiPoisonStringProtectionDoesNotMaskAngleContentOutsideUserText(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"tools": []any{
			map[string]any{
				"type":        "function",
				"name":        "read_file",
				"description": "Read <path> and mention .env in documentation",
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "responses", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected tool documentation angle content to remain visible, got %s", protected)
	}
	if len(ctx.Records) != 0 {
		t.Fatalf("expected no records outside user text, got %#v", ctx.Records)
	}
}

func TestAntiPoisonStringProtectionMasksSensitiveToolReadContent(t *testing.T) {
	config := testAntiPoisonConfig()
	raw := mustJSON(t, map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_read_env",
						"content":     ".env\nOPENAI_API_KEY=sk-live-1234567890abcdef\nDEBUG=true",
					},
				},
			},
		},
	})

	protected, ctx, err := applyAntiPoisonStringProtectionToJSONBody(raw, config, "chat", "provider-test", "openai")
	if err != nil {
		t.Fatalf("protect failed: %v", err)
	}
	if !strings.Contains(string(protected), "__AAD_STR_") {
		t.Fatalf("expected sensitive tool-read content protected, got %s", protected)
	}
	if strings.Contains(string(protected), "OPENAI_API_KEY=sk-live-1234567890abcdef") {
		t.Fatalf("expected sensitive tool-read content hidden, got %s", protected)
	}
	if len(ctx.Records) != 1 || ctx.Records[0].Rule != "protect sensitive tool result" {
		t.Fatalf("expected one sensitive tool-read record, got %#v", ctx.Records)
	}

	restored := restoreAntiPoisonStringProtectionInJSONBody(protected, &ctx, "chat", "provider-test", "openai")
	if !strings.Contains(string(restored), "OPENAI_API_KEY=sk-live-1234567890abcdef") {
		t.Fatalf("expected sensitive tool-read content restored, got %s", restored)
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
	if ctx.Records[0].Before != `sk-json-key-value-with-quotes-"-and-slash-\` {
		t.Fatalf("expected original protected value in before, got %#v", ctx.Records[0])
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

func TestAntiPoisonExactRetryIsDisabled(t *testing.T) {
	result := antiPoisonValidationResult{
		Blocked:   true,
		RealCount: 1,
		Reason:    "missing_guard_toolcall",
	}
	if antiPoisonExactRetryEligible(result) {
		t.Fatalf("expected exact retry disabled in minimal guard mode")
	}
}
