package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseMappedCheckHeadersSupportsSemicolonDelimitedHeaders(t *testing.T) {
	got := parseMappedCheckHeaders("User-Agent: claude-cli/2.1.129 (external, cli); x-app: cli")
	want := map[string]string{
		"User-Agent": "claude-cli/2.1.129 (external, cli)",
		"X-App":      "cli",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected headers: got=%#v want=%#v", got, want)
	}
}

func TestBuildCheckRequestHeadersAppliesMappedHeaders(t *testing.T) {
	headers, match := buildCheckRequestHeaders(normalizedCheckKeyPayload{
		Key:   "sk-test",
		Model: "gpt-4o-mini",
		UserAgentMappings: []checkUserAgentMapping{
			{
				ModelContains: "gpt",
				TargetUA:      "originator: Codex Desktop\nuser-agent: Codex Desktop/0.142.0-alpha.6 (Windows 10.0.19044; x86_64) unknown (Codex Desktop; 26.616.51431)",
			},
		},
	})
	if match != "gpt" {
		t.Fatalf("unexpected mapping match: %q", match)
	}
	if headers["Originator"] != "Codex Desktop" {
		t.Fatalf("expected originator header, got %#v", headers)
	}
	if headers["User-Agent"] != "Codex Desktop/0.142.0-alpha.6 (Windows 10.0.19044; x86_64) unknown (Codex Desktop; 26.616.51431)" {
		t.Fatalf("unexpected user-agent header: %#v", headers)
	}
	if headers["Authorization"] != "Bearer sk-test" {
		t.Fatalf("expected authorization header, got %#v", headers)
	}
}

func TestBuildCheckRequestHeadersAppliesExpandedClaudeMappedHeaders(t *testing.T) {
	headers, match := buildCheckRequestHeaders(normalizedCheckKeyPayload{
		Key:   "sk-test",
		Model: "claude-3-7-sonnet",
		UserAgentMappings: []checkUserAgentMapping{
			{
				ModelContains: "claude",
				TargetUA: strings.Join([]string{
					"User-Agent: claude-cli/2.1.129 (external, cli)",
					"x-app: cli",
					"anthropic-version: 2023-06-01",
					"anthropic-beta: claude-code-20250219,interleaved-thinking-2025-05-14",
					"anthropic-dangerous-direct-browser-access: true",
					"X-Stainless-Arch: x64",
					"X-Stainless-Lang: js",
					"X-Stainless-OS: Windows",
					"X-Stainless-Package-Version: 0.93.0",
					"X-Stainless-Retry-Count: 0",
					"X-Stainless-Runtime: node",
					"X-Stainless-Runtime-Version: v24.3.0",
					"X-Stainless-Timeout: 600",
				}, "\n"),
			},
		},
	})
	if match != "claude" {
		t.Fatalf("unexpected mapping match: %q", match)
	}
	for key, want := range map[string]string{
		"User-Agent":                                 "claude-cli/2.1.129 (external, cli)",
		"X-App":                                      "cli",
		"Anthropic-Version":                          "2023-06-01",
		"Anthropic-Beta":                             "claude-code-20250219,interleaved-thinking-2025-05-14",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-Stainless-Arch":                           "x64",
		"X-Stainless-Lang":                           "js",
		"X-Stainless-Os":                             "Windows",
		"X-Stainless-Package-Version":                "0.93.0",
		"X-Stainless-Retry-Count":                    "0",
		"X-Stainless-Runtime":                        "node",
		"X-Stainless-Runtime-Version":                "v24.3.0",
		"X-Stainless-Timeout":                        "600",
	} {
		if headers[key] != want {
			t.Fatalf("expected header %s=%q, got %#v", key, want, headers)
		}
	}
}

func TestBuildCheckEndpointCandidatesAnyrouterOrder(t *testing.T) {
	t.Setenv("BATCH_API_CHECK_RUNTIME_DIR", t.TempDir())
	resetCheckProtocolPreferencesForTests()
	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	payload := normalizedCheckKeyPayload{
		URL:      "https://anyrouter.top",
		SiteType: "anyrouter",
	}

	got := buildCheckEndpointCandidates(payload)
	if len(got) < 3 {
		t.Fatalf("unexpected candidate count: %#v", got)
	}
	if got[0] != "https://anyrouter.top/v1/chat/completions" {
		t.Fatalf("expected chat candidate first, got %#v", got)
	}
	if got[len(got)-1] != "https://anyrouter.top/v1/messages" {
		t.Fatalf("expected anthropic candidate last, got %#v", got)
	}
	hasResponses := false
	for _, candidate := range got {
		if candidate == "https://anyrouter.top/v1/responses" {
			hasResponses = true
			break
		}
	}
	if !hasResponses {
		t.Fatalf("expected responses candidate in %#v", got)
	}
}

func TestBuildCheckEndpointPhasesClaudeFallbackOrder(t *testing.T) {
	t.Setenv("BATCH_API_CHECK_RUNTIME_DIR", t.TempDir())
	resetCheckProtocolPreferencesForTests()
	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	phases := buildCheckEndpointPhases(normalizedCheckKeyPayload{
		URL:   "https://example.com",
		Key:   "sk-test",
		Model: "Claude-3.7-Sonnet",
	})
	if len(phases) != 3 {
		t.Fatalf("expected three phases for claude fallback, got %#v", phases)
	}
	if phases[0].protocol != "chat" || phases[1].protocol != "messages" || phases[2].protocol != "responses" {
		t.Fatalf("expected chat -> messages -> responses order, got %#v", phases)
	}
}

func TestExtractResponsesOutputText(t *testing.T) {
	payload := map[string]any{
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": "Hello from responses",
					},
				},
			},
		},
	}

	got := extractResponsesOutputText(payload)
	if got != "Hello from responses" {
		t.Fatalf("unexpected text: got=%q", got)
	}
}

func TestBuildAnyrouterClaudeUpgradeHint(t *testing.T) {
	got := buildAnyrouterClaudeUpgradeHint("claude-opus-4-7")
	if !strings.Contains(got, "Opus 4.7 1m") {
		t.Fatalf("expected upgrade hint to mention Opus 4.7 1m, got %q", got)
	}
}

func TestExtractCheckErrorMessageUsesTopLevelErrorString(t *testing.T) {
	payload := map[string]any{
		"error": "1m 上下文已经全量可用，请启用 1m 上下文后重试",
		"type":  "error",
	}

	got := extractCheckErrorMessage(payload, "HTTP 400")
	if got != "1m 上下文已经全量可用，请启用 1m 上下文后重试" {
		t.Fatalf("unexpected error message: %q", got)
	}
}

func TestExecuteCheckKeySmartFallsBackToResponsesAndPersistsScopedPreference(t *testing.T) {
	t.Setenv("BATCH_API_CHECK_RUNTIME_DIR", t.TempDir())
	resetCheckProtocolPreferencesForTests()
	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/v1/chat/completions":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"当前 API 不支持所选模型 gpt-5.5"}}`))
		case "/chat/completions":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`<html><head><title>403 Forbidden</title></head><body>forbidden</body></html>`))
		case "/v1/responses":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode responses request: %v", err)
			}
			if body["stream"] != true {
				t.Fatalf("expected responses check to request stream=true, got %#v", body["stream"])
			}
			writer.Header().Set("Content-Type", "text/event-stream")
			_, _ = writer.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok from responses\"}\n\n"))
			_, _ = writer.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-5.5\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n"))
			_, _ = writer.Write([]byte("data: [DONE]\n\n"))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	status, body := executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "gpt-5.5",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected fallback to responses to succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) != 3 {
		t.Fatalf("expected three attempts including responses fallback, got %#v", requests)
	}
	if requests[2] != "/v1/responses" {
		t.Fatalf("expected third attempt to be /v1/responses, got %#v", requests)
	}
	basePayload := normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "gpt-5.5",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	}
	if got := getCheckProtocolPreference(basePayload); got != checkProtocolPreferResponses {
		t.Fatalf("expected responses preference to persist, got %d", got)
	}

	requests = requests[:0]
	status, body = executeCheckKeySmart(basePayload)
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected preferred responses path to succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) == 0 || requests[0] != "/v1/responses" {
		t.Fatalf("expected persisted preference to try responses first, got %#v", requests)
	}

	requests = requests[:0]
	status, body = executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "gpt-5.5-mini",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected different-model fallback to still succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) == 0 || requests[0] != "/v1/chat/completions" {
		t.Fatalf("expected different model to start from chat, got %#v", requests)
	}

	requests = requests[:0]
	status, body = executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-other",
		Model:     "gpt-5.5",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected different-key fallback to still succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) == 0 || requests[0] != "/v1/chat/completions" {
		t.Fatalf("expected different key to start from chat, got %#v", requests)
	}
}

func TestExecuteCheckKeySmartClaudeFallsBackToMessagesBeforeResponses(t *testing.T) {
	t.Setenv("BATCH_API_CHECK_RUNTIME_DIR", t.TempDir())
	resetCheckProtocolPreferencesForTests()
	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/v1/chat/completions":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"model not found"}}`))
		case "/chat/completions":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`<html><head><title>403 Forbidden</title></head><body>forbidden</body></html>`))
		case "/v1/messages":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"id":"msg_test",
				"type":"message",
				"role":"assistant",
				"model":"claude-3-7-sonnet",
				"content":[{"type":"text","text":"ok from messages"}],
				"usage":{"input_tokens":3,"output_tokens":2}
			}`))
		case "/v1/responses":
			t.Fatalf("responses should not be tried after messages success")
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	status, body := executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "claude-3-7-sonnet",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected messages fallback to succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) != 3 {
		t.Fatalf("expected v1 chat, chat fallback, then messages, got %#v", requests)
	}
	if requests[0] != "/v1/chat/completions" || requests[1] != "/chat/completions" || requests[2] != "/v1/messages" {
		t.Fatalf("expected chat candidates before messages, got %#v", requests)
	}
}

func TestExecuteCheckKeySmartClaudeFallsBackToResponsesAfterMessagesFailure(t *testing.T) {
	t.Setenv("BATCH_API_CHECK_RUNTIME_DIR", t.TempDir())
	resetCheckProtocolPreferencesForTests()
	if _, err := saveOutboundProxyConfig(OutboundProxyConfig{Mode: outboundProxyModeDirect}); err != nil {
		t.Fatalf("save outbound proxy config: %v", err)
	}

	requests := make([]string, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		switch request.URL.Path {
		case "/v1/chat/completions":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"model not found"}}`))
		case "/chat/completions":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`<html><head><title>403 Forbidden</title></head><body>forbidden</body></html>`))
		case "/v1/messages":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"message":"messages route not supported"}}`))
		case "/messages":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = writer.Write([]byte(`<html><head><title>403 Forbidden</title></head><body>forbidden</body></html>`))
		case "/v1/responses":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode responses request: %v", err)
			}
			if body["stream"] != true {
				t.Fatalf("expected responses check to request stream=true, got %#v", body["stream"])
			}
			writer.Header().Set("Content-Type", "text/event-stream")
			_, _ = writer.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"model\":\"claude-3-7-sonnet\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok from responses\"}]}],\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n"))
			_, _ = writer.Write([]byte("data: [DONE]\n\n"))
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	status, body := executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "claude-3-7-sonnet",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected responses fallback after messages failure, got status=%d body=%s", status, raw)
	}
	if len(requests) != 5 {
		t.Fatalf("expected chat candidates -> messages candidates -> responses, got %#v", requests)
	}
	if requests[0] != "/v1/chat/completions" || requests[1] != "/chat/completions" || requests[2] != "/v1/messages" || requests[3] != "/messages" || requests[4] != "/v1/responses" {
		t.Fatalf("unexpected request order %#v", requests)
	}
}
