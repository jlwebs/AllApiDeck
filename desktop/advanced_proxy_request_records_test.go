package main

import (
	"net/http"
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
			AppType:         "claude",
			ClientRoute:     "messages",
			InboundEndpoint: buildAdvancedProxyClaudeInboundEndpoint(),
			OutboundRoute:   "responses",
			Source:          "fallback",
			Provider:        provider,
			TargetURL:       "https://example.com/v1/responses",
			RequestBody:     []byte(`{"model":"gpt-5.5","stream":true}`),
			ResolvedModel:   "claude-sonnet-4-7",
			StartedAt:       startedAt,
			ObservedFormat:  "openai_responses",
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
