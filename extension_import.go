package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	extensionStorageKey = "site_accounts"
	defaultExtensionID  = "lapnciffpekdengooeolaienkeoilfeo"
)

type ExtensionImportResult struct {
	SourcePath   string         `json:"sourcePath"`
	StorageKey   string         `json:"storageKey"`
	AccountCount int            `json:"accountCount"`
	Payload      map[string]any `json:"payload"`
}

func (a *App) ImportExtensionAccounts() (*ExtensionImportResult, error) {
	candidates := discoverExtensionStorageCandidates(defaultExtensionID)
	var lastErr error

	for _, candidate := range candidates {
		result, err := importExtensionAccountsFromDir(candidate)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to import browser extension accounts automatically: %w", lastErr)
	}

	return nil, fmt.Errorf("browser extension storage directory not found automatically")
}

func importExtensionAccountsFromDir(inputDir string) (*ExtensionImportResult, error) {
	sourceDir, err := resolveExtensionStorageDir(inputDir, defaultExtensionID)
	if err != nil {
		return nil, err
	}

	tempRoot, err := os.MkdirTemp("", "batch-api-check-extdb-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	tempDBDir := filepath.Join(tempRoot, "db-copy")
	if err := copyDirectory(sourceDir, tempDBDir); err != nil {
		return nil, fmt.Errorf("failed to copy extension database: %w", err)
	}

	db, err := leveldb.OpenFile(tempDBDir, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb copy: %w", err)
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	var parseErr error
	for iter.Next() {
		keyStr := string(iter.Key())
		if !strings.Contains(keyStr, extensionStorageKey) {
			continue
		}

		decoded, err := decodeExtensionStorageValue(iter.Value())
		if err != nil {
			parseErr = err
			continue
		}

		payload, accountCount, err := normalizeAccountsBackupPayload(decoded)
		if err != nil {
			parseErr = err
			continue
		}

		return &ExtensionImportResult{
			SourcePath:   sourceDir,
			StorageKey:   keyStr,
			AccountCount: accountCount,
			Payload:      payload,
		}, nil
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("failed to iterate leveldb: %w", err)
	}
	if parseErr != nil {
		return nil, fmt.Errorf("found %q but failed to parse it: %w", extensionStorageKey, parseErr)
	}

	return nil, fmt.Errorf("did not find %q in extension storage", extensionStorageKey)
}

func resolveExtensionStorageDir(inputDir string, extensionID string) (string, error) {
	dir := filepath.Clean(strings.TrimSpace(inputDir))
	if dir == "" {
		return "", fmt.Errorf("directory is empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", dir)
	}

	if strings.EqualFold(filepath.Base(dir), extensionID) {
		return dir, nil
	}

	if candidate := filepath.Join(dir, extensionID); isDirectory(candidate) {
		return candidate, nil
	}

	if candidate := filepath.Join(dir, "Local Extension Settings", extensionID); isDirectory(candidate) {
		return candidate, nil
	}

	return "", fmt.Errorf("unable to locate extension directory under %s", dir)
}

func discoverExtensionStorageCandidates(extensionID string) []string {
	var candidates []string

	for _, root := range browserUserDataRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(root, entry.Name(), "Local Extension Settings", extensionID)
			if isDirectory(candidate) {
				candidates = append(candidates, candidate)
			}
		}
	}

	slices.SortStableFunc(candidates, func(a, b string) int {
		statA, errA := os.Stat(a)
		statB, errB := os.Stat(b)
		if errA != nil && errB != nil {
			return strings.Compare(a, b)
		}
		if errA != nil {
			return 1
		}
		if errB != nil {
			return -1
		}
		return statB.ModTime().Compare(statA.ModTime())
	})

	return slices.Compact(candidates)
}

func browserUserDataRoots() []string {
	var roots []string

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if isDirectory(value) {
			roots = append(roots, value)
		}
	}

	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		add(filepath.Join(localAppData, "Google", "Chrome", "User Data"))
		add(filepath.Join(localAppData, "Microsoft", "Edge", "User Data"))
		add(filepath.Join(localAppData, "Chromium", "User Data"))
		add(filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data"))
	case "darwin":
		home, _ := os.UserHomeDir()
		add(filepath.Join(home, "Library", "Application Support", "Google", "Chrome"))
		add(filepath.Join(home, "Library", "Application Support", "Microsoft Edge"))
		add(filepath.Join(home, "Library", "Application Support", "Chromium"))
		add(filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser"))
	default:
		home, _ := os.UserHomeDir()
		add(filepath.Join(home, ".config", "google-chrome"))
		add(filepath.Join(home, ".config", "microsoft-edge"))
		add(filepath.Join(home, ".config", "chromium"))
		add(filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser"))
	}

	return slices.Compact(roots)
}

func isDirectory(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func decodeExtensionStorageValue(raw []byte) (any, error) {
	var direct any
	if err := json.Unmarshal(raw, &direct); err == nil {
		return unwrapJSONStringIfNeeded(direct)
	}

	text := strings.Trim(string(raw), "\x00 \t\r\n")
	text = strings.TrimPrefix(text, "\ufeff")
	if text == "" {
		return nil, fmt.Errorf("storage value is empty")
	}

	if parsed, err := decodeJSONText(text); err == nil {
		return parsed, nil
	}

	extracted, err := extractEmbeddedJSON(text)
	if err != nil {
		return nil, err
	}
	return decodeJSONText(extracted)
}

func decodeJSONText(text string) (any, error) {
	var decoded any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return nil, err
	}
	return unwrapJSONStringIfNeeded(decoded)
}

func unwrapJSONStringIfNeeded(value any) (any, error) {
	for {
		str, ok := value.(string)
		if !ok {
			return value, nil
		}

		trimmed := strings.TrimSpace(str)
		if trimmed == "" {
			return "", nil
		}
		if !(strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
			return str, nil
		}

		var nested any
		if err := json.Unmarshal([]byte(trimmed), &nested); err != nil {
			return nil, err
		}
		value = nested
	}
}

func extractEmbeddedJSON(text string) (string, error) {
	start := -1
	for i, r := range text {
		if r == '{' || r == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("did not find embedded json start token")
	}

	for end := len(text); end > start; end-- {
		last := text[end-1]
		if last != '}' && last != ']' {
			continue
		}
		candidate := strings.TrimSpace(text[start:end])
		var decoded any
		if err := json.Unmarshal([]byte(candidate), &decoded); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("failed to extract valid embedded json")
}

func normalizeAccountsBackupPayload(decoded any) (map[string]any, int, error) {
	now := time.Now().UnixMilli()
	accountsConfig, tagStore, err := extractAccountsConfig(decoded, now)
	if err != nil {
		return nil, 0, err
	}

	accountList, _ := accountsConfig["accounts"].([]any)
	payload := map[string]any{
		"version":   "2.0",
		"timestamp": now,
		"type":      "accounts",
		"accounts":  accountsConfig,
		"tagStore":  tagStore,
	}

	return payload, len(accountList), nil
}

func extractAccountsConfig(decoded any, now int64) (map[string]any, map[string]any, error) {
	defaultTagStore := map[string]any{
		"version":  1,
		"tagsById": map[string]any{},
	}

	switch value := decoded.(type) {
	case []any:
		return map[string]any{
			"accounts":          value,
			"bookmarks":         []any{},
			"pinnedAccountIds":  []any{},
			"orderedAccountIds": []any{},
			"last_updated":      now,
		}, defaultTagStore, nil
	case map[string]any:
		if accountsConfig, ok := normalizeAccountsConfig(value["accounts"], now); ok {
			tagStore := defaultTagStore
			if rawTagStore, ok := value["tagStore"].(map[string]any); ok {
				tagStore = rawTagStore
			}
			return accountsConfig, tagStore, nil
		}

		if dataField, ok := value["data"].(map[string]any); ok {
			if accountsConfig, ok := normalizeAccountsConfig(dataField["accounts"], now); ok {
				tagStore := defaultTagStore
				if rawTagStore, ok := dataField["tagStore"].(map[string]any); ok {
					tagStore = rawTagStore
				} else if rawTagStore, ok := value["tagStore"].(map[string]any); ok {
					tagStore = rawTagStore
				}
				return accountsConfig, tagStore, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("unsupported site_accounts structure")
}

func normalizeAccountsConfig(raw any, now int64) (map[string]any, bool) {
	switch value := raw.(type) {
	case []any:
		return map[string]any{
			"accounts":          value,
			"bookmarks":         []any{},
			"pinnedAccountIds":  []any{},
			"orderedAccountIds": []any{},
			"last_updated":      now,
		}, true
	case map[string]any:
		accounts, ok := value["accounts"].([]any)
		if !ok {
			return nil, false
		}

		bookmarks, _ := value["bookmarks"].([]any)
		pinnedIDs, _ := value["pinnedAccountIds"].([]any)
		orderedIDs, _ := value["orderedAccountIds"].([]any)
		lastUpdated, hasLastUpdated := value["last_updated"]
		if !hasLastUpdated {
			lastUpdated = now
		}

		return map[string]any{
			"accounts":          accounts,
			"bookmarks":         defaultAnySlice(bookmarks),
			"pinnedAccountIds":  defaultAnySlice(pinnedIDs),
			"orderedAccountIds": defaultAnySlice(orderedIDs),
			"last_updated":      lastUpdated,
		}, true
	}

	return nil, false
}

func defaultAnySlice(value []any) []any {
	if value == nil {
		return []any{}
	}
	return value
}

func copyDirectory(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if strings.EqualFold(filepath.Base(path), "LOCK") {
				return nil
			}
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if strings.EqualFold(info.Name(), "LOCK") {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			if strings.EqualFold(info.Name(), "LOCK") {
				return nil
			}
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
