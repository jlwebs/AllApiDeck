package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestExecuteCheckKeySmartFallsBackToResponsesAndPersistsHostPreference(t *testing.T) {
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
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"model":"gpt-5.5",
				"output":[{"type":"message","content":[{"type":"output_text","text":"ok from responses"}]}],
				"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}
			}`))
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
	if got := getCheckProtocolPreference(server.URL); got != checkProtocolPreferResponses {
		t.Fatalf("expected responses preference to persist, got %d", got)
	}

	requests = requests[:0]
	status, body = executeCheckKeySmart(normalizedCheckKeyPayload{
		URL:       server.URL,
		Key:       "sk-test",
		Model:     "gpt-5.5",
		TimeoutMs: 20000,
		Messages:  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if status != http.StatusOK {
		raw, _ := json.Marshal(body)
		t.Fatalf("expected preferred responses path to succeed, got status=%d body=%s", status, raw)
	}
	if len(requests) == 0 || requests[0] != "/v1/responses" {
		t.Fatalf("expected persisted preference to try responses first, got %#v", requests)
	}
}
