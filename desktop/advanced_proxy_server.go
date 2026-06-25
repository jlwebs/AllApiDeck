package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	advancedProxySSEScannerMaxTokenSize = 16 * 1024 * 1024
	advancedProxyMaxRequestBodyBytes    = 64 * 1024 * 1024
)

var webSearchResultURLPattern = regexp.MustCompile(`https?://[^\s<>"')\]]+`)
var encryptedContentNeedlePattern = regexp.MustCompile(`(?i)encrypted_content`)

type providerAttemptResult struct {
	Response          map[string]any
	StatusCode        int
	Message           string
	Headers           http.Header
	StreamBody        io.ReadCloser
	APIFormat         string
	Model             string
	RecordCtx         *advancedProxyStreamRequestRecordContext
	AntiPoisonBlocked bool
}

type rawProviderAttemptResult struct {
	StatusCode        int
	Message           string
	ErrorCode         string
	ErrorType         string
	Body              []byte
	Headers           http.Header
	StreamBody        io.ReadCloser
	ProviderID        string
	Provider          string
	TargetURL         string
	RouteKind         string
	RecordCtx         *advancedProxyStreamRequestRecordContext
	AntiPoisonBlocked bool
}

type advancedProxyStreamRequestRecordContext struct {
	AppType                  string
	ClientRoute              string
	InboundEndpoint          string
	OutboundRoute            string
	RouteTrace               []AdvancedProxyRequestRouteStep
	Source                   string
	Provider                 AdvancedProxyProvider
	TargetURL                string
	RequestBody              []byte
	TimeoutSeconds           int
	ResolvedModel            string
	StartedAt                time.Time
	ObservedFormat           string
	AntiPoisonCtx            antiPoisonRequestContext
	StringProtect            antiPoisonStringProtectionContext
	AntiPoisonOps            []antiPoisonOperationRecord
	UpstreamResponsePreview  string
	UpstreamResponseRaw      string
	DeliveredResponsePreview string
	UpstreamToolCalls        []string
	UpstreamToolArgsPreview  []string
	UpstreamAssistantPreview string
	UpstreamLatestObserved   *advancedProxyObservedItem
}

type advancedProxyStreamObservation struct {
	StartedAt     time.Time
	FirstOutputAt *time.Time
	CompletedAt   time.Time
	InputTokens   *int
	OutputTokens  *int
}

type encryptedContentHealingContext struct {
	SessionKey           string
	OriginalCount        int
	AppliedHistoricalCut int
	RemovedIncludeRefs   int
}

type encryptedContentFinalSanitizationStats struct {
	RemovedFields      int
	RemovedIncludeRefs int
	ScrubbedStrings    int
	ResidualHits       int
}

type advancedProxyEncryptedContentHealStore struct {
	mu       sync.Mutex
	sessions map[string]int
}

const encryptedContentHealingNotice = "【ALL-API-Deck 网关已探测到，将自动愈合，请继续对话】"

var advancedProxyEncryptedContentHealState = advancedProxyEncryptedContentHealStore{
	sessions: map[string]int{},
}

func resolveAdvancedProxyLogPath() string {
	return filepath.Join(resolveRuntimeLogDir(), "advanced-proxy.log")
}

func appendAdvancedProxyLogf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	appendLine(resolveAdvancedProxyLogPath(), message)
	debugLogf("[ADV_PROXY] %s", message)
}

func advancedProxyDebugEnabled(config AdvancedProxyConfig) bool {
	if config.DebugLogging {
		return true
	}
	value := strings.TrimSpace(strings.ToLower(os.Getenv("BATCH_API_CHECK_ADVANCED_PROXY_DEBUG")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func summarizeAdvancedProxyJSON(value any, limit int) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return previewAdvancedProxyText(fmt.Sprint(value), limit)
	}
	return previewAdvancedProxyText(string(raw), limit)
}

func previewAdvancedProxyText(raw string, limit int) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "\r", " "), "\n", " "))
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if limit <= 0 || len(runes) <= limit {
		return normalized
	}
	return string(runes[:limit]) + "..."
}

func summarizeAdvancedProxyStreamResult(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return ""
	}
	return previewAdvancedProxyText(strings.Join(filtered, " | "), 2200)
}

func summarizeAdvancedProxyRawStreamPreview(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return previewAdvancedProxyText(string(raw), 2200)
}

func summarizeAdvancedProxyRawStreamFeedbackContext(raw []byte, observedFormat string) ([]string, []string, string, *advancedProxyObservedItem) {
	if len(raw) == 0 {
		return nil, nil, "", nil
	}
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		return nil, nil, previewAdvancedProxyText(string(raw), 800), &advancedProxyObservedItem{
			Type:       "raw_stream",
			RawPreview: string(raw),
		}
	}
	toolNames := make([]string, 0, 8)
	toolArgs := make([]string, 0, 8)
	textParts := make([]string, 0, 16)
	var latest *advancedProxyObservedItem
	appendTool := func(name string, args string) {
		name = strings.TrimSpace(name)
		args = strings.TrimSpace(args)
		if name != "" {
			toolNames = append(toolNames, name)
		}
		if args != "" {
			toolArgs = append(toolArgs, args)
		}
		latest = &advancedProxyObservedItem{
			Type:             "function_call",
			Name:             name,
			ArgumentsPreview: args,
			RawPreview:       args,
		}
	}
	appendText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		textParts = append(textParts, text)
		latest = &advancedProxyObservedItem{
			Type:        "message",
			TextPreview: strings.Join(textParts, " "),
			RawPreview:  text,
		}
	}
	switch normalizeAdvancedProxyObservedFormat(observedFormat) {
	case "responses":
		for _, event := range events {
			data, ok := advancedProxySSEJSONPayload(event)
			if !ok {
				continue
			}
			eventType := firstNonEmpty(strings.TrimSpace(event.Event), strings.TrimSpace(toStringValue(data["type"])))
			itemMap, _ := data["item"].(map[string]any)
			switch eventType {
			case "response.output_item.added", "response.output_item.done":
				itemType := strings.TrimSpace(toStringValue(itemMap["type"]))
				switch itemType {
				case "function_call":
					appendTool(toStringValue(itemMap["name"]), stringifyJSON(itemMap["arguments"]))
				case "web_search_call":
					appendTool("web_search_call", stringifyJSON(itemMap["action"]))
				}
			case "response.function_call_arguments.done":
				appendTool(firstNonEmpty(toStringValue(data["name"]), toStringValue(itemMap["name"])), firstNonEmpty(stringifyJSON(data["arguments"]), stringifyJSON(itemMap["arguments"])))
			case "response.output_text.delta", "response.refusal.delta":
				appendText(firstNonEmptyExact(toStringValue(data["delta"]), toStringValue(data["text"])))
			case "response.completed":
				responseMap, _ := data["response"].(map[string]any)
				for _, rawItem := range anySliceValue(responseMap["output"]) {
					outputItem, _ := rawItem.(map[string]any)
					itemType := strings.TrimSpace(toStringValue(outputItem["type"]))
					switch itemType {
					case "function_call":
						appendTool(toStringValue(outputItem["name"]), stringifyJSON(outputItem["arguments"]))
					case "web_search_call":
						appendTool("web_search_call", stringifyJSON(outputItem["action"]))
					case "message":
						if content, ok := outputItem["content"].([]any); ok {
							for _, rawContent := range content {
								contentMap, _ := rawContent.(map[string]any)
								appendText(firstNonEmptyExact(toStringValue(contentMap["text"]), toStringValue(contentMap["content"])))
							}
						}
					}
				}
			}
		}
	default:
		for _, event := range events {
			data, ok := advancedProxySSEJSONPayload(event)
			if !ok {
				continue
			}
			choices, _ := data["choices"].([]any)
			for _, rawChoice := range choices {
				choiceMap, _ := rawChoice.(map[string]any)
				delta, _ := choiceMap["delta"].(map[string]any)
				if delta == nil {
					continue
				}
				appendText(toStringValue(delta["content"]))
				if toolCalls, ok := delta["tool_calls"].([]any); ok {
					for _, rawCall := range toolCalls {
						callMap, _ := rawCall.(map[string]any)
						functionMap, _ := callMap["function"].(map[string]any)
						appendTool(toStringValue(functionMap["name"]), toStringValue(functionMap["arguments"]))
					}
				}
			}
		}
	}
	return normalizeAdvancedProxyPreviewList(toolNames, 24, 160), normalizeAdvancedProxyPreviewList(toolArgs, 24, 280), previewAdvancedProxyText(strings.Join(textParts, " "), 800), latest
}

func (s *advancedProxyEncryptedContentHealStore) get(sessionKey string) int {
	if strings.TrimSpace(sessionKey) == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionKey]
}

func (s *advancedProxyEncryptedContentHealStore) record(sessionKey string, encryptedCount int) int {
	if strings.TrimSpace(sessionKey) == "" || encryptedCount <= 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.sessions[sessionKey]
	if encryptedCount > previous {
		s.sessions[sessionKey] = encryptedCount
		return encryptedCount
	}
	return previous
}

func appendEncryptedContentHealingNotice(message string) string {
	resolved := strings.TrimSpace(message)
	if strings.Contains(resolved, encryptedContentHealingNotice) {
		return resolved
	}
	if resolved == "" {
		return encryptedContentHealingNotice
	}
	return resolved + " " + encryptedContentHealingNotice
}

func parseEmbeddedJSONObject(raw string) map[string]any {
	text := strings.TrimSpace(raw)
	if text == "" || (!strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[")) {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return nil
	}
	return decoded
}

func extractEncryptedContentHealingSessionKey(body map[string]any, appType string) string {
	if body == nil {
		return ""
	}

	sessionKeys := []string{
		"session_id",
		"sessionId",
		"conversation_id",
		"conversationId",
		"thread_id",
		"threadId",
		"resume_id",
		"resumeId",
		"prompt_cache_key",
		"promptCacheKey",
		"previous_response_id",
		"previousResponseId",
		"response_id",
		"responseId",
		"x-codex-session-id",
		"x-codex-conversation-id",
	}
	fingerprintKeys := []string{
		"x-codex-installation-id",
		"installation_id",
		"installationId",
	}

	var search func(any) string
	search = func(value any) string {
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range sessionKeys {
				if candidate := strings.TrimSpace(toStringValue(typed[key])); candidate != "" {
					return candidate
				}
			}
			for _, key := range []string{"metadata", "user", "context", "client", "client_metadata"} {
				if candidate := search(typed[key]); candidate != "" {
					return candidate
				}
			}
			for _, key := range orderedJSONMapKeys(typed) {
				if key == "metadata" || key == "user" || key == "context" || key == "client" || key == "client_metadata" {
					continue
				}
				if candidate := search(typed[key]); candidate != "" {
					return candidate
				}
			}
		case []any:
			for _, item := range typed {
				if candidate := search(item); candidate != "" {
					return candidate
				}
			}
		case string:
			if decoded := parseEmbeddedJSONObject(typed); decoded != nil {
				return search(decoded)
			}
		}
		return ""
	}

	sessionID := search(body)
	if sessionID == "" {
		fingerprint := firstNonEmpty(
			searchMapStringKey(body, fingerprintKeys...),
			strings.TrimSpace(toStringValue(body["prompt_cache_key"])),
		)
		if fingerprint == "" {
			return ""
		}
		textFingerprint := previewAdvancedProxyText(firstNonEmpty(
			strings.TrimSpace(toStringValue(body["instructions"])),
			extractStableEncryptedContentPromptFingerprint(body),
			strings.TrimSpace(toStringValue(body["model"])),
		), 200)
		digest := sha1.Sum([]byte(strings.TrimSpace(appType) + "\n" + fingerprint + "\n" + textFingerprint))
		sessionID = fmt.Sprintf("fallback:%x", digest[:8])
	}
	return strings.TrimSpace(appType) + "|" + sessionID
}

func searchMapStringKey(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if candidate := strings.TrimSpace(toStringValue(typed[key])); candidate != "" {
				return candidate
			}
		}
		for _, item := range typed {
			if candidate := searchMapStringKey(item, keys...); candidate != "" {
				return candidate
			}
		}
	case []any:
		for _, item := range typed {
			if candidate := searchMapStringKey(item, keys...); candidate != "" {
				return candidate
			}
		}
	case string:
		if decoded := parseEmbeddedJSONObject(typed); decoded != nil {
			return searchMapStringKey(decoded, keys...)
		}
	}
	return ""
}

func extractStableEncryptedContentPromptFingerprint(body map[string]any) string {
	if body == nil {
		return ""
	}
	inputItems, _ := body["input"].([]any)
	if len(inputItems) == 0 {
		return ""
	}
	parts := make([]string, 0, 8)
	for _, rawItem := range inputItems {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(toStringValue(itemMap["role"]))
		itemType := strings.TrimSpace(toStringValue(itemMap["type"]))
		text := previewAdvancedProxyText(firstNonEmpty(
			openAIMessageContentToText(itemMap["content"]),
			strings.TrimSpace(toStringValue(itemMap["text"])),
		), 120)
		if role == "" && itemType == "" && text == "" {
			continue
		}
		parts = append(parts, role+"|"+itemType+"|"+text)
		if len(parts) >= 6 {
			break
		}
	}
	return strings.Join(parts, "\n")
}

func orderedJSONMapKeys(source map[string]any) []string {
	if len(source) == 0 {
		return nil
	}

	prioritized := []string{
		"metadata",
		"input",
		"messages",
		"content",
		"items",
		"output",
		"previous_response",
		"conversation",
		"history",
		"encrypted_content",
	}
	keys := make([]string, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, key := range prioritized {
		if _, exists := source[key]; exists {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	rest := make([]string, 0, len(source)-len(keys))
	for key := range source {
		if _, exists := seen[key]; exists {
			continue
		}
		rest = append(rest, key)
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func containsEncryptedContentNeedle(rawBody []byte) bool {
	return len(rawBody) > 0 && encryptedContentNeedlePattern.Match(rawBody)
}

func countEncryptedContentNeedle(rawBody []byte) int {
	if len(rawBody) == 0 {
		return 0
	}
	return len(encryptedContentNeedlePattern.FindAllIndex(rawBody, -1))
}

func scrubEncryptedContentString(value string) (string, int) {
	if strings.TrimSpace(value) == "" {
		return value, 0
	}
	indexes := encryptedContentNeedlePattern.FindAllStringIndex(value, -1)
	if len(indexes) == 0 {
		return value, 0
	}
	return encryptedContentNeedlePattern.ReplaceAllString(value, "stripped_content"), len(indexes)
}

func countEncryptedContentEntries(value any) int {
	count := 0
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			if encrypted := strings.TrimSpace(toStringValue(typed["encrypted_content"])); encrypted != "" {
				count += 1
			}
			for _, key := range orderedJSONMapKeys(typed) {
				if key == "encrypted_content" {
					continue
				}
				walk(typed[key])
			}
		}
	}
	walk(value)
	return count
}

func stripHistoricalEncryptedContent(value any, remaining *int) int {
	if remaining == nil || *remaining <= 0 {
		return 0
	}

	stripped := 0
	var walk func(any)
	walk = func(node any) {
		if *remaining <= 0 {
			return
		}
		switch typed := node.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
				if *remaining <= 0 {
					return
				}
			}
		case map[string]any:
			if encrypted := strings.TrimSpace(toStringValue(typed["encrypted_content"])); encrypted != "" && *remaining > 0 {
				delete(typed, "encrypted_content")
				*remaining -= 1
				stripped += 1
			}
			for _, key := range orderedJSONMapKeys(typed) {
				if key == "encrypted_content" {
					continue
				}
				walk(typed[key])
				if *remaining <= 0 {
					return
				}
			}
		}
	}
	walk(value)
	return stripped
}

func stripEncryptedContentIncludeReferences(value any) int {
	removed := 0
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			if includeItems, ok := typed["include"].([]any); ok && len(includeItems) > 0 {
				filtered := make([]any, 0, len(includeItems))
				for _, rawItem := range includeItems {
					itemText := strings.TrimSpace(toStringValue(rawItem))
					if itemText != "" && strings.Contains(strings.ToLower(itemText), "encrypted_content") {
						removed += 1
						continue
					}
					filtered = append(filtered, rawItem)
				}
				typed["include"] = filtered
			}
			for _, key := range orderedJSONMapKeys(typed) {
				if key == "include" || key == "encrypted_content" {
					continue
				}
				walk(typed[key])
			}
		}
	}
	walk(value)
	return removed
}

func stripAllEncryptedContentForHealedSession(value any) encryptedContentFinalSanitizationStats {
	stats := encryptedContentFinalSanitizationStats{}
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case []any:
			for index, item := range typed {
				if text, ok := item.(string); ok {
					scrubbed, replaced := scrubEncryptedContentString(text)
					if replaced > 0 {
						typed[index] = scrubbed
						stats.ScrubbedStrings += replaced
					}
					continue
				}
				walk(item)
			}
		case map[string]any:
			for key := range typed {
				if strings.EqualFold(strings.TrimSpace(key), "encrypted_content") {
					delete(typed, key)
					stats.RemovedFields += 1
				}
			}
			if includeItems, ok := typed["include"].([]any); ok && len(includeItems) > 0 {
				filtered := make([]any, 0, len(includeItems))
				for _, rawItem := range includeItems {
					itemText := strings.TrimSpace(toStringValue(rawItem))
					if itemText != "" && strings.Contains(strings.ToLower(itemText), "encrypted_content") {
						stats.RemovedIncludeRefs += 1
						continue
					}
					filtered = append(filtered, rawItem)
				}
				typed["include"] = filtered
			}
			for _, key := range orderedJSONMapKeys(typed) {
				value := typed[key]
				if text, ok := value.(string); ok {
					scrubbed, replaced := scrubEncryptedContentString(text)
					if replaced > 0 {
						typed[key] = scrubbed
						stats.ScrubbedStrings += replaced
					}
					continue
				}
				walk(value)
			}
		}
	}
	walk(value)
	return stats
}

func finalizeOpenAIRequestForEncryptedContentHealing(rawBody []byte, sessionKey string) ([]byte, encryptedContentFinalSanitizationStats, error) {
	stats := encryptedContentFinalSanitizationStats{}
	if strings.TrimSpace(sessionKey) == "" || advancedProxyEncryptedContentHealState.get(sessionKey) <= 0 {
		return rawBody, stats, nil
	}
	if !containsEncryptedContentNeedle(rawBody) {
		return rawBody, stats, nil
	}

	var requestBody map[string]any
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		return rawBody, stats, err
	}

	stats = stripAllEncryptedContentForHealedSession(requestBody)
	sanitizedBody, err := json.Marshal(requestBody)
	if err != nil {
		return rawBody, stats, err
	}
	stats.ResidualHits = countEncryptedContentNeedle(sanitizedBody)
	return sanitizedBody, stats, nil
}

func prepareOpenAIRequestForEncryptedContentHealing(rawBody []byte, appType string) ([]byte, encryptedContentHealingContext, error) {
	context := encryptedContentHealingContext{}
	if len(rawBody) == 0 {
		return rawBody, context, nil
	}
	if !containsEncryptedContentNeedle(rawBody) {
		return rawBody, context, nil
	}

	var requestBody map[string]any
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		return rawBody, context, err
	}

	context.SessionKey = extractEncryptedContentHealingSessionKey(requestBody, appType)
	context.OriginalCount = countEncryptedContentEntries(requestBody)
	if context.SessionKey == "" {
		return rawBody, context, nil
	}

	historicalCut := advancedProxyEncryptedContentHealState.get(context.SessionKey)
	if historicalCut <= 0 {
		return rawBody, context, nil
	}

	remaining := historicalCut
	context.AppliedHistoricalCut = stripHistoricalEncryptedContent(requestBody, &remaining)
	context.RemovedIncludeRefs = stripEncryptedContentIncludeReferences(requestBody)
	if context.AppliedHistoricalCut <= 0 && context.RemovedIncludeRefs <= 0 {
		return rawBody, context, nil
	}

	sanitizedBody, err := json.Marshal(requestBody)
	if err != nil {
		return rawBody, context, err
	}
	return sanitizedBody, context, nil
}

func isInvalidEncryptedContentError(statusCode int, body []byte) (string, string, string, bool) {
	if statusCode >= 200 && statusCode < 300 || len(body) == 0 {
		return "", "", "", false
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		message := normalizeAnthropicErrorMessage(body)
		lower := strings.ToLower(message)
		if strings.Contains(lower, "encrypted content") && (strings.Contains(lower, "could not be verified") || strings.Contains(lower, "decrypted") || strings.Contains(lower, "parsed")) {
			return message, "invalid_encrypted_content", "invalid_request_error", true
		}
		return "", "", "", false
	}

	message := firstNonEmpty(
		getNestedString(decoded, "error", "message"),
		strings.TrimSpace(toStringValue(decoded["message"])),
	)
	code := firstNonEmpty(
		getNestedString(decoded, "error", "code"),
		strings.TrimSpace(toStringValue(decoded["code"])),
	)
	errorType := firstNonEmpty(
		getNestedString(decoded, "error", "type"),
		strings.TrimSpace(toStringValue(decoded["type"])),
	)
	lowerMessage := strings.ToLower(strings.TrimSpace(message))
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	if lowerCode == "invalid_encrypted_content" {
		return message, code, firstNonEmpty(errorType, "invalid_request_error"), true
	}
	if strings.Contains(lowerMessage, "encrypted content") && (strings.Contains(lowerMessage, "could not be verified") || strings.Contains(lowerMessage, "decrypted") || strings.Contains(lowerMessage, "parsed")) {
		return message, firstNonEmpty(code, "invalid_encrypted_content"), firstNonEmpty(errorType, "invalid_request_error"), true
	}
	return "", "", "", false
}

func advancedProxyProviderLabel(provider AdvancedProxyProvider) string {
	return firstNonEmpty(
		strings.TrimSpace(provider.Name),
		strings.TrimSpace(provider.ID),
		strings.TrimSpace(provider.BaseURL),
		"unknown-provider",
	)
}

func summarizeAdvancedProxyBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return previewAdvancedProxyText(normalizeAnthropicErrorMessage(raw), 220)
}

func appendAdvancedProxyRouteTraceStep(trace []AdvancedProxyRequestRouteStep, route string, source string, status string) []AdvancedProxyRequestRouteStep {
	route = strings.TrimSpace(route)
	if route == "" {
		return cloneAdvancedProxyRouteTrace(trace)
	}
	next := cloneAdvancedProxyRouteTrace(trace)
	step := AdvancedProxyRequestRouteStep{
		Route:  route,
		Source: strings.TrimSpace(strings.ToLower(source)),
		Status: strings.TrimSpace(strings.ToLower(status)),
	}
	if len(next) > 0 {
		last := next[len(next)-1]
		if last.Route == step.Route && last.Source == step.Source && last.Status == step.Status {
			return next
		}
	}
	return append(next, step)
}

func shouldFallbackClaudeResponsesToOpenAIChat(statusCode int, responseBody []byte, features claudeRequestFeatures) bool {
	if features.HasAnthropicWebSearchTool {
		return false
	}
	return shouldFallbackResponsesToChat(statusCode, responseBody)
}

func shouldFallbackClaudeOpenAIChatToResponses(statusCode int, responseBody []byte, features claudeRequestFeatures) bool {
	if features.HasAnthropicWebSearchTool {
		return false
	}
	return shouldFallbackChatPreferenceBackToResponses(statusCode, responseBody)
}

func shouldAdvanceClaudeProxyPhase(current claudeProxyAttemptPhase, next claudeProxyAttemptPhase, statusCode int, responseBody []byte, features claudeRequestFeatures) bool {
	switch current.apiFormat {
	case "anthropic":
		return next.apiFormat != "anthropic" && shouldFallbackClaudeMessagesToOpenAIRoute(statusCode, responseBody)
	case "openai_responses":
		if next.apiFormat == "anthropic" {
			return shouldFallbackResponsesToChat(statusCode, responseBody)
		}
		return next.apiFormat != "openai_responses" && shouldFallbackClaudeResponsesToOpenAIChat(statusCode, responseBody, features)
	case "openai_chat":
		if next.apiFormat == "anthropic" {
			return shouldFallbackChatPreferenceBackToResponses(statusCode, responseBody)
		}
		return next.apiFormat != "openai_chat" && shouldFallbackClaudeOpenAIChatToResponses(statusCode, responseBody, features)
	default:
		return false
	}
}

type claudeProxyAttemptPhase struct {
	apiFormat          string
	routeKind          string
	source             string
	preferenceValue    int
	preferenceScopeKey string
}

func buildClaudeProxyAttemptPhases(provider AdvancedProxyProvider, requestBody map[string]any, features claudeRequestFeatures) []claudeProxyAttemptPhase {
	routeKindForFormat := func(apiFormat string) string {
		switch apiFormat {
		case "openai_chat":
			return "chat"
		case "openai_responses":
			return "responses"
		default:
			return "messages"
		}
	}

	appendPhase := func(phases []claudeProxyAttemptPhase, apiFormat string, source string, preferenceValue int, preferenceScopeKey string) []claudeProxyAttemptPhase {
		apiFormat = normalizeClaudeAPIFormat(apiFormat)
		if apiFormat == "" {
			apiFormat = "anthropic"
		}
		for _, existing := range phases {
			if existing.apiFormat == apiFormat {
				return phases
			}
		}
		return append(phases, claudeProxyAttemptPhase{
			apiFormat:          apiFormat,
			routeKind:          routeKindForFormat(apiFormat),
			source:             source,
			preferenceValue:    preferenceValue,
			preferenceScopeKey: strings.TrimSpace(preferenceScopeKey),
		})
	}

	model := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(requestBody["model"])))
	scopeKey := resolveAdvancedProxyClaudeProtocolPreferenceScopeKey(provider, model)

	if features.HasAnthropicWebSearchTool {
		if preferenceValue, ok := getAdvancedProxyClaudeProtocolPreference(scopeKey); ok && preferenceValue == advancedProxyClaudeProtocolPreferResponses {
			phases := appendPhase(nil, "openai_responses", "preference", advancedProxyClaudeProtocolPreferResponses, scopeKey)
			return appendPhase(phases, "anthropic", "fallback_restore", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
		}
		phases := appendPhase(nil, "anthropic", "original", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
		return appendPhase(phases, "openai_responses", "fallback", advancedProxyClaudeProtocolPreferResponses, scopeKey)
	}

	if preferenceValue, ok := getAdvancedProxyClaudeProtocolPreference(scopeKey); ok {
		switch preferenceValue {
		case advancedProxyClaudeProtocolPreferResponses:
			phases := appendPhase(nil, "openai_responses", "preference", advancedProxyClaudeProtocolPreferResponses, scopeKey)
			phases = appendPhase(phases, "anthropic", "fallback_restore", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
			return appendPhase(phases, "openai_chat", "fallback_restore", advancedProxyClaudeProtocolPreferChat, scopeKey)
		case advancedProxyClaudeProtocolPreferChat:
			phases := appendPhase(nil, "openai_chat", "preference", advancedProxyClaudeProtocolPreferChat, scopeKey)
			phases = appendPhase(phases, "anthropic", "fallback_restore", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
			return appendPhase(phases, "openai_responses", "fallback_restore", advancedProxyClaudeProtocolPreferResponses, scopeKey)
		default:
			phases := appendPhase(nil, "anthropic", "preference", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
			phases = appendPhase(phases, "openai_responses", "fallback_restore", advancedProxyClaudeProtocolPreferResponses, scopeKey)
			return appendPhase(phases, "openai_chat", "fallback_restore", advancedProxyClaudeProtocolPreferChat, scopeKey)
		}
	}

	switch normalizeClaudeAPIFormat(provider.APIFormat) {
	case "openai_responses":
		phases := appendPhase(nil, "openai_responses", "provider_config", advancedProxyClaudeProtocolPreferResponses, scopeKey)
		phases = appendPhase(phases, "anthropic", "fallback", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
		return appendPhase(phases, "openai_chat", "fallback_secondary", advancedProxyClaudeProtocolPreferChat, scopeKey)
	case "openai_chat":
		phases := appendPhase(nil, "openai_chat", "provider_config", advancedProxyClaudeProtocolPreferChat, scopeKey)
		phases = appendPhase(phases, "anthropic", "fallback", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
		return appendPhase(phases, "openai_responses", "fallback_secondary", advancedProxyClaudeProtocolPreferResponses, scopeKey)
	}

	phases := appendPhase(nil, "anthropic", "original", advancedProxyClaudeProtocolPreferAnthropic, scopeKey)
	phases = appendPhase(phases, "openai_responses", "fallback", advancedProxyClaudeProtocolPreferResponses, scopeKey)
	return appendPhase(phases, "openai_chat", "fallback_secondary", advancedProxyClaudeProtocolPreferChat, scopeKey)
}

func describeOutboundProxyMode() string {
	config, err := loadOutboundProxyConfig()
	if err != nil {
		return "unknown"
	}
	switch strings.ToLower(strings.TrimSpace(config.Mode)) {
	case outboundProxyModeDirect:
		return "direct"
	case outboundProxyModeCustom:
		return "custom"
	default:
		return "system"
	}
}

func formatAdvancedProxyFailure(appType string, routeKind string, provider AdvancedProxyProvider, targetURL string, detail string) string {
	message := firstNonEmpty(strings.TrimSpace(detail), "advanced proxy request failed")
	parts := []string{
		fmt.Sprintf("app=%s", strings.TrimSpace(appType)),
		fmt.Sprintf("route=%s", strings.TrimSpace(routeKind)),
		fmt.Sprintf("provider=%s", advancedProxyProviderLabel(provider)),
	}
	if strings.TrimSpace(targetURL) != "" {
		parts = append(parts, fmt.Sprintf("endpoint=%s", strings.TrimSpace(targetURL)))
	}
	return strings.Join(parts, " | ") + " | " + message
}

func firstNonEmptyExact(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func openAIMessageContentToText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			contentMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			text := firstNonEmptyExact(
				toStringValue(contentMap["text"]),
				toStringValue(contentMap["content"]),
				toStringValue(contentMap["refusal"]),
			)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func openAIMessageThinkingToText(message map[string]any) string {
	if message == nil {
		return ""
	}
	return firstNonEmpty(
		openAIMessageContentToText(message["reasoning_content"]),
		openAIMessageContentToText(message["thinking"]),
	)
}

func mapOpenAIStopReason(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tool_calls", "function_call":
		return "tool_use"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func mapOpenAIResponsesStopReason(status string, hasToolUse bool, incompleteReason string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		if hasToolUse {
			return "tool_use"
		}
		return "end_turn"
	case "incomplete":
		switch strings.ToLower(strings.TrimSpace(incompleteReason)) {
		case "", "max_output_tokens", "max_tokens":
			return "max_tokens"
		default:
			return "end_turn"
		}
	default:
		return "end_turn"
	}
}

func appendToolArgumentsParseFailure(contentBlocks *[]map[string]any, toolName string, rawArguments any, parseErr error) {
	if contentBlocks == nil || parseErr == nil {
		return
	}
	resolvedToolName := firstNonEmpty(strings.TrimSpace(toolName), "unknown_tool")
	preview := previewAdvancedProxyText(stringifyJSON(rawArguments), 240)
	message := fmt.Sprintf("Tool `%s` arguments were invalid and were skipped. Please retry the action.", resolvedToolName)
	if preview != "" {
		message += "\n\nArguments preview: " + preview
	}
	*contentBlocks = append(*contentBlocks, map[string]any{
		"type": "text",
		"text": message,
	})
	appendAdvancedProxyLogf(
		"[CLAUDE_PROXY_TOOL_ARGUMENTS_INVALID] tool=%s reason=%s arguments=%s",
		resolvedToolName,
		parseErr.Error(),
		preview,
	)
}

func openAIUsageToAnthropic(response map[string]any) map[string]any {
	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	usageMap, ok := response["usage"].(map[string]any)
	if !ok || usageMap == nil {
		return usage
	}

	usage["input_tokens"] = toIntValue(usageMap["prompt_tokens"])
	usage["output_tokens"] = toIntValue(usageMap["completion_tokens"])

	if promptDetails, ok := usageMap["prompt_tokens_details"].(map[string]any); ok {
		if cached := toIntValue(promptDetails["cached_tokens"]); cached > 0 {
			usage["cache_read_input_tokens"] = cached
		}
	}
	if cachedRead := toIntValue(usageMap["cache_read_input_tokens"]); cachedRead > 0 {
		usage["cache_read_input_tokens"] = cachedRead
	}
	if cacheCreated := toIntValue(usageMap["cache_creation_input_tokens"]); cacheCreated > 0 {
		usage["cache_creation_input_tokens"] = cacheCreated
	}
	return usage
}

func openAIChatToAnthropic(response map[string]any, fallbackModel string, includeThinking bool) map[string]any {
	choices, _ := response["choices"].([]any)
	message := map[string]any{}
	finishReason := "end_turn"
	if len(choices) > 0 {
		choiceMap, _ := choices[0].(map[string]any)
		if choiceMap != nil {
			if finish := strings.TrimSpace(toStringValue(choiceMap["finish_reason"])); finish != "" {
				finishReason = mapOpenAIStopReason(finish)
			}
			message, _ = choiceMap["message"].(map[string]any)
		}
	}

	contentBlocks := make([]map[string]any, 0, 2)
	thinkingContent := openAIMessageThinkingToText(message)
	if includeThinking && thinkingContent != "" {
		contentBlocks = append(contentBlocks, map[string]any{
			"type":     "thinking",
			"thinking": thinkingContent,
		})
	}
	textContent := openAIMessageContentToText(message["content"])
	if textContent != "" {
		contentBlocks = append(contentBlocks, map[string]any{
			"type": "text",
			"text": textContent,
		})
	}
	if toolCalls, ok := message["tool_calls"].([]any); ok {
		for _, rawToolCall := range toolCalls {
			toolCallMap, ok := rawToolCall.(map[string]any)
			if !ok {
				continue
			}
			functionMap, _ := toolCallMap["function"].(map[string]any)
			toolName := strings.TrimSpace(toStringValue(functionMap["name"]))
			toolInput, parseErr := parseToolInputMap(functionMap["arguments"])
			if parseErr != nil {
				appendToolArgumentsParseFailure(&contentBlocks, toolName, functionMap["arguments"], parseErr)
				continue
			}
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "tool_use",
				"id":    firstNonEmpty(strings.TrimSpace(toStringValue(toolCallMap["id"])), fmt.Sprintf("tool_%d", len(contentBlocks)+1)),
				"name":  toolName,
				"input": toolInput,
			})
		}
	}
	if functionMap, ok := message["function_call"].(map[string]any); ok && functionMap != nil {
		toolName := strings.TrimSpace(toStringValue(functionMap["name"]))
		toolInput, parseErr := parseToolInputMap(functionMap["arguments"])
		if parseErr != nil {
			appendToolArgumentsParseFailure(&contentBlocks, toolName, functionMap["arguments"], parseErr)
		} else {
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "tool_use",
				"id":    fmt.Sprintf("tool_%d", len(contentBlocks)+1),
				"name":  toolName,
				"input": toolInput,
			})
		}
	}
	if finishReason == "tool_use" {
		hasToolUse := false
		for _, block := range contentBlocks {
			if strings.TrimSpace(toStringValue(block["type"])) == "tool_use" {
				hasToolUse = true
				break
			}
		}
		if !hasToolUse {
			finishReason = "end_turn"
		}
	}
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": ""})
	}

	model := strings.TrimSpace(toStringValue(response["model"]))
	if model == "" {
		model = fallbackModel
	}
	return map[string]any{
		"id":            firstNonEmpty(strings.TrimSpace(toStringValue(response["id"])), fmt.Sprintf("msg_%d", time.Now().UnixNano())),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   finishReason,
		"stop_sequence": nil,
		"usage":         openAIUsageToAnthropic(response),
	}
}

func openAIResponsesToAnthropic(response map[string]any, fallbackModel string) map[string]any {
	contentBlocks := make([]map[string]any, 0, 2)
	hasToolUse := false
	webSearchRequests := 0
	annotationResultContents := extractResponsesAnnotatedWebSearchResultContents(response["output"])
	textResultContents := extractResponsesTextWebSearchResultContents(response["output"])
	annotationResultIndex := 0
	textResultIndex := 0
	if outputText := toStringValue(response["output_text"]); outputText != "" {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": outputText})
	}
	if outputs, ok := response["output"].([]any); ok {
		for _, rawOutput := range outputs {
			outputMap, ok := rawOutput.(map[string]any)
			if !ok {
				continue
			}
			switch strings.TrimSpace(toStringValue(outputMap["type"])) {
			case "message":
				if contents, ok := outputMap["content"].([]any); ok {
					for _, rawContent := range contents {
						contentMap, ok := rawContent.(map[string]any)
						if !ok {
							continue
						}
						contentType := strings.TrimSpace(toStringValue(contentMap["type"]))
						switch contentType {
						case "output_text", "text":
							text := toStringValue(contentMap["text"])
							if text != "" {
								textBlock := map[string]any{"type": "text", "text": text}
								if citations := buildAnthropicWebSearchCitations(text, contentMap["annotations"]); len(citations) > 0 {
									textBlock["citations"] = citations
								}
								contentBlocks = append(contentBlocks, textBlock)
							}
						case "refusal":
							text := toStringValue(contentMap["refusal"])
							if text != "" {
								contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": text})
							}
						}
					}
				}
			case "function_call":
				toolName := strings.TrimSpace(toStringValue(outputMap["name"]))
				toolInput, parseErr := parseToolInputMap(outputMap["arguments"])
				if parseErr != nil {
					appendToolArgumentsParseFailure(&contentBlocks, toolName, outputMap["arguments"], parseErr)
					continue
				}
				hasToolUse = true
				contentBlocks = append(contentBlocks, map[string]any{
					"type":  "tool_use",
					"id":    firstNonEmpty(strings.TrimSpace(toStringValue(outputMap["call_id"])), fmt.Sprintf("tool_%d", len(contentBlocks)+1)),
					"name":  toolName,
					"input": toolInput,
				})
			case "web_search_call":
				webSearchRequests++
				toolUseID := normalizeAnthropicServerToolUseID(
					strings.TrimSpace(toStringValue(outputMap["id"])),
					webSearchRequests,
				)
				if input := buildAnthropicWebSearchInput(outputMap); len(input) > 0 {
					contentBlocks = append(contentBlocks, map[string]any{
						"type":  "server_tool_use",
						"id":    toolUseID,
						"name":  "web_search",
						"input": input,
					})
				}
				result := buildAnthropicWebSearchResultBlock(toolUseID, outputMap)
				if result == nil && annotationResultIndex < len(annotationResultContents) {
					result = buildAnthropicWebSearchResultBlockFromContent(toolUseID, annotationResultContents[annotationResultIndex])
					annotationResultIndex++
				}
				if result == nil && textResultIndex < len(textResultContents) {
					result = buildAnthropicWebSearchResultBlockFromContent(toolUseID, textResultContents[textResultIndex])
					textResultIndex++
				}
				if result != nil {
					contentBlocks = append(contentBlocks, result)
				}
			}
		}
	}
	if webSearchRequests == 0 && len(annotationResultContents) > 0 {
		for _, annotationResultContent := range annotationResultContents {
			webSearchRequests++
			toolUseID := normalizeAnthropicServerToolUseID("", webSearchRequests)
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "server_tool_use",
				"id":    toolUseID,
				"name":  "web_search",
				"input": map[string]any{},
			})
			if result := buildAnthropicWebSearchResultBlockFromContent(toolUseID, annotationResultContent); result != nil {
				contentBlocks = append(contentBlocks, result)
			}
		}
	} else if webSearchRequests == 0 && len(textResultContents) > 0 {
		for _, textResultContent := range textResultContents {
			webSearchRequests++
			toolUseID := normalizeAnthropicServerToolUseID("", webSearchRequests)
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "server_tool_use",
				"id":    toolUseID,
				"name":  "web_search",
				"input": map[string]any{},
			})
			if result := buildAnthropicWebSearchResultBlockFromContent(toolUseID, textResultContent); result != nil {
				contentBlocks = append(contentBlocks, result)
			}
		}
	}
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": ""})
	}
	usage := map[string]any{}
	if usageMap, ok := response["usage"].(map[string]any); ok {
		usage["input_tokens"] = toIntValue(usageMap["input_tokens"])
		usage["output_tokens"] = toIntValue(usageMap["output_tokens"])
	}
	if webSearchRequests > 0 {
		usage["server_tool_use"] = map[string]any{
			"web_search_requests": webSearchRequests,
		}
	}
	incompleteReason := ""
	if incompleteMap, ok := response["incomplete_details"].(map[string]any); ok {
		incompleteReason = toStringValue(incompleteMap["reason"])
	}
	stopReason := mapOpenAIResponsesStopReason(
		toStringValue(response["status"]),
		hasToolUse,
		incompleteReason,
	)
	model := strings.TrimSpace(toStringValue(response["model"]))
	if model == "" {
		model = fallbackModel
	}
	if webSearchRequests > 0 || len(annotationResultContents) > 0 || len(textResultContents) > 0 {
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_WEB_SEARCH_NONSTREAM] response_id=%s web_search_requests=%d annotation_result_sets=%d text_result_sets=%d content_blocks=%d",
			firstNonEmpty(strings.TrimSpace(toStringValue(response["id"])), "unknown"),
			webSearchRequests,
			len(annotationResultContents),
			len(textResultContents),
			len(contentBlocks),
		)
	}
	return map[string]any{
		"id":            firstNonEmpty(strings.TrimSpace(toStringValue(response["id"])), fmt.Sprintf("msg_%d", time.Now().UnixNano())),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         usage,
	}
}

func buildAnthropicWebSearchInput(webSearchCall map[string]any) map[string]any {
	actionMap, _ := webSearchCall["action"].(map[string]any)
	if actionMap == nil {
		return nil
	}
	query := strings.TrimSpace(toStringValue(actionMap["query"]))
	if query == "" {
		if queries, ok := actionMap["queries"].([]any); ok {
			for _, rawQuery := range queries {
				query = strings.TrimSpace(toStringValue(rawQuery))
				if query != "" {
					break
				}
			}
		}
	}
	if query == "" {
		return nil
	}
	return map[string]any{"query": query}
}

func buildAnthropicWebSearchResultBlock(toolUseID string, webSearchCall map[string]any) map[string]any {
	content := buildAnthropicWebSearchResultContent(webSearchCall)
	return buildAnthropicWebSearchResultBlockFromContent(toolUseID, content)
}

func buildAnthropicWebSearchResultBlockFromContent(toolUseID string, content any) map[string]any {
	if content == nil {
		return nil
	}
	return map[string]any{
		"type":        "web_search_tool_result",
		"tool_use_id": toolUseID,
		"content":     content,
	}
}

func buildAnthropicWebSearchResultContent(webSearchCall map[string]any) any {
	actionMap, _ := webSearchCall["action"].(map[string]any)
	if actionMap == nil {
		return nil
	}
	sources, ok := actionMap["sources"].([]any)
	if !ok || len(sources) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(sources))
	for _, rawSource := range sources {
		sourceMap, ok := rawSource.(map[string]any)
		if !ok {
			continue
		}
		url := strings.TrimSpace(toStringValue(sourceMap["url"]))
		if url == "" {
			continue
		}
		result := map[string]any{
			"type": "web_search_result",
			"url":  url,
		}
		if title := strings.TrimSpace(toStringValue(sourceMap["title"])); title != "" {
			result["title"] = title
		}
		if pageAge := strings.TrimSpace(toStringValue(sourceMap["page_age"])); pageAge != "" {
			result["page_age"] = pageAge
		}
		if encrypted := strings.TrimSpace(toStringValue(sourceMap["encrypted_content"])); encrypted != "" {
			result["encrypted_content"] = encrypted
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func buildAnthropicWebSearchResultContentFromAnnotations(annotations any) any {
	items, ok := annotations.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(toStringValue(itemMap["type"])) != "url_citation" {
			continue
		}
		url := strings.TrimSpace(toStringValue(itemMap["url"]))
		if url == "" {
			continue
		}
		if _, exists := seen[url]; exists {
			continue
		}
		seen[url] = struct{}{}
		result := map[string]any{
			"type": "web_search_result",
			"url":  url,
		}
		if title := strings.TrimSpace(toStringValue(itemMap["title"])); title != "" {
			result["title"] = title
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func buildAnthropicWebSearchResultContentFromText(text string) any {
	matches := webSearchResultURLPattern.FindAllString(strings.TrimSpace(text), -1)
	if len(matches) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		url := strings.TrimSpace(strings.TrimRight(match, ".,;:"))
		if url == "" {
			continue
		}
		if _, exists := seen[url]; exists {
			continue
		}
		seen[url] = struct{}{}
		results = append(results, map[string]any{
			"type": "web_search_result",
			"url":  url,
		})
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func countAnthropicWebSearchResults(content any) int {
	switch typed := content.(type) {
	case []map[string]any:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return 0
	}
}

func extractResponsesWebSearchCalls(output any) []map[string]any {
	items, ok := output.([]any)
	if !ok {
		return nil
	}
	results := make([]map[string]any, 0, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(toStringValue(itemMap["type"])) != "web_search_call" {
			continue
		}
		results = append(results, itemMap)
	}
	return results
}

func extractResponsesAnnotatedWebSearchResultContents(output any) []any {
	items, ok := output.([]any)
	if !ok {
		return nil
	}
	results := make([]any, 0, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(toStringValue(itemMap["type"])) != "message" {
			continue
		}
		contentItems, _ := itemMap["content"].([]any)
		for _, rawContent := range contentItems {
			contentMap, ok := rawContent.(map[string]any)
			if !ok {
				continue
			}
			contentType := strings.TrimSpace(toStringValue(contentMap["type"]))
			if contentType != "output_text" && contentType != "text" {
				continue
			}
			if resultContent := buildAnthropicWebSearchResultContentFromAnnotations(contentMap["annotations"]); resultContent != nil {
				results = append(results, resultContent)
			}
		}
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func extractResponsesTextWebSearchResultContents(output any) []any {
	items, ok := output.([]any)
	if !ok {
		return nil
	}
	results := make([]any, 0, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok || strings.TrimSpace(toStringValue(itemMap["type"])) != "message" {
			continue
		}
		contentItems, _ := itemMap["content"].([]any)
		for _, rawContent := range contentItems {
			contentMap, ok := rawContent.(map[string]any)
			if !ok {
				continue
			}
			contentType := strings.TrimSpace(toStringValue(contentMap["type"]))
			if contentType != "output_text" && contentType != "text" {
				continue
			}
			if resultContent := buildAnthropicWebSearchResultContentFromText(toStringValue(contentMap["text"])); resultContent != nil {
				results = append(results, resultContent)
			}
		}
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func buildAnthropicWebSearchCitations(text string, annotations any) []map[string]any {
	items, ok := annotations.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	results := make([]map[string]any, 0, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(toStringValue(itemMap["type"])) != "url_citation" {
			continue
		}
		url := strings.TrimSpace(toStringValue(itemMap["url"]))
		if url == "" {
			continue
		}
		citation := map[string]any{
			"type": "web_search_result_location",
			"url":  url,
		}
		if title := strings.TrimSpace(toStringValue(itemMap["title"])); title != "" {
			citation["title"] = title
		}
		start := toIntValue(itemMap["start_index"])
		end := toIntValue(itemMap["end_index"])
		if start >= 0 && end > start {
			runes := []rune(text)
			if start < len(runes) {
				if end > len(runes) {
					end = len(runes)
				}
				citedText := strings.TrimSpace(string(runes[start:end]))
				if citedText != "" {
					citation["cited_text"] = citedText
				}
			}
		}
		results = append(results, citation)
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func normalizeAnthropicServerToolUseID(raw string, fallbackIndex int) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "srvtoolu_") || strings.HasPrefix(trimmed, "servertoolu_") {
		return trimmed
	}
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, trimmed)
	if sanitized == "" {
		if fallbackIndex < 1 {
			fallbackIndex = 1
		}
		sanitized = fmt.Sprintf("%d", fallbackIndex)
	}
	return "srvtoolu_" + sanitized
}

func performJSONUpstreamRequest(method string, targetURL string, headers map[string]string, payload map[string]any, timeoutSeconds int) (int, http.Header, []byte, time.Duration, error) {
	startedAt := time.Now()
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, nil, time.Since(startedAt), err
	}
	request, err := http.NewRequest(method, targetURL, bytes.NewReader(rawBody))
	if err != nil {
		return 0, nil, nil, time.Since(startedAt), err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		request.Header.Set(key, value)
	}
	client, err := newOutboundHTTPClient(time.Duration(clampInt(timeoutSeconds, 5, 900)) * time.Second)
	if err != nil {
		return 0, nil, nil, time.Since(startedAt), err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, nil, time.Since(startedAt), err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return response.StatusCode, response.Header.Clone(), nil, time.Since(startedAt), err
	}
	return response.StatusCode, response.Header.Clone(), body, time.Since(startedAt), nil
}

func performRawUpstreamRequest(method string, targetURL string, headers map[string]string, rawBody []byte, timeoutSeconds int, keepStream bool) (int, http.Header, []byte, io.ReadCloser, time.Duration, error) {
	startedAt := time.Now()
	request, err := http.NewRequest(method, targetURL, bytes.NewReader(rawBody))
	if err != nil {
		return 0, nil, nil, nil, time.Since(startedAt), err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		request.Header.Set(key, value)
	}
	clientTimeout := time.Duration(clampInt(timeoutSeconds, 5, 900)) * time.Second
	var client *http.Client
	if keepStream {
		client, err = newOutboundStreamingHTTPClient(clientTimeout)
	} else {
		client, err = newOutboundHTTPClient(clientTimeout)
	}
	if err != nil {
		return 0, nil, nil, nil, time.Since(startedAt), err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, nil, nil, time.Since(startedAt), err
	}
	if keepStream && response.StatusCode >= 200 && response.StatusCode < 300 {
		if shouldBufferSuccessfulStreamingUpstreamResponse(response.Header) {
			defer response.Body.Close()
			body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
			if err != nil {
				return response.StatusCode, response.Header.Clone(), nil, nil, time.Since(startedAt), err
			}
			return response.StatusCode, response.Header.Clone(), body, nil, time.Since(startedAt), nil
		}
		return response.StatusCode, response.Header.Clone(), nil, response.Body, time.Since(startedAt), nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return response.StatusCode, response.Header.Clone(), nil, nil, time.Since(startedAt), err
	}
	return response.StatusCode, response.Header.Clone(), body, nil, time.Since(startedAt), nil
}

func shouldBufferSuccessfulStreamingUpstreamResponse(headers http.Header) bool {
	contentType := strings.ToLower(strings.TrimSpace(headers.Get("Content-Type")))
	if contentType == "" || strings.Contains(contentType, "text/event-stream") {
		return false
	}
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/problem+json") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/html")
}

func normalizeAdvancedProxyObservedFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "responses", "responses_compact", "openai_responses":
		return "responses"
	case "chat", "openai_chat":
		return "chat"
	default:
		return ""
	}
}

func (observation *advancedProxyStreamObservation) markFirstOutput(at time.Time) {
	if observation == nil || observation.FirstOutputAt != nil {
		return
	}
	next := at
	observation.FirstOutputAt = &next
}

func (observation *advancedProxyStreamObservation) markCompleted(at time.Time) {
	if observation == nil || !observation.CompletedAt.IsZero() {
		return
	}
	observation.CompletedAt = at
}

func (observation *advancedProxyStreamObservation) updateUsage(inputTokens *int, outputTokens *int) {
	if observation == nil {
		return
	}
	if inputTokens != nil {
		observation.InputTokens = intPtrValue(*inputTokens)
	}
	if outputTokens != nil {
		observation.OutputTokens = intPtrValue(*outputTokens)
	}
}

func recordAdvancedProxyStreamObservation(recordContext *advancedProxyStreamRequestRecordContext, observation advancedProxyStreamObservation, statusCode int, errorDetail string) {
	if recordContext == nil {
		return
	}
	if observation.CompletedAt.IsZero() {
		observation.CompletedAt = time.Now()
	}
	switch strings.ToLower(strings.TrimSpace(recordContext.AppType)) {
	case "claude":
		recordAdvancedProxyClaudeStreamAttemptWithTraceAndOps(
			recordContext.AppType,
			recordContext.InboundEndpoint,
			recordContext.OutboundRoute,
			recordContext.Provider,
			recordContext.TargetURL,
			recordContext.RequestBody,
			recordContext.ResolvedModel,
			statusCode,
			true,
			recordContext.StartedAt,
			observation.FirstOutputAt,
			observation.CompletedAt,
			observation.InputTokens,
			observation.OutputTokens,
			errorDetail,
			recordContext.UpstreamResponsePreview,
			recordContext.UpstreamResponseRaw,
			recordContext.DeliveredResponsePreview,
			recordContext.RouteTrace,
			recordContext.AntiPoisonOps,
			recordContext.UpstreamToolCalls,
			recordContext.UpstreamToolArgsPreview,
			recordContext.UpstreamAssistantPreview,
			recordContext.UpstreamLatestObserved,
		)
	default:
		recordAdvancedProxyOpenAIStreamAttemptWithTraceAndOps(
			recordContext.AppType,
			recordContext.ClientRoute,
			recordContext.InboundEndpoint,
			recordContext.OutboundRoute,
			recordContext.Source,
			recordContext.Provider,
			recordContext.TargetURL,
			recordContext.RequestBody,
			recordContext.ResolvedModel,
			statusCode,
			true,
			recordContext.StartedAt,
			observation.FirstOutputAt,
			observation.CompletedAt,
			observation.InputTokens,
			observation.OutputTokens,
			errorDetail,
			recordContext.UpstreamResponsePreview,
			recordContext.UpstreamResponseRaw,
			recordContext.DeliveredResponsePreview,
			recordContext.RouteTrace,
			recordContext.AntiPoisonOps,
			recordContext.UpstreamToolCalls,
			recordContext.UpstreamToolArgsPreview,
			recordContext.UpstreamAssistantPreview,
			recordContext.UpstreamLatestObserved,
		)
	}
}

func hasOpenAIChatStreamOutput(chunk map[string]any) bool {
	if chunk == nil {
		return false
	}
	choices, _ := chunk["choices"].([]any)
	for _, rawChoice := range choices {
		choiceMap, _ := rawChoice.(map[string]any)
		if choiceMap == nil {
			continue
		}
		deltaMap, _ := choiceMap["delta"].(map[string]any)
		if deltaMap == nil {
			continue
		}
		if toStringValue(deltaMap["content"]) != "" ||
			toStringValue(deltaMap["reasoning_content"]) != "" ||
			toStringValue(deltaMap["thinking"]) != "" ||
			toStringValue(deltaMap["reasoning"]) != "" {
			return true
		}
		if toolCalls, ok := deltaMap["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			return true
		}
	}
	return false
}

func hasOpenAIResponsesStreamOutput(eventType string, data map[string]any) bool {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" || data == nil {
		return false
	}
	switch eventType {
	case "response.created", "response.in_progress":
		return false
	case "response.completed":
		responseMap, _ := data["response"].(map[string]any)
		if responseMap == nil {
			return false
		}
		if outputItems, ok := responseMap["output"].([]any); ok && len(outputItems) > 0 {
			return true
		}
		inputTokens, outputTokens := extractAdvancedProxyUsageFromMap(responseMap)
		return inputTokens != nil || outputTokens != nil
	default:
		return strings.HasPrefix(eventType, "response.output_") ||
			strings.HasPrefix(eventType, "response.reasoning.") ||
			strings.HasPrefix(eventType, "response.function_call_arguments.")
	}
}

func processOpenAIStreamMetricsLine(line []byte, observedFormat string, observation *advancedProxyStreamObservation) {
	if observation == nil {
		return
	}
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" {
		return
	}
	if payload == "[DONE]" {
		observation.markCompleted(time.Now())
		return
	}

	data := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return
	}

	switch normalizeAdvancedProxyObservedFormat(observedFormat) {
	case "responses":
		if responseMap, ok := data["response"].(map[string]any); ok && responseMap != nil {
			inputTokens, outputTokens := extractAdvancedProxyUsageFromMap(responseMap)
			observation.updateUsage(inputTokens, outputTokens)
		}
		if eventType := strings.TrimSpace(toStringValue(data["type"])); hasOpenAIResponsesStreamOutput(eventType, data) {
			observation.markFirstOutput(time.Now())
		}
	default:
		inputTokens, outputTokens := extractAdvancedProxyUsageFromMap(data)
		observation.updateUsage(inputTokens, outputTokens)
		if hasOpenAIChatStreamOutput(data) {
			observation.markFirstOutput(time.Now())
		}
	}
}

func processAnthropicStreamMetricsLine(line []byte, observation *advancedProxyStreamObservation) {
	if observation == nil {
		return
	}
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" || !strings.HasPrefix(trimmed, "data:") {
		return
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if payload == "" {
		return
	}

	data := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return
	}

	inputTokens, outputTokens := extractAdvancedProxyUsageFromMap(data)
	observation.updateUsage(inputTokens, outputTokens)

	switch strings.TrimSpace(toStringValue(data["type"])) {
	case "message_start":
		if messageMap, ok := data["message"].(map[string]any); ok && messageMap != nil {
			inputTokens, outputTokens = extractAdvancedProxyUsageFromMap(messageMap)
			observation.updateUsage(inputTokens, outputTokens)
		}
	case "content_block_start", "content_block_delta":
		observation.markFirstOutput(time.Now())
	case "message_stop":
		observation.markCompleted(time.Now())
	}
}

func proxyAnthropicStreamToClientWithMetrics(writer http.ResponseWriter, streamBody io.ReadCloser, recordContext *advancedProxyStreamRequestRecordContext) error {
	defer streamBody.Close()

	observation := advancedProxyStreamObservation{}
	if recordContext != nil {
		observation.StartedAt = recordContext.StartedAt
	}
	streamRaw, guardResult, readErr := readAndPrepareAntiPoisonAnthropicStream(streamBody, recordContext)
	if readErr != nil {
		observation.markCompleted(time.Now())
		if recordContext != nil {
			recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusBadGateway, readErr.Error())
		}
		return readErr
	}
	if guardResult.Blocked {
		observation.markFirstOutput(time.Now())
		observation.markCompleted(time.Now())
		writeAnthropicStreamAntiPoisonError(writer, "AllApiDeck anti-poison validation failed: "+guardResult.Reason)
		if recordContext != nil {
			recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusBadGateway, guardResult.Reason)
		}
		return nil
	}
	reader := bufio.NewReader(bytes.NewReader(streamRaw))
	flusher, _ := writer.(http.Flusher)
	var streamErr error
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			processAnthropicStreamMetricsLine(line, &observation)
			if _, writeErr := writer.Write(line); writeErr != nil {
				streamErr = writeErr
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				streamErr = err
			}
			break
		}
	}

	if recordContext != nil {
		observation.markCompleted(time.Now())
		errorDetail := ""
		if streamErr != nil {
			errorDetail = streamErr.Error()
		}
		if recordContext.DeliveredResponsePreview == "" {
			recordContext.DeliveredResponsePreview = summarizeAdvancedProxyRawStreamPreview(streamRaw)
		}
		recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusOK, errorDetail)
	}
	return streamErr
}

func proxyOpenAIStreamToClientWithMetrics(writer http.ResponseWriter, streamBody io.ReadCloser, recordContext *advancedProxyStreamRequestRecordContext) error {
	defer streamBody.Close()

	observation := advancedProxyStreamObservation{}
	if recordContext != nil {
		observation.StartedAt = recordContext.StartedAt
	}
	streamRaw, guardResult, readErr := readAndPrepareAntiPoisonOpenAIStream(streamBody, recordContext)
	if readErr != nil {
		observation.markCompleted(time.Now())
		if recordContext != nil {
			recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusBadGateway, readErr.Error())
		}
		return readErr
	}
	if guardResult.Blocked {
		observation.markFirstOutput(time.Now())
		observation.markCompleted(time.Now())
		observedFormat := ""
		if recordContext != nil {
			observedFormat = firstNonEmpty(recordContext.ObservedFormat, recordContext.ClientRoute, recordContext.OutboundRoute)
		}
		writeOpenAIStreamAntiPoisonError(writer, "AllApiDeck anti-poison validation failed: "+guardResult.Reason, observedFormat)
		if recordContext != nil {
			recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusBadGateway, guardResult.Reason)
		}
		return nil
	}

	flusher, _ := writer.(http.Flusher)
	reader := bufio.NewReader(bytes.NewReader(streamRaw))
	var streamErr error

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			observedFormat := ""
			if recordContext != nil {
				observedFormat = firstNonEmpty(recordContext.ObservedFormat, recordContext.ClientRoute)
			}
			processOpenAIStreamMetricsLine(line, observedFormat, &observation)
			if _, writeErr := writer.Write(line); writeErr != nil {
				streamErr = writeErr
				break
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		streamErr = err
		break
	}

	observation.markCompleted(time.Now())
	errorDetail := ""
	if streamErr != nil {
		errorDetail = fmt.Sprintf("stream forward failed: %s", streamErr.Error())
	}
	if recordContext != nil && recordContext.DeliveredResponsePreview == "" {
		recordContext.DeliveredResponsePreview = summarizeAdvancedProxyRawStreamPreview(streamRaw)
	}
	recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusOK, errorDetail)
	return streamErr
}

func writeAnthropicSSEFromOpenAIResponsesStream(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string) {
	writeAnthropicSSEFromOpenAIResponsesStreamWithRecord(writer, streamBody, fallbackModel, nil)
}

func writeAnthropicSSEFromOpenAIResponsesStreamWithRecord(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string, recordContext *advancedProxyStreamRequestRecordContext) {
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	defer streamBody.Close()

	observation := advancedProxyStreamObservation{}
	if recordContext != nil {
		observation.StartedAt = recordContext.StartedAt
	}
	streamRecordDetail := ""
	defer func() {
		if recordContext == nil {
			return
		}
		observation.markCompleted(time.Now())
		if strings.TrimSpace(recordContext.DeliveredResponsePreview) == "" && strings.TrimSpace(streamRecordDetail) != "" {
			recordContext.DeliveredResponsePreview = streamRecordDetail
		}
		recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusOK, streamRecordDetail)
	}()

	flusher, _ := writer.(http.Flusher)
	writeEvent := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(raw))
		if flusher != nil {
			flusher.Flush()
		}
	}

	streamReader := io.Reader(streamBody)
	if recordContext != nil {
		sanitizedRaw, guardResult, readErr := readAndPrepareAntiPoisonOpenAIStream(streamBody, recordContext)
		if readErr != nil {
			streamRecordDetail = fmt.Sprintf("responses stream read failed: %s", readErr.Error())
			writeAnthropicStreamAntiPoisonError(writer, "Advanced proxy stream read failed")
			return
		}
		if guardResult.Blocked {
			observation.markFirstOutput(time.Now())
			streamRecordDetail = "AllApiDeck anti-poison validation failed: " + guardResult.Reason
			writeAnthropicStreamAntiPoisonError(writer, streamRecordDetail)
			return
		}
		streamReader = bytes.NewReader(sanitizedRaw)
	}

	type responsesToolStreamState struct {
		Index       int
		ID          string
		Name        string
		Started     bool
		PendingArgs string
		EmittedArgs string
	}

	messageID := ""
	model := strings.TrimSpace(fallbackModel)
	if model == "" {
		model = "claude-proxy"
	}
	messageStarted := false
	messageStopped := false
	messageDeltaSent := false
	hasToolUse := false
	webSearchRequests := 0
	webSearchAnnotationEvents := 0
	nextContentIndex := 0
	currentTextIndex := -1
	currentThinkingIndex := -1
	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	toolStates := map[string]*responsesToolStreamState{}
	webSearchSeen := map[string]bool{}
	webSearchResultEmitted := map[string]bool{}
	webSearchToolUseIDs := map[string]string{}
	streamedOutputText := map[string]string{}
	streamedOutputAnnotations := map[string][]any{}
	streamPreviewText := ""
	streamPreviewStopReason := ""

	appendStreamPreviewText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if streamPreviewText == "" {
			streamPreviewText = text
			return
		}
		streamPreviewText = previewAdvancedProxyText(streamPreviewText+" "+text, 320)
	}

	emitMessageStart := func() {
		if messageStarted {
			return
		}
		writeEvent("message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    firstNonEmpty(messageID, fmt.Sprintf("msg_%d", time.Now().UnixNano())),
				"type":  "message",
				"role":  "assistant",
				"model": model,
				"usage": usage,
			},
		})
		messageStarted = true
	}
	closeIndex := func(index *int) {
		if index == nil || *index < 0 {
			return
		}
		writeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": *index,
		})
		*index = -1
	}
	closeOpenTools := func() {
		indices := make([]int, 0, len(toolStates))
		for _, state := range toolStates {
			if state != nil && state.Started {
				indices = append(indices, state.Index)
			}
		}
		sort.Ints(indices)
		for _, index := range indices {
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			})
			for key, state := range toolStates {
				if state != nil && state.Started && state.Index == index {
					delete(toolStates, key)
				}
			}
		}
	}
	emitMessageStop := func(stopReason string) {
		if messageStopped {
			return
		}
		streamPreviewStopReason = strings.TrimSpace(stopReason)
		closeIndex(&currentTextIndex)
		closeIndex(&currentThinkingIndex)
		closeOpenTools()
		emitMessageStart()
		if !messageDeltaSent {
			var resolvedStopReason any
			if strings.TrimSpace(stopReason) != "" {
				resolvedStopReason = strings.TrimSpace(stopReason)
			}
			writeEvent("message_delta", map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason":   resolvedStopReason,
					"stop_sequence": nil,
				},
				"usage": usage,
			})
			messageDeltaSent = true
		}
		writeEvent("message_stop", map[string]any{"type": "message_stop"})
		messageStopped = true
	}
	ensureTextBlock := func(blockType string) int {
		if blockType == "thinking" {
			closeIndex(&currentTextIndex)
			if currentThinkingIndex >= 0 {
				return currentThinkingIndex
			}
			observation.markFirstOutput(time.Now())
			emitMessageStart()
			currentThinkingIndex = nextContentIndex
			nextContentIndex++
			writeEvent("content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": currentThinkingIndex,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "",
				},
			})
			return currentThinkingIndex
		}
		closeIndex(&currentThinkingIndex)
		if currentTextIndex >= 0 {
			return currentTextIndex
		}
		observation.markFirstOutput(time.Now())
		emitMessageStart()
		currentTextIndex = nextContentIndex
		nextContentIndex++
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": currentTextIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		})
		return currentTextIndex
	}
	responsesUsageToAnthropic := func(source map[string]any) map[string]any {
		mapped := map[string]any{
			"input_tokens":  toIntValue(source["input_tokens"]),
			"output_tokens": toIntValue(source["output_tokens"]),
		}
		if details, ok := source["input_tokens_details"].(map[string]any); ok {
			if cached := toIntValue(details["cached_tokens"]); cached > 0 {
				mapped["cache_read_input_tokens"] = cached
			}
		}
		if cachedRead := toIntValue(source["cache_read_input_tokens"]); cachedRead > 0 {
			mapped["cache_read_input_tokens"] = cachedRead
		}
		if cacheCreated := toIntValue(source["cache_creation_input_tokens"]); cacheCreated > 0 {
			mapped["cache_creation_input_tokens"] = cacheCreated
		}
		if serverToolUse, ok := source["server_tool_use"].(map[string]any); ok && serverToolUse != nil {
			if webSearchCount := toIntValue(serverToolUse["web_search_requests"]); webSearchCount > 0 {
				mapped["server_tool_use"] = map[string]any{
					"web_search_requests": webSearchCount,
				}
			}
		}
		return mapped
	}
	resolveToolKey := func(data map[string]any) string {
		if itemID := strings.TrimSpace(toStringValue(data["item_id"])); itemID != "" {
			return "item:" + itemID
		}
		if callID := strings.TrimSpace(toStringValue(data["call_id"])); callID != "" {
			return "call:" + callID
		}
		if outputIndex := toIntValue(data["output_index"]); outputIndex > 0 || toStringValue(data["output_index"]) == "0" {
			return fmt.Sprintf("output:%d", outputIndex)
		}
		return ""
	}
	resolveToolState := func(key string) *responsesToolStreamState {
		if strings.TrimSpace(key) == "" {
			key = fmt.Sprintf("auto:%d", nextContentIndex)
		}
		if state, exists := toolStates[key]; exists {
			return state
		}
		state := &responsesToolStreamState{Index: nextContentIndex}
		nextContentIndex++
		toolStates[key] = state
		return state
	}
	resolveWebSearchKey := func(data map[string]any, itemMap map[string]any) string {
		key := resolveToolKey(data)
		if key != "" {
			return key
		}
		if itemID := strings.TrimSpace(toStringValue(itemMap["id"])); itemID != "" {
			return "ws:" + itemID
		}
		return fmt.Sprintf("ws:auto:%d", nextContentIndex)
	}
	resolveResponsesOutputTextKey := func(data map[string]any) string {
		itemID := strings.TrimSpace(toStringValue(data["item_id"]))
		if itemID == "" {
			itemID = strings.TrimSpace(toStringValue(data["output_item_id"]))
		}
		outputIndex := -1
		if raw, exists := data["output_index"]; exists {
			outputIndex = toIntValue(raw)
		}
		contentIndex := -1
		if raw, exists := data["content_index"]; exists {
			contentIndex = toIntValue(raw)
		}
		return fmt.Sprintf("item:%s|output:%d|content:%d", itemID, outputIndex, contentIndex)
	}
	buildStreamedWebSearchResultContents := func() []any {
		keySet := map[string]struct{}{}
		for key := range streamedOutputAnnotations {
			keySet[key] = struct{}{}
		}
		for key := range streamedOutputText {
			keySet[key] = struct{}{}
		}
		if len(keySet) == 0 {
			return nil
		}
		keys := make([]string, 0, len(keySet))
		for key := range keySet {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		results := make([]any, 0, len(keys))
		for _, key := range keys {
			content := buildAnthropicWebSearchResultContentFromAnnotations(streamedOutputAnnotations[key])
			if content == nil {
				content = buildAnthropicWebSearchResultContentFromText(streamedOutputText[key])
			}
			if content == nil {
				continue
			}
			results = append(results, content)
		}
		if len(results) == 0 {
			return nil
		}
		return results
	}
	firstPendingWebSearchKey := func() string {
		keys := make([]string, 0, len(webSearchSeen))
		for key := range webSearchSeen {
			if webSearchResultEmitted[key] {
				continue
			}
			keys = append(keys, key)
		}
		if len(keys) == 0 {
			return ""
		}
		sort.Strings(keys)
		return keys[0]
	}
	emitWebSearchLifecycle := func(key string, itemMap map[string]any) {
		toolUseID := normalizeAnthropicServerToolUseID(firstNonEmpty(
			webSearchToolUseIDs[key],
			strings.TrimSpace(toStringValue(itemMap["id"])),
			fmt.Sprintf("srvtoolu_%d", webSearchRequests+1),
		), webSearchRequests+1)
		webSearchToolUseIDs[key] = toolUseID
		if !webSearchSeen[key] {
			webSearchSeen[key] = true
			webSearchRequests++
			closeIndex(&currentTextIndex)
			closeIndex(&currentThinkingIndex)
			observation.markFirstOutput(time.Now())
			emitMessageStart()
			writeEvent("content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": nextContentIndex,
				"content_block": map[string]any{
					"type": "server_tool_use",
					"id":   toolUseID,
					"name": "web_search",
				},
			})
			if input := buildAnthropicWebSearchInput(itemMap); len(input) > 0 {
				writeEvent("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": nextContentIndex,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": stringifyJSON(input),
					},
				})
			}
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": nextContentIndex,
			})
			nextContentIndex++
			usage["server_tool_use"] = map[string]any{
				"web_search_requests": webSearchRequests,
			}
			appendAdvancedProxyLogf(
				"[CLAUDE_PROXY_WEB_SEARCH_STREAM_TOOL] response_id=%s key=%s tool_use_id=%s query=%s",
				firstNonEmpty(messageID, "unknown"),
				key,
				toolUseID,
				previewAdvancedProxyText(toStringValue(buildAnthropicWebSearchInput(itemMap)["query"]), 200),
			)
		}
		if !webSearchResultEmitted[key] {
			if result := buildAnthropicWebSearchResultBlock(toolUseID, itemMap); result != nil {
				writeEvent("content_block_start", map[string]any{
					"type":          "content_block_start",
					"index":         nextContentIndex,
					"content_block": result,
				})
				writeEvent("content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": nextContentIndex,
				})
				nextContentIndex++
				webSearchResultEmitted[key] = true
				appendAdvancedProxyLogf(
					"[CLAUDE_PROXY_WEB_SEARCH_STREAM_RESULT] response_id=%s key=%s tool_use_id=%s source=web_search_call results=%d",
					firstNonEmpty(messageID, "unknown"),
					key,
					toolUseID,
					countAnthropicWebSearchResults(result["content"]),
				)
			}
		}
	}
	emitWebSearchResultContent := func(key string, content any) {
		if webSearchResultEmitted[key] || content == nil {
			return
		}
		toolUseID := normalizeAnthropicServerToolUseID(firstNonEmpty(
			webSearchToolUseIDs[key],
			fmt.Sprintf("srvtoolu_%d", webSearchRequests+1),
		), webSearchRequests+1)
		webSearchToolUseIDs[key] = toolUseID
		result := buildAnthropicWebSearchResultBlockFromContent(toolUseID, content)
		if result == nil {
			return
		}
		observation.markFirstOutput(time.Now())
		writeEvent("content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         nextContentIndex,
			"content_block": result,
		})
		writeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": nextContentIndex,
		})
		nextContentIndex++
		webSearchResultEmitted[key] = true
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_WEB_SEARCH_STREAM_RESULT] response_id=%s key=%s tool_use_id=%s source=synthesized results=%d",
			firstNonEmpty(messageID, "unknown"),
			key,
			toolUseID,
			countAnthropicWebSearchResults(content),
		)
	}
	emitSyntheticWebSearchLifecycle := func(content any) {
		if content == nil {
			return
		}
		webSearchRequests++
		toolUseID := normalizeAnthropicServerToolUseID("", webSearchRequests)
		closeIndex(&currentTextIndex)
		closeIndex(&currentThinkingIndex)
		observation.markFirstOutput(time.Now())
		emitMessageStart()
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": nextContentIndex,
			"content_block": map[string]any{
				"type":  "server_tool_use",
				"id":    toolUseID,
				"name":  "web_search",
				"input": map[string]any{},
			},
		})
		writeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": nextContentIndex,
		})
		nextContentIndex++
		usage["server_tool_use"] = map[string]any{
			"web_search_requests": webSearchRequests,
		}
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": nextContentIndex,
			"content_block": map[string]any{
				"type":        "web_search_tool_result",
				"tool_use_id": toolUseID,
				"content":     content,
			},
		})
		writeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": nextContentIndex,
		})
		nextContentIndex++
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_WEB_SEARCH_STREAM_SYNTHETIC] response_id=%s tool_use_id=%s results=%d",
			firstNonEmpty(messageID, "unknown"),
			toolUseID,
			countAnthropicWebSearchResults(content),
		)
	}
	startToolState := func(state *responsesToolStreamState) {
		if state == nil || state.Started || strings.TrimSpace(state.ID) == "" || strings.TrimSpace(state.Name) == "" {
			return
		}
		observation.markFirstOutput(time.Now())
		emitMessageStart()
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": state.Index,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   state.ID,
				"name": state.Name,
			},
		})
		state.Started = true
		if state.PendingArgs != "" {
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": state.Index,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": state.PendingArgs,
				},
			})
			state.EmittedArgs = state.PendingArgs
			state.PendingArgs = ""
		}
	}
	mergeToolArguments := func(existing string, incoming string) string {
		if incoming == "" {
			return existing
		}
		if existing == "" {
			return incoming
		}
		switch {
		case incoming == existing:
			return existing
		case strings.HasPrefix(incoming, existing):
			return incoming
		case strings.HasPrefix(existing, incoming):
			return existing
		default:
			return existing + incoming
		}
	}
	emitToolArguments := func(state *responsesToolStreamState, incoming string) {
		if state == nil || incoming == "" {
			return
		}
		if !state.Started {
			state.PendingArgs = mergeToolArguments(state.PendingArgs, incoming)
			return
		}
		next := mergeToolArguments(state.EmittedArgs, incoming)
		if len(next) <= len(state.EmittedArgs) {
			return
		}
		delta := next[len(state.EmittedArgs):]
		writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": state.Index,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": delta,
			},
		})
		state.EmittedArgs = next
	}
	stringifyToolArgumentsForStream := func(value any) (string, error) {
		switch typed := value.(type) {
		case nil:
			return "", nil
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				return "", nil
			}
			return trimmed, nil
		default:
			return normalizeToolArgumentsJSON(typed)
		}
	}
	extractResponsesToolArguments := func(data map[string]any) string {
		args, err := stringifyToolArgumentsForStream(data["arguments"])
		if err != nil {
			appendAdvancedProxyLogf("[CLAUDE_PROXY_TOOL_ARGUMENTS_INVALID_STREAM] reason=%s arguments=%s", err.Error(), previewAdvancedProxyText(stringifyJSON(data["arguments"]), 240))
			return ""
		}
		if args != "" {
			return args
		}
		itemMap, _ := data["item"].(map[string]any)
		args, err = stringifyToolArgumentsForStream(itemMap["arguments"])
		if err != nil {
			appendAdvancedProxyLogf("[CLAUDE_PROXY_TOOL_ARGUMENTS_INVALID_STREAM] reason=%s arguments=%s", err.Error(), previewAdvancedProxyText(stringifyJSON(itemMap["arguments"]), 240))
			return ""
		}
		if args != "" {
			return args
		}
		return ""
	}

	scanner := bufio.NewScanner(streamReader)
	scanner.Buffer(make([]byte, 0, 64*1024), advancedProxySSEScannerMaxTokenSize)
	eventName := ""
	dataParts := make([]string, 0, 4)
	processEvent := func(eventName string, dataParts []string) {
		if len(dataParts) == 0 {
			return
		}
		payload := strings.Join(dataParts, "\n")
		data := map[string]any{}
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			return
		}
		responseData := data
		if responseMap, ok := data["response"].(map[string]any); ok && responseMap != nil {
			responseData = responseMap
		}
		if id := strings.TrimSpace(toStringValue(responseData["id"])); id != "" && messageID == "" {
			messageID = id
		}
		if resolvedModel := strings.TrimSpace(toStringValue(responseData["model"])); resolvedModel != "" {
			model = resolvedModel
		}

		switch strings.TrimSpace(eventName) {
		case "response.created":
			if usageMap, ok := responseData["usage"].(map[string]any); ok {
				usage = responsesUsageToAnthropic(usageMap)
				observation.updateUsage(
					intPtrValue(toIntValue(usage["input_tokens"])),
					intPtrValue(toIntValue(usage["output_tokens"])),
				)
			}
			emitMessageStart()
		case "response.content_part.added":
			partMap, _ := data["part"].(map[string]any)
			partType := strings.TrimSpace(toStringValue(partMap["type"]))
			if partType == "output_text" || partType == "refusal" {
				_ = ensureTextBlock("text")
			}
		case "response.output_text.delta", "response.refusal.delta":
			delta := firstNonEmptyExact(toStringValue(data["delta"]), toStringValue(data["text"]))
			if delta == "" {
				return
			}
			appendStreamPreviewText(delta)
			streamedOutputText[resolveResponsesOutputTextKey(data)] += delta
			index := ensureTextBlock("text")
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type": "text_delta",
					"text": delta,
				},
			})
		case "response.output_text.annotation.added":
			key := resolveResponsesOutputTextKey(data)
			annotationMap, _ := data["annotation"].(map[string]any)
			if annotationMap != nil {
				streamedOutputAnnotations[key] = append(streamedOutputAnnotations[key], annotationMap)
				webSearchAnnotationEvents++
				appendAdvancedProxyLogf(
					"[CLAUDE_PROXY_WEB_SEARCH_STREAM_ANNOTATION] response_id=%s key=%s annotation_type=%s url=%s total_for_key=%d",
					firstNonEmpty(messageID, "unknown"),
					key,
					strings.TrimSpace(toStringValue(annotationMap["type"])),
					previewAdvancedProxyText(toStringValue(annotationMap["url"]), 220),
					len(streamedOutputAnnotations[key]),
				)
			}
		case "response.output_text.done", "response.refusal.done":
			closeIndex(&currentTextIndex)
		case "response.reasoning.delta":
			delta := firstNonEmptyExact(toStringValue(data["delta"]), toStringValue(data["text"]))
			if delta == "" {
				return
			}
			index := ensureTextBlock("thinking")
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": delta,
				},
			})
		case "response.reasoning.done":
			closeIndex(&currentThinkingIndex)
		case "response.output_item.added":
			itemMap, _ := data["item"].(map[string]any)
			switch strings.TrimSpace(toStringValue(itemMap["type"])) {
			case "web_search_call":
				emitWebSearchLifecycle(resolveWebSearchKey(data, itemMap), itemMap)
				return
			case "function_call":
			default:
				return
			}
			hasToolUse = true
			closeIndex(&currentTextIndex)
			closeIndex(&currentThinkingIndex)
			state := resolveToolState(firstNonEmpty(resolveToolKey(data), resolveToolKey(itemMap)))
			state.ID = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), state.ID, strings.TrimSpace(toStringValue(itemMap["id"])))
			state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["name"])), state.Name)
			startToolState(state)
			emitToolArguments(state, extractResponsesToolArguments(map[string]any{"item": itemMap}))
		case "response.function_call_arguments.delta":
			hasToolUse = true
			closeIndex(&currentTextIndex)
			closeIndex(&currentThinkingIndex)
			state := resolveToolState(resolveToolKey(data))
			state.ID = firstNonEmpty(strings.TrimSpace(toStringValue(data["call_id"])), state.ID, strings.TrimSpace(toStringValue(data["item_id"])))
			state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(data["name"])), state.Name)
			startToolState(state)
			delta := toStringValue(data["delta"])
			if delta == "" {
				return
			}
			emitToolArguments(state, delta)
		case "response.function_call_arguments.done", "response.output_item.done":
			itemMap, _ := data["item"].(map[string]any)
			if strings.TrimSpace(toStringValue(itemMap["type"])) == "web_search_call" {
				emitWebSearchLifecycle(resolveWebSearchKey(data, itemMap), itemMap)
				return
			}
			if strings.TrimSpace(toStringValue(itemMap["type"])) == "message" {
				annotationResultContents := extractResponsesAnnotatedWebSearchResultContents([]any{itemMap})
				if len(annotationResultContents) == 0 {
					annotationResultContents = buildStreamedWebSearchResultContents()
				}
				if len(annotationResultContents) > 0 {
					if key := firstPendingWebSearchKey(); key != "" {
						emitWebSearchResultContent(key, annotationResultContents[0])
					} else {
						emitSyntheticWebSearchLifecycle(annotationResultContents[0])
					}
				}
				return
			}
			key := resolveToolKey(data)
			if key == "" {
				return
			}
			if state, exists := toolStates[key]; exists && state != nil {
				itemMap, _ := data["item"].(map[string]any)
				if strings.TrimSpace(toStringValue(itemMap["type"])) == "function_call" || strings.TrimSpace(eventName) == "response.function_call_arguments.done" {
					state.ID = firstNonEmpty(
						strings.TrimSpace(toStringValue(data["call_id"])),
						strings.TrimSpace(toStringValue(itemMap["call_id"])),
						state.ID,
						strings.TrimSpace(toStringValue(data["item_id"])),
						strings.TrimSpace(toStringValue(itemMap["id"])),
					)
					state.Name = firstNonEmpty(
						strings.TrimSpace(toStringValue(data["name"])),
						strings.TrimSpace(toStringValue(itemMap["name"])),
						state.Name,
					)
					startToolState(state)
					emitToolArguments(state, extractResponsesToolArguments(data))
				}
			}
			if state, exists := toolStates[key]; exists && state != nil && state.Started {
				writeEvent("content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": state.Index,
				})
				delete(toolStates, key)
			}
		case "response.completed":
			annotationResultContents := extractResponsesAnnotatedWebSearchResultContents(responseData["output"])
			textResultContents := extractResponsesTextWebSearchResultContents(responseData["output"])
			if len(annotationResultContents) == 0 {
				annotationResultContents = buildStreamedWebSearchResultContents()
			}
			if len(textResultContents) == 0 {
				textResultContents = buildStreamedWebSearchResultContents()
			}
			annotationResultIndex := 0
			textResultIndex := 0
			webSearchCalls := extractResponsesWebSearchCalls(responseData["output"])
			for _, webSearchCall := range webSearchCalls {
				key := resolveWebSearchKey(map[string]any{}, webSearchCall)
				emitWebSearchLifecycle(key, webSearchCall)
				if !webSearchResultEmitted[key] && annotationResultIndex < len(annotationResultContents) {
					emitWebSearchResultContent(key, annotationResultContents[annotationResultIndex])
					annotationResultIndex++
					continue
				}
				if !webSearchResultEmitted[key] && textResultIndex < len(textResultContents) {
					emitWebSearchResultContent(key, textResultContents[textResultIndex])
					textResultIndex++
				}
			}
			if len(webSearchCalls) == 0 && len(annotationResultContents) > 0 {
				for _, annotationResultContent := range annotationResultContents {
					emitSyntheticWebSearchLifecycle(annotationResultContent)
				}
			} else if len(webSearchCalls) == 0 && len(textResultContents) > 0 {
				for _, textResultContent := range textResultContents {
					emitSyntheticWebSearchLifecycle(textResultContent)
				}
			}
			appendAdvancedProxyLogf(
				"[CLAUDE_PROXY_WEB_SEARCH_STREAM_COMPLETE] response_id=%s web_search_calls=%d annotation_events=%d annotation_result_sets=%d text_result_sets=%d emitted_results=%d web_search_requests=%d",
				firstNonEmpty(messageID, "unknown"),
				len(webSearchCalls),
				webSearchAnnotationEvents,
				len(annotationResultContents),
				len(textResultContents),
				len(webSearchResultEmitted),
				webSearchRequests,
			)
			if usageMap, ok := responseData["usage"].(map[string]any); ok {
				usage = responsesUsageToAnthropic(usageMap)
				observation.updateUsage(
					intPtrValue(toIntValue(usage["input_tokens"])),
					intPtrValue(toIntValue(usage["output_tokens"])),
				)
			}
			if webSearchRequests > 0 {
				usage["server_tool_use"] = map[string]any{
					"web_search_requests": webSearchRequests,
				}
			}
			incompleteReason := ""
			if incompleteMap, ok := responseData["incomplete_details"].(map[string]any); ok {
				incompleteReason = toStringValue(incompleteMap["reason"])
			}
			emitMessageStop(mapOpenAIResponsesStopReason(toStringValue(responseData["status"]), hasToolUse, incompleteReason))
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			processEvent(eventName, dataParts)
			eventName = ""
			dataParts = dataParts[:0]
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataParts = append(dataParts, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(dataParts) > 0 {
		processEvent(eventName, dataParts)
	}
	if err := scanner.Err(); err != nil {
		appendAdvancedProxyLogf("responses stream scanner failed: %v", err)
		streamRecordDetail = fmt.Sprintf("responses stream scanner failed: %s", err.Error())
		index := ensureTextBlock("text")
		writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]any{
				"type": "text_delta",
				"text": "Advanced proxy stream interrupted before tool conversion completed. Please retry the previous action.",
			},
		})
		emitMessageStop("end_turn")
		return
	}
	emitMessageStop(mapOpenAIResponsesStopReason("completed", hasToolUse, ""))
	if streamRecordDetail == "" {
		streamRecordDetail = summarizeAdvancedProxyStreamResult(
			fmt.Sprintf("stop_reason=%s", firstNonEmpty(streamPreviewStopReason, "end_turn")),
			fmt.Sprintf("tool_use=%t", hasToolUse),
			fmt.Sprintf("web_search=%d", webSearchRequests),
			func() string {
				if strings.TrimSpace(streamPreviewText) == "" {
					return "text=-"
				}
				return "text=" + previewAdvancedProxyText(streamPreviewText, 220)
			}(),
		)
	}
	if recordContext != nil && strings.TrimSpace(streamRecordDetail) != "" {
		recordContext.DeliveredResponsePreview = streamRecordDetail
	}
}

func isAdvancedProxyTimeoutStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusGatewayTimeout, 524, 598, 599:
		return true
	default:
		return false
	}
}

func isAdvancedProxyTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded")
}

func observeAdvancedProxyAttempt(appType string, provider AdvancedProxyProvider, statusCode int, elapsed time.Duration, err error) {
	timeout := isAdvancedProxyTimeoutError(err) || isAdvancedProxyTimeoutStatusCode(statusCode)
	success := err == nil && statusCode >= 200 && statusCode < 300
	advancedProxyRuntime.ObserveProviderOutcome(appType, provider, statusCode, elapsed, success, timeout)
}

func buildAdvancedProxyMappedHeaders(provider AdvancedProxyProvider, mappingModel string) (map[string]string, string) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, ""
	}
	return resolveMappedHeadersForCheckModel(firstNonEmpty(strings.TrimSpace(mappingModel), strings.TrimSpace(provider.Model)), config.UserAgentMappings)
}

func buildClaudeProviderHeaders(provider AdvancedProxyProvider, apiFormat string, requestHeaders http.Header, stream bool, mappingModel string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "AllApiDeck/advanced-proxy",
	}
	mappedHeaders, _ := buildAdvancedProxyMappedHeaders(provider, mappingModel)
	if len(mappedHeaders) > 0 {
		for key, value := range mappedHeaders {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			headers[key] = value
		}
	}
	if stream {
		headers["Accept"] = "text/event-stream"
	} else {
		headers["Accept"] = "application/json"
	}
	if requestHeaders != nil {
		if _, mappedUserAgent := mappedHeaders["User-Agent"]; !mappedUserAgent {
			if userAgent := strings.TrimSpace(requestHeaders.Get("User-Agent")); userAgent != "" {
				headers["User-Agent"] = userAgent
			}
		}
		if _, mappedOriginator := mappedHeaders["Originator"]; !mappedOriginator {
			if originator := strings.TrimSpace(requestHeaders.Get("Originator")); originator != "" {
				headers["Originator"] = originator
			}
		}
	}
	if apiFormat == "anthropic" {
		headers["x-api-key"] = provider.APIKey
		headers["anthropic-version"] = firstNonEmpty(strings.TrimSpace(requestHeaders.Get("anthropic-version")), "2023-06-01")
		if beta := strings.TrimSpace(requestHeaders.Get("anthropic-beta")); beta != "" {
			headers["anthropic-beta"] = beta
		}
		return headers
	}
	headers["Authorization"] = "Bearer " + provider.APIKey
	return headers
}

func copySelectedHeaders(target http.Header, source http.Header, keys ...string) {
	if target == nil || source == nil {
		return
	}
	for _, key := range keys {
		for _, value := range source.Values(key) {
			if strings.TrimSpace(value) != "" {
				target.Add(key, value)
			}
		}
	}
}

func anthropicThinkingEnabled(body map[string]any) bool {
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(toStringValue(thinking["type"]))) {
	case "enabled", "adaptive":
		return true
	default:
		return false
	}
}

type anthropicToolStreamState struct {
	Index       int
	ID          string
	Name        string
	Started     bool
	PendingArgs string
	EmittedArgs string
}

func mergeAdvancedProxyToolArguments(existing string, incoming string) string {
	updated, _ := extendAdvancedProxyToolArguments(existing, incoming)
	return updated
}

func longestCommonPrefixLength(left string, right string) int {
	max := len(left)
	if len(right) < max {
		max = len(right)
	}
	for index := 0; index < max; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return max
}

func extendAdvancedProxyToolArguments(existing string, incoming string) (string, string) {
	if incoming == "" {
		return existing, ""
	}
	if existing == "" {
		return incoming, incoming
	}
	switch {
	case incoming == existing:
		return existing, ""
	case strings.HasPrefix(incoming, existing):
		return incoming, incoming[len(existing):]
	case len(incoming) > len(existing) && json.Valid([]byte(existing)) && json.Valid([]byte(incoming)):
		commonPrefixLength := longestCommonPrefixLength(existing, incoming)
		if commonPrefixLength >= len(existing)-1 {
			return incoming, incoming[commonPrefixLength:]
		}
	case strings.HasPrefix(existing, incoming):
		return existing, ""
	default:
		return existing + incoming, incoming
	}
	return existing + incoming, incoming
}

func accumulateAdvancedProxyToolArguments(existing string, incoming string) string {
	updated, _ := extendAdvancedProxyToolArguments(existing, incoming)
	return updated
}

func mapOpenAIStopReasonOptional(value string) any {
	resolved := strings.TrimSpace(mapOpenAIStopReason(value))
	if resolved == "" {
		return nil
	}
	return resolved
}

func writeAnthropicSSEFromOpenAIChatStream(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string, includeThinking bool) {
	writeAnthropicSSEFromOpenAIChatStreamWithRecord(writer, streamBody, fallbackModel, includeThinking, nil)
}

func writeAnthropicSSEFromOpenAIChatStreamWithRecord(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string, includeThinking bool, recordContext *advancedProxyStreamRequestRecordContext) {
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	defer streamBody.Close()

	observation := advancedProxyStreamObservation{}
	if recordContext != nil {
		observation.StartedAt = recordContext.StartedAt
	}
	streamRecordDetail := ""
	defer func() {
		if recordContext == nil {
			return
		}
		observation.markCompleted(time.Now())
		if strings.TrimSpace(recordContext.DeliveredResponsePreview) == "" && strings.TrimSpace(streamRecordDetail) != "" {
			recordContext.DeliveredResponsePreview = streamRecordDetail
		}
		recordAdvancedProxyStreamObservation(recordContext, observation, http.StatusOK, streamRecordDetail)
	}()

	flusher, _ := writer.(http.Flusher)
	writeEvent := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(raw))
		if flusher != nil {
			flusher.Flush()
		}
	}

	streamReader := io.Reader(streamBody)
	if recordContext != nil {
		sanitizedRaw, guardResult, readErr := readAndPrepareAntiPoisonOpenAIStream(streamBody, recordContext)
		if readErr != nil {
			streamRecordDetail = fmt.Sprintf("chat stream read failed: %s", readErr.Error())
			writeAnthropicStreamAntiPoisonError(writer, "Advanced proxy stream read failed")
			return
		}
		if guardResult.Blocked {
			observation.markFirstOutput(time.Now())
			streamRecordDetail = "AllApiDeck anti-poison validation failed: " + guardResult.Reason
			writeAnthropicStreamAntiPoisonError(writer, streamRecordDetail)
			return
		}
		streamReader = bytes.NewReader(sanitizedRaw)
	}

	messageID := ""
	model := strings.TrimSpace(fallbackModel)
	if model == "" {
		model = "claude-proxy"
	}
	messageStarted := false
	messageDeltaSent := false
	nextContentIndex := 0
	currentBlockType := ""
	currentBlockIndex := -1
	stopReason := "end_turn"
	hasToolUse := false
	streamPreviewText := ""
	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	toolStates := map[int]*anthropicToolStreamState{}
	openToolIndices := map[int]struct{}{}
	var startToolState func(state *anthropicToolStreamState)
	appendStreamPreviewText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if streamPreviewText == "" {
			streamPreviewText = text
			return
		}
		streamPreviewText = previewAdvancedProxyText(streamPreviewText+" "+text, 320)
	}

	emitMessageStart := func() {
		if messageStarted {
			return
		}
		writeEvent("message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    firstNonEmpty(messageID, fmt.Sprintf("msg_%d", time.Now().UnixNano())),
				"type":  "message",
				"role":  "assistant",
				"model": model,
				"usage": usage,
			},
		})
		messageStarted = true
	}
	emitToolArguments := func(state *anthropicToolStreamState, incoming string) {
		if state == nil || incoming == "" {
			return
		}
		if !state.Started {
			state.PendingArgs = accumulateAdvancedProxyToolArguments(state.PendingArgs, incoming)
			return
		}
		next, delta := extendAdvancedProxyToolArguments(state.EmittedArgs, incoming)
		if delta == "" {
			return
		}
		writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": state.Index,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": delta,
			},
		})
		state.EmittedArgs = next
	}
	startToolState = func(state *anthropicToolStreamState) {
		if state == nil || state.Started || state.ID == "" || state.Name == "" {
			return
		}
		observation.markFirstOutput(time.Now())
		emitMessageStart()
		writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": state.Index,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   state.ID,
				"name": state.Name,
			},
		})
		openToolIndices[state.Index] = struct{}{}
		state.Started = true
		if state.PendingArgs != "" {
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": state.Index,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": state.PendingArgs,
				},
			})
			state.EmittedArgs = state.PendingArgs
			state.PendingArgs = ""
		}
	}
	closeCurrentBlock := func() {
		if currentBlockIndex < 0 {
			return
		}
		writeEvent("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": currentBlockIndex,
		})
		currentBlockType = ""
		currentBlockIndex = -1
	}
	closeOpenToolBlocks := func() {
		lateToolIndices := make([]int, 0, len(toolStates))
		for toolIndex := range toolStates {
			lateToolIndices = append(lateToolIndices, toolIndex)
		}
		sort.Ints(lateToolIndices)
		for _, toolIndex := range lateToolIndices {
			state := toolStates[toolIndex]
			if state == nil || state.Started {
				continue
			}
			hasPayload := state.PendingArgs != "" || state.ID != "" || state.Name != ""
			if !hasPayload {
				continue
			}
			if state.ID == "" {
				state.ID = fmt.Sprintf("tool_call_%d", toolIndex)
			}
			if state.Name == "" {
				state.Name = "unknown_tool"
			}
			startToolState(state)
		}
		if len(openToolIndices) == 0 {
			return
		}
		indices := make([]int, 0, len(openToolIndices))
		for index := range openToolIndices {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			})
			delete(openToolIndices, index)
		}
	}
	emitMessageDelta := func() {
		if messageDeltaSent {
			return
		}
		writeEvent("message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   mapOpenAIStopReasonOptional(stopReason),
				"stop_sequence": nil,
			},
			"usage": usage,
		})
		messageDeltaSent = true
	}
	ensureContentBlock := func(blockType string, payload map[string]any) {
		if currentBlockType == blockType && currentBlockIndex >= 0 {
			return
		}
		closeCurrentBlock()
		observation.markFirstOutput(time.Now())
		emitMessageStart()
		currentBlockIndex = nextContentIndex
		nextContentIndex++
		currentBlockType = blockType
		writeEvent("content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         currentBlockIndex,
			"content_block": payload,
		})
	}

	scanner := bufio.NewScanner(streamReader)
	scanner.Buffer(make([]byte, 0, 64*1024), advancedProxySSEScannerMaxTokenSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			closeCurrentBlock()
			closeOpenToolBlocks()
			emitMessageStart()
			emitMessageDelta()
			writeEvent("message_stop", map[string]any{"type": "message_stop"})
			return
		}

		chunk := map[string]any{}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if strings.TrimSpace(toStringValue(chunk["id"])) != "" && messageID == "" {
			messageID = strings.TrimSpace(toStringValue(chunk["id"]))
		}
		if strings.TrimSpace(toStringValue(chunk["model"])) != "" {
			model = strings.TrimSpace(toStringValue(chunk["model"]))
		}
		if chunkUsage := openAIUsageToAnthropic(chunk); len(chunkUsage) > 0 {
			for key, value := range chunkUsage {
				if toIntValue(value) > 0 {
					usage[key] = value
				}
			}
			observation.updateUsage(
				intPtrValue(toIntValue(usage["input_tokens"])),
				intPtrValue(toIntValue(usage["output_tokens"])),
			)
		}

		choices, _ := chunk["choices"].([]any)
		if len(choices) == 0 {
			continue
		}
		choiceMap, _ := choices[0].(map[string]any)
		if choiceMap == nil {
			continue
		}
		if finish := strings.TrimSpace(toStringValue(choiceMap["finish_reason"])); finish != "" {
			stopReason = finish
		}
		delta, _ := choiceMap["delta"].(map[string]any)
		if delta == nil {
			continue
		}

		if includeThinking {
			thinkingText := firstNonEmptyExact(
				toStringValue(delta["reasoning_content"]),
				toStringValue(delta["thinking"]),
				toStringValue(delta["reasoning"]),
			)
			if thinkingText != "" {
				ensureContentBlock("thinking", map[string]any{
					"type":     "thinking",
					"thinking": "",
				})
				writeEvent("content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": currentBlockIndex,
					"delta": map[string]any{
						"type":     "thinking_delta",
						"thinking": thinkingText,
					},
				})
			}
		}

		if text := toStringValue(delta["content"]); text != "" {
			appendStreamPreviewText(text)
			ensureContentBlock("text", map[string]any{
				"type": "text",
				"text": "",
			})
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": currentBlockIndex,
				"delta": map[string]any{
					"type": "text_delta",
					"text": text,
				},
			})
		}

		if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
			hasToolUse = true
			closeCurrentBlock()
			for _, rawToolCall := range toolCalls {
				toolCallMap, ok := rawToolCall.(map[string]any)
				if !ok {
					continue
				}
				toolIndex := toIntValue(toolCallMap["index"])
				state, exists := toolStates[toolIndex]
				if !exists {
					state = &anthropicToolStreamState{Index: nextContentIndex}
					nextContentIndex++
					toolStates[toolIndex] = state
				}
				if id := strings.TrimSpace(toStringValue(toolCallMap["id"])); id != "" {
					state.ID = id
				}
				functionMap, _ := toolCallMap["function"].(map[string]any)
				if functionMap != nil {
					if name := strings.TrimSpace(toStringValue(functionMap["name"])); name != "" {
						state.Name = name
					}
				}
				startToolState(state)
				if functionMap != nil {
					args := toStringValue(functionMap["arguments"])
					emitToolArguments(state, args)
				}
			}
		}

		if strings.TrimSpace(toStringValue(choiceMap["finish_reason"])) != "" {
			closeCurrentBlock()
			closeOpenToolBlocks()
			emitMessageStart()
			emitMessageDelta()
		}
	}
	if err := scanner.Err(); err != nil {
		appendAdvancedProxyLogf("chat stream scanner failed: %v", err)
		streamRecordDetail = fmt.Sprintf("chat stream scanner failed: %s", err.Error())
		return
	}

	closeCurrentBlock()
	closeOpenToolBlocks()
	emitMessageStart()
	emitMessageDelta()
	writeEvent("message_stop", map[string]any{"type": "message_stop"})
	streamRecordDetail = summarizeAdvancedProxyStreamResult(
		fmt.Sprintf("stop_reason=%s", firstNonEmpty(strings.TrimSpace(mapOpenAIStopReason(stopReason)), "end_turn")),
		fmt.Sprintf("tool_use=%t", hasToolUse),
		func() string {
			if strings.TrimSpace(streamPreviewText) == "" {
				return "text=-"
			}
			return "text=" + previewAdvancedProxyText(streamPreviewText, 220)
		}(),
	)
}

func buildOpenAIProviderHeaders(provider AdvancedProxyProvider, mappingModel string) map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
		"User-Agent":    "AllApiDeck/advanced-proxy",
		"Authorization": "Bearer " + provider.APIKey,
	}
	if mappedHeaders, _ := buildAdvancedProxyMappedHeaders(provider, mappingModel); len(mappedHeaders) > 0 {
		for key, value := range mappedHeaders {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			headers[key] = value
		}
	}
	return headers
}

func forwardClaudeRequestViaProvider(provider AdvancedProxyProvider, requestBody map[string]any, requestHeaders http.Header, stream bool, config AdvancedProxyConfig) providerAttemptResult {
	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled
	timeoutSeconds := computeAdvancedProxyTimeoutSeconds(stream, failoverActive, config.Failover)
	requestFeatures := classifyClaudeRequestFeatures(requestBody)
	capabilities := resolveAdvancedProxyProviderCapabilities(provider)
	debugEnabled := advancedProxyDebugEnabled(config)
	phases := buildClaudeProxyAttemptPhases(provider, requestBody, requestFeatures)
	if len(phases) == 0 {
		return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "no compatible upstream endpoint found"}
	}

	basePayload := deepCopyJSONMap(requestBody)
	basePayload["stream"] = stream
	if strings.TrimSpace(provider.Model) != "" {
		basePayload["model"] = provider.Model
	}
	if capabilities.SanitizeOrphanToolResults {
		sanitizedCount := sanitizeOrphanToolResults(basePayload)
		if sanitizedCount > 0 {
			appendAdvancedProxyLogf("[CLAUDE_PROXY_SANITIZE] provider=%s sanitized_orphan_tool_results=%d", advancedProxyProviderLabel(provider), sanitizedCount)
		}
	}
	antiPoisonCtx := antiPoisonRequestContext{Config: sanitizeAntiPoisonConfig(config.AntiPoison), AppType: "claude", RouteKind: "claude_messages"}
	if config.AntiPoison.Enabled {
		guardedPayload, guardCtx, guardErr := applyAntiPoisonPromptToAnthropicRequest(basePayload, config.AntiPoison)
		if guardErr != nil {
			appendAdvancedProxyLogf(
				"[ANTI_POISON_PROMPT_FAIL] app=claude route=messages provider=%s detail=%s",
				advancedProxyProviderLabel(provider),
				previewAdvancedProxyText(guardErr.Error(), 220),
			)
		} else if guardCtx.Enabled {
			basePayload = guardedPayload
			antiPoisonCtx = guardCtx
			appendAdvancedProxyLogf(
				"[ANTI_POISON_PROMPT_APPLY] app=claude route=messages provider=%s alias=%s guard=%s strategy=%d phrase=%d insertion=%s",
				advancedProxyProviderLabel(provider),
				previewAdvancedProxyText(guardCtx.Alias, 40),
				previewAdvancedProxyText(guardCtx.GuardToolName, 80),
				guardCtx.StrategySlot,
				guardCtx.PhraseVariant,
				previewAdvancedProxyText(guardCtx.InsertionPoint, 60),
			)
		}
	}
	if debugEnabled {
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_REQUEST] provider=%s stream=%t capabilities=%s phases=%s payload=%s",
			advancedProxyProviderLabel(provider),
			stream,
			summarizeAdvancedProxyJSON(capabilities, 320),
			summarizeAdvancedProxyJSON(phases, 640),
			summarizeAdvancedProxyJSON(basePayload, 1800),
		)
	}

	buildTargets := func(apiFormat string) []string {
		switch apiFormat {
		case "openai_chat":
			return buildOpenAIChatCheckEndpointCandidates(provider.BaseURL)
		case "openai_responses":
			return buildResponsesEndpointCandidates(provider.BaseURL)
		default:
			return []string{resolveAnthropicMessagesEndpoint(provider.BaseURL)}
		}
	}
	buildClaudeRouteTrace := func(base []AdvancedProxyRequestRouteStep, routeKind string, source string, status string) []AdvancedProxyRequestRouteStep {
		return appendAdvancedProxyRouteTraceStep(base, routeKind, source, status)
	}
	buildClaudePhaseTraceBase := func(phaseIndex int) []AdvancedProxyRequestRouteStep {
		trace := make([]AdvancedProxyRequestRouteStep, 0, phaseIndex+1)
		for index := 0; index < phaseIndex && index < len(phases); index++ {
			trace = appendAdvancedProxyRouteTraceStep(trace, phases[index].routeKind, phases[index].source, "failed")
		}
		return trace
	}

	fallbackModel := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(basePayload["model"])))
	lastStatus := http.StatusBadGateway
	lastMessage := "no compatible upstream endpoint found"
	for phaseIndex, phase := range phases {
		var nextPhase *claudeProxyAttemptPhase
		if phaseIndex+1 < len(phases) {
			nextPhase = &phases[phaseIndex+1]
		}
		phaseTraceBase := buildClaudePhaseTraceBase(phaseIndex)
		currentRouteSource := phase.source
		payload := deepCopyJSONMap(basePayload)
		signatureRectified := false
		budgetRectified := false
		chatSystemRectified := false

	retryPhase:
		targets := buildTargets(phase.apiFormat)
		if len(targets) == 0 {
			lastStatus = http.StatusBadGateway
			lastMessage = "provider endpoint is empty"
			continue
		}

		var transformed map[string]any
		switch phase.apiFormat {
		case "openai_chat":
			transformed = anthropicRequestToOpenAIChat(payload, provider)
		case "openai_responses":
			transformed = anthropicRequestToOpenAIResponses(payload, provider)
		default:
			transformed = payload
		}
		stringProtectionCtx := antiPoisonStringProtectionContext{}
		if config.AntiPoison.Enabled && config.AntiPoison.StringProtection.Enabled {
			rawTransformedForProtection, marshalErr := json.Marshal(transformed)
			if marshalErr == nil {
				protectedRaw, protectionCtx, protectionErr := applyAntiPoisonStringProtectionToJSONBody(rawTransformedForProtection, config.AntiPoison, phase.routeKind, advancedProxyProviderLabel(provider), "claude")
				stringProtectionCtx = protectionCtx
				if protectionErr != nil {
					appendAdvancedProxyLogf(
						"[ANTI_POISON_STRING_PROTECT_FAIL] app=claude route=%s provider=%s detail=%s",
						phase.routeKind,
						advancedProxyProviderLabel(provider),
						previewAdvancedProxyText(protectionErr.Error(), 220),
					)
				} else if protectionCtx.Enabled {
					protectedMap := map[string]any{}
					if err := json.Unmarshal(protectedRaw, &protectedMap); err == nil {
						transformed = protectedMap
						appendAdvancedProxyLogf(
							"[ANTI_POISON_STRING_PROTECT] app=claude route=%s provider=%s ops=%d placeholders=%d",
							phase.routeKind,
							advancedProxyProviderLabel(provider),
							len(protectionCtx.Records),
							len(protectionCtx.mapping),
						)
					}
				}
			}
		}
		resolvedPhaseModel := firstNonEmpty(strings.TrimSpace(toStringValue(transformed["model"])), fallbackModel)
		requestSnapshot, _ := json.Marshal(transformed)
		if debugEnabled {
			appendAdvancedProxyLogf(
				"[CLAUDE_PROXY_TRANSFORM] provider=%s format=%s route=%s source=%s transformed=%s",
				advancedProxyProviderLabel(provider),
				phase.apiFormat,
				phase.routeKind,
				currentRouteSource,
				summarizeAdvancedProxyJSON(transformed, 2200),
			)
		}

		advanceToNextPhase := false
		for _, targetURL := range targets {
			advancedProxyRuntime.MarkDispatch("claude", provider, phase.routeKind, targetURL)
			if stream {
				rawTransformed, err := json.Marshal(transformed)
				if err != nil {
					advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, 0, 0, err)
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid upstream JSON request"}
				}
				attemptStartedAt := time.Now()
				statusCode, responseHeaders, rawResponse, streamBody, elapsed, err := performRawUpstreamRequest(http.MethodPost, targetURL, buildClaudeProviderHeaders(provider, phase.apiFormat, requestHeaders, stream, resolvedPhaseModel), rawTransformed, timeoutSeconds, true)
				if err != nil {
					advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, err)
					if debugEnabled {
						appendAdvancedProxyLogf("[CLAUDE_PROXY_STREAM_ERROR] provider=%s format=%s route=%s endpoint=%s detail=%s", advancedProxyProviderLabel(provider), phase.apiFormat, phase.routeKind, targetURL, previewAdvancedProxyText(err.Error(), 320))
					}
					recordAdvancedProxyClaudeAttemptWithTrace("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, rawTransformed, resolvedPhaseModel, nil, nil, stream, http.StatusBadGateway, elapsed, err.Error(), buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"))
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: err.Error()}
				}
				if statusCode < 200 || statusCode >= 300 {
					advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					if streamBody != nil {
						streamBody.Close()
					}
					errorMessage := summarizeAdvancedProxyBody(rawResponse)
					lastStatus = statusCode
					lastMessage = firstNonEmpty(errorMessage, fmt.Sprintf("HTTP %d", statusCode))
					if debugEnabled {
						appendAdvancedProxyLogf("[CLAUDE_PROXY_STREAM_REJECT] provider=%s format=%s route=%s endpoint=%s status=%d detail=%s", advancedProxyProviderLabel(provider), phase.apiFormat, phase.routeKind, targetURL, statusCode, errorMessage)
					}
					recordAdvancedProxyClaudeAttemptWithTrace("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, rawTransformed, resolvedPhaseModel, nil, rawResponse, stream, statusCode, elapsed, errorMessage, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"))
					if nextPhase != nil && shouldAdvanceClaudeProxyPhase(phase, *nextPhase, statusCode, rawResponse, requestFeatures) {
						if debugEnabled {
							appendAdvancedProxyLogf(
								"[CLAUDE_PROXY_FALLBACK] provider=%s from=%s to=%s scope=%s reason=%s",
								advancedProxyProviderLabel(provider),
								phase.routeKind,
								nextPhase.routeKind,
								previewAdvancedProxyText(nextPhase.preferenceScopeKey, 160),
								previewAdvancedProxyText(errorMessage, 220),
							)
						}
						advanceToNextPhase = true
						break
					}
					return providerAttemptResult{
						StatusCode: statusCode,
						Message:    firstNonEmpty(errorMessage, fmt.Sprintf("HTTP %d", statusCode)),
					}
				}
				advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, true)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
				if phase.preferenceScopeKey != "" {
					setAdvancedProxyClaudeProtocolPreference(phase.preferenceScopeKey, phase.preferenceValue)
					if debugEnabled {
						preferName := describeAdvancedProxyClaudeProtocolPreference(phase.preferenceValue)
						appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_SET] provider=%s scope=%s prefer=%s route=%s", advancedProxyProviderLabel(provider), previewAdvancedProxyText(phase.preferenceScopeKey, 160), preferName, phase.routeKind)
					}
				}
				return providerAttemptResult{
					StatusCode: http.StatusOK,
					Headers:    responseHeaders,
					StreamBody: streamBody,
					APIFormat:  phase.apiFormat,
					Model:      fallbackModel,
					RecordCtx: &advancedProxyStreamRequestRecordContext{
						AppType:         "claude",
						ClientRoute:     "messages",
						InboundEndpoint: buildAdvancedProxyClaudeInboundEndpoint(),
						OutboundRoute:   phase.routeKind,
						RouteTrace:      buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "success"),
						Source:          currentRouteSource,
						Provider:        provider,
						TargetURL:       targetURL,
						RequestBody:     rawTransformed,
						TimeoutSeconds:  timeoutSeconds,
						ResolvedModel:   resolvedPhaseModel,
						StartedAt:       attemptStartedAt,
						ObservedFormat:  phase.apiFormat,
						AntiPoisonCtx:   antiPoisonCtx,
						StringProtect:   stringProtectionCtx,
					},
				}
			}

		retryCurrentTarget:
			statusCode, responseHeaders, rawResponse, elapsed, err := performJSONUpstreamRequest(http.MethodPost, targetURL, buildClaudeProviderHeaders(provider, phase.apiFormat, requestHeaders, stream, resolvedPhaseModel), transformed, timeoutSeconds)
			if err != nil {
				advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, err)
				if debugEnabled {
					appendAdvancedProxyLogf("[CLAUDE_PROXY_ERROR] provider=%s format=%s route=%s endpoint=%s detail=%s", advancedProxyProviderLabel(provider), phase.apiFormat, phase.routeKind, targetURL, previewAdvancedProxyText(err.Error(), 320))
				}
				recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, nil, stream, http.StatusBadGateway, elapsed, err.Error(), buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: err.Error()}
			}
			if statusCode < 200 || statusCode >= 300 {
				errorMessage := normalizeAnthropicErrorMessage(rawResponse)
				lastStatus = statusCode
				lastMessage = firstNonEmpty(errorMessage, fmt.Sprintf("HTTP %d", statusCode))
				if debugEnabled {
					appendAdvancedProxyLogf("[CLAUDE_PROXY_REJECT] provider=%s format=%s route=%s endpoint=%s status=%d detail=%s raw=%s", advancedProxyProviderLabel(provider), phase.apiFormat, phase.routeKind, targetURL, statusCode, previewAdvancedProxyText(errorMessage, 320), summarizeAdvancedProxyBody(rawResponse))
				}
				if phase.apiFormat == "anthropic" && !signatureRectified && shouldRectifyThinkingSignature(errorMessage, config.Rectifier) && rectifyThinkingSignature(payload) {
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, statusCode, elapsed, errorMessage, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
					phaseTraceBase = buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed")
					currentRouteSource = "rectified"
					signatureRectified = true
					goto retryPhase
				}
				if phase.apiFormat == "anthropic" && !budgetRectified && shouldRectifyThinkingBudget(errorMessage, config.Rectifier) && rectifyThinkingBudget(payload) {
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, statusCode, elapsed, errorMessage, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
					phaseTraceBase = buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed")
					currentRouteSource = "rectified"
					budgetRectified = true
					goto retryPhase
				}
				if phase.apiFormat == "openai_chat" && !chatSystemRectified && shouldRectifyOpenAIChatSystemPrompt(errorMessage) && inlineOpenAIChatSystemPrompt(transformed) {
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, statusCode, elapsed, errorMessage, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
					phaseTraceBase = buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed")
					currentRouteSource = "rectified"
					chatSystemRectified = true
					resolvedPhaseModel = firstNonEmpty(strings.TrimSpace(toStringValue(transformed["model"])), fallbackModel)
					requestSnapshot, _ = json.Marshal(transformed)
					appendAdvancedProxyLogf(
						"[CLAUDE_PROXY_CHAT_SYSTEM_RECTIFY] provider=%s route=%s endpoint=%s reason=%s",
						advancedProxyProviderLabel(provider),
						phase.routeKind,
						targetURL,
						previewAdvancedProxyText(errorMessage, 160),
					)
					goto retryCurrentTarget
				}
				advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
				recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, statusCode, elapsed, errorMessage, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
				if nextPhase != nil && shouldAdvanceClaudeProxyPhase(phase, *nextPhase, statusCode, rawResponse, requestFeatures) {
					if debugEnabled {
						appendAdvancedProxyLogf(
							"[CLAUDE_PROXY_FALLBACK] provider=%s from=%s to=%s scope=%s reason=%s",
							advancedProxyProviderLabel(provider),
							phase.routeKind,
							nextPhase.routeKind,
							previewAdvancedProxyText(nextPhase.preferenceScopeKey, 160),
							previewAdvancedProxyText(errorMessage, 220),
						)
					}
					advanceToNextPhase = true
					break
				}
				if isRetryableCheckStatus(statusCode) && (phase.apiFormat == "openai_chat" || phase.apiFormat == "openai_responses") {
					continue
				}
				return providerAttemptResult{
					StatusCode: statusCode,
					Message:    firstNonEmpty(errorMessage, fmt.Sprintf("HTTP %d", statusCode)),
				}
			}

			responseMap := map[string]any{}
			if err := json.Unmarshal(rawResponse, &responseMap); err != nil {
				advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
				recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, http.StatusBadGateway, elapsed, "invalid upstream JSON response", buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid upstream JSON response"}
			}
			if antiPoisonCtx.Enabled {
				validationBody := rawResponse
				validationRoute := ""
				switch phase.apiFormat {
				case "openai_chat":
					validationRoute = "chat"
				case "openai_responses":
					validationRoute = "responses"
				default:
					validationRoute = "anthropic"
				}
				var guardResult antiPoisonValidationResult
				if validationRoute == "anthropic" {
					guardResult = validateAndStripAntiPoisonAnthropicResponse(validationBody, antiPoisonCtx)
				} else {
					guardResult = validateAndStripAntiPoisonOpenAIResponse(validationBody, validationRoute, antiPoisonCtx)
				}
				appendAdvancedProxyLogf(
					"[ANTI_POISON_VALIDATE] app=claude route=messages provider=%s format=%s alias=%s valid=%t blocked=%t reason=%s real=%d guard=%d stripped=%d",
					advancedProxyProviderLabel(provider),
					phase.apiFormat,
					previewAdvancedProxyText(antiPoisonCtx.Alias, 40),
					guardResult.Valid,
					guardResult.Blocked,
					previewAdvancedProxyText(guardResult.Reason, 120),
					guardResult.RealCount,
					guardResult.GuardCount,
					guardResult.RemovedGuards,
				)
				if guardResult.Blocked {
					advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, http.StatusBadGateway, elapsed, nil)
					ops := appendAntiPoisonBlockedOperation(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider), "claude", guardResult.Reason)
					recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, guardResult.Body, stream, http.StatusBadGateway, elapsed, guardResult.Reason, buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(ops, phase.routeKind, advancedProxyProviderLabel(provider)))
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "AllApiDeck anti-poison validation failed: " + guardResult.Reason, AntiPoisonBlocked: true}
				}
				if guardResult.Applied {
					rawResponse = guardResult.Body
					responseMap = map[string]any{}
					if err := json.Unmarshal(rawResponse, &responseMap); err != nil {
						advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
						recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, http.StatusBadGateway, elapsed, "invalid stripped anti-poison response", buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
						return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid stripped anti-poison response"}
					}
				}
			}
			if !stream && stringProtectionCtx.Enabled {
				rawResponse = restoreAntiPoisonStringProtectionInJSONBody(rawResponse, &stringProtectionCtx, phase.routeKind, advancedProxyProviderLabel(provider), "claude")
				responseMap = map[string]any{}
				if err := json.Unmarshal(rawResponse, &responseMap); err != nil {
					advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, false)
					recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, nil, rawResponse, stream, http.StatusBadGateway, elapsed, "invalid restored anti-poison response", buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid restored anti-poison response"}
				}
			}
			switch phase.apiFormat {
			case "openai_chat":
				responseMap = openAIChatToAnthropic(responseMap, fallbackModel, anthropicThinkingEnabled(requestBody))
			case "openai_responses":
				responseMap = openAIResponsesToAnthropic(responseMap, fallbackModel)
			}
			advancedProxyRuntime.MarkResult("claude", provider, phase.routeKind, targetURL, true)
			observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
			if phase.preferenceScopeKey != "" {
				setAdvancedProxyClaudeProtocolPreference(phase.preferenceScopeKey, phase.preferenceValue)
				if debugEnabled {
					preferName := describeAdvancedProxyClaudeProtocolPreference(phase.preferenceValue)
					appendAdvancedProxyLogf("[CLAUDE_PROXY_PREFERENCE_SET] provider=%s scope=%s prefer=%s route=%s", advancedProxyProviderLabel(provider), previewAdvancedProxyText(phase.preferenceScopeKey, 160), preferName, phase.routeKind)
				}
			}
			recordAdvancedProxyClaudeAttemptWithTraceAndOps("claude", buildAdvancedProxyClaudeInboundEndpoint(), phase.routeKind, provider, targetURL, requestSnapshot, resolvedPhaseModel, responseMap, rawResponse, stream, http.StatusOK, elapsed, "", buildClaudeRouteTrace(phaseTraceBase, phase.routeKind, currentRouteSource, "success"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, phase.routeKind, advancedProxyProviderLabel(provider)))
			return providerAttemptResult{Response: responseMap, StatusCode: http.StatusOK, Headers: responseHeaders}
		}

		if advanceToNextPhase {
			continue
		}
		return providerAttemptResult{StatusCode: lastStatus, Message: lastMessage}
	}

	return providerAttemptResult{StatusCode: lastStatus, Message: lastMessage}
}

func normalizeOpenAIProviderDispatchRoute(apiFormat string) string {
	switch strings.ToLower(strings.TrimSpace(apiFormat)) {
	case "openai_chat":
		return "chat"
	case "openai_responses":
		return "responses"
	default:
		return ""
	}
}

func extractJSONRequestModel(rawBody []byte) string {
	requestBody := map[string]any{}
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		return ""
	}
	return strings.TrimSpace(toStringValue(requestBody["model"]))
}

func normalizeOpenAIProxyRequestForProvider(rawBody []byte, provider AdvancedProxyProvider) ([]byte, string, error) {
	requestBody := map[string]any{}
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		return nil, "", err
	}

	resolvedModel := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(requestBody["model"])))
	if resolvedModel == "" {
		return rawBody, "", nil
	}
	if strings.TrimSpace(toStringValue(requestBody["model"])) == resolvedModel {
		return rawBody, resolvedModel, nil
	}

	requestBody["model"] = resolvedModel
	normalizedBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, "", err
	}
	return normalizedBody, resolvedModel, nil
}

func forwardOpenAIRequestViaProvider(appType string, provider AdvancedProxyProvider, routeKind string, rawBody []byte, stream bool, config AdvancedProxyConfig) rawProviderAttemptResult {
	providerLabel := advancedProxyProviderLabel(provider)
	if normalizeClaudeAPIFormat(provider.APIFormat) == "anthropic" {
		return rawProviderAttemptResult{
			StatusCode: http.StatusBadGateway,
			Message:    formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "provider does not support OpenAI-compatible proxy routes"),
			ErrorCode:  "advanced_proxy_error",
			ErrorType:  "invalid_request_error",
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}

	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled
	timeoutSeconds := computeAdvancedProxyTimeoutSeconds(stream, failoverActive, config.Failover)
	originalRequestModel := extractJSONRequestModel(rawBody)

	preparedBody, healingContext, prepareErr := prepareOpenAIRequestForEncryptedContentHealing(rawBody, appType)
	if prepareErr != nil {
		preparedBody = rawBody
	}
	if healingContext.AppliedHistoricalCut > 0 {
		appendAdvancedProxyLogf(
			"[OPENAI_PROXY_HEAL_APPLY] app=%s route=%s session=%s stripped=%d cutoff=%d",
			appType,
			routeKind,
			previewAdvancedProxyText(healingContext.SessionKey, 80),
			healingContext.AppliedHistoricalCut,
			advancedProxyEncryptedContentHealState.get(healingContext.SessionKey),
		)
	}
	if healingContext.SessionKey != "" && advancedProxyEncryptedContentHealState.get(healingContext.SessionKey) > 0 && containsEncryptedContentNeedle(preparedBody) {
		finalBody, finalStats, finalErr := finalizeOpenAIRequestForEncryptedContentHealing(preparedBody, healingContext.SessionKey)
		if finalErr != nil {
			message := formatAdvancedProxyFailure(appType, routeKind, provider, "", fmt.Sprintf("healed session final strip failed: %s", finalErr.Error()))
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_HEAL_FATAL] app=%s route=%s session=%s reason=final_strip_parse_failed hits=%d detail=%s",
				appType,
				routeKind,
				previewAdvancedProxyText(healingContext.SessionKey, 80),
				countEncryptedContentNeedle(preparedBody),
				previewAdvancedProxyText(message, 260),
			)
			return rawProviderAttemptResult{
				StatusCode: http.StatusInternalServerError,
				Message:    message,
				ErrorCode:  "encrypted_content_heal_failed",
				ErrorType:  "invalid_request_error",
				ProviderID: strings.TrimSpace(provider.ID),
				Provider:   providerLabel,
				TargetURL:  strings.TrimSpace(provider.BaseURL),
				RouteKind:  routeKind,
			}
		}
		preparedBody = finalBody
		if finalStats.RemovedFields > 0 || finalStats.RemovedIncludeRefs > 0 || finalStats.ScrubbedStrings > 0 {
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_HEAL_FINAL] app=%s route=%s session=%s removed_fields=%d removed_include_refs=%d scrubbed_strings=%d residual_hits=%d",
				appType,
				routeKind,
				previewAdvancedProxyText(healingContext.SessionKey, 80),
				finalStats.RemovedFields,
				finalStats.RemovedIncludeRefs,
				finalStats.ScrubbedStrings,
				finalStats.ResidualHits,
			)
		}
		if finalStats.ResidualHits > 0 {
			message := formatAdvancedProxyFailure(appType, routeKind, provider, "", fmt.Sprintf("healed session still contains encrypted_content after final strip (hits=%d)", finalStats.ResidualHits))
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_HEAL_FATAL] app=%s route=%s session=%s reason=residual_after_final_strip hits=%d detail=%s",
				appType,
				routeKind,
				previewAdvancedProxyText(healingContext.SessionKey, 80),
				finalStats.ResidualHits,
				previewAdvancedProxyText(message, 260),
			)
			return rawProviderAttemptResult{
				StatusCode: http.StatusInternalServerError,
				Message:    message,
				ErrorCode:  "encrypted_content_heal_failed",
				ErrorType:  "invalid_request_error",
				ProviderID: strings.TrimSpace(provider.ID),
				Provider:   providerLabel,
				TargetURL:  strings.TrimSpace(provider.BaseURL),
				RouteKind:  routeKind,
			}
		}
	}

	normalizedBody, resolvedModel, normalizeErr := normalizeOpenAIProxyRequestForProvider(preparedBody, provider)
	if normalizeErr != nil {
		message := formatAdvancedProxyFailure(appType, routeKind, provider, "", fmt.Sprintf("invalid upstream JSON request after normalization (%s)", normalizeErr.Error()))
		appendAdvancedProxyLogf(
			"[OPENAI_PROXY_NORMALIZE_FAIL] app=%s route=%s provider=%s detail=%s",
			appType,
			routeKind,
			providerLabel,
			previewAdvancedProxyText(normalizeErr.Error(), 260),
		)
		return rawProviderAttemptResult{
			StatusCode: http.StatusInternalServerError,
			Message:    message,
			ErrorCode:  "advanced_proxy_error",
			ErrorType:  "invalid_request_error",
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(provider.Model)
	}
	antiPoisonCtx := antiPoisonRequestContext{Config: sanitizeAntiPoisonConfig(config.AntiPoison), AppType: appType, RouteKind: routeKind}
	stringProtectionCtx := antiPoisonStringProtectionContext{}
	if config.AntiPoison.Enabled {
		guardedBody, guardCtx, guardErr := applyAntiPoisonPromptToOpenAIRequest(normalizedBody, routeKind, config.AntiPoison)
		if guardErr != nil {
			appendAdvancedProxyLogf(
				"[ANTI_POISON_PROMPT_FAIL] app=%s route=%s provider=%s detail=%s",
				appType,
				routeKind,
				providerLabel,
				previewAdvancedProxyText(guardErr.Error(), 220),
			)
		} else if guardCtx.Enabled {
			normalizedBody = guardedBody
			guardCtx.AppType = appType
			antiPoisonCtx = guardCtx
			appendAdvancedProxyLogf(
				"[ANTI_POISON_PROMPT_APPLY] app=%s route=%s provider=%s alias=%s guard=%s strategy=%d phrase=%d insertion=%s",
				appType,
				routeKind,
				providerLabel,
				previewAdvancedProxyText(guardCtx.Alias, 40),
				previewAdvancedProxyText(guardCtx.GuardToolName, 80),
				guardCtx.StrategySlot,
				guardCtx.PhraseVariant,
				previewAdvancedProxyText(guardCtx.InsertionPoint, 60),
			)
		}
	}
	if config.AntiPoison.Enabled && config.AntiPoison.StringProtection.Enabled {
		protectedBody, protectionCtx, protectionErr := applyAntiPoisonStringProtectionToJSONBody(normalizedBody, config.AntiPoison, routeKind, providerLabel, "openai")
		stringProtectionCtx = protectionCtx
		if protectionErr != nil {
			appendAdvancedProxyLogf(
				"[ANTI_POISON_STRING_PROTECT_FAIL] app=%s route=%s provider=%s detail=%s",
				appType,
				routeKind,
				providerLabel,
				previewAdvancedProxyText(protectionErr.Error(), 220),
			)
		} else if protectionCtx.Enabled {
			normalizedBody = protectedBody
			appendAdvancedProxyLogf(
				"[ANTI_POISON_STRING_PROTECT] app=%s route=%s provider=%s ops=%d placeholders=%d",
				appType,
				routeKind,
				providerLabel,
				len(protectionCtx.Records),
				len(protectionCtx.mapping),
			)
		}
	}

	type openAIProxyAttemptPhase struct {
		outboundRoute      string
		requestBody        []byte
		resolvedModel      string
		responseTransform  string
		preferenceValue    int
		preferenceScopeKey string
		source             string
		antiPoisonCtx      antiPoisonRequestContext
		stringProtect      antiPoisonStringProtectionContext
	}

	buildTargets := func(outboundRoute string) []string {
		switch outboundRoute {
		case "chat":
			return buildOpenAIChatCheckEndpointCandidates(provider.BaseURL)
		case "responses":
			return buildResponsesEndpointCandidates(provider.BaseURL)
		case "responses_compact":
			return buildResponsesCompactEndpointCandidates(provider.BaseURL)
		default:
			return nil
		}
	}

	phases := make([]openAIProxyAttemptPhase, 0, 2)
	appendPhase := func(phase openAIProxyAttemptPhase) {
		if len(phase.requestBody) == 0 || strings.TrimSpace(phase.outboundRoute) == "" {
			return
		}
		phases = append(phases, phase)
	}
	buildRouteTraceSnapshot := func(currentIndex int, currentStatus string) []AdvancedProxyRequestRouteStep {
		trace := make([]AdvancedProxyRequestRouteStep, 0, currentIndex+1)
		for index := 0; index < currentIndex && index < len(phases); index++ {
			trace = appendAdvancedProxyRouteTraceStep(trace, phases[index].outboundRoute, phases[index].source, "failed")
		}
		if currentIndex >= 0 && currentIndex < len(phases) {
			trace = appendAdvancedProxyRouteTraceStep(trace, phases[currentIndex].outboundRoute, phases[currentIndex].source, currentStatus)
		}
		return trace
	}

	switch routeKind {
	case "chat", "responses", "responses_compact":
	default:
		return rawProviderAttemptResult{
			StatusCode: http.StatusNotFound,
			Message:    formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "unsupported OpenAI proxy route"),
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}

	if routeKind == "responses" {
		fallbackPlan, fallbackErr := buildOpenAIChatFallbackPlanFromResponses(normalizedBody, provider)
		if fallbackErr != nil {
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_FALLBACK_PREPARE_FAIL] app=%s provider=%s route=%s detail=%s",
				appType,
				providerLabel,
				routeKind,
				previewAdvancedProxyText(fallbackErr.Error(), 260),
			)
		}
		if !fallbackPlan.SupportsChat && len(fallbackPlan.Blockers) > 0 {
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_FALLBACK_BLOCKED] app=%s provider=%s route=%s blockers=%s",
				appType,
				providerLabel,
				routeKind,
				previewAdvancedProxyText(strings.Join(fallbackPlan.Blockers, ","), 260),
			)
		}

		appendResponsesPhase := func(source string, preferenceValue int, preferenceScopeKey string) {
			appendPhase(openAIProxyAttemptPhase{
				outboundRoute:      "responses",
				requestBody:        normalizedBody,
				resolvedModel:      firstNonEmpty(resolvedModel, fallbackPlan.Model),
				preferenceValue:    preferenceValue,
				preferenceScopeKey: strings.TrimSpace(preferenceScopeKey),
				source:             source,
				antiPoisonCtx:      antiPoisonCtx,
				stringProtect:      stringProtectionCtx,
			})
		}
		appendChatPhase := func(source string, preferenceValue int, preferenceScopeKey string) {
			if fallbackPlan.SupportsChat {
				appendPhase(openAIProxyAttemptPhase{
					outboundRoute:      "chat",
					requestBody:        fallbackPlan.ChatBody,
					resolvedModel:      firstNonEmpty(fallbackPlan.Model, resolvedModel),
					responseTransform:  "chat_to_responses",
					preferenceValue:    preferenceValue,
					preferenceScopeKey: strings.TrimSpace(preferenceScopeKey),
					source:             source,
					antiPoisonCtx:      antiPoisonCtx,
					stringProtect:      stringProtectionCtx,
				})
			}
		}

		providerPreferredRoute := normalizeOpenAIProviderDispatchRoute(provider.APIFormat)
		if preferenceValue, ok := getAdvancedProxyOpenAIProtocolPreference(fallbackPlan.ScopeKey); ok {
			switch preferenceValue {
			case advancedProxyOpenAIProtocolPreferChat:
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_PREFERENCE_HIT] app=%s provider=%s scope=%s prefer=chat original_route=%s",
					appType,
					providerLabel,
					previewAdvancedProxyText(fallbackPlan.ScopeKey, 160),
					routeKind,
				)
				appendChatPhase("preference", advancedProxyOpenAIProtocolPreferChat, fallbackPlan.ScopeKey)
				appendResponsesPhase("fallback_restore", advancedProxyOpenAIProtocolPreferResponses, fallbackPlan.ScopeKey)
			case advancedProxyOpenAIProtocolPreferResponses:
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_PREFERENCE_HIT] app=%s provider=%s scope=%s prefer=responses original_route=%s",
					appType,
					providerLabel,
					previewAdvancedProxyText(fallbackPlan.ScopeKey, 160),
					routeKind,
				)
				appendResponsesPhase("preference", advancedProxyOpenAIProtocolPreferResponses, fallbackPlan.ScopeKey)
				appendChatPhase("fallback_restore", advancedProxyOpenAIProtocolPreferChat, fallbackPlan.ScopeKey)
			}
		}
		if len(phases) == 0 {
			switch providerPreferredRoute {
			case "chat":
				if fallbackPlan.SupportsChat {
					appendChatPhase("provider_config", advancedProxyOpenAIProtocolPreferChat, fallbackPlan.ScopeKey)
					appendResponsesPhase("fallback_restore", advancedProxyOpenAIProtocolPreferResponses, fallbackPlan.ScopeKey)
				} else {
					appendResponsesPhase("original", 0, "")
				}
			case "responses":
				appendResponsesPhase("provider_config", advancedProxyOpenAIProtocolPreferResponses, fallbackPlan.ScopeKey)
				appendChatPhase("fallback", advancedProxyOpenAIProtocolPreferChat, fallbackPlan.ScopeKey)
			default:
				appendResponsesPhase("original", 0, "")
				appendChatPhase("fallback", advancedProxyOpenAIProtocolPreferChat, fallbackPlan.ScopeKey)
			}
		}
		if len(phases) == 0 {
			appendResponsesPhase("original", 0, "")
		}
	} else {
		phaseSource := "original"
		if routeKind == "chat" && normalizeOpenAIProviderDispatchRoute(provider.APIFormat) == "chat" {
			phaseSource = "provider_config"
		}
		appendPhase(openAIProxyAttemptPhase{
			outboundRoute: routeKind,
			requestBody:   normalizedBody,
			resolvedModel: resolvedModel,
			source:        phaseSource,
			antiPoisonCtx: antiPoisonCtx,
			stringProtect: stringProtectionCtx,
		})
	}

	if len(phases) == 0 {
		return rawProviderAttemptResult{
			StatusCode: http.StatusBadGateway,
			Message:    formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "provider endpoint is empty"),
			ErrorCode:  "advanced_proxy_error",
			ErrorType:  "invalid_request_error",
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}

	lastStatus := http.StatusBadGateway
	lastMessage := formatAdvancedProxyFailure(appType, routeKind, provider, "", "no compatible upstream endpoint found")
	lastErrorCode := "advanced_proxy_error"
	lastErrorType := "invalid_request_error"
	for phaseIndex, phase := range phases {
		targets := buildTargets(phase.outboundRoute)
		if len(targets) == 0 {
			lastStatus = http.StatusBadGateway
			lastMessage = formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "provider endpoint is empty")
			lastErrorCode = "advanced_proxy_error"
			lastErrorType = "invalid_request_error"
			continue
		}
		phaseModel := resolveAdvancedProxyRecordedModel(phase.resolvedModel, phase.requestBody, provider.Model)

		advanceToNextPhase := false
		for _, targetURL := range targets {
			advancedProxyRuntime.MarkDispatch(appType, provider, phase.outboundRoute, targetURL)
			appendAdvancedProxyLogf(
				"[OPENAI_PROXY_TRY] app=%s route=%s provider=%s endpoint=%s stream=%t timeout=%ds outbound=%s source=%s client_route=%s",
				appType,
				phase.outboundRoute,
				providerLabel,
				targetURL,
				stream,
				timeoutSeconds,
				describeOutboundProxyMode(),
				phase.source,
				routeKind,
			)
			attemptStartedAt := time.Now()
			headerMappingModel := firstNonEmpty(originalRequestModel, phase.resolvedModel, phaseModel, provider.Model)
			statusCode, headers, body, streamBody, elapsed, err := performRawUpstreamRequest(http.MethodPost, targetURL, buildOpenAIProviderHeaders(provider, headerMappingModel), phase.requestBody, timeoutSeconds, stream)
			if err != nil {
				advancedProxyRuntime.MarkResult(appType, provider, phase.outboundRoute, targetURL, false)
				observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, err)
				message := formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, fmt.Sprintf("upstream request failed (%s, outbound=%s)", err.Error(), describeOutboundProxyMode()))
				appendAdvancedProxyLogf("[OPENAI_PROXY_ERROR] status=%d app=%s route=%s provider=%s endpoint=%s detail=%s", http.StatusBadGateway, appType, phase.outboundRoute, providerLabel, targetURL, previewAdvancedProxyText(message, 260))
				recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, nil, stream, http.StatusBadGateway, elapsed, message, buildRouteTraceSnapshot(phaseIndex, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, routeKind, providerLabel))
				return rawProviderAttemptResult{
					StatusCode: http.StatusBadGateway,
					Message:    message,
					ErrorCode:  "advanced_proxy_error",
					ErrorType:  "invalid_request_error",
					ProviderID: strings.TrimSpace(provider.ID),
					Provider:   providerLabel,
					TargetURL:  targetURL,
					RouteKind:  routeKind,
				}
			}
			if statusCode < 200 || statusCode >= 300 {
				advancedProxyRuntime.MarkResult(appType, provider, phase.outboundRoute, targetURL, false)
				observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, nil)
				lastStatus = statusCode
				lastMessage = formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, firstNonEmpty(summarizeAdvancedProxyBody(body), fmt.Sprintf("HTTP %d", statusCode)))
				lastErrorCode = "advanced_proxy_error"
				lastErrorType = "invalid_request_error"
				if healingMessage, healingCode, healingType, ok := isInvalidEncryptedContentError(statusCode, body); ok {
					if healingContext.SessionKey != "" && healingContext.OriginalCount > 0 {
						recordedCutoff := advancedProxyEncryptedContentHealState.record(healingContext.SessionKey, healingContext.OriginalCount)
						appendAdvancedProxyLogf(
							"[OPENAI_PROXY_HEAL_RECORD] app=%s route=%s session=%s cutoff=%d encrypted=%d stripped=%d",
							appType,
							routeKind,
							previewAdvancedProxyText(healingContext.SessionKey, 80),
							recordedCutoff,
							healingContext.OriginalCount,
							healingContext.AppliedHistoricalCut,
						)
					} else {
						appendAdvancedProxyLogf(
							"[OPENAI_PROXY_HEAL_MISS] app=%s route=%s session=%s encrypted=%d has_raw_hit=%t",
							appType,
							routeKind,
							previewAdvancedProxyText(healingContext.SessionKey, 80),
							healingContext.OriginalCount,
							containsEncryptedContentNeedle(rawBody),
						)
					}
					lastMessage = appendEncryptedContentHealingNotice(formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, healingMessage))
					lastErrorCode = healingCode
					lastErrorType = healingType
				}
				if !stream && stringProtectionCtx.Enabled {
					body = restoreAntiPoisonStringProtectionInJSONBody(body, &stringProtectionCtx, routeKind, providerLabel, "openai")
				}
				appendAdvancedProxyLogf("[OPENAI_PROXY_FAIL] status=%d app=%s route=%s provider=%s endpoint=%s detail=%s", statusCode, appType, phase.outboundRoute, providerLabel, targetURL, previewAdvancedProxyText(lastMessage, 260))
				recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, body, stream, statusCode, elapsed, lastMessage, buildRouteTraceSnapshot(phaseIndex, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, routeKind, providerLabel))
				if isRetryableCheckStatus(statusCode) {
					continue
				}
				if phaseIndex < len(phases)-1 {
					if phase.outboundRoute == "responses" && phases[phaseIndex+1].outboundRoute == "chat" && shouldFallbackResponsesToChat(statusCode, body) {
						appendAdvancedProxyLogf(
							"[OPENAI_PROXY_FALLBACK] app=%s provider=%s from=responses to=chat reason=%s",
							appType,
							providerLabel,
							previewAdvancedProxyText(summarizeAdvancedProxyBody(body), 220),
						)
						advanceToNextPhase = true
						break
					}
					if phase.outboundRoute == "chat" && phases[phaseIndex+1].outboundRoute == "responses" && shouldFallbackChatPreferenceBackToResponses(statusCode, body) {
						appendAdvancedProxyLogf(
							"[OPENAI_PROXY_CHAT_RESTORE] app=%s provider=%s scope=%s source=%s reason=%s",
							appType,
							providerLabel,
							previewAdvancedProxyText(phase.preferenceScopeKey, 160),
							phase.source,
							previewAdvancedProxyText(summarizeAdvancedProxyBody(body), 220),
						)
						advanceToNextPhase = true
						break
					}
				}
				return rawProviderAttemptResult{
					StatusCode: statusCode,
					Message:    lastMessage,
					ErrorCode:  lastErrorCode,
					ErrorType:  lastErrorType,
					Body:       body,
					Headers:    headers,
					ProviderID: strings.TrimSpace(provider.ID),
					Provider:   providerLabel,
					TargetURL:  targetURL,
					RouteKind:  routeKind,
				}
			}

			if phaseIndex < len(phases)-1 && phase.outboundRoute == "responses" && phases[phaseIndex+1].outboundRoute == "chat" && shouldFallbackSuccessfulResponsesToChat(statusCode, body) {
				advancedProxyRuntime.MarkResult(appType, provider, phase.outboundRoute, targetURL, false)
				observeAdvancedProxyAttempt(appType, provider, http.StatusBadGateway, elapsed, nil)
				if !stream && stringProtectionCtx.Enabled {
					body = restoreAntiPoisonStringProtectionInJSONBody(body, &stringProtectionCtx, routeKind, providerLabel, "openai")
				}
				lastStatus = http.StatusBadGateway
				lastMessage = formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, firstNonEmpty(summarizeAdvancedProxyBody(body), fmt.Sprintf("HTTP %d semantic error", statusCode)))
				lastErrorCode = "advanced_proxy_error"
				lastErrorType = "invalid_request_error"
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_SEMANTIC_FAIL] status=%d app=%s route=%s provider=%s endpoint=%s detail=%s",
					statusCode,
					appType,
					phase.outboundRoute,
					providerLabel,
					targetURL,
					previewAdvancedProxyText(lastMessage, 260),
				)
				recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, body, stream, lastStatus, elapsed, lastMessage, buildRouteTraceSnapshot(phaseIndex, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, routeKind, providerLabel))
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_FALLBACK] app=%s provider=%s from=responses to=chat reason=%s",
					appType,
					providerLabel,
					previewAdvancedProxyText(summarizeAdvancedProxyBody(body), 220),
				)
				advanceToNextPhase = true
				break
			}

			advancedProxyRuntime.MarkResult(appType, provider, phase.outboundRoute, targetURL, true)
			observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, nil)
			result := rawProviderAttemptResult{
				StatusCode: statusCode,
				Body:       body,
				Headers:    headers,
				StreamBody: streamBody,
				ProviderID: strings.TrimSpace(provider.ID),
				Provider:   providerLabel,
				TargetURL:  targetURL,
				RouteKind:  routeKind,
			}
			if phase.responseTransform == "chat_to_responses" {
				transformedResult, transformErr := transformOpenAIChatResultToResponses(result, firstNonEmpty(phaseModel, strings.TrimSpace(provider.Model), ""))
				if transformErr != nil {
					if streamBody != nil {
						_ = streamBody.Close()
					}
					lastStatus = http.StatusBadGateway
					lastMessage = formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, fmt.Sprintf("chat->responses transform failed (%s)", transformErr.Error()))
					lastErrorCode = "advanced_proxy_error"
					lastErrorType = "invalid_request_error"
					appendAdvancedProxyLogf(
						"[OPENAI_PROXY_TRANSFORM_FAIL] app=%s provider=%s from=chat to=responses endpoint=%s detail=%s",
						appType,
						providerLabel,
						targetURL,
						previewAdvancedProxyText(transformErr.Error(), 260),
					)
					recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, nil, stream, lastStatus, elapsed, lastMessage, buildRouteTraceSnapshot(phaseIndex, "failed"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, routeKind, providerLabel))
					if phaseIndex < len(phases)-1 {
						advanceToNextPhase = true
						break
					}
					return rawProviderAttemptResult{
						StatusCode: lastStatus,
						Message:    lastMessage,
						ErrorCode:  lastErrorCode,
						ErrorType:  lastErrorType,
						ProviderID: strings.TrimSpace(provider.ID),
						Provider:   providerLabel,
						TargetURL:  targetURL,
						RouteKind:  routeKind,
					}
				}
				result = transformedResult
				result.RecordCtx = nil
			}
			if phase.preferenceScopeKey != "" {
				setAdvancedProxyOpenAIProtocolPreference(phase.preferenceScopeKey, phase.preferenceValue)
				preferName := "chat"
				if phase.preferenceValue == advancedProxyOpenAIProtocolPreferResponses {
					preferName = "responses"
				}
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_PREFERENCE_SAVE] app=%s provider=%s scope=%s prefer=%s",
					appType,
					providerLabel,
					previewAdvancedProxyText(phase.preferenceScopeKey, 160),
					preferName,
				)
			}
			if !stream && antiPoisonCtx.Enabled {
				guardResult := validateAndStripAntiPoisonOpenAIResponse(result.Body, routeKind, antiPoisonCtx)
				appendAdvancedProxyLogf(
					"[ANTI_POISON_VALIDATE] app=%s route=%s provider=%s alias=%s valid=%t blocked=%t reason=%s real=%d guard=%d stripped=%d",
					appType,
					routeKind,
					providerLabel,
					previewAdvancedProxyText(antiPoisonCtx.Alias, 40),
					guardResult.Valid,
					guardResult.Blocked,
					previewAdvancedProxyText(guardResult.Reason, 120),
					guardResult.RealCount,
					guardResult.GuardCount,
					guardResult.RemovedGuards,
				)
				if guardResult.Blocked {
					result.StatusCode = http.StatusBadGateway
					result.Body = []byte(fmt.Sprintf(`{"error":{"message":"AllApiDeck anti-poison validation failed: %s","type":"invalid_request_error","code":"anti_poison_validation_failed"}}`, previewAdvancedProxyText(guardResult.Reason, 160)))
					result.StreamBody = nil
					result.Headers = result.Headers.Clone()
					result.Headers.Set("Content-Type", "application/json")
					result.Message = "AllApiDeck anti-poison validation failed: " + guardResult.Reason
					result.ErrorCode = "anti_poison_validation_failed"
					result.ErrorType = "invalid_request_error"
					result.AntiPoisonBlocked = true
					ops := appendAntiPoisonBlockedOperation(stringProtectionCtx.Records, routeKind, providerLabel, "openai", guardResult.Reason)
					recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, result.Body, false, result.StatusCode, elapsed, guardResult.Reason, buildRouteTraceSnapshot(phaseIndex, "failed"), annotateAntiPoisonStringProtectionRecords(ops, routeKind, providerLabel))
					return result
				} else if guardResult.Applied {
					result.Body = guardResult.Body
				}
			}
			if !stream && stringProtectionCtx.Enabled {
				result.Body = restoreAntiPoisonStringProtectionInJSONBody(result.Body, &stringProtectionCtx, routeKind, providerLabel, "openai")
			}
			if stream && result.StreamBody != nil {
				observedFormat := "chat"
				if phase.responseTransform == "chat_to_responses" || routeKind == "responses" || routeKind == "responses_compact" {
					observedFormat = "responses"
				}
				recordCtxObservedFormat := observedFormat
				if phase.responseTransform == "chat_to_responses" {
					recordCtxObservedFormat = "chat"
				}
				result.RecordCtx = &advancedProxyStreamRequestRecordContext{
					AppType:         appType,
					ClientRoute:     routeKind,
					InboundEndpoint: buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind),
					OutboundRoute:   phase.outboundRoute,
					RouteTrace:      buildRouteTraceSnapshot(phaseIndex, "success"),
					Source:          phase.source,
					Provider:        provider,
					TargetURL:       targetURL,
					RequestBody:     phase.requestBody,
					TimeoutSeconds:  timeoutSeconds,
					ResolvedModel:   phaseModel,
					StartedAt:       attemptStartedAt,
					ObservedFormat:  recordCtxObservedFormat,
					AntiPoisonCtx:   antiPoisonCtx,
					StringProtect:   stringProtectionCtx,
				}
			} else {
				recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, routeKind, buildAdvancedProxyOpenAIInboundEndpoint(appType, routeKind), phase.outboundRoute, phase.source, provider, targetURL, phase.requestBody, phaseModel, result.Body, stream, statusCode, elapsed, "", buildRouteTraceSnapshot(phaseIndex, "success"), annotateAntiPoisonStringProtectionRecords(stringProtectionCtx.Records, routeKind, providerLabel))
			}
			appendAdvancedProxyLogf("[OPENAI_PROXY_OK] status=%d app=%s route=%s provider=%s endpoint=%s stream=%t", statusCode, appType, phase.outboundRoute, providerLabel, targetURL, stream)
			return result
		}
		if advanceToNextPhase {
			continue
		}
	}

	return rawProviderAttemptResult{
		StatusCode: lastStatus,
		Message:    lastMessage,
		ErrorCode:  lastErrorCode,
		ErrorType:  lastErrorType,
		ProviderID: strings.TrimSpace(provider.ID),
		Provider:   providerLabel,
		RouteKind:  routeKind,
	}
}

func writeAnthropicSSE(writer http.ResponseWriter, response map[string]any) {
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	flusher, _ := writer.(http.Flusher)
	writeEvent := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(raw))
		if flusher != nil {
			flusher.Flush()
		}
	}

	messageID := firstNonEmpty(strings.TrimSpace(toStringValue(response["id"])), fmt.Sprintf("msg_%d", time.Now().UnixNano()))
	model := firstNonEmpty(strings.TrimSpace(toStringValue(response["model"])), "claude-proxy")
	usageMap, _ := response["usage"].(map[string]any)
	inputTokens := 0
	outputTokens := 0
	if usageMap != nil {
		inputTokens = toIntValue(usageMap["input_tokens"])
		outputTokens = toIntValue(usageMap["output_tokens"])
	}

	writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": 0,
			},
		},
	})

	contentList, _ := response["content"].([]any)
	for index, rawBlock := range contentList {
		blockMap, ok := rawBlock.(map[string]any)
		if !ok {
			continue
		}
		blockType := strings.TrimSpace(toStringValue(blockMap["type"]))
		switch blockType {
		case "tool_use":
			writeEvent("content_block_start", map[string]any{
				"type":          "content_block_start",
				"index":         index,
				"content_block": blockMap,
			})
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			})
		case "thinking":
			writeEvent("content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "",
				},
			})
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": toStringValue(blockMap["thinking"]),
				},
			})
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			})
		default:
			writeEvent("content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type": "text",
					"text": "",
				},
			})
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type": "text_delta",
					"text": toStringValue(blockMap["text"]),
				},
			})
			writeEvent("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": index,
			})
		}
	}

	writeEvent("message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   firstNonEmpty(strings.TrimSpace(toStringValue(response["stop_reason"])), "end_turn"),
			"stop_sequence": response["stop_sequence"],
		},
		"usage": map[string]any{
			"output_tokens": outputTokens,
		},
	})
	writeEvent("message_stop", map[string]any{"type": "message_stop"})
}

func writeAnthropicProxyError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "invalid_request_error",
			"message": firstNonEmpty(strings.TrimSpace(message), "advanced proxy request failed"),
		},
	})
}

func writeOpenAIProxyError(writer http.ResponseWriter, status int, message string, errorCode string, errorType string) {
	resolvedMessage := firstNonEmpty(strings.TrimSpace(message), "advanced proxy request failed")
	resolvedCode := firstNonEmpty(strings.TrimSpace(errorCode), "advanced_proxy_error")
	resolvedType := firstNonEmpty(strings.TrimSpace(errorType), "invalid_request_error")
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"message": resolvedMessage,
		"detail":  resolvedMessage,
		"error": map[string]any{
			"type":    resolvedType,
			"code":    resolvedCode,
			"message": resolvedMessage,
		},
	})
}

func writeOpenAIProxySuccess(writer http.ResponseWriter, result rawProviderAttemptResult, defaultContentType string) {
	if result.Headers != nil {
		for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id", "OpenAI-Processing-Ms"} {
			values := result.Headers.Values(key)
			for _, value := range values {
				if strings.TrimSpace(value) != "" {
					writer.Header().Add(key, value)
				}
			}
		}
	}
	if strings.TrimSpace(writer.Header().Get("Content-Type")) == "" {
		writer.Header().Set("Content-Type", defaultContentType)
	}
	statusCode := result.StatusCode
	if statusCode < 200 || statusCode >= 300 {
		statusCode = http.StatusOK
	}
	writer.WriteHeader(statusCode)
	if result.StreamBody != nil {
		if result.RecordCtx != nil {
			if err := proxyOpenAIStreamToClientWithMetrics(writer, result.StreamBody, result.RecordCtx); err != nil {
				appendAdvancedProxyLogf(
					"[OPENAI_PROXY_STREAM_FORWARD_FAIL] app=%s route=%s provider=%s endpoint=%s detail=%s",
					result.RecordCtx.AppType,
					result.RecordCtx.OutboundRoute,
					advancedProxyProviderLabel(result.RecordCtx.Provider),
					result.RecordCtx.TargetURL,
					previewAdvancedProxyText(err.Error(), 260),
				)
			}
			return
		}
		defer result.StreamBody.Close()
		_, _ = io.Copy(writer, result.StreamBody)
		return
	}
	if len(result.Body) > 0 {
		_, _ = writer.Write(result.Body)
	}
}

func (a *App) handleAdvancedProxyPing(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusOK)
		return
	}
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		writeAnthropicProxyError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok":            true,
		"enabled":       config.Enabled,
		"listenHost":    bridgeServerHost,
		"listenPort":    currentBridgeServerPort(),
		"providerCount": len(config.Queues.Global.Providers),
		"apps": map[string]any{
			"claude": map[string]any{
				"enabled":  config.Claude.Enabled,
				"basePath": config.Claude.BasePath,
			},
			"codex": map[string]any{
				"enabled":  config.Codex.Enabled,
				"basePath": config.Codex.BasePath,
			},
			"opencode": map[string]any{
				"enabled":  config.OpenCode.Enabled,
				"basePath": config.OpenCode.BasePath,
			},
			"openclaw": map[string]any{
				"enabled":  config.OpenClaw.Enabled,
				"basePath": config.OpenClaw.BasePath,
			},
		},
	})
}

func (a *App) handleAdvancedProxyClaude(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		writeAnthropicProxyError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	remoteIP := extractBridgeRemoteIP(request.RemoteAddr)
	if !isLoopbackBridgeRemote(remoteIP) {
		writeAnthropicProxyError(writer, http.StatusForbidden, "advanced proxy only accepts loopback requests")
		return
	}
	if !strings.HasSuffix(strings.TrimSpace(request.URL.Path), "/messages") {
		writeAnthropicProxyError(writer, http.StatusNotFound, "unsupported advanced proxy path")
		return
	}

	config, err := loadAdvancedProxyConfig()
	if err != nil {
		writeAnthropicProxyError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	providers := resolveAdvancedProxyEffectiveProviders(config, "claude")
	providers = advancedProxyRuntime.OrderProvidersForDispatch(config, "claude", providers)
	if !config.Enabled || !config.Claude.Enabled || len(providers) == 0 {
		writeAnthropicProxyError(writer, http.StatusServiceUnavailable, "advanced Claude proxy is disabled or has no providers")
		return
	}

	var requestBody map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, advancedProxyMaxRequestBodyBytes)).Decode(&requestBody); err != nil {
		writeAnthropicProxyError(writer, http.StatusBadRequest, fmt.Sprintf("invalid JSON request body: %v", err))
		return
	}

	requestFeatures := classifyClaudeRequestFeatures(requestBody)
	compatibleProviders := filterCompatibleClaudeProviders(providers, requestFeatures)
	if len(compatibleProviders) == 0 {
		writeAnthropicProxyError(writer, http.StatusBadRequest, incompatibleClaudeRequestMessage(requestFeatures))
		return
	}
	if len(compatibleProviders) != len(providers) && advancedProxyDebugEnabled(config) {
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_ROUTE_FILTER] feature=anthropic_web_search compatible=%d total=%d",
			len(compatibleProviders),
			len(providers),
		)
	}
	providers = compatibleProviders

	stream := truthy(requestBody["stream"])
	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled

	maxAttempts := 1
	if failoverActive {
		maxAttempts = clampInt(config.Failover.MaxRetries+1, 1, len(providers))
	}
	if requestFeatures.HasAnthropicWebSearchTool {
		maxAttempts = len(providers)
	}

	lastStatus := http.StatusBadGateway
	lastMessage := "no provider succeeded"
	attempted := 0
	for _, provider := range providers {
		if attempted >= maxAttempts {
			break
		}
		if failoverActive && !advancedProxyRuntime.Allow("claude", provider.ID, config.Failover) {
			continue
		}
		attempted++
		result := forwardClaudeRequestViaProvider(provider, requestBody, request.Header, stream, config)
		if result.Response != nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			if failoverActive {
				advancedProxyRuntime.Record("claude", provider.ID, config.Failover, true)
			}
			if stream {
				copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id")
				writeAnthropicSSE(writer, result.Response)
				return
			}
			copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id", "Cache-Control")
			writer.Header().Set("Content-Type", "application/json; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(writer).Encode(result.Response)
			return
		}
		if result.StreamBody != nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			if failoverActive {
				advancedProxyRuntime.Record("claude", provider.ID, config.Failover, true)
			}
			copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id")
			switch result.APIFormat {
			case "anthropic":
				writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
				writer.Header().Set("Cache-Control", "no-cache")
				writer.Header().Set("Connection", "keep-alive")
				writer.WriteHeader(http.StatusOK)
				if err := proxyAnthropicStreamToClientWithMetrics(writer, result.StreamBody, result.RecordCtx); err != nil {
					appendAdvancedProxyLogf(
						"[CLAUDE_PROXY_STREAM_FORWARD_FAIL] provider=%s endpoint=%s detail=%s",
						advancedProxyProviderLabel(result.RecordCtx.Provider),
						result.RecordCtx.TargetURL,
						previewAdvancedProxyText(err.Error(), 260),
					)
				}
				return
			case "openai_chat":
				writeAnthropicSSEFromOpenAIChatStreamWithRecord(writer, result.StreamBody, result.Model, anthropicThinkingEnabled(requestBody), result.RecordCtx)
			case "openai_responses":
				writeAnthropicSSEFromOpenAIResponsesStreamWithRecord(writer, result.StreamBody, result.Model, result.RecordCtx)
			default:
				result.StreamBody.Close()
				writeAnthropicProxyError(writer, http.StatusBadGateway, "unsupported Claude streaming proxy format")
			}
			return
		}
		if failoverActive {
			advancedProxyRuntime.Record("claude", provider.ID, config.Failover, false)
		}
		if result.AntiPoisonBlocked {
			writeAnthropicProxyError(writer, http.StatusBadGateway, firstNonEmpty(result.Message, "AllApiDeck anti-poison validation failed"))
			return
		}
		if result.StatusCode > 0 {
			lastStatus = result.StatusCode
		}
		if strings.TrimSpace(result.Message) != "" {
			lastMessage = result.Message
		}
	}
	if attempted == 0 && failoverActive && len(providers) > 0 {
		forcedProvider := providers[0]
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_FORCE_PROBE] provider=%s reason=all_candidates_blocked_by_circuit",
			advancedProxyProviderLabel(forcedProvider),
		)
		result := forwardClaudeRequestViaProvider(forcedProvider, requestBody, request.Header, stream, config)
		if result.Response != nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			advancedProxyRuntime.Record("claude", forcedProvider.ID, config.Failover, true)
			if stream {
				copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id")
				writeAnthropicSSE(writer, result.Response)
				return
			}
			copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id", "Cache-Control")
			writer.Header().Set("Content-Type", "application/json; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(writer).Encode(result.Response)
			return
		}
		if result.StreamBody != nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			advancedProxyRuntime.Record("claude", forcedProvider.ID, config.Failover, true)
			copySelectedHeaders(writer.Header(), result.Headers, "Request-Id", "X-Request-Id")
			switch result.APIFormat {
			case "anthropic":
				writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
				writer.Header().Set("Cache-Control", "no-cache")
				writer.Header().Set("Connection", "keep-alive")
				writer.WriteHeader(http.StatusOK)
				if err := proxyAnthropicStreamToClientWithMetrics(writer, result.StreamBody, result.RecordCtx); err != nil {
					appendAdvancedProxyLogf(
						"[CLAUDE_PROXY_STREAM_FORWARD_FAIL] provider=%s endpoint=%s detail=%s",
						advancedProxyProviderLabel(result.RecordCtx.Provider),
						result.RecordCtx.TargetURL,
						previewAdvancedProxyText(err.Error(), 260),
					)
				}
				return
			case "openai_chat":
				writeAnthropicSSEFromOpenAIChatStreamWithRecord(writer, result.StreamBody, result.Model, anthropicThinkingEnabled(requestBody), result.RecordCtx)
			case "openai_responses":
				writeAnthropicSSEFromOpenAIResponsesStreamWithRecord(writer, result.StreamBody, result.Model, result.RecordCtx)
			default:
				result.StreamBody.Close()
				writeAnthropicProxyError(writer, http.StatusBadGateway, "unsupported Claude streaming proxy format")
			}
			return
		}
		advancedProxyRuntime.Record("claude", forcedProvider.ID, config.Failover, false)
		if result.StatusCode > 0 {
			lastStatus = result.StatusCode
		}
		if strings.TrimSpace(result.Message) != "" {
			lastMessage = result.Message
		}
	}

	writeAnthropicProxyError(writer, lastStatus, lastMessage)
}

func (a *App) handleAdvancedProxyCodex(writer http.ResponseWriter, request *http.Request) {
	a.handleAdvancedProxyOpenAI("codex", writer, request)
}

func (a *App) handleAdvancedProxyOpenCode(writer http.ResponseWriter, request *http.Request) {
	a.handleAdvancedProxyOpenAI("opencode", writer, request)
}

func (a *App) handleAdvancedProxyOpenClaw(writer http.ResponseWriter, request *http.Request) {
	a.handleAdvancedProxyOpenAI("openclaw", writer, request)
}

func (a *App) handleAdvancedProxyOpenAI(appType string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		writeOpenAIProxyError(writer, http.StatusMethodNotAllowed, "method not allowed", "advanced_proxy_error", "invalid_request_error")
		return
	}

	remoteIP := extractBridgeRemoteIP(request.RemoteAddr)
	if !isLoopbackBridgeRemote(remoteIP) {
		writeOpenAIProxyError(writer, http.StatusForbidden, "advanced proxy only accepts loopback requests", "advanced_proxy_error", "invalid_request_error")
		return
	}

	path := strings.TrimSpace(request.URL.Path)
	routeKind := ""
	switch {
	case strings.HasSuffix(path, "/responses/compact"):
		routeKind = "responses_compact"
	case strings.HasSuffix(path, "/responses"):
		routeKind = "responses"
	case strings.HasSuffix(path, "/chat/completions"):
		routeKind = "chat"
	default:
		writeOpenAIProxyError(writer, http.StatusNotFound, "unsupported advanced proxy path", "advanced_proxy_error", "invalid_request_error")
		return
	}

	config, err := loadAdvancedProxyConfig()
	if err != nil {
		writeOpenAIProxyError(writer, http.StatusInternalServerError, err.Error(), "advanced_proxy_error", "invalid_request_error")
		return
	}
	providers := resolveAdvancedProxyEffectiveProviders(config, appType)
	providers = advancedProxyRuntime.OrderProvidersForDispatch(config, appType, providers)
	if !config.Enabled || !advancedProxyAppEnabled(config, appType) || len(providers) == 0 {
		writeOpenAIProxyError(writer, http.StatusServiceUnavailable, "advanced proxy is disabled or has no providers", "advanced_proxy_error", "invalid_request_error")
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, advancedProxyMaxRequestBodyBytes))
	if err != nil {
		writeOpenAIProxyError(writer, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err), "advanced_proxy_error", "invalid_request_error")
		return
	}
	requestBody := map[string]any{}
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		writeOpenAIProxyError(writer, http.StatusBadRequest, "invalid JSON request body", "advanced_proxy_error", "invalid_request_error")
		return
	}
	stream := truthy(requestBody["stream"])

	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled

	maxAttempts := 1
	if failoverActive {
		maxAttempts = clampInt(config.Failover.MaxRetries+1, 1, len(providers))
	}

	lastStatus := http.StatusBadGateway
	lastMessage := "no provider succeeded"
	lastErrorCode := "advanced_proxy_error"
	lastErrorType := "invalid_request_error"
	attempted := 0
	for _, provider := range providers {
		if attempted >= maxAttempts {
			break
		}
		if failoverActive && !advancedProxyRuntime.Allow(appType, provider.ID, config.Failover) {
			continue
		}
		attempted++
		result := forwardOpenAIRequestViaProvider(appType, provider, routeKind, rawBody, stream, config)
		if result.StatusCode >= 200 && result.StatusCode < 300 && (result.StreamBody != nil || result.Body != nil) {
			if failoverActive {
				advancedProxyRuntime.Record(appType, provider.ID, config.Failover, true)
			}
			defaultContentType := "application/json; charset=utf-8"
			if stream {
				defaultContentType = "text/event-stream; charset=utf-8"
			}
			writeOpenAIProxySuccess(writer, result, defaultContentType)
			return
		}
		if failoverActive {
			advancedProxyRuntime.Record(appType, provider.ID, config.Failover, false)
		}
		if result.AntiPoisonBlocked {
			writeOpenAIProxyError(writer, http.StatusBadGateway, firstNonEmpty(result.Message, "AllApiDeck anti-poison validation failed"), firstNonEmpty(result.ErrorCode, "anti_poison_validation_failed"), firstNonEmpty(result.ErrorType, "invalid_request_error"))
			return
		}
		if result.StatusCode > 0 {
			lastStatus = result.StatusCode
		}
		if strings.TrimSpace(result.Message) != "" {
			lastMessage = result.Message
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			lastErrorCode = result.ErrorCode
		}
		if strings.TrimSpace(result.ErrorType) != "" {
			lastErrorType = result.ErrorType
		}
	}
	if attempted == 0 && failoverActive && len(providers) > 0 {
		forcedProvider := providers[0]
		appendAdvancedProxyLogf(
			"[OPENAI_PROXY_FORCE_PROBE] app=%s provider=%s reason=all_candidates_blocked_by_circuit",
			appType,
			advancedProxyProviderLabel(forcedProvider),
		)
		result := forwardOpenAIRequestViaProvider(appType, forcedProvider, routeKind, rawBody, stream, config)
		if result.StatusCode >= 200 && result.StatusCode < 300 && (result.StreamBody != nil || result.Body != nil) {
			advancedProxyRuntime.Record(appType, forcedProvider.ID, config.Failover, true)
			defaultContentType := "application/json; charset=utf-8"
			if stream {
				defaultContentType = "text/event-stream; charset=utf-8"
			}
			writeOpenAIProxySuccess(writer, result, defaultContentType)
			return
		}
		advancedProxyRuntime.Record(appType, forcedProvider.ID, config.Failover, false)
		if result.StatusCode > 0 {
			lastStatus = result.StatusCode
		}
		if strings.TrimSpace(result.Message) != "" {
			lastMessage = result.Message
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			lastErrorCode = result.ErrorCode
		}
		if strings.TrimSpace(result.ErrorType) != "" {
			lastErrorType = result.ErrorType
		}
	}

	appendAdvancedProxyLogf("[OPENAI_PROXY_FINAL_FAIL] status=%d app=%s route=%s detail=%s", lastStatus, appType, routeKind, previewAdvancedProxyText(lastMessage, 260))
	writeOpenAIProxyError(writer, lastStatus, lastMessage, lastErrorCode, lastErrorType)
}
