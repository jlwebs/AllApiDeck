package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	advancedProxyOpenAIProtocolPreferChat      = 0
	advancedProxyOpenAIProtocolPreferResponses = 1
)

type advancedProxyOpenAIProtocolPreferenceStore struct {
	mu     sync.Mutex
	loaded bool
	values map[string]int
}

type advancedProxyResponsesChatFallbackPlan struct {
	ChatBody        []byte
	ScopeKey        string
	Model           string
	Blockers        []string
	SupportsChat    bool
	BlockedReason   string
	HostedWebSearch bool
}

type advancedProxyChatToResponsesToolState struct {
	ItemID      string
	CallID      string
	Name        string
	Arguments   strings.Builder
	OutputIndex int
	Added       bool
	Closed      bool
}

var advancedProxyOpenAIProtocolPreferences = advancedProxyOpenAIProtocolPreferenceStore{}

func streamAdvancedProxySSEDataPayloads(reader io.Reader, onPayload func(payload string) (bool, error)) error {
	buffer := make([]byte, 0, 64*1024)
	chunk := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			for {
				payload, consumed, ok := nextAdvancedProxySSEDataPayload(buffer)
				if consumed > 0 {
					buffer = buffer[consumed:]
				}
				if !ok {
					break
				}
				stop, err := onPayload(payload)
				if err != nil {
					return err
				}
				if stop {
					return nil
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				for {
					payload, consumed, ok := nextAdvancedProxySSEDataPayload(buffer)
					if consumed > 0 {
						buffer = buffer[consumed:]
					}
					if !ok {
						break
					}
					stop, err := onPayload(payload)
					if err != nil {
						return err
					}
					if stop {
						return nil
					}
				}
				return nil
			}
			return readErr
		}
	}
}

func nextAdvancedProxySSEDataPayload(buffer []byte) (string, int, bool) {
	dataIndex := bytes.Index(buffer, []byte("data:"))
	if dataIndex < 0 {
		if len(buffer) > len("data:") {
			return "", len(buffer) - len("data:"), false
		}
		return "", 0, false
	}
	cursor := dataIndex + len("data:")
	for cursor < len(buffer) && (buffer[cursor] == ' ' || buffer[cursor] == '\t') {
		cursor++
	}
	if cursor >= len(buffer) {
		return "", dataIndex, false
	}
	if bytes.HasPrefix(buffer[cursor:], []byte("[DONE]")) {
		return "[DONE]", cursor + len("[DONE]"), true
	}
	if buffer[cursor] == '{' || buffer[cursor] == '[' {
		end, ok := findAdvancedProxyJSONPayloadEnd(buffer, cursor)
		if !ok {
			return "", dataIndex, false
		}
		return strings.TrimSpace(string(buffer[cursor:end])), end, true
	}
	if lineEnd := bytes.IndexByte(buffer[cursor:], '\n'); lineEnd >= 0 {
		end := cursor + lineEnd
		payload := strings.TrimSpace(strings.TrimRight(string(buffer[cursor:end]), "\r"))
		return payload, end + 1, true
	}
	return "", dataIndex, false
}

func findAdvancedProxyJSONPayloadEnd(buffer []byte, start int) (int, bool) {
	if start < 0 || start >= len(buffer) {
		return 0, false
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(buffer); index++ {
		char := buffer[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return index + 1, true
			}
			if depth < 0 {
				return 0, false
			}
		}
	}
	return 0, false
}

func resolveAdvancedProxyOpenAIProtocolPreferencePath() string {
	return filepath.Join(resolveRuntimeRootDir(), "advanced-proxy", "openai-protocol-preferences.json")
}

func buildAdvancedProxyOpenAIProtocolPreferenceKeyFingerprint(rawKey string) string {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func resolveAdvancedProxyOpenAIProtocolPreferenceScopeKey(provider AdvancedProxyProvider, model string) string {
	hostKey := resolveCheckProtocolPreferenceHostKey(provider.BaseURL)
	if hostKey == "" {
		return ""
	}
	keyFingerprint := buildAdvancedProxyOpenAIProtocolPreferenceKeyFingerprint(provider.APIKey)
	modelKey := strings.ToLower(strings.TrimSpace(model))
	if keyFingerprint == "" || modelKey == "" {
		return ""
	}
	return fmt.Sprintf(
		"host=%s&key=%s&model=%s",
		url.QueryEscape(hostKey),
		url.QueryEscape(keyFingerprint),
		url.QueryEscape(modelKey),
	)
}

func loadAdvancedProxyOpenAIProtocolPreferencesLocked() {
	if advancedProxyOpenAIProtocolPreferences.loaded {
		return
	}
	advancedProxyOpenAIProtocolPreferences.loaded = true
	advancedProxyOpenAIProtocolPreferences.values = map[string]int{}

	raw, err := os.ReadFile(resolveAdvancedProxyOpenAIProtocolPreferencePath())
	if err != nil {
		return
	}

	var decoded map[string]int
	if err := json.Unmarshal(raw, &decoded); err != nil {
		appendAdvancedProxyLogf("[OPENAI_PROXY_PREFERENCE_DECODE_FAIL] detail=%s", previewAdvancedProxyText(err.Error(), 220))
		return
	}

	for scopeKey, value := range decoded {
		if strings.TrimSpace(scopeKey) == "" {
			continue
		}
		switch value {
		case advancedProxyOpenAIProtocolPreferChat, advancedProxyOpenAIProtocolPreferResponses:
			advancedProxyOpenAIProtocolPreferences.values[strings.TrimSpace(scopeKey)] = value
		}
	}
}

func getAdvancedProxyOpenAIProtocolPreference(scopeKey string) (int, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return 0, false
	}

	advancedProxyOpenAIProtocolPreferences.mu.Lock()
	defer advancedProxyOpenAIProtocolPreferences.mu.Unlock()
	loadAdvancedProxyOpenAIProtocolPreferencesLocked()

	value, ok := advancedProxyOpenAIProtocolPreferences.values[scopeKey]
	return value, ok
}

func setAdvancedProxyOpenAIProtocolPreference(scopeKey string, value int) {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return
	}
	if value != advancedProxyOpenAIProtocolPreferResponses {
		value = advancedProxyOpenAIProtocolPreferChat
	}

	advancedProxyOpenAIProtocolPreferences.mu.Lock()
	loadAdvancedProxyOpenAIProtocolPreferencesLocked()
	current, exists := advancedProxyOpenAIProtocolPreferences.values[scopeKey]
	if exists && current == value {
		advancedProxyOpenAIProtocolPreferences.mu.Unlock()
		return
	}
	advancedProxyOpenAIProtocolPreferences.values[scopeKey] = value
	snapshot := make(map[string]int, len(advancedProxyOpenAIProtocolPreferences.values))
	for key, item := range advancedProxyOpenAIProtocolPreferences.values {
		snapshot[key] = item
	}
	advancedProxyOpenAIProtocolPreferences.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		appendAdvancedProxyLogf("[OPENAI_PROXY_PREFERENCE_ENCODE_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
	if err := os.MkdirAll(filepath.Dir(resolveAdvancedProxyOpenAIProtocolPreferencePath()), 0o755); err != nil {
		appendAdvancedProxyLogf("[OPENAI_PROXY_PREFERENCE_MKDIR_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
	if err := os.WriteFile(resolveAdvancedProxyOpenAIProtocolPreferencePath(), data, 0o644); err != nil {
		appendAdvancedProxyLogf("[OPENAI_PROXY_PREFERENCE_WRITE_FAIL] scope=%s detail=%s", previewAdvancedProxyText(scopeKey, 120), previewAdvancedProxyText(err.Error(), 220))
		return
	}
}

func resetAdvancedProxyOpenAIProtocolPreferencesForTests() {
	advancedProxyOpenAIProtocolPreferences.mu.Lock()
	defer advancedProxyOpenAIProtocolPreferences.mu.Unlock()
	advancedProxyOpenAIProtocolPreferences.loaded = false
	advancedProxyOpenAIProtocolPreferences.values = nil
}

func buildOpenAIChatFallbackPlanFromResponses(rawBody []byte, provider AdvancedProxyProvider) (advancedProxyResponsesChatFallbackPlan, error) {
	plan := advancedProxyResponsesChatFallbackPlan{}

	requestBody := map[string]any{}
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		return plan, err
	}

	model := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(requestBody["model"])))
	plan.Model = model
	plan.ScopeKey = resolveAdvancedProxyOpenAIProtocolPreferenceScopeKey(provider, model)
	backfillToolReasoning := shouldBackfillOpenAIChatToolReasoningForProvider(provider)

	blockers := make([]string, 0, 4)
	if previousResponseID := strings.TrimSpace(toStringValue(requestBody["previous_response_id"])); previousResponseID != "" {
		blockers = append(blockers, "previous_response_id")
	}
	if conversationID := strings.TrimSpace(toStringValue(requestBody["conversation"])); conversationID != "" {
		blockers = append(blockers, "conversation")
	}

	systemParts := make([]string, 0, 2)
	messages := make([]map[string]any, 0, 8)
	appendMessage := func(role string, content any, toolCalls []map[string]any, extra map[string]any) {
		role = strings.TrimSpace(role)
		if role == "" {
			return
		}
		if content == nil && len(toolCalls) == 0 {
			if role == "assistant" && len(extra) > 0 {
				reasoningText := strings.TrimSpace(toStringValue(extra["reasoning_content"]))
				if reasoningText != "" {
					content = "Reasoning recorded for context.\n" + reasoningText
				}
			}
			if content == nil {
				return
			}
		}
		payload := map[string]any{
			"role": role,
		}
		if content != nil {
			payload["content"] = content
		}
		if len(toolCalls) > 0 {
			payload["tool_calls"] = toolCalls
		}
		for key, value := range extra {
			if strings.TrimSpace(key) != "" && value != nil {
				payload[key] = value
			}
		}
		if content == nil && len(toolCalls) == 0 && len(extra) == 0 {
			return
		}
		messages = append(messages, payload)
	}

	switch typed := requestBody["input"].(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text != "" {
			appendMessage("user", text, nil, nil)
		}
	case []any:
		pendingReasoning := ""
		pendingToolCalls := make([]map[string]any, 0, 2)
		pendingToolOutputs := make([]map[string]any, 0, 2)
		var pendingToolExtra map[string]any
		attachPendingReasoningToToolCall := func() {
			if pendingReasoning == "" {
				return
			}
			if pendingToolExtra == nil {
				pendingToolExtra = map[string]any{}
			}
			pendingToolExtra["reasoning_content"] = joinNonEmptyLines(toStringValue(pendingToolExtra["reasoning_content"]), pendingReasoning)
			pendingReasoning = ""
		}
		flushPendingToolCalls := func() {
			if len(pendingToolCalls) == 0 {
				return
			}
			attachPendingReasoningToToolCall()
			outputIDs := make(map[string]struct{}, len(pendingToolOutputs))
			for _, output := range pendingToolOutputs {
				if toolCallID := strings.TrimSpace(toStringValue(output["tool_call_id"])); toolCallID != "" {
					outputIDs[toolCallID] = struct{}{}
				}
			}
			validCalls := make([]map[string]any, 0, len(pendingToolCalls))
			validCallIDs := make(map[string]struct{}, len(pendingToolCalls))
			for _, toolCall := range pendingToolCalls {
				toolCallID := strings.TrimSpace(toStringValue(toolCall["id"]))
				if toolCallID == "" {
					continue
				}
				if _, exists := outputIDs[toolCallID]; !exists {
					blockers = append(blockers, "tool_call_missing_output")
					continue
				}
				validCalls = append(validCalls, toolCall)
				validCallIDs[toolCallID] = struct{}{}
			}
			if len(validCalls) > 0 {
				appendMessage("assistant", nil, validCalls, pendingToolExtra)
				for _, output := range pendingToolOutputs {
					toolCallID := strings.TrimSpace(toStringValue(output["tool_call_id"]))
					if _, exists := validCallIDs[toolCallID]; !exists {
						blockers = append(blockers, "tool_output_without_call")
						continue
					}
					appendMessage("tool", output["content"], nil, nil)
					if len(messages) > 0 {
						messages[len(messages)-1]["tool_call_id"] = toolCallID
					}
				}
			}
			pendingToolCalls = nil
			pendingToolOutputs = nil
			pendingToolExtra = nil
		}
		queueToolCall := func(toolCall map[string]any) {
			attachPendingReasoningToToolCall()
			pendingToolCalls = append(pendingToolCalls, toolCall)
		}
		appendToolOutput := func(toolCallID string, outputText string) {
			pendingToolOutputs = append(pendingToolOutputs, map[string]any{
				"tool_call_id": toolCallID,
				"content":      outputText,
			})
		}
		for _, rawItem := range typed {
			itemMap, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			itemType := strings.ToLower(strings.TrimSpace(toStringValue(itemMap["type"])))
			role := strings.TrimSpace(toStringValue(itemMap["role"]))
			if role != "" || itemType == "message" || itemType == "input_text" || itemType == "input_image" || itemType == "text" || itemType == "output_text" || itemType == "" {
				flushPendingToolCalls()
				if role == "system" || role == "developer" {
					text := openAIMessageContentToText(itemMap["content"])
					if text != "" {
						systemParts = append(systemParts, text)
					}
					continue
				}
				content, itemBlockers := convertResponsesRequestContentToChatContent(itemMap["content"])
				blockers = append(blockers, itemBlockers...)
				if role == "" {
					role = "user"
				}
				extra := extractResponsesReasoningForChatMessage(itemMap)
				if role == "assistant" && pendingReasoning != "" {
					if _, exists := extra["reasoning_content"]; !exists {
						if extra == nil {
							extra = map[string]any{}
						}
						extra["reasoning_content"] = pendingReasoning
					}
					pendingReasoning = ""
				}
				appendMessage(role, content, nil, extra)
				continue
			}

			switch itemType {
			case "reasoning":
				reasoningText := extractResponsesReasoningText(itemMap)
				if reasoningText != "" {
					pendingReasoning = joinNonEmptyLines(pendingReasoning, reasoningText)
				}
			case "function_call":
				toolCallID := firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), strings.TrimSpace(toStringValue(itemMap["id"])))
				name := strings.TrimSpace(toStringValue(itemMap["name"]))
				if toolCallID == "" || name == "" {
					blockers = append(blockers, "function_call_missing_identity")
					continue
				}
				queueToolCall(map[string]any{
					"id":   toolCallID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": stringifyJSON(itemMap["arguments"]),
					},
				})
			case "function_call_output":
				toolCallID := strings.TrimSpace(toStringValue(itemMap["call_id"]))
				if toolCallID == "" {
					blockers = append(blockers, "function_call_output_missing_call_id")
					continue
				}
				outputText := firstNonEmptyExact(
					openAIMessageContentToText(itemMap["output"]),
					openAIMessageContentToText(itemMap["content"]),
					toStringValue(itemMap["text"]),
				)
				appendToolOutput(toolCallID, outputText)
			case "web_search_call":
				continue
			case "custom_tool_call":
				toolCallID := firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), strings.TrimSpace(toStringValue(itemMap["id"])))
				name := strings.TrimSpace(toStringValue(itemMap["name"]))
				if toolCallID == "" || name == "" {
					blockers = append(blockers, "custom_tool_call_missing_identity")
					continue
				}
				arguments := firstNonEmptyExact(
					stringifyJSON(itemMap["input"]),
					stringifyJSON(itemMap["arguments"]),
				)
				queueToolCall(map[string]any{
					"id":   toolCallID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": arguments,
					},
				})
			case "custom_tool_call_output":
				toolCallID := strings.TrimSpace(toStringValue(itemMap["call_id"]))
				if toolCallID == "" {
					blockers = append(blockers, "custom_tool_call_output_missing_call_id")
					continue
				}
				outputText := firstNonEmptyExact(
					openAIMessageContentToText(itemMap["output"]),
					openAIMessageContentToText(itemMap["content"]),
					toStringValue(itemMap["text"]),
				)
				appendToolOutput(toolCallID, outputText)
			default:
				flushPendingToolCalls()
				blockers = append(blockers, "unsupported_input_type:"+itemType)
			}
		}
		flushPendingToolCalls()
		if pendingReasoning != "" {
			appendMessage("assistant", pendingReasoning, nil, map[string]any{
				"reasoning_content": pendingReasoning,
			})
		}
		if backfillToolReasoning {
			for _, message := range messages {
				if strings.TrimSpace(toStringValue(message["role"])) != "assistant" {
					continue
				}
				if _, ok := message["tool_calls"]; !ok {
					continue
				}
				if strings.TrimSpace(toStringValue(message["reasoning_content"])) == "" {
					message["reasoning_content"] = "tool call"
				}
			}
		}
	}

	if instructions := strings.TrimSpace(toStringValue(requestBody["instructions"])); instructions != "" {
		systemParts = append(systemParts, instructions)
	}

	chatBody := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   truthy(requestBody["stream"]),
	}
	if len(systemParts) > 0 {
		chatBody["messages"] = append([]map[string]any{{
			"role":    "system",
			"content": strings.Join(systemParts, "\n\n"),
		}}, messages...)
	}
	copyOptionalField(requestBody, chatBody, "temperature")
	copyOptionalField(requestBody, chatBody, "top_p")
	if maxTokens := toIntValue(requestBody["max_output_tokens"]); maxTokens > 0 {
		chatBody["max_tokens"] = maxTokens
	} else if maxTokens := toIntValue(requestBody["max_completion_tokens"]); maxTokens > 0 {
		chatBody["max_tokens"] = maxTokens
	}
	if reasoningMap, ok := requestBody["reasoning"].(map[string]any); ok {
		if effort := strings.TrimSpace(toStringValue(reasoningMap["effort"])); effort != "" {
			chatBody["reasoning_effort"] = effort
		}
	}
	if parallelToolCalls, ok := requestBody["parallel_tool_calls"].(bool); ok {
		chatBody["parallel_tool_calls"] = parallelToolCalls
	}

	tools, toolBlockers, hostedWebSearch := convertResponsesRequestToolsToChat(requestBody["tools"])
	if shouldUseOpenAIChatOnlyForResponsesProvider(provider) {
		hostedWebSearch = false
	}
	blockers = append(blockers, toolBlockers...)
	if len(tools) > 0 {
		chatBody["tools"] = tools
	}
	if toolChoice := convertResponsesRequestToolChoiceToChat(requestBody["tool_choice"]); toolChoice != nil {
		chatBody["tool_choice"] = toolChoice
	}

	chatRaw, err := json.Marshal(chatBody)
	if err != nil {
		return plan, err
	}

	plan.ChatBody = chatRaw
	plan.Blockers = compactStringList(blockers)
	plan.SupportsChat = len(plan.ChatBody) > 0
	plan.HostedWebSearch = hostedWebSearch
	if len(plan.Blockers) > 0 {
		plan.BlockedReason = strings.Join(plan.Blockers, ",")
	}
	return plan, nil
}

func compactStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func joinNonEmptyLines(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractResponsesReasoningForChatMessage(itemMap map[string]any) map[string]any {
	reasoningText := firstNonEmptyExact(
		openAIMessageContentToText(itemMap["reasoning_content"]),
		openAIMessageContentToText(itemMap["thinking"]),
		extractResponsesReasoningSummaryText(itemMap),
	)
	if reasoningText == "" {
		return nil
	}
	return map[string]any{
		"reasoning_content": reasoningText,
	}
}

func extractResponsesReasoningSummaryText(itemMap map[string]any) string {
	if itemMap == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	appendText := func(value any) {
		text := strings.TrimSpace(openAIMessageContentToText(value))
		if text == "" {
			text = strings.TrimSpace(toStringValue(value))
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	if rawSummary, ok := itemMap["summary"].([]any); ok {
		for _, rawItem := range rawSummary {
			summaryMap, ok := rawItem.(map[string]any)
			if !ok {
				appendText(rawItem)
				continue
			}
			appendText(firstNonEmptyExact(
				toStringValue(summaryMap["text"]),
				toStringValue(summaryMap["content"]),
			))
		}
	}
	appendText(itemMap["encrypted_content"])
	if rawDetails, ok := itemMap["details"].([]any); ok {
		for _, rawItem := range rawDetails {
			detailMap, ok := rawItem.(map[string]any)
			if !ok {
				appendText(rawItem)
				continue
			}
			appendText(firstNonEmptyExact(
				toStringValue(detailMap["text"]),
				toStringValue(detailMap["content"]),
			))
		}
	}
	return strings.Join(compactStringList(parts), "\n")
}

func extractResponsesReasoningText(itemMap map[string]any) string {
	if itemMap == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	appendText := func(value any) {
		text := strings.TrimSpace(openAIMessageContentToText(value))
		if text == "" {
			text = strings.TrimSpace(toStringValue(value))
		}
		if text != "" {
			parts = append(parts, text)
		}
	}
	appendText(itemMap["text"])
	appendText(itemMap["content"])
	appendText(itemMap["reasoning_content"])
	if rawSummary, ok := itemMap["summary"].([]any); ok {
		for _, rawItem := range rawSummary {
			summaryMap, ok := rawItem.(map[string]any)
			if !ok {
				appendText(rawItem)
				continue
			}
			appendText(firstNonEmptyExact(
				toStringValue(summaryMap["text"]),
				toStringValue(summaryMap["content"]),
			))
		}
	}
	appendText(itemMap["encrypted_content"])
	if rawDetails, ok := itemMap["details"].([]any); ok {
		for _, rawItem := range rawDetails {
			detailMap, ok := rawItem.(map[string]any)
			if !ok {
				appendText(rawItem)
				continue
			}
			appendText(firstNonEmptyExact(
				toStringValue(detailMap["text"]),
				toStringValue(detailMap["content"]),
			))
		}
	}
	return strings.Join(compactStringList(parts), "\n")
}

func convertResponsesRequestContentToChatContent(raw any) (any, []string) {
	switch typed := raw.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil
		}
		return text, nil
	case []any:
		parts := make([]map[string]any, 0, len(typed))
		blockers := make([]string, 0, 2)
		for _, item := range typed {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType := strings.ToLower(strings.TrimSpace(toStringValue(itemMap["type"])))
			switch itemType {
			case "input_text", "output_text", "text":
				text := firstNonEmptyExact(
					toStringValue(itemMap["text"]),
					toStringValue(itemMap["content"]),
					toStringValue(itemMap["refusal"]),
				)
				if text != "" {
					parts = append(parts, map[string]any{
						"type": "text",
						"text": text,
					})
				}
			case "input_image":
				imageURL := resolveResponsesRequestImageURL(itemMap)
				if imageURL == "" {
					blockers = append(blockers, "input_image_missing_url")
					continue
				}
				parts = append(parts, map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": imageURL,
					},
				})
			case "refusal":
				text := strings.TrimSpace(toStringValue(itemMap["text"]))
				if text != "" {
					parts = append(parts, map[string]any{
						"type": "text",
						"text": text,
					})
				}
			default:
				if itemType != "" {
					blockers = append(blockers, "unsupported_content_type:"+itemType)
				}
			}
		}
		if len(parts) == 0 {
			return nil, compactStringList(blockers)
		}
		textOnly := true
		textParts := make([]string, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(toStringValue(part["type"])) != "text" {
				textOnly = false
				break
			}
			textParts = append(textParts, toStringValue(part["text"]))
		}
		if textOnly {
			return strings.Join(textParts, "\n"), compactStringList(blockers)
		}
		items := make([]any, 0, len(parts))
		for _, part := range parts {
			items = append(items, part)
		}
		return items, compactStringList(blockers)
	default:
		return nil, nil
	}
}

func resolveResponsesRequestImageURL(itemMap map[string]any) string {
	if itemMap == nil {
		return ""
	}
	if imageURL := strings.TrimSpace(toStringValue(itemMap["image_url"])); imageURL != "" {
		return imageURL
	}
	if imageMap, ok := itemMap["image_url"].(map[string]any); ok {
		if imageURL := strings.TrimSpace(toStringValue(imageMap["url"])); imageURL != "" {
			return imageURL
		}
	}
	if sourceMap, ok := itemMap["source"].(map[string]any); ok {
		if dataURL := anthropicImageSourceToDataURL(sourceMap); dataURL != "" {
			return dataURL
		}
	}
	return ""
}

func convertResponsesRequestToolsToChat(raw any) ([]any, []string, bool) {
	typed, ok := raw.([]any)
	if !ok || len(typed) == 0 {
		return nil, nil, false
	}
	tools := make([]any, 0, len(typed))
	blockers := make([]string, 0, 2)
	hostedWebSearch := false
	for _, item := range typed {
		toolMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		toolType := strings.ToLower(strings.TrimSpace(toStringValue(toolMap["type"])))
		switch toolType {
		case "", "function":
			functionMap, _ := toolMap["function"].(map[string]any)
			name := strings.TrimSpace(toStringValue(toolMap["name"]))
			description := strings.TrimSpace(toStringValue(toolMap["description"]))
			parameters := toolMap["parameters"]
			strict, hasStrict := toolMap["strict"]
			if functionMap != nil {
				name = firstNonEmpty(name, strings.TrimSpace(toStringValue(functionMap["name"])))
				description = firstNonEmpty(description, strings.TrimSpace(toStringValue(functionMap["description"])))
				if parameters == nil {
					parameters = functionMap["parameters"]
				}
				if !hasStrict {
					strict, hasStrict = functionMap["strict"]
				}
			}
			if name == "" {
				blockers = append(blockers, "function_tool_missing_name")
				continue
			}
			functionPayload := map[string]any{
				"name":        name,
				"description": description,
				"parameters":  cleanJSONSchema(parameters),
			}
			if hasStrict {
				functionPayload["strict"] = strict
			}
			tools = append(tools, map[string]any{
				"type":     "function",
				"function": functionPayload,
			})
		case "custom", "custom_tool":
			name := strings.TrimSpace(toStringValue(toolMap["name"]))
			if name == "" {
				blockers = append(blockers, "custom_tool_missing_name")
				continue
			}
			description := strings.TrimSpace(toStringValue(toolMap["description"]))
			if formatMap, ok := toolMap["format"].(map[string]any); ok && len(formatMap) > 0 {
				description = strings.TrimSpace(firstNonEmpty(description, "Freeform custom tool input."))
				if formatType := strings.TrimSpace(toStringValue(formatMap["type"])); formatType != "" {
					description += "\nFormat type: " + formatType + "."
				}
				if syntax := strings.TrimSpace(toStringValue(formatMap["syntax"])); syntax != "" {
					description += "\nSyntax: " + syntax + "."
				}
			}
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        name,
					"description": description,
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"input": map[string]any{
								"type":        "string",
								"description": "Freeform input for the custom tool.",
							},
						},
						"required": []any{"input"},
					},
				},
			})
		case "web_search", "web_search_preview", "web_search_preview_2025_03_11":
			hostedWebSearch = true
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "web_search",
					"description": "Search the web for current information. This tool is executed by the AllApiDeck gateway.",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "Search query.",
							},
						},
						"required": []any{"query"},
					},
				},
			})
		default:
			blockers = append(blockers, "tool:"+toolType)
		}
	}
	return tools, compactStringList(blockers), hostedWebSearch
}

func convertResponsesRequestToolChoiceToChat(raw any) any {
	switch typed := raw.(type) {
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "required", "auto", "none":
			return normalized
		default:
			return nil
		}
	case map[string]any:
		choiceType := strings.ToLower(strings.TrimSpace(toStringValue(typed["type"])))
		if choiceType != "function" {
			return nil
		}
		name := strings.TrimSpace(toStringValue(typed["name"]))
		if name == "" {
			return nil
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": name,
			},
		}
	default:
		return nil
	}
}

func shouldFallbackResponsesToChat(statusCode int, responseBody []byte) bool {
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(firstNonEmpty(summarizeAdvancedProxyBody(responseBody), fmt.Sprintf("http %d", statusCode))))
	if message == "" {
		return false
	}
	switch {
	case strings.Contains(message, "unknown api route"):
		return true
	case strings.Contains(message, "does not support selected model"):
		return true
	case strings.Contains(message, "not support selected model"):
		return true
	case strings.Contains(message, "field messages is required"):
		return true
	case strings.Contains(message, "messages field is required"):
		return true
	case strings.Contains(message, "messages is required"):
		return true
	case strings.Contains(message, "invalid json"):
		return true
	case strings.Contains(message, "failed to deserialize") && strings.Contains(message, "tools"):
		return true
	case strings.Contains(message, "missing field") && strings.Contains(message, "tools"):
		return true
	case strings.Contains(message, "(html)"):
		return true
	case strings.Contains(message, "unsupported") && strings.Contains(message, "route"):
		return true
	case strings.Contains(message, "not implemented"):
		return true
	default:
		return false
	}
}

func shouldFallbackSuccessfulResponsesToChat(statusCode int, responseBody []byte) bool {
	if statusCode < 200 || statusCode >= 300 || !hasOpenAIErrorEnvelope(responseBody) {
		return false
	}
	return shouldFallbackResponsesToChat(statusCode, responseBody)
}

func hasOpenAIErrorEnvelope(responseBody []byte) bool {
	if len(responseBody) == 0 {
		return false
	}
	var decoded map[string]any
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return false
	}
	if errorValue, exists := decoded["error"]; exists {
		switch typed := errorValue.(type) {
		case map[string]any:
			return strings.TrimSpace(toStringValue(typed["message"])) != "" ||
				strings.TrimSpace(toStringValue(typed["code"])) != "" ||
				strings.TrimSpace(toStringValue(typed["type"])) != ""
		case string:
			return strings.TrimSpace(typed) != ""
		}
	}
	return strings.TrimSpace(toStringValue(decoded["message"])) != "" &&
		(strings.TrimSpace(toStringValue(decoded["code"])) != "" ||
			strings.Contains(strings.ToLower(strings.TrimSpace(toStringValue(decoded["type"]))), "error"))
}

func shouldFallbackChatPreferenceBackToResponses(statusCode int, responseBody []byte) bool {
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(firstNonEmpty(summarizeAdvancedProxyBody(responseBody), fmt.Sprintf("http %d", statusCode))))
	return strings.Contains(message, "unknown api route") || strings.Contains(message, "unsupported") || strings.Contains(message, "not implemented")
}

func convertOpenAIChatResponseBodyToResponses(rawBody []byte, fallbackModel string) ([]byte, error) {
	responseMap := map[string]any{}
	if err := json.Unmarshal(rawBody, &responseMap); err != nil {
		return nil, err
	}
	converted := convertOpenAIChatResponseMapToResponses(responseMap, fallbackModel)
	return json.Marshal(converted)
}

func convertOpenAIChatResponseMapToResponses(response map[string]any, fallbackModel string) map[string]any {
	choices, _ := response["choices"].([]any)
	createdAt := int64(toIntValue(response["created"]))
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}
	model := firstNonEmpty(strings.TrimSpace(toStringValue(response["model"])), strings.TrimSpace(fallbackModel))

	output := make([]any, 0, 3)
	if len(choices) > 0 {
		choiceMap, _ := choices[0].(map[string]any)
		messageMap, _ := choiceMap["message"].(map[string]any)
		if messageMap != nil {
			reasoningText := openAIMessageThinkingToText(messageMap)
			if reasoningText != "" {
				output = append(output, map[string]any{
					"id":     fmt.Sprintf("rs_%d", time.Now().UnixNano()),
					"type":   "reasoning",
					"status": "completed",
					"summary": []any{
						map[string]any{
							"type": "summary_text",
							"text": reasoningText,
						},
					},
				})
			}

			messageText := openAIMessageContentToText(messageMap["content"])
			if messageText != "" {
				output = append(output, map[string]any{
					"id":     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					"type":   "message",
					"status": "completed",
					"role":   "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": messageText,
						},
					},
				})
			}

			if toolCalls, ok := messageMap["tool_calls"].([]any); ok {
				for index, rawToolCall := range toolCalls {
					toolCallMap, ok := rawToolCall.(map[string]any)
					if !ok {
						continue
					}
					functionMap, _ := toolCallMap["function"].(map[string]any)
					callID := firstNonEmpty(strings.TrimSpace(toStringValue(toolCallMap["id"])), fmt.Sprintf("call_%d", index+1))
					output = append(output, map[string]any{
						"id":        fmt.Sprintf("fc_%s", callID),
						"type":      "function_call",
						"status":    "completed",
						"call_id":   callID,
						"name":      strings.TrimSpace(toStringValue(functionMap["name"])),
						"arguments": toStringValue(functionMap["arguments"]),
					})
				}
			}
		}
	}

	result := map[string]any{
		"id":         firstNonEmpty(strings.TrimSpace(toStringValue(response["id"])), fmt.Sprintf("resp_%d", time.Now().UnixNano())),
		"object":     "response",
		"created_at": createdAt,
		"model":      model,
		"status":     "completed",
		"output":     output,
	}

	if usageMap, ok := response["usage"].(map[string]any); ok && usageMap != nil {
		result["usage"] = map[string]any{
			"input_tokens":  toIntValue(usageMap["prompt_tokens"]),
			"output_tokens": toIntValue(usageMap["completion_tokens"]),
			"total_tokens":  toIntValue(usageMap["total_tokens"]),
		}
	}

	return result
}

func transformOpenAIChatStreamToResponsesStream(streamBody io.ReadCloser, fallbackModel string) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer streamBody.Close()
		defer writer.Close()

		responseID := ""
		model := strings.TrimSpace(fallbackModel)
		createdAt := time.Now().Unix()
		sequence := 0
		responseCreated := false
		responseCompleted := false
		finished := false
		usage := map[string]any{}

		messageItemID := ""
		messageOutputIndex := 0
		messageStarted := false
		contentPartStarted := false
		var messageText strings.Builder
		var reasoningText strings.Builder
		outputItems := make([]any, 0, 4)
		outputIndex := 0
		toolStates := map[int]*advancedProxyChatToResponsesToolState{}

		writeEvent := func(eventType string, payload map[string]any) error {
			if payload == nil {
				payload = map[string]any{}
			}
			payload["type"] = eventType
			payload["sequence_number"] = sequence
			sequence++
			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", eventType, string(raw)); err != nil {
				return err
			}
			return nil
		}

		emitResponseCreated := func() error {
			if responseCreated {
				return nil
			}
			if strings.TrimSpace(responseID) == "" {
				responseID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
			}
			responseCreated = true
			response := map[string]any{
				"id":         responseID,
				"object":     "response",
				"created_at": createdAt,
				"model":      model,
				"status":     "in_progress",
				"output":     []any{},
			}
			if len(usage) > 0 {
				response["usage"] = usage
			}
			if err := writeEvent("response.created", map[string]any{"response": response}); err != nil {
				return err
			}
			return writeEvent("response.in_progress", map[string]any{"response": response})
		}

		closeMessage := func() error {
			if !messageStarted {
				return nil
			}
			fullText := messageText.String()
			fullReasoning := strings.TrimSpace(reasoningText.String())
			if contentPartStarted {
				if err := writeEvent("response.output_text.done", map[string]any{
					"item_id":       messageItemID,
					"output_index":  messageOutputIndex,
					"content_index": 0,
					"text":          fullText,
				}); err != nil {
					return err
				}
				if err := writeEvent("response.content_part.done", map[string]any{
					"item_id":       messageItemID,
					"output_index":  messageOutputIndex,
					"content_index": 0,
					"part": map[string]any{
						"type": "output_text",
						"text": fullText,
					},
				}); err != nil {
					return err
				}
			}
			item := map[string]any{
				"id":     messageItemID,
				"type":   "message",
				"status": "completed",
				"role":   "assistant",
				"content": []any{
					map[string]any{
						"type": "output_text",
						"text": fullText,
					},
				},
			}
			if fullReasoning != "" {
				item["reasoning_content"] = fullReasoning
			}
			if err := writeEvent("response.output_item.done", map[string]any{
				"output_index": messageOutputIndex,
				"item":         item,
			}); err != nil {
				return err
			}
			outputItems = append(outputItems, item)
			messageStarted = false
			contentPartStarted = false
			messageItemID = ""
			messageText.Reset()
			reasoningText.Reset()
			return nil
		}

		ensureMessage := func() error {
			if messageStarted {
				return nil
			}
			messageStarted = true
			contentPartStarted = false
			messageOutputIndex = outputIndex
			outputIndex++
			messageItemID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
			return writeEvent("response.output_item.added", map[string]any{
				"output_index": messageOutputIndex,
				"item": map[string]any{
					"id":     messageItemID,
					"type":   "message",
					"status": "in_progress",
					"role":   "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": "",
						},
					},
				},
			})
		}

		closeTools := func() error {
			for _, state := range toolStates {
				if state == nil || state.Closed {
					continue
				}
				arguments := state.Arguments.String()
				if err := writeEvent("response.function_call_arguments.done", map[string]any{
					"item_id":      state.ItemID,
					"output_index": state.OutputIndex,
					"arguments":    arguments,
				}); err != nil {
					return err
				}
				item := map[string]any{
					"id":        state.ItemID,
					"type":      "function_call",
					"status":    "completed",
					"call_id":   state.CallID,
					"name":      state.Name,
					"arguments": arguments,
				}
				if err := writeEvent("response.output_item.done", map[string]any{
					"output_index": state.OutputIndex,
					"item":         item,
				}); err != nil {
					return err
				}
				outputItems = append(outputItems, item)
				state.Closed = true
			}
			return nil
		}

		emitCompleted := func() error {
			if responseCompleted {
				return nil
			}
			responseCompleted = true
			if err := closeMessage(); err != nil {
				return err
			}
			if err := closeTools(); err != nil {
				return err
			}
			response := map[string]any{
				"id":         firstNonEmpty(strings.TrimSpace(responseID), fmt.Sprintf("resp_%d", time.Now().UnixNano())),
				"object":     "response",
				"created_at": createdAt,
				"model":      model,
				"status":     "completed",
				"output":     outputItems,
			}
			if len(usage) > 0 {
				response["usage"] = usage
			}
			if err := writeEvent("response.completed", map[string]any{"response": response}); err != nil {
				return err
			}
			_, err := writer.Write([]byte("data: [DONE]\n\n"))
			return err
		}

		emitIncomplete := func(reason string) error {
			if responseCompleted {
				return nil
			}
			responseCompleted = true
			if strings.TrimSpace(reason) == "" {
				reason = "stream_interrupted"
			}
			if !responseCreated {
				if err := emitResponseCreated(); err != nil {
					return err
				}
			}
			if err := closeMessage(); err != nil {
				return err
			}
			if err := closeTools(); err != nil {
				return err
			}
			response := map[string]any{
				"id":         firstNonEmpty(strings.TrimSpace(responseID), fmt.Sprintf("resp_%d", time.Now().UnixNano())),
				"object":     "response",
				"created_at": createdAt,
				"model":      model,
				"status":     "incomplete",
				"output":     outputItems,
				"incomplete_details": map[string]any{
					"reason": reason,
				},
			}
			if len(usage) > 0 {
				response["usage"] = usage
			}
			appendAdvancedProxyLogf(
				"[OPENAI_CHAT_TO_RESPONSES_STREAM_INCOMPLETE] response_id=%s model=%s reason=%s output_items=%d",
				firstNonEmpty(strings.TrimSpace(responseID), "unknown"),
				previewAdvancedProxyText(model, 120),
				previewAdvancedProxyText(reason, 120),
				len(outputItems),
			)
			if err := writeEvent("response.incomplete", map[string]any{"response": response}); err != nil {
				return err
			}
			if err := writeEvent("response.completed", map[string]any{"response": response}); err != nil {
				return err
			}
			_, err := writer.Write([]byte("data: [DONE]\n\n"))
			return err
		}

		streamErr := streamAdvancedProxySSEDataPayloads(streamBody, func(payload string) (bool, error) {
			payload = strings.TrimSpace(payload)
			if payload == "" {
				return false, nil
			}
			if payload == "[DONE]" {
				finished = true
				if err := emitCompleted(); err != nil {
					return true, err
				}
				return true, nil
			}

			chunk := map[string]any{}
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				return false, nil
			}
			if chunkID := strings.TrimSpace(toStringValue(chunk["id"])); chunkID != "" && responseID == "" {
				responseID = chunkID
			}
			if chunkModel := strings.TrimSpace(toStringValue(chunk["model"])); chunkModel != "" {
				model = chunkModel
			}
			if created := int64(toIntValue(chunk["created"])); created > 0 && createdAt == 0 {
				createdAt = created
			}
			if usageMap, ok := chunk["usage"].(map[string]any); ok && usageMap != nil {
				usage = map[string]any{
					"input_tokens":  toIntValue(usageMap["prompt_tokens"]),
					"output_tokens": toIntValue(usageMap["completion_tokens"]),
					"total_tokens":  toIntValue(usageMap["total_tokens"]),
				}
			}
			if err := emitResponseCreated(); err != nil {
				return true, err
			}

			choices, _ := chunk["choices"].([]any)
			if len(choices) == 0 {
				return false, nil
			}
			choiceMap, _ := choices[0].(map[string]any)
			if choiceMap == nil {
				return false, nil
			}
			delta, _ := choiceMap["delta"].(map[string]any)
			if reasoning := firstNonEmptyExact(
				openAIMessageContentToText(delta["reasoning_content"]),
				openAIMessageContentToText(delta["reasoning"]),
				openAIMessageContentToText(delta["thinking"]),
			); reasoning != "" {
				if err := ensureMessage(); err != nil {
					return true, err
				}
				reasoningText.WriteString(reasoning)
			}
			if text := toStringValue(delta["content"]); text != "" {
				if err := ensureMessage(); err != nil {
					return true, err
				}
				if !contentPartStarted {
					contentPartStarted = true
					if err := writeEvent("response.content_part.added", map[string]any{
						"item_id":       messageItemID,
						"output_index":  messageOutputIndex,
						"content_index": 0,
						"part": map[string]any{
							"type": "output_text",
							"text": "",
						},
					}); err != nil {
						return true, err
					}
				}
				messageText.WriteString(text)
				if err := writeEvent("response.output_text.delta", map[string]any{
					"item_id":       messageItemID,
					"output_index":  messageOutputIndex,
					"content_index": 0,
					"delta":         text,
				}); err != nil {
					return true, err
				}
			}

			if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
				if err := closeMessage(); err != nil {
					return true, err
				}
				for _, rawToolCall := range toolCalls {
					toolCallMap, ok := rawToolCall.(map[string]any)
					if !ok {
						continue
					}
					index := toIntValue(toolCallMap["index"])
					state, exists := toolStates[index]
					if !exists {
						itemID := firstNonEmpty(strings.TrimSpace(toStringValue(toolCallMap["id"])), fmt.Sprintf("fc_%d_%d", time.Now().UnixNano(), index))
						state = &advancedProxyChatToResponsesToolState{
							ItemID:      itemID,
							CallID:      itemID,
							OutputIndex: outputIndex,
						}
						outputIndex++
						toolStates[index] = state
					}
					if toolID := strings.TrimSpace(toStringValue(toolCallMap["id"])); toolID != "" {
						state.CallID = toolID
						state.ItemID = fmt.Sprintf("fc_%s", toolID)
					}
					functionMap, _ := toolCallMap["function"].(map[string]any)
					if functionMap != nil {
						if name := strings.TrimSpace(toStringValue(functionMap["name"])); name != "" {
							state.Name = name
						}
					}
					if !state.Added && !state.Closed {
						if err := writeEvent("response.output_item.added", map[string]any{
							"output_index": state.OutputIndex,
							"item": map[string]any{
								"id":      state.ItemID,
								"type":    "function_call",
								"status":  "in_progress",
								"call_id": state.CallID,
								"name":    state.Name,
							},
						}); err != nil {
							return true, err
						}
						state.Added = true
					}
					if functionMap != nil {
						arguments := toStringValue(functionMap["arguments"])
						if arguments != "" {
							state.Arguments.WriteString(arguments)
							if err := writeEvent("response.function_call_arguments.delta", map[string]any{
								"item_id":      state.ItemID,
								"output_index": state.OutputIndex,
								"delta":        arguments,
							}); err != nil {
								return true, err
							}
						}
					}
				}
			}

			if finishReason := strings.TrimSpace(toStringValue(choiceMap["finish_reason"])); finishReason != "" {
				finished = true
				if err := closeMessage(); err != nil {
					return true, err
				}
				if err := closeTools(); err != nil {
					return true, err
				}
			}
			return false, nil
		})

		if streamErr != nil {
			appendAdvancedProxyLogf(
				"[OPENAI_CHAT_TO_RESPONSES_STREAM_ERROR] response_id=%s model=%s detail=%s",
				firstNonEmpty(strings.TrimSpace(responseID), "unknown"),
				previewAdvancedProxyText(model, 120),
				previewAdvancedProxyText(streamErr.Error(), 220),
			)
			if incompleteErr := emitIncomplete("stream_read_error"); incompleteErr != nil {
				_ = writer.CloseWithError(incompleteErr)
			}
			return
		}
		if finished {
			if err := emitCompleted(); err != nil {
				_ = writer.CloseWithError(err)
			}
			return
		}
		if responseCreated {
			if err := emitIncomplete("stream_ended_without_done"); err != nil {
				_ = writer.CloseWithError(err)
			}
		}
	}()
	return reader
}

func transformOpenAIChatResultToResponses(result rawProviderAttemptResult, fallbackModel string) (rawProviderAttemptResult, error) {
	converted := result
	if result.StreamBody != nil {
		converted.StreamBody = transformOpenAIChatStreamToResponsesStream(result.StreamBody, fallbackModel)
		if converted.Headers == nil {
			converted.Headers = http.Header{}
		} else {
			converted.Headers = converted.Headers.Clone()
		}
		converted.Headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		converted.Body = nil
		return converted, nil
	}

	convertedBody, err := convertOpenAIChatResponseBodyToResponses(result.Body, fallbackModel)
	if err != nil {
		return result, err
	}
	converted.Body = convertedBody
	if converted.Headers == nil {
		converted.Headers = http.Header{}
	} else {
		converted.Headers = converted.Headers.Clone()
	}
	converted.Headers.Set("Content-Type", "application/json; charset=utf-8")
	return converted, nil
}

func openAIResponsesBodyToSSEStream(rawBody []byte) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		responseMap := map[string]any{}
		_ = json.Unmarshal(rawBody, &responseMap)
		sequence := 0
		writeEvent := func(eventType string, payload map[string]any) error {
			if payload == nil {
				payload = map[string]any{}
			}
			payload["type"] = eventType
			payload["sequence_number"] = sequence
			sequence++
			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", eventType, string(raw))
			return err
		}
		responseID := firstNonEmpty(strings.TrimSpace(toStringValue(responseMap["id"])), fmt.Sprintf("resp_%d", time.Now().UnixNano()))
		responseMap["id"] = responseID
		responseMap["object"] = "response"
		responseMap["status"] = firstNonEmpty(strings.TrimSpace(toStringValue(responseMap["status"])), "completed")
		if _, exists := responseMap["created_at"]; !exists {
			responseMap["created_at"] = time.Now().Unix()
		}
		startedResponse := make(map[string]any, len(responseMap))
		for key, value := range responseMap {
			startedResponse[key] = value
		}
		startedResponse["status"] = "in_progress"
		startedResponse["output"] = []any{}
		if err := writeEvent("response.created", map[string]any{"response": startedResponse}); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if err := writeEvent("response.in_progress", map[string]any{"response": startedResponse}); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		outputItems, _ := responseMap["output"].([]any)
		for outputIndex, rawItem := range outputItems {
			itemMap, _ := rawItem.(map[string]any)
			if itemMap == nil {
				continue
			}
			itemID := firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["id"])), fmt.Sprintf("item_%d_%d", time.Now().UnixNano(), outputIndex))
			itemMap["id"] = itemID
			if err := writeEvent("response.output_item.added", map[string]any{"output_index": outputIndex, "item": itemMap}); err != nil {
				_ = writer.CloseWithError(err)
				return
			}
			content, _ := itemMap["content"].([]any)
			for contentIndex, rawContent := range content {
				contentMap, _ := rawContent.(map[string]any)
				if contentMap == nil {
					continue
				}
				if err := writeEvent("response.content_part.added", map[string]any{"item_id": itemID, "output_index": outputIndex, "content_index": contentIndex, "part": contentMap}); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
				if strings.TrimSpace(toStringValue(contentMap["type"])) == "output_text" {
					text := toStringValue(contentMap["text"])
					if text != "" {
						if err := writeEvent("response.output_text.delta", map[string]any{"item_id": itemID, "output_index": outputIndex, "content_index": contentIndex, "delta": text}); err != nil {
							_ = writer.CloseWithError(err)
							return
						}
						if err := writeEvent("response.output_text.done", map[string]any{"item_id": itemID, "output_index": outputIndex, "content_index": contentIndex, "text": text}); err != nil {
							_ = writer.CloseWithError(err)
							return
						}
					}
				}
				if err := writeEvent("response.content_part.done", map[string]any{"item_id": itemID, "output_index": outputIndex, "content_index": contentIndex, "part": contentMap}); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
			}
			if err := writeEvent("response.output_item.done", map[string]any{"output_index": outputIndex, "item": itemMap}); err != nil {
				_ = writer.CloseWithError(err)
				return
			}
		}
		responseMap["status"] = "completed"
		if err := writeEvent("response.completed", map[string]any{"response": responseMap}); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}()
	return reader
}

func transformAnthropicMessagesResultToResponses(result rawProviderAttemptResult, fallbackModel string) (rawProviderAttemptResult, error) {
	converted := result
	if result.StreamBody != nil {
		converted.StreamBody = transformAnthropicMessagesStreamToResponsesStream(result.StreamBody, fallbackModel)
		if converted.Headers == nil {
			converted.Headers = http.Header{}
		} else {
			converted.Headers = converted.Headers.Clone()
		}
		converted.Headers.Set("Content-Type", "text/event-stream; charset=utf-8")
		converted.Body = nil
		return converted, nil
	}

	convertedBody, err := convertAnthropicMessagesResponseBodyToResponses(result.Body, fallbackModel)
	if err != nil {
		return result, err
	}
	converted.Body = convertedBody
	if converted.Headers == nil {
		converted.Headers = http.Header{}
	} else {
		converted.Headers = converted.Headers.Clone()
	}
	converted.Headers.Set("Content-Type", "application/json; charset=utf-8")
	return converted, nil
}

func convertAnthropicMessagesResponseBodyToResponses(rawBody []byte, fallbackModel string) ([]byte, error) {
	var anthropicResp map[string]any
	if err := json.Unmarshal(rawBody, &anthropicResp); err != nil {
		return nil, err
	}

	model := firstNonEmpty(strings.TrimSpace(toStringValue(anthropicResp["model"])), fallbackModel)
	createdAt := time.Now().Unix()

	output := make([]any, 0, 2)

	contentBlocks, _ := anthropicResp["content"].([]any)
	for _, rawBlock := range contentBlocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			continue
		}
		blockType := strings.TrimSpace(toStringValue(block["type"]))
		if blockType == "text" {
			text := strings.TrimSpace(toStringValue(block["text"]))
			if text != "" {
				output = append(output, map[string]any{
					"id":     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					"type":   "message",
					"status": "completed",
					"role":   "assistant",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": text,
						},
					},
				})
			}
		}
	}

	responsesResp := map[string]any{
		"id":         firstNonEmpty(strings.TrimSpace(toStringValue(anthropicResp["id"])), fmt.Sprintf("resp_%d", time.Now().UnixNano())),
		"object":     "response",
		"created_at": createdAt,
		"model":      model,
		"status":     "completed",
		"output":     output,
	}

	if usageMap, ok := anthropicResp["usage"].(map[string]any); ok && usageMap != nil {
		responsesResp["usage"] = map[string]any{
			"input_tokens":  toIntValue(usageMap["input_tokens"]),
			"output_tokens": toIntValue(usageMap["output_tokens"]),
			"total_tokens":  toIntValue(usageMap["input_tokens"]) + toIntValue(usageMap["output_tokens"]),
		}
	}

	return json.Marshal(responsesResp)
}

func transformAnthropicMessagesStreamToResponsesStream(streamBody io.ReadCloser, fallbackModel string) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer streamBody.Close()
		defer writer.Close()

		scanner := bufio.NewScanner(streamBody)
		scanner.Buffer(make([]byte, 0, 64*1024), advancedProxySSEScannerMaxTokenSize)

		responseID := ""
		model := strings.TrimSpace(fallbackModel)
		createdAt := time.Now().Unix()
		sequence := 0
		responseCreated := false
		messageItemID := ""
		messageOutputIndex := 0
		var messageText strings.Builder
		outputItems := make([]any, 0, 2)

		writeEvent := func(eventType string, payload map[string]any) error {
			if payload == nil {
				payload = map[string]any{}
			}
			payload["type"] = eventType
			payload["sequence_number"] = sequence
			sequence++
			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", eventType, string(raw)); err != nil {
				return err
			}
			return nil
		}

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" {
				continue
			}

			var event map[string]any
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}

			eventType := strings.TrimSpace(toStringValue(event["type"]))

			switch eventType {
			case "message_start":
				if msgMap, ok := event["message"].(map[string]any); ok {
					responseID = strings.TrimSpace(toStringValue(msgMap["id"]))
					model = firstNonEmpty(strings.TrimSpace(toStringValue(msgMap["model"])), model)
				}
				if !responseCreated {
					responseCreated = true
					_ = writeEvent("response.created", map[string]any{
						"response": map[string]any{
							"id":         firstNonEmpty(responseID, fmt.Sprintf("resp_%d", time.Now().UnixNano())),
							"object":     "response",
							"created_at": createdAt,
							"model":      model,
							"status":     "in_progress",
							"output":     []any{},
						},
					})
				}

			case "content_block_start":
				if blockMap, ok := event["content_block"].(map[string]any); ok {
					if strings.TrimSpace(toStringValue(blockMap["type"])) == "text" {
						messageItemID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
						messageOutputIndex = len(outputItems)
						_ = writeEvent("response.output_item.added", map[string]any{
							"output_index": messageOutputIndex,
							"item": map[string]any{
								"id":     messageItemID,
								"type":   "message",
								"status": "in_progress",
								"role":   "assistant",
								"content": []any{
									map[string]any{
										"type": "output_text",
										"text": "",
									},
								},
							},
						})
						_ = writeEvent("response.content_part.added", map[string]any{
							"item_id":       messageItemID,
							"output_index":  messageOutputIndex,
							"content_index": 0,
							"part": map[string]any{
								"type": "output_text",
								"text": "",
							},
						})
					}
				}

			case "content_block_delta":
				if deltaMap, ok := event["delta"].(map[string]any); ok {
					if strings.TrimSpace(toStringValue(deltaMap["type"])) == "text_delta" {
						text := toStringValue(deltaMap["text"])
						if text != "" && messageItemID != "" {
							messageText.WriteString(text)
							_ = writeEvent("response.output_text.delta", map[string]any{
								"item_id":       messageItemID,
								"output_index":  messageOutputIndex,
								"content_index": 0,
								"delta":         text,
							})
						}
					}
				}

			case "content_block_stop":
				if messageItemID != "" {
					fullText := messageText.String()
					_ = writeEvent("response.output_text.done", map[string]any{
						"item_id":       messageItemID,
						"output_index":  messageOutputIndex,
						"content_index": 0,
						"text":          fullText,
					})
					_ = writeEvent("response.content_part.done", map[string]any{
						"item_id":       messageItemID,
						"output_index":  messageOutputIndex,
						"content_index": 0,
						"part": map[string]any{
							"type": "output_text",
							"text": fullText,
						},
					})
					_ = writeEvent("response.output_item.done", map[string]any{
						"output_index": messageOutputIndex,
						"item": map[string]any{
							"id":     messageItemID,
							"type":   "message",
							"status": "completed",
							"role":   "assistant",
							"content": []any{
								map[string]any{
									"type": "output_text",
									"text": fullText,
								},
							},
						},
					})
					outputItems = append(outputItems, map[string]any{
						"id":     messageItemID,
						"type":   "message",
						"status": "completed",
						"role":   "assistant",
						"content": []any{
							map[string]any{
								"type": "output_text",
								"text": fullText,
							},
						},
					})
					messageItemID = ""
					messageText.Reset()
				}

			case "message_delta":
				// Handle usage update if needed

			case "message_stop":
				_ = writeEvent("response.completed", map[string]any{
					"response": map[string]any{
						"id":         firstNonEmpty(responseID, fmt.Sprintf("resp_%d", time.Now().UnixNano())),
						"object":     "response",
						"created_at": createdAt,
						"model":      model,
						"status":     "completed",
						"output":     outputItems,
					},
				})
				_, _ = writer.Write([]byte("data: [DONE]\n\n"))
			}
		}
	}()
	return reader
}
