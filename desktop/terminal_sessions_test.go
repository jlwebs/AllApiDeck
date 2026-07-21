package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanTerminalSessionsSortsByLastActiveBeforePaging(t *testing.T) {
	root := t.TempDir()
	writeTerminalSessionTestFile(t, filepath.Join(root, "old.json"), 1000)
	writeTerminalSessionTestFile(t, filepath.Join(root, "middle.json"), 3000)
	writeTerminalSessionTestFile(t, filepath.Join(root, "latest.json"), 5000)

	provider := terminalSessionProviderDef{
		id:      "test",
		roots:   func() []string { return []string{root} },
		fileExt: ".json",
		parse: func(path string) (TerminalSessionMeta, bool) {
			switch filepath.Base(path) {
			case "old.json":
				return TerminalSessionMeta{SessionID: "old", LastActiveAt: 1000}, true
			case "middle.json":
				return TerminalSessionMeta{SessionID: "middle", LastActiveAt: 3000}, true
			case "latest.json":
				return TerminalSessionMeta{SessionID: "latest", LastActiveAt: 5000}, true
			default:
				return TerminalSessionMeta{}, false
			}
		},
	}

	sessions, total := scanTerminalSessionsForProvider(provider, 1, 1)
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one session on page, got %d", len(sessions))
	}
	if sessions[0].SessionID != "latest" {
		t.Fatalf("expected latest session first, got %#v", sessions[0])
	}
}

func writeTerminalSessionTestFile(t *testing.T, path string, modTimeMs int64) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write test session file: %v", err)
	}
	seconds := modTimeMs / 1000
	nanos := (modTimeMs % 1000) * int64(1000000)
	modTime := time.Unix(seconds, nanos)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes test session file: %v", err)
	}
}

func TestScanGrokTerminalSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	sessionDir := filepath.Join(home, ".grok", "sessions", "encoded_cwd", "sess-123")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	summary := `{
  "info": {"id": "sess-123", "cwd": "S:\\project\\demo"},
  "session_summary": "hello grok",
  "generated_title": "Hello Grok",
  "created_at": "2026-07-19T02:46:20.145970900Z",
  "last_active_at": "2026-07-19T06:08:09.410679900Z"
}`
	if err := os.WriteFile(filepath.Join(sessionDir, "summary.json"), []byte(summary), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	chat := strings.Join([]string{
		`{"type":"system","content":"sys"}`,
		`{"type":"user","content":[{"type":"text","text":"<user_info>\nOS\n</user_info>"}]}`,
		`{"type":"user","synthetic_reason":"system_reminder","content":[{"type":"text","text":"<system-reminder>x</system-reminder>"}]}`,
		`{"type":"user","content":[{"type":"text","text":"<user_query>\nclone repo please\n</user_query>"}]}`,
		`{"type":"assistant","content":"Sure, cloning now."}`,
		`{"type":"assistant","content":"","tool_calls":[{"name":"run"}]}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "chat_history.jsonl"), []byte(chat), 0o644); err != nil {
		t.Fatalf("write chat: %v", err)
	}

	sessions, total := scanGrokTerminalSessions(1, 10)
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(sessions) != 1 || sessions[0].SessionID != "sess-123" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
	if sessions[0].Title != "Hello Grok" {
		t.Fatalf("unexpected title: %q", sessions[0].Title)
	}
	if sessions[0].ProviderID != "grok" {
		t.Fatalf("unexpected provider: %q", sessions[0].ProviderID)
	}

	messages, err := loadGrokTerminalSessionMessages(sessions[0].SourcePath)
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %#v", messages)
	}
	if messages[0].Role != "user" || messages[0].Content != "clone repo please" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "Sure, cloning now." {
		t.Fatalf("unexpected assistant message: %#v", messages[1])
	}
}
