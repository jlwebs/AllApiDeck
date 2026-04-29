package main

import (
	"strings"
	"testing"
)

func TestBuildCheckEndpointCandidatesAnyrouterOrder(t *testing.T) {
	payload := normalizedCheckKeyPayload{
		URL:      "https://anyrouter.top",
		SiteType: "anyrouter",
	}

	got := buildCheckEndpointCandidates(payload)
	want := []string{
		"https://anyrouter.top/v1/chat/completions",
		"https://anyrouter.top/v1/responses",
		"https://anyrouter.top/v1/messages",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected candidate count: got=%d want=%d values=%#v", len(got), len(want), got)
	}

	for index, expected := range want {
		if got[index] != expected {
			t.Fatalf("unexpected candidate at %d: got=%q want=%q all=%#v", index, got[index], expected, got)
		}
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
