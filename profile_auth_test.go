package main

import (
	"path/filepath"
	"testing"
)

func TestChromeDefaultLocalStorageCandidatesWindows(t *testing.T) {
	got := chromeDefaultLocalStorageCandidates("windows", `C:\Users\alice\AppData\Local`, "")
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}

	want := filepath.Join(`C:\Users\alice\AppData\Local`, "Google", "Chrome", "User Data", "Default", "Local Storage", "leveldb")
	if got[0] != want {
		t.Fatalf("unexpected windows candidate: got %q want %q", got[0], want)
	}
}

func TestChromeDefaultLocalStorageCandidatesMacOS(t *testing.T) {
	got := chromeDefaultLocalStorageCandidates("darwin", "", "/Users/alice")
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(got))
	}

	want := filepath.Join("/Users/alice", "Library", "Application Support", "Google", "Chrome", "Default", "Local Storage", "leveldb")
	if got[0] != want {
		t.Fatalf("unexpected macOS candidate: got %q want %q", got[0], want)
	}
}

func TestChromeDefaultLocalStorageCandidatesLinux(t *testing.T) {
	got := chromeDefaultLocalStorageCandidates("linux", "", "/home/alice")
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(got))
	}

	wantFirst := filepath.Join("/home/alice", ".config", "google-chrome", "Default", "Local Storage", "leveldb")
	wantSecond := filepath.Join("/home/alice", ".config", "chromium", "Default", "Local Storage", "leveldb")
	if got[0] != wantFirst || got[1] != wantSecond {
		t.Fatalf("unexpected linux candidates: got %q", got)
	}
}
