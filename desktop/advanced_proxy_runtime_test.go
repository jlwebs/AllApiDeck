package main

import (
	"strings"
	"testing"
)

func TestParseJSONStringMapReturnsEmptyMapOnInvalidJSON(t *testing.T) {
	parsed := parseJSONStringMap(`{"file_path":`)
	if len(parsed) != 0 {
		t.Fatalf("expected invalid json to degrade to empty map, got %#v", parsed)
	}
}

func TestParseToolInputMapDropsOptionalEmptyPaginationFields(t *testing.T) {
	parsed, err := parseToolInputMap(`{"file_path":"C:/tmp/test.txt","limit":60,"offset":2900,"pages":""}`)
	if err != nil {
		t.Fatalf("expected valid tool arguments, got error: %v", err)
	}
	if _, exists := parsed["pages"]; exists {
		t.Fatalf("expected optional empty pages field removed, got %#v", parsed)
	}
	if parsed["file_path"] != "C:/tmp/test.txt" || toIntValue(parsed["limit"]) != 60 || toIntValue(parsed["offset"]) != 2900 {
		t.Fatalf("expected remaining tool args preserved, got %#v", parsed)
	}
}

func TestAnthropicRequestToOpenAIChatMapsImagesStopAndCleansSchema(t *testing.T) {
	request := anthropicRequestToOpenAIChat(map[string]any{
		"model": "gpt-5.4",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "look at this"},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"media_type": "image/png",
							"data":       "abc123",
						},
					},
				},
			},
		},
		"tools": []any{
			map[string]any{
				"name":        "Write",
				"description": "write file",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{"type": "string", "format": "uri"},
					},
					"required": []any{"file_path"},
				},
			},
		},
		"stop_sequences": []any{"</tool>"},
	}, AdvancedProxyProvider{})

	messages, ok := request["messages"].([]map[string]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("expected one mapped chat message, got %#v", request["messages"])
	}
	content, ok := messages[0]["content"].([]map[string]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected multimodal content array, got %#v", messages[0]["content"])
	}
	if content[1]["type"] != "image_url" {
		t.Fatalf("expected image_url content part, got %#v", content[1])
	}
	imageURL, _ := content[1]["image_url"].(map[string]any)
	if imageURL["url"] != "data:image/png;base64,abc123" {
		t.Fatalf("unexpected image url payload: %#v", imageURL)
	}
	stop, ok := request["stop"].([]any)
	if !ok || len(stop) != 1 || stop[0] != "</tool>" {
		t.Fatalf("expected stop_sequences mapped to stop, got %#v", request["stop"])
	}
	tools, ok := request["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one tool, got %#v", request["tools"])
	}
	functionMap, _ := tools[0]["function"].(map[string]any)
	parameters, _ := functionMap["parameters"].(map[string]any)
	properties, _ := parameters["properties"].(map[string]any)
	filePath, _ := properties["file_path"].(map[string]any)
	if _, exists := filePath["format"]; exists {
		t.Fatalf("expected unsupported uri format removed, got %#v", filePath)
	}
}

func TestAnthropicRequestToOpenAIResponsesUsesInstructionsAndMapsImages(t *testing.T) {
	request := anthropicRequestToOpenAIResponses(map[string]any{
		"model":  "gpt-5.4",
		"system": []any{map[string]any{"type": "text", "text": "You are Claude Code."}},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "analyze"},
					map[string]any{
						"type": "image",
						"source": map[string]any{
							"media_type": "image/jpeg",
							"data":       "xyz987",
						},
					},
				},
			},
		},
		"tools": []any{
			map[string]any{
				"name": "Read",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "format": "uri"},
					},
				},
			},
		},
	}, AdvancedProxyProvider{})

	if request["instructions"] != "You are Claude Code." {
		t.Fatalf("expected responses instructions, got %#v", request["instructions"])
	}
	input, ok := request["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("expected one input message, got %#v", request["input"])
	}
	message, _ := input[0].(map[string]any)
	if role := message["role"]; role != "user" {
		t.Fatalf("expected user role in input, got %#v", role)
	}
	content, ok := message["content"].([]map[string]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected text + image content items, got %#v", message["content"])
	}
	if content[1]["type"] != "input_image" || content[1]["image_url"] != "data:image/jpeg;base64,xyz987" {
		t.Fatalf("unexpected image mapping: %#v", content[1])
	}
	tools, ok := request["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one responses tool, got %#v", request["tools"])
	}
	parameters, _ := tools[0]["parameters"].(map[string]any)
	properties, _ := parameters["properties"].(map[string]any)
	path, _ := properties["path"].(map[string]any)
	if _, exists := path["format"]; exists {
		t.Fatalf("expected cleaned responses schema, got %#v", path)
	}
}

func TestAnthropicRequestToOpenAIResponsesAssignsMessageIds(t *testing.T) {
	request := anthropicRequestToOpenAIResponses(map[string]any{
		"model": "gpt-5.4",
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": []any{map[string]any{"type": "text", "text": "hello"}},
			},
			map[string]any{
				"role":    "assistant",
				"content": []any{map[string]any{"type": "tool_use", "id": "call_1", "name": "shell_command", "input": map[string]any{"command": "pwd"}}},
			},
		},
	}, AdvancedProxyProvider{})

	input, ok := request["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("expected two input items, got %#v", request["input"])
	}
	first, _ := input[0].(map[string]any)
	second, _ := input[1].(map[string]any)
	if strings.TrimSpace(toStringValue(first["id"])) == "" {
		t.Fatalf("expected first message id, got %#v", first)
	}
	if strings.TrimSpace(toStringValue(second["id"])) == "" {
		t.Fatalf("expected second message id, got %#v", second)
	}
}

func TestAnthropicRequestToOpenAIResponsesMapsWebSearchTool(t *testing.T) {
	request := anthropicRequestToOpenAIResponses(map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{
				"type":            "web_search_20250305",
				"name":            "web_search",
				"allowed_domains": []any{"github.com"},
				"blocked_domains": []any{"reddit.com"},
			},
		},
		"tool_choice": map[string]any{
			"type": "tool",
			"name": "web_search",
		},
	}, AdvancedProxyProvider{})

	tools, ok := request["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one responses tool, got %#v", request["tools"])
	}
	if tools[0]["type"] != "web_search" {
		t.Fatalf("expected web_search tool mapping, got %#v", tools[0])
	}
	filters, _ := tools[0]["filters"].(map[string]any)
	allowedDomains, _ := filters["allowed_domains"].([]any)
	blockedDomains, _ := filters["blocked_domains"].([]any)
	if len(allowedDomains) != 1 || allowedDomains[0] != "github.com" {
		t.Fatalf("expected allowed_domains passthrough, got %#v", filters)
	}
	if len(blockedDomains) != 1 || blockedDomains[0] != "reddit.com" {
		t.Fatalf("expected blocked_domains passthrough, got %#v", filters)
	}
	if toolChoice := request["tool_choice"]; toolChoice != "required" {
		t.Fatalf("expected web_search forced tool choice to map to required, got %#v", toolChoice)
	}
	include, _ := request["include"].([]any)
	if len(include) != 1 || include[0] != "web_search_call.action.sources" {
		t.Fatalf("expected web search sources include, got %#v", request["include"])
	}
}

func TestClassifyClaudeRequestFeaturesDetectsAnthropicWebSearchTool(t *testing.T) {
	features := classifyClaudeRequestFeatures(map[string]any{
		"tools": []any{
			map[string]any{
				"type":            "web_search_20250305",
				"name":            "web_search",
				"allowed_domains": []any{"github.com"},
			},
		},
	})

	if !features.HasAnthropicWebSearchTool {
		t.Fatalf("expected web_search tool to be detected, got %#v", features)
	}
	if !features.requiresResponsesOrAnthropicProvider() {
		t.Fatalf("expected detected web_search tool to require responses-or-anthropic compatibility")
	}
}

func TestFilterCompatibleClaudeProvidersPreservesQueueOrderForWebSearch(t *testing.T) {
	providers := []AdvancedProxyProvider{
		{ID: "openai-chat", APIFormat: "openai_chat"},
		{ID: "anthropic", APIFormat: "anthropic"},
		{ID: "openai-responses", APIFormat: "openai_responses"},
	}

	filtered := filterCompatibleClaudeProviders(providers, claudeRequestFeatures{
		HasAnthropicWebSearchTool: true,
	})

	if len(filtered) != 3 {
		t.Fatalf("expected three compatible providers, got %#v", filtered)
	}
	if filtered[0].ID != "openai-chat" || filtered[1].ID != "anthropic" || filtered[2].ID != "openai-responses" {
		t.Fatalf("expected original queue order to be preserved, got %#v", filtered)
	}
}

func TestFilterCompatibleClaudeProvidersKeepsRegularRequestsUntouched(t *testing.T) {
	providers := []AdvancedProxyProvider{
		{ID: "openai-chat", APIFormat: "openai_chat"},
		{ID: "anthropic", APIFormat: "anthropic"},
	}

	filtered := filterCompatibleClaudeProviders(providers, claudeRequestFeatures{})

	if len(filtered) != len(providers) {
		t.Fatalf("expected all providers retained, got %#v", filtered)
	}
	for index := range providers {
		if filtered[index].ID != providers[index].ID {
			t.Fatalf("expected provider order preserved, got %#v", filtered)
		}
	}
}

func TestIncompatibleClaudeRequestMessageMentionsAnthropicWebSearch(t *testing.T) {
	message := incompatibleClaudeRequestMessage(claudeRequestFeatures{
		HasAnthropicWebSearchTool: true,
	})

	if !strings.Contains(message, "web_search") || !strings.Contains(strings.ToLower(message), "anthropic") || !strings.Contains(strings.ToLower(message), "responses") {
		t.Fatalf("expected explicit compatibility message, got %q", message)
	}
}

func TestSanitizeOrphanToolResultsRewritesDanglingToolResult(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_orphan",
						"content": []any{
							map[string]any{"type": "text", "text": "write failed"},
						},
					},
				},
			},
		},
	}

	sanitized := sanitizeOrphanToolResults(body)
	if sanitized != 1 {
		t.Fatalf("expected one sanitized block, got %d", sanitized)
	}

	messages, _ := body["messages"].([]any)
	message, _ := messages[0].(map[string]any)
	content, _ := message["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Fatalf("expected orphan tool_result rewritten as text, got %#v", block)
	}
	if block["text"] != "[Tool result for toolu_orphan]: write failed" {
		t.Fatalf("unexpected fallback text: %#v", block["text"])
	}
}

func TestSanitizeOrphanToolResultsPreservesAdjacentToolResult(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "toolu_keep",
						"name":  "write_file",
						"input": map[string]any{"file_path": "/tmp/test.txt"},
					},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_keep",
						"content": []any{
							map[string]any{"type": "text", "text": "ok"},
						},
					},
				},
			},
		},
	}

	sanitized := sanitizeOrphanToolResults(body)
	if sanitized != 0 {
		t.Fatalf("expected no sanitization for adjacent tool_result, got %d", sanitized)
	}

	messages, _ := body["messages"].([]any)
	message, _ := messages[1].(map[string]any)
	content, _ := message["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["type"] != "tool_result" || block["tool_use_id"] != "toolu_keep" {
		t.Fatalf("expected valid tool_result preserved, got %#v", block)
	}
}
