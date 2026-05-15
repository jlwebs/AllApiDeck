package main

import (
	"bufio"
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
	ChatBody      []byte
	ScopeKey      string
	Model         string
	Blockers      []string
	SupportsChat  bool
	BlockedReason string
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

	blockers := make([]string, 0, 4)
	if previousResponseID := strings.TrimSpace(toStringValue(requestBody["previous_response_id"])); previousResponseID != "" {
		blockers = append(blockers, "previous_response_id")
	}
	if conversationID := strings.TrimSpace(toStringValue(requestBody["conversation"])); conversationID != "" {
		blockers = append(blockers, "conversation")
	}

	systemParts := make([]string, 0, 2)
	messages := make([]map[string]any, 0, 8)
	appendMessage := func(role string, content any, toolCalls []map[string]any) {
		payload := map[string]any{
			"role": role,
		}
		if content != nil {
			payload["content"] = content
		}
		if len(toolCalls) > 0 {
			payload["tool_calls"] = toolCalls
		}
		if content == nil && len(toolCalls) == 0 {
			return
		}
		messages = append(messages, payload)
	}

	switch typed := requestBody["input"].(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text != "" {
			appendMessage("user", text, nil)
		}
	case []any:
		for _, rawItem := range typed {
			itemMap, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			itemType := strings.ToLower(strings.TrimSpace(toStringValue(itemMap["type"])))
			role := strings.TrimSpace(toStringValue(itemMap["role"]))
			if role != "" || itemType == "message" || itemType == "input_text" || itemType == "input_image" || itemType == "text" || itemType == "output_text" || itemType == "" {
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
				appendMessage(role, content, nil)
				continue
			}

			switch itemType {
			case "reasoning":
				continue
			case "function_call":
				toolCallID := firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), strings.TrimSpace(toStringValue(itemMap["id"])))
				name := strings.TrimSpace(toStringValue(itemMap["name"]))
				if toolCallID == "" || name == "" {
					blockers = append(blockers, "function_call_missing_identity")
					continue
				}
				appendMessage("assistant", nil, []map[string]any{{
					"id":   toolCallID,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": stringifyJSON(itemMap["arguments"]),
					},
				}})
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
				appendMessage("tool", outputText, nil)
				if len(messages) > 0 {
					messages[len(messages)-1]["tool_call_id"] = toolCallID
				}
			case "web_search_call":
				blockers = append(blockers, "web_search_call")
			case "custom_tool_call", "custom_tool_call_output":
				blockers = append(blockers, itemType)
			default:
				blockers = append(blockers, "unsupported_input_type:"+itemType)
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

	tools, toolBlockers := convertResponsesRequestToolsToChat(requestBody["tools"])
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
	plan.SupportsChat = len(plan.Blockers) == 0
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

func convertResponsesRequestToolsToChat(raw any) ([]any, []string) {
	typed, ok := raw.([]any)
	if !ok || len(typed) == 0 {
		return nil, nil
	}
	tools := make([]any, 0, len(typed))
	blockers := make([]string, 0, 2)
	for _, item := range typed {
		toolMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		toolType := strings.ToLower(strings.TrimSpace(toStringValue(toolMap["type"])))
		switch toolType {
		case "", "function":
			name := strings.TrimSpace(toStringValue(toolMap["name"]))
			if name == "" {
				blockers = append(blockers, "function_tool_missing_name")
				continue
			}
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        name,
					"description": strings.TrimSpace(toStringValue(toolMap["description"])),
					"parameters":  cleanJSONSchema(toolMap["parameters"]),
				},
			})
		case "web_search":
			blockers = append(blockers, "tool:web_search")
		default:
			blockers = append(blockers, "tool:"+toolType)
		}
	}
	return tools, compactStringList(blockers)
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
	case strings.Contains(message, "invalid json"):
		return true
	case strings.Contains(message, "(html)"):
		return true
	case strings.Contains(message, "unsupported") && strings.Contains(message, "route"):
		return true
	default:
		return false
	}
}

func shouldFallbackChatPreferenceBackToResponses(statusCode int, responseBody []byte) bool {
	if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(firstNonEmpty(summarizeAdvancedProxyBody(responseBody), fmt.Sprintf("http %d", statusCode))))
	return strings.Contains(message, "unknown api route") || strings.Contains(message, "unsupported")
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

		scanner := bufio.NewScanner(streamBody)
		scanner.Buffer(make([]byte, 0, 64*1024), advancedProxySSEScannerMaxTokenSize)

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
				finished = true
				if err := emitCompleted(); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
				return
			}

			chunk := map[string]any{}
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
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
				_ = writer.CloseWithError(err)
				return
			}

			choices, _ := chunk["choices"].([]any)
			if len(choices) == 0 {
				continue
			}
			choiceMap, _ := choices[0].(map[string]any)
			if choiceMap == nil {
				continue
			}
			delta, _ := choiceMap["delta"].(map[string]any)
			if text := toStringValue(delta["content"]); text != "" {
				if err := ensureMessage(); err != nil {
					_ = writer.CloseWithError(err)
					return
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
						_ = writer.CloseWithError(err)
						return
					}
				}
				messageText.WriteString(text)
				if err := writeEvent("response.output_text.delta", map[string]any{
					"item_id":       messageItemID,
					"output_index":  messageOutputIndex,
					"content_index": 0,
					"delta":         text,
				}); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
			}

			if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
				if err := closeMessage(); err != nil {
					_ = writer.CloseWithError(err)
					return
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
							_ = writer.CloseWithError(err)
							return
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
								_ = writer.CloseWithError(err)
								return
							}
						}
					}
				}
			}

			if finishReason := strings.TrimSpace(toStringValue(choiceMap["finish_reason"])); finishReason != "" {
				finished = true
				if err := closeMessage(); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
				if err := closeTools(); err != nil {
					_ = writer.CloseWithError(err)
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if finished || responseCreated {
			if err := emitCompleted(); err != nil {
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
