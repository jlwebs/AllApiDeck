package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"
)

const advancedProxyRequestRecordLimit = 400

type AdvancedProxyRequestRecord struct {
	ID                 string                          `json:"id"`
	RecordedAt         string                          `json:"recordedAt"`
	AppType            string                          `json:"appType"`
	ClientRoute        string                          `json:"clientRoute"`
	InboundEndpoint    string                          `json:"inboundEndpoint"`
	OutboundRoute      string                          `json:"outboundRoute"`
	RouteTrace         []AdvancedProxyRequestRouteStep `json:"routeTrace,omitempty"`
	ProviderID         string                          `json:"providerId"`
	ProviderRowKey     string                          `json:"providerRowKey"`
	ProviderName       string                          `json:"providerName"`
	ProviderKeyPreview string                          `json:"providerKeyPreview"`
	Model              string                          `json:"model"`
	Stream             bool                            `json:"stream"`
	StatusCode         int                             `json:"statusCode"`
	DurationMs         int64                           `json:"durationMs"`
	TTFTMs             *int64                          `json:"ttftMs,omitempty"`
	LatencyMs          *int64                          `json:"latencyMs,omitempty"`
	InputTokens        *int                            `json:"inputTokens,omitempty"`
	OutputTokens       *int                            `json:"outputTokens,omitempty"`
	TPS                *float64                        `json:"tps,omitempty"`
	UpstreamURL        string                          `json:"upstreamUrl"`
	UpstreamEndpoint   string                          `json:"upstreamEndpoint"`
	ErrorDetail        string                          `json:"errorDetail"`
	Source             string                          `json:"source"`
}

type AdvancedProxyRequestRouteStep struct {
	Route  string `json:"route"`
	Source string `json:"source,omitempty"`
	Status string `json:"status"`
}

type advancedProxyRecordedMetrics struct {
	DurationMs   int64
	TTFTMs       *int64
	LatencyMs    *int64
	InputTokens  *int
	OutputTokens *int
	TPS          *float64
}

type advancedProxyRequestRecordStore struct {
	mu      sync.Mutex
	records []AdvancedProxyRequestRecord
	seq     uint64
}

var advancedProxyRequestRecords advancedProxyRequestRecordStore

func (s *advancedProxyRequestRecordStore) append(record AdvancedProxyRequestRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	record.ID = fmt.Sprintf("advreq-%d-%d", time.Now().UnixMilli(), s.seq)
	if strings.TrimSpace(record.RecordedAt) == "" {
		record.RecordedAt = time.Now().Format(time.RFC3339Nano)
	}
	s.records = append(s.records, record)
	if overflow := len(s.records) - advancedProxyRequestRecordLimit; overflow > 0 {
		s.records = append([]AdvancedProxyRequestRecord(nil), s.records[overflow:]...)
	}
}

func (s *advancedProxyRequestRecordStore) list(limit int) []AdvancedProxyRequestRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 120
	}
	if limit > advancedProxyRequestRecordLimit {
		limit = advancedProxyRequestRecordLimit
	}
	if len(s.records) == 0 {
		return []AdvancedProxyRequestRecord{}
	}
	start := len(s.records) - limit
	if start < 0 {
		start = 0
	}
	source := s.records[start:]
	result := make([]AdvancedProxyRequestRecord, 0, len(source))
	for index := len(source) - 1; index >= 0; index-- {
		result = append(result, source[index])
	}
	return result
}

func (s *advancedProxyRequestRecordStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
	s.seq = 0
}

func (a *App) GetAdvancedProxyRequestRecords(limit int) ([]AdvancedProxyRequestRecord, error) {
	return advancedProxyRequestRecords.list(limit), nil
}

func (a *App) ClearAdvancedProxyRequestRecords() (bool, error) {
	advancedProxyRequestRecords.clear()
	return true, nil
}

func appendAdvancedProxyRequestRecord(record AdvancedProxyRequestRecord) {
	record.AppType = strings.TrimSpace(strings.ToLower(record.AppType))
	record.ClientRoute = strings.TrimSpace(record.ClientRoute)
	record.InboundEndpoint = strings.TrimSpace(record.InboundEndpoint)
	record.OutboundRoute = strings.TrimSpace(record.OutboundRoute)
	record.RouteTrace = normalizeAdvancedProxyRouteTrace(record.RouteTrace)
	record.ProviderID = strings.TrimSpace(record.ProviderID)
	record.ProviderRowKey = strings.TrimSpace(record.ProviderRowKey)
	record.ProviderName = strings.TrimSpace(record.ProviderName)
	record.ProviderKeyPreview = strings.TrimSpace(record.ProviderKeyPreview)
	record.Model = strings.TrimSpace(record.Model)
	record.UpstreamURL = strings.TrimSpace(record.UpstreamURL)
	record.UpstreamEndpoint = strings.TrimSpace(record.UpstreamEndpoint)
	record.ErrorDetail = strings.TrimSpace(record.ErrorDetail)
	record.Source = strings.TrimSpace(record.Source)
	advancedProxyRequestRecords.append(record)
}

func recordAdvancedProxyOpenAIAttempt(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, responseBody []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string) {
	recordAdvancedProxyOpenAIAttemptWithTrace(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, responseBody, stream, statusCode, elapsed, errorDetail, nil)
}

func recordAdvancedProxyOpenAIAttemptWithTrace(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, responseBody []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	usageInput, usageOutput := extractAdvancedProxyUsageFromBody(responseBody)
	metrics := buildAdvancedProxyRecordedMetrics(elapsed, usageInput, usageOutput)
	resolvedDetail := strings.TrimSpace(errorDetail)
	if resolvedDetail == "" && (statusCode < 200 || statusCode >= 300) {
		resolvedDetail = summarizeAdvancedProxyBody(responseBody)
	}
	appendAdvancedProxyOpenAIRecord(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, resolvedDetail, routeTrace)
}

func recordAdvancedProxyOpenAIStreamAttempt(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string) {
	recordAdvancedProxyOpenAIStreamAttemptWithTrace(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, nil)
}

func recordAdvancedProxyOpenAIStreamAttemptWithTrace(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	metrics := buildAdvancedProxyStreamRecordedMetrics(startedAt, firstOutputAt, completedAt, inputTokens, outputTokens)
	appendAdvancedProxyOpenAIRecord(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, errorDetail, routeTrace)
}

func appendAdvancedProxyOpenAIRecord(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, stream bool, statusCode int, metrics advancedProxyRecordedMetrics, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	record := AdvancedProxyRequestRecord{
		RecordedAt:         time.Now().Format(time.RFC3339Nano),
		AppType:            appType,
		ClientRoute:        clientRoute,
		InboundEndpoint:    inboundEndpoint,
		OutboundRoute:      outboundRoute,
		RouteTrace:         cloneAdvancedProxyRouteTrace(routeTrace),
		ProviderID:         strings.TrimSpace(provider.ID),
		ProviderRowKey:     strings.TrimSpace(provider.RowKey),
		ProviderName:       advancedProxyProviderLabel(provider),
		ProviderKeyPreview: maskAdvancedProxyAPIKey(provider.APIKey),
		Model:              resolveAdvancedProxyRecordedModel(resolvedModel, requestBody, provider.Model),
		Stream:             stream,
		StatusCode:         statusCode,
		DurationMs:         metrics.DurationMs,
		TTFTMs:             metrics.TTFTMs,
		LatencyMs:          metrics.LatencyMs,
		InputTokens:        metrics.InputTokens,
		OutputTokens:       metrics.OutputTokens,
		TPS:                metrics.TPS,
		UpstreamURL:        targetURL,
		UpstreamEndpoint:   extractAdvancedProxyURLPath(targetURL),
		ErrorDetail:        strings.TrimSpace(errorDetail),
		Source:             source,
	}
	appendAdvancedProxyRequestRecord(record)
}

func recordAdvancedProxyClaudeAttempt(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, response map[string]any, rawResponse []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string) {
	recordAdvancedProxyClaudeAttemptWithTrace(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, response, rawResponse, stream, statusCode, elapsed, errorDetail, nil)
}

func recordAdvancedProxyClaudeAttemptWithTrace(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, response map[string]any, rawResponse []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	usageInput, usageOutput := extractAdvancedProxyUsageFromMap(response)
	metrics := buildAdvancedProxyRecordedMetrics(elapsed, usageInput, usageOutput)
	resolvedDetail := strings.TrimSpace(errorDetail)
	if resolvedDetail == "" && (statusCode < 200 || statusCode >= 300) {
		resolvedDetail = summarizeAdvancedProxyBody(rawResponse)
	}
	appendAdvancedProxyClaudeRecord(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, resolvedDetail, routeTrace)
}

func recordAdvancedProxyClaudeStreamAttempt(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string) {
	recordAdvancedProxyClaudeStreamAttemptWithTrace(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, nil)
}

func recordAdvancedProxyClaudeStreamAttemptWithTrace(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	metrics := buildAdvancedProxyStreamRecordedMetrics(startedAt, firstOutputAt, completedAt, inputTokens, outputTokens)
	appendAdvancedProxyClaudeRecord(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, errorDetail, routeTrace)
}

func appendAdvancedProxyClaudeRecord(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, stream bool, statusCode int, metrics advancedProxyRecordedMetrics, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	record := AdvancedProxyRequestRecord{
		RecordedAt:         time.Now().Format(time.RFC3339Nano),
		AppType:            appType,
		ClientRoute:        "messages",
		InboundEndpoint:    inboundEndpoint,
		OutboundRoute:      outboundRoute,
		RouteTrace:         cloneAdvancedProxyRouteTrace(routeTrace),
		ProviderID:         strings.TrimSpace(provider.ID),
		ProviderRowKey:     strings.TrimSpace(provider.RowKey),
		ProviderName:       advancedProxyProviderLabel(provider),
		ProviderKeyPreview: maskAdvancedProxyAPIKey(provider.APIKey),
		Model:              resolveAdvancedProxyRecordedModel(resolvedModel, requestBody, provider.Model),
		Stream:             stream,
		StatusCode:         statusCode,
		DurationMs:         metrics.DurationMs,
		TTFTMs:             metrics.TTFTMs,
		LatencyMs:          metrics.LatencyMs,
		InputTokens:        metrics.InputTokens,
		OutputTokens:       metrics.OutputTokens,
		TPS:                metrics.TPS,
		UpstreamURL:        targetURL,
		UpstreamEndpoint:   extractAdvancedProxyURLPath(targetURL),
		ErrorDetail:        strings.TrimSpace(errorDetail),
		Source:             "direct",
	}
	appendAdvancedProxyRequestRecord(record)
}

func cloneAdvancedProxyRouteTrace(source []AdvancedProxyRequestRouteStep) []AdvancedProxyRequestRouteStep {
	if len(source) == 0 {
		return nil
	}
	result := make([]AdvancedProxyRequestRouteStep, 0, len(source))
	for _, step := range source {
		route := strings.TrimSpace(step.Route)
		if route == "" {
			continue
		}
		result = append(result, AdvancedProxyRequestRouteStep{
			Route:  route,
			Source: strings.TrimSpace(step.Source),
			Status: strings.TrimSpace(step.Status),
		})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeAdvancedProxyRouteTrace(source []AdvancedProxyRequestRouteStep) []AdvancedProxyRequestRouteStep {
	cloned := cloneAdvancedProxyRouteTrace(source)
	if len(cloned) == 0 {
		return nil
	}
	result := make([]AdvancedProxyRequestRouteStep, 0, len(cloned))
	for _, step := range cloned {
		status := strings.ToLower(strings.TrimSpace(step.Status))
		switch status {
		case "success", "failed":
		default:
			status = "success"
		}
		result = append(result, AdvancedProxyRequestRouteStep{
			Route:  strings.TrimSpace(step.Route),
			Source: strings.TrimSpace(strings.ToLower(step.Source)),
			Status: status,
		})
	}
	return result
}

func buildAdvancedProxyRecordedMetrics(elapsed time.Duration, inputTokens *int, outputTokens *int) advancedProxyRecordedMetrics {
	durationMs := elapsed.Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	metrics := advancedProxyRecordedMetrics{
		DurationMs:   durationMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
	if durationMs > 0 {
		metrics.LatencyMs = int64Ptr(durationMs)
	}
	metrics.TPS = calculateAdvancedProxyTPS(outputTokens, metrics.LatencyMs)
	return metrics
}

func buildAdvancedProxyStreamRecordedMetrics(startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int) advancedProxyRecordedMetrics {
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	if startedAt.IsZero() {
		startedAt = completedAt
	}
	durationMs := completedAt.Sub(startedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	metrics := advancedProxyRecordedMetrics{
		DurationMs:   durationMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
	if firstOutputAt != nil && !firstOutputAt.IsZero() && !firstOutputAt.Before(startedAt) {
		ttftMs := firstOutputAt.Sub(startedAt).Milliseconds()
		if ttftMs > 0 {
			metrics.TTFTMs = int64Ptr(ttftMs)
		}
		generationMs := completedAt.Sub(*firstOutputAt).Milliseconds()
		if generationMs > 0 {
			metrics.LatencyMs = int64Ptr(generationMs)
		}
	}
	if metrics.LatencyMs == nil && durationMs > 0 {
		metrics.LatencyMs = int64Ptr(durationMs)
	}
	metrics.TPS = calculateAdvancedProxyTPS(outputTokens, metrics.LatencyMs)
	return metrics
}

func calculateAdvancedProxyTPS(outputTokens *int, latencyMs *int64) *float64 {
	if outputTokens == nil || latencyMs == nil {
		return nil
	}
	if *outputTokens <= 0 || *latencyMs <= 0 {
		return nil
	}
	value := float64(*outputTokens) / (float64(*latencyMs) / 1000)
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return nil
	}
	return float64PtrValue(value)
}

func buildAdvancedProxyOpenAIInboundEndpoint(appType string, routeKind string) string {
	basePath := advancedProxyCodexBasePath
	switch strings.TrimSpace(strings.ToLower(appType)) {
	case "opencode":
		basePath = advancedProxyOpenCodePath
	case "openclaw":
		basePath = advancedProxyOpenClawPath
	}
	switch strings.TrimSpace(routeKind) {
	case "chat":
		return strings.TrimRight(basePath, "/") + "/chat/completions"
	case "responses_compact":
		return strings.TrimRight(basePath, "/") + "/responses/compact"
	default:
		return strings.TrimRight(basePath, "/") + "/responses"
	}
}

func buildAdvancedProxyClaudeInboundEndpoint() string {
	return strings.TrimRight(advancedProxyClaudeBasePath, "/") + "/messages"
}

func extractAdvancedProxyUsageFromBody(body []byte) (*int, *int) {
	if len(body) == 0 {
		return nil, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil
	}
	return extractAdvancedProxyUsageFromMap(payload)
}

func extractAdvancedProxyUsageFromMap(payload map[string]any) (*int, *int) {
	if payload == nil {
		return nil, nil
	}
	usageMap, _ := payload["usage"].(map[string]any)
	if usageMap == nil {
		return nil, nil
	}
	inputValue, inputOK := extractAdvancedProxyFirstPositiveInt(
		usageMap["input_tokens"],
		usageMap["prompt_tokens"],
		usageMap["inputTokens"],
		usageMap["promptTokens"],
	)
	outputValue, outputOK := extractAdvancedProxyFirstPositiveInt(
		usageMap["output_tokens"],
		usageMap["completion_tokens"],
		usageMap["outputTokens"],
		usageMap["completionTokens"],
	)
	var inputPtr *int
	var outputPtr *int
	if inputOK {
		inputPtr = intPtrValue(inputValue)
	}
	if outputOK {
		outputPtr = intPtrValue(outputValue)
	}
	return inputPtr, outputPtr
}

func extractAdvancedProxyFirstPositiveInt(values ...any) (int, bool) {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed), true
			}
		default:
			if parsed := toIntValue(value); parsed > 0 {
				return parsed, true
			}
		}
		if parsed := toIntValue(value); parsed == 0 {
			return 0, true
		}
	}
	return 0, false
}

func extractAdvancedProxyModelFromBody(requestBody []byte, fallback string) string {
	payload := map[string]any{}
	if len(requestBody) > 0 && json.Unmarshal(requestBody, &payload) == nil {
		if model := strings.TrimSpace(toStringValue(payload["model"])); model != "" {
			return model
		}
	}
	return strings.TrimSpace(fallback)
}

func resolveAdvancedProxyRecordedModel(resolvedModel string, requestBody []byte, fallback string) string {
	if model := strings.TrimSpace(resolvedModel); model != "" {
		return model
	}
	return extractAdvancedProxyModelFromBody(requestBody, fallback)
}

func extractAdvancedProxyURLPath(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return ""
	}
	path := strings.TrimSpace(parsed.Path)
	if path == "" {
		path = "/"
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		path += "?" + parsed.RawQuery
	}
	return path
}

func maskAdvancedProxyAPIKey(apiKey string) string {
	text := strings.TrimSpace(apiKey)
	if text == "" {
		return ""
	}
	if len(text) <= 10 {
		hash := sha1.Sum([]byte(text))
		return fmt.Sprintf("%s···%x", text[:minRequestRecordInt(len(text), 4)], hash[:2])
	}
	return fmt.Sprintf("%s···%s", text[:6], text[len(text)-4:])
}

func intPtrValue(value int) *int {
	next := value
	return &next
}

func int64Ptr(value int64) *int64 {
	next := value
	return &next
}

func float64PtrValue(value float64) *float64 {
	next := value
	return &next
}

func minRequestRecordInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
