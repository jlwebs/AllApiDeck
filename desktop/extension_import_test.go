package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportExtensionAccountsFromExistingChromeStorage(t *testing.T) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		t.Skip("LOCALAPPDATA is empty")
	}

	path := filepath.Join(
		localAppData,
		"Google",
		"Chrome",
		"User Data",
		"Default",
		"Local Extension Settings",
		defaultExtensionID,
	)
	if !isDirectory(path) {
		t.Skipf("extension storage not found: %s", path)
	}

	result, err := importExtensionAccountsFromDir(path)
	if err != nil {
		t.Fatalf("importExtensionAccountsFromDir failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.AccountCount <= 0 {
		t.Fatalf("expected accountCount > 0, got %d", result.AccountCount)
	}

	accountsConfig, ok := result.Payload["accounts"].(map[string]any)
	if !ok {
		t.Fatalf("payload.accounts missing or wrong type: %#v", result.Payload["accounts"])
	}
	accounts, ok := accountsConfig["accounts"].([]any)
	if !ok {
		t.Fatalf("payload.accounts.accounts missing or wrong type: %#v", accountsConfig["accounts"])
	}
	if len(accounts) != result.AccountCount {
		t.Fatalf("account count mismatch: payload=%d result=%d", len(accounts), result.AccountCount)
	}
}
