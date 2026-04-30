package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const advancedProxySSEScannerMaxTokenSize = 16 * 1024 * 1024

type providerAttemptResult struct {
	Response   map[string]any
	StatusCode int
	Message    string
	Headers    http.Header
	StreamBody io.ReadCloser
	APIFormat  string
	Model      string
}

type rawProviderAttemptResult struct {
	StatusCode int
	Message    string
	Body       []byte
	Headers    http.Header
	StreamBody io.ReadCloser
	ProviderID string
	Provider   string
	TargetURL  string
	RouteKind  string
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
			contentBlocks = append(contentBlocks, map[string]any{
				"type":  "tool_use",
				"id":    firstNonEmpty(strings.TrimSpace(toStringValue(toolCallMap["id"])), fmt.Sprintf("tool_%d", len(contentBlocks)+1)),
				"name":  strings.TrimSpace(toStringValue(functionMap["name"])),
				"input": parseJSONStringMap(functionMap["arguments"]),
			})
		}
	}
	if functionMap, ok := message["function_call"].(map[string]any); ok && functionMap != nil {
		contentBlocks = append(contentBlocks, map[string]any{
			"type":  "tool_use",
			"id":    fmt.Sprintf("tool_%d", len(contentBlocks)+1),
			"name":  strings.TrimSpace(toStringValue(functionMap["name"])),
			"input": parseJSONStringMap(functionMap["arguments"]),
		})
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
								contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": text})
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
				hasToolUse = true
				contentBlocks = append(contentBlocks, map[string]any{
					"type":  "tool_use",
					"id":    firstNonEmpty(strings.TrimSpace(toStringValue(outputMap["call_id"])), fmt.Sprintf("tool_%d", len(contentBlocks)+1)),
					"name":  strings.TrimSpace(toStringValue(outputMap["name"])),
					"input": parseJSONStringMap(outputMap["arguments"]),
				})
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
	client, err := newOutboundHTTPClient(time.Duration(clampInt(timeoutSeconds, 5, 900)) * time.Second)
	if err != nil {
		return 0, nil, nil, nil, time.Since(startedAt), err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, nil, nil, time.Since(startedAt), err
	}
	if keepStream && response.StatusCode >= 200 && response.StatusCode < 300 {
		return response.StatusCode, response.Header.Clone(), nil, response.Body, time.Since(startedAt), nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return response.StatusCode, response.Header.Clone(), nil, nil, time.Since(startedAt), err
	}
	return response.StatusCode, response.Header.Clone(), body, nil, time.Since(startedAt), nil
}

func writeAnthropicSSEFromOpenAIResponsesStream(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string) {
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	defer streamBody.Close()

	flusher, _ := writer.(http.Flusher)
	writeEvent := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(raw))
		if flusher != nil {
			flusher.Flush()
		}
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
	nextContentIndex := 0
	currentTextIndex := -1
	currentThinkingIndex := -1
	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	toolStates := map[string]*responsesToolStreamState{}

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
	startToolState := func(state *responsesToolStreamState) {
		if state == nil || state.Started || strings.TrimSpace(state.ID) == "" || strings.TrimSpace(state.Name) == "" {
			return
		}
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
	extractResponsesToolArguments := func(data map[string]any) string {
		if args := strings.TrimSpace(toStringValue(data["arguments"])); args != "" {
			return args
		}
		itemMap, _ := data["item"].(map[string]any)
		if args := strings.TrimSpace(toStringValue(itemMap["arguments"])); args != "" {
			return args
		}
		return ""
	}

	scanner := bufio.NewScanner(streamBody)
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
			index := ensureTextBlock("text")
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": index,
				"delta": map[string]any{
					"type": "text_delta",
					"text": delta,
				},
			})
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
			if strings.TrimSpace(toStringValue(itemMap["type"])) != "function_call" {
				return
			}
			hasToolUse = true
			closeIndex(&currentTextIndex)
			closeIndex(&currentThinkingIndex)
			state := resolveToolState(firstNonEmpty(resolveToolKey(data), resolveToolKey(itemMap)))
			state.ID = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), state.ID, strings.TrimSpace(toStringValue(itemMap["id"])))
			state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["name"])), state.Name)
			startToolState(state)
			emitToolArguments(state, strings.TrimSpace(toStringValue(itemMap["arguments"])))
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
			if usageMap, ok := responseData["usage"].(map[string]any); ok {
				usage = responsesUsageToAnthropic(usageMap)
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
	}
	emitMessageStop(mapOpenAIResponsesStopReason("completed", hasToolUse, ""))
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

func buildClaudeProviderHeaders(provider AdvancedProxyProvider, apiFormat string, requestHeaders http.Header, stream bool) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "AllApiDeck/advanced-proxy",
	}
	if stream {
		headers["Accept"] = "text/event-stream"
	} else {
		headers["Accept"] = "application/json"
	}
	if requestHeaders != nil {
		if userAgent := strings.TrimSpace(requestHeaders.Get("User-Agent")); userAgent != "" {
			headers["User-Agent"] = userAgent
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

func accumulateAdvancedProxyToolArguments(existing string, incoming string) string {
	if incoming == "" {
		return existing
	}
	if existing == "" {
		return incoming
	}
	if json.Valid([]byte(existing)) && json.Valid([]byte(incoming)) {
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

func mapOpenAIStopReasonOptional(value string) any {
	resolved := strings.TrimSpace(mapOpenAIStopReason(value))
	if resolved == "" {
		return nil
	}
	return resolved
}

func writeAnthropicSSEFromOpenAIChatStream(writer http.ResponseWriter, streamBody io.ReadCloser, fallbackModel string, includeThinking bool) {
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	defer streamBody.Close()

	flusher, _ := writer.(http.Flusher)
	writeEvent := func(event string, payload any) {
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, string(raw))
		if flusher != nil {
			flusher.Flush()
		}
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
	usage := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
	}
	toolStates := map[int]*anthropicToolStreamState{}
	openToolIndices := map[int]struct{}{}

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
		if len(openToolIndices) == 0 {
			return
		}
		indices := make([]int, 0, len(openToolIndices))
		for index := range openToolIndices {
			indices = append(indices, index)
		}
		sort.Ints(indices)
		for _, index := range indices {
			for _, state := range toolStates {
				if state == nil || !state.Started || state.Index != index {
					continue
				}
				if state.PendingArgs != "" && state.EmittedArgs != state.PendingArgs {
					writeEvent("content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": state.Index,
						"delta": map[string]any{
							"type":         "input_json_delta",
							"partial_json": state.PendingArgs,
						},
					})
					state.EmittedArgs = state.PendingArgs
				}
			}
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
	emitToolArguments := func(state *anthropicToolStreamState, incoming string) {
		if state == nil || incoming == "" {
			return
		}
		state.PendingArgs = accumulateAdvancedProxyToolArguments(state.PendingArgs, incoming)
	}

	scanner := bufio.NewScanner(streamBody)
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
					args := toStringValue(functionMap["arguments"])
					emitToolArguments(state, args)
				}
				if !state.Started && state.ID != "" && state.Name != "" {
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
	}

	closeCurrentBlock()
	closeOpenToolBlocks()
	emitMessageStart()
	emitMessageDelta()
	writeEvent("message_stop", map[string]any{"type": "message_stop"})
}

func buildOpenAIProviderHeaders(provider AdvancedProxyProvider) map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
		"User-Agent":    "AllApiDeck/advanced-proxy",
		"Authorization": "Bearer " + provider.APIKey,
	}
}

func forwardClaudeRequestViaProvider(provider AdvancedProxyProvider, requestBody map[string]any, requestHeaders http.Header, stream bool, config AdvancedProxyConfig) providerAttemptResult {
	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled
	timeoutSeconds := computeAdvancedProxyTimeoutSeconds(stream, failoverActive, config.Failover)
	capabilities := resolveAdvancedProxyProviderCapabilities(provider)
	apiFormat := capabilities.APIFormat
	debugEnabled := advancedProxyDebugEnabled(config)
	routeKind := "messages"
	switch apiFormat {
	case "openai_chat":
		routeKind = "chat"
	case "openai_responses":
		routeKind = "responses"
	}

	targets := []string{}
	switch apiFormat {
	case "openai_chat":
		targets = buildOpenAIChatCheckEndpointCandidates(provider.BaseURL)
	case "openai_responses":
		targets = buildResponsesEndpointCandidates(provider.BaseURL)
	default:
		targets = []string{resolveAnthropicMessagesEndpoint(provider.BaseURL)}
	}
	if len(targets) == 0 {
		return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "provider endpoint is empty"}
	}

	basePayload := deepCopyJSONMap(requestBody)
	basePayload["stream"] = stream
	if strings.TrimSpace(provider.Model) != "" {
		basePayload["model"] = provider.Model
	}
	if capabilities.SanitizeOrphanToolResults {
		sanitizedCount := sanitizeOrphanToolResults(basePayload)
		if sanitizedCount > 0 {
			appendAdvancedProxyLogf("[CLAUDE_PROXY_SANITIZE] provider=%s route=%s sanitized_orphan_tool_results=%d", advancedProxyProviderLabel(provider), routeKind, sanitizedCount)
		}
	}
	if debugEnabled {
		appendAdvancedProxyLogf(
			"[CLAUDE_PROXY_REQUEST] provider=%s format=%s route=%s stream=%t capabilities=%s payload=%s",
			advancedProxyProviderLabel(provider),
			apiFormat,
			routeKind,
			stream,
			summarizeAdvancedProxyJSON(capabilities, 320),
			summarizeAdvancedProxyJSON(basePayload, 1800),
		)
	}

	signatureRectified := false
	budgetRectified := false

	for {
		payload := deepCopyJSONMap(basePayload)
		var transformed map[string]any
		switch apiFormat {
		case "openai_chat":
			transformed = anthropicRequestToOpenAIChat(payload, provider)
		case "openai_responses":
			transformed = anthropicRequestToOpenAIResponses(payload, provider)
		default:
			transformed = payload
		}
		if debugEnabled {
			appendAdvancedProxyLogf(
				"[CLAUDE_PROXY_TRANSFORM] provider=%s format=%s route=%s transformed=%s",
				advancedProxyProviderLabel(provider),
				apiFormat,
				routeKind,
				summarizeAdvancedProxyJSON(transformed, 2200),
			)
		}

		for _, targetURL := range targets {
			advancedProxyRuntime.MarkDispatch("claude", provider, routeKind, targetURL)
			fallbackModel := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(basePayload["model"])))
			if stream && (apiFormat == "openai_chat" || apiFormat == "openai_responses") {
				rawTransformed, err := json.Marshal(transformed)
				if err != nil {
					advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, 0, 0, err)
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid upstream JSON request"}
				}
				statusCode, responseHeaders, rawResponse, streamBody, elapsed, err := performRawUpstreamRequest(http.MethodPost, targetURL, buildClaudeProviderHeaders(provider, apiFormat, requestHeaders, stream), rawTransformed, timeoutSeconds, true)
				if err != nil {
					advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, err)
					if debugEnabled {
						appendAdvancedProxyLogf("[CLAUDE_PROXY_STREAM_ERROR] provider=%s format=%s route=%s endpoint=%s detail=%s", advancedProxyProviderLabel(provider), apiFormat, routeKind, targetURL, previewAdvancedProxyText(err.Error(), 320))
					}
					return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: err.Error()}
				}
				if statusCode < 200 || statusCode >= 300 {
					advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					if streamBody != nil {
						streamBody.Close()
					}
					if debugEnabled {
						appendAdvancedProxyLogf("[CLAUDE_PROXY_STREAM_REJECT] provider=%s format=%s route=%s endpoint=%s status=%d detail=%s", advancedProxyProviderLabel(provider), apiFormat, routeKind, targetURL, statusCode, summarizeAdvancedProxyBody(rawResponse))
					}
					return providerAttemptResult{
						StatusCode: statusCode,
						Message:    fmt.Sprintf("HTTP %d", statusCode),
					}
				}
				advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, true)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
				return providerAttemptResult{
					StatusCode: http.StatusOK,
					Headers:    responseHeaders,
					StreamBody: streamBody,
					APIFormat:  apiFormat,
					Model:      fallbackModel,
				}
			}
			statusCode, responseHeaders, rawResponse, elapsed, err := performJSONUpstreamRequest(http.MethodPost, targetURL, buildClaudeProviderHeaders(provider, apiFormat, requestHeaders, stream), transformed, timeoutSeconds)
			if err != nil {
				advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, err)
				if debugEnabled {
					appendAdvancedProxyLogf("[CLAUDE_PROXY_ERROR] provider=%s format=%s route=%s endpoint=%s detail=%s", advancedProxyProviderLabel(provider), apiFormat, routeKind, targetURL, previewAdvancedProxyText(err.Error(), 320))
				}
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: err.Error()}
			}
			if statusCode < 200 || statusCode >= 300 {
				errorMessage := normalizeAnthropicErrorMessage(rawResponse)
				if debugEnabled {
					appendAdvancedProxyLogf("[CLAUDE_PROXY_REJECT] provider=%s format=%s route=%s endpoint=%s status=%d detail=%s raw=%s", advancedProxyProviderLabel(provider), apiFormat, routeKind, targetURL, statusCode, previewAdvancedProxyText(errorMessage, 320), summarizeAdvancedProxyBody(rawResponse))
				}
				if apiFormat == "anthropic" && !signatureRectified && shouldRectifyThinkingSignature(errorMessage, config.Rectifier) && rectifyThinkingSignature(basePayload) {
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					signatureRectified = true
					goto retryProvider
				}
				if apiFormat == "anthropic" && !budgetRectified && shouldRectifyThinkingBudget(errorMessage, config.Rectifier) && rectifyThinkingBudget(basePayload) {
					observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
					budgetRectified = true
					goto retryProvider
				}
				advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
				observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
				if isRetryableCheckStatus(statusCode) && (apiFormat == "openai_chat" || apiFormat == "openai_responses") {
					continue
				}
				return providerAttemptResult{
					StatusCode: statusCode,
					Message:    firstNonEmpty(errorMessage, fmt.Sprintf("HTTP %d", statusCode)),
				}
			}

			responseMap := map[string]any{}
			if err := json.Unmarshal(rawResponse, &responseMap); err != nil {
				advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, false)
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid upstream JSON response"}
			}
			switch apiFormat {
			case "openai_chat":
				responseMap = openAIChatToAnthropic(responseMap, fallbackModel, anthropicThinkingEnabled(requestBody))
			case "openai_responses":
				responseMap = openAIResponsesToAnthropic(responseMap, fallbackModel)
			}
			advancedProxyRuntime.MarkResult("claude", provider, routeKind, targetURL, true)
			observeAdvancedProxyAttempt("claude", provider, statusCode, elapsed, nil)
			return providerAttemptResult{Response: responseMap, StatusCode: http.StatusOK, Headers: responseHeaders}
		}

		return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "no compatible upstream endpoint found"}

	retryProvider:
		continue
	}
}

func forwardOpenAIRequestViaProvider(appType string, provider AdvancedProxyProvider, routeKind string, rawBody []byte, stream bool, config AdvancedProxyConfig) rawProviderAttemptResult {
	providerLabel := advancedProxyProviderLabel(provider)
	if normalizeClaudeAPIFormat(provider.APIFormat) == "anthropic" {
		return rawProviderAttemptResult{
			StatusCode: http.StatusBadGateway,
			Message:    formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "provider does not support OpenAI-compatible proxy routes"),
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}

	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled
	timeoutSeconds := computeAdvancedProxyTimeoutSeconds(stream, failoverActive, config.Failover)

	var targets []string
	switch routeKind {
	case "chat":
		targets = buildOpenAIChatCheckEndpointCandidates(provider.BaseURL)
	case "responses":
		targets = buildResponsesEndpointCandidates(provider.BaseURL)
	case "responses_compact":
		targets = buildResponsesCompactEndpointCandidates(provider.BaseURL)
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
	if len(targets) == 0 {
		return rawProviderAttemptResult{
			StatusCode: http.StatusBadGateway,
			Message:    formatAdvancedProxyFailure(appType, routeKind, provider, provider.BaseURL, "provider endpoint is empty"),
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  strings.TrimSpace(provider.BaseURL),
			RouteKind:  routeKind,
		}
	}

	lastStatus := http.StatusBadGateway
	lastMessage := formatAdvancedProxyFailure(appType, routeKind, provider, "", "no compatible upstream endpoint found")
	for _, targetURL := range targets {
		advancedProxyRuntime.MarkDispatch(appType, provider, routeKind, targetURL)
		appendAdvancedProxyLogf(
			"[OPENAI_PROXY_TRY] app=%s route=%s provider=%s endpoint=%s stream=%t timeout=%ds outbound=%s",
			appType,
			routeKind,
			providerLabel,
			targetURL,
			stream,
			timeoutSeconds,
			describeOutboundProxyMode(),
		)
		statusCode, headers, body, streamBody, elapsed, err := performRawUpstreamRequest(http.MethodPost, targetURL, buildOpenAIProviderHeaders(provider), rawBody, timeoutSeconds, stream)
		if err != nil {
			advancedProxyRuntime.MarkResult(appType, provider, routeKind, targetURL, false)
			observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, err)
			message := formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, fmt.Sprintf("upstream request failed (%s, outbound=%s)", err.Error(), describeOutboundProxyMode()))
			appendAdvancedProxyLogf("[OPENAI_PROXY_ERROR] status=%d app=%s route=%s provider=%s endpoint=%s detail=%s", http.StatusBadGateway, appType, routeKind, providerLabel, targetURL, previewAdvancedProxyText(message, 260))
			return rawProviderAttemptResult{
				StatusCode: http.StatusBadGateway,
				Message:    message,
				ProviderID: strings.TrimSpace(provider.ID),
				Provider:   providerLabel,
				TargetURL:  targetURL,
				RouteKind:  routeKind,
			}
		}
		if statusCode < 200 || statusCode >= 300 {
			advancedProxyRuntime.MarkResult(appType, provider, routeKind, targetURL, false)
			observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, nil)
			lastStatus = statusCode
			lastMessage = formatAdvancedProxyFailure(appType, routeKind, provider, targetURL, firstNonEmpty(summarizeAdvancedProxyBody(body), fmt.Sprintf("HTTP %d", statusCode)))
			appendAdvancedProxyLogf("[OPENAI_PROXY_FAIL] status=%d app=%s route=%s provider=%s endpoint=%s detail=%s", statusCode, appType, routeKind, providerLabel, targetURL, previewAdvancedProxyText(lastMessage, 260))
			if isRetryableCheckStatus(statusCode) {
				continue
			}
			return rawProviderAttemptResult{
				StatusCode: statusCode,
				Message:    lastMessage,
				Body:       body,
				Headers:    headers,
				ProviderID: strings.TrimSpace(provider.ID),
				Provider:   providerLabel,
				TargetURL:  targetURL,
				RouteKind:  routeKind,
			}
		}
		advancedProxyRuntime.MarkResult(appType, provider, routeKind, targetURL, true)
		observeAdvancedProxyAttempt(appType, provider, statusCode, elapsed, nil)
		appendAdvancedProxyLogf("[OPENAI_PROXY_OK] status=%d app=%s route=%s provider=%s endpoint=%s stream=%t", statusCode, appType, routeKind, providerLabel, targetURL, stream)
		return rawProviderAttemptResult{
			StatusCode: statusCode,
			Body:       body,
			Headers:    headers,
			StreamBody: streamBody,
			ProviderID: strings.TrimSpace(provider.ID),
			Provider:   providerLabel,
			TargetURL:  targetURL,
			RouteKind:  routeKind,
		}
	}

	return rawProviderAttemptResult{
		StatusCode: lastStatus,
		Message:    lastMessage,
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

func writeOpenAIProxyError(writer http.ResponseWriter, status int, message string) {
	resolvedMessage := firstNonEmpty(strings.TrimSpace(message), "advanced proxy request failed")
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"message": resolvedMessage,
		"detail":  resolvedMessage,
		"error": map[string]any{
			"type":    "invalid_request_error",
			"code":    "advanced_proxy_error",
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
		"listenHost":    config.ListenHost,
		"listenPort":    config.ListenPort,
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
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4*1024*1024)).Decode(&requestBody); err != nil {
		writeAnthropicProxyError(writer, http.StatusBadRequest, "invalid JSON request body")
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
			case "openai_chat":
				writeAnthropicSSEFromOpenAIChatStream(writer, result.StreamBody, result.Model, anthropicThinkingEnabled(requestBody))
			case "openai_responses":
				writeAnthropicSSEFromOpenAIResponsesStream(writer, result.StreamBody, result.Model)
			default:
				result.StreamBody.Close()
				writeAnthropicProxyError(writer, http.StatusBadGateway, "unsupported Claude streaming proxy format")
			}
			return
		}
		if failoverActive {
			advancedProxyRuntime.Record("claude", provider.ID, config.Failover, false)
		}
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
		writeOpenAIProxyError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	remoteIP := extractBridgeRemoteIP(request.RemoteAddr)
	if !isLoopbackBridgeRemote(remoteIP) {
		writeOpenAIProxyError(writer, http.StatusForbidden, "advanced proxy only accepts loopback requests")
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
		writeOpenAIProxyError(writer, http.StatusNotFound, "unsupported advanced proxy path")
		return
	}

	config, err := loadAdvancedProxyConfig()
	if err != nil {
		writeOpenAIProxyError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	providers := resolveAdvancedProxyEffectiveProviders(config, appType)
	providers = advancedProxyRuntime.OrderProvidersForDispatch(config, appType, providers)
	if !config.Enabled || !advancedProxyAppEnabled(config, appType) || len(providers) == 0 {
		writeOpenAIProxyError(writer, http.StatusServiceUnavailable, "advanced proxy is disabled or has no providers")
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 4*1024*1024))
	if err != nil {
		writeOpenAIProxyError(writer, http.StatusBadRequest, "failed to read request body")
		return
	}
	requestBody := map[string]any{}
	if err := json.Unmarshal(rawBody, &requestBody); err != nil {
		writeOpenAIProxyError(writer, http.StatusBadRequest, "invalid JSON request body")
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
		if result.StatusCode > 0 {
			lastStatus = result.StatusCode
		}
		if strings.TrimSpace(result.Message) != "" {
			lastMessage = result.Message
		}
	}

	appendAdvancedProxyLogf("[OPENAI_PROXY_FINAL_FAIL] status=%d app=%s route=%s detail=%s", lastStatus, appType, routeKind, previewAdvancedProxyText(lastMessage, 260))
	writeOpenAIProxyError(writer, lastStatus, lastMessage)
}
