package main

import (
	"os"
	"path/filepath"
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
