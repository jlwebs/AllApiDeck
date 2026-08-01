package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildLocalTokenUsageAnalyticsUsesLatestSessionTotal(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "07", "05")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := `{"timestamp":"2026-07-05T09:50:00Z","type":"session_meta","payload":{"session_id":"a","id":"a","timestamp":"2026-07-05T09:50:00Z"}}`
	turn := `{"timestamp":"2026-07-05T09:55:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`
	tool := `{"timestamp":"2026-07-05T10:05:00Z","type":"response_item","payload":{"type":"function_call","name":"shell_command"}}`
	first := `{"timestamp":"2026-07-05T10:00:00Z","type":"event_msg","payload":{"info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}}}}`
	second := `{"timestamp":"2026-07-05T10:10:00Z","type":"event_msg","payload":{"info":{"total_token_usage":{"input_tokens":160,"cached_input_tokens":80,"output_tokens":30,"reasoning_output_tokens":7,"total_tokens":190}}}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "rollout-a.jsonl"), []byte(meta+"\n"+turn+"\n"+first+"\n"+tool+"\n"+second+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	otherMeta := `{"timestamp":"2026-07-04T08:50:00Z","type":"session_meta","payload":{"session_id":"b","id":"b","timestamp":"2026-07-04T08:50:00Z"}}`
	other := `{"timestamp":"2026-07-04T09:00:00Z","type":"event_msg","payload":{"info":{"total_token_usage":{"input_tokens":40,"cached_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":3,"total_tokens":50}}}}`
	if err := os.MkdirAll(filepath.Join(root, "sessions", "2026", "07", "04"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sessions", "2026", "07", "04", "rollout-b.jsonl"), []byte(otherMeta+"\n"+other+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	analytics, err := buildLocalTokenUsageAnalytics(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}

	if analytics.SessionCount != 2 {
		t.Fatalf("session count = %d, want 2", analytics.SessionCount)
	}
	if analytics.TotalTokens != 240 {
		t.Fatalf("total tokens = %d, want 240", analytics.TotalTokens)
	}
	if analytics.InputTokens != 200 {
		t.Fatalf("input tokens = %d, want 200", analytics.InputTokens)
	}
	if analytics.OutputTokens != 40 {
		t.Fatalf("output tokens = %d, want 40", analytics.OutputTokens)
	}
	if analytics.ReasoningTokens != 10 {
		t.Fatalf("reasoning tokens = %d, want 10", analytics.ReasoningTokens)
	}
	if analytics.TotalTurns != 1 {
		t.Fatalf("total turns = %d, want 1", analytics.TotalTurns)
	}
	if analytics.ToolCallCount != 1 || len(analytics.ToolRanking) != 1 || analytics.ToolRanking[0].Name != "shell_command" {
		t.Fatalf("unexpected tool ranking: count=%d items=%+v", analytics.ToolCallCount, analytics.ToolRanking)
	}
	if len(analytics.Series) != 2 {
		t.Fatalf("series length = %d, want 2", len(analytics.Series))
	}
	firstLocal, _ := time.Parse(time.RFC3339, "2026-07-04T09:00:00Z")
	secondLocal, _ := time.Parse(time.RFC3339, "2026-07-05T10:10:00Z")
	if analytics.Series[0].Date != firstLocal.Local().Format("2006-01-02") || analytics.Series[0].Hour != firstLocal.Local().Format("15") || analytics.Series[0].TotalTokens != 50 {
		t.Fatalf("unexpected first series point: %+v", analytics.Series[0])
	}
	if analytics.Series[1].Date != secondLocal.Local().Format("2006-01-02") || analytics.Series[1].Hour != secondLocal.Local().Format("15") || analytics.Series[1].TotalTokens != 190 {
		t.Fatalf("unexpected second series point: %+v", analytics.Series[1])
	}
}

func TestBuildLocalTokenUsageAnalyticsMissingDirectoryIsEmpty(t *testing.T) {
	analytics, err := buildLocalTokenUsageAnalytics(filepath.Join(t.TempDir(), "missing-sessions"))
	if err != nil {
		t.Fatalf("missing sessions directory returned error: %v", err)
	}
	if analytics.TotalTokens != 0 || len(analytics.Sessions) != 0 {
		t.Fatalf("unexpected analytics for missing directory: %+v", analytics)
	}
}

func TestBuildLocalTokenUsageAnalyticsIncludesArchivedSessions(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions")
	archivedDir := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(archivedDir, 0755); err != nil {
		t.Fatal(err)
	}
	contents := strings.Join([]string{
		`{"timestamp":"2026-07-01T10:00:00Z","type":"session_meta","payload":{"id":"archived"}}`,
		`{"timestamp":"2026-07-01T10:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(archivedDir, "rollout-archived.jsonl"), []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	analytics, err := buildLocalTokenUsageAnalytics(sessionsDir)
	if err != nil {
		t.Fatalf("archived sessions returned error: %v", err)
	}
	if analytics.TotalTokens != 12 || len(analytics.Sessions) != 1 || analytics.Sessions[0].ID != "archived" {
		t.Fatalf("archived session was not included: %+v", analytics)
	}
}

func TestBuildLocalTokenUsageAnalyticsExtractsModelAndCost(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "08", "01")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := `{"timestamp":"2026-08-01T09:50:00Z","type":"session_meta","payload":{"id":"model-session"}}`
	turn := `{"timestamp":"2026-08-01T09:55:00Z","type":"turn_context","payload":{"model":"openai/GPT-5.6-LUNA"}}`
	usage := `{"timestamp":"2026-08-01T10:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"model":"gpt-5.6-luna","total_token_usage":{"input_tokens":160,"cached_input_tokens":80,"output_tokens":30,"reasoning_output_tokens":7,"total_tokens":190}}}}`
	path := filepath.Join(sessionDir, "rollout-model-session.jsonl")
	if err := os.WriteFile(path, []byte(meta+"\n"+turn+"\n"+usage+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	analytics, err := buildLocalTokenUsageAnalytics(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(analytics.Sessions) != 1 {
		t.Fatalf("sessions length = %d, want 1", len(analytics.Sessions))
	}
	session := analytics.Sessions[0]
	if session.Model != "gpt-5.6-luna" {
		t.Fatalf("model = %q, want gpt-5.6-luna", session.Model)
	}
	if session.ModelName != "GPT-5.6 Luna" {
		t.Fatalf("model name = %q, want GPT-5.6 Luna", session.ModelName)
	}
	if session.CacheReadTokens != 80 {
		t.Fatalf("cache read tokens = %d, want 80", session.CacheReadTokens)
	}
	if session.Cost == nil {
		t.Fatal("expected a priced session")
	}
	const wantCost = 0.0000536
	if diff := *session.Cost - wantCost; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("cost = %.10f, want %.10f", *session.Cost, wantCost)
	}
}

func TestNormalizeCodexModelForPricing(t *testing.T) {
	tests := map[string]string{
		"OPENAI/GPT-5.6-LUNA":      "gpt-5.6-luna",
		"gpt-5.4-2026-03-05":       "gpt-5.4",
		"gpt-5.4-20260305":         "gpt-5.4",
		"openai/gpt-5.6-luna:free": "gpt-5.6-luna",
		"azure/gpt-5.2-codex@fast": "gpt-5.2-codex-fast",
	}
	for input, want := range tests {
		if got := normalizeCodexModel(input); got != want {
			t.Errorf("normalizeCodexModel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCodexPricingResolvesCompatibilitySuffixes(t *testing.T) {
	catalog := defaultCodexPricingCatalog()
	for _, model := range []string{
		"gpt-5.4-low",
		"gpt-5.4-20260514",
		"openai.gpt-5.4[1m]",
		"global.openai.gpt-5.4-v1",
	} {
		if _, ok := catalog.resolve(model); !ok {
			t.Fatalf("expected pricing for %q", model)
		}
	}
}

func TestCodexParserAcceptsCacheReadAliasAndLastUsage(t *testing.T) {
	var usage codexSessionAnalytics
	usage.ModelUsages = map[string]codexModelUsage{}
	usage.ToolCounts = map[string]int{}
	readCodexAnalyticsLine(`{"timestamp":"2026-08-01T09:59:00Z","type":"session_meta","payload":{"id":"thread-from-meta"}}`, &usage)
	readCodexAnalyticsLine(`{"timestamp":"2026-08-01T10:00:00Z","type":"turn_context","payload":{"model":"gpt-5.4"}}`, &usage)
	readCodexAnalyticsLine(`{"timestamp":"2026-08-01T10:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"model_name":"GPT-5.4","last_token_usage":{"input_tokens":100,"cache_read_input_tokens":40,"output_tokens":20,"total_tokens":120}}}}`, &usage)
	if usage.SessionID != "thread-from-meta" {
		t.Fatalf("session id = %q, want thread-from-meta", usage.SessionID)
	}
	if usage.InputTokens != 100 || usage.CacheReadTokens != 40 || usage.OutputTokens != 20 || usage.TotalTokens != 120 {
		t.Fatalf("unexpected last usage totals: %+v", usage)
	}
	if usage.ModelUsages["gpt-5.4"].CacheReadTokens != 40 {
		t.Fatalf("cache alias was not tracked: %+v", usage.ModelUsages)
	}
}

func TestMergeModelsDevPricingPrefersCanonicalOpenAIEntry(t *testing.T) {
	input := 0.2
	output := 1.2
	cacheRead := 0.02
	catalog := defaultCodexPricingCatalog()
	mergeModelsDevPricing(catalog, map[string]modelsDevProvider{
		"mirror": {Models: map[string]modelsDevModel{
			"gpt-5.6-luna": {Cost: &modelsDevCost{Input: &input, Output: &output}},
		}},
		"openai": {Models: map[string]modelsDevModel{
			"gpt-5.6-luna": {Cost: &modelsDevCost{Input: &input, Output: &output, CacheRead: &cacheRead}},
		}},
	})
	pricing, ok := catalog.resolve("openai/gpt-5.6-luna")
	if !ok || pricing.Source != "openai" || pricing.CacheReadPerMillion != cacheRead {
		t.Fatalf("unexpected pricing: ok=%v pricing=%+v", ok, pricing)
	}
}

func TestCodexSessionCostTracksModelSwitches(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "08", "01")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"timestamp":"2026-08-01T09:50:00Z","type":"session_meta","payload":{"id":"switch-session"}}`,
		`{"timestamp":"2026-08-01T09:51:00Z","type":"turn_context","payload":{"model":"gpt-5.5"}}`,
		`{"timestamp":"2026-08-01T09:52:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":10,"total_tokens":110}}}}`,
		`{"timestamp":"2026-08-01T09:53:00Z","type":"turn_context","payload":{"model":"gpt-5.6-luna"}}`,
		`{"timestamp":"2026-08-01T09:54:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":160,"cached_input_tokens":40,"output_tokens":30,"total_tokens":190}}}}`,
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "rollout-switch-session.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	analytics, err := buildLocalTokenUsageAnalytics(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(analytics.Sessions) != 1 || analytics.Sessions[0].Model != "Multiple models" {
		t.Fatalf("unexpected switched session: %+v", analytics.Sessions)
	}
	if analytics.Sessions[0].Cost == nil {
		t.Fatal("expected switched session to have a combined cost")
	}
	if len(analytics.Sessions[0].ModelCosts) != 2 {
		t.Fatalf("model cost breakdown length = %d, want 2: %+v", len(analytics.Sessions[0].ModelCosts), analytics.Sessions[0].ModelCosts)
	}
	var breakdownTotal float64
	for _, modelCost := range analytics.Sessions[0].ModelCosts {
		if modelCost.Cost <= 0 {
			t.Fatalf("expected positive cost for %q, got %.10f", modelCost.Model, modelCost.Cost)
		}
		breakdownTotal += modelCost.Cost
	}
	if breakdownTotal != *analytics.Sessions[0].Cost {
		t.Fatalf("model cost breakdown = %.10f, combined cost = %.10f", breakdownTotal, *analytics.Sessions[0].Cost)
	}
}
