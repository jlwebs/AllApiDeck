package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadClaudeSessionAnalyticsAggregatesUsageAndDeduplicatesMessages(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "project", "session-claude.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"type":"user","timestamp":"2026-08-01T09:00:00Z","sessionId":"session-claude","message":{"role":"user"}}`,
		`{"type":"assistant","timestamp":"2026-08-01T09:01:00Z","sessionId":"session-claude","message":{"id":"msg-1","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":100,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":10}}}`,
		`{"type":"assistant","timestamp":"2026-08-01T09:01:01Z","sessionId":"session-claude","message":{"id":"msg-1","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":100,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":10}}}`,
		`{"type":"assistant","timestamp":"2026-08-01T09:02:00Z","sessionId":"session-claude","message":{"id":"msg-synthetic","role":"assistant","model":"<synthetic>","usage":{"input_tokens":999,"output_tokens":999}}}`,
		`{"type":"assistant","timestamp":"2026-08-01T09:03:00Z","sessionId":"session-claude","message":{"id":"msg-2","role":"assistant","model":"claude-sonnet-4-8","usage":{"input_tokens":40,"cache_read_input_tokens":5,"output_tokens":7}}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	usage, ok := readClaudeSessionAnalytics(path)
	if !ok {
		t.Fatal("expected Claude session usage")
	}
	if usage.SessionID != "session-claude" {
		t.Fatalf("session id = %q, want session-claude", usage.SessionID)
	}
	if usage.InputTokens != 160 || usage.CacheReadTokens != 35 || usage.OutputTokens != 17 || usage.TotalTokens != 212 {
		t.Fatalf("unexpected token totals: %+v", usage)
	}
	if usage.TurnCount != 2 || usage.Model != "Multiple models" {
		t.Fatalf("unexpected session metadata: %+v", usage)
	}
	if len(usage.ModelUsages) != 2 {
		t.Fatalf("model usage count = %d, want 2: %+v", len(usage.ModelUsages), usage.ModelUsages)
	}
	if usage.ModelUsages["claude-opus-4-8"].TotalTokens != 160 || usage.ModelUsages["claude-sonnet-4-8"].TotalTokens != 52 {
		t.Fatalf("unexpected per-model token totals: %+v", usage.ModelUsages)
	}
}

func TestMergeClaudeLocalTokenUsageAnalyticsIncludesModelTokenSplit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "project", "session-claude.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	contents := strings.Join([]string{
		`{"type":"assistant","timestamp":"2026-08-01T09:01:00Z","sessionId":"session-claude","message":{"id":"msg-1","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":100,"output_tokens":10}}}`,
		`{"type":"assistant","timestamp":"2026-08-01T09:02:00Z","sessionId":"session-claude","message":{"id":"msg-2","role":"assistant","model":"claude-sonnet-4-8","usage":{"input_tokens":40,"output_tokens":7}}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	analytics := LocalTokenUsageAnalytics{}
	if err := mergeClaudeLocalTokenUsageAnalytics(&analytics, filepath.Join(root, "project")); err != nil {
		t.Fatal(err)
	}
	if len(analytics.Sessions) != 1 {
		t.Fatalf("session count = %d, want 1", len(analytics.Sessions))
	}
	modelCosts := analytics.Sessions[0].ModelCosts
	if len(modelCosts) != 2 {
		t.Fatalf("model cost count = %d, want 2: %+v", len(modelCosts), modelCosts)
	}
	if modelCosts[0].Tokens != 110 || modelCosts[1].Tokens != 47 {
		t.Fatalf("unexpected model token split: %+v", modelCosts)
	}
	for _, item := range modelCosts {
		if item.CostKnown {
			t.Fatalf("Claude model cost should be token-only: %+v", item)
		}
	}
}

func TestMergeClaudeLocalTokenUsageAnalyticsCreatesSeparateSources(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "project", "session-claude.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	contents := `{"type":"assistant","timestamp":"2026-08-01T09:03:00Z","sessionId":"session-claude","message":{"id":"msg-1","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":40,"output_tokens":7}}}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	analytics := LocalTokenUsageAnalytics{
		Source:      "codex",
		SourceLabel: "Codex",
		Sessions: []LocalTokenUsageSession{{
			ID:           "codex-session",
			Timestamp:    "2026-08-01T08:00:00Z",
			AppType:      "codex",
			Source:       "codex_session",
			SourceLabel:  "Codex",
			InputTokens:  100,
			OutputTokens: 10,
			TotalTokens:  110,
		}},
		InputTokens:  100,
		OutputTokens: 10,
		TotalTokens:  110,
		SessionCount: 1,
	}
	if err := mergeClaudeLocalTokenUsageAnalytics(&analytics, filepath.Join(root, "project")); err != nil {
		t.Fatal(err)
	}
	if len(analytics.Sessions) != 2 || analytics.TotalTokens != 157 || analytics.InputTokens != 140 || analytics.OutputTokens != 17 {
		t.Fatalf("unexpected merged analytics: %+v", analytics)
	}
	if analytics.Source != "local_sessions" || len(analytics.Sources) != 2 {
		t.Fatalf("unexpected merged sources: source=%q sources=%+v", analytics.Source, analytics.Sources)
	}
	if analytics.Sources[0].Source != "codex" || analytics.Sources[1].Source != "claude" {
		t.Fatalf("unexpected source ordering: %+v", analytics.Sources)
	}
}
