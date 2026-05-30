package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type advancedProxySSEEvent struct {
	Event string
	Data  []string
}

type antiPoisonStreamToolState struct {
	Key       string
	Order     int
	Index     int
	Kind      string
	Name      string
	CallID    string
	Arguments string
}

type antiPoisonStreamTextState struct {
	Key  string
	Text string
}

func parseAdvancedProxySSEEvents(raw []byte) ([]advancedProxySSEEvent, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), advancedProxySSEScannerMaxTokenSize)
	events := make([]advancedProxySSEEvent, 0, 64)
	eventName := ""
	dataParts := make([]string, 0, 4)
	flush := func() {
		if eventName == "" && len(dataParts) == 0 {
			return
		}
		events = append(events, advancedProxySSEEvent{
			Event: strings.TrimSpace(eventName),
			Data:  append([]string(nil), dataParts...),
		})
		eventName = ""
		dataParts = dataParts[:0]
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			flush()
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
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func encodeAdvancedProxySSEEvents(events []advancedProxySSEEvent) []byte {
	var builder strings.Builder
	for _, event := range events {
		if strings.TrimSpace(event.Event) != "" {
			builder.WriteString("event: ")
			builder.WriteString(strings.TrimSpace(event.Event))
			builder.WriteString("\n")
		}
		for _, part := range event.Data {
			builder.WriteString("data: ")
			builder.WriteString(part)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func advancedProxySSEEventPayload(event advancedProxySSEEvent) string {
	return strings.Join(event.Data, "\n")
}

func advancedProxySSEJSONPayload(event advancedProxySSEEvent) (map[string]any, bool) {
	payload := strings.TrimSpace(advancedProxySSEEventPayload(event))
	if payload == "" || payload == "[DONE]" {
		return nil, false
	}
	data := map[string]any{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, false
	}
	return data, true
}

func setAdvancedProxySSEJSONPayload(event *advancedProxySSEEvent, data map[string]any) {
	if event == nil || data == nil {
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	event.Data = []string{string(raw)}
}

func antiPoisonStreamValidationResult(calls []antiPoisonToolCall, ctx antiPoisonRequestContext) antiPoisonValidationResult {
	return validateAndStripAntiPoisonToolCalls([]byte("{}"), calls, ctx, func() []byte {
		return []byte("{}")
	}, 0)
}

func appendAntiPoisonStreamOperation(records []antiPoisonOperationRecord, route string, provider string, channel string, rule string, before string, after string, count int, blocked bool, reason string) []antiPoisonOperationRecord {
	return append(records, antiPoisonOperationRecord{
		ID:       fmt.Sprintf("ap-stream-%s", randomAntiPoisonHex(4)),
		Time:     time.Now().Format(time.RFC3339Nano),
		Stage:    "respond in",
		Channel:  channel,
		Route:    route,
		Provider: provider,
		Rule:     rule,
		Path:     "stream.toolcalls",
		Before:   before,
		After:    after,
		Count:    count,
		Blocked:  blocked,
		Reason:   reason,
	})
}

func appendAntiPoisonStreamValidationOps(records []antiPoisonOperationRecord, result antiPoisonValidationResult, route string, provider string, channel string) []antiPoisonOperationRecord {
	if !result.Applied {
		return records
	}
	if result.Blocked {
		return appendAntiPoisonBlockedOperation(records, route, provider, channel, result.Reason)
	}
	if result.RemovedGuards > 0 {
		return appendAntiPoisonStreamOperation(
			records,
			route,
			provider,
			channel,
			"流式 guard JSON 剥离",
			fmt.Sprintf("real=%d guard=%d", result.RealCount, result.GuardCount),
			"guard stripped before client",
			result.RemovedGuards,
			false,
			result.Reason,
		)
	}
	return appendAntiPoisonStreamOperation(
		records,
		route,
		provider,
		channel,
		"流式 toolcall 校验",
		fmt.Sprintf("real=%d guard=%d", result.RealCount, result.GuardCount),
		"validated",
		result.RealCount+result.GuardCount,
		false,
		result.Reason,
	)
}

func restoreAntiPoisonStringProtectionInSSEBody(raw []byte, ctx *antiPoisonStringProtectionContext, route string, provider string, channel string) []byte {
	if ctx == nil || !ctx.Enabled || len(ctx.mapping) == 0 || len(raw) == 0 {
		return raw
	}
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		return restoreAntiPoisonStringProtectionInJSONBody(raw, ctx, route, provider, channel)
	}
	total := 0
	for index := range events {
		data, ok := advancedProxySSEJSONPayload(events[index])
		if !ok {
			continue
		}
		restored, count := restoreAntiPoisonStringValue(data, ctx.mapping)
		if count <= 0 {
			continue
		}
		restoredMap, _ := restored.(map[string]any)
		if restoredMap == nil {
			continue
		}
		setAdvancedProxySSEJSONPayload(&events[index], restoredMap)
		total += count
	}
	if total <= 0 {
		return raw
	}
	ctx.addRecord(antiPoisonOperationRecord{
		Stage:    "respond in",
		Channel:  channel,
		Route:    route,
		Provider: provider,
		Rule:     "字符串保护还原",
		Before:   fmt.Sprintf("%d placeholder(s)", total),
		After:    "restored for client stream",
		Count:    total,
	})
	appendAdvancedProxyLogf(
		"[ANTI_POISON_STRING_RESTORE] route=%s provider=%s channel=%s placeholders=%d mode=sse",
		previewAdvancedProxyText(route, 80),
		previewAdvancedProxyText(provider, 120),
		previewAdvancedProxyText(channel, 40),
		total,
	)
	return encodeAdvancedProxySSEEvents(events)
}

func writeOpenAIStreamAntiPoisonError(writer http.ResponseWriter, message string) {
	payload := map[string]any{
		"error": map[string]any{
			"message": firstNonEmpty(strings.TrimSpace(message), "AllApiDeck anti-poison validation failed"),
			"type":    "invalid_request_error",
			"code":    "anti_poison_validation_failed",
		},
	}
	raw, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(writer, "data: %s\n\n", string(raw))
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeAnthropicStreamAntiPoisonError(writer http.ResponseWriter, message string) {
	payload := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "invalid_request_error",
			"message": firstNonEmpty(strings.TrimSpace(message), "AllApiDeck anti-poison validation failed"),
		},
	}
	raw, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(writer, "event: error\ndata: %s\n\n", string(raw))
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func sanitizeAntiPoisonOpenAIStreamBody(raw []byte, observedFormat string, routeKind string, ctx antiPoisonRequestContext) ([]byte, antiPoisonValidationResult, error) {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	if !ctx.Enabled {
		return raw, antiPoisonValidationResult{Body: raw}, nil
	}
	switch normalizeAdvancedProxyObservedFormat(firstNonEmpty(observedFormat, routeKind)) {
	case "responses":
		return sanitizeAntiPoisonOpenAIResponsesStreamBody(raw, ctx)
	default:
		return sanitizeAntiPoisonOpenAIChatStreamBody(raw, ctx)
	}
}

func sanitizeAntiPoisonOpenAIChatStreamBody(raw []byte, ctx antiPoisonRequestContext) ([]byte, antiPoisonValidationResult, error) {
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		return raw, antiPoisonValidationResult{Applied: true, Body: raw, Blocked: antiPoisonShouldBlock(ctx.Config), Reason: "invalid_stream_sse"}, err
	}
	states := map[string]*antiPoisonStreamToolState{}
	textStates := map[string]*antiPoisonStreamTextState{}
	order := 0
	resolveTextState := func(key string) *antiPoisonStreamTextState {
		if strings.TrimSpace(key) == "" {
			key = fmt.Sprintf("choice:%d", len(textStates))
		}
		state, exists := textStates[key]
		if !exists {
			state = &antiPoisonStreamTextState{Key: key}
			textStates[key] = state
		}
		return state
	}
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			continue
		}
		choices, _ := data["choices"].([]any)
		for choiceOffset, rawChoice := range choices {
			choice, _ := rawChoice.(map[string]any)
			if choice == nil {
				continue
			}
			choiceIndex := toIntValue(choice["index"])
			if _, exists := choice["index"]; !exists {
				choiceIndex = choiceOffset
			}
			delta, _ := choice["delta"].(map[string]any)
			if delta == nil {
				continue
			}
			if content := toStringValue(delta["content"]); content != "" {
				resolveTextState(fmt.Sprintf("choice:%d", choiceIndex)).Text += content
			}
			toolCalls, _ := delta["tool_calls"].([]any)
			for toolOffset, rawCall := range toolCalls {
				callMap, _ := rawCall.(map[string]any)
				if callMap == nil {
					continue
				}
				toolIndex := toIntValue(callMap["index"])
				if _, exists := callMap["index"]; !exists {
					toolIndex = toolOffset
				}
				key := fmt.Sprintf("%d:%d", choiceIndex, toolIndex)
				state, exists := states[key]
				if !exists {
					state = &antiPoisonStreamToolState{Key: key, Order: order, Index: toolIndex}
					order++
					states[key] = state
				}
				if id := strings.TrimSpace(toStringValue(callMap["id"])); id != "" {
					state.CallID = id
				}
				functionMap, _ := callMap["function"].(map[string]any)
				if functionMap == nil {
					continue
				}
				if name := strings.TrimSpace(toStringValue(functionMap["name"])); name != "" {
					state.Name = accumulateAdvancedProxyToolArguments(state.Name, name)
				}
				if arguments := toStringValue(functionMap["arguments"]); arguments != "" {
					state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, arguments)
				}
			}
		}
	}
	calls := antiPoisonStreamStatesToCalls(states, "chat.tool_call")
	guardCalls, guardCount := antiPoisonStreamGuardCallsFromTextStates(textStates, ctx)
	if guardCount == 0 {
		guardCalls, guardCount = antiPoisonStreamGuardCallsFromRawEvents(events, ctx)
	}
	allCalls := append(calls, guardCalls...)
	result := validateAndStripAntiPoisonToolCalls([]byte("{}"), allCalls, ctx, func() []byte {
		return []byte("{}")
	}, guardCount)
	result.Body = raw
	result.RemovedGuards = guardCount
	if result.Blocked {
		return raw, result, nil
	}
	if guardCount == 0 {
		result.Body = raw
		return raw, result, nil
	}
	sanitized := stripAntiPoisonOpenAIChatStreamGuardEvents(events, result.RealCount == 0, ctx)
	result.Body = sanitized
	return sanitized, result, nil
}

func antiPoisonStreamStatesToCalls(states map[string]*antiPoisonStreamToolState, kind string) []antiPoisonToolCall {
	items := make([]*antiPoisonStreamToolState, 0, len(states))
	for _, state := range states {
		if state == nil || strings.TrimSpace(state.Name) == "" {
			continue
		}
		items = append(items, state)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})
	calls := make([]antiPoisonToolCall, 0, len(items))
	dedupeIndex := map[string]int{}
	for _, state := range items {
		callKind := strings.TrimSpace(state.Kind)
		if callKind == "" {
			callKind = kind
		}
		call := antiPoisonToolCall{
			Kind:          callKind,
			Name:          state.Name,
			CallID:        state.CallID,
			ArgumentsText: state.Arguments,
			ToolType:      classifyAntiPoisonToolName(state.Name),
			IsGuard:       false,
		}
		if key := antiPoisonStreamToolCallDedupeKey(call); key != "" {
			if index, exists := dedupeIndex[key]; exists {
				calls[index] = mergeAntiPoisonStreamToolCall(calls[index], call)
				continue
			}
			dedupeIndex[key] = len(calls)
		}
		calls = append(calls, call)
	}
	return calls
}

func antiPoisonStreamToolCallDedupeKey(call antiPoisonToolCall) string {
	if callID := strings.TrimSpace(call.CallID); callID != "" {
		return "call:" + callID
	}
	name := strings.TrimSpace(call.Name)
	args := canonicalAntiPoisonArgumentText(call.ArgumentsText)
	if name != "" && args != "" {
		return "nameargs:" + name + "|" + args
	}
	return ""
}

func mergeAntiPoisonStreamToolCall(current antiPoisonToolCall, next antiPoisonToolCall) antiPoisonToolCall {
	if strings.TrimSpace(current.Name) == "" {
		current.Name = next.Name
	}
	if strings.TrimSpace(current.CallID) == "" {
		current.CallID = next.CallID
	}
	if strings.TrimSpace(current.ToolType) == "" {
		current.ToolType = next.ToolType
	}
	currentArgs := strings.TrimSpace(current.ArgumentsText)
	nextArgs := strings.TrimSpace(next.ArgumentsText)
	if currentArgs == "" || (nextArgs != "" && len(nextArgs) > len(currentArgs)) {
		current.ArgumentsText = next.ArgumentsText
	}
	return current
}

func antiPoisonStreamGuardCallsFromTextStates(states map[string]*antiPoisonStreamTextState, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	guards := make([]antiPoisonGuardPayload, 0, 4)
	for _, key := range keys {
		state := states[key]
		if state == nil || strings.TrimSpace(state.Text) == "" {
			continue
		}
		extracted := extractAntiPoisonGuardsFromText(state.Text, ctx)
		guards = append(guards, extracted.GuardBlocks...)
	}
	return antiPoisonGuardPayloadsToToolCalls(guards), len(guards)
}

func extractAntiPoisonGuardsFromEscapedSSEPayload(payload string, ctx antiPoisonRequestContext) antiPoisonTextGuardExtraction {
	payload = firstNonEmptyExact(payload)
	if strings.TrimSpace(payload) == "" || !strings.Contains(payload, antiPoisonGuardJSONOpenTag) {
		return antiPoisonTextGuardExtraction{Text: payload}
	}
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(antiPoisonGuardJSONOpenTag) + `.*?` + regexp.QuoteMeta(antiPoisonGuardJSONCloseTag))
	guards := make([]antiPoisonGuardPayload, 0, 4)
	stripped := pattern.ReplaceAllStringFunc(payload, func(match string) string {
		decoded := ""
		if err := json.Unmarshal([]byte(`"`+match+`"`), &decoded); err == nil {
			extracted := extractAntiPoisonGuardsFromText(decoded, ctx)
			guards = append(guards, extracted.GuardBlocks...)
		}
		return ""
	})
	return antiPoisonTextGuardExtraction{
		Text:        stripped,
		GuardCount:  len(guards),
		GuardBlocks: guards,
	}
}

func antiPoisonStreamGuardCallsFromRawEvents(events []advancedProxySSEEvent, ctx antiPoisonRequestContext) ([]antiPoisonToolCall, int) {
	guards := make([]antiPoisonGuardPayload, 0, 4)
	for _, event := range events {
		payload := advancedProxySSEEventPayload(event)
		if strings.TrimSpace(payload) == "" || payload == "[DONE]" || !strings.Contains(payload, antiPoisonGuardJSONOpenTag) {
			continue
		}
		if _, ok := advancedProxySSEJSONPayload(event); ok {
			continue
		}
		extracted := extractAntiPoisonGuardsFromEscapedSSEPayload(payload, ctx)
		guards = append(guards, extracted.GuardBlocks...)
	}
	return antiPoisonGuardPayloadsToToolCalls(guards), len(guards)
}

func stripAntiPoisonGuardJSONFromSSEPayload(payload string, ctx antiPoisonRequestContext) string {
	if strings.TrimSpace(payload) == "" || payload == "[DONE]" || !strings.Contains(payload, antiPoisonGuardJSONOpenTag) {
		return payload
	}
	extracted := extractAntiPoisonGuardsFromEscapedSSEPayload(payload, ctx)
	if extracted.GuardCount <= 0 {
		return payload
	}
	return extracted.Text
}

func stripAntiPoisonGuardJSONFromSSEEvent(event *advancedProxySSEEvent, ctx antiPoisonRequestContext) {
	if event == nil {
		return
	}
	for index, part := range event.Data {
		event.Data[index] = stripAntiPoisonGuardJSONFromSSEPayload(part, ctx)
	}
}

func stripAntiPoisonOpenAIChatStreamGuardEvents(events []advancedProxySSEEvent, guardOnly bool, ctx antiPoisonRequestContext) []byte {
	next := make([]advancedProxySSEEvent, 0, len(events))
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
			next = append(next, event)
			continue
		}
		choices, _ := data["choices"].([]any)
		for choiceOffset, rawChoice := range choices {
			choice, _ := rawChoice.(map[string]any)
			if choice == nil {
				continue
			}
			_ = choiceOffset
			delta, _ := choice["delta"].(map[string]any)
			if delta != nil {
				if content := toStringValue(delta["content"]); content != "" {
					delta["content"] = extractAntiPoisonGuardsFromText(content, ctx).Text
				}
			}
			if guardOnly && strings.TrimSpace(toStringValue(choice["finish_reason"])) == "tool_calls" {
				choice["finish_reason"] = "stop"
			}
		}
		setAdvancedProxySSEJSONPayload(&event, data)
		stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
		next = append(next, event)
	}
	return encodeAdvancedProxySSEEvents(next)
}

func sanitizeAntiPoisonOpenAIResponsesStreamBody(raw []byte, ctx antiPoisonRequestContext) ([]byte, antiPoisonValidationResult, error) {
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		return raw, antiPoisonValidationResult{Applied: true, Body: raw, Blocked: antiPoisonShouldBlock(ctx.Config), Reason: "invalid_stream_sse"}, err
	}
	states := map[string]*antiPoisonStreamToolState{}
	textStates := map[string]*antiPoisonStreamTextState{}
	order := 0
	resolveState := func(key string) *antiPoisonStreamToolState {
		if strings.TrimSpace(key) == "" {
			key = fmt.Sprintf("auto:%d", order)
		}
		state, exists := states[key]
		if !exists {
			state = &antiPoisonStreamToolState{Key: key, Order: order}
			order++
			states[key] = state
		}
		return state
	}
	resolveTextState := func(key string) *antiPoisonStreamTextState {
		if strings.TrimSpace(key) == "" {
			key = fmt.Sprintf("text:%d", len(textStates))
		}
		state, exists := textStates[key]
		if !exists {
			state = &antiPoisonStreamTextState{Key: key}
			textStates[key] = state
		}
		return state
	}
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
			if itemType == "function_call" {
				state := resolveState(resolveAntiPoisonResponsesStreamToolKey(data, itemMap))
				state.Kind = firstNonEmpty("responses.function_call", state.Kind)
				state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["call_id"])), state.CallID, strings.TrimSpace(toStringValue(itemMap["id"])))
				state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["name"])), state.Name)
				if args := stringifyAntiPoisonStreamArguments(itemMap["arguments"]); args != "" {
					state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
				}
				continue
			}
			if itemType == "web_search_call" {
				state := resolveState(resolveAntiPoisonResponsesStreamToolKey(data, itemMap))
				state.Kind = firstNonEmpty("responses.web_search_call", state.Kind)
				state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(itemMap["id"])), state.CallID, strings.TrimSpace(toStringValue(data["item_id"])))
				state.Name = firstNonEmpty("web_search_call", state.Name)
				if args := stringifyAntiPoisonStreamArguments(itemMap["action"]); args != "" {
					state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
				}
				continue
			}
			if itemType == "message" {
				for contentIndex, rawContent := range anySliceValue(itemMap["content"]) {
					contentMap, _ := rawContent.(map[string]any)
					text := firstNonEmptyExact(toStringValue(contentMap["text"]), toStringValue(contentMap["content"]))
					if text != "" {
						resolveTextState(fmt.Sprintf("%s:content:%d", resolveAntiPoisonResponsesStreamToolKey(data, itemMap), contentIndex)).Text += text
					}
				}
			}
		case "response.function_call_arguments.delta", "response.function_call_arguments.done":
			state := resolveState(resolveAntiPoisonResponsesStreamToolKey(data, itemMap))
			state.Kind = firstNonEmpty("responses.function_call", state.Kind)
			state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(data["call_id"])), state.CallID, strings.TrimSpace(toStringValue(data["item_id"])))
			state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(data["name"])), state.Name, strings.TrimSpace(toStringValue(itemMap["name"])))
			if delta := toStringValue(data["delta"]); delta != "" {
				state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, delta)
			}
			if args := stringifyAntiPoisonStreamArguments(data["arguments"]); args != "" {
				state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
			}
			if args := stringifyAntiPoisonStreamArguments(itemMap["arguments"]); args != "" {
				state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
			}
		case "response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done":
			text := firstNonEmptyExact(toStringValue(data["delta"]), toStringValue(data["text"]))
			if text != "" {
				key := fmt.Sprintf("item:%s:content:%s", strings.TrimSpace(toStringValue(data["item_id"])), strings.TrimSpace(toStringValue(data["content_index"])))
				resolveTextState(key).Text += text
			}
		case "response.completed":
			responseMap, _ := data["response"].(map[string]any)
			for _, rawItem := range anySliceValue(responseMap["output"]) {
				outputItem, _ := rawItem.(map[string]any)
				switch strings.TrimSpace(toStringValue(outputItem["type"])) {
				case "function_call":
					state := resolveState(resolveAntiPoisonResponsesStreamToolKey(map[string]any{}, outputItem))
					state.Kind = firstNonEmpty("responses.function_call", state.Kind)
					state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(outputItem["call_id"])), state.CallID, strings.TrimSpace(toStringValue(outputItem["id"])))
					state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(outputItem["name"])), state.Name)
					if args := stringifyAntiPoisonStreamArguments(outputItem["arguments"]); args != "" {
						state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
					}
				case "web_search_call":
					state := resolveState(resolveAntiPoisonResponsesStreamToolKey(map[string]any{}, outputItem))
					state.Kind = firstNonEmpty("responses.web_search_call", state.Kind)
					state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(outputItem["id"])), state.CallID)
					state.Name = firstNonEmpty("web_search_call", state.Name)
					if args := stringifyAntiPoisonStreamArguments(outputItem["action"]); args != "" {
						state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
					}
				case "message":
					itemKey := resolveAntiPoisonResponsesStreamToolKey(map[string]any{}, outputItem)
					for contentIndex, rawContent := range anySliceValue(outputItem["content"]) {
						contentMap, _ := rawContent.(map[string]any)
						text := firstNonEmptyExact(toStringValue(contentMap["text"]), toStringValue(contentMap["content"]))
						if text != "" {
							resolveTextState(fmt.Sprintf("%s:content:%d", itemKey, contentIndex)).Text += text
						}
					}
				}
			}
		}
	}
	calls := antiPoisonStreamStatesToCalls(states, "responses.function_call")
	guardCalls, guardCount := antiPoisonStreamGuardCallsFromTextStates(textStates, ctx)
	if guardCount == 0 {
		guardCalls, guardCount = antiPoisonStreamGuardCallsFromRawEvents(events, ctx)
	}
	allCalls := append(calls, guardCalls...)
	result := validateAndStripAntiPoisonToolCalls([]byte("{}"), allCalls, ctx, func() []byte {
		return []byte("{}")
	}, guardCount)
	result.Body = raw
	result.RemovedGuards = guardCount
	if result.Blocked {
		return raw, result, nil
	}
	if guardCount == 0 {
		return raw, result, nil
	}
	sanitized := stripAntiPoisonOpenAIResponsesStreamGuardEvents(events, ctx)
	result.Body = sanitized
	return sanitized, result, nil
}

func resolveAntiPoisonResponsesStreamToolKey(data map[string]any, item map[string]any) string {
	for _, source := range []map[string]any{data, item} {
		if source == nil {
			continue
		}
		if itemID := strings.TrimSpace(toStringValue(source["item_id"])); itemID != "" {
			return "item:" + itemID
		}
		if itemID := strings.TrimSpace(toStringValue(source["id"])); itemID != "" {
			return "item:" + itemID
		}
		if callID := strings.TrimSpace(toStringValue(source["call_id"])); callID != "" {
			return "call:" + callID
		}
	}
	if data != nil {
		if outputIndex := toIntValue(data["output_index"]); outputIndex > 0 || toStringValue(data["output_index"]) == "0" {
			return fmt.Sprintf("output:%d", outputIndex)
		}
	}
	return ""
}

func stringifyAntiPoisonStreamArguments(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

func anySliceValue(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func stripAntiPoisonOpenAIResponsesStreamGuardEvents(events []advancedProxySSEEvent, ctx antiPoisonRequestContext) []byte {
	next := make([]advancedProxySSEEvent, 0, len(events))
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
			next = append(next, event)
			continue
		}
		eventType := firstNonEmpty(strings.TrimSpace(event.Event), strings.TrimSpace(toStringValue(data["type"])))
		itemMap, _ := data["item"].(map[string]any)
		switch eventType {
		case "response.output_item.added", "response.output_item.done":
			if strings.TrimSpace(toStringValue(itemMap["type"])) == "message" {
				for _, rawContent := range anySliceValue(itemMap["content"]) {
					contentMap, _ := rawContent.(map[string]any)
					if _, exists := contentMap["text"]; exists {
						contentMap["text"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["text"]), ctx).Text
					}
					if _, exists := contentMap["content"]; exists {
						contentMap["content"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["content"]), ctx).Text
					}
				}
			}
		case "response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done":
			if value := firstNonEmptyExact(toStringValue(data["delta"]), toStringValue(data["text"])); value != "" {
				clean := extractAntiPoisonGuardsFromText(value, ctx).Text
				if _, exists := data["delta"]; exists {
					data["delta"] = clean
				}
				if _, exists := data["text"]; exists {
					data["text"] = clean
				}
			}
		case "response.completed":
			responseMap, _ := data["response"].(map[string]any)
			if responseMap != nil {
				responseMap["output"] = stripAntiPoisonResponsesStreamOutput(responseMap["output"], ctx)
			}
		}
		setAdvancedProxySSEJSONPayload(&event, data)
		stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
		next = append(next, event)
	}
	return encodeAdvancedProxySSEEvents(next)
}

func stripAntiPoisonResponsesStreamOutput(rawOutput any, ctx antiPoisonRequestContext) []any {
	output := anySliceValue(rawOutput)
	next := make([]any, 0, len(output))
	for _, rawItem := range output {
		item, _ := rawItem.(map[string]any)
		if strings.TrimSpace(toStringValue(item["type"])) == "message" {
			for _, rawContent := range anySliceValue(item["content"]) {
				contentMap, _ := rawContent.(map[string]any)
				if _, exists := contentMap["text"]; exists {
					contentMap["text"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["text"]), ctx).Text
				}
				if _, exists := contentMap["content"]; exists {
					contentMap["content"] = extractAntiPoisonGuardsFromText(toStringValue(contentMap["content"]), ctx).Text
				}
			}
		}
		next = append(next, rawItem)
	}
	return next
}

func sanitizeAntiPoisonAnthropicStreamBody(raw []byte, ctx antiPoisonRequestContext) ([]byte, antiPoisonValidationResult, error) {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	if !ctx.Enabled {
		return raw, antiPoisonValidationResult{Body: raw}, nil
	}
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil {
		return raw, antiPoisonValidationResult{Applied: true, Body: raw, Blocked: antiPoisonShouldBlock(ctx.Config), Reason: "invalid_stream_sse"}, err
	}
	states := map[int]*antiPoisonStreamToolState{}
	textStates := map[int]*antiPoisonStreamTextState{}
	order := 0
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			continue
		}
		eventType := firstNonEmpty(strings.TrimSpace(event.Event), strings.TrimSpace(toStringValue(data["type"])))
		index := toIntValue(data["index"])
		switch eventType {
		case "content_block_start":
			block, _ := data["content_block"].(map[string]any)
			blockType := strings.TrimSpace(toStringValue(block["type"]))
			if blockType == "tool_use" {
				state := states[index]
				if state == nil {
					state = &antiPoisonStreamToolState{Key: fmt.Sprintf("%d", index), Order: order, Index: index}
					order++
					states[index] = state
				}
				state.CallID = firstNonEmpty(strings.TrimSpace(toStringValue(block["id"])), state.CallID)
				state.Name = firstNonEmpty(strings.TrimSpace(toStringValue(block["name"])), state.Name)
				if args := stringifyAntiPoisonStreamArguments(block["input"]); args != "" {
					state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, args)
				}
				continue
			}
			if blockType == "text" || blockType == "thinking" || blockType == "redacted_thinking" {
				state := textStates[index]
				if state == nil {
					state = &antiPoisonStreamTextState{Key: fmt.Sprintf("%d", index)}
					textStates[index] = state
				}
				text := firstNonEmptyExact(toStringValue(block["text"]), toStringValue(block["thinking"]))
				if text != "" {
					state.Text += text
				}
			}
		case "content_block_delta":
			delta, _ := data["delta"].(map[string]any)
			deltaType := strings.TrimSpace(toStringValue(delta["type"]))
			if deltaType == "input_json_delta" {
				state := states[index]
				if state == nil {
					state = &antiPoisonStreamToolState{Key: fmt.Sprintf("%d", index), Order: order, Index: index}
					order++
					states[index] = state
				}
				if partial := toStringValue(delta["partial_json"]); partial != "" {
					state.Arguments = accumulateAdvancedProxyToolArguments(state.Arguments, partial)
				}
				continue
			}
			if deltaType == "text_delta" {
				state := textStates[index]
				if state == nil {
					state = &antiPoisonStreamTextState{Key: fmt.Sprintf("%d", index)}
					textStates[index] = state
				}
				text := firstNonEmptyExact(toStringValue(delta["text"]), toStringValue(data["text"]))
				if text != "" {
					state.Text += text
				}
			}
		}
	}
	stateMap := map[string]*antiPoisonStreamToolState{}
	for index, state := range states {
		if state == nil {
			continue
		}
		stateMap[fmt.Sprintf("%d", index)] = state
	}
	textStateMap := map[string]*antiPoisonStreamTextState{}
	for index, state := range textStates {
		if state == nil {
			continue
		}
		textStateMap[fmt.Sprintf("%d", index)] = state
	}
	calls := antiPoisonStreamStatesToCalls(stateMap, "anthropic.tool_use")
	guardCalls, guardCount := antiPoisonStreamGuardCallsFromTextStates(textStateMap, ctx)
	if guardCount == 0 {
		guardCalls, guardCount = antiPoisonStreamGuardCallsFromRawEvents(events, ctx)
	}
	allCalls := append(calls, guardCalls...)
	result := validateAndStripAntiPoisonToolCalls([]byte("{}"), allCalls, ctx, func() []byte {
		return []byte("{}")
	}, guardCount)
	result.Body = raw
	result.RemovedGuards = guardCount
	if result.Blocked {
		return raw, result, nil
	}
	if guardCount == 0 {
		return raw, result, nil
	}
	sanitized := stripAntiPoisonAnthropicStreamGuardEvents(events, result.RealCount == 0, ctx)
	result.Body = sanitized
	return sanitized, result, nil
}

func stripAntiPoisonAnthropicStreamGuardEvents(events []advancedProxySSEEvent, guardOnly bool, ctx antiPoisonRequestContext) []byte {
	next := make([]advancedProxySSEEvent, 0, len(events))
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
			next = append(next, event)
			continue
		}
		eventType := firstNonEmpty(strings.TrimSpace(event.Event), strings.TrimSpace(toStringValue(data["type"])))
		switch eventType {
		case "content_block_start":
			block, _ := data["content_block"].(map[string]any)
			blockType := strings.TrimSpace(toStringValue(block["type"]))
			if blockType == "text" || blockType == "thinking" || blockType == "redacted_thinking" {
				if _, exists := block["text"]; exists {
					block["text"] = extractAntiPoisonGuardsFromText(toStringValue(block["text"]), ctx).Text
				}
				if _, exists := block["thinking"]; exists {
					block["thinking"] = extractAntiPoisonGuardsFromText(toStringValue(block["thinking"]), ctx).Text
				}
			}
		case "content_block_delta":
			if delta, _ := data["delta"].(map[string]any); delta != nil && strings.TrimSpace(toStringValue(delta["type"])) == "text_delta" {
				if _, exists := delta["text"]; exists {
					delta["text"] = extractAntiPoisonGuardsFromText(toStringValue(delta["text"]), ctx).Text
				}
			}
			if _, exists := data["text"]; exists {
				data["text"] = extractAntiPoisonGuardsFromText(toStringValue(data["text"]), ctx).Text
			}
		case "message_delta":
			if guardOnly {
				if delta, _ := data["delta"].(map[string]any); delta != nil && strings.TrimSpace(toStringValue(delta["stop_reason"])) == "tool_use" {
					delta["stop_reason"] = "end_turn"
				}
			}
		}
		setAdvancedProxySSEJSONPayload(&event, data)
		stripAntiPoisonGuardJSONFromSSEEvent(&event, ctx)
		next = append(next, event)
	}
	return encodeAdvancedProxySSEEvents(next)
}

func readAndPrepareAntiPoisonOpenAIStream(streamBody io.Reader, recordContext *advancedProxyStreamRequestRecordContext) ([]byte, antiPoisonValidationResult, error) {
	raw, err := io.ReadAll(streamBody)
	if err != nil {
		return raw, antiPoisonValidationResult{Body: raw}, err
	}
	if recordContext == nil {
		return raw, antiPoisonValidationResult{Body: raw}, nil
	}
	recordContext.UpstreamResponseRaw = string(raw)
	recordContext.UpstreamResponsePreview = summarizeAdvancedProxyRawStreamPreview(raw)
	recordContext.UpstreamToolCalls, recordContext.UpstreamToolArgsPreview, recordContext.UpstreamAssistantPreview, recordContext.UpstreamLatestObserved = summarizeAdvancedProxyRawStreamFeedbackContext(raw, recordContext.ObservedFormat)
	route := firstNonEmpty(
		normalizeAdvancedProxyObservedFormat(recordContext.ObservedFormat),
		normalizeAdvancedProxyObservedFormat(recordContext.OutboundRoute),
		normalizeAdvancedProxyObservedFormat(recordContext.ClientRoute),
		normalizeAdvancedProxyObservedFormat(recordContext.AntiPoisonCtx.RouteKind),
		recordContext.OutboundRoute,
		recordContext.ClientRoute,
		recordContext.AntiPoisonCtx.RouteKind,
	)
	provider := advancedProxyProviderLabel(recordContext.Provider)
	sanitized := raw
	result := antiPoisonValidationResult{Body: raw}
	if recordContext.AntiPoisonCtx.Enabled {
		var sanitizeErr error
		sanitized, result, sanitizeErr = sanitizeAntiPoisonOpenAIStreamBody(raw, recordContext.ObservedFormat, route, recordContext.AntiPoisonCtx)
		recordContext.AntiPoisonOps = appendAntiPoisonStreamValidationOps(recordContext.AntiPoisonOps, result, route, provider, "openai")
		appendAdvancedProxyLogf(
			"[ANTI_POISON_STREAM_VALIDATE] channel=openai route=%s provider=%s alias=%s valid=%t blocked=%t reason=%s real=%d guard=%d stripped=%d",
			previewAdvancedProxyText(route, 80),
			previewAdvancedProxyText(provider, 120),
			previewAdvancedProxyText(recordContext.AntiPoisonCtx.Alias, 40),
			result.Valid,
			result.Blocked,
			previewAdvancedProxyText(result.Reason, 120),
			result.RealCount,
			result.GuardCount,
			result.RemovedGuards,
		)
		if sanitizeErr != nil {
			return raw, result, sanitizeErr
		}
		if result.Blocked {
			return raw, result, nil
		}
	}
	sanitized = restoreAntiPoisonStringProtectionInSSEBody(sanitized, &recordContext.StringProtect, route, provider, "openai")
	recordContext.AntiPoisonOps = append(recordContext.AntiPoisonOps, recordContext.StringProtect.Records...)
	result.Body = sanitized
	recordContext.DeliveredResponsePreview = summarizeAdvancedProxyRawStreamPreview(sanitized)
	return sanitized, result, nil
}

func readAndPrepareAntiPoisonAnthropicStream(streamBody io.Reader, recordContext *advancedProxyStreamRequestRecordContext) ([]byte, antiPoisonValidationResult, error) {
	raw, err := io.ReadAll(streamBody)
	if err != nil {
		return raw, antiPoisonValidationResult{Body: raw}, err
	}
	if recordContext == nil {
		return raw, antiPoisonValidationResult{Body: raw}, nil
	}
	recordContext.UpstreamResponseRaw = string(raw)
	recordContext.UpstreamResponsePreview = summarizeAdvancedProxyRawStreamPreview(raw)
	recordContext.UpstreamToolCalls, recordContext.UpstreamToolArgsPreview, recordContext.UpstreamAssistantPreview, recordContext.UpstreamLatestObserved = summarizeAdvancedProxyRawStreamFeedbackContext(raw, recordContext.ObservedFormat)
	route := firstNonEmpty(recordContext.ClientRoute, recordContext.OutboundRoute, recordContext.AntiPoisonCtx.RouteKind)
	provider := advancedProxyProviderLabel(recordContext.Provider)
	sanitized := raw
	result := antiPoisonValidationResult{Body: raw}
	if recordContext.AntiPoisonCtx.Enabled {
		var sanitizeErr error
		sanitized, result, sanitizeErr = sanitizeAntiPoisonAnthropicStreamBody(raw, recordContext.AntiPoisonCtx)
		recordContext.AntiPoisonOps = appendAntiPoisonStreamValidationOps(recordContext.AntiPoisonOps, result, route, provider, "claude")
		appendAdvancedProxyLogf(
			"[ANTI_POISON_STREAM_VALIDATE] channel=claude route=%s provider=%s alias=%s valid=%t blocked=%t reason=%s real=%d guard=%d stripped=%d",
			previewAdvancedProxyText(route, 80),
			previewAdvancedProxyText(provider, 120),
			previewAdvancedProxyText(recordContext.AntiPoisonCtx.Alias, 40),
			result.Valid,
			result.Blocked,
			previewAdvancedProxyText(result.Reason, 120),
			result.RealCount,
			result.GuardCount,
			result.RemovedGuards,
		)
		if sanitizeErr != nil {
			return raw, result, sanitizeErr
		}
		if result.Blocked {
			return raw, result, nil
		}
	}
	sanitized = restoreAntiPoisonStringProtectionInSSEBody(sanitized, &recordContext.StringProtect, route, provider, "claude")
	recordContext.AntiPoisonOps = append(recordContext.AntiPoisonOps, recordContext.StringProtect.Records...)
	result.Body = sanitized
	recordContext.DeliveredResponsePreview = summarizeAdvancedProxyRawStreamPreview(sanitized)
	return sanitized, result, nil
}
