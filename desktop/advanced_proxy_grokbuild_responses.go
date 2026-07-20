package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

var grokBuildResponsesSupportedEventTypes = map[string]struct{}{
	"response.created":                             {},
	"response.in_progress":                         {},
	"response.completed":                           {},
	"response.failed":                              {},
	"response.incomplete":                          {},
	"response.queued":                              {},
	"response.output_item.added":                   {},
	"response.output_item.done":                    {},
	"response.content_part.added":                  {},
	"response.content_part.done":                   {},
	"response.output_text.delta":                   {},
	"response.output_text.done":                    {},
	"response.output_text.annotation.added":        {},
	"response.refusal.delta":                       {},
	"response.refusal.done":                        {},
	"response.function_call_arguments.delta":       {},
	"response.function_call_arguments.done":        {},
	"response.file_search_call.in_progress":        {},
	"response.file_search_call.searching":          {},
	"response.file_search_call.completed":          {},
	"response.web_search_call.in_progress":         {},
	"response.web_search_call.searching":           {},
	"response.web_search_call.completed":           {},
	"response.reasoning_summary_part.added":        {},
	"response.reasoning_summary_part.done":         {},
	"response.reasoning_summary_text.delta":        {},
	"response.reasoning_summary_text.done":         {},
	"response.reasoning_text.delta":                {},
	"response.reasoning_text.done":                 {},
	"response.image_generation_call.completed":     {},
	"response.image_generation_call.generating":    {},
	"response.image_generation_call.in_progress":   {},
	"response.image_generation_call.partial_image": {},
	"response.mcp_call_arguments.delta":            {},
	"response.mcp_call_arguments.done":             {},
	"response.mcp_call.completed":                  {},
	"response.mcp_call.failed":                     {},
	"response.mcp_call.in_progress":                {},
	"response.mcp_list_tools.completed":            {},
	"response.mcp_list_tools.failed":               {},
	"response.mcp_list_tools.in_progress":          {},
	"response.code_interpreter_call.in_progress":   {},
	"response.code_interpreter_call.interpreting":  {},
	"response.code_interpreter_call.completed":     {},
	"response.code_interpreter_call_code.delta":    {},
	"response.code_interpreter_call_code.done":     {},
	"response.custom_tool_call_input.delta":        {},
	"response.custom_tool_call_input.done":         {},
	"error":                                        {},
}

func shouldPruneGrokBuildResponsesStream(recordContext *advancedProxyStreamRequestRecordContext) bool {
	if recordContext == nil || !strings.EqualFold(strings.TrimSpace(recordContext.AppType), "grokbuild") {
		return false
	}
	observedFormat := firstNonEmpty(recordContext.ObservedFormat, recordContext.ClientRoute, recordContext.OutboundRoute)
	return normalizeAdvancedProxyObservedFormat(observedFormat) == "responses"
}

func isSupportedGrokBuildResponsesEvent(eventType string) bool {
	_, ok := grokBuildResponsesSupportedEventTypes[strings.TrimSpace(eventType)]
	return ok
}

func pruneGrokBuildResponsesSSEEvent(event advancedProxySSEEvent, sequenceNumber int) (advancedProxySSEEvent, bool) {
	payload := strings.TrimSpace(advancedProxySSEEventPayload(event))
	if payload == "[DONE]" {
		return event, true
	}

	data, isJSON := advancedProxySSEJSONPayload(event)
	if !isJSON {
		return advancedProxySSEEvent{}, false
	}

	eventType := strings.TrimSpace(event.Event)
	if payloadType, ok := data["type"].(string); ok && strings.TrimSpace(payloadType) != "" {
		eventType = strings.TrimSpace(payloadType)
	}
	if !isSupportedGrokBuildResponsesEvent(eventType) {
		return advancedProxySSEEvent{}, false
	}

	// Ensure the SSE header and JSON discriminator describe the same accepted event.
	event.Event = eventType
	data["type"] = eventType
	normalizeGrokBuildResponsesSSEPayload(data, eventType, sequenceNumber)
	setAdvancedProxySSEJSONPayload(&event, data)
	return event, true
}

func normalizeGrokBuildResponsesSSEPayload(data map[string]any, eventType string, sequenceNumber int) {
	if data == nil {
		return
	}
	if _, ok := data["sequence_number"]; !ok {
		data["sequence_number"] = sequenceNumber
	}

	switch eventType {
	case "response.created", "response.in_progress", "response.completed", "response.failed", "response.incomplete", "response.queued":
		if response, ok := data["response"].(map[string]any); ok {
			normalizeGrokBuildResponsesObject(response, sequenceNumber, grokBuildResponsesResponseStatus(eventType))
		}
	case "response.output_item.added", "response.output_item.done":
		ensureGrokBuildResponsesIndex(data, "output_index")
		if item, ok := data["item"].(map[string]any); ok {
			normalizeGrokBuildResponsesItem(item, sequenceNumber, advancedProxyNumberAsInt(data["output_index"]), grokBuildResponsesItemStatus(eventType))
		}
	case "response.content_part.added", "response.content_part.done":
		ensureGrokBuildResponsesIndex(data, "output_index")
		ensureGrokBuildResponsesIndex(data, "content_index")
		if _, ok := data["item_id"]; !ok {
			data["item_id"] = grokBuildResponsesFallbackItemID(data)
		}
		if part, ok := data["part"].(map[string]any); ok {
			normalizeGrokBuildResponsesContentPart(part)
		}
	case "response.output_text.delta", "response.output_text.done", "response.output_text.annotation.added":
		ensureGrokBuildResponsesIndex(data, "output_index")
		ensureGrokBuildResponsesIndex(data, "content_index")
		if _, ok := data["item_id"]; !ok {
			data["item_id"] = grokBuildResponsesFallbackItemID(data)
		}
		if eventType == "response.output_text.delta" {
			ensureGrokBuildResponsesString(data, "delta")
		}
		if eventType == "response.output_text.done" {
			ensureGrokBuildResponsesString(data, "text")
		}
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		ensureGrokBuildResponsesIndex(data, "output_index")
		if _, ok := data["item_id"]; !ok {
			data["item_id"] = grokBuildResponsesFallbackFunctionCallID(data)
		}
		if eventType == "response.function_call_arguments.delta" {
			ensureGrokBuildResponsesString(data, "delta")
		}
		if eventType == "response.function_call_arguments.done" {
			ensureGrokBuildResponsesString(data, "arguments")
			stringifyGrokBuildResponsesJSONField(data, "arguments")
		}
	}
	syntheticCounter := 0
	normalizeGrokBuildResponsesNestedValue(data, eventType, sequenceNumber, &syntheticCounter, true)
}

func normalizeGrokBuildResponsesObject(response map[string]any, sequenceNumber int, defaultStatus string) {
	if response == nil {
		return
	}
	if _, ok := response["id"]; !ok {
		response["id"] = fmt.Sprintf("resp_grokbuild_%d", sequenceNumber)
	}
	if _, ok := response["status"]; !ok && defaultStatus != "" {
		response["status"] = defaultStatus
	}
	itemStatus := grokBuildResponsesOutputStatusFromResponse(response["status"])
	if output, ok := response["output"].([]any); ok {
		for index, item := range output {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			normalizeGrokBuildResponsesItem(itemMap, sequenceNumber, index, itemStatus)
		}
	}
}

func normalizeGrokBuildResponsesItem(item map[string]any, sequenceNumber int, index int, defaultStatus string) {
	if item == nil {
		return
	}
	itemType, _ := item["type"].(string)
	if _, ok := item["id"]; !ok {
		prefix := "item"
		if itemType == "message" {
			prefix = "msg"
		} else if itemType == "function_call" {
			prefix = "fc"
		}
		item["id"] = fmt.Sprintf("%s_grokbuild_%d_%d", prefix, sequenceNumber, index)
	}
	if _, ok := item["status"]; !ok && defaultStatus != "" {
		item["status"] = defaultStatus
	}
	switch itemType {
	case "message":
		if _, ok := item["role"]; !ok {
			item["role"] = "assistant"
		}
		if _, ok := item["content"]; !ok {
			item["content"] = []any{}
		}
	case "function_call":
		if _, ok := item["call_id"]; !ok {
			item["call_id"] = fmt.Sprintf("call_grokbuild_%d_%d", sequenceNumber, index)
		}
		if _, ok := item["name"]; !ok {
			item["name"] = "unknown_tool"
		}
		normalizeGrokBuildResponsesArgumentsField(item)
	case "reasoning":
		if _, ok := item["summary"]; !ok {
			item["summary"] = []any{}
		}
	}
	if content, ok := item["content"].([]any); ok {
		for _, part := range content {
			partMap, ok := part.(map[string]any)
			if !ok {
				continue
			}
			normalizeGrokBuildResponsesContentPart(partMap)
		}
	}
}

func normalizeGrokBuildResponsesNestedValue(value any, eventType string, sequenceNumber int, syntheticCounter *int, isEventRoot bool) {
	switch typed := value.(type) {
	case map[string]any:
		normalizeGrokBuildResponsesNestedMap(typed, eventType, sequenceNumber, syntheticCounter, isEventRoot)
	case []any:
		for _, item := range typed {
			normalizeGrokBuildResponsesNestedValue(item, eventType, sequenceNumber, syntheticCounter, false)
		}
	}
}

func normalizeGrokBuildResponsesNestedMap(data map[string]any, eventType string, sequenceNumber int, syntheticCounter *int, isEventRoot bool) {
	if data == nil {
		return
	}
	itemType := strings.TrimSpace(toStringValue(data["type"]))
	switch itemType {
	case "message":
		ensureGrokBuildResponsesNonEmptyString(data, "id", nextGrokBuildResponsesSyntheticID("msg", sequenceNumber, syntheticCounter))
		ensureGrokBuildResponsesNonEmptyString(data, "status", grokBuildResponsesDefaultNestedStatus(eventType))
		if _, ok := data["role"]; !ok {
			data["role"] = "assistant"
		}
		if _, ok := data["content"]; !ok {
			data["content"] = []any{}
		}
	case "function_call":
		normalizeGrokBuildResponsesFunctionCallMap(data, eventType, sequenceNumber, syntheticCounter, false)
	case "reasoning":
		ensureGrokBuildResponsesNonEmptyString(data, "id", nextGrokBuildResponsesSyntheticID("rs", sequenceNumber, syntheticCounter))
		ensureGrokBuildResponsesNonEmptyString(data, "status", grokBuildResponsesDefaultNestedStatus(eventType))
		if _, ok := data["summary"]; !ok {
			data["summary"] = []any{}
		}
	case "output_text":
		normalizeGrokBuildResponsesContentPart(data)
	}
	if !isEventRoot && itemType == "" && looksLikeGrokBuildResponsesFunctionCallMap(data) {
		normalizeGrokBuildResponsesFunctionCallMap(data, eventType, sequenceNumber, syntheticCounter, true)
	}
	for _, key := range orderedJSONMapKeys(data) {
		normalizeGrokBuildResponsesNestedValue(data[key], eventType, sequenceNumber, syntheticCounter, false)
	}
}

func looksLikeGrokBuildResponsesFunctionCallMap(data map[string]any) bool {
	if data == nil {
		return false
	}
	if strings.TrimSpace(toStringValue(data["call_id"])) != "" {
		return true
	}
	if strings.TrimSpace(toStringValue(data["name"])) == "" {
		return false
	}
	_, hasArguments := data["arguments"]
	_, hasInput := data["input"]
	_, hasParameters := data["parameters"]
	return hasArguments || hasInput || hasParameters
}

func normalizeGrokBuildResponsesFunctionCallMap(data map[string]any, eventType string, sequenceNumber int, syntheticCounter *int, setMissingType bool) {
	if data == nil {
		return
	}
	if setMissingType {
		data["type"] = "function_call"
	}
	ensureGrokBuildResponsesNonEmptyString(data, "id", nextGrokBuildResponsesSyntheticID("fc", sequenceNumber, syntheticCounter))
	ensureGrokBuildResponsesNonEmptyString(data, "call_id", nextGrokBuildResponsesSyntheticID("call", sequenceNumber, syntheticCounter))
	ensureGrokBuildResponsesNonEmptyString(data, "name", "unknown_tool")
	ensureGrokBuildResponsesNonEmptyString(data, "status", grokBuildResponsesDefaultNestedStatus(eventType))
	normalizeGrokBuildResponsesArgumentsField(data)
}

func normalizeGrokBuildResponsesArgumentsField(data map[string]any) {
	if data == nil {
		return
	}
	if _, ok := data["arguments"]; !ok {
		if _, hasInput := data["input"]; hasInput {
			data["arguments"] = data["input"]
		} else if _, hasParameters := data["parameters"]; hasParameters {
			data["arguments"] = data["parameters"]
		} else {
			data["arguments"] = ""
		}
	}
	if data["arguments"] == nil {
		data["arguments"] = ""
		return
	}
	stringifyGrokBuildResponsesJSONField(data, "arguments")
}

func ensureGrokBuildResponsesNonEmptyString(data map[string]any, key string, fallback string) {
	if data == nil {
		return
	}
	if strings.TrimSpace(toStringValue(data[key])) != "" {
		return
	}
	data[key] = fallback
}

func nextGrokBuildResponsesSyntheticID(prefix string, sequenceNumber int, syntheticCounter *int) string {
	if syntheticCounter == nil {
		return fmt.Sprintf("%s_grokbuild_%d", prefix, sequenceNumber)
	}
	*syntheticCounter = *syntheticCounter + 1
	return fmt.Sprintf("%s_grokbuild_%d_%d", prefix, sequenceNumber, *syntheticCounter)
}

func grokBuildResponsesDefaultNestedStatus(eventType string) string {
	switch eventType {
	case "response.completed", "response.output_item.done", "response.content_part.done":
		return "completed"
	case "response.failed":
		return "failed"
	case "response.incomplete":
		return "incomplete"
	case "response.queued":
		return "queued"
	default:
		return "in_progress"
	}
}

func grokBuildResponsesResponseStatus(eventType string) string {
	switch eventType {
	case "response.completed":
		return "completed"
	case "response.failed":
		return "failed"
	case "response.incomplete":
		return "incomplete"
	case "response.queued":
		return "queued"
	default:
		return "in_progress"
	}
}

func grokBuildResponsesItemStatus(eventType string) string {
	if eventType == "response.output_item.done" {
		return "completed"
	}
	return "in_progress"
}

func grokBuildResponsesOutputStatusFromResponse(status any) string {
	switch strings.TrimSpace(toStringValue(status)) {
	case "completed":
		return "completed"
	case "failed", "incomplete":
		return "incomplete"
	default:
		return "in_progress"
	}
}

func ensureGrokBuildResponsesIndex(data map[string]any, key string) {
	if data == nil {
		return
	}
	if _, ok := data[key]; !ok {
		data[key] = 0
	}
}

func ensureGrokBuildResponsesString(data map[string]any, key string) {
	if data == nil {
		return
	}
	if _, ok := data[key]; !ok {
		data[key] = ""
	}
}

func stringifyGrokBuildResponsesJSONField(data map[string]any, key string) {
	if data == nil {
		return
	}
	if _, ok := data[key].(string); ok {
		return
	}
	raw, err := json.Marshal(data[key])
	if err != nil {
		data[key] = ""
		return
	}
	data[key] = string(raw)
}

func normalizeGrokBuildResponsesContentPart(part map[string]any) {
	if part == nil {
		return
	}
	if partType, _ := part["type"].(string); partType != "output_text" {
		return
	}
	ensureGrokBuildResponsesString(part, "text")
	if _, ok := part["annotations"]; !ok {
		part["annotations"] = []any{}
	}
	if _, ok := part["logprobs"]; !ok {
		part["logprobs"] = []any{}
	}
}

func grokBuildResponsesFallbackItemID(data map[string]any) string {
	return fmt.Sprintf("msg_grokbuild_%d", advancedProxyNumberAsInt(data["output_index"]))
}

func grokBuildResponsesFallbackFunctionCallID(data map[string]any) string {
	return fmt.Sprintf("fc_grokbuild_%d", advancedProxyNumberAsInt(data["output_index"]))
}

func advancedProxyNumberAsInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func pruneGrokBuildResponsesSSEBody(raw []byte) ([]byte, int) {
	events, err := parseAdvancedProxySSEEvents(raw)
	if err != nil || len(events) == 0 {
		return raw, 0
	}

	filtered := make([]advancedProxySSEEvent, 0, len(events))
	dropped := 0
	sequenceNumber := 0
	for _, event := range events {
		sequenceNumber++
		pruned, keep := pruneGrokBuildResponsesSSEEvent(event, sequenceNumber)
		if !keep {
			dropped++
			continue
		}
		filtered = append(filtered, pruned)
	}
	return encodeAdvancedProxySSEEvents(filtered), dropped
}
