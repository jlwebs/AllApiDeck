package main

import (
	"os"
	"path/filepath"
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
