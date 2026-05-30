package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func resetAdvancedProxyRequestRecordsForTest(t *testing.T) {
	t.Helper()
	advancedProxyRequestRecords.clear()
	t.Cleanup(func() {
		advancedProxyRequestRecords.clear()
	})
}

func TestRecordAdvancedProxyClaudeAttemptUsesResolvedModel(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	provider := AdvancedProxyProvider{
		ID:      "claude-provider",
		RowKey:  "row-claude-provider",
		Name:    "Claude Provider",
		BaseURL: "https://example.com/v1",
		APIKey:  "sk-claude-provider",
		Model:   "claude-opus-4-7",
	}

	recordAdvancedProxyClaudeAttemptWithTrace(
		"claude",
		buildAdvancedProxyClaudeInboundEndpoint(),
		"messages",
		provider,
		"https://example.com/v1/messages",
		[]byte(`{"model":"gpt-5.5","messages":[]}`),
		"claude-opus-4-7",
		map[string]any{
			"usage": map[string]any{
				"input_tokens":  12,
				"output_tokens": 4,
			},
		},
		[]byte(`{"id":"msg_test"}`),
		false,
		http.StatusOK,
		25*time.Millisecond,
		"",
		nil,
	)

	records := advancedProxyRequestRecords.list(10)
	if len(records) != 1 {
		t.Fatalf("expected one request record, got %#v", records)
	}
	if records[0].Model != "claude-opus-4-7" {
		t.Fatalf("expected record model to use resolved upstream model, got %#v", records[0])
	}
}

func TestRecordAdvancedProxyClaudeAttemptCapturesLatestObservedToolUse(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	provider := AdvancedProxyProvider{
		ID:      "claude-provider",
		RowKey:  "row-claude-provider",
		Name:    "Claude Provider",
		BaseURL: "https://example.com/v1",
		APIKey:  "sk-claude-provider",
		Model:   "claude-opus-4-7",
	}

	rawResponse := []byte(`{
		"id":"msg_test",
		"type":"message",
		"role":"assistant",
		"content":[
			{"type":"thinking","thinking":"need web search"},
			{"type":"tool_use","id":"toolu_123","name":"WebSearch","input":{"query":"today top news"}}
		]
	}`)

	recordAdvancedProxyClaudeAttemptWithTrace(
		"claude",
		buildAdvancedProxyClaudeInboundEndpoint(),
		"messages",
		provider,
		"https://example.com/v1/messages",
		[]byte(`{"model":"claude-opus-4-7","messages":[]}`),
		"claude-opus-4-7",
		nil,
		rawResponse,
		false,
		http.StatusBadGateway,
		25*time.Millisecond,
		"missing_guard_toolcall",
		nil,
	)

	records := advancedProxyRequestRecords.list(10)
	if len(records) != 1 {
		t.Fatalf("expected one request record, got %#v", records)
	}
	if records[0].UpstreamLatestObserved == nil {
		t.Fatalf("expected latest observed block captured, got %#v", records[0])
	}
	if records[0].UpstreamLatestObserved.Type != "tool_use" {
		t.Fatalf("expected latest observed type tool_use, got %#v", records[0].UpstreamLatestObserved)
	}
	if records[0].UpstreamLatestObserved.Name != "WebSearch" {
		t.Fatalf("expected latest observed tool name WebSearch, got %#v", records[0].UpstreamLatestObserved)
	}
	if !strings.Contains(records[0].UpstreamLatestObserved.ArgumentsPreview, "today top news") {
		t.Fatalf("expected latest observed tool args captured, got %#v", records[0].UpstreamLatestObserved)
	}
	if len(records[0].UpstreamToolCalls) != 1 || records[0].UpstreamToolCalls[0] != "WebSearch" {
		t.Fatalf("expected upstream tool list captured, got %#v", records[0])
	}
}

func TestRecordAdvancedProxyOpenAIAttemptCapturesLatestObservedFunctionCall(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	provider := AdvancedProxyProvider{
		ID:      "provider-test",
		RowKey:  "row-provider-test",
		Name:    "Provider Test",
		BaseURL: "https://example.com/v1",
		APIKey:  "sk-provider-test",
	}

	responseBody := []byte(`{
		"id":"resp_test",
		"output":[
			{"type":"message","content":[{"type":"output_text","text":"searching<aad_guard_json>{\"name\":\"aad_guard_xxx_WebSearch\",\"tool_name\":\"WebSearch\",\"tool_type\":\"network\",\"algorithm\":\"APTX\",\"nonce\":\"abc\",\"digest\":\"deadbeef\"}</aad_guard_json>"}]},
			{"type":"function_call","call_id":"call_123","name":"WebSearch","arguments":"{\"query\":\"today top news\"}"}
		]
	}`)

	recordAdvancedProxyOpenAIAttemptWithTraceAndOps(
		"codex",
		"responses",
		"/advanced-proxy/codex/v1/responses",
		"responses",
		"direct",
		provider,
		"https://example.com/v1/responses",
		[]byte(`{"model":"gpt-test"}`),
		"gpt-test",
		responseBody,
		false,
		http.StatusBadGateway,
		25*time.Millisecond,
		"guard_digest_mismatch",
		nil,
		nil,
	)

	records := advancedProxyRequestRecords.list(10)
	if len(records) != 1 {
		t.Fatalf("expected one request record, got %#v", records)
	}
	if records[0].UpstreamLatestObserved == nil {
		t.Fatalf("expected latest observed function call captured, got %#v", records[0])
	}
	if records[0].UpstreamLatestObserved.Type != "function_call" {
		t.Fatalf("expected latest observed type function_call, got %#v", records[0].UpstreamLatestObserved)
	}
	if records[0].UpstreamLatestObserved.Name != "WebSearch" {
		t.Fatalf("expected latest observed real tool name captured, got %#v", records[0].UpstreamLatestObserved)
	}
	if !strings.Contains(records[0].UpstreamLatestObserved.ArgumentsPreview, "today top news") {
		t.Fatalf("expected latest observed real tool args captured, got %#v", records[0].UpstreamLatestObserved)
	}
	if records[0].UpstreamResponseRaw == "" || !strings.Contains(records[0].UpstreamResponseRaw, "<aad_guard_json>") {
		t.Fatalf("expected upstream raw response retained in full, got %#v", records[0])
	}
}

func TestRecordAdvancedProxyStreamObservationUsesResolvedModel(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	provider := AdvancedProxyProvider{
		ID:      "stream-model-provider",
		RowKey:  "row-stream-model-provider",
		Name:    "Stream Model Provider",
		BaseURL: "https://example.com/v1",
		APIKey:  "sk-stream-model-provider",
		Model:   "claude-sonnet-4-7",
	}
	startedAt := time.Now().Add(-120 * time.Millisecond)
	firstOutputAt := startedAt.Add(35 * time.Millisecond)
	completedAt := startedAt.Add(90 * time.Millisecond)

	recordAdvancedProxyStreamObservation(
		&advancedProxyStreamRequestRecordContext{
			AppType:                  "claude",
			ClientRoute:              "messages",
			InboundEndpoint:          buildAdvancedProxyClaudeInboundEndpoint(),
			OutboundRoute:            "responses",
			Source:                   "fallback",
			Provider:                 provider,
			TargetURL:                "https://example.com/v1/responses",
			RequestBody:              []byte(`{"model":"gpt-5.5","stream":true}`),
			ResolvedModel:            "claude-sonnet-4-7",
			StartedAt:                startedAt,
			ObservedFormat:           "openai_responses",
			UpstreamResponsePreview:  "data: upstream-preview",
			DeliveredResponsePreview: "stop_reason=end_turn | tool_use=false | text=hello",
			UpstreamToolCalls:        []string{"WebSearch"},
			UpstreamToolArgsPreview:  []string{`{"query":"latest openai docs"}`},
			UpstreamAssistantPreview: "I need to search first.",
		},
		advancedProxyStreamObservation{
			StartedAt:     startedAt,
			FirstOutputAt: &firstOutputAt,
			CompletedAt:   completedAt,
			InputTokens:   intPtrValue(18),
			OutputTokens:  intPtrValue(6),
		},
		http.StatusOK,
		"",
	)

	records := advancedProxyRequestRecords.list(10)
	if len(records) != 1 {
		t.Fatalf("expected one request record, got %#v", records)
	}
	if records[0].Model != "claude-sonnet-4-7" {
		t.Fatalf("expected stream record model to use resolved upstream model, got %#v", records[0])
	}
	if records[0].UpstreamResponsePreview != "data: upstream-preview" {
		t.Fatalf("expected upstream preview captured, got %#v", records[0])
	}
	if records[0].ResponsePreview != "stop_reason=end_turn | tool_use=false | text=hello" {
		t.Fatalf("expected delivered response preview captured, got %#v", records[0])
	}
	if len(records[0].UpstreamToolCalls) != 1 || records[0].UpstreamToolCalls[0] != "WebSearch" {
		t.Fatalf("expected upstream tool names captured, got %#v", records[0])
	}
	if len(records[0].UpstreamToolArgsPreview) != 1 || !strings.Contains(records[0].UpstreamToolArgsPreview[0], "latest openai docs") {
		t.Fatalf("expected upstream tool args captured, got %#v", records[0])
	}
	if records[0].UpstreamAssistantPreview != "I need to search first." {
		t.Fatalf("expected upstream assistant preview captured, got %#v", records[0])
	}
}

func TestAdvancedProxyRequestRecordsKeepOnlyLastFiftyRequestBodies(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	for index := 0; index < advancedProxyRequestPayloadLimit+5; index++ {
		appendAdvancedProxyRequestRecord(AdvancedProxyRequestRecord{
			RecordedAt:   time.Now().Format(time.RFC3339Nano),
			AppType:      "codex",
			ProviderID:   "provider-test",
			ProviderName: "Provider Test",
			RequestBody:  `{"index":` + string(rune('0'+(index%10))) + `}`,
		})
	}

	records := advancedProxyRequestRecords.list(advancedProxyRequestRecordLimit)
	if len(records) != advancedProxyRequestPayloadLimit+5 {
		t.Fatalf("expected %d records, got %d", advancedProxyRequestPayloadLimit+5, len(records))
	}

	payloadCount := 0
	for _, record := range records {
		if record.RequestBody != "" {
			payloadCount++
		}
	}
	if payloadCount != advancedProxyRequestPayloadLimit {
		t.Fatalf("expected only last %d request bodies to remain, got %d", advancedProxyRequestPayloadLimit, payloadCount)
	}
}

func TestAdvancedProxyRecordExtractsAntiPoisonPromptPreview(t *testing.T) {
	resetAdvancedProxyRequestRecordsForTest(t)

	recordAdvancedProxyOpenAIAttemptWithTraceAndOps(
		"codex",
		"responses",
		"/advanced-proxy/codex/v1/responses",
		"responses",
		"direct",
		AdvancedProxyProvider{
			ID:      "provider-test",
			RowKey:  "row-provider-test",
			Name:    "Provider Test",
			BaseURL: "https://example.com/v1",
			APIKey:  "sk-provider-test",
		},
		"https://example.com/v1/responses",
		[]byte(`{"instructions":"<important_gateway_rules>\nIMPORTANT: AllApiDeck guard rules\n</important_gateway_rules>","model":"gpt-test"}`),
		"gpt-test",
		[]byte(`{"id":"resp_test","output":[]}`),
		false,
		http.StatusOK,
		25*time.Millisecond,
		"",
		nil,
		nil,
	)

	records := advancedProxyRequestRecords.list(10)
	if len(records) != 1 {
		t.Fatalf("expected one request record, got %#v", records)
	}
	if records[0].AntiPoisonPromptPreview == "" || !strings.Contains(records[0].AntiPoisonPromptPreview, "<important_gateway_rules>") {
		t.Fatalf("expected anti-poison prompt preview captured, got %#v", records[0])
	}
}
