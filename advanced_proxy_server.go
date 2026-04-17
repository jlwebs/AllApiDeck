package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type providerAttemptResult struct {
	Response   map[string]any
	StatusCode int
	Message    string
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

func openAIMessageContentToText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, raw := range typed {
			contentMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			text := firstNonEmpty(
				strings.TrimSpace(toStringValue(contentMap["text"])),
				strings.TrimSpace(toStringValue(contentMap["content"])),
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

func mapOpenAIStopReason(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tool_calls", "function_call":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

func openAIChatToAnthropic(response map[string]any, fallbackModel string) map[string]any {
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
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": ""})
	}

	usage := map[string]any{}
	if usageMap, ok := response["usage"].(map[string]any); ok {
		usage["input_tokens"] = toIntValue(usageMap["prompt_tokens"])
		usage["output_tokens"] = toIntValue(usageMap["completion_tokens"])
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
		"usage":         usage,
	}
}

func openAIResponsesToAnthropic(response map[string]any, fallbackModel string) map[string]any {
	contentBlocks := make([]map[string]any, 0, 2)
	if outputText := strings.TrimSpace(toStringValue(response["output_text"])); outputText != "" {
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
							text := strings.TrimSpace(toStringValue(contentMap["text"]))
							if text != "" {
								contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": text})
							}
						case "refusal":
							text := strings.TrimSpace(toStringValue(contentMap["refusal"]))
							if text != "" {
								contentBlocks = append(contentBlocks, map[string]any{"type": "text", "text": text})
							}
						}
					}
				}
			case "function_call":
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
	stopReason := "end_turn"
	if strings.TrimSpace(toStringValue(response["status"])) == "incomplete" {
		stopReason = "max_tokens"
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
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage":         usage,
	}
}

func performJSONUpstreamRequest(method string, targetURL string, headers map[string]string, payload map[string]any, timeoutSeconds int) (int, []byte, error) {
	rawBody, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	request, err := http.NewRequest(method, targetURL, bytes.NewReader(rawBody))
	if err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		request.Header.Set(key, value)
	}
	client, err := newOutboundHTTPClient(time.Duration(clampInt(timeoutSeconds, 5, 900)) * time.Second)
	if err != nil {
		return 0, nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, body, nil
}

func performRawUpstreamRequest(method string, targetURL string, headers map[string]string, rawBody []byte, timeoutSeconds int, keepStream bool) (int, http.Header, []byte, io.ReadCloser, error) {
	request, err := http.NewRequest(method, targetURL, bytes.NewReader(rawBody))
	if err != nil {
		return 0, nil, nil, nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		request.Header.Set(key, value)
	}
	client, err := newOutboundHTTPClient(time.Duration(clampInt(timeoutSeconds, 5, 900)) * time.Second)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	if keepStream && response.StatusCode >= 200 && response.StatusCode < 300 {
		return response.StatusCode, response.Header.Clone(), nil, response.Body, nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return response.StatusCode, response.Header.Clone(), nil, nil, err
	}
	return response.StatusCode, response.Header.Clone(), body, nil, nil
}

func buildProviderHeaders(provider AdvancedProxyProvider, apiFormat string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"User-Agent":   "AllApiDeck/advanced-proxy",
	}
	if apiFormat == "anthropic" {
		headers["x-api-key"] = provider.APIKey
		headers["anthropic-version"] = "2023-06-01"
		return headers
	}
	headers["Authorization"] = "Bearer " + provider.APIKey
	return headers
}

func buildOpenAIProviderHeaders(provider AdvancedProxyProvider) map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
		"User-Agent":    "AllApiDeck/advanced-proxy",
		"Authorization": "Bearer " + provider.APIKey,
	}
}

func forwardClaudeRequestViaProvider(provider AdvancedProxyProvider, requestBody map[string]any, stream bool, config AdvancedProxyConfig) providerAttemptResult {
	failoverActive := config.Failover.Enabled && config.Failover.AutoFailoverEnabled
	timeoutSeconds := computeAdvancedProxyTimeoutSeconds(stream, failoverActive, config.Failover)
	apiFormat := normalizeClaudeAPIFormat(provider.APIFormat)

	targets := []string{}
	switch apiFormat {
	case "openai_chat":
		targets = buildCheckEndpointCandidates(provider.BaseURL)
	case "openai_responses":
		targets = buildResponsesEndpointCandidates(provider.BaseURL)
	default:
		targets = []string{resolveAnthropicMessagesEndpoint(provider.BaseURL)}
	}
	if len(targets) == 0 {
		return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "provider endpoint is empty"}
	}

	basePayload := deepCopyJSONMap(requestBody)
	basePayload["stream"] = false
	if strings.TrimSpace(provider.Model) != "" {
		basePayload["model"] = provider.Model
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

		for _, targetURL := range targets {
			statusCode, rawResponse, err := performJSONUpstreamRequest(http.MethodPost, targetURL, buildProviderHeaders(provider, apiFormat), transformed, timeoutSeconds)
			if err != nil {
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: err.Error()}
			}
			if statusCode < 200 || statusCode >= 300 {
				errorMessage := normalizeAnthropicErrorMessage(rawResponse)
				if apiFormat == "anthropic" && !signatureRectified && shouldRectifyThinkingSignature(errorMessage, config.Rectifier) && rectifyThinkingSignature(basePayload) {
					signatureRectified = true
					goto retryProvider
				}
				if apiFormat == "anthropic" && !budgetRectified && shouldRectifyThinkingBudget(errorMessage, config.Rectifier) && rectifyThinkingBudget(basePayload) {
					budgetRectified = true
					goto retryProvider
				}
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
				return providerAttemptResult{StatusCode: http.StatusBadGateway, Message: "invalid upstream JSON response"}
			}
			fallbackModel := firstNonEmpty(strings.TrimSpace(provider.Model), strings.TrimSpace(toStringValue(basePayload["model"])))
			switch apiFormat {
			case "openai_chat":
				responseMap = openAIChatToAnthropic(responseMap, fallbackModel)
			case "openai_responses":
				responseMap = openAIResponsesToAnthropic(responseMap, fallbackModel)
			}
			return providerAttemptResult{Response: responseMap, StatusCode: http.StatusOK}
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
		targets = buildCheckEndpointCandidates(provider.BaseURL)
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
		statusCode, headers, body, streamBody, err := performRawUpstreamRequest(http.MethodPost, targetURL, buildOpenAIProviderHeaders(provider), rawBody, timeoutSeconds, stream)
		if err != nil {
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
					"text": strings.TrimSpace(toStringValue(blockMap["text"])),
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
	if !config.Enabled || !config.Claude.Enabled || len(providers) == 0 {
		writeAnthropicProxyError(writer, http.StatusServiceUnavailable, "advanced Claude proxy is disabled or has no providers")
		return
	}

	var requestBody map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4*1024*1024)).Decode(&requestBody); err != nil {
		writeAnthropicProxyError(writer, http.StatusBadRequest, "invalid JSON request body")
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
		if failoverActive && !advancedProxyRuntime.Allow("claude", provider.ID, config.Failover) {
			continue
		}
		attempted++
		result := forwardClaudeRequestViaProvider(provider, requestBody, stream, config)
		if result.Response != nil && result.StatusCode >= 200 && result.StatusCode < 300 {
			if failoverActive {
				advancedProxyRuntime.Record("claude", provider.ID, config.Failover, true)
			}
			if stream {
				writeAnthropicSSE(writer, result.Response)
				return
			}
			writer.Header().Set("Content-Type", "application/json; charset=utf-8")
			writer.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(writer).Encode(result.Response)
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
