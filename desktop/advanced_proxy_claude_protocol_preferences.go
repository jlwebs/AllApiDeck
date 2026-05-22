package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	advancedProxyClaudeProtocolPreferAnthropic = 0
	advancedProxyClaudeProtocolPreferResponses = 1
	advancedProxyClaudeProtocolPreferChat      = 2
)

type advancedProxyClaudeProtocolPreferenceStore struct {
	mu     sync.Mutex
	loaded bool
	values map[string]int
}

var advancedProxyClaudeProtocolPreferences = advancedProxyClaudeProtocolPreferenceStore{}

func resolveAdvancedProxyClaudeProtocolPreferencePath() string {
	return filepath.Join(resolveRuntimeRootDir(), "advanced-proxy", "claude-protocol-preferences.json")
}

func resolveAdvancedProxyClaudeProtocolPreferenceScopeKey(provider AdvancedProxyProvider, model string) string {
	baseScope := resolveAdvancedProxyOpenAIProtocolPreferenceScopeKey(provider, model)
	if strings.TrimSpace(baseScope) == "" {
		return ""
	}
	return baseScope + "&claude_api_format=" + url.QueryEscape(normalizeClaudeAPIFormat(provider.APIFormat))
}

func loadAdvancedProxyClaudeProtocolPreferencesLocked() {
	if advancedProxyClaudeProtocolPreferences.loaded {
		return
	}
	advancedProxyClaudeProtocolPreferences.loaded = true
	advancedProxyClaudeProtocolPreferences.values = map[string]int{}

	raw, err := os.ReadFile(resolveAdvancedProxyClaudeProtocolPreferencePath())
	if err != nil {
		return
	}

	var decoded map[string]int
	if err := json.Unmarshal(raw, &decoded); err != nil {
		appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_DECODE_FAIL] detail=%s", previewAdvancedProxyText(err.Error(), 220))
		return
	}

	for scopeKey, value := range decoded {
		scopeKey = strings.TrimSpace(scopeKey)
		if scopeKey == "" {
			continue
		}
		switch value {
		case advancedProxyClaudeProtocolPreferAnthropic,
			advancedProxyClaudeProtocolPreferResponses,
			advancedProxyClaudeProtocolPreferChat:
			advancedProxyClaudeProtocolPreferences.values[scopeKey] = value
		}
	}
}

func getAdvancedProxyClaudeProtocolPreference(scopeKey string) (int, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return 0, false
	}

	advancedProxyClaudeProtocolPreferences.mu.Lock()
	defer advancedProxyClaudeProtocolPreferences.mu.Unlock()
	loadAdvancedProxyClaudeProtocolPreferencesLocked()

	value, ok := advancedProxyClaudeProtocolPreferences.values[scopeKey]
	return value, ok
}

func normalizeAdvancedProxyClaudeProtocolPreference(value int) int {
	switch value {
	case advancedProxyClaudeProtocolPreferResponses:
		return advancedProxyClaudeProtocolPreferResponses
	case advancedProxyClaudeProtocolPreferChat:
		return advancedProxyClaudeProtocolPreferChat
	default:
		return advancedProxyClaudeProtocolPreferAnthropic
	}
}

func setAdvancedProxyClaudeProtocolPreference(scopeKey string, value int) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return
	}
	value = normalizeAdvancedProxyClaudeProtocolPreference(value)

	advancedProxyClaudeProtocolPreferences.mu.Lock()
	loadAdvancedProxyClaudeProtocolPreferencesLocked()
	current, exists := advancedProxyClaudeProtocolPreferences.values[scopeKey]
	if exists && current == value {
		advancedProxyClaudeProtocolPreferences.mu.Unlock()
		return
	}
	advancedProxyClaudeProtocolPreferences.values[scopeKey] = value
	snapshot := make(map[string]int, len(advancedProxyClaudeProtocolPreferences.values))
	for key, item := range advancedProxyClaudeProtocolPreferences.values {
		snapshot[key] = item
	}
	advancedProxyClaudeProtocolPreferences.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_ENCODE_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
	if err := os.MkdirAll(filepath.Dir(resolveAdvancedProxyClaudeProtocolPreferencePath()), 0o755); err != nil {
		appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_MKDIR_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
	if err := os.WriteFile(resolveAdvancedProxyClaudeProtocolPreferencePath(), data, 0o644); err != nil {
		appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_WRITE_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
}

func resetAdvancedProxyClaudeProtocolPreferencesForTests() {
	advancedProxyClaudeProtocolPreferences.mu.Lock()
	defer advancedProxyClaudeProtocolPreferences.mu.Unlock()
	advancedProxyClaudeProtocolPreferences.loaded = false
	advancedProxyClaudeProtocolPreferences.values = nil
}

func describeAdvancedProxyClaudeProtocolPreference(value int) string {
	switch normalizeAdvancedProxyClaudeProtocolPreference(value) {
	case advancedProxyClaudeProtocolPreferResponses:
		return "responses"
	case advancedProxyClaudeProtocolPreferChat:
		return "chat"
	default:
		return "messages"
	}
}

func shouldFallbackClaudeMessagesToOpenAIRoute(statusCode int, responseBody []byte) bool {
	switch statusCode {
	case http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusUnsupportedMediaType:
		return true
	}

	message := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		normalizeAnthropicErrorMessage(responseBody),
		summarizeAdvancedProxyBody(responseBody),
		fmt.Sprintf("http %d", statusCode),
	)))
	if message == "" {
		return false
	}

	switch {
	case strings.Contains(message, "unknown api route"):
		return true
	case strings.Contains(message, "unsupported") && strings.Contains(message, "route"):
		return true
	case strings.Contains(message, "not implemented"):
		return true
	case strings.Contains(message, "not found"):
		return true
	case strings.Contains(message, "unauthorized"):
		return true
	case strings.Contains(message, "forbidden"):
		return true
	case strings.Contains(message, "authentication"):
		return true
	case strings.Contains(message, "bearer"):
		return true
	case strings.Contains(message, "(html)"):
		return true
	default:
		return false
	}
}
