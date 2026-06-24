package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	antiPoisonAlgorithmPlaceholder = "{{ALGORITHM_ALIAS}}"
	antiPoisonDefaultAlias         = "APTX9997"
	antiPoisonDefaultPrefix        = "aad_guard"
	antiPoisonDigestLength         = 16
	antiPoisonInjectionModeFixed   = "fixed_prepend"
	antiPoisonGuardJSONTagName     = "aad_guard_json"
	antiPoisonGuardJSONOpenTag     = "<aad_guard_json>"
	antiPoisonGuardJSONCloseTag    = "</aad_guard_json>"
)

var (
	antiPoisonGuardJSONPattern             = regexp.MustCompile(`(?is)<\s*aad_guard_json\s*>\s*(\{.*?\})\s*<\s*/\s*aad_guard_json\s*>`)
	antiPoisonGuardLikePattern             = regexp.MustCompile(`(?is)<\s*a\s*a\s*d\s*_?\s*g\s*u\s*a\s*r\s*d\s*_?\s*j\s*s\s*o\s*n\s*>\s*\{.*?\}\s*<\s*/\s*a\s*a\s*d\s*_?\s*g\s*u\s*a\s*r\s*d\s*_?\s*j\s*s\s*o\s*n\s*>`)
	antiPoisonHistoricalGuardRulePattern   = regexp.MustCompile("(?i)naming rule `aad_guard_[a-f0-9]{10}_[^`]+`")
	antiPoisonHistoricalGuardNamePattern   = regexp.MustCompile(`(?i)aad_guard_[a-f0-9]{10}_[A-Za-z0-9_.:-]+`)
	antiPoisonHistoricalFailureLinePattern = regexp.MustCompile(`(?im)^.*(?:AllApiDeck anti-poison validation failed|missing_guard_toolcall|guard_coverage_mismatch|guard_digest_mismatch).*$(?:\r?\n)?`)
)

type antiPoisonRequestContext struct {
	Enabled        bool
	Config         AntiPoisonConfig
	AppType        string
	RouteKind      string
	Alias          string
	Prefix         string
	GuardToolName  string
	Seed           string
	StrategySlot   int
	PhraseVariant  int
	InsertionPoint string
}

type antiPoisonToolCall struct {
	Container     map[string]any
	Kind          string
	Name          string
	CallID        string
	ArgumentsText string
	ToolType      string
	IsGuard       bool
}

type antiPoisonGuardPayload struct {
	Name      string
	ToolName  string
	ToolType  string
	Algorithm string
	Nonce     string
	Digest    string
	Chain     string
	Cover     string
	RawJSON   string
}

type antiPoisonGuardValidation struct {
	ValidGuardCount int
	DigestMismatch  bool
}

type antiPoisonTextGuardExtraction struct {
	Text        string
	GuardCount  int
	GuardBlocks []antiPoisonGuardPayload
}

type antiPoisonValidationResult struct {
	Applied       bool
	Valid         bool
	Blocked       bool
	Reason        string
	RealCount     int
	RealCalls     []antiPoisonToolCall
	GuardCount    int
	RemovedGuards int
	Body          []byte
}

type antiPoisonOperationRecord struct {
	ID       string `json:"id"`
	Time     string `json:"time"`
	Stage    string `json:"stage"`
	Channel  string `json:"channel"`
	Rule     string `json:"rule"`
	Path     string `json:"path"`
	Before   string `json:"before"`
	After    string `json:"after"`
	Context  string `json:"context"`
	Count    int    `json:"count"`
	Route    string `json:"route"`
	Provider string `json:"provider"`
	Blocked  bool   `json:"blocked"`
	Reason   string `json:"reason"`
}

type antiPoisonStringProtectionContext struct {
	Enabled bool
	Records []antiPoisonOperationRecord
	mapping map[string]string
	seq     int
}

type antiPoisonStringRestoreHit struct {
	Placeholder string
	Original    string
	Count       int
}

type antiPoisonStringProtectionRule struct {
	Description string
	Scope       string
	Pattern     string
	Regexp      *regexp.Regexp
}

var antiPoisonSensitiveToolFilePattern = regexp.MustCompile(`(?i)(^|[\\/])\.(env(?:\.[A-Za-z0-9_-]+)?|npmrc|pypirc|yarnrc|netrc|gitconfig|git-credentials)$`)

func newAntiPoisonRequestContext(routeKind string, config AntiPoisonConfig) antiPoisonRequestContext {
	config = sanitizeAntiPoisonConfig(config)
	if !config.Enabled {
		return antiPoisonRequestContext{Config: config, RouteKind: strings.TrimSpace(routeKind)}
	}
	return buildAntiPoisonRequestContextFromSeed(routeKind, config, randomAntiPoisonHex(8))
}

func (ctx *antiPoisonStringProtectionContext) addRecord(record antiPoisonOperationRecord) {
	if ctx == nil {
		return
	}
	ctx.seq++
	if strings.TrimSpace(record.ID) == "" {
		record.ID = fmt.Sprintf("apsp-%03d", ctx.seq)
	}
	if strings.TrimSpace(record.Time) == "" {
		record.Time = time.Now().Format(time.RFC3339Nano)
	}
	ctx.Records = append(ctx.Records, record)
}

func buildAntiPoisonStringProtectionContext(config AntiPoisonConfig) antiPoisonStringProtectionContext {
	config = sanitizeAntiPoisonConfig(config)
	return antiPoisonStringProtectionContext{
		Enabled: config.Enabled && config.StringProtection.Enabled,
		mapping: map[string]string{},
	}
}

func applyAntiPoisonStringProtectionToJSONBody(rawBody []byte, config AntiPoisonConfig, route string, provider string, channel string) ([]byte, antiPoisonStringProtectionContext, error) {
	ctx := buildAntiPoisonStringProtectionContext(config)
	if !ctx.Enabled || len(rawBody) == 0 {
		return rawBody, ctx, nil
	}
	var body any
	decoder := json.NewDecoder(strings.NewReader(string(rawBody)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		ctx.addRecord(antiPoisonOperationRecord{
			Stage:    "request out",
			Channel:  channel,
			Route:    route,
			Provider: provider,
			Reason:   "invalid_json_skip",
		})
		return rawBody, ctx, nil
	}
	rules := compileAntiPoisonStringProtectionRules(config.StringProtection)
	if len(rules) == 0 {
		return rawBody, ctx, nil
	}
	next := protectAntiPoisonStringValue(body, "$", rules, &ctx, route, provider, channel, false)
	nextRaw, err := json.Marshal(next)
	if err != nil {
		return rawBody, ctx, err
	}
	return nextRaw, ctx, nil
}

func restoreAntiPoisonStringProtectionInJSONBody(rawBody []byte, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) []byte {
	if ctx == nil || !ctx.Enabled || len(ctx.mapping) == 0 || len(rawBody) == 0 {
		return rawBody
	}
	var body any
	decoder := json.NewDecoder(strings.NewReader(string(rawBody)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err == nil {
		restored, count, hits := restoreAntiPoisonStringValueWithHits(body, ctx.mapping)
		if count > 0 {
			nextRaw, marshalErr := json.Marshal(restored)
			if marshalErr == nil {
				ctx.addRecord(antiPoisonOperationRecord{
					Stage:    "respond in",
					Channel:  channel,
					Route:    route,
					Provider: provider,
					Rule:     "string protection restore",
					Before:   summarizeAntiPoisonRestorePlaceholders(hits),
					After:    summarizeAntiPoisonRestoreOriginals(hits),
					Count:    count,
				})
				appendAdvancedProxyLogf(
					"[ANTI_POISON_STRING_RESTORE] route=%s provider=%s channel=%s placeholders=%d mode=json",
					previewAdvancedProxyText(route, 80),
					previewAdvancedProxyText(provider, 120),
					previewAdvancedProxyText(channel, 40),
					count,
				)
				return nextRaw
			}
			appendAdvancedProxyLogf("[ANTI_POISON_STRING_RESTORE_FAIL] route=%s provider=%s channel=%s detail=%s", previewAdvancedProxyText(route, 80), previewAdvancedProxyText(provider, 120), previewAdvancedProxyText(channel, 40), previewAdvancedProxyText(marshalErr.Error(), 180))
		}
		return rawBody
	}

	restored := string(rawBody)
	count := 0
	hits := make([]antiPoisonStringRestoreHit, 0, len(ctx.mapping))
	for placeholder, original := range ctx.mapping {
		placeholderHits := strings.Count(restored, placeholder)
		if placeholderHits <= 0 {
			continue
		}
		restored = strings.ReplaceAll(restored, placeholder, original)
		count += placeholderHits
		hits = appendAntiPoisonRestoreHits(hits, []antiPoisonStringRestoreHit{{Placeholder: placeholder, Original: original, Count: placeholderHits}})
	}
	if count > 0 {
		ctx.addRecord(antiPoisonOperationRecord{
			Stage:    "respond in",
			Channel:  channel,
			Route:    route,
			Provider: provider,
			Rule:     "string protection restore",
			Before:   summarizeAntiPoisonRestorePlaceholders(hits),
			After:    summarizeAntiPoisonRestoreOriginals(hits),
			Count:    count,
			Reason:   "invalid_json_raw_fallback",
		})
		appendAdvancedProxyLogf(
			"[ANTI_POISON_STRING_RESTORE] route=%s provider=%s channel=%s placeholders=%d mode=raw_fallback",
			previewAdvancedProxyText(route, 80),
			previewAdvancedProxyText(provider, 120),
			previewAdvancedProxyText(channel, 40),
			count,
		)
	}
	return []byte(restored)
}

func restoreAntiPoisonStringValue(value any, mapping map[string]string) (any, int) {
	restored, count, _ := restoreAntiPoisonStringValueWithHits(value, mapping)
	return restored, count
}

func restoreAntiPoisonStringValueWithHits(value any, mapping map[string]string) (any, int, []antiPoisonStringRestoreHit) {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		count := 0
		hits := make([]antiPoisonStringRestoreHit, 0)
		for key, child := range typed {
			restored, childCount, childHits := restoreAntiPoisonStringValueWithHits(child, mapping)
			next[key] = restored
			count += childCount
			hits = appendAntiPoisonRestoreHits(hits, childHits)
		}
		return next, count, hits
	case []any:
		next := make([]any, 0, len(typed))
		count := 0
		hits := make([]antiPoisonStringRestoreHit, 0)
		for _, child := range typed {
			restored, childCount, childHits := restoreAntiPoisonStringValueWithHits(child, mapping)
			next = append(next, restored)
			count += childCount
			hits = appendAntiPoisonRestoreHits(hits, childHits)
		}
		return next, count, hits
	case string:
		result := typed
		count := 0
		hits := make([]antiPoisonStringRestoreHit, 0)
		for placeholder, original := range mapping {
			placeholderHits := strings.Count(result, placeholder)
			if placeholderHits <= 0 {
				continue
			}
			result = strings.ReplaceAll(result, placeholder, original)
			count += placeholderHits
			hits = appendAntiPoisonRestoreHits(hits, []antiPoisonStringRestoreHit{{Placeholder: placeholder, Original: original, Count: placeholderHits}})
		}
		return result, count, hits
	default:
		return value, 0, nil
	}
}

func appendAntiPoisonRestoreHits(base []antiPoisonStringRestoreHit, additions []antiPoisonStringRestoreHit) []antiPoisonStringRestoreHit {
	for _, addition := range additions {
		merged := false
		for index := range base {
			if base[index].Placeholder == addition.Placeholder && base[index].Original == addition.Original {
				base[index].Count += addition.Count
				merged = true
				break
			}
		}
		if !merged {
			base = append(base, addition)
		}
	}
	return base
}

func summarizeAntiPoisonRestorePlaceholders(hits []antiPoisonStringRestoreHit) string {
	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		if hit.Count > 1 {
			parts = append(parts, fmt.Sprintf("%s x%d", hit.Placeholder, hit.Count))
			continue
		}
		parts = append(parts, hit.Placeholder)
	}
	return strings.Join(parts, "\n")
}

func summarizeAntiPoisonRestoreOriginals(hits []antiPoisonStringRestoreHit) string {
	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		if hit.Count > 1 {
			parts = append(parts, fmt.Sprintf("%s x%d", hit.Original, hit.Count))
			continue
		}
		parts = append(parts, hit.Original)
	}
	return strings.Join(parts, "\n")
}

func annotateAntiPoisonStringProtectionRecords(records []antiPoisonOperationRecord, route string, provider string) []antiPoisonOperationRecord {
	next := make([]antiPoisonOperationRecord, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Route) == "" {
			record.Route = route
		}
		if strings.TrimSpace(record.Provider) == "" {
			record.Provider = provider
		}
		next = append(next, record)
	}
	return next
}

func appendAntiPoisonBlockedOperation(records []antiPoisonOperationRecord, route string, provider string, channel string, reason string) []antiPoisonOperationRecord {
	record := antiPoisonOperationRecord{
		ID:       fmt.Sprintf("ap-block-%s", randomAntiPoisonHex(4)),
		Time:     time.Now().Format(time.RFC3339Nano),
		Stage:    "respond in",
		Channel:  strings.TrimSpace(channel),
		Rule:     "anti-poison validation failed",
		Path:     strings.TrimSpace(route),
		Before:   "upstream toolcall chain",
		After:    "blocked before client",
		Count:    1,
		Route:    strings.TrimSpace(route),
		Provider: strings.TrimSpace(provider),
		Blocked:  true,
		Reason:   strings.TrimSpace(reason),
	}
	return append(records, record)
}

func compileAntiPoisonStringProtectionRules(config AntiPoisonStringProtectionConfig) []antiPoisonStringProtectionRule {
	rules := make([]antiPoisonStringProtectionRule, 0, len(config.Rules))
	for _, rawRule := range config.Rules {
		description, scope, pattern := parseAntiPoisonStringProtectionRule(rawRule)
		if pattern == "" {
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			appendAdvancedProxyLogf("[ANTI_POISON_STRING_RULE_INVALID] rule=%s detail=%s", previewAdvancedProxyText(description, 120), previewAdvancedProxyText(err.Error(), 180))
			continue
		}
		rules = append(rules, antiPoisonStringProtectionRule{
			Description: description,
			Scope:       scope,
			Pattern:     pattern,
			Regexp:      compiled,
		})
		if len(rules) >= 80 {
			break
		}
	}
	return rules
}

func parseAntiPoisonStringProtectionRule(rawRule string) (string, string, string) {
	rawRule = strings.TrimSpace(rawRule)
	if rawRule == "" {
		return "", "", ""
	}
	for _, separator := range []string{": "} {
		if index := strings.Index(rawRule, separator); index > 0 {
			description := strings.TrimSpace(rawRule[:index])
			pattern := strings.TrimSpace(rawRule[index+len(separator):])
			if pattern != "" {
				scope, normalizedPattern := parseAntiPoisonStringProtectionRuleScope(pattern)
				return firstNonEmpty(description, normalizedPattern), scope, normalizedPattern
			}
		}
	}
	scope, pattern := parseAntiPoisonStringProtectionRuleScope(rawRule)
	return pattern, scope, pattern
}

func parseAntiPoisonStringProtectionRuleScope(pattern string) (string, string) {
	pattern = strings.TrimSpace(pattern)
	lower := strings.ToLower(pattern)
	switch {
	case strings.HasPrefix(lower, "key:"):
		return "key", strings.TrimSpace(pattern[len("key:"):])
	case strings.HasPrefix(lower, "path:"):
		return "path", strings.TrimSpace(pattern[len("path:"):])
	case strings.HasPrefix(lower, "text:"):
		return "text", strings.TrimSpace(pattern[len("text:"):])
	case strings.HasPrefix(lower, "user_text:"):
		return "user_text", strings.TrimSpace(pattern[len("user_text:"):])
	default:
		return "text", pattern
	}
}

func protectAntiPoisonStringValue(value any, path string, rules []antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string, userTextContext bool) any {
	switch typed := value.(type) {
	case map[string]any:
		if sensitiveField, sensitiveText, sensitiveRule, ok := detectAntiPoisonSensitiveToolContent(typed, path); ok {
			next := make(map[string]any, len(typed))
			for key, child := range typed {
				next[key] = child
			}
			next[sensitiveField] = protectAntiPoisonStringWholeText(sensitiveText, path+"."+sensitiveField, sensitiveRule, ctx, route, provider, channel)
			return next
		}
		next := make(map[string]any, len(typed))
		childUserTextContext := userTextContext || strings.EqualFold(strings.TrimSpace(toStringValue(typed["role"])), "user")
		for key, child := range typed {
			childPath := path + "." + key
			if matchedRule := matchAntiPoisonStringProtectionKeyRule(key, childPath, rules); matchedRule != nil {
				next[key] = protectAntiPoisonStringValueByRule(child, childPath, *matchedRule, ctx, route, provider, channel)
				continue
			}
			next[key] = protectAntiPoisonStringValue(child, childPath, rules, ctx, route, provider, channel, childUserTextContext)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for index, child := range typed {
			next = append(next, protectAntiPoisonStringValue(child, fmt.Sprintf("%s[%d]", path, index), rules, ctx, route, provider, channel, userTextContext))
		}
		return next
	case string:
		return protectAntiPoisonStringText(typed, path, rules, ctx, route, provider, channel, userTextContext)
	default:
		return value
	}
}

func matchAntiPoisonStringProtectionKeyRule(key string, path string, rules []antiPoisonStringProtectionRule) *antiPoisonStringProtectionRule {
	key = strings.TrimSpace(key)
	path = strings.TrimSpace(path)
	for index := range rules {
		rule := &rules[index]
		if rule.Regexp == nil {
			continue
		}
		switch strings.TrimSpace(rule.Scope) {
		case "key":
			if key != "" && rule.Regexp.MatchString(key) {
				return rule
			}
		case "path":
			if path != "" && rule.Regexp.MatchString(path) {
				return rule
			}
		}
	}
	return nil
}

func protectAntiPoisonStringValueByRule(value any, path string, rule antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, child := range typed {
			next[key] = protectAntiPoisonStringValue(child, path+"."+key, []antiPoisonStringProtectionRule{rule}, ctx, route, provider, channel, false)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for index, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			if _, ok := child.(string); ok {
				next = append(next, protectAntiPoisonStringValueByRule(child, childPath, rule, ctx, route, provider, channel))
			} else {
				next = append(next, protectAntiPoisonStringValue(child, childPath, []antiPoisonStringProtectionRule{rule}, ctx, route, provider, channel, false))
			}
		}
		return next
	case string:
		return protectAntiPoisonStringWholeText(typed, path, rule, ctx, route, provider, channel)
	default:
		return value
	}
}

func protectAntiPoisonStringWholeText(text string, path string, rule antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) string {
	if strings.TrimSpace(text) == "" || ctx == nil || strings.Contains(text, "__AAD_STR_") {
		return text
	}
	return storeAntiPoisonProtectedString(text, path, rule.Description, summarizeAntiPoisonPayloadContext(text, text), ctx, route, provider, channel)
}

func protectAntiPoisonStringText(text string, path string, rules []antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string, userTextContext bool) string {
	if text == "" || ctx == nil {
		return text
	}
	result := text
	for _, rule := range rules {
		scope := strings.TrimSpace(rule.Scope)
		if rule.Regexp == nil || (scope != "text" && scope != "user_text") {
			continue
		}
		if scope == "user_text" && !isAntiPoisonUserTextPath(path, userTextContext) {
			continue
		}
		originalText := result
		result = rule.Regexp.ReplaceAllStringFunc(result, func(match string) string {
			if match == "" || strings.Contains(match, "__AAD_STR_") {
				return match
			}
			if scope == "user_text" && isAntiPoisonReservedMarkup(match) {
				return match
			}
			return storeAntiPoisonProtectedString(match, path, rule.Description, summarizeAntiPoisonPayloadContext(originalText, match), ctx, route, provider, channel)
		})
	}
	return result
}

func isAntiPoisonUserTextPath(path string, userTextContext bool) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if path == "$.input" || path == "$.prompt" {
		return true
	}
	if strings.HasPrefix(path, "$.messages[") && strings.HasSuffix(path, ".content") {
		return userTextContext
	}
	if strings.HasPrefix(path, "$.input[") && strings.HasSuffix(path, ".content") {
		return userTextContext
	}
	if strings.HasPrefix(path, "$.input[") && !strings.Contains(path, ".content[") {
		return userTextContext
	}
	if !userTextContext {
		return false
	}
	if strings.HasPrefix(path, "$.messages[") && strings.HasSuffix(path, ".content") {
		return true
	}
	if strings.HasPrefix(path, "$.messages[") && strings.Contains(path, ".content[") {
		return strings.HasSuffix(path, ".text") || strings.HasSuffix(path, ".content")
	}
	if strings.HasPrefix(path, "$.input[") && strings.Contains(path, ".content[") {
		return strings.HasSuffix(path, ".text") || strings.HasSuffix(path, ".content")
	}
	return false
}

func isAntiPoisonReservedMarkup(match string) bool {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "<"), ">"))
	if inner == "" {
		return true
	}
	if strings.ContainsAny(inner, " \t/") {
		return true
	}
	reserved := map[string]struct{}{
		"command-args":         {},
		"command-message":      {},
		"command-name":         {},
		"local-command-caveat": {},
		"local-command-stdout": {},
		"system-reminder":      {},
		"tool_use_error":       {},
	}
	_, ok := reserved[strings.ToLower(inner)]
	return ok
}

func storeAntiPoisonProtectedString(original string, path string, ruleDescription string, context string, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) string {
	placeholder := fmt.Sprintf("__AAD_STR_%s_%03d__", randomAntiPoisonHex(4), len(ctx.mapping)+1)
	ctx.mapping[placeholder] = original
	ctx.addRecord(antiPoisonOperationRecord{
		Stage:    "request out",
		Channel:  channel,
		Rule:     ruleDescription,
		Path:     path,
		Before:   summarizeAntiPoisonProtectedText(original),
		After:    placeholder,
		Context:  context,
		Count:    1,
		Route:    route,
		Provider: provider,
	})
	return placeholder
}

func summarizeAntiPoisonPayloadContext(text string, match string) string {
	text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
	if text == "" {
		return ""
	}
	match = strings.TrimSpace(match)
	if match == "" {
		return previewAdvancedProxyText(text, 480)
	}
	index := strings.Index(text, match)
	if index < 0 {
		return previewAdvancedProxyText(text, 480)
	}
	start := index - 220
	if start < 0 {
		start = 0
	}
	end := index + len(match) + 220
	if end > len(text) {
		end = len(text)
	}
	context := strings.TrimSpace(text[start:end])
	if start > 0 {
		context = "..." + context
	}
	if end < len(text) {
		context += "..."
	}
	return context
}

func summarizeAntiPoisonProtectedText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty"
	}
	return text
}

func detectAntiPoisonSensitiveToolContent(value map[string]any, path string) (string, string, antiPoisonStringProtectionRule, bool) {
	normalizedPath := strings.ToLower(strings.TrimSpace(path))
	if !strings.Contains(normalizedPath, ".content[") {
		return "", "", antiPoisonStringProtectionRule{}, false
	}

	blockType := strings.TrimSpace(toStringValue(value["type"]))
	switch blockType {
	case "tool_result":
		if !antiPoisonSensitiveToolTextLooksLikeContent(value["content"]) {
			return "", "", antiPoisonStringProtectionRule{}, false
		}
		return "content", anthropicContentValueToText(value["content"]), antiPoisonStringProtectionRule{Description: "protect sensitive tool result"}, true
	case "function_call_output":
		if !antiPoisonSensitiveToolTextLooksLikeContent(value["output"]) {
			return "", "", antiPoisonStringProtectionRule{}, false
		}
		return "output", toStringValue(value["output"]), antiPoisonStringProtectionRule{Description: "protect sensitive tool result"}, true
	default:
		return "", "", antiPoisonStringProtectionRule{}, false
	}
}

func antiPoisonSensitiveToolTextLooksLikeContent(raw any) bool {
	text := strings.TrimSpace(toStringValue(raw))
	if text == "" {
		return false
	}
	if !antiPoisonLooksLikeSensitiveFileDump(text) {
		return false
	}
	return antiPoisonLooksLikeStructuredFileContent(text)
}

func antiPoisonLooksLikeSensitiveFileDump(text string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if antiPoisonSensitiveToolFilePattern.MatchString(line) {
			return true
		}
		if strings.Contains(line, "/.env") || strings.Contains(line, "\\.env") {
			return true
		}
	}
	return false
}

func antiPoisonLooksLikeStructuredFileContent(text string) bool {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.Contains(normalized, "BEGIN ") && strings.Contains(normalized, "PRIVATE KEY") {
		return true
	}
	if regexp.MustCompile(`(?im)^[A-Z0-9_]{2,}\s*=\s*.+$`).MatchString(normalized) {
		return true
	}
	if regexp.MustCompile(`(?im)"(api[_-]?key|secret|token|password|authorization|auth[_-]?token|private[_-]?key)"\s*:`).MatchString(normalized) {
		return true
	}
	if regexp.MustCompile(`(?im)^(api[_-]?key|secret|token|password|authorization|auth[_-]?token|private[_-]?key)\s*[:=]\s*.+$`).MatchString(normalized) {
		return true
	}
	return false
}

func buildAntiPoisonRequestContextFromSeed(routeKind string, config AntiPoisonConfig, seed string) antiPoisonRequestContext {
	config = sanitizeAntiPoisonConfig(config)
	seed = strings.ToLower(strings.TrimSpace(seed))
	if seed == "" {
		seed = randomAntiPoisonHex(8)
	}
	seedDigest := sha256Hex(seed)
	alias := "APTX_" + strings.ToUpper(seedDigest[:8])
	prefix := "aad_guard_" + seedDigest[8:18]
	strategyPoolSize := clampInt(config.Randomization.StrategyPoolSize, 1, 100)
	phraseVariants := clampInt(config.Randomization.MinPhraseVariantsPerStrategy, 1, 50)
	return antiPoisonRequestContext{
		Enabled:        config.Enabled,
		Config:         config,
		RouteKind:      strings.TrimSpace(routeKind),
		Alias:          alias,
		Prefix:         prefix,
		GuardToolName:  prefix + "_<original_tool_name>",
		Seed:           seed,
		StrategySlot:   1 + antiPoisonDerivedIndex(seed, "strategy", strategyPoolSize),
		PhraseVariant:  1 + antiPoisonDerivedIndex(seed, "phrase", phraseVariants),
		InsertionPoint: antiPoisonInjectionModeFixed,
	}
}

func normalizeAntiPoisonRequestContext(ctx antiPoisonRequestContext) antiPoisonRequestContext {
	ctx.Config = sanitizeAntiPoisonConfig(ctx.Config)
	ctx.AppType = strings.TrimSpace(ctx.AppType)
	ctx.RouteKind = strings.TrimSpace(ctx.RouteKind)
	ctx.Alias = strings.TrimSpace(ctx.Alias)
	if ctx.Alias == "" {
		ctx.Alias = antiPoisonDefaultAlias
	}
	ctx.Prefix = strings.TrimSpace(ctx.Prefix)
	if ctx.Prefix == "" {
		ctx.Prefix = antiPoisonDefaultPrefix
	}
	ctx.GuardToolName = strings.TrimSpace(ctx.GuardToolName)
	if ctx.GuardToolName == "" {
		ctx.GuardToolName = ctx.Prefix + "_<original_tool_name>"
	}
	ctx.Seed = strings.TrimSpace(ctx.Seed)
	if ctx.Seed == "" {
		ctx.Seed = "preview"
	}
	if ctx.StrategySlot <= 0 {
		ctx.StrategySlot = 1
	}
	if ctx.PhraseVariant <= 0 {
		ctx.PhraseVariant = 1
	}
	if strings.TrimSpace(ctx.InsertionPoint) == "" {
		ctx.InsertionPoint = antiPoisonInjectionModeFixed
	}
	return ctx
}

func randomAntiPoisonHex(byteCount int) string {
	if byteCount <= 0 {
		byteCount = 8
	}
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return sha256Hex(fmt.Sprintf("%d", time.Now().UnixNano()))[:byteCount*2]
}

func antiPoisonDerivedIndex(seed string, label string, modulo int) int {
	if modulo <= 1 {
		return 0
	}
	sum := sha256.Sum256([]byte(seed + ":" + label))
	return int(sum[0]) % modulo
}

func buildAntiPoisonPromptPreview(config AntiPoisonConfig, alias string, prefix string) string {
	config = sanitizeAntiPoisonConfig(config)
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = antiPoisonDefaultAlias
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = antiPoisonDefaultPrefix
	}
	return buildAntiPoisonPrompt(normalizeAntiPoisonRequestContext(antiPoisonRequestContext{
		Enabled:        true,
		Config:         config,
		Alias:          alias,
		Prefix:         prefix,
		GuardToolName:  prefix + "_<original_tool_name>",
		Seed:           "preview",
		StrategySlot:   1,
		PhraseVariant:  1,
		InsertionPoint: antiPoisonInjectionModeFixed,
	}))
}

func antiPoisonGuardToolPattern(ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	return ctx.Prefix + "_<original_tool_name>"
}

func antiPoisonGuardToolNameForTool(ctx antiPoisonRequestContext, toolName string) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return antiPoisonGuardToolPattern(ctx)
	}
	return ctx.Prefix + "_" + toolName
}

func normalizeAntiPoisonGuardToolBindingName(name string) string {
	name = strings.TrimSpace(name)
	for _, prefix := range []string{"functions.", "function.", "tools.", "tool."} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(name, prefix))
		}
	}
	return name
}

func antiPoisonGuardNameMatchesTool(ctx antiPoisonRequestContext, guardName string, realToolName string) bool {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	guardName = strings.TrimSpace(guardName)
	realToolName = strings.TrimSpace(realToolName)
	if guardName == "" || realToolName == "" {
		return false
	}
	if guardName == antiPoisonGuardToolNameForTool(ctx, realToolName) {
		return true
	}
	prefix := strings.TrimSpace(ctx.Prefix) + "_"
	if !strings.HasPrefix(guardName, prefix) {
		return false
	}
	guardToolName := strings.TrimSpace(strings.TrimPrefix(guardName, prefix))
	return normalizeAntiPoisonGuardToolBindingName(guardToolName) == normalizeAntiPoisonGuardToolBindingName(realToolName)
}

func antiPoisonExactRetryEligible(result antiPoisonValidationResult) bool {
	return false
}

func antiPoisonExactRetryRealCalls(calls []antiPoisonToolCall) []antiPoisonToolCall {
	next := make([]antiPoisonToolCall, 0, len(calls))
	for _, call := range calls {
		if call.IsGuard {
			continue
		}
		call.Name = strings.TrimSpace(call.Name)
		call.ArgumentsText = strings.TrimSpace(call.ArgumentsText)
		if call.Name == "" || call.ArgumentsText == "" {
			continue
		}
		call.ArgumentsText = canonicalAntiPoisonArgumentText(call.ArgumentsText)
		if strings.TrimSpace(call.ToolType) == "" {
			call.ToolType = classifyAntiPoisonToolName(call.Name)
		}
		next = append(next, call)
	}
	return next
}

func antiPoisonFirstRealCall(calls []antiPoisonToolCall) (antiPoisonToolCall, bool) {
	realCalls := antiPoisonExactRetryRealCalls(calls)
	if len(realCalls) == 0 {
		return antiPoisonToolCall{}, false
	}
	return realCalls[0], true
}

func antiPoisonObservedItemToToolCall(item *advancedProxyObservedItem) (antiPoisonToolCall, bool) {
	if item == nil || strings.TrimSpace(item.Type) != "function_call" {
		return antiPoisonToolCall{}, false
	}
	name := strings.TrimSpace(item.Name)
	args := strings.TrimSpace(item.ArgumentsPreview)
	if name == "" || args == "" {
		return antiPoisonToolCall{}, false
	}
	return antiPoisonToolCall{
		Name:          name,
		ArgumentsText: args,
		ToolType:      classifyAntiPoisonToolName(name),
	}, true
}

func buildAntiPoisonExactGuardPrompt(ctx antiPoisonRequestContext, call antiPoisonToolCall) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	call.Name = strings.TrimSpace(call.Name)
	guard := antiPoisonGuardJSONOpenTag + mustMarshalAntiPoisonJSONString(map[string]any{
		"name":      antiPoisonGuardToolNameForTool(ctx, call.Name),
		"tool_name": call.Name,
	}) + antiPoisonGuardJSONCloseTag
	return strings.Join([]string{
		"<important_gateway_rules>",
		"Exact retry is disabled; this template is retained only for legacy callers.",
		guard,
		"</important_gateway_rules>",
	}, "\n")
}

func buildAntiPoisonExactGuardPromptForCalls(ctx antiPoisonRequestContext, calls []antiPoisonToolCall) (string, error) {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	realCalls := antiPoisonExactRetryRealCalls(calls)
	if len(realCalls) == 0 {
		return "", fmt.Errorf("no real toolcalls for exact retry")
	}
	parts := make([]string, 0, len(realCalls))
	for _, call := range realCalls {
		parts = append(parts, antiPoisonGuardJSONOpenTag+mustMarshalAntiPoisonJSONString(map[string]any{
			"name":      antiPoisonGuardToolNameForTool(ctx, call.Name),
			"tool_name": call.Name,
		})+antiPoisonGuardJSONCloseTag)
	}
	return strings.Join([]string{
		"<important_gateway_rules>",
		"Exact retry is disabled; this template is retained only for legacy callers.",
		strings.Join(parts, "\n"),
		"</important_gateway_rules>",
	}, "\n"), nil
}
func buildAntiPoisonExactRetryOpenAIRequest(rawBody []byte, routeKind string, ctx antiPoisonRequestContext, call antiPoisonToolCall) ([]byte, error) {
	return buildAntiPoisonExactRetryOpenAIRequestForCalls(rawBody, routeKind, ctx, []antiPoisonToolCall{call})
}

func buildAntiPoisonExactRetryOpenAIRequestForCalls(rawBody []byte, routeKind string, ctx antiPoisonRequestContext, calls []antiPoisonToolCall) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, err
	}
	body = sanitizeAntiPoisonHistoricalContextMap(body)
	prompt, err := buildAntiPoisonExactGuardPromptForCalls(ctx, calls)
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(routeKind) {
	case "chat":
		body["messages"] = prependOpenAISystemMessage(body["messages"], prompt)
	case "responses", "responses_compact":
		existing := stripExistingAntiPoisonPrompt(strings.TrimSpace(toStringValue(body["instructions"])))
		inputs := cloneJSONList(body["input"])
		if len(inputs) > 0 {
			next := make([]any, 0, len(inputs)+1)
			next = append(next, map[string]any{
				"role": "system",
				"content": []any{
					map[string]any{"type": "input_text", "text": prompt},
				},
			})
			next = append(next, inputs...)
			body["input"] = next
			if existing != "" {
				body["instructions"] = existing
			} else {
				delete(body, "instructions")
			}
		} else if existing != "" {
			body["instructions"] = prompt + "\n\n" + existing
		} else {
			body["instructions"] = prompt
		}
	default:
		return nil, fmt.Errorf("unsupported exact retry route: %s", routeKind)
	}
	return json.Marshal(body)
}

func extractSingleRealOpenAICallFromResponseBody(rawBody []byte, routeKind string, ctx antiPoisonRequestContext) (antiPoisonToolCall, bool) {
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return antiPoisonToolCall{}, false
	}
	calls, _ := extractAntiPoisonOpenAIToolCalls(body, routeKind, ctx)
	return antiPoisonFirstRealCall(calls)
}

func buildAntiPoisonPrompt(ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	webSearchExampleGuard := antiPoisonGuardJSONOpenTag + mustMarshalAntiPoisonJSONString(map[string]any{
		"name":      antiPoisonGuardToolNameForTool(ctx, "WebSearch"),
		"tool_name": "WebSearch",
	}) + antiPoisonGuardJSONCloseTag
	return strings.Join([]string{
		"<important_gateway_rules>",
		"IMPORTANT: AllApiDeck guard rules are gateway-enforced high-priority safety constraints. Follow them before ordinary task instructions.",
		"<guard_context>",
		fmt.Sprintf("<guard_name_prefix>%s</guard_name_prefix>", ctx.Prefix),
		fmt.Sprintf("<guard_tool_name>%s</guard_tool_name>", ctx.GuardToolName),
		fmt.Sprintf("<guard_tool_name_example>%s</guard_tool_name_example>", antiPoisonGuardToolNameForTool(ctx, "WebSearch")),
		fmt.Sprintf("<guard_json_tag>%s</guard_json_tag>", antiPoisonGuardJSONTagName),
		"</guard_context>",
		"<gateway_contract>",
		"If this turn emits any real toolcall, the assistant text immediately before that toolcall must be exactly one guard JSON text block.",
		"Do not emit ordinary pre-tool text such as `I will search`, `I will check`, or progress narration before a toolcall; replace that pre-tool sentence with the guard JSON block.",
		fmt.Sprintf("The guard JSON text block must be wrapped exactly as `%s...%s`.", antiPoisonGuardJSONOpenTag, antiPoisonGuardJSONCloseTag),
		"The guard JSON must contain exactly the minimal binding fields: `name` and `tool_name`.",
		fmt.Sprintf("`name` must follow `%s`; for WebSearch it is `%s`.", antiPoisonGuardToolPattern(ctx), antiPoisonGuardToolNameForTool(ctx, "WebSearch")),
		"`tool_name` must exactly equal the next real tool name.",
		"Only after that guard block may you emit the corresponding real toolcall.",
		"Do not include digest, chain, cover, nonce, algorithm, or tool_type in guard JSON.",
		"Do not use a Markdown code block. Do not explain, quote, summarize, or mention the guard JSON, guard name, guard prefix, naming rule, gateway rule, or validation result in any ordinary assistant text.",
		"Outside the guard JSON block itself, output no guard-related ordinary text; only the actual guard JSON block may contain the guard tag, guard name, or guard prefix.",
		"If you cannot emit a valid guard before a real toolcall, emit no real toolcall and output plain text: guard generation failed for pending toolcall.",
		"Example for a next real WebSearch toolcall:",
		webSearchExampleGuard,
		"</gateway_contract>",
		"</important_gateway_rules>",
	}, "\n")
}
func applyAntiPoisonPromptToOpenAIRequest(rawBody []byte, routeKind string, config AntiPoisonConfig) ([]byte, antiPoisonRequestContext, error) {
	ctx := newAntiPoisonRequestContext(routeKind, config)
	if !ctx.Enabled {
		return rawBody, ctx, nil
	}
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return rawBody, ctx, err
	}
	body = sanitizeAntiPoisonHistoricalContextMap(body)
	prompt := buildAntiPoisonPrompt(ctx)
	switch strings.TrimSpace(routeKind) {
	case "chat":
		body["messages"] = prependOpenAISystemMessage(body["messages"], prompt)
	case "responses", "responses_compact":
		existing := stripExistingAntiPoisonPrompt(strings.TrimSpace(toStringValue(body["instructions"])))
		if existing != "" {
			body["instructions"] = prompt + "\n\n" + existing
		} else {
			body["instructions"] = prompt
		}
	default:
		ctx.Enabled = false
		return rawBody, ctx, nil
	}
	nextRaw, err := json.Marshal(body)
	if err != nil {
		return rawBody, ctx, err
	}
	return nextRaw, ctx, nil
}

func applyAntiPoisonPromptToAnthropicRequest(requestBody map[string]any, config AntiPoisonConfig) (map[string]any, antiPoisonRequestContext, error) {
	ctx := newAntiPoisonRequestContext("claude_messages", config)
	if !ctx.Enabled {
		return requestBody, ctx, nil
	}
	body := sanitizeAntiPoisonHistoricalContextMap(deepCopyJSONMap(requestBody))
	body["system"] = appendAntiPoisonAnthropicSystem(body["system"], buildAntiPoisonPrompt(ctx))
	return body, ctx, nil
}

func prependOpenAISystemMessage(rawMessages any, prompt string) []any {
	messages, _ := rawMessages.([]any)
	next := make([]any, 0, len(messages)+1)
	next = append(next, map[string]any{
		"role":    "system",
		"content": prompt,
	})
	next = append(next, messages...)
	return next
}

func appendAntiPoisonAnthropicSystem(rawSystem any, prompt string) any {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return rawSystem
	}
	switch typed := rawSystem.(type) {
	case string:
		existing := stripExistingAntiPoisonPrompt(strings.TrimSpace(typed))
		if existing == "" {
			return prompt
		}
		return prompt + "\n\n" + existing
	case []any:
		next := make([]any, 0, len(typed)+1)
		next = append(next, map[string]any{"type": "text", "text": prompt})
		for _, item := range typed {
			block, _ := item.(map[string]any)
			if block == nil {
				next = append(next, item)
				continue
			}
			if strings.TrimSpace(toStringValue(block["type"])) != "text" {
				next = append(next, item)
				continue
			}
			clean := stripExistingAntiPoisonPrompt(strings.TrimSpace(toStringValue(block["text"])))
			if clean == "" {
				continue
			}
			copied := deepCopyJSONMap(block)
			copied["text"] = clean
			next = append(next, copied)
		}
		return next
	case []map[string]any:
		next := make([]any, 0, len(typed)+1)
		next = append(next, map[string]any{"type": "text", "text": prompt})
		for _, item := range typed {
			if strings.TrimSpace(toStringValue(item["type"])) != "text" {
				next = append(next, item)
				continue
			}
			clean := stripExistingAntiPoisonPrompt(strings.TrimSpace(toStringValue(item["text"])))
			if clean == "" {
				continue
			}
			copied := deepCopyJSONMap(item)
			copied["text"] = clean
			next = append(next, copied)
		}
		return next
	default:
		return prompt
	}
}

func stripExistingAntiPoisonPrompt(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || !strings.Contains(text, "<important_gateway_rules>") {
		return text
	}
	pattern := regexp.MustCompile(`(?s)\s*<important_gateway_rules>.*?</important_gateway_rules>\s*`)
	clean := strings.TrimSpace(pattern.ReplaceAllString(text, "\n\n"))
	if clean == "" {
		return ""
	}
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(clean, "\n\n")
}

func sanitizeAntiPoisonHistoricalContextMap(body map[string]any) map[string]any {
	if body == nil {
		return body
	}
	cleaned, _ := sanitizeAntiPoisonHistoricalContextValue(body).(map[string]any)
	if cleaned == nil {
		return body
	}
	sanitizeAntiPoisonToolArgumentsInPlace(cleaned)
	return cleaned
}

func sanitizeAntiPoisonHistoricalContextValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, item := range typed {
			next[key] = sanitizeAntiPoisonHistoricalContextValue(item)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			next = append(next, sanitizeAntiPoisonHistoricalContextValue(item))
		}
		return next
	case []map[string]any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			next = append(next, sanitizeAntiPoisonHistoricalContextMap(item))
		}
		return next
	case string:
		return sanitizeAntiPoisonHistoricalContextText(typed)
	default:
		return value
	}
}

func sanitizeAntiPoisonHistoricalContextText(text string) string {
	if text == "" || (!strings.Contains(strings.ToLower(text), "guard") && !strings.Contains(text, "AllApiDeck anti-poison")) {
		return text
	}
	clean := stripExistingAntiPoisonPrompt(text)
	clean = antiPoisonGuardLikePattern.ReplaceAllString(clean, "")
	clean = antiPoisonHistoricalGuardRulePattern.ReplaceAllString(clean, "")
	clean = antiPoisonHistoricalGuardNamePattern.ReplaceAllString(clean, "")
	clean = antiPoisonHistoricalFailureLinePattern.ReplaceAllString(clean, "")
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	clean = regexp.MustCompile(`[ \t]{2,}`).ReplaceAllString(clean, " ")
	clean = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(clean, "\n")
	clean = regexp.MustCompile(`\n{3,}`).ReplaceAllString(clean, "\n\n")
	return strings.TrimSpace(clean)
}

func sanitizeAntiPoisonToolArgumentsInPlace(body map[string]any) {
	if body == nil {
		return
	}
	for _, rawItem := range anySliceValue(body["input"]) {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) != "function_call" {
			continue
		}
		if normalized, err := normalizeToolArgumentsJSON(item["arguments"]); err == nil {
			item["arguments"] = normalized
		}
	}
	for _, rawItem := range anySliceValue(body["output"]) {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) != "function_call" {
			continue
		}
		if normalized, err := normalizeToolArgumentsJSON(item["arguments"]); err == nil {
			item["arguments"] = normalized
		}
	}
	for _, rawMessage := range anySliceValue(body["messages"]) {
		message, _ := rawMessage.(map[string]any)
		for _, rawToolCall := range anySliceValue(message["tool_calls"]) {
			toolCall, _ := rawToolCall.(map[string]any)
			functionMap, _ := toolCall["function"].(map[string]any)
			if functionMap == nil {
				continue
			}
			if normalized, err := normalizeToolArgumentsJSON(functionMap["arguments"]); err == nil {
				functionMap["arguments"] = normalized
			}
		}
	}
}

func cloneJSONList(raw any) []any {
	switch typed := raw.(type) {
	case []any:
		return append([]any{}, typed...)
	case []map[string]any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			next = append(next, item)
		}
		return next
	default:
		return []any{}
	}
}

func validateAndStripAntiPoisonOpenAIResponse(rawBody []byte, routeKind string, ctx antiPoisonRequestContext) antiPoisonValidationResult {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	result := antiPoisonValidationResult{Body: rawBody}
	if !ctx.Enabled || len(rawBody) == 0 {
		return result
	}
	result.Applied = true
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = "invalid_response_json"
		return result
	}

	calls, guardCount := extractAntiPoisonOpenAIToolCalls(body, routeKind, ctx)
	return validateAndStripAntiPoisonToolCalls(
		rawBody,
		calls,
		ctx,
		func() []byte {
			return mustMarshalAntiPoisonBody(stripAntiPoisonOpenAIGuards(body, routeKind), rawBody)
		},
		guardCount,
	)
}

func validateAndStripAntiPoisonAnthropicResponse(rawBody []byte, ctx antiPoisonRequestContext) antiPoisonValidationResult {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	result := antiPoisonValidationResult{Body: rawBody}
	if !ctx.Enabled || len(rawBody) == 0 {
		return result
	}
	result.Applied = true
	var body map[string]any
	if err := json.Unmarshal(rawBody, &body); err != nil {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = "invalid_response_json"
		return result
	}

	calls, guardCount := extractAntiPoisonAnthropicToolCalls(body, ctx)
	return validateAndStripAntiPoisonToolCalls(
		rawBody,
		calls,
		ctx,
		func() []byte {
			return mustMarshalAntiPoisonBody(stripAntiPoisonAnthropicGuards(body), rawBody)
		},
		guardCount,
	)
}

func validateAndStripAntiPoisonToolCalls(rawBody []byte, calls []antiPoisonToolCall, ctx antiPoisonRequestContext, stripGuards func() []byte, observedGuardCount int) antiPoisonValidationResult {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	result := antiPoisonValidationResult{Applied: true, Body: rawBody}
	realCalls := make([]antiPoisonToolCall, 0, len(calls))
	guardCalls := make([]antiPoisonToolCall, 0, len(calls))
	for _, call := range calls {
		if call.IsGuard {
			guardCalls = append(guardCalls, call)
		} else if antiPoisonToolCallRequiresGuard(call, ctx) {
			realCalls = append(realCalls, call)
		}
	}
	result.RealCount = len(realCalls)
	if len(realCalls) > 0 {
		result.RealCalls = append([]antiPoisonToolCall(nil), realCalls...)
	}
	result.GuardCount = maxInt(len(guardCalls), observedGuardCount)
	if len(realCalls) == 0 {
		result.Valid = true
		if result.GuardCount > 0 && stripGuards != nil {
			result.RemovedGuards = result.GuardCount
			result.Body = stripGuards()
		}
		return result
	}

	minGuardCount := clampInt(ctx.Config.Randomization.MinFakeToolcalls, 1, 20)
	if result.GuardCount < minGuardCount {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = antiPoisonValidationReasonMissingGuard(len(realCalls), result.GuardCount, minGuardCount, ctx)
		if !result.Blocked && result.GuardCount > 0 && stripGuards != nil {
			result.RemovedGuards = result.GuardCount
			result.Body = stripGuards()
		}
		return result
	}

	guardValidation := validateAntiPoisonGuardCoverage(guardCalls, realCalls, ctx)
	if guardValidation.ValidGuardCount < len(realCalls) {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = antiPoisonValidationReasonGuardCoverageMismatch(len(realCalls), result.GuardCount, guardValidation.ValidGuardCount, ctx)
		if !result.Blocked && stripGuards != nil {
			result.RemovedGuards = result.GuardCount
			result.Body = stripGuards()
		}
		return result
	}

	result.Valid = true
	if guardValidation.DigestMismatch {
		result.Reason = antiPoisonValidationReasonGuardDigestMismatch(len(realCalls), result.GuardCount, ctx)
	}
	if result.GuardCount > 0 && stripGuards != nil {
		result.RemovedGuards = result.GuardCount
		result.Body = stripGuards()
	}
	return result
}

func antiPoisonToolCallRequiresGuard(call antiPoisonToolCall, ctx antiPoisonRequestContext) bool {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	if call.IsGuard {
		return false
	}
	normalizedName := normalizeAntiPoisonGuardToolBindingName(call.Name)
	if strings.EqualFold(ctx.AppType, "codex") &&
		strings.EqualFold(strings.TrimSpace(call.Kind), "responses.function_call") &&
		strings.EqualFold(normalizedName, "WebSearch") {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(call.Kind), "responses.web_search_call")
}

func antiPoisonShouldBlock(config AntiPoisonConfig) bool {
	config = sanitizeAntiPoisonConfig(config)
	return config.StrictMode && config.FailureMode != "warn"
}

func mustMarshalAntiPoisonBody(body map[string]any, fallback []byte) []byte {
	raw, err := json.Marshal(body)
	if err != nil {
		return fallback
	}
	return raw
}

func mustMarshalAntiPoisonJSONString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func extractAntiPoisonGuardsFromText(text string, ctx antiPoisonRequestContext) antiPoisonTextGuardExtraction {
	text = firstNonEmptyExact(text)
	if strings.TrimSpace(text) == "" {
		return antiPoisonTextGuardExtraction{Text: text}
	}
	guards := make([]antiPoisonGuardPayload, 0, 4)
	stripped := antiPoisonGuardJSONPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := antiPoisonGuardJSONPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return ""
		}
		rawJSON := strings.TrimSpace(submatches[1])
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
			return ""
		}
		name := strings.TrimSpace(toStringValue(payload["name"]))
		toolName := strings.TrimSpace(toStringValue(payload["tool_name"]))
		toolType := strings.TrimSpace(toStringValue(payload["tool_type"]))
		if toolType == "" && toolName != "" {
			toolType = classifyAntiPoisonToolName(toolName)
		}
		guards = append(guards, antiPoisonGuardPayload{
			Name:      name,
			ToolName:  toolName,
			ToolType:  toolType,
			Algorithm: strings.TrimSpace(toStringValue(payload["algorithm"])),
			Nonce:     strings.TrimSpace(toStringValue(payload["nonce"])),
			Digest:    strings.TrimSpace(toStringValue(payload["digest"])),
			Chain:     strings.TrimSpace(toStringValue(payload["chain"])),
			Cover:     strings.TrimSpace(toStringValue(payload["cover"])),
			RawJSON:   rawJSON,
		})
		return ""
	})
	return antiPoisonTextGuardExtraction{
		Text:        strings.TrimSpace(cleanAntiPoisonGuardText(stripped)),
		GuardCount:  len(guards),
		GuardBlocks: guards,
	}
}

func antiPoisonGuardPayloadsToToolCalls(payloads []antiPoisonGuardPayload) []antiPoisonToolCall {
	calls := make([]antiPoisonToolCall, 0, len(payloads))
	for _, payload := range payloads {
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			name = strings.TrimSpace(payload.ToolName)
		}
		toolType := strings.TrimSpace(payload.ToolType)
		if toolType == "" {
			toolType = classifyAntiPoisonToolName(payload.ToolName)
		}
		calls = append(calls, antiPoisonToolCall{
			Kind:          "guard.json",
			Name:          name,
			ArgumentsText: payload.RawJSON,
			ToolType:      toolType,
			IsGuard:       true,
		})
	}
	return calls
}

func cleanAntiPoisonGuardText(text string) string {
	text = antiPoisonGuardLikePattern.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	return text
}

func extractAntiPoisonOpenAIToolCalls(body map[string]any, routeKind string, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	switch strings.TrimSpace(routeKind) {
	case "chat":
		return extractAntiPoisonChatToolCalls(body, ctx)
	case "responses", "responses_compact":
		return extractAntiPoisonResponsesToolCalls(body, ctx)
	default:
		return nil, 0
	}
}

func extractAntiPoisonChatToolCalls(body map[string]any, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	choices, _ := body["choices"].([]any)
	calls := []antiPoisonToolCall{}
	guards := make([]antiPoisonGuardPayload, 0, 4)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}
		guardText := extractAntiPoisonGuardsFromText(toStringValue(message["content"]), ctx)
		guards = append(guards, guardText.GuardBlocks...)
		if toolCalls, ok := message["tool_calls"].([]any); ok {
			for _, rawCall := range toolCalls {
				callMap, _ := rawCall.(map[string]any)
				functionMap, _ := callMap["function"].(map[string]any)
				name := strings.TrimSpace(toStringValue(functionMap["name"]))
				calls = append(calls, antiPoisonToolCall{
					Container:     callMap,
					Kind:          "chat.tool_call",
					Name:          name,
					CallID:        strings.TrimSpace(toStringValue(callMap["id"])),
					ArgumentsText: toStringValue(functionMap["arguments"]),
					ToolType:      classifyAntiPoisonToolName(name),
					IsGuard:       false,
				})
			}
		}
		if functionMap, ok := message["function_call"].(map[string]any); ok {
			name := strings.TrimSpace(toStringValue(functionMap["name"]))
			calls = append(calls, antiPoisonToolCall{
				Container:     functionMap,
				Kind:          "chat.function_call",
				Name:          name,
				ArgumentsText: toStringValue(functionMap["arguments"]),
				ToolType:      classifyAntiPoisonToolName(name),
				IsGuard:       false,
			})
		}
	}
	return append(calls, antiPoisonGuardPayloadsToToolCalls(guards)...), len(guards)
}

func extractAntiPoisonResponsesToolCalls(body map[string]any, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	output, _ := body["output"].([]any)
	calls := []antiPoisonToolCall{}
	guards := make([]antiPoisonGuardPayload, 0, 4)
	for _, rawItem := range output {
		item, _ := rawItem.(map[string]any)
		switch strings.TrimSpace(toStringValue(item["type"])) {
		case "function_call":
			name := strings.TrimSpace(toStringValue(item["name"]))
			calls = append(calls, antiPoisonToolCall{
				Container:     item,
				Kind:          "responses.function_call",
				Name:          name,
				CallID:        strings.TrimSpace(toStringValue(item["call_id"])),
				ArgumentsText: toStringValue(item["arguments"]),
				ToolType:      classifyAntiPoisonToolName(name),
				IsGuard:       false,
			})
		case "web_search_call":
			calls = append(calls, antiPoisonToolCall{
				Container:     item,
				Kind:          "responses.web_search_call",
				Name:          "web_search_call",
				CallID:        strings.TrimSpace(toStringValue(item["id"])),
				ArgumentsText: stringifyJSON(item["action"]),
				ToolType:      classifyAntiPoisonToolName("web_search_call"),
				IsGuard:       false,
			})
		case "message":
			for _, rawContent := range anySliceValue(item["content"]) {
				contentMap, _ := rawContent.(map[string]any)
				extracted := extractAntiPoisonGuardsFromText(
					firstNonEmptyExact(toStringValue(contentMap["text"]), toStringValue(contentMap["content"])),
					ctx,
				)
				guards = append(guards, extracted.GuardBlocks...)
			}
		}
	}
	return append(calls, antiPoisonGuardPayloadsToToolCalls(guards)...), len(guards)
}

func extractAntiPoisonAnthropicToolCalls(body map[string]any, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	content, _ := body["content"].([]any)
	calls := []antiPoisonToolCall{}
	guards := make([]antiPoisonGuardPayload, 0, 4)
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		switch strings.TrimSpace(toStringValue(block["type"])) {
		case "tool_use":
			name := strings.TrimSpace(toStringValue(block["name"]))
			calls = append(calls, antiPoisonToolCall{
				Container:     block,
				Kind:          "anthropic.tool_use",
				Name:          name,
				CallID:        strings.TrimSpace(toStringValue(block["id"])),
				ArgumentsText: stringifyJSON(block["input"]),
				ToolType:      classifyAntiPoisonToolName(name),
				IsGuard:       false,
			})
		case "text", "thinking", "redacted_thinking":
			extracted := extractAntiPoisonGuardsFromText(firstNonEmptyExact(toStringValue(block["text"]), toStringValue(block["thinking"])), ctx)
			guards = append(guards, extracted.GuardBlocks...)
		}
	}
	return append(calls, antiPoisonGuardPayloadsToToolCalls(guards)...), len(guards)
}

func classifyAntiPoisonToolName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "shell"), strings.Contains(lower, "command"), strings.Contains(lower, "exec"):
		return "command"
	case strings.Contains(lower, "web"), strings.Contains(lower, "http"), strings.Contains(lower, "fetch"):
		return "network"
	case strings.Contains(lower, "read"), strings.Contains(lower, "file"), strings.Contains(lower, "grep"), strings.Contains(lower, "search"), strings.Contains(lower, "rg"):
		return "read"
	default:
		return "other"
	}
}

func computeAntiPoisonToolChainDigest(calls []antiPoisonToolCall, ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	parts := make([]string, 0, len(calls)+2)
	parts = append(parts, "alias="+strings.TrimSpace(ctx.Alias))
	parts = append(parts, "nonce="+strings.TrimSpace(ctx.Seed))
	for index, call := range calls {
		toolType := strings.TrimSpace(call.ToolType)
		if toolType == "" {
			toolType = classifyAntiPoisonToolName(call.Name)
		}
		parts = append(parts, fmt.Sprintf(
			"%d|%s|%s|%s",
			index,
			toolType,
			call.Name,
			sha256Hex(canonicalAntiPoisonArgumentText(call.ArgumentsText)),
		))
	}
	return sha256Hex(strings.Join(parts, "\n"))[:antiPoisonDigestLength]
}

func antiPoisonValidationReasonMissingGuard(realCount int, guardCount int, minGuardCount int, ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	return fmt.Sprintf(
		"missing_guard_toolcall: detected %d real toolcall(s) but only %d guard json block(s); requires >= %d guard block(s) using `%s...%s`, naming rule `%s`, and each guard must minimally bind the next real toolcall with fields `name` and `tool_name`",
		realCount,
		guardCount,
		minGuardCount,
		antiPoisonGuardJSONOpenTag,
		antiPoisonGuardJSONCloseTag,
		antiPoisonGuardToolPattern(ctx),
	)
}

func antiPoisonValidationReasonGuardCoverageMismatch(realCount int, guardCount int, matchedGuardCount int, ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	return fmt.Sprintf(
		"guard_coverage_mismatch: detected %d real toolcall(s), %d guard json block(s), but only %d guard block(s) matched the required minimal binding fields name/tool_name; requires one valid guard block per real toolcall using `%s...%s`, naming rule `%s`",
		realCount,
		guardCount,
		matchedGuardCount,
		antiPoisonGuardJSONOpenTag,
		antiPoisonGuardJSONCloseTag,
		antiPoisonGuardToolPattern(ctx),
	)
}

func antiPoisonValidationReasonGuardDigestMismatch(realCount int, guardCount int, ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	return fmt.Sprintf(
		"guard_digest_mismatch: detected %d real toolcall(s) and %d guard json block(s); this field is ignored in minimal guard mode",
		realCount,
		guardCount,
	)
}

func validateAntiPoisonGuardCoverage(guards []antiPoisonToolCall, realCalls []antiPoisonToolCall, ctx antiPoisonRequestContext) antiPoisonGuardValidation {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	result := antiPoisonGuardValidation{}
	if len(guards) == 0 || len(realCalls) == 0 {
		return result
	}
	used := make([]bool, len(guards))
	for index, realCall := range realCalls {
		for guardIndex, guard := range guards {
			if used[guardIndex] || !antiPoisonGuardMatchesRealCall(guard, realCall, index, ctx) {
				continue
			}
			used[guardIndex] = true
			result.ValidGuardCount++
			break
		}
	}
	return result
}

func antiPoisonGuardMatchesRealCall(guard antiPoisonToolCall, realCall antiPoisonToolCall, index int, ctx antiPoisonRequestContext) bool {
	var payload map[string]any
	if err := json.Unmarshal([]byte(guard.ArgumentsText), &payload); err != nil {
		return false
	}
	guardName := strings.TrimSpace(toStringValue(payload["name"]))
	guardToolName := strings.TrimSpace(toStringValue(payload["tool_name"]))
	guardToolType := strings.TrimSpace(toStringValue(payload["tool_type"]))
	if guardToolType == "" {
		guardToolType = classifyAntiPoisonToolName(guardToolName)
	}
	expectedToolType := strings.TrimSpace(realCall.ToolType)
	if expectedToolType == "" {
		expectedToolType = classifyAntiPoisonToolName(realCall.Name)
	}
	if !antiPoisonGuardNameMatchesTool(ctx, guardName, realCall.Name) {
		return false
	}
	if normalizeAntiPoisonGuardToolBindingName(guardToolName) != normalizeAntiPoisonGuardToolBindingName(realCall.Name) {
		return false
	}
	_ = guardToolType
	_ = expectedToolType
	_ = index
	return true
}

func antiPoisonDigestPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-f]{16}$`)
}

func canonicalAntiPoisonArgumentText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	return canonicalAntiPoisonJSON(value)
}

func canonicalAntiPoisonJSON(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%q:%s", key, canonicalAntiPoisonJSON(typed[key])))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, canonicalAntiPoisonJSON(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case json.Number:
		return typed.String()
	case string:
		raw, _ := json.Marshal(typed)
		return string(raw)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
}

func stripAntiPoisonOpenAIGuards(body map[string]any, routeKind string) map[string]any {
	stripped := deepCopyJSONMap(body)
	switch strings.TrimSpace(routeKind) {
	case "chat":
		stripAntiPoisonChatGuards(stripped)
	case "responses", "responses_compact":
		stripAntiPoisonResponsesGuards(stripped)
	}
	return stripped
}

func stripAntiPoisonChatGuards(body map[string]any) {
	choices, _ := body["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}
		if content := toStringValue(message["content"]); content != "" {
			message["content"] = extractAntiPoisonGuardsFromText(content, antiPoisonRequestContext{}).Text
		}
	}
}

func stripAntiPoisonResponsesGuards(body map[string]any) {
	for _, rawItem := range anySliceValue(body["output"]) {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) == "function_call" {
			if normalized, err := normalizeToolArgumentsJSON(item["arguments"]); err == nil {
				item["arguments"] = normalized
			}
			continue
		}
		if strings.TrimSpace(toStringValue(item["type"])) != "message" {
			continue
		}
		for _, rawContent := range anySliceValue(item["content"]) {
			contentMap, _ := rawContent.(map[string]any)
			if _, exists := contentMap["text"]; exists {
				contentMap["text"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["text"]), antiPoisonRequestContext{}).Text
			}
			if _, exists := contentMap["content"]; exists {
				contentMap["content"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["content"]), antiPoisonRequestContext{}).Text
			}
		}
	}
}

func stripAntiPoisonAnthropicGuards(body map[string]any) map[string]any {
	stripped := deepCopyJSONMap(body)
	for _, rawBlock := range anySliceValue(stripped["content"]) {
		block, _ := rawBlock.(map[string]any)
		if _, exists := block["text"]; exists {
			block["text"] = extractAntiPoisonGuardsFromText(toStringValue(block["text"]), antiPoisonRequestContext{}).Text
		}
		if _, exists := block["thinking"]; exists {
			block["thinking"] = extractAntiPoisonGuardsFromText(toStringValue(block["thinking"]), antiPoisonRequestContext{}).Text
		}
	}
	return stripped
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
