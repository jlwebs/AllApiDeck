package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type failingReadCloser struct {
	chunks [][]byte
	err    error
}

func (f *failingReadCloser) Read(p []byte) (int, error) {
	if len(f.chunks) == 0 {
		if f.err != nil {
			return 0, f.err
		}
		return 0, io.EOF
	}
	chunk := f.chunks[0]
	f.chunks = f.chunks[1:]
	return copy(p, chunk), nil
}

func (f *failingReadCloser) Close() error {
	return nil
}

func contentBlocksOf(t *testing.T, raw any) []map[string]any {
	t.Helper()
	switch typed := raw.(type) {
	case []map[string]any:
		return typed
	case []any:
		blocks := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("unexpected content block: %#v", item)
			}
			blocks = append(blocks, block)
		}
		return blocks
	default:
		t.Fatalf("unexpected content shape: %#v", raw)
		return nil
	}
}

func TestOpenAIChatToAnthropicSkipsThinkingWhenNotRequested(t *testing.T) {
	response := map[string]any{
		"id":    "resp_123",
		"model": "gpt-5.4",
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content":           "hello",
					"reasoning_content": "internal reasoning",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 5,
		},
	}

	result := openAIChatToAnthropic(response, "gpt-5.4", false)
	content := contentBlocksOf(t, result["content"])
	if len(content) != 1 || content[0]["type"] != "text" {
		t.Fatalf("expected only text content, got %#v", content)
	}
}

func TestOpenAIChatToAnthropicIncludesThinkingWhenRequested(t *testing.T) {
	response := map[string]any{
		"id":    "resp_123",
		"model": "gpt-5.4",
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content":           "hello",
					"reasoning_content": "internal reasoning",
				},
				"finish_reason": "stop",
			},
		},
	}

	result := openAIChatToAnthropic(response, "gpt-5.4", true)
	content := contentBlocksOf(t, result["content"])
	if len(content) < 2 {
		t.Fatalf("expected thinking + text blocks, got %#v", result["content"])
	}

	first := content[0]
	second := content[1]
	if first["type"] != "thinking" || second["type"] != "text" {
		t.Fatalf("unexpected content blocks: %#v", content)
	}
}

func TestOpenAIResponsesToAnthropicPreservesWhitespaceOnlyTextSegments(t *testing.T) {
	response := map[string]any{
		"id":    "resp_456",
		"model": "gpt-5.4",
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "text", "text": "line1"},
					map[string]any{"type": "text", "text": "\n"},
					map[string]any{"type": "text", "text": "line2"},
				},
			},
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	if len(content) != 3 {
		t.Fatalf("expected 3 text blocks, got %#v", content)
	}
	if content[1]["type"] != "text" || content[1]["text"] != "\n" {
		t.Fatalf("expected newline text block to be preserved, got %#v", content[1])
	}
}

func TestOpenAIResponsesToAnthropicMarksToolUseStopReason(t *testing.T) {
	response := map[string]any{
		"id":     "resp_tool_123",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_123",
				"name":      "search_docs",
				"arguments": `{"q":"advanced proxy"}`,
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 6,
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	if got := result["stop_reason"]; got != "tool_use" {
		t.Fatalf("expected stop_reason tool_use, got %#v", got)
	}
	content := contentBlocksOf(t, result["content"])
	if len(content) != 1 || content[0]["type"] != "tool_use" {
		t.Fatalf("expected tool_use content block, got %#v", content)
	}
}

func TestOpenAIResponsesToAnthropicSurfacesInvalidToolArgumentsAsText(t *testing.T) {
	response := map[string]any{
		"id":     "resp_tool_invalid",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_bad",
				"name":      "Read",
				"arguments": `{"file_path":`,
			},
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	if len(content) != 1 || content[0]["type"] != "text" {
		t.Fatalf("expected invalid tool args to become assistant text, got %#v", content)
	}
	if !strings.Contains(toStringValue(content[0]["text"]), "invalid") {
		t.Fatalf("expected error text to mention invalid arguments, got %#v", content[0]["text"])
	}
	if got := result["stop_reason"]; got != "end_turn" {
		t.Fatalf("expected stop_reason end_turn without valid tool_use, got %#v", got)
	}
}

func TestOpenAIResponsesToAnthropicMapsWebSearchBlocksAndUsage(t *testing.T) {
	response := map[string]any{
		"id":     "resp_search_123",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type": "web_search_call",
				"id":   "ws_123",
				"action": map[string]any{
					"type":  "search",
					"query": "site:github.com CLIProxyAPI",
					"sources": []any{
						map[string]any{
							"type":  "url",
							"url":   "https://github.com/router-for-me/CLIProxyAPI/issues/2599",
							"title": "router-for-me/CLIProxyAPI issue #2599",
						},
					},
				},
			},
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "output_text", "text": "Top GitHub results ..."},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 8,
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	if len(content) != 3 {
		t.Fatalf("expected server_tool_use + web_search_tool_result + text, got %#v", content)
	}
	if content[0]["type"] != "server_tool_use" || content[0]["name"] != "web_search" {
		t.Fatalf("expected server_tool_use block, got %#v", content[0])
	}
	if content[1]["type"] != "web_search_tool_result" || content[1]["tool_use_id"] != "srvtoolu_ws123" {
		t.Fatalf("expected web_search_tool_result block, got %#v", content[1])
	}
	usage, _ := result["usage"].(map[string]any)
	serverToolUse, _ := usage["server_tool_use"].(map[string]any)
	if toIntValue(serverToolUse["web_search_requests"]) != 1 {
		t.Fatalf("expected one web search request in usage, got %#v", result["usage"])
	}
}

func TestOpenAIResponsesToAnthropicBuildsWebSearchResultFromAnnotations(t *testing.T) {
	response := map[string]any{
		"id":     "resp_search_annotations",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type": "web_search_call",
				"id":   "ws_annotations",
				"action": map[string]any{
					"type":  "search",
					"query": "site:github.com CLIProxyAPI",
				},
			},
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": "Top GitHub results ...",
						"annotations": []any{
							map[string]any{
								"type":        "url_citation",
								"url":         "https://github.com/router-for-me/CLIProxyAPI/issues/2599",
								"title":       "router-for-me/CLIProxyAPI issue #2599",
								"start_index": 4,
								"end_index":   10,
							},
						},
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 8,
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	if len(content) != 3 {
		t.Fatalf("expected server_tool_use + synthesized web_search_tool_result + text, got %#v", content)
	}
	if content[1]["type"] != "web_search_tool_result" || content[1]["tool_use_id"] != "srvtoolu_wsannotations" {
		t.Fatalf("expected synthesized web_search_tool_result block, got %#v", content[1])
	}
	switch typed := content[1]["content"].(type) {
	case []map[string]any:
		if len(typed) != 1 {
			t.Fatalf("expected synthesized search result content, got %#v", content[1]["content"])
		}
	case []any:
		if len(typed) != 1 {
			t.Fatalf("expected synthesized search result content, got %#v", content[1]["content"])
		}
	default:
		t.Fatalf("expected synthesized search result content, got %#v", content[1]["content"])
	}
	textBlock := content[2]
	citations, ok := textBlock["citations"].([]map[string]any)
	if !ok || len(citations) != 1 {
		t.Fatalf("expected text citations derived from annotations, got %#v", textBlock)
	}
}

func TestOpenAIResponsesToAnthropicSynthesizesLifecycleFromAnnotationsWithoutCall(t *testing.T) {
	response := map[string]any{
		"id":     "resp_search_annotations_only",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": "Top GitHub results ...",
						"annotations": []any{
							map[string]any{
								"type":  "url_citation",
								"url":   "https://github.com/router-for-me/CLIProxyAPI/issues/2599",
								"title": "router-for-me/CLIProxyAPI issue #2599",
							},
						},
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 8,
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	if len(content) != 3 {
		t.Fatalf("expected synthesized server_tool_use + web_search_tool_result + text, got %#v", content)
	}
	foundServerToolUse := false
	foundSearchResult := false
	for _, block := range content {
		switch block["type"] {
		case "server_tool_use":
			foundServerToolUse = true
		case "web_search_tool_result":
			foundSearchResult = true
		}
	}
	if !foundServerToolUse || !foundSearchResult {
		t.Fatalf("expected synthesized web search lifecycle, got %#v", content)
	}
	usage, _ := result["usage"].(map[string]any)
	serverToolUse, _ := usage["server_tool_use"].(map[string]any)
	if toIntValue(serverToolUse["web_search_requests"]) != 1 {
		t.Fatalf("expected one synthesized web search request in usage, got %#v", result["usage"])
	}
}

func TestOpenAIResponsesToAnthropicSynthesizesLifecycleFromTextURLsWithoutCall(t *testing.T) {
	response := map[string]any{
		"id":     "resp_search_text_only",
		"model":  "gpt-5.4",
		"status": "completed",
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": "Sources:\n- https://github.com/anthropics/claude-code\n- https://github.com/anthropics/claude-code/blob/main/README.md",
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  12,
			"output_tokens": 8,
		},
	}

	result := openAIResponsesToAnthropic(response, "gpt-5.4")
	content := contentBlocksOf(t, result["content"])
	foundServerToolUse := false
	foundSearchResult := false
	for _, block := range content {
		switch block["type"] {
		case "server_tool_use":
			foundServerToolUse = true
		case "web_search_tool_result":
			foundSearchResult = true
		}
	}
	if !foundServerToolUse || !foundSearchResult {
		t.Fatalf("expected synthesized web search lifecycle from text URLs, got %#v", content)
	}
}

func TestBuildClaudeProviderHeadersPreservesAnthropicHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("User-Agent", "Claude-Client")
	headers.Set("anthropic-version", "2023-06-01")
	headers.Set("anthropic-beta", "claude-code-20250219,test-beta")

	result := buildClaudeProviderHeaders(
		AdvancedProxyProvider{APIKey: "sk-test"},
		"anthropic",
		headers,
		false,
	)

	if result["User-Agent"] != "Claude-Client" {
		t.Fatalf("expected user-agent passthrough, got %q", result["User-Agent"])
	}
	if result["anthropic-version"] != "2023-06-01" {
		t.Fatalf("expected anthropic-version passthrough, got %q", result["anthropic-version"])
	}
	if result["anthropic-beta"] != "claude-code-20250219,test-beta" {
		t.Fatalf("expected anthropic-beta passthrough, got %q", result["anthropic-beta"])
	}
	if result["x-api-key"] != "sk-test" {
		t.Fatalf("expected x-api-key to be set, got %q", result["x-api-key"])
	}
}

func resetAdvancedProxyRuntimeForTest(t *testing.T) string {
	t.Helper()

	runtimeDir := t.TempDir()
	previousRuntimeDir := os.Getenv("BATCH_API_CHECK_RUNTIME_DIR")
	if err := os.Setenv("BATCH_API_CHECK_RUNTIME_DIR", runtimeDir); err != nil {
		t.Fatalf("set runtime dir env: %v", err)
	}
	t.Cleanup(func() {
		if previousRuntimeDir == "" {
			_ = os.Unsetenv("BATCH_API_CHECK_RUNTIME_DIR")
		} else {
			_ = os.Setenv("BATCH_API_CHECK_RUNTIME_DIR", previousRuntimeDir)
		}
	})

	advancedProxyRuntime.mu.Lock()
	advancedProxyRuntime.breakers = map[string]*proxyCircuitBreaker{}
	advancedProxyRuntime.routes = map[string]*proxyRouteState{}
	advancedProxyRuntime.providerRoutes = map[string]*proxyProviderRouteState{}
	advancedProxyRuntime.providerHealth = map[string]*proxyProviderHealthState{}
	advancedProxyRuntime.rpmDispatchHistory = map[string][]time.Time{}
	advancedProxyRuntime.logs = map[string]time.Time{}
	advancedProxyRuntime.mu.Unlock()

	advancedProxyEncryptedContentHealState.mu.Lock()
	advancedProxyEncryptedContentHealState.sessions = map[string]int{}
	advancedProxyEncryptedContentHealState.mu.Unlock()

	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	return runtimeDir
}

func TestForwardClaudeRequestViaProviderUpdatesRoutingSnapshotForAnthropic(t *testing.T) {
	runtimeDir := resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-provider",
		RowKey:    "row-claude",
		Name:      "Claude Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "anthropic",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "claude-sonnet",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})

	if result.StatusCode != http.StatusOK || result.Response == nil {
		t.Fatalf("expected successful response, got %#v", result)
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state, exists := snapshot.Apps["claude"]
	if !exists {
		t.Fatalf("expected claude routing snapshot, got %#v", snapshot.Apps)
	}
	if state.ProviderID != provider.ID || state.ProviderRowKey != provider.RowKey {
		t.Fatalf("unexpected provider binding: %#v", state)
	}
	if state.RouteKind != "messages" || state.Status != "success" {
		t.Fatalf("unexpected claude routing state: %#v", state)
	}
	if state.TargetURL != server.URL+"/v1/messages" {
		t.Fatalf("unexpected target url: %#v", state)
	}

	snapshotPath := filepath.Join(runtimeDir, "advanced-proxy", "routing-snapshot.json")
	if _, err := os.Stat(snapshotPath); err != nil {
		t.Fatalf("expected persisted routing snapshot at %s: %v", snapshotPath, err)
	}
}

func TestForwardClaudeRequestViaProviderPersistsResponsesUpgradeAfterSuccessfulProbe(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"resp_test","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-provider",
		RowKey:    "row-claude-openai",
		Name:      "Claude OpenAI Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	_, err := saveAdvancedProxyConfig(AdvancedProxyConfig{
		Queues: AdvancedProxyQueuesConfig{
			Global: AdvancedProxyQueueConfig{
				Providers: []AdvancedProxyProvider{provider},
			},
			Claude:   defaultAdvancedProxyQueueConfig(true),
			Codex:    defaultAdvancedProxyQueueConfig(true),
			OpenCode: defaultAdvancedProxyQueueConfig(true),
			OpenClaw: defaultAdvancedProxyQueueConfig(true),
		},
		Claude: ClaudeProxyCompatConfig{
			Enabled:  true,
			BasePath: advancedProxyClaudeBasePath,
		},
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "claude-sonnet",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})

	if result.StatusCode != http.StatusOK || result.Response == nil {
		t.Fatalf("expected successful response, got %#v", result)
	}

	saved, err := loadAdvancedProxyConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(saved.Queues.Global.Providers) != 1 {
		t.Fatalf("expected one saved provider, got %#v", saved.Queues.Global.Providers)
	}
	if saved.Queues.Global.Providers[0].APIFormat != "openai_responses" {
		t.Fatalf("expected provider format persisted as openai_responses, got %#v", saved.Queues.Global.Providers[0])
	}
}

func TestForwardClaudeRequestViaProviderFallsBackToChatWithoutPersistingResponses(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/responses":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"responses endpoint not found"}}`))
		case "/v1/chat/completions":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-provider",
		RowKey:    "row-claude-openai",
		Name:      "Claude OpenAI Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	_, err := saveAdvancedProxyConfig(AdvancedProxyConfig{
		Queues: AdvancedProxyQueuesConfig{
			Global: AdvancedProxyQueueConfig{
				Providers: []AdvancedProxyProvider{provider},
			},
			Claude:   defaultAdvancedProxyQueueConfig(true),
			Codex:    defaultAdvancedProxyQueueConfig(true),
			OpenCode: defaultAdvancedProxyQueueConfig(true),
			OpenClaw: defaultAdvancedProxyQueueConfig(true),
		},
		Claude: ClaudeProxyCompatConfig{
			Enabled:  true,
			BasePath: advancedProxyClaudeBasePath,
		},
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "claude-sonnet",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})

	if result.StatusCode != http.StatusOK || result.Response == nil {
		t.Fatalf("expected fallback chat response to succeed, got %#v", result)
	}

	saved, err := loadAdvancedProxyConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(saved.Queues.Global.Providers) != 1 {
		t.Fatalf("expected one saved provider, got %#v", saved.Queues.Global.Providers)
	}
	if saved.Queues.Global.Providers[0].APIFormat != "openai_chat" {
		t.Fatalf("expected provider format to remain openai_chat after fallback, got %#v", saved.Queues.Global.Providers[0])
	}
}

func TestForwardClaudeRequestViaProviderUpdatesRoutingSnapshotForOpenAIChatConfiguredProviderStream(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte(strings.Join([]string{
			`event: response.created`,
			`data: {"type":"response.created","response":{"id":"resp_test","model":"gpt-5.4"}}`,
			"",
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"hi"}`,
			"",
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"id":"resp_test","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":1}}}`,
			"",
		}, "\n")))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-provider",
		RowKey:    "row-claude-openai",
		Name:      "Claude OpenAI Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model":  "gpt-5.4",
		"stream": true,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, true, AdvancedProxyConfig{})
	if result.StatusCode != http.StatusOK || result.StreamBody == nil {
		t.Fatalf("expected successful stream response, got %#v", result)
	}
	_ = result.StreamBody.Close()

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state := snapshot.Apps["claude"]
	if state.ProviderID != provider.ID || state.ProviderRowKey != provider.RowKey {
		t.Fatalf("unexpected provider binding: %#v", state)
	}
	if state.RouteKind != "responses" || state.Status != "success" {
		t.Fatalf("unexpected claude streaming route state: %#v", state)
	}
	if state.TargetURL != server.URL+"/v1/responses" {
		t.Fatalf("unexpected target url: %#v", state)
	}
}

func TestForwardClaudeRequestViaProviderUpdatesRoutingSnapshotForOpenAIResponsesStream(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte(strings.Join([]string{
			`event: response.created`,
			`data: {"type":"response.created","response":{"id":"resp_test","model":"gpt-5.4"}}`,
			"",
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2}}}`,
			"",
		}, "\n")))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-responses-provider",
		RowKey:    "row-claude-openai-responses",
		Name:      "Claude OpenAI Responses Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_responses",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model":  "gpt-5.4",
		"stream": true,
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, true, AdvancedProxyConfig{})
	if result.StatusCode != http.StatusOK || result.StreamBody == nil {
		t.Fatalf("expected successful responses stream, got %#v", result)
	}
	_ = result.StreamBody.Close()

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state := snapshot.Apps["claude"]
	if state.ProviderID != provider.ID || state.ProviderRowKey != provider.RowKey {
		t.Fatalf("unexpected provider binding: %#v", state)
	}
	if state.RouteKind != "responses" || state.Status != "success" {
		t.Fatalf("unexpected claude responses route state: %#v", state)
	}
	if state.TargetURL != server.URL+"/v1/responses" {
		t.Fatalf("unexpected target url: %#v", state)
	}
}

func TestForwardClaudeRequestViaProviderFallsBackFromResponsesToChat(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		switch request.URL.Path {
		case "/v1/responses", "/responses":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"OpenAI Responses endpoint is not supported on this upstream"}}`))
		case "/v1/chat/completions", "/chat/completions":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","object":"chat.completion","model":"gpt-5.5","choices":[{"index":0,"message":{"role":"assistant","content":"fallback ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-responses-provider",
		RowKey:    "row-claude-openai-responses",
		Name:      "Claude OpenAI Responses Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_responses",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "gpt-5.5",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})
	if result.StatusCode != http.StatusOK || result.Response == nil {
		t.Fatalf("expected successful chat fallback, got %#v", result)
	}
	if requestCount != 2 {
		t.Fatalf("expected one responses attempt and one chat fallback, got %d", requestCount)
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state := snapshot.Apps["claude"]
	if state.RouteKind != "chat" || state.Status != "success" {
		t.Fatalf("expected chat fallback route state, got %#v", state)
	}
	if state.TargetURL != server.URL+"/v1/chat/completions" && state.TargetURL != server.URL+"/chat/completions" {
		t.Fatalf("unexpected fallback target url: %#v", state)
	}
}

func TestForwardClaudeRequestViaProviderPromotesChatConfiguredWebSearchToResponses(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		if request.URL.Path != "/v1/responses" && request.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"resp_test","object":"response","model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_1","status":"completed","content":[{"type":"output_text","text":"search ok"}]}],"usage":{"input_tokens":3,"output_tokens":2}}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-chat-provider",
		RowKey:    "row-claude-openai-chat",
		Name:      "Claude OpenAI Chat Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{
				"type":            "web_search_20250305",
				"name":            "web_search",
				"allowed_domains": []any{"github.com"},
			},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "search"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})
	if result.StatusCode != http.StatusOK || result.Response == nil {
		t.Fatalf("expected promoted responses request to succeed, got %#v", result)
	}
	if requestCount != 1 {
		t.Fatalf("expected single promoted responses attempt, got %d", requestCount)
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state := snapshot.Apps["claude"]
	if state.RouteKind != "responses" || state.Status != "success" {
		t.Fatalf("expected responses route after promotion, got %#v", state)
	}
}

func TestForwardClaudeRequestViaProviderDoesNotFallbackWebSearchToChat(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		if request.URL.Path != "/v1/responses" && request.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte(`{"error":{"message":"OpenAI Responses endpoint is not supported on this upstream"}}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "claude-openai-responses-provider",
		RowKey:    "row-claude-openai-responses",
		Name:      "Claude OpenAI Responses Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_responses",
	}

	result := forwardClaudeRequestViaProvider(provider, map[string]any{
		"model": "gpt-5.5",
		"tools": []any{
			map[string]any{
				"type":            "web_search_20250305",
				"name":            "web_search",
				"allowed_domains": []any{"github.com"},
			},
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "search"},
		},
	}, http.Header{}, false, AdvancedProxyConfig{})
	if result.StatusCode != http.StatusNotFound {
		t.Fatalf("expected responses failure to bubble for web_search request, got %#v", result)
	}
	if requestCount != 2 {
		t.Fatalf("expected web_search request to stay on responses candidates only, got %d attempts", requestCount)
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	state := snapshot.Apps["claude"]
	if state.RouteKind != "responses" || state.Status != "failed" {
		t.Fatalf("expected responses route error state without chat fallback, got %#v", state)
	}
}

func TestWriteAnthropicSSEFromOpenAIChatStreamPreservesNewlineDeltas(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-test","choices":[{"delta":{"content":"line1"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"\n"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"line2"}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIChatStream(recorder, streamBody, "gpt-5.4", false)

	body := recorder.Body.String()
	if !strings.Contains(body, `"text":"line1"`) {
		t.Fatalf("expected first text delta, got %q", body)
	}
	if strings.Count(body, `"type":"content_block_delta"`) != 3 {
		t.Fatalf("expected 3 content deltas including newline chunk, got %q", body)
	}
	if !strings.Contains(body, `"text":"line2"`) {
		t.Fatalf("expected second text delta, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIChatStreamMergesCumulativeToolArgs(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-tool","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_write","function":{"name":"write_file","arguments":"{\"file_path\":\"/tmp/test.txt\"}"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file_path\":\"/tmp/test.txt\",\"content\":\"hello\"}"}}]}}]}`,
		"",
		`data: {"choices":[{"finish_reason":"tool_calls","delta":{}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIChatStream(recorder, streamBody, "gpt-5.4", false)

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"tool_use"`) {
		t.Fatalf("expected tool_use block, got %q", body)
	}
	if strings.Count(body, `"type":"content_block_delta"`) != 2 {
		t.Fatalf("expected cumulative tool args to stream as two deltas, got %q", body)
	}
	if !strings.Contains(body, `"partial_json":"{\"file_path\":\"/tmp/test.txt\"}"`) {
		t.Fatalf("expected initial tool args delta, got %q", body)
	}
	if !strings.Contains(body, `"partial_json":",\"content\":\"hello\"}"`) {
		t.Fatalf("expected incremental suffix delta for cumulative tool args, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIChatStreamDelaysToolStartUntilIDAndNameReady(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`data: {"id":"chatcmpl-tool-delay","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"file_path\":\"/tmp/test.txt\",\"content\":\"hel"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_write","function":{"name":"write_file"}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"lo\"}"}}]}}]}`,
		"",
		`data: {"choices":[{"finish_reason":"tool_calls","delta":{}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIChatStream(recorder, streamBody, "gpt-5.4", false)

	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"call_write"`) || !strings.Contains(body, `"name":"write_file"`) {
		t.Fatalf("expected delayed tool block start to preserve id/name, got %q", body)
	}
	if !strings.Contains(body, `"partial_json":"{\"file_path\":\"/tmp/test.txt\",\"content\":\"hel"`) {
		t.Fatalf("expected buffered tool args emitted after delayed start, got %q", body)
	}
	if !strings.Contains(body, `"partial_json":"lo\"}"`) {
		t.Fatalf("expected trailing tool args delta after start, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamEmitsToolUseLifecycle(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tool_stream","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item_id":"fc_1","item":{"id":"fc_1","type":"function_call","call_id":"call_123","name":"search_docs"}}`,
		"",
		`event: response.function_call_arguments.delta`,
		`data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"q\":\"hello\"}"}`,
		"",
		`event: response.function_call_arguments.done`,
		`data: {"type":"response.function_call_arguments.done","item_id":"fc_1"}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":8,"output_tokens":4}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"tool_use"`) {
		t.Fatalf("expected tool_use block, got %q", body)
	}
	if !strings.Contains(body, `"partial_json":"{\"q\":\"hello\"}"`) {
		t.Fatalf("expected input_json_delta for tool args, got %q", body)
	}
	if !strings.Contains(body, `"stop_reason":"tool_use"`) {
		t.Fatalf("expected tool_use stop reason, got %q", body)
	}
	if !strings.Contains(body, `"type":"message_stop"`) {
		t.Fatalf("expected message_stop, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamEmitsWebSearchLifecycle(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"site:github.com CLIProxyAPI","sources":[{"type":"url","url":"https://github.com/router-for-me/CLIProxyAPI/issues/2599","title":"router-for-me/CLIProxyAPI issue #2599"}]}}}`,
		"",
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Top GitHub results ..."}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"server_tool_use"`) {
		t.Fatalf("expected server_tool_use SSE block, got %q", body)
	}
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected web_search_tool_result SSE block, got %q", body)
	}
	if !strings.Contains(body, `"web_search_requests":1`) {
		t.Fatalf("expected web search usage count in SSE message_delta, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamReplaysWebSearchFromCompletedPayload(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Top GitHub results ..."}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2},"output":[{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"site:github.com CLIProxyAPI","sources":[{"type":"url","url":"https://github.com/router-for-me/CLIProxyAPI/issues/2599","title":"router-for-me/CLIProxyAPI issue #2599"}]}}]}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"server_tool_use"`) {
		t.Fatalf("expected replayed server_tool_use from completed payload, got %q", body)
	}
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected replayed web_search_tool_result from completed payload, got %q", body)
	}
	if !strings.Contains(body, `"web_search_requests":1`) {
		t.Fatalf("expected replayed web search usage count, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamSynthesizesWebSearchResultFromAnnotations(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"site:github.com CLIProxyAPI"}}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2},"output":[{"type":"web_search_call","id":"ws_1","status":"completed","action":{"type":"search","query":"site:github.com CLIProxyAPI"}},{"type":"message","content":[{"type":"output_text","text":"Top GitHub results ...","annotations":[{"type":"url_citation","url":"https://github.com/router-for-me/CLIProxyAPI/issues/2599","title":"router-for-me/CLIProxyAPI issue #2599"}]}]}]}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected synthesized web_search_tool_result from annotations, got %q", body)
	}
	if !strings.Contains(body, `https://github.com/router-for-me/CLIProxyAPI/issues/2599`) {
		t.Fatalf("expected synthesized source URL in web_search_tool_result, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamSynthesizesLifecycleFromAnnotationsWithoutCall(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2},"output":[{"type":"message","content":[{"type":"output_text","text":"Top GitHub results ...","annotations":[{"type":"url_citation","url":"https://github.com/router-for-me/CLIProxyAPI/issues/2599","title":"router-for-me/CLIProxyAPI issue #2599"}]}]}]}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"server_tool_use"`) {
		t.Fatalf("expected synthesized server_tool_use from annotations-only response, got %q", body)
	}
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected synthesized web_search_tool_result from annotations-only response, got %q", body)
	}
	if !strings.Contains(body, `"web_search_requests":1`) {
		t.Fatalf("expected synthesized web search usage count, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamSynthesizesFromAnnotationAddedEvents(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Top GitHub results ..."}`,
		"",
		`event: response.output_text.annotation.added`,
		`data: {"type":"response.output_text.annotation.added","item_id":"msg_1","output_index":0,"content_index":0,"annotation":{"type":"url_citation","url":"https://github.com/router-for-me/CLIProxyAPI/issues/2599","title":"router-for-me/CLIProxyAPI issue #2599"}}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"type":"message","id":"msg_1"}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"server_tool_use"`) {
		t.Fatalf("expected synthesized server_tool_use from annotation stream events, got %q", body)
	}
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected synthesized web_search_tool_result from annotation stream events, got %q", body)
	}
	if !strings.Contains(body, `https://github.com/router-for-me/CLIProxyAPI/issues/2599`) {
		t.Fatalf("expected synthesized source URL from annotation stream events, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamSynthesizesFromTextURLsWithoutAnnotations(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_search","model":"gpt-5.4"}}`,
		"",
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Sources:\n- https://github.com/anthropics/claude-code\n- https://github.com/anthropics/claude-code/blob/main/README.md"}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"type":"message","id":"msg_1"}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"server_tool_use"`) {
		t.Fatalf("expected synthesized server_tool_use from text URL stream events, got %q", body)
	}
	if !strings.Contains(body, `"type":"web_search_tool_result"`) {
		t.Fatalf("expected synthesized web_search_tool_result from text URL stream events, got %q", body)
	}
	if !strings.Contains(body, `https://github.com/anthropics/claude-code`) {
		t.Fatalf("expected synthesized source URL from text URL stream events, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamEmitsToolArgsFromDoneEvents(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tool_done","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item_id":"fc_done","item":{"id":"fc_done","type":"function_call","call_id":"call_done","name":"write_file"}}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item_id":"fc_done","item":{"id":"fc_done","type":"function_call","call_id":"call_done","name":"write_file","arguments":"{\"file_path\":\"/tmp/test.txt\",\"content\":\"hello\"}"}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":9,"output_tokens":5}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"partial_json":"{\"file_path\":\"/tmp/test.txt\",\"content\":\"hello\"}"`) {
		t.Fatalf("expected tool args emitted from done event, got %q", body)
	}
	if !strings.Contains(body, `"type":"message_stop"`) {
		t.Fatalf("expected message_stop, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamSupportsLargeToolArgs(t *testing.T) {
	lines := make([]string, 0, 2000)
	for index := 0; index < 2000; index++ {
		lines = append(lines, fmt.Sprintf("export const value%04d = { id: %d, label: \"line-%04d\", enabled: %t };", index, index, index, index%2 == 0))
	}
	largeContent := strings.Join(lines, "\n")
	toolArgs := fmt.Sprintf("{\"file_path\":\"/tmp/large.txt\",\"content\":\"%s\"}", largeContent)
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tool_large","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item_id":"fc_large","item":{"id":"fc_large","type":"function_call","call_id":"call_large","name":"write_file"}}`,
		"",
		`event: response.function_call_arguments.done`,
		fmt.Sprintf(`data: {"type":"response.function_call_arguments.done","item_id":"fc_large","arguments":%q}`, toolArgs),
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":10,"output_tokens":6}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"call_large"`) {
		t.Fatalf("expected tool_use block, got body length=%d", len(body))
	}
	if !strings.Contains(body, `/tmp/large.txt`) || !strings.Contains(body, `export const value0000`) || !strings.Contains(body, `export const value1999`) {
		t.Fatalf("expected large tool args to survive SSE scanning, got body length=%d", len(body))
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamNormalizesObjectToolArgs(t *testing.T) {
	streamBody := io.NopCloser(strings.NewReader(strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_tool_object","model":"gpt-5.4"}}`,
		"",
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item_id":"fc_obj","item":{"id":"fc_obj","type":"function_call","call_id":"call_obj","name":"Read"}}`,
		"",
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item_id":"fc_obj","item":{"id":"fc_obj","type":"function_call","call_id":"call_obj","name":"Read","arguments":{"file_path":"C:/tmp/log.jsonl","offset":120,"limit":60,"pages":""}}}`,
		"",
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":9,"output_tokens":5}}}`,
		"",
	}, "\n")))

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"partial_json":"{\"file_path\":\"C:/tmp/log.jsonl\",\"limit\":60,\"offset\":120}"`) {
		t.Fatalf("expected object tool args normalized into JSON without empty optional pages, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIResponsesStreamEmitsStopOnScannerError(t *testing.T) {
	streamBody := &failingReadCloser{
		chunks: [][]byte{
			[]byte("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_err\",\"model\":\"gpt-5.4\"}}\n\n"),
		},
		err: errors.New("stream interrupted"),
	}

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIResponsesStream(recorder, streamBody, "gpt-5.4")

	body := recorder.Body.String()
	if !strings.Contains(body, `"type":"message_stop"`) {
		t.Fatalf("expected message_stop on responses scanner error, got %q", body)
	}
	if !strings.Contains(body, "stream interrupted before tool conversion completed") {
		t.Fatalf("expected user-visible stream interruption message, got %q", body)
	}
}

func TestWriteAnthropicSSEFromOpenAIChatStreamDoesNotEmitMessageStopOnScannerError(t *testing.T) {
	streamBody := &failingReadCloser{
		chunks: [][]byte{
			[]byte("data: {\"id\":\"chatcmpl-tool\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_write\",\"function\":{\"name\":\"write_file\"}}]}}]}\n\n"),
		},
		err: errors.New("stream interrupted"),
	}

	recorder := httptest.NewRecorder()
	writeAnthropicSSEFromOpenAIChatStream(recorder, streamBody, "gpt-5.4", false)

	body := recorder.Body.String()
	if strings.Contains(body, `"type":"message_stop"`) {
		t.Fatalf("expected interrupted chat stream to avoid message_stop, got %q", body)
	}
}

func TestPerformRawUpstreamRequestStreamingDoesNotUseWholeBodyTimeout(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.WriteHeader(http.StatusOK)
		flusher, _ := writer.(http.Flusher)
		_, _ = writer.Write([]byte("data: first\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(1500 * time.Millisecond)
		_, _ = writer.Write([]byte("data: second\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	statusCode, _, _, streamBody, _, err := performRawUpstreamRequest(
		http.MethodPost,
		server.URL,
		map[string]string{"Content-Type": "application/json"},
		[]byte(`{}`),
		1,
		true,
	)
	if err != nil {
		t.Fatalf("expected streaming request to succeed, got %v", err)
	}
	if statusCode != http.StatusOK || streamBody == nil {
		t.Fatalf("expected streaming body, got status=%d body=%v", statusCode, streamBody)
	}
	defer streamBody.Close()

	bodyBytes, readErr := io.ReadAll(streamBody)
	if readErr != nil {
		t.Fatalf("expected full streamed body without whole-body timeout, got %v", readErr)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "data: first") || !strings.Contains(body, "data: second") {
		t.Fatalf("expected both streamed chunks, got %q", body)
	}
}

func TestWriteAnthropicSSEPreservesWhitespaceOnlyTextBlocks(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAnthropicSSE(recorder, map[string]any{
		"id":    "msg_test",
		"model": "gpt-5.4",
		"content": []any{
			map[string]any{"type": "text", "text": "line1"},
			map[string]any{"type": "text", "text": "\n"},
			map[string]any{"type": "text", "text": "line2"},
		},
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  1,
			"output_tokens": 2,
		},
	})

	body := recorder.Body.String()
	if strings.Count(body, `"type":"content_block_delta"`) != 3 {
		t.Fatalf("expected 3 content deltas including newline-only block, got %q", body)
	}
}

func TestForwardOpenAIRequestViaProviderPreservesSeparateRoutingSnapshotsPerApp(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	appProviders := map[string]AdvancedProxyProvider{
		"codex": {
			ID:        "codex-provider",
			RowKey:    "row-codex",
			Name:      "Codex Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-test",
			APIFormat: "openai_chat",
		},
		"opencode": {
			ID:        "opencode-provider",
			RowKey:    "row-opencode",
			Name:      "OpenCode Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-test",
			APIFormat: "openai_chat",
		},
		"openclaw": {
			ID:        "openclaw-provider",
			RowKey:    "row-openclaw",
			Name:      "OpenClaw Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-test",
			APIFormat: "openai_chat",
		},
	}

	for appType, provider := range appProviders {
		result := forwardOpenAIRequestViaProvider(appType, provider, "chat", []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`), false, AdvancedProxyConfig{})
		if result.StatusCode != http.StatusOK {
			t.Fatalf("expected %s request to succeed, got %#v", appType, result)
		}
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	for appType, provider := range appProviders {
		state, exists := snapshot.Apps[appType]
		if !exists {
			t.Fatalf("expected routing snapshot for %s, got %#v", appType, snapshot.Apps)
		}
		if state.ProviderID != provider.ID || state.ProviderRowKey != provider.RowKey {
			t.Fatalf("unexpected %s provider binding: %#v", appType, state)
		}
		if state.RouteKind != "chat" || state.Status != "success" {
			t.Fatalf("unexpected %s route state: %#v", appType, state)
		}
		if !strings.HasSuffix(state.TargetURL, "/v1/chat/completions") {
			t.Fatalf("unexpected %s target url: %#v", appType, state)
		}
	}
}

func TestAdvancedProxyRoutingSnapshotSupportsConcurrentApps(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/v1/messages" && request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/v1/messages" {
			_, _ = writer.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	appProviders := map[string]AdvancedProxyProvider{
		"claude": {
			ID:        "claude-provider",
			RowKey:    "row-claude",
			Name:      "Claude Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-claude",
			APIFormat: "anthropic",
		},
		"codex": {
			ID:        "codex-provider",
			RowKey:    "row-codex",
			Name:      "Codex Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-codex",
			APIFormat: "openai_chat",
		},
		"opencode": {
			ID:        "opencode-provider",
			RowKey:    "row-opencode",
			Name:      "OpenCode Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-opencode",
			APIFormat: "openai_chat",
		},
		"openclaw": {
			ID:        "openclaw-provider",
			RowKey:    "row-openclaw",
			Name:      "OpenClaw Provider",
			BaseURL:   server.URL,
			APIKey:    "sk-openclaw",
			APIFormat: "openai_chat",
		},
	}

	var wg sync.WaitGroup
	for appType, provider := range appProviders {
		wg.Add(1)
		go func(appType string, provider AdvancedProxyProvider) {
			defer wg.Done()
			if appType == "claude" {
				claudeResult := forwardClaudeRequestViaProvider(
					provider,
					map[string]any{
						"model": "claude-sonnet",
						"messages": []any{
							map[string]any{"role": "user", "content": "hello"},
						},
					},
					http.Header{},
					false,
					AdvancedProxyConfig{},
				)
				if claudeResult.StatusCode != http.StatusOK || claudeResult.Response == nil {
					t.Errorf("expected %s request to succeed, got %#v", appType, claudeResult)
				}
				return
			}

			openAIResult := forwardOpenAIRequestViaProvider(
				appType,
				provider,
				"chat",
				[]byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`),
				false,
				AdvancedProxyConfig{},
			)
			if openAIResult.StatusCode != http.StatusOK {
				t.Errorf("expected %s request to succeed, got %#v", appType, openAIResult)
			}
		}(appType, provider)
	}
	wg.Wait()

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	for appType, provider := range appProviders {
		state, exists := snapshot.Apps[appType]
		if !exists {
			t.Fatalf("expected routing snapshot for %s, got %#v", appType, snapshot.Apps)
		}
		if state.ProviderID != provider.ID || state.ProviderRowKey != provider.RowKey {
			t.Fatalf("unexpected %s provider binding: %#v", appType, state)
		}
		if state.Status != "success" {
			t.Fatalf("unexpected %s route status: %#v", appType, state)
		}
	}
}

func TestExtractEncryptedContentHealingSessionKeyFromEmbeddedMetadata(t *testing.T) {
	body := map[string]any{
		"metadata": map[string]any{
			"user_id": `{"session_id":"sess-embedded-123"}`,
		},
	}

	got := extractEncryptedContentHealingSessionKey(body, "codex")
	if got != "codex|sess-embedded-123" {
		t.Fatalf("expected embedded session key to be extracted, got %q", got)
	}
}

func TestExtractEncryptedContentHealingSessionKeyFromPromptCacheKey(t *testing.T) {
	body := map[string]any{
		"prompt_cache_key": "pcache-123",
		"client_metadata": map[string]any{
			"x-codex-installation-id": "install-123",
		},
	}

	got := extractEncryptedContentHealingSessionKey(body, "codex")
	if got != "codex|pcache-123" {
		t.Fatalf("expected prompt cache key to be extracted, got %q", got)
	}
}

func TestPrepareOpenAIRequestForEncryptedContentHealingStripsRecordedHistory(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	sessionKey := "codex|sess-prepare-123"
	advancedProxyEncryptedContentHealState.record(sessionKey, 3)

	rawBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-prepare-123\"}"},
		"messages":[
			{"role":"assistant","content":[{"type":"input_text","text":"1","encrypted_content":"enc-1"}]},
			{"role":"user","content":[{"type":"input_text","text":"2","encrypted_content":"enc-2"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"3","encrypted_content":"enc-3"}]},
			{"role":"user","content":[{"type":"input_text","text":"4","encrypted_content":"enc-4"}]}
		]
	}`)

	sanitizedBody, healingContext, err := prepareOpenAIRequestForEncryptedContentHealing(rawBody, "codex")
	if err != nil {
		t.Fatalf("prepare request: %v", err)
	}
	if healingContext.SessionKey != sessionKey {
		t.Fatalf("unexpected session key: %#v", healingContext)
	}
	if healingContext.OriginalCount != 4 {
		t.Fatalf("expected original encrypted count 4, got %#v", healingContext)
	}
	if healingContext.AppliedHistoricalCut != 3 {
		t.Fatalf("expected 3 historical encrypted_content entries to be stripped, got %#v", healingContext)
	}

	var decoded map[string]any
	if err := json.Unmarshal(sanitizedBody, &decoded); err != nil {
		t.Fatalf("decode sanitized body: %v", err)
	}

	if remaining := countEncryptedContentEntries(decoded); remaining != 1 {
		t.Fatalf("expected one encrypted_content entry to remain, got %d", remaining)
	}

	messages, ok := decoded["messages"].([]any)
	if !ok || len(messages) != 4 {
		t.Fatalf("unexpected messages payload: %#v", decoded["messages"])
	}

	for index := 0; index < 3; index++ {
		message, _ := messages[index].(map[string]any)
		content, _ := message["content"].([]any)
		block, _ := content[0].(map[string]any)
		if _, exists := block["encrypted_content"]; exists {
			t.Fatalf("expected historical encrypted_content to be removed at message %d: %#v", index, block)
		}
	}

	lastMessage, _ := messages[3].(map[string]any)
	lastContent, _ := lastMessage["content"].([]any)
	lastBlock, _ := lastContent[0].(map[string]any)
	if got := toStringValue(lastBlock["encrypted_content"]); got != "enc-4" {
		t.Fatalf("expected latest encrypted_content to remain, got %#v", lastBlock)
	}
}

func TestPrepareOpenAIRequestForEncryptedContentHealingStripsIncludeOnlyPayloads(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	sessionKey := "codex|sess-include-only-123"
	advancedProxyEncryptedContentHealState.record(sessionKey, 3)

	rawBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-include-only-123\"}"},
		"include":["reasoning.encrypted_content"],
		"messages":[
			{"role":"assistant","content":[{"type":"input_text","text":"1"}]},
			{"role":"user","content":[{"type":"input_text","text":"2"}]}
		]
	}`)

	sanitizedBody, healingContext, err := prepareOpenAIRequestForEncryptedContentHealing(rawBody, "codex")
	if err != nil {
		t.Fatalf("prepare request: %v", err)
	}
	if healingContext.SessionKey != sessionKey {
		t.Fatalf("unexpected session key: %#v", healingContext)
	}
	if healingContext.OriginalCount != 0 {
		t.Fatalf("expected zero encrypted content fields in include-only payload, got %#v", healingContext)
	}
	if healingContext.RemovedIncludeRefs != 1 {
		t.Fatalf("expected one include reference to be stripped, got %#v", healingContext)
	}

	var decoded map[string]any
	if err := json.Unmarshal(sanitizedBody, &decoded); err != nil {
		t.Fatalf("decode sanitized body: %v", err)
	}

	includeItems, ok := decoded["include"].([]any)
	if !ok {
		t.Fatalf("expected include array to remain decodable, got %#v", decoded["include"])
	}
	if len(includeItems) != 0 {
		t.Fatalf("expected include array to have encrypted references stripped, got %#v", includeItems)
	}
}

func TestFinalizeOpenAIRequestForEncryptedContentHealingRemovesAllResidualHits(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	sessionKey := "codex|sess-finalize-123"
	advancedProxyEncryptedContentHealState.record(sessionKey, 1)

	rawBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-finalize-123\"}"},
		"include":["reasoning.encrypted_content"],
		"input":[
			{"type":"reasoning","encrypted_content":"enc-1"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"please remove encrypted_content mention"}]}
		]
	}`)

	sanitizedBody, stats, err := finalizeOpenAIRequestForEncryptedContentHealing(rawBody, sessionKey)
	if err != nil {
		t.Fatalf("finalize request: %v", err)
	}
	if stats.RemovedFields != 1 || stats.RemovedIncludeRefs != 1 || stats.ScrubbedStrings != 1 {
		t.Fatalf("unexpected final sanitization stats: %#v", stats)
	}
	if stats.ResidualHits != 0 {
		t.Fatalf("expected zero residual hits, got %#v", stats)
	}
	if containsEncryptedContentNeedle(sanitizedBody) {
		t.Fatalf("expected sanitized body to remove encrypted_content entirely: %s", sanitizedBody)
	}

	var decoded map[string]any
	if err := json.Unmarshal(sanitizedBody, &decoded); err != nil {
		t.Fatalf("decode sanitized body: %v", err)
	}
	if count := countEncryptedContentEntries(decoded); count != 0 {
		t.Fatalf("expected zero encrypted_content fields after finalize, got %d", count)
	}
}

func TestForwardOpenAIRequestViaProviderHealsInvalidEncryptedContentAcrossRequests(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	var mu sync.Mutex
	requestBodies := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		mu.Lock()
		requestBodies = append(requestBodies, body)
		callIndex := len(requestBodies)
		mu.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		if callIndex == 1 {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{
				"error": {
					"message": "The encrypted content QVhO...FQ== could not be verified. Reason: Encrypted content could not be decrypted or parsed.",
					"type": "invalid_request_error",
					"param": null,
					"code": "invalid_encrypted_content"
				}
			}`))
			return
		}

		_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "codex-heal-provider",
		RowKey:    "row-codex-heal",
		Name:      "Codex Heal Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	firstBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-heal-123\"}"},
		"messages":[
			{"role":"assistant","content":[{"type":"input_text","text":"1","encrypted_content":"enc-1"}]},
			{"role":"user","content":[{"type":"input_text","text":"2","encrypted_content":"enc-2"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"3","encrypted_content":"enc-3"}]}
		]
	}`)
	firstResult := forwardOpenAIRequestViaProvider("codex", provider, "chat", firstBody, false, AdvancedProxyConfig{})
	if firstResult.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected first call to fail with upstream invalid_encrypted_content, got %#v", firstResult)
	}
	if firstResult.ErrorCode != "invalid_encrypted_content" || firstResult.ErrorType != "invalid_request_error" {
		t.Fatalf("expected upstream error metadata to be preserved, got %#v", firstResult)
	}
	if !strings.Contains(firstResult.Message, encryptedContentHealingNotice) {
		t.Fatalf("expected healing notice to be appended, got %q", firstResult.Message)
	}

	sessionKey := "codex|sess-heal-123"
	if cutoff := advancedProxyEncryptedContentHealState.get(sessionKey); cutoff != 3 {
		t.Fatalf("expected recorded cutoff 3, got %d", cutoff)
	}

	secondBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-heal-123\"}"},
		"messages":[
			{"role":"assistant","content":[{"type":"input_text","text":"1","encrypted_content":"enc-1"}]},
			{"role":"user","content":[{"type":"input_text","text":"2","encrypted_content":"enc-2"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"3","encrypted_content":"enc-3"}]},
			{"role":"user","content":[{"type":"input_text","text":"4","encrypted_content":"enc-4"}]}
		]
	}`)
	secondResult := forwardOpenAIRequestViaProvider("codex", provider, "chat", secondBody, false, AdvancedProxyConfig{})
	if secondResult.StatusCode != http.StatusOK {
		t.Fatalf("expected healed follow-up call to succeed, got %#v", secondResult)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestBodies) != 2 {
		t.Fatalf("expected two upstream requests, got %d", len(requestBodies))
	}
	if count := countEncryptedContentEntries(requestBodies[0]); count != 3 {
		t.Fatalf("expected first upstream body to retain 3 encrypted_content entries, got %d", count)
	}
	if count := countEncryptedContentEntries(requestBodies[1]); count != 0 {
		t.Fatalf("expected second upstream body to remove all encrypted_content entries after heal activation, got %d", count)
	}

	secondRaw, err := json.Marshal(requestBodies[1])
	if err != nil {
		t.Fatalf("marshal second request body: %v", err)
	}
	if containsEncryptedContentNeedle(secondRaw) {
		t.Fatalf("expected healed request to strip encrypted_content completely, got %s", secondRaw)
	}
}

func TestForwardOpenAIRequestViaProviderFinalGateStripsReasoningPayloads(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	var mu sync.Mutex
	requestBodies := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		mu.Lock()
		requestBodies = append(requestBodies, body)
		callIndex := len(requestBodies)
		mu.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		if callIndex == 1 {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{
				"error": {
					"message": "The encrypted content QVhO...FQ== could not be verified. Reason: Encrypted content could not be decrypted or parsed.",
					"type": "invalid_request_error",
					"code": "invalid_encrypted_content"
				}
			}`))
			return
		}

		_, _ = writer.Write([]byte(`{"id":"resp_test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "codex-heal-provider",
		RowKey:    "row-codex-heal",
		Name:      "Codex Heal Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	firstBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-final-gate-123\"}"},
		"input":[
			{"type":"reasoning","encrypted_content":"enc-1"},
			{"type":"reasoning","encrypted_content":"enc-2"}
		]
	}`)
	firstResult := forwardOpenAIRequestViaProvider("codex", provider, "chat", firstBody, false, AdvancedProxyConfig{})
	if firstResult.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected first call to fail with upstream invalid_encrypted_content, got %#v", firstResult)
	}

	secondBody := []byte(`{
		"metadata":{"user_id":"{\"session_id\":\"sess-final-gate-123\"}"},
		"include":["reasoning.encrypted_content"],
		"input":[
			{"type":"reasoning","encrypted_content":"enc-1"},
			{"type":"reasoning","encrypted_content":"enc-2"},
			{"type":"reasoning","encrypted_content":"enc-3"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"encrypted_content should be scrubbed"}]}
		]
	}`)
	secondResult := forwardOpenAIRequestViaProvider("codex", provider, "chat", secondBody, false, AdvancedProxyConfig{})
	if secondResult.StatusCode != http.StatusOK {
		t.Fatalf("expected healed follow-up call to succeed, got %#v", secondResult)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestBodies) != 2 {
		t.Fatalf("expected two upstream requests, got %d", len(requestBodies))
	}

	secondRaw, err := json.Marshal(requestBodies[1])
	if err != nil {
		t.Fatalf("marshal second request body: %v", err)
	}
	if containsEncryptedContentNeedle(secondRaw) {
		t.Fatalf("expected final gate to strip encrypted_content from second upstream body: %s", secondRaw)
	}
	if count := countEncryptedContentEntries(requestBodies[1]); count != 0 {
		t.Fatalf("expected second upstream body to retain zero encrypted_content fields, got %d", count)
	}
}

func TestForwardOpenAIRequestViaProviderHealsPromptCacheSessions(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	var mu sync.Mutex
	requestBodies := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		mu.Lock()
		requestBodies = append(requestBodies, body)
		callIndex := len(requestBodies)
		mu.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		if callIndex == 1 {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{
				"error": {
					"message": "The encrypted content QVhO...FQ== could not be verified. Reason: Encrypted content could not be decrypted or parsed.",
					"type": "invalid_request_error",
					"code": "invalid_encrypted_content"
				}
			}`))
			return
		}

		_, _ = writer.Write([]byte(`{"id":"resp_test","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer server.Close()

	provider := AdvancedProxyProvider{
		ID:        "codex-heal-provider",
		RowKey:    "row-codex-heal",
		Name:      "Codex Heal Provider",
		BaseURL:   server.URL,
		APIKey:    "sk-test",
		APIFormat: "openai_chat",
	}

	firstBody := []byte(`{
		"prompt_cache_key":"pcache-heal-123",
		"client_metadata":{"x-codex-installation-id":"install-123"},
		"input":[
			{"type":"reasoning","encrypted_content":"enc-1"},
			{"type":"reasoning","encrypted_content":"enc-2"}
		]
	}`)
	firstResult := forwardOpenAIRequestViaProvider("codex", provider, "responses", firstBody, false, AdvancedProxyConfig{})
	if firstResult.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected first call to fail with upstream invalid_encrypted_content, got %#v", firstResult)
	}
	if cutoff := advancedProxyEncryptedContentHealState.get("codex|pcache-heal-123"); cutoff != 2 {
		t.Fatalf("expected prompt cache session to be recorded with cutoff 2, got %d", cutoff)
	}

	secondBody := []byte(`{
		"prompt_cache_key":"pcache-heal-123",
		"client_metadata":{"x-codex-installation-id":"install-123"},
		"include":["reasoning.encrypted_content"],
		"input":[
			{"type":"reasoning","encrypted_content":"enc-1"},
			{"type":"reasoning","encrypted_content":"enc-2"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"encrypted_content should not survive"}]}
		]
	}`)
	secondResult := forwardOpenAIRequestViaProvider("codex", provider, "responses", secondBody, false, AdvancedProxyConfig{})
	if secondResult.StatusCode != http.StatusOK {
		t.Fatalf("expected healed prompt-cache follow-up call to succeed, got %#v", secondResult)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestBodies) != 2 {
		t.Fatalf("expected two upstream requests, got %d", len(requestBodies))
	}
	secondRaw, err := json.Marshal(requestBodies[1])
	if err != nil {
		t.Fatalf("marshal second request body: %v", err)
	}
	if containsEncryptedContentNeedle(secondRaw) {
		t.Fatalf("expected prompt-cache healed request to strip encrypted_content completely, got %s", secondRaw)
	}
}

func TestAdvancedProxyRoutingSnapshotTracksConcurrentProvidersForOneAppType(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		started <- struct{}{}
		<-release
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	providers := []AdvancedProxyProvider{
		{
			ID:        "codex-provider-a",
			RowKey:    "row-codex-a",
			Name:      "Codex Provider A",
			BaseURL:   server.URL,
			APIKey:    "sk-codex-a",
			APIFormat: "openai_chat",
		},
		{
			ID:        "codex-provider-b",
			RowKey:    "row-codex-b",
			Name:      "Codex Provider B",
			BaseURL:   server.URL,
			APIKey:    "sk-codex-b",
			APIFormat: "openai_chat",
		},
	}

	var wg sync.WaitGroup
	for _, provider := range providers {
		wg.Add(1)
		go func(provider AdvancedProxyProvider) {
			defer wg.Done()
			result := forwardOpenAIRequestViaProvider(
				"codex",
				provider,
				"chat",
				[]byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`),
				false,
				AdvancedProxyConfig{},
			)
			if result.StatusCode != http.StatusOK {
				t.Errorf("expected request to succeed, got %#v", result)
			}
		}(provider)
	}

	for i := 0; i < len(providers); i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for provider requests to start")
		}
	}

	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	for _, provider := range providers {
		var state *AdvancedProxyProviderRoutingState
		for _, entry := range snapshot.Providers {
			if entry.ProviderRowKey == provider.RowKey {
				copy := entry
				state = &copy
				break
			}
		}
		if state == nil {
			t.Fatalf("expected provider routing snapshot for %s, got %#v", provider.RowKey, snapshot.Providers)
		}
		if state.ActiveCount != 1 || state.Status != "dispatching" {
			t.Fatalf("unexpected active provider state: %#v", state)
		}
		if len(state.AppTypes) != 1 || state.AppTypes[0] != "codex" {
			t.Fatalf("unexpected active app types for provider %s: %#v", provider.RowKey, state.AppTypes)
		}
	}

	close(release)
	wg.Wait()
}

func TestOrderProvidersByHealthPrefersLowerFailureRate(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	healthy := AdvancedProxyProvider{
		ID:        "provider-healthy",
		RowKey:    "row-healthy",
		Name:      "Healthy Provider",
		BaseURL:   "http://127.0.0.1:1",
		APIFormat: "openai_chat",
	}
	unhealthy := AdvancedProxyProvider{
		ID:        "provider-unhealthy",
		RowKey:    "row-unhealthy",
		Name:      "Unhealthy Provider",
		BaseURL:   "http://127.0.0.1:2",
		APIFormat: "openai_chat",
	}

	for i := 0; i < 4; i++ {
		advancedProxyRuntime.ObserveProviderOutcome("codex", healthy, http.StatusOK, 100*time.Millisecond, true, false)
	}
	for i := 0; i < 4; i++ {
		advancedProxyRuntime.ObserveProviderOutcome("codex", unhealthy, http.StatusBadGateway, 1500*time.Millisecond, false, false)
	}

	ordered := advancedProxyRuntime.OrderProvidersByHealth(
		AdvancedProxyConfig{
			HighAvailability: HighAvailabilityConfig{
				DynamicOptimizeQueue: true,
			},
		},
		"codex",
		[]AdvancedProxyProvider{unhealthy, healthy},
	)

	if len(ordered) != 2 {
		t.Fatalf("expected two providers, got %#v", ordered)
	}
	if ordered[0].ID != healthy.ID {
		t.Fatalf("expected healthy provider first, got %#v", ordered)
	}
	if ordered[1].ID != unhealthy.ID {
		t.Fatalf("expected unhealthy provider second, got %#v", ordered)
	}
}

func TestOrderProvidersForDispatchAccountsForActiveLoadAndRpm(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	providerA := AdvancedProxyProvider{
		ID:        "provider-a",
		RowKey:    "row-a",
		Name:      "Provider A",
		BaseURL:   "http://127.0.0.1:3",
		APIFormat: "openai_chat",
	}
	providerB := AdvancedProxyProvider{
		ID:        "provider-b",
		RowKey:    "row-b",
		Name:      "Provider B",
		BaseURL:   "http://127.0.0.1:4",
		APIFormat: "openai_chat",
	}

	advancedProxyRuntime.mu.Lock()
	advancedProxyRuntime.providerRoutes[providerRoutingKey(providerA)] = &proxyProviderRouteState{
		ProviderID:     providerA.ID,
		ProviderRowKey: providerA.RowKey,
		ProviderName:   providerA.Name,
		activeRoutes:   map[string]int{"codex|chat|http://127.0.0.1:3": 3},
		activeAppTypes: map[string]int{"codex": 3},
		activeCount:    3,
		Status:         "dispatching",
	}
	advancedProxyRuntime.mu.Unlock()

	ordered := advancedProxyRuntime.OrderProvidersForDispatch(
		AdvancedProxyConfig{
			HighAvailability: HighAvailabilityConfig{
				Enabled:      true,
				DispatchMode: "ordered",
				RPM: HighAvailabilityRPMConfig{
					Global: 0,
					Providers: map[string]*int{
						"codex": intPtr(2),
					},
				},
			},
		},
		"codex",
		[]AdvancedProxyProvider{providerA, providerB},
	)

	if len(ordered) != 2 {
		t.Fatalf("expected two providers, got %#v", ordered)
	}
	if ordered[0].ID != providerB.ID {
		t.Fatalf("expected idle provider first, got %#v", ordered)
	}
	if ordered[1].ID != providerA.ID {
		t.Fatalf("expected loaded provider second, got %#v", ordered)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestAdvancedProxyResetClearsProviderHealthForProvider(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	provider := AdvancedProxyProvider{
		ID:        "provider-reset",
		RowKey:    "row-reset",
		Name:      "Reset Provider",
		BaseURL:   "http://127.0.0.1:5",
		APIFormat: "openai_chat",
	}

	advancedProxyRuntime.ObserveProviderOutcome("codex", provider, http.StatusOK, 50*time.Millisecond, true, false)

	advancedProxyRuntime.mu.Lock()
	healthCount := 0
	for _, state := range advancedProxyRuntime.providerHealth {
		if state != nil && state.AppType == "codex" && state.ProviderID == provider.ID {
			healthCount++
		}
	}
	advancedProxyRuntime.mu.Unlock()
	if healthCount == 0 {
		t.Fatalf("expected provider health to be recorded")
	}

	advancedProxyRuntime.Reset("codex", provider.ID)

	advancedProxyRuntime.mu.Lock()
	defer advancedProxyRuntime.mu.Unlock()
	for _, state := range advancedProxyRuntime.providerHealth {
		if state != nil && state.AppType == "codex" && state.ProviderID == provider.ID {
			t.Fatalf("expected provider health to be cleared, got %#v", state)
		}
	}
}

func TestRPMDispatchHistoryTracksRecentDispatches(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	provider := AdvancedProxyProvider{
		ID:        "provider-rpm",
		RowKey:    "row-rpm",
		Name:      "RPM Provider",
		BaseURL:   "http://127.0.0.1:6",
		APIFormat: "openai_chat",
	}

	advancedProxyRuntime.recordRPMDispatch(providerRPMKey(provider), time.Now().Add(-70*time.Second))
	advancedProxyRuntime.recordRPMDispatch(providerRPMKey(provider), time.Now())

	advancedProxyRuntime.mu.Lock()
	recentCount := advancedProxyRuntime.currentRPMDispatchCountLocked(providerRPMKey(provider), time.Now())
	advancedProxyRuntime.mu.Unlock()
	if recentCount != 1 {
		t.Fatalf("expected one recent RPM dispatch, got %d", recentCount)
	}

	advancedProxyRuntime.MarkDispatch("codex", provider, "chat", provider.BaseURL)

	advancedProxyRuntime.mu.Lock()
	defer advancedProxyRuntime.mu.Unlock()
	recentCount = advancedProxyRuntime.currentRPMDispatchCountLocked(providerRPMKey(provider), time.Now())
	if recentCount != 2 {
		t.Fatalf("expected two recent RPM dispatches after mark dispatch, got %d", recentCount)
	}
}

func TestAdvancedProxyRPMIgnoresLegacyAppScopes(t *testing.T) {
	config := defaultAdvancedProxyConfig()
	config.HighAvailability.RPM = HighAvailabilityRPMConfig{
		Global: 3,
		Providers: map[string]*int{
			"row-rpm": intPtr(9),
			"codex":   intPtr(7),
		},
	}

	normalized := normalizeAdvancedProxyHighAvailabilityRPMConfig(config.HighAvailability.RPM)
	if _, exists := normalized.Providers["codex"]; exists {
		t.Fatalf("expected legacy app scope to be stripped, got %#v", normalized.Providers)
	}

	provider := AdvancedProxyProvider{
		ID:      "provider-rpm",
		RowKey:  "row-rpm",
		BaseURL: "http://127.0.0.1:6",
	}

	if got := resolveAdvancedProxyHighAvailabilityRPM(config, provider, "codex"); got != 9 {
		t.Fatalf("expected provider-specific RPM to win, got %d", got)
	}

	otherProvider := AdvancedProxyProvider{
		ID:      "provider-other",
		BaseURL: "http://127.0.0.1:7",
	}
	if got := resolveAdvancedProxyHighAvailabilityRPM(config, otherProvider, "codex"); got != 3 {
		t.Fatalf("expected global RPM fallback, got %d", got)
	}
}
