package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const antiCandyReasoningStep = 518

var antiCandyTerminalTypes = map[string]bool{
	"response.completed":  true,
	"response.failed":     true,
	"response.incomplete": true,
}

type antiCandySSEEvent struct {
	Raw       []byte
	EventName string
	Type      string
	Payload   map[string]any
	Done      bool
}

type antiCandyRound struct {
	Events                    []antiCandySSEEvent
	Terminal                  map[string]any
	TerminalType              string
	HasTerminal               bool
	BaseResponse              map[string]any
	Usage                     map[string]any
	ReasoningItems            []map[string]any
	NonReasoningItems         []map[string]any
	ReasoningIndexBySource    map[int]int
	NonReasoningIndexBySource map[int]int
}

type antiCandyFoldStats struct {
	Folded        bool
	Rounds        int
	Continuations int
	StoppedReason string
}

type antiCandyContinuationFunc func(rawBody []byte) (status int, headers http.Header, nextBody []byte, err error)

func antiCandyModelAllowed(config AntiCandyConfig, model string) bool {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if normalizedModel == "" {
		return false
	}
	for _, allowed := range config.Models {
		candidate := strings.ToLower(strings.TrimSpace(allowed))
		if candidate == "" {
			continue
		}
		if candidate == "*" || candidate == normalizedModel {
			return true
		}
		if strings.HasSuffix(candidate, "*") && strings.HasPrefix(normalizedModel, strings.TrimSuffix(candidate, "*")) {
			return true
		}
	}
	return false
}

func shouldApplyAntiCandyToOpenAIRequest(appType string, routeKind string, stream bool, rawBody []byte, model string, config AntiCandyConfig) bool {
	apply, _ := explainAntiCandyOpenAIRequest(appType, routeKind, stream, rawBody, model, config)
	return apply
}

func explainAntiCandyOpenAIRequest(appType string, routeKind string, stream bool, rawBody []byte, model string, config AntiCandyConfig) (bool, string) {
	config = sanitizeAntiCandyConfig(config)
	if !config.Enabled {
		return false, "disabled"
	}
	if !stream {
		return false, "not_streaming"
	}
	if !strings.EqualFold(strings.TrimSpace(appType), "codex") {
		return false, "not_codex"
	}
	if strings.TrimSpace(routeKind) != "responses" {
		return false, "not_responses"
	}
	if !antiCandyModelAllowed(config, model) {
		return false, "model_not_allowed"
	}
	body := map[string]any{}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return false, "invalid_json"
	}
	if _, ok := body["previous_response_id"]; ok && strings.TrimSpace(toStringValue(body["previous_response_id"])) != "" {
		return false, "continuation_request"
	}
	_, ok := body["input"].([]any)
	if !ok {
		return false, "input_not_array"
	}
	return true, "eligible"
}

func foldOpenAIResponsesStreamWithAntiCandy(
	streamBody io.ReadCloser,
	appType string,
	provider AdvancedProxyProvider,
	targetURL string,
	requestHeaders map[string]string,
	baseBody []byte,
	model string,
	timeoutSeconds int,
	config AntiCandyConfig,
) (io.ReadCloser, antiCandyFoldStats, error) {
	defer streamBody.Close()
	foldStartedAt := time.Now()
	appendAdvancedProxyLogf(
		"[ANTI_CANDY_STEP] run=%s app=%s stage=round_wait round=1 detail=waiting_upstream_terminal",
		advancedProxyCandyEvalRunID(provider),
		appType,
	)
	rawInitial, err := io.ReadAll(io.LimitReader(streamBody, 16*1024*1024))
	if err != nil {
		return nil, antiCandyFoldStats{}, err
	}
	appendAdvancedProxyLogf(
		"[ANTI_CANDY_STEP] run=%s app=%s stage=round_received round=1 bytes=%d elapsed_ms=%d",
		advancedProxyCandyEvalRunID(provider),
		appType,
		len(rawInitial),
		time.Since(foldStartedAt).Milliseconds(),
	)
	if !shouldApplyAntiCandyToOpenAIRequest(appType, "responses", true, baseBody, model, config) {
		return io.NopCloser(bytes.NewReader(rawInitial)), antiCandyFoldStats{}, nil
	}

	continuation := func(rawBody []byte) (int, http.Header, []byte, error) {
		continuationStartedAt := time.Now()
		appendAdvancedProxyLogf(
			"[ANTI_CANDY_STEP] run=%s app=%s stage=continuation_request round=next detail=waiting_upstream_terminal",
			advancedProxyCandyEvalRunID(provider),
			appType,
		)
		status, headers, body, nextStream, _, requestErr := performRawUpstreamRequest(
			http.MethodPost,
			targetURL,
			requestHeaders,
			rawBody,
			timeoutSeconds,
			true,
		)
		if requestErr != nil {
			return status, headers, body, requestErr
		}
		if nextStream != nil {
			defer nextStream.Close()
			body, requestErr = io.ReadAll(io.LimitReader(nextStream, 16*1024*1024))
		}
		appendAdvancedProxyLogf(
			"[ANTI_CANDY_STEP] run=%s app=%s stage=continuation_received round=next status=%d bytes=%d elapsed_ms=%d error=%t",
			advancedProxyCandyEvalRunID(provider),
			appType,
			status,
			len(body),
			time.Since(continuationStartedAt).Milliseconds(),
			requestErr != nil,
		)
		return status, headers, body, requestErr
	}

	folded, stats, foldErr := foldAntiCandyResponsesStreamBytes(rawInitial, baseBody, config, continuation)
	if foldErr != nil {
		return nil, stats, foldErr
	}
	if stats.Folded {
		appendAdvancedProxyLogf(
			"[ANTI_CANDY_FOLD] run=%s app=%s provider=%s model=%s rounds=%d continuations=%d stopped=%s",
			advancedProxyCandyEvalRunID(provider),
			appType,
			advancedProxyProviderLabel(provider),
			previewAdvancedProxyText(model, 120),
			stats.Rounds,
			stats.Continuations,
			firstNonEmpty(stats.StoppedReason, "completed"),
		)
	} else {
		appendAdvancedProxyLogf(
			"[ANTI_CANDY_RESULT] run=%s app=%s provider=%s model=%s folded=false stopped=%s",
			advancedProxyCandyEvalRunID(provider),
			appType,
			advancedProxyProviderLabel(provider),
			previewAdvancedProxyText(model, 120),
			firstNonEmpty(stats.StoppedReason, "not_triggered"),
		)
	}
	return io.NopCloser(bytes.NewReader(folded)), stats, nil
}

func foldAntiCandyResponsesStreamBytes(initialRaw []byte, baseBody []byte, config AntiCandyConfig, continuation antiCandyContinuationFunc) ([]byte, antiCandyFoldStats, error) {
	config = sanitizeAntiCandyConfig(config)
	baseRequest := map[string]any{}
	if err := json.Unmarshal(baseBody, &baseRequest); err != nil {
		return initialRaw, antiCandyFoldStats{}, nil
	}
	baseInput, ok := baseRequest["input"].([]any)
	if !ok {
		return initialRaw, antiCandyFoldStats{}, nil
	}

	currentRaw := initialRaw
	continuationInput := antiCandyCloneSlice(baseInput)
	rounds := make([]*antiCandyRound, 0, config.MaxContinue+1)
	roundMetadata := make([]map[string]any, 0, config.MaxContinue+1)
	billedUsage := map[string]any{}
	stats := antiCandyFoldStats{}
	var baseResponse map[string]any

	for {
		round, parsed := parseAntiCandyRound(currentRaw)
		if !parsed || !round.HasTerminal {
			if !stats.Folded {
				return initialRaw, stats, nil
			}
			stats.StoppedReason = firstNonEmpty(stats.StoppedReason, "invalid_round")
			return buildAntiCandyFoldedStream(rounds, rounds[len(rounds)-1], baseResponse, billedUsage, roundMetadata, stats.StoppedReason), stats, nil
		}
		if baseResponse == nil {
			baseResponse = antiCandyCloneMap(round.BaseResponse)
			if baseResponse == nil {
				if response, ok := round.Terminal["response"].(map[string]any); ok {
					baseResponse = antiCandyCloneMap(response)
				}
			}
		}
		rounds = append(rounds, round)
		stats.Rounds = len(rounds)
		antiCandySumUsage(billedUsage, round.Usage)

		tierN := antiCandyTierN(round.Usage)
		// The reference implementation does not accept a continuation that
		// immediately returns zero reasoning tokens. Retry it (within the same
		// continuation budget), otherwise a transient empty continuation can
		// defeat the very fold we just detected.
		zeroReasoningRetry := stats.Folded && tierN == 0 && len(round.ReasoningItems) == 0 && round.HasTerminal && round.TerminalType != "response.failed"
		canContinue := zeroReasoningRetry || antiCandyRoundCanContinue(round, tierN, config)
		metadata := map[string]any{
			"round":                len(rounds),
			"input_tokens":         antiCandyNumber(round.Usage["input_tokens"]),
			"output_tokens":        antiCandyNumber(round.Usage["output_tokens"]),
			"total_tokens":         antiCandyNumber(round.Usage["total_tokens"]),
			"reasoning_tokens":     antiCandyNumberFromDetails(round.Usage, "output_tokens_details", "reasoning_tokens"),
			"tier_n":               tierN,
			"continued":            false,
			"zero_reasoning_retry": zeroReasoningRetry,
		}
		if cached := antiCandyNumberFromDetails(round.Usage, "input_tokens_details", "cached_tokens"); cached > 0 {
			metadata["cached_tokens"] = cached
		}

		if !canContinue {
			roundMetadata = append(roundMetadata, metadata)
			if !stats.Folded {
				return initialRaw, stats, nil
			}
			return buildAntiCandyFoldedStream(rounds, round, baseResponse, billedUsage, roundMetadata, stats.StoppedReason), stats, nil
		}

		stats.Folded = true
		if stats.Continuations >= config.MaxContinue {
			stats.StoppedReason = "max_continue"
			roundMetadata = append(roundMetadata, metadata)
			return buildAntiCandyFoldedStream(rounds, round, baseResponse, billedUsage, roundMetadata, stats.StoppedReason), stats, nil
		}
		metadata["continued"] = true
		roundMetadata = append(roundMetadata, metadata)

		for _, item := range round.ReasoningItems {
			continuationInput = append(continuationInput, antiCandyCloneMap(item))
		}
		continuationInput = append(continuationInput, antiCandyCommentaryNudge(config.MarkerText))
		nextBody, bodyOK := buildAntiCandyContinuationBody(baseRequest, continuationInput)
		if !bodyOK {
			stats.StoppedReason = "invalid_continuation"
			return buildAntiCandyFoldedStream(rounds, round, baseResponse, billedUsage, roundMetadata, stats.StoppedReason), stats, nil
		}
		stats.Continuations++
		status, _, nextRaw, continuationErr := continuation(nextBody)
		if continuationErr != nil || status < 200 || status >= 300 || len(nextRaw) == 0 {
			stats.StoppedReason = "continuation_failed"
			return buildAntiCandyFoldedStream(rounds, round, baseResponse, billedUsage, roundMetadata, stats.StoppedReason), stats, nil
		}
		currentRaw = nextRaw
	}
}

func antiCandyRoundCanContinue(round *antiCandyRound, tierN int, config AntiCandyConfig) bool {
	if round == nil || !round.HasTerminal || round.TerminalType == "response.failed" {
		return false
	}
	if tierN < 1 || (config.MaxTierN > 0 && tierN > config.MaxTierN) || !antiCandyRoundHasEncryptedReasoning(round) {
		return false
	}
	return true
}

func antiCandyRoundHasEncryptedReasoning(round *antiCandyRound) bool {
	if round == nil {
		return false
	}
	for _, item := range round.ReasoningItems {
		if encrypted, ok := item["encrypted_content"].(string); ok && strings.TrimSpace(encrypted) != "" {
			return true
		}
	}
	return false
}

func buildAntiCandyContinuationBody(base map[string]any, input []any) ([]byte, bool) {
	if base == nil {
		return nil, false
	}
	body := antiCandyCloneMap(base)
	body["stream"] = true
	body["input"] = antiCandyCloneSlice(input)
	body["include"] = antiCandyIncludeEncryptedContent(body["include"])
	delete(body, "previous_response_id")
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func antiCandyIncludeEncryptedContent(value any) []any {
	include := make([]any, 0, 2)
	switch values := value.(type) {
	case []any:
		for _, item := range values {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				include = append(include, text)
			}
		}
	case []string:
		for _, item := range values {
			if strings.TrimSpace(item) != "" {
				include = append(include, item)
			}
		}
	}
	for _, item := range include {
		if item == "reasoning.encrypted_content" {
			return include
		}
	}
	return append(include, "reasoning.encrypted_content")
}

func antiCandyCommentaryNudge(markerText string) map[string]any {
	markerText = strings.TrimSpace(markerText)
	if markerText == "" {
		markerText = "Continue thinking..."
	}
	return map[string]any{
		"type":  "message",
		"role":  "assistant",
		"phase": "commentary",
		"content": []any{map[string]any{
			"type": "output_text",
			"text": markerText,
		}},
	}
}

func parseAntiCandyRound(raw []byte) (*antiCandyRound, bool) {
	events := parseAntiCandySSEEvents(raw)
	if len(events) == 0 {
		return nil, false
	}
	round := &antiCandyRound{
		Events:                    events,
		ReasoningIndexBySource:    map[int]int{},
		NonReasoningIndexBySource: map[int]int{},
	}
	reasoningByIndex := map[int]map[string]any{}
	nonReasoningByIndex := map[int]map[string]any{}
	for _, event := range events {
		if event.Done {
			continue
		}
		if event.Type == "response.created" && round.BaseResponse == nil {
			if response, ok := event.Payload["response"].(map[string]any); ok {
				round.BaseResponse = antiCandyCloneMap(response)
			}
		}
		if antiCandyTerminalTypes[event.Type] {
			round.Terminal = antiCandyCloneMap(event.Payload)
			round.TerminalType = event.Type
			round.HasTerminal = true
			if response, ok := event.Payload["response"].(map[string]any); ok {
				if usage, ok := response["usage"].(map[string]any); ok {
					round.Usage = antiCandyCloneMap(usage)
				}
			}
		}
		if event.Type != "response.output_item.added" && event.Type != "response.output_item.done" {
			continue
		}
		index, hasIndex := antiCandyIntValue(event.Payload["output_index"])
		item, ok := event.Payload["item"].(map[string]any)
		if !ok {
			continue
		}
		if !hasIndex {
			index = len(reasoningByIndex) + len(nonReasoningByIndex)
		}
		itemCopy := antiCandyCloneMap(item)
		itemType := strings.ToLower(strings.TrimSpace(antiCandyString(itemCopy["type"])))
		if itemType == "reasoning" {
			if event.Type == "response.output_item.done" || reasoningByIndex[index] == nil || antiCandyRoundItemHasEncryptedContent(itemCopy) {
				reasoningByIndex[index] = antiCandyMergeItem(reasoningByIndex[index], itemCopy)
			}
		} else if event.Type == "response.output_item.done" || nonReasoningByIndex[index] == nil {
			nonReasoningByIndex[index] = antiCandyMergeItem(nonReasoningByIndex[index], itemCopy)
		}
	}
	if round.Terminal != nil {
		if response, ok := round.Terminal["response"].(map[string]any); ok {
			if round.Usage == nil {
				if usage, ok := response["usage"].(map[string]any); ok {
					round.Usage = antiCandyCloneMap(usage)
				}
			}
			if output, ok := response["output"].([]any); ok {
				for index, rawItem := range output {
					item, ok := rawItem.(map[string]any)
					if !ok {
						continue
					}
					itemCopy := antiCandyCloneMap(item)
					itemType := strings.ToLower(strings.TrimSpace(antiCandyString(itemCopy["type"])))
					if itemType == "reasoning" {
						if reasoningByIndex[index] == nil || antiCandyRoundItemHasEncryptedContent(itemCopy) {
							reasoningByIndex[index] = antiCandyMergeItem(reasoningByIndex[index], itemCopy)
						}
					} else if nonReasoningByIndex[index] == nil {
						nonReasoningByIndex[index] = antiCandyMergeItem(nonReasoningByIndex[index], itemCopy)
					}
				}
			}
		}
	}
	if round.Usage == nil {
		round.Usage = map[string]any{}
	}

	reasoningIndexes := antiCandySortedMapIndexes(reasoningByIndex)
	for localIndex, sourceIndex := range reasoningIndexes {
		round.ReasoningIndexBySource[sourceIndex] = localIndex
		round.ReasoningItems = append(round.ReasoningItems, reasoningByIndex[sourceIndex])
	}
	nonReasoningIndexes := antiCandySortedMapIndexes(nonReasoningByIndex)
	for localIndex, sourceIndex := range nonReasoningIndexes {
		round.NonReasoningIndexBySource[sourceIndex] = localIndex
		round.NonReasoningItems = append(round.NonReasoningItems, nonReasoningByIndex[sourceIndex])
	}
	return round, true
}

func parseAntiCandySSEEvents(raw []byte) []antiCandySSEEvent {
	blocks := antiCandySSEBlocks(raw)
	events := make([]antiCandySSEEvent, 0, len(blocks))
	for _, block := range blocks {
		var eventName string
		dataLines := make([]string, 0, 1)
		scanner := bufio.NewScanner(bytes.NewReader(block))
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r")
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			} else if strings.HasPrefix(trimmed, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
			}
		}
		payloadText := strings.TrimSpace(strings.Join(dataLines, "\n"))
		if payloadText == "" {
			continue
		}
		if payloadText == "[DONE]" {
			events = append(events, antiCandySSEEvent{Raw: append([]byte(nil), block...), EventName: eventName, Done: true})
			continue
		}
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
			continue
		}
		eventType := strings.TrimSpace(antiCandyString(payload["type"]))
		if eventType == "" {
			eventType = eventName
		}
		events = append(events, antiCandySSEEvent{
			Raw:       append([]byte(nil), block...),
			EventName: eventName,
			Type:      eventType,
			Payload:   payload,
		})
	}
	return events
}

func antiCandySSEBlocks(raw []byte) [][]byte {
	reader := bufio.NewReader(bytes.NewReader(raw))
	blocks := make([][]byte, 0, 32)
	var block bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = block.Write(line)
			if strings.TrimSpace(string(line)) == "" {
				if block.Len() > 0 {
					blocks = append(blocks, append([]byte(nil), block.Bytes()...))
					block.Reset()
				}
			}
		}
		if err != nil {
			if err == io.EOF && block.Len() > 0 {
				blocks = append(blocks, append([]byte(nil), block.Bytes()...))
			}
			break
		}
	}
	return blocks
}

func antiCandySortedMapIndexes(values map[int]map[string]any) []int {
	indexes := make([]int, 0, len(values))
	for index := range values {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	return indexes
}

func antiCandyRoundItemHasEncryptedContent(item map[string]any) bool {
	return strings.TrimSpace(antiCandyString(item["encrypted_content"])) != ""
}

func antiCandyMergeItem(existing map[string]any, incoming map[string]any) map[string]any {
	if existing == nil {
		return antiCandyCloneMap(incoming)
	}
	merged := antiCandyCloneMap(existing)
	for key, value := range incoming {
		merged[key] = antiCandyCloneValue(value)
	}
	return merged
}

func antiCandyTierN(usage map[string]any) int {
	reasoningTokens := antiCandyNumberFromDetails(usage, "output_tokens_details", "reasoning_tokens")
	if reasoningTokens < antiCandyReasoningStep-2 {
		return 0
	}
	value := int(reasoningTokens)
	if (value+2)%antiCandyReasoningStep != 0 {
		return 0
	}
	return (value + 2) / antiCandyReasoningStep
}

func antiCandySumUsage(acc map[string]any, usage map[string]any) {
	if acc == nil || usage == nil {
		return
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens"} {
		if _, ok := usage[key]; ok {
			acc[key] = antiCandyNumber(acc[key]) + antiCandyNumber(usage[key])
		}
	}
	for _, group := range []struct {
		parent string
		child  string
	}{
		{parent: "input_tokens_details", child: "cached_tokens"},
		{parent: "output_tokens_details", child: "reasoning_tokens"},
	} {
		value := antiCandyNumberFromDetails(usage, group.parent, group.child)
		if value == 0 {
			continue
		}
		details, _ := acc[group.parent].(map[string]any)
		if details == nil {
			details = map[string]any{}
			acc[group.parent] = details
		}
		details[group.child] = antiCandyNumber(details[group.child]) + value
	}
}

func antiCandyAgentUsage(first map[string]any, summed map[string]any, final map[string]any) map[string]any {
	inputTokens := antiCandyNumber(first["input_tokens"])
	reasoningTokens := antiCandyNumberFromDetails(summed, "output_tokens_details", "reasoning_tokens")
	finalOutput := antiCandyNumber(final["output_tokens"])
	finalReasoning := antiCandyNumberFromDetails(final, "output_tokens_details", "reasoning_tokens")
	finalVisibleOutput := finalOutput - finalReasoning
	if finalVisibleOutput < 0 {
		finalVisibleOutput = 0
	}
	usage := map[string]any{
		"input_tokens":  inputTokens,
		"output_tokens": reasoningTokens + finalVisibleOutput,
		"total_tokens":  inputTokens + reasoningTokens + finalVisibleOutput,
		"output_tokens_details": map[string]any{
			"reasoning_tokens": reasoningTokens,
		},
	}
	if cached := antiCandyNumberFromDetails(first, "input_tokens_details", "cached_tokens"); cached > 0 {
		usage["input_tokens_details"] = map[string]any{"cached_tokens": cached}
	}
	return usage
}

func buildAntiCandyFoldedStream(rounds []*antiCandyRound, finalRound *antiCandyRound, baseResponse map[string]any, billedUsage map[string]any, roundMetadata []map[string]any, stoppedReason string) []byte {
	if len(rounds) == 0 || finalRound == nil {
		return nil
	}
	totalReasoning := 0
	for _, round := range rounds {
		totalReasoning += len(round.ReasoningItems)
	}
	finalIndex := len(rounds) - 1
	indexMaps := make([]map[int]int, len(rounds))
	reasoningOffset := 0
	for roundIndex, round := range rounds {
		mapping := map[int]int{}
		for sourceIndex, localIndex := range round.ReasoningIndexBySource {
			mapping[sourceIndex] = reasoningOffset + localIndex
		}
		if roundIndex == finalIndex {
			for sourceIndex, localIndex := range round.NonReasoningIndexBySource {
				mapping[sourceIndex] = totalReasoning + localIndex
			}
		}
		indexMaps[roundIndex] = mapping
		reasoningOffset += len(round.ReasoningItems)
	}

	var output bytes.Buffer
	for roundIndex, round := range rounds {
		for _, event := range round.Events {
			if event.Done || event.Type == "" || antiCandyTerminalTypes[event.Type] {
				continue
			}
			isReasoning := antiCandyEventIsReasoning(event, round)
			isOutput := antiCandyEventIsOutput(event)
			if roundIndex == 0 && antiCandyEventIsLifecycle(event) {
				antiCandyWriteEvent(&output, event, nil)
				continue
			}
			if !isReasoning && (roundIndex != finalIndex || !isOutput) {
				continue
			}
			antiCandyWriteEvent(&output, event, indexMaps[roundIndex])
		}
	}

	finalResponse := antiCandyCloneMap(baseResponse)
	if finalResponse == nil {
		if response, ok := finalRound.Terminal["response"].(map[string]any); ok {
			finalResponse = antiCandyCloneMap(response)
		} else {
			finalResponse = map[string]any{}
		}
	}
	finalOutput := make([]any, 0, totalReasoning+len(finalRound.NonReasoningItems))
	for _, round := range rounds {
		for _, item := range round.ReasoningItems {
			finalOutput = append(finalOutput, antiCandyCloneMap(item))
		}
	}
	for _, item := range finalRound.NonReasoningItems {
		finalOutput = append(finalOutput, antiCandyCloneMap(item))
	}
	finalResponse["output"] = finalOutput
	firstUsage := rounds[0].Usage
	finalResponse["usage"] = antiCandyAgentUsage(firstUsage, billedUsage, finalRound.Usage)
	metadata, _ := finalResponse["metadata"].(map[string]any)
	metadata = antiCandyCloneMap(metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["proxy_rounds"] = roundMetadata
	metadata["proxy_billed_usage"] = antiCandyCloneMap(billedUsage)
	if strings.TrimSpace(stoppedReason) != "" {
		metadata["proxy_stopped_reason"] = stoppedReason
	}
	finalResponse["metadata"] = metadata
	if terminalResponse, ok := finalRound.Terminal["response"].(map[string]any); ok {
		if status := strings.TrimSpace(antiCandyString(terminalResponse["status"])); status != "" {
			finalResponse["status"] = status
		}
		if incompleteDetails, exists := terminalResponse["incomplete_details"]; exists {
			finalResponse["incomplete_details"] = antiCandyCloneValue(incompleteDetails)
		}
	}
	if strings.TrimSpace(antiCandyString(finalResponse["status"])) == "" {
		finalResponse["status"] = "completed"
	}
	terminalType := firstNonEmpty(finalRound.TerminalType, "response.completed")
	terminalPayload := map[string]any{
		"type":     terminalType,
		"response": finalResponse,
	}
	antiCandyWriteSyntheticEvent(&output, terminalType, terminalPayload)
	_, _ = output.WriteString("data: [DONE]\n\n")
	return output.Bytes()
}

func antiCandyEventIsLifecycle(event antiCandySSEEvent) bool {
	switch event.Type {
	case "response.created", "response.queued", "response.in_progress":
		return true
	default:
		return false
	}
}

func antiCandyEventIsReasoning(event antiCandySSEEvent, round *antiCandyRound) bool {
	if event.Type == "response.output_item.added" || event.Type == "response.output_item.done" {
		if item, ok := event.Payload["item"].(map[string]any); ok {
			return strings.EqualFold(strings.TrimSpace(antiCandyString(item["type"])), "reasoning")
		}
		if index, ok := antiCandyIntValue(event.Payload["output_index"]); ok {
			_, found := round.ReasoningIndexBySource[index]
			return found
		}
	}
	return strings.Contains(strings.ToLower(event.Type), "reasoning")
}

func antiCandyEventIsOutput(event antiCandySSEEvent) bool {
	lower := strings.ToLower(event.Type)
	for _, marker := range []string{"output_item", "output_text", "content_part", "function_call", "custom_tool_call", "web_search_call", "file_search_call", "computer_call", "refusal"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func antiCandyWriteEvent(output *bytes.Buffer, event antiCandySSEEvent, indexMap map[int]int) {
	if event.Payload == nil {
		if len(event.Raw) > 0 {
			_, _ = output.Write(event.Raw)
		}
		return
	}
	payload := antiCandyCloneMap(event.Payload)
	if indexMap != nil {
		if sourceIndex, ok := antiCandyIntValue(payload["output_index"]); ok {
			if mapped, found := indexMap[sourceIndex]; found {
				payload["output_index"] = mapped
			}
		}
	}
	eventName := firstNonEmpty(event.EventName, event.Type)
	antiCandyWriteSyntheticEvent(output, eventName, payload)
}

func antiCandyWriteSyntheticEvent(output *bytes.Buffer, eventName string, payload map[string]any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(output, "event: %s\ndata: %s\n\n", eventName, string(raw))
}

func antiCandyCloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = antiCandyCloneValue(value)
	}
	return output
}

func antiCandyCloneSlice(input []any) []any {
	if input == nil {
		return nil
	}
	output := make([]any, len(input))
	for index, value := range input {
		output[index] = antiCandyCloneValue(value)
	}
	return output
}

func antiCandyCloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return antiCandyCloneMap(typed)
	case []any:
		return antiCandyCloneSlice(typed)
	default:
		return typed
	}
}

func antiCandyString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func antiCandyNumber(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed
	default:
		return 0
	}
}

func antiCandyNumberFromDetails(parent map[string]any, group string, key string) float64 {
	if parent == nil {
		return 0
	}
	details, _ := parent[group].(map[string]any)
	if details == nil {
		return 0
	}
	return antiCandyNumber(details[key])
}

func antiCandyIntValue(value any) (int, bool) {
	if value == nil {
		return 0, false
	}
	return int(antiCandyNumber(value)), true
}
