package main

import (
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

func TestForwardClaudeRequestViaProviderUpdatesRoutingSnapshotForOpenAIChatStream(t *testing.T) {
	resetAdvancedProxyRuntimeForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"id\":\"chatcmpl-test\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
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
	if state.RouteKind != "chat" || state.Status != "success" {
		t.Fatalf("unexpected claude streaming route state: %#v", state)
	}
	if state.TargetURL != server.URL+"/v1/chat/completions" {
		t.Fatalf("unexpected target url: %#v", state)
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
