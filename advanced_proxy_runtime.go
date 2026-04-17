package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	circuitStateClosed   = "closed"
	circuitStateOpen     = "open"
	circuitStateHalfOpen = "half_open"
)

type proxyCircuitBreaker struct {
	mu                   sync.Mutex
	state                string
	consecutiveFailures  int
	consecutiveSuccesses int
	totalRequests        int
	failedRequests       int
	lastOpenedAt         time.Time
}

type advancedProxyRuntimeState struct {
	mu       sync.Mutex
	breakers map[string]*proxyCircuitBreaker
}

var advancedProxyRuntime = &advancedProxyRuntimeState{
	breakers: map[string]*proxyCircuitBreaker{},
}

func breakerKey(appType string, providerID string) string {
	appType = strings.TrimSpace(strings.ToLower(appType))
	if appType == "" {
		appType = "claude"
	}
	return appType + ":" + strings.TrimSpace(providerID)
}

func (r *advancedProxyRuntimeState) getBreaker(appType string, providerID string) *proxyCircuitBreaker {
	key := breakerKey(appType, providerID)
	r.mu.Lock()
	defer r.mu.Unlock()

	breaker, exists := r.breakers[key]
	if exists {
		return breaker
	}
	breaker = &proxyCircuitBreaker{state: circuitStateClosed}
	r.breakers[key] = breaker
	return breaker
}

func (r *advancedProxyRuntimeState) Allow(appType string, providerID string, config AppFailoverConfig) bool {
	return r.getBreaker(appType, providerID).allow(config)
}

func (r *advancedProxyRuntimeState) Record(appType string, providerID string, config AppFailoverConfig, success bool) {
	r.getBreaker(appType, providerID).record(config, success)
}

func (r *advancedProxyRuntimeState) GetStats(appType string, providerID string) CircuitBreakerStats {
	return r.getBreaker(appType, providerID).stats()
}

func (r *advancedProxyRuntimeState) Reset(appType string, providerID string) {
	r.getBreaker(appType, providerID).reset()
}

func (b *proxyCircuitBreaker) allow(config AppFailoverConfig) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == circuitStateOpen {
		timeout := time.Duration(clampInt(config.CircuitTimeoutSeconds, 5, 600)) * time.Second
		if timeout <= 0 {
			timeout = 45 * time.Second
		}
		if !b.lastOpenedAt.IsZero() && time.Since(b.lastOpenedAt) >= timeout {
			b.state = circuitStateHalfOpen
			b.consecutiveSuccesses = 0
			return true
		}
		return false
	}
	return true
}

func (b *proxyCircuitBreaker) record(config AppFailoverConfig, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalRequests++
	if success {
		switch b.state {
		case circuitStateHalfOpen:
			b.consecutiveSuccesses++
			b.consecutiveFailures = 0
			if b.consecutiveSuccesses >= clampInt(config.CircuitSuccessThreshold, 1, 20) {
				b.state = circuitStateClosed
				b.consecutiveSuccesses = 0
			}
		default:
			b.consecutiveFailures = 0
			b.consecutiveSuccesses = 0
			b.state = circuitStateClosed
		}
		return
	}

	b.failedRequests++
	b.consecutiveFailures++
	b.consecutiveSuccesses = 0
	shouldOpen := b.consecutiveFailures >= clampInt(config.CircuitFailureThreshold, 1, 20)
	minRequests := clampInt(config.CircuitMinRequests, 1, 100)
	if !shouldOpen && b.totalRequests >= minRequests {
		errorRate := float64(b.failedRequests) / math.Max(float64(b.totalRequests), 1)
		if errorRate >= config.CircuitErrorRateThreshold {
			shouldOpen = true
		}
	}
	if shouldOpen {
		b.state = circuitStateOpen
		b.lastOpenedAt = time.Now()
	}
}

func (b *proxyCircuitBreaker) stats() CircuitBreakerStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.state
	if state == "" {
		state = circuitStateClosed
	}
	return CircuitBreakerStats{
		State:                state,
		ConsecutiveFailures:  b.consecutiveFailures,
		ConsecutiveSuccesses: b.consecutiveSuccesses,
		TotalRequests:        b.totalRequests,
		FailedRequests:       b.failedRequests,
	}
}

func (b *proxyCircuitBreaker) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = circuitStateClosed
	b.consecutiveFailures = 0
	b.consecutiveSuccesses = 0
	b.totalRequests = 0
	b.failedRequests = 0
	b.lastOpenedAt = time.Time{}
}

func computeAdvancedProxyTimeoutSeconds(stream bool, failoverActive bool, config AppFailoverConfig) int {
	if !failoverActive {
		return 90
	}
	if stream {
		return clampInt(config.StreamingFirstByteTimeout+config.StreamingIdleTimeout, 10, 900)
	}
	return clampInt(config.NonStreamingTimeout, 5, 600)
}

func resolveAnthropicMessagesEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	lower := strings.ToLower(baseURL)
	switch {
	case strings.HasSuffix(lower, "/v1/messages"):
		return baseURL
	case strings.HasSuffix(lower, "/v1"):
		return baseURL + "/messages"
	case strings.HasSuffix(lower, "/messages"):
		return baseURL
	default:
		return baseURL + "/v1/messages"
	}
}

func buildResponsesEndpointCandidates(baseURL string) []string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(values *[]string, candidate string) {
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		*values = append(*values, candidate)
	}

	candidates := make([]string, 0, 2)
	lower := strings.ToLower(baseURL)
	switch {
	case strings.HasSuffix(lower, "/v1/responses"), strings.HasSuffix(lower, "/responses"):
		add(&candidates, baseURL)
	case strings.HasSuffix(lower, "/v1"):
		add(&candidates, baseURL+"/responses")
	default:
		add(&candidates, baseURL+"/v1/responses")
		add(&candidates, baseURL+"/responses")
	}
	return candidates
}

func buildResponsesCompactEndpointCandidates(baseURL string) []string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(values *[]string, candidate string) {
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		*values = append(*values, candidate)
	}

	candidates := make([]string, 0, 2)
	lower := strings.ToLower(baseURL)
	switch {
	case strings.HasSuffix(lower, "/v1/responses/compact"), strings.HasSuffix(lower, "/responses/compact"):
		add(&candidates, baseURL)
	case strings.HasSuffix(lower, "/v1"):
		add(&candidates, baseURL+"/responses/compact")
	default:
		add(&candidates, baseURL+"/v1/responses/compact")
		add(&candidates, baseURL+"/responses/compact")
	}
	return candidates
}

func normalizeAnthropicErrorMessage(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err == nil {
		message := firstNonEmpty(
			getNestedString(decoded, "error", "message"),
			strings.TrimSpace(toStringValue(decoded["message"])),
		)
		if message != "" {
			return message
		}
	}
	if title := extractHTMLTitle(text); title != "" {
		return title
	}
	return text
}

func shouldRectifyThinkingSignature(errorMessage string, config RectifierConfig) bool {
	if !config.Enabled || !config.RequestThinkingSignature {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	switch {
	case strings.Contains(lower, "invalid") && strings.Contains(lower, "signature") && strings.Contains(lower, "thinking") && strings.Contains(lower, "block"):
		return true
	case strings.Contains(lower, "must start with a thinking block"):
		return true
	case strings.Contains(lower, "expected") && (strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) && strings.Contains(lower, "tool_use"):
		return true
	case strings.Contains(lower, "signature") && strings.Contains(lower, "field required"):
		return true
	case strings.Contains(lower, "signature") && strings.Contains(lower, "extra inputs are not permitted"):
		return true
	case (strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) && strings.Contains(lower, "cannot be modified"):
		return true
	case strings.Contains(lower, "invalid request") || strings.Contains(lower, "illegal request"):
		return true
	default:
		return false
	}
}

func shouldRectifyThinkingBudget(errorMessage string, config RectifierConfig) bool {
	if !config.Enabled || !config.RequestThinkingBudget {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	hasBudget := strings.Contains(lower, "budget_tokens") || strings.Contains(lower, "budget tokens")
	hasThinking := strings.Contains(lower, "thinking")
	hasConstraint := strings.Contains(lower, "greater than or equal to 1024") || strings.Contains(lower, ">= 1024") || (strings.Contains(lower, "1024") && strings.Contains(lower, "input should be"))
	return hasBudget && hasThinking && hasConstraint
}

func rectifyThinkingSignature(body map[string]any) bool {
	messages, ok := body["messages"].([]any)
	if !ok {
		return false
	}

	applied := false
	for messageIndex, rawMessage := range messages {
		messageMap, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		contentList, ok := messageMap["content"].([]any)
		if !ok {
			continue
		}
		nextContent := make([]any, 0, len(contentList))
		for _, rawBlock := range contentList {
			blockMap, ok := rawBlock.(map[string]any)
			if !ok {
				nextContent = append(nextContent, rawBlock)
				continue
			}
			blockType := strings.TrimSpace(toStringValue(blockMap["type"]))
			if blockType == "thinking" || blockType == "redacted_thinking" {
				applied = true
				continue
			}
			if _, exists := blockMap["signature"]; exists {
				delete(blockMap, "signature")
				applied = true
			}
			nextContent = append(nextContent, blockMap)
		}
		messageMap["content"] = nextContent
		messages[messageIndex] = messageMap
	}
	body["messages"] = messages
	if shouldRemoveTopLevelThinking(body) {
		delete(body, "thinking")
		applied = true
	}
	return applied
}

func shouldRemoveTopLevelThinking(body map[string]any) bool {
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || strings.TrimSpace(toStringValue(thinking["type"])) != "enabled" {
		return false
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}
	for index := len(messages) - 1; index >= 0; index-- {
		messageMap, ok := messages[index].(map[string]any)
		if !ok || strings.TrimSpace(toStringValue(messageMap["role"])) != "assistant" {
			continue
		}
		contentList, ok := messageMap["content"].([]any)
		if !ok || len(contentList) == 0 {
			return false
		}
		firstBlock, _ := contentList[0].(map[string]any)
		firstType := strings.TrimSpace(toStringValue(firstBlock["type"]))
		if firstType == "thinking" || firstType == "redacted_thinking" {
			return false
		}
		for _, rawBlock := range contentList {
			blockMap, ok := rawBlock.(map[string]any)
			if ok && strings.TrimSpace(toStringValue(blockMap["type"])) == "tool_use" {
				return true
			}
		}
		return false
	}
	return false
}

func rectifyThinkingBudget(body map[string]any) bool {
	thinking, ok := body["thinking"].(map[string]any)
	if ok && strings.TrimSpace(toStringValue(thinking["type"])) == "adaptive" {
		return false
	}
	if !ok {
		thinking = map[string]any{}
		body["thinking"] = thinking
	}
	beforeType := strings.TrimSpace(toStringValue(thinking["type"]))
	beforeBudget := toIntValue(thinking["budget_tokens"])
	beforeMax := toIntValue(body["max_tokens"])
	thinking["type"] = "enabled"
	thinking["budget_tokens"] = 32000
	if beforeMax == 0 || beforeMax < 32001 {
		body["max_tokens"] = 64000
	}
	return beforeType != "enabled" || beforeBudget != 32000 || beforeMax == 0 || beforeMax < 32001
}

func deepCopyJSONMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	raw, _ := json.Marshal(input)
	decoded := map[string]any{}
	_ = json.Unmarshal(raw, &decoded)
	return decoded
}

func toIntValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func stringifyJSON(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(raw)
	}
}

func parseJSONStringMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		if strings.TrimSpace(typed) == "" {
			return map[string]any{}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded
		}
		return map[string]any{"raw": typed}
	default:
		return map[string]any{}
	}
}

func copyOptionalField(source map[string]any, target map[string]any, key string) {
	if value, exists := source[key]; exists && value != nil {
		target[key] = value
	}
}

func anthropicSystemText(system any) string {
	switch typed := system.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			blockMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(toStringValue(blockMap["type"])) == "text" {
				text := strings.TrimSpace(toStringValue(blockMap["text"]))
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func anthropicContentValueToText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			blockMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(toStringValue(blockMap["type"])) == "text" {
				text := strings.TrimSpace(toStringValue(blockMap["text"]))
				if text != "" {
					parts = append(parts, text)
				}
				continue
			}
			if serialized := stringifyJSON(blockMap); serialized != "" {
				parts = append(parts, serialized)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return stringifyJSON(value)
	}
}

func anthropicThinkingToReasoningEffort(raw any) string {
	thinking, ok := raw.(map[string]any)
	if !ok || strings.TrimSpace(toStringValue(thinking["type"])) == "" {
		return ""
	}
	budget := toIntValue(thinking["budget_tokens"])
	switch {
	case budget >= 32000:
		return "high"
	case budget >= 8000:
		return "medium"
	case budget > 0:
		return "low"
	default:
		return "medium"
	}
}

func anthropicToolsToOpenAI(raw any) []map[string]any {
	typed, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(typed))
	for _, item := range typed {
		toolMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        firstNonEmpty(strings.TrimSpace(toStringValue(toolMap["name"])), "tool"),
				"description": strings.TrimSpace(toStringValue(toolMap["description"])),
				"parameters":  toolMap["input_schema"],
			},
		})
	}
	return result
}

func anthropicToolsToResponses(raw any) []map[string]any {
	typed, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(typed))
	for _, item := range typed {
		toolMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"type":        "function",
			"name":        firstNonEmpty(strings.TrimSpace(toStringValue(toolMap["name"])), "tool"),
			"description": strings.TrimSpace(toStringValue(toolMap["description"])),
			"parameters":  toolMap["input_schema"],
		})
	}
	return result
}

func anthropicToolChoiceToOpenAI(raw any) any {
	choiceMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	switch strings.TrimSpace(toStringValue(choiceMap["type"])) {
	case "any":
		return "required"
	case "tool":
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": strings.TrimSpace(toStringValue(choiceMap["name"])),
			},
		}
	case "auto":
		return "auto"
	default:
		return nil
	}
}

func anthropicToolChoiceToResponses(raw any) any {
	choiceMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	switch strings.TrimSpace(toStringValue(choiceMap["type"])) {
	case "any":
		return "required"
	case "tool":
		return map[string]any{
			"type": "function",
			"name": strings.TrimSpace(toStringValue(choiceMap["name"])),
		}
	case "auto":
		return "auto"
	default:
		return nil
	}
}

func anthropicContentToChatPayloads(role string, content any) ([]string, []map[string]any, []map[string]any) {
	switch typed := content.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil, nil
		}
		return []string{text}, nil, nil
	case []any:
		textParts := make([]string, 0, len(typed))
		toolCalls := make([]map[string]any, 0)
		toolResults := make([]map[string]any, 0)
		for _, raw := range typed {
			blockMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch strings.TrimSpace(toStringValue(blockMap["type"])) {
			case "text":
				text := strings.TrimSpace(toStringValue(blockMap["text"]))
				if text != "" {
					textParts = append(textParts, text)
				}
			case "tool_use":
				toolCalls = append(toolCalls, map[string]any{
					"id":   firstNonEmpty(strings.TrimSpace(toStringValue(blockMap["id"])), fmt.Sprintf("tool_%d", len(toolCalls)+1)),
					"type": "function",
					"function": map[string]any{
						"name":      firstNonEmpty(strings.TrimSpace(toStringValue(blockMap["name"])), "tool"),
						"arguments": stringifyJSON(blockMap["input"]),
					},
				})
			case "tool_result":
				toolResults = append(toolResults, map[string]any{
					"role":         "tool",
					"tool_call_id": strings.TrimSpace(toStringValue(blockMap["tool_use_id"])),
					"content":      anthropicContentValueToText(blockMap["content"]),
				})
			}
		}
		return textParts, toolCalls, toolResults
	default:
		return nil, nil, nil
	}
}

func anthropicContentToResponsesPayloads(role string, content any) ([]string, []map[string]any, []map[string]any) {
	switch typed := content.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil, nil
		}
		return []string{text}, nil, nil
	case []any:
		textParts := make([]string, 0, len(typed))
		toolCalls := make([]map[string]any, 0)
		toolResults := make([]map[string]any, 0)
		for _, raw := range typed {
			blockMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch strings.TrimSpace(toStringValue(blockMap["type"])) {
			case "text":
				text := strings.TrimSpace(toStringValue(blockMap["text"]))
				if text != "" {
					textParts = append(textParts, text)
				}
			case "tool_use":
				toolCalls = append(toolCalls, map[string]any{
					"type":      "function_call",
					"call_id":   firstNonEmpty(strings.TrimSpace(toStringValue(blockMap["id"])), fmt.Sprintf("tool_%d", len(toolCalls)+1)),
					"name":      firstNonEmpty(strings.TrimSpace(toStringValue(blockMap["name"])), "tool"),
					"arguments": stringifyJSON(blockMap["input"]),
				})
			case "tool_result":
				toolResults = append(toolResults, map[string]any{
					"type":    "function_call_output",
					"call_id": strings.TrimSpace(toStringValue(blockMap["tool_use_id"])),
					"output":  anthropicContentValueToText(blockMap["content"]),
				})
			}
		}
		return textParts, toolCalls, toolResults
	default:
		return nil, nil, nil
	}
}

func anthropicRequestToOpenAIChat(body map[string]any, provider AdvancedProxyProvider) map[string]any {
	model := strings.TrimSpace(provider.Model)
	if model == "" {
		model = strings.TrimSpace(toStringValue(body["model"]))
	}
	messages := make([]map[string]any, 0, 8)
	systemText := anthropicSystemText(body["system"])
	if systemText != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": systemText,
		})
	}
	if rawMessages, ok := body["messages"].([]any); ok {
		for _, rawMessage := range rawMessages {
			messageMap, ok := rawMessage.(map[string]any)
			if !ok {
				continue
			}
			role := strings.TrimSpace(toStringValue(messageMap["role"]))
			textParts, toolCalls, toolResults := anthropicContentToChatPayloads(role, messageMap["content"])
			if len(textParts) > 0 || len(toolCalls) > 0 {
				payload := map[string]any{"role": role}
				if len(textParts) > 0 {
					payload["content"] = strings.Join(textParts, "\n")
				}
				if len(toolCalls) > 0 {
					payload["tool_calls"] = toolCalls
				}
				messages = append(messages, payload)
			}
			messages = append(messages, toolResults...)
		}
	}
	request := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	copyOptionalField(body, request, "temperature")
	copyOptionalField(body, request, "top_p")
	copyOptionalField(body, request, "max_tokens")
	if tools := anthropicToolsToOpenAI(body["tools"]); len(tools) > 0 {
		request["tools"] = tools
	}
	if toolChoice := anthropicToolChoiceToOpenAI(body["tool_choice"]); toolChoice != nil {
		request["tool_choice"] = toolChoice
	}
	if effort := anthropicThinkingToReasoningEffort(body["thinking"]); effort != "" {
		request["reasoning_effort"] = effort
	}
	return request
}

func anthropicRequestToOpenAIResponses(body map[string]any, provider AdvancedProxyProvider) map[string]any {
	model := strings.TrimSpace(provider.Model)
	if model == "" {
		model = strings.TrimSpace(toStringValue(body["model"]))
	}
	inputItems := make([]any, 0, 8)
	systemText := anthropicSystemText(body["system"])
	if systemText != "" {
		inputItems = append(inputItems, map[string]any{
			"role": "system",
			"content": []map[string]any{
				{"type": "input_text", "text": systemText},
			},
		})
	}
	if rawMessages, ok := body["messages"].([]any); ok {
		for _, rawMessage := range rawMessages {
			messageMap, ok := rawMessage.(map[string]any)
			if !ok {
				continue
			}
			role := strings.TrimSpace(toStringValue(messageMap["role"]))
			textParts, toolCalls, toolResults := anthropicContentToResponsesPayloads(role, messageMap["content"])
			if len(textParts) > 0 {
				contentItems := make([]map[string]any, 0, len(textParts))
				for _, text := range textParts {
					contentType := "input_text"
					if role == "assistant" {
						contentType = "output_text"
					}
					contentItems = append(contentItems, map[string]any{
						"type": contentType,
						"text": text,
					})
				}
				inputItems = append(inputItems, map[string]any{
					"role":    role,
					"content": contentItems,
				})
			}
			for _, item := range toolCalls {
				inputItems = append(inputItems, item)
			}
			for _, item := range toolResults {
				inputItems = append(inputItems, item)
			}
		}
	}
	request := map[string]any{
		"model":  model,
		"input":  inputItems,
		"stream": false,
	}
	if tools := anthropicToolsToResponses(body["tools"]); len(tools) > 0 {
		request["tools"] = tools
	}
	if toolChoice := anthropicToolChoiceToResponses(body["tool_choice"]); toolChoice != nil {
		request["tool_choice"] = toolChoice
	}
	if effort := anthropicThinkingToReasoningEffort(body["thinking"]); effort != "" {
		request["reasoning"] = map[string]any{"effort": effort}
	}
	if maxTokens := toIntValue(body["max_tokens"]); maxTokens > 0 {
		request["max_output_tokens"] = maxTokens
	}
	copyOptionalField(body, request, "temperature")
	copyOptionalField(body, request, "top_p")
	return request
}
