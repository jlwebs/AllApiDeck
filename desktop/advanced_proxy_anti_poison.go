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
	antiPoisonGuardToolSuffix      = "_trace"
	antiPoisonDigestLength         = 16
)

type antiPoisonRequestContext struct {
	Enabled        bool
	Config         AntiPoisonConfig
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

type antiPoisonValidationResult struct {
	Applied       bool
	Valid         bool
	Blocked       bool
	Reason        string
	RealCount     int
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

type antiPoisonStringProtectionRule struct {
	Description string
	Scope       string
	Pattern     string
	Regexp      *regexp.Regexp
}

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
	next := protectAntiPoisonStringValue(body, "$", rules, &ctx, route, provider, channel)
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
		restored, count := restoreAntiPoisonStringValue(body, ctx.mapping)
		if count > 0 {
			nextRaw, marshalErr := json.Marshal(restored)
			if marshalErr == nil {
				ctx.addRecord(antiPoisonOperationRecord{
					Stage:    "respond in",
					Channel:  channel,
					Route:    route,
					Provider: provider,
					Rule:     "字符串保护还原",
					Before:   fmt.Sprintf("%d placeholder(s)", count),
					After:    "restored for client",
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
	for placeholder, original := range ctx.mapping {
		hits := strings.Count(restored, placeholder)
		if hits <= 0 {
			continue
		}
		restored = strings.ReplaceAll(restored, placeholder, original)
		count += hits
	}
	if count > 0 {
		ctx.addRecord(antiPoisonOperationRecord{
			Stage:    "respond in",
			Channel:  channel,
			Route:    route,
			Provider: provider,
			Rule:     "字符串保护还原",
			Before:   fmt.Sprintf("%d placeholder(s)", count),
			After:    "restored for client (raw fallback)",
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
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		count := 0
		for key, child := range typed {
			restored, hits := restoreAntiPoisonStringValue(child, mapping)
			next[key] = restored
			count += hits
		}
		return next, count
	case []any:
		next := make([]any, 0, len(typed))
		count := 0
		for _, child := range typed {
			restored, hits := restoreAntiPoisonStringValue(child, mapping)
			next = append(next, restored)
			count += hits
		}
		return next, count
	case string:
		result := typed
		count := 0
		for placeholder, original := range mapping {
			hits := strings.Count(result, placeholder)
			if hits <= 0 {
				continue
			}
			result = strings.ReplaceAll(result, placeholder, original)
			count += hits
		}
		return result, count
	default:
		return value, 0
	}
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
		Rule:     "防投毒校验失败",
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
	for _, separator := range []string{": ", "："} {
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
	default:
		return "text", pattern
	}
}

func protectAntiPoisonStringValue(value any, path string, rules []antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, child := range typed {
			childPath := path + "." + key
			if matchedRule := matchAntiPoisonStringProtectionKeyRule(key, childPath, rules); matchedRule != nil {
				next[key] = protectAntiPoisonStringValueByRule(child, childPath, *matchedRule, ctx, route, provider, channel)
				continue
			}
			next[key] = protectAntiPoisonStringValue(child, childPath, rules, ctx, route, provider, channel)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for index, child := range typed {
			next = append(next, protectAntiPoisonStringValue(child, fmt.Sprintf("%s[%d]", path, index), rules, ctx, route, provider, channel))
		}
		return next
	case string:
		return protectAntiPoisonStringText(typed, path, rules, ctx, route, provider, channel)
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
			next[key] = protectAntiPoisonStringValue(child, path+"."+key, []antiPoisonStringProtectionRule{rule}, ctx, route, provider, channel)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for index, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			if _, ok := child.(string); ok {
				next = append(next, protectAntiPoisonStringValueByRule(child, childPath, rule, ctx, route, provider, channel))
			} else {
				next = append(next, protectAntiPoisonStringValue(child, childPath, []antiPoisonStringProtectionRule{rule}, ctx, route, provider, channel))
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
	return storeAntiPoisonProtectedString(text, path, rule.Description, ctx, route, provider, channel)
}

func protectAntiPoisonStringText(text string, path string, rules []antiPoisonStringProtectionRule, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) string {
	if text == "" || ctx == nil {
		return text
	}
	result := text
	for _, rule := range rules {
		if rule.Regexp == nil || strings.TrimSpace(rule.Scope) != "text" {
			continue
		}
		result = rule.Regexp.ReplaceAllStringFunc(result, func(match string) string {
			if match == "" || strings.Contains(match, "__AAD_STR_") {
				return match
			}
			return storeAntiPoisonProtectedString(match, path, rule.Description, ctx, route, provider, channel)
		})
	}
	return result
}

func storeAntiPoisonProtectedString(original string, path string, ruleDescription string, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) string {
	placeholder := fmt.Sprintf("__AAD_STR_%s_%03d__", randomAntiPoisonHex(4), len(ctx.mapping)+1)
	ctx.mapping[placeholder] = original
	ctx.addRecord(antiPoisonOperationRecord{
		Stage:    "request out",
		Channel:  channel,
		Rule:     ruleDescription,
		Path:     path,
		Before:   summarizeAntiPoisonProtectedText(original),
		After:    placeholder,
		Count:    1,
		Route:    route,
		Provider: provider,
	})
	return placeholder
}

func summarizeAntiPoisonProtectedText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty"
	}
	return fmt.Sprintf("len=%d sha256=%s", len([]rune(text)), sha256Hex(text)[:12])
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
	insertionPoints := []string{"system_prepend", "system_append", "tool_schema_tail", "instruction_middle", "guard_contract_tail"}
	return antiPoisonRequestContext{
		Enabled:        config.Enabled,
		Config:         config,
		RouteKind:      strings.TrimSpace(routeKind),
		Alias:          alias,
		Prefix:         prefix,
		GuardToolName:  prefix + antiPoisonGuardToolSuffix,
		Seed:           seed,
		StrategySlot:   1 + antiPoisonDerivedIndex(seed, "strategy", strategyPoolSize),
		PhraseVariant:  1 + antiPoisonDerivedIndex(seed, "phrase", phraseVariants),
		InsertionPoint: insertionPoints[antiPoisonDerivedIndex(seed, "insertion", len(insertionPoints))],
	}
}

func normalizeAntiPoisonRequestContext(ctx antiPoisonRequestContext) antiPoisonRequestContext {
	ctx.Config = sanitizeAntiPoisonConfig(ctx.Config)
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
		ctx.GuardToolName = ctx.Prefix + antiPoisonGuardToolSuffix
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
		ctx.InsertionPoint = "system_prepend"
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
		GuardToolName:  prefix + antiPoisonGuardToolSuffix,
		Seed:           "preview",
		StrategySlot:   1,
		PhraseVariant:  1,
		InsertionPoint: "system_prepend",
	}))
}

func buildAntiPoisonPrompt(ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	strategy := strings.ReplaceAll(ctx.Config.StrategyPrompt, antiPoisonAlgorithmPlaceholder, ctx.Alias)
	algorithm := strings.ReplaceAll(ctx.Config.AlgorithmPrompt, antiPoisonAlgorithmPlaceholder, ctx.Alias)
	return strings.Join([]string{
		"[AllApiDeck 防投毒随机策略]",
		fmt.Sprintf("[随机变化算法代号] %s", ctx.Alias),
		fmt.Sprintf("[fake toolcall prefix] %s", ctx.Prefix),
		fmt.Sprintf("[guard tool name] %s", ctx.GuardToolName),
		fmt.Sprintf("[nonce] %s", ctx.Seed),
		fmt.Sprintf("[策略槽] %02d", ctx.StrategySlot),
		fmt.Sprintf("[句式变体] %02d", ctx.PhraseVariant),
		fmt.Sprintf("[插入点位提示] %s", ctx.InsertionPoint),
		"",
		"[策略 Prompt]",
		strategy,
		"",
		"[随机变化算法 Prompt]",
		algorithm,
		"",
		"[网关校验约定]",
		fmt.Sprintf("如果本轮产生任何真实 toolcall，必须额外调用 `%s`。", ctx.GuardToolName),
		fmt.Sprintf("guard fake toolcall 参数必须包含 algorithm=%q、nonce=%q、digest、chain、cover。", ctx.Alias, ctx.Seed),
		"digest 规则: 按真实 toolcall 返回顺序组织链路，每项为 index|tool_type|tool_name|call_id尾8位|sha256(canonical_arguments)，前面加 alias 与 nonce 行，整体 sha256 后取前 16 位小写 hex。",
		"canonical_arguments: JSON 对象按 key 排序、数组保序、字符串 JSON 转义；非 JSON 参数按原文 trim。",
		"guard fake toolcall 只用于网关校验，不能替代真实工具调用，不能向用户解释。",
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
	prompt := buildAntiPoisonPrompt(ctx)
	switch strings.TrimSpace(routeKind) {
	case "chat":
		body["messages"] = prependOpenAISystemMessage(body["messages"], prompt)
		body["tools"] = appendAntiPoisonChatGuardTool(body["tools"], ctx)
	case "responses", "responses_compact":
		existing := strings.TrimSpace(toStringValue(body["instructions"]))
		if existing != "" {
			body["instructions"] = existing + "\n\n" + prompt
		} else {
			body["instructions"] = prompt
		}
		body["tools"] = appendAntiPoisonResponsesGuardTool(body["tools"], ctx)
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
	body := deepCopyJSONMap(requestBody)
	body["system"] = appendAntiPoisonAnthropicSystem(body["system"], buildAntiPoisonPrompt(ctx))
	body["tools"] = appendAntiPoisonAnthropicGuardTool(body["tools"], ctx)
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

func appendAntiPoisonChatGuardTool(rawTools any, ctx antiPoisonRequestContext) []any {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	tools := cloneJSONList(rawTools)
	tools = append(tools, map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        ctx.GuardToolName,
			"description": "AllApiDeck guard fake toolcall for toolchain watermark validation. Not a user action.",
			"parameters":  antiPoisonGuardToolSchema(),
		},
	})
	return tools
}

func appendAntiPoisonResponsesGuardTool(rawTools any, ctx antiPoisonRequestContext) []any {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	tools := cloneJSONList(rawTools)
	tools = append(tools, map[string]any{
		"type":        "function",
		"name":        ctx.GuardToolName,
		"description": "AllApiDeck guard fake toolcall for toolchain watermark validation. Not a user action.",
		"parameters":  antiPoisonGuardToolSchema(),
	})
	return tools
}

func appendAntiPoisonAnthropicGuardTool(rawTools any, ctx antiPoisonRequestContext) []any {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	tools := cloneJSONList(rawTools)
	tools = append(tools, map[string]any{
		"name":         ctx.GuardToolName,
		"description":  "AllApiDeck guard fake toolcall for toolchain watermark validation. Not a user action.",
		"input_schema": antiPoisonGuardToolSchema(),
	})
	return tools
}

func appendAntiPoisonAnthropicSystem(rawSystem any, prompt string) any {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return rawSystem
	}
	switch typed := rawSystem.(type) {
	case string:
		existing := strings.TrimSpace(typed)
		if existing == "" {
			return prompt
		}
		return existing + "\n\n" + prompt
	case []any:
		next := append([]any{}, typed...)
		next = append(next, map[string]any{"type": "text", "text": prompt})
		return next
	case []map[string]any:
		next := make([]any, 0, len(typed)+1)
		for _, item := range typed {
			next = append(next, item)
		}
		next = append(next, map[string]any{"type": "text", "text": prompt})
		return next
	default:
		return prompt
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

func antiPoisonGuardToolSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"required":             []any{"algorithm", "nonce", "digest"},
		"properties": map[string]any{
			"algorithm": map[string]any{
				"type":        "string",
				"description": "Current random variation algorithm alias.",
			},
			"nonce": map[string]any{
				"type":        "string",
				"description": "Current request nonce/seed.",
			},
			"digest": map[string]any{
				"type":        "string",
				"description": "Toolchain digest generated from the current AllApiDeck guard rules.",
			},
			"chain": map[string]any{
				"type":        "string",
				"description": "Compact real toolcall chain summary.",
			},
			"cover": map[string]any{
				"type":        "string",
				"description": "Tool category coverage summary.",
			},
		},
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

	return validateAndStripAntiPoisonToolCalls(
		rawBody,
		extractAntiPoisonOpenAIToolCalls(body, routeKind, ctx),
		ctx,
		func() []byte {
			return mustMarshalAntiPoisonBody(stripAntiPoisonOpenAIGuards(body, routeKind, ctx), rawBody)
		},
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

	return validateAndStripAntiPoisonToolCalls(
		rawBody,
		extractAntiPoisonAnthropicToolCalls(body, ctx),
		ctx,
		func() []byte {
			return mustMarshalAntiPoisonBody(stripAntiPoisonAnthropicGuards(body, ctx), rawBody)
		},
	)
}

func validateAndStripAntiPoisonToolCalls(rawBody []byte, calls []antiPoisonToolCall, ctx antiPoisonRequestContext, stripGuards func() []byte) antiPoisonValidationResult {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	result := antiPoisonValidationResult{Applied: true, Body: rawBody}
	realCalls := make([]antiPoisonToolCall, 0, len(calls))
	guardCalls := make([]antiPoisonToolCall, 0, len(calls))
	for _, call := range calls {
		if call.IsGuard {
			guardCalls = append(guardCalls, call)
		} else {
			realCalls = append(realCalls, call)
		}
	}
	result.RealCount = len(realCalls)
	result.GuardCount = len(guardCalls)
	if len(realCalls) == 0 {
		result.Valid = true
		if len(guardCalls) > 0 && stripGuards != nil {
			result.RemovedGuards = len(guardCalls)
			result.Body = stripGuards()
		}
		return result
	}

	minGuardCount := clampInt(ctx.Config.Randomization.MinFakeToolcalls, 1, 20)
	if len(guardCalls) < minGuardCount {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = "missing_guard_toolcall"
		if !result.Blocked && len(guardCalls) > 0 && stripGuards != nil {
			result.RemovedGuards = len(guardCalls)
			result.Body = stripGuards()
		}
		return result
	}

	expectedDigest := computeAntiPoisonToolChainDigest(realCalls, ctx)
	if !antiPoisonGuardDigestMatches(guardCalls, expectedDigest, ctx) {
		result.Valid = false
		result.Blocked = antiPoisonShouldBlock(ctx.Config)
		result.Reason = "guard_digest_mismatch"
		if !result.Blocked && stripGuards != nil {
			result.RemovedGuards = len(guardCalls)
			result.Body = stripGuards()
		}
		return result
	}

	result.Valid = true
	if len(guardCalls) > 0 && stripGuards != nil {
		result.RemovedGuards = len(guardCalls)
		result.Body = stripGuards()
	}
	return result
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

func extractAntiPoisonOpenAIToolCalls(body map[string]any, routeKind string, ctx antiPoisonRequestContext) []antiPoisonToolCall {
	switch strings.TrimSpace(routeKind) {
	case "chat":
		return extractAntiPoisonChatToolCalls(body, ctx)
	case "responses", "responses_compact":
		return extractAntiPoisonResponsesToolCalls(body, ctx)
	default:
		return nil
	}
}

func extractAntiPoisonChatToolCalls(body map[string]any, ctx antiPoisonRequestContext) []antiPoisonToolCall {
	choices, _ := body["choices"].([]any)
	calls := []antiPoisonToolCall{}
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}
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
					IsGuard:       isAntiPoisonGuardToolName(name, ctx),
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
				IsGuard:       isAntiPoisonGuardToolName(name, ctx),
			})
		}
	}
	return calls
}

func extractAntiPoisonResponsesToolCalls(body map[string]any, ctx antiPoisonRequestContext) []antiPoisonToolCall {
	output, _ := body["output"].([]any)
	calls := []antiPoisonToolCall{}
	for _, rawItem := range output {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) != "function_call" {
			continue
		}
		name := strings.TrimSpace(toStringValue(item["name"]))
		calls = append(calls, antiPoisonToolCall{
			Container:     item,
			Kind:          "responses.function_call",
			Name:          name,
			CallID:        strings.TrimSpace(toStringValue(item["call_id"])),
			ArgumentsText: toStringValue(item["arguments"]),
			ToolType:      classifyAntiPoisonToolName(name),
			IsGuard:       isAntiPoisonGuardToolName(name, ctx),
		})
	}
	return calls
}

func extractAntiPoisonAnthropicToolCalls(body map[string]any, ctx antiPoisonRequestContext) []antiPoisonToolCall {
	content, _ := body["content"].([]any)
	calls := []antiPoisonToolCall{}
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		if strings.TrimSpace(toStringValue(block["type"])) != "tool_use" {
			continue
		}
		name := strings.TrimSpace(toStringValue(block["name"]))
		calls = append(calls, antiPoisonToolCall{
			Container:     block,
			Kind:          "anthropic.tool_use",
			Name:          name,
			CallID:        strings.TrimSpace(toStringValue(block["id"])),
			ArgumentsText: stringifyJSON(block["input"]),
			ToolType:      classifyAntiPoisonToolName(name),
			IsGuard:       isAntiPoisonGuardToolName(name, ctx),
		})
	}
	return calls
}

func isAntiPoisonGuardToolName(name string, ctx antiPoisonRequestContext) bool {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	guardName := strings.ToLower(strings.TrimSpace(ctx.GuardToolName))
	prefix := strings.ToLower(strings.TrimSpace(ctx.Prefix))
	return name == guardName || (prefix != "" && strings.HasPrefix(name, prefix+"_"))
}

func classifyAntiPoisonToolName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "shell"), strings.Contains(lower, "command"), strings.Contains(lower, "exec"):
		return "command"
	case strings.Contains(lower, "read"), strings.Contains(lower, "file"), strings.Contains(lower, "grep"), strings.Contains(lower, "search"), strings.Contains(lower, "rg"):
		return "read"
	case strings.Contains(lower, "web"), strings.Contains(lower, "http"), strings.Contains(lower, "fetch"):
		return "network"
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
			"%d|%s|%s|%s|%s",
			index,
			toolType,
			call.Name,
			tailString(call.CallID, 8),
			sha256Hex(canonicalAntiPoisonArgumentText(call.ArgumentsText)),
		))
	}
	return sha256Hex(strings.Join(parts, "\n"))[:antiPoisonDigestLength]
}

func antiPoisonGuardDigestMatches(guards []antiPoisonToolCall, expectedDigest string, ctx antiPoisonRequestContext) bool {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	for _, guard := range guards {
		var payload map[string]any
		if err := json.Unmarshal([]byte(guard.ArgumentsText), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(toStringValue(payload["algorithm"])) != ctx.Alias {
			continue
		}
		if strings.TrimSpace(toStringValue(payload["nonce"])) != ctx.Seed {
			continue
		}
		if strings.TrimSpace(toStringValue(payload["digest"])) == expectedDigest {
			return true
		}
	}
	return false
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

func stripAntiPoisonOpenAIGuards(body map[string]any, routeKind string, ctx antiPoisonRequestContext) map[string]any {
	stripped := deepCopyJSONMap(body)
	switch strings.TrimSpace(routeKind) {
	case "chat":
		stripAntiPoisonChatGuards(stripped, ctx)
	case "responses", "responses_compact":
		stripAntiPoisonResponsesGuards(stripped, ctx)
	}
	return stripped
}

func stripAntiPoisonChatGuards(body map[string]any, ctx antiPoisonRequestContext) {
	choices, _ := body["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}
		if toolCalls, ok := message["tool_calls"].([]any); ok {
			next := make([]any, 0, len(toolCalls))
			for _, rawCall := range toolCalls {
				callMap, _ := rawCall.(map[string]any)
				functionMap, _ := callMap["function"].(map[string]any)
				if isAntiPoisonGuardToolName(toStringValue(functionMap["name"]), ctx) {
					continue
				}
				next = append(next, rawCall)
			}
			if len(next) > 0 {
				message["tool_calls"] = next
			} else {
				delete(message, "tool_calls")
			}
		}
		if functionMap, ok := message["function_call"].(map[string]any); ok && isAntiPoisonGuardToolName(toStringValue(functionMap["name"]), ctx) {
			delete(message, "function_call")
		}
	}
}

func stripAntiPoisonResponsesGuards(body map[string]any, ctx antiPoisonRequestContext) {
	output, _ := body["output"].([]any)
	next := make([]any, 0, len(output))
	for _, rawItem := range output {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) == "function_call" && isAntiPoisonGuardToolName(toStringValue(item["name"]), ctx) {
			continue
		}
		next = append(next, rawItem)
	}
	body["output"] = next
}

func stripAntiPoisonAnthropicGuards(body map[string]any, ctx antiPoisonRequestContext) map[string]any {
	stripped := deepCopyJSONMap(body)
	content, _ := stripped["content"].([]any)
	next := make([]any, 0, len(content))
	hasToolUse := false
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		if strings.TrimSpace(toStringValue(block["type"])) == "tool_use" {
			if isAntiPoisonGuardToolName(toStringValue(block["name"]), ctx) {
				continue
			}
			hasToolUse = true
		}
		next = append(next, rawBlock)
	}
	stripped["content"] = next
	if strings.TrimSpace(toStringValue(stripped["stop_reason"])) == "tool_use" && !hasToolUse {
		stripped["stop_reason"] = "end_turn"
	}
	return stripped
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func tailString(value string, length int) string {
	value = strings.TrimSpace(value)
	if length <= 0 || len(value) <= length {
		return value
	}
	return value[len(value)-length:]
}
