package main

import "testing"

import (
	"os"
	"path/filepath"
	"strings"
)

func TestNormalizeCandyEvalOpenAIBaseURLMatchesQuickTestResponsesRoute(t *testing.T) {
	cases := map[string]string{
		"https://www.cun.ai":               "https://www.cun.ai/v1",
		"https://example.com/v1":           "https://example.com/v1",
		"https://example.com/v1/responses": "https://example.com/v1",
		"https://example.com/responses":    "https://example.com",
	}
	for input, expected := range cases {
		if actual := normalizeCandyEvalOpenAIBaseURL(input); actual != expected {
			t.Errorf("normalizeCandyEvalOpenAIBaseURL(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestCandyEvalAnswerKeyMatchesReferenceSuite(t *testing.T) {
	if candyEvalExpectedAnswer != 21 {
		t.Fatalf("answer key = %d, want 21", candyEvalExpectedAnswer)
	}
	if !candyEvalAnswerPattern.MatchString("参考题库的答案是 21") {
		t.Fatal("21 should be accepted")
	}
	if candyEvalAnswerPattern.MatchString("答案是 29") {
		t.Fatal("29 must not be accepted")
	}
}

func TestPrepareCandyEvalCodexHomePersistsRequestedEffort(t *testing.T) {
	home, cleanup, err := prepareCandyEvalCodexHome(candyEvalTarget{SiteURL: "https://example.test/v1", APIKey: "test-key"}, candyEvalModeDirect, "", "test-run", "max")
	if err != nil {
		t.Fatalf("prepareCandyEvalCodexHome returned error: %v", err)
	}
	defer cleanup()
	rawConfig, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatalf("read temporary config: %v", err)
	}
	if !strings.Contains(string(rawConfig), `model_reasoning_effort = "max"`) {
		t.Fatalf("temporary config did not persist effort: %s", rawConfig)
	}
}
