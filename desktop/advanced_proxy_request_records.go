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
const advancedProxyRequestPayloadLimit = 50

type AdvancedProxyRequestRecord struct {
	ID                       string                          `json:"id"`
	RecordedAt               string                          `json:"recordedAt"`
	AppType                  string                          `json:"appType"`
	ClientRoute              string                          `json:"clientRoute"`
	InboundEndpoint          string                          `json:"inboundEndpoint"`
	OutboundRoute            string                          `json:"outboundRoute"`
	RouteTrace               []AdvancedProxyRequestRouteStep `json:"routeTrace,omitempty"`
	ProviderID               string                          `json:"providerId"`
	ProviderRowKey           string                          `json:"providerRowKey"`
	ProviderName             string                          `json:"providerName"`
	ProviderKeyPreview       string                          `json:"providerKeyPreview"`
	Model                    string                          `json:"model"`
	Stream                   bool                            `json:"stream"`
	StatusCode               int                             `json:"statusCode"`
	DurationMs               int64                           `json:"durationMs"`
	TTFTMs                   *int64                          `json:"ttftMs,omitempty"`
	LatencyMs                *int64                          `json:"latencyMs,omitempty"`
	InputTokens              *int                            `json:"inputTokens,omitempty"`
	OutputTokens             *int                            `json:"outputTokens,omitempty"`
	TPS                      *float64                        `json:"tps,omitempty"`
	UpstreamURL              string                          `json:"upstreamUrl"`
	UpstreamEndpoint         string                          `json:"upstreamEndpoint"`
	ErrorDetail              string                          `json:"errorDetail"`
	Source                   string                          `json:"source"`
	RequestBody              string                          `json:"requestBody,omitempty"`
	AntiPoisonPromptPreview  string                          `json:"antiPoisonPromptPreview,omitempty"`
	UpstreamResponsePreview  string                          `json:"upstreamResponsePreview,omitempty"`
	UpstreamResponseRaw      string                          `json:"upstreamResponseRaw,omitempty"`
	ResponsePreview          string                          `json:"responsePreview,omitempty"`
	UpstreamToolCalls        []string                        `json:"upstreamToolCalls,omitempty"`
	UpstreamToolArgsPreview  []string                        `json:"upstreamToolArgsPreview,omitempty"`
	UpstreamAssistantPreview string                          `json:"upstreamAssistantPreview,omitempty"`
	UpstreamLatestObserved   *advancedProxyObservedItem      `json:"upstreamLatestObserved,omitempty"`
	AntiPoisonOps            []antiPoisonOperationRecord     `json:"antiPoisonOps,omitempty"`
}

func (r AdvancedProxyRequestRecord) withoutHeavyPayloads() AdvancedProxyRequestRecord {
	r.RequestBody = ""
	r.UpstreamResponseRaw = ""
	return r
}

type advancedProxyObservedItem struct {
	Type             string `json:"type"`
	Name             string `json:"name,omitempty"`
	ArgumentsPreview string `json:"argumentsPreview,omitempty"`
	TextPreview      string `json:"textPreview,omitempty"`
	RawPreview       string `json:"rawPreview,omitempty"`
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
	if payloadOverflow := len(s.records) - advancedProxyRequestPayloadLimit; payloadOverflow > 0 {
		for index := 0; index < payloadOverflow; index++ {
			s.records[index].RequestBody = ""
		}
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

func (s *advancedProxyRequestRecordStore) listSummaries(limit int) []AdvancedProxyRequestRecord {
	records := s.list(limit)
	for index := range records {
		records[index] = records[index].withoutHeavyPayloads()
	}
	return records
}

func (s *advancedProxyRequestRecordStore) get(recordID string) (AdvancedProxyRequestRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return AdvancedProxyRequestRecord{}, false
	}
	for index := len(s.records) - 1; index >= 0; index-- {
		record := s.records[index]
		if record.ID == recordID {
			return record, true
		}
	}
	return AdvancedProxyRequestRecord{}, false
}

func (s *advancedProxyRequestRecordStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
	s.seq = 0
}

func (a *App) GetAdvancedProxyRequestRecords(limit int) ([]AdvancedProxyRequestRecord, error) {
	return advancedProxyRequestRecords.listSummaries(limit), nil
}

func (a *App) ClearAdvancedProxyRequestRecords() (bool, error) {
	advancedProxyRequestRecords.clear()
	return true, nil
}

func (a *App) GetAdvancedProxyRequestRecord(recordID string) (*AdvancedProxyRequestRecord, error) {
	record, ok := advancedProxyRequestRecords.get(recordID)
	if !ok {
		return nil, nil
	}
	return &record, nil
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
	record.RequestBody = strings.TrimSpace(record.RequestBody)
	record.AntiPoisonPromptPreview = strings.TrimSpace(record.AntiPoisonPromptPreview)
	record.UpstreamResponsePreview = strings.TrimSpace(record.UpstreamResponsePreview)
	record.UpstreamResponseRaw = strings.TrimSpace(record.UpstreamResponseRaw)
	record.ResponsePreview = strings.TrimSpace(record.ResponsePreview)
	record.UpstreamToolCalls = normalizeAdvancedProxyPreviewList(record.UpstreamToolCalls, 24, 160)
	record.UpstreamToolArgsPreview = normalizeAdvancedProxyPreviewList(record.UpstreamToolArgsPreview, 24, 280)
	record.UpstreamAssistantPreview = strings.TrimSpace(record.UpstreamAssistantPreview)
	record.UpstreamLatestObserved = normalizeAdvancedProxyObservedItem(record.UpstreamLatestObserved)
	record.AntiPoisonOps = normalizeAntiPoisonOperationRecords(record.AntiPoisonOps)
	advancedProxyRequestRecords.append(record)
}

func normalizeAdvancedProxyPreviewList(values []string, maxItems int, itemLimit int) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = previewAdvancedProxyText(value, itemLimit)
		if value == "" {
			continue
		}
		result = append(result, value)
		if len(result) >= maxItems {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeAntiPoisonOperationRecords(records []antiPoisonOperationRecord) []antiPoisonOperationRecord {
	if len(records) == 0 {
		return nil
	}
	result := make([]antiPoisonOperationRecord, 0, len(records))
	for _, record := range records {
		record.ID = strings.TrimSpace(record.ID)
		record.Time = strings.TrimSpace(record.Time)
		record.Stage = strings.TrimSpace(record.Stage)
		record.Channel = strings.TrimSpace(record.Channel)
		record.Rule = strings.TrimSpace(record.Rule)
		record.Path = strings.TrimSpace(record.Path)
		record.Before = previewAdvancedProxyText(record.Before, 180)
		record.After = previewAdvancedProxyText(record.After, 180)
		record.Route = strings.TrimSpace(record.Route)
		record.Provider = strings.TrimSpace(record.Provider)
		record.Reason = strings.TrimSpace(record.Reason)
		if record.Stage == "" && record.Rule == "" && record.Before == "" && record.After == "" && record.Reason == "" {
			continue
		}
		result = append(result, record)
		if len(result) >= 120 {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeAdvancedProxyObservedItem(item *advancedProxyObservedItem) *advancedProxyObservedItem {
	if item == nil {
		return nil
	}
	next := &advancedProxyObservedItem{
		Type:             strings.TrimSpace(item.Type),
		Name:             previewAdvancedProxyText(item.Name, 160),
		ArgumentsPreview: previewAdvancedProxyText(item.ArgumentsPreview, 600),
		TextPreview:      previewAdvancedProxyText(item.TextPreview, 1200),
		RawPreview:       previewAdvancedProxyText(item.RawPreview, 1600),
	}
	if next.Type == "" && next.Name == "" && next.ArgumentsPreview == "" && next.TextPreview == "" && next.RawPreview == "" {
		return nil
	}
	return next
}

func recordAdvancedProxyOpenAIAttempt(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, responseBody []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string) {
	recordAdvancedProxyOpenAIAttemptWithTrace(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, responseBody, stream, statusCode, elapsed, errorDetail, nil)
}

func recordAdvancedProxyOpenAIAttemptWithTrace(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, responseBody []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, responseBody, stream, statusCode, elapsed, errorDetail, routeTrace, nil)
}

func recordAdvancedProxyOpenAIAttemptWithTraceAndOps(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, responseBody []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord) {
	usageInput, usageOutput := extractAdvancedProxyUsageFromBody(responseBody)
	metrics := buildAdvancedProxyRecordedMetrics(elapsed, usageInput, usageOutput)
	resolvedDetail := strings.TrimSpace(errorDetail)
	if resolvedDetail == "" && (statusCode < 200 || statusCode >= 300) {
		resolvedDetail = summarizeAdvancedProxyBody(responseBody)
	}
	responsePreview := summarizeAdvancedProxyBody(responseBody)
	feedback := summarizeAdvancedProxyBodyFeedbackContext(responseBody, normalizeAdvancedProxyObservedFormat(outboundRoute))
	appendAdvancedProxyOpenAIRecord(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, resolvedDetail, responsePreview, string(responseBody), responsePreview, routeTrace, antiPoisonOps, feedback.ToolCalls, feedback.ToolArgsPreview, feedback.AssistantPreview, feedback.LatestObserved)
}

func recordAdvancedProxyOpenAIStreamAttempt(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string) {
	recordAdvancedProxyOpenAIStreamAttemptWithTrace(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, nil)
}

func recordAdvancedProxyOpenAIStreamAttemptWithTrace(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	recordAdvancedProxyOpenAIStreamAttemptWithTraceAndOps(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, "", "", "", routeTrace, nil, nil, nil, "", nil)
}

func recordAdvancedProxyOpenAIStreamAttemptWithTraceAndOps(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, upstreamResponsePreview string, upstreamResponseRaw string, responsePreview string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord, upstreamToolCalls []string, upstreamToolArgsPreview []string, upstreamAssistantPreview string, upstreamLatestObserved *advancedProxyObservedItem) {
	metrics := buildAdvancedProxyStreamRecordedMetrics(startedAt, firstOutputAt, completedAt, inputTokens, outputTokens)
	appendAdvancedProxyOpenAIRecord(appType, clientRoute, inboundEndpoint, outboundRoute, source, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, errorDetail, upstreamResponsePreview, upstreamResponseRaw, responsePreview, routeTrace, antiPoisonOps, upstreamToolCalls, upstreamToolArgsPreview, upstreamAssistantPreview, upstreamLatestObserved)
}

func appendAdvancedProxyOpenAIRecord(appType string, clientRoute string, inboundEndpoint string, outboundRoute string, source string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, stream bool, statusCode int, metrics advancedProxyRecordedMetrics, errorDetail string, upstreamResponsePreview string, upstreamResponseRaw string, responsePreview string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord, upstreamToolCalls []string, upstreamToolArgsPreview []string, upstreamAssistantPreview string, upstreamLatestObserved *advancedProxyObservedItem) {
	record := AdvancedProxyRequestRecord{
		RecordedAt:               time.Now().Format(time.RFC3339Nano),
		AppType:                  appType,
		ClientRoute:              clientRoute,
		InboundEndpoint:          inboundEndpoint,
		OutboundRoute:            outboundRoute,
		RouteTrace:               cloneAdvancedProxyRouteTrace(routeTrace),
		ProviderID:               strings.TrimSpace(provider.ID),
		ProviderRowKey:           strings.TrimSpace(provider.RowKey),
		ProviderName:             advancedProxyProviderLabel(provider),
		ProviderKeyPreview:       maskAdvancedProxyAPIKey(provider.APIKey),
		Model:                    resolveAdvancedProxyRecordedModel(resolvedModel, requestBody, provider.Model),
		Stream:                   stream,
		StatusCode:               statusCode,
		DurationMs:               metrics.DurationMs,
		TTFTMs:                   metrics.TTFTMs,
		LatencyMs:                metrics.LatencyMs,
		InputTokens:              metrics.InputTokens,
		OutputTokens:             metrics.OutputTokens,
		TPS:                      metrics.TPS,
		UpstreamURL:              targetURL,
		UpstreamEndpoint:         extractAdvancedProxyURLPath(targetURL),
		ErrorDetail:              strings.TrimSpace(errorDetail),
		Source:                   source,
		RequestBody:              string(requestBody),
		AntiPoisonPromptPreview:  extractAntiPoisonPromptPreviewFromRequestBody(requestBody),
		UpstreamResponsePreview:  previewAdvancedProxyText(upstreamResponsePreview, 2200),
		UpstreamResponseRaw:      upstreamResponseRaw,
		ResponsePreview:          previewAdvancedProxyText(responsePreview, 2200),
		UpstreamToolCalls:        upstreamToolCalls,
		UpstreamToolArgsPreview:  upstreamToolArgsPreview,
		UpstreamAssistantPreview: previewAdvancedProxyText(upstreamAssistantPreview, 1200),
		UpstreamLatestObserved:   upstreamLatestObserved,
		AntiPoisonOps:            antiPoisonOps,
	}
	appendAdvancedProxyRequestRecord(record)
}

func recordAdvancedProxyClaudeAttempt(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, response map[string]any, rawResponse []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string) {
	recordAdvancedProxyClaudeAttemptWithTrace(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, response, rawResponse, stream, statusCode, elapsed, errorDetail, nil)
}

func recordAdvancedProxyClaudeAttemptWithTrace(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, response map[string]any, rawResponse []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	recordAdvancedProxyClaudeAttemptWithTraceAndOps(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, response, rawResponse, stream, statusCode, elapsed, errorDetail, routeTrace, nil)
}

func recordAdvancedProxyClaudeAttemptWithTraceAndOps(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, response map[string]any, rawResponse []byte, stream bool, statusCode int, elapsed time.Duration, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord) {
	usageInput, usageOutput := extractAdvancedProxyUsageFromMap(response)
	metrics := buildAdvancedProxyRecordedMetrics(elapsed, usageInput, usageOutput)
	resolvedDetail := strings.TrimSpace(errorDetail)
	if resolvedDetail == "" && (statusCode < 200 || statusCode >= 300) {
		resolvedDetail = summarizeAdvancedProxyBody(rawResponse)
	}
	upstreamPreview := summarizeAdvancedProxyBody(rawResponse)
	responsePreview := upstreamPreview
	if response != nil {
		responsePreview = summarizeAdvancedProxyJSON(response, 2200)
	}
	feedback := summarizeAdvancedProxyBodyFeedbackContext(rawResponse, normalizeAdvancedProxyObservedFormat(outboundRoute))
	appendAdvancedProxyClaudeRecord(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, resolvedDetail, upstreamPreview, string(rawResponse), responsePreview, routeTrace, antiPoisonOps, feedback.ToolCalls, feedback.ToolArgsPreview, feedback.AssistantPreview, feedback.LatestObserved)
}

func recordAdvancedProxyClaudeStreamAttempt(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string) {
	recordAdvancedProxyClaudeStreamAttemptWithTrace(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, nil)
}

func recordAdvancedProxyClaudeStreamAttemptWithTrace(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, routeTrace []AdvancedProxyRequestRouteStep) {
	recordAdvancedProxyClaudeStreamAttemptWithTraceAndOps(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, statusCode, stream, startedAt, firstOutputAt, completedAt, inputTokens, outputTokens, errorDetail, "", "", "", routeTrace, nil, nil, nil, "", nil)
}

func recordAdvancedProxyClaudeStreamAttemptWithTraceAndOps(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, statusCode int, stream bool, startedAt time.Time, firstOutputAt *time.Time, completedAt time.Time, inputTokens *int, outputTokens *int, errorDetail string, upstreamResponsePreview string, upstreamResponseRaw string, responsePreview string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord, upstreamToolCalls []string, upstreamToolArgsPreview []string, upstreamAssistantPreview string, upstreamLatestObserved *advancedProxyObservedItem) {
	metrics := buildAdvancedProxyStreamRecordedMetrics(startedAt, firstOutputAt, completedAt, inputTokens, outputTokens)
	appendAdvancedProxyClaudeRecord(appType, inboundEndpoint, outboundRoute, provider, targetURL, requestBody, resolvedModel, stream, statusCode, metrics, errorDetail, upstreamResponsePreview, upstreamResponseRaw, responsePreview, routeTrace, antiPoisonOps, upstreamToolCalls, upstreamToolArgsPreview, upstreamAssistantPreview, upstreamLatestObserved)
}

func appendAdvancedProxyClaudeRecord(appType string, inboundEndpoint string, outboundRoute string, provider AdvancedProxyProvider, targetURL string, requestBody []byte, resolvedModel string, stream bool, statusCode int, metrics advancedProxyRecordedMetrics, errorDetail string, upstreamResponsePreview string, upstreamResponseRaw string, responsePreview string, routeTrace []AdvancedProxyRequestRouteStep, antiPoisonOps []antiPoisonOperationRecord, upstreamToolCalls []string, upstreamToolArgsPreview []string, upstreamAssistantPreview string, upstreamLatestObserved *advancedProxyObservedItem) {
	record := AdvancedProxyRequestRecord{
		RecordedAt:               time.Now().Format(time.RFC3339Nano),
		AppType:                  appType,
		ClientRoute:              "messages",
		InboundEndpoint:          inboundEndpoint,
		OutboundRoute:            outboundRoute,
		RouteTrace:               cloneAdvancedProxyRouteTrace(routeTrace),
		ProviderID:               strings.TrimSpace(provider.ID),
		ProviderRowKey:           strings.TrimSpace(provider.RowKey),
		ProviderName:             advancedProxyProviderLabel(provider),
		ProviderKeyPreview:       maskAdvancedProxyAPIKey(provider.APIKey),
		Model:                    resolveAdvancedProxyRecordedModel(resolvedModel, requestBody, provider.Model),
		Stream:                   stream,
		StatusCode:               statusCode,
		DurationMs:               metrics.DurationMs,
		TTFTMs:                   metrics.TTFTMs,
		LatencyMs:                metrics.LatencyMs,
		InputTokens:              metrics.InputTokens,
		OutputTokens:             metrics.OutputTokens,
		TPS:                      metrics.TPS,
		UpstreamURL:              targetURL,
		UpstreamEndpoint:         extractAdvancedProxyURLPath(targetURL),
		ErrorDetail:              strings.TrimSpace(errorDetail),
		Source:                   "direct",
		RequestBody:              string(requestBody),
		AntiPoisonPromptPreview:  extractAntiPoisonPromptPreviewFromRequestBody(requestBody),
		UpstreamResponsePreview:  previewAdvancedProxyText(upstreamResponsePreview, 2200),
		UpstreamResponseRaw:      upstreamResponseRaw,
		ResponsePreview:          previewAdvancedProxyText(responsePreview, 2200),
		UpstreamToolCalls:        upstreamToolCalls,
		UpstreamToolArgsPreview:  upstreamToolArgsPreview,
		UpstreamAssistantPreview: previewAdvancedProxyText(upstreamAssistantPreview, 1200),
		UpstreamLatestObserved:   upstreamLatestObserved,
		AntiPoisonOps:            antiPoisonOps,
	}
	appendAdvancedProxyRequestRecord(record)
}

func extractAntiPoisonPromptPreviewFromRequestBody(requestBody []byte) string {
	if len(requestBody) == 0 {
		return ""
	}
	var body map[string]any
	if err := json.Unmarshal(requestBody, &body); err != nil {
		return ""
	}
	if instructions := strings.TrimSpace(toStringValue(body["instructions"])); instructions != "" {
		if strings.Contains(instructions, "<important_gateway_rules>") || strings.Contains(instructions, "[AllApiDeck 防投毒随机策略]") {
			return previewAdvancedProxyText(instructions, 2200)
		}
	}
	if messages, ok := body["messages"].([]any); ok {
		for _, rawMessage := range messages {
			message, _ := rawMessage.(map[string]any)
			if !strings.EqualFold(strings.TrimSpace(toStringValue(message["role"])), "system") {
				continue
			}
			content := strings.TrimSpace(toStringValue(message["content"]))
			if strings.Contains(content, "<important_gateway_rules>") || strings.Contains(content, "[AllApiDeck 防投毒随机策略]") {
				return previewAdvancedProxyText(content, 2200)
			}
		}
	}
	if system := body["system"]; system != nil {
		switch typed := system.(type) {
		case string:
			if strings.Contains(typed, "<important_gateway_rules>") || strings.Contains(typed, "[AllApiDeck 防投毒随机策略]") {
				return previewAdvancedProxyText(typed, 2200)
			}
		case []any:
			for _, rawItem := range typed {
				item, _ := rawItem.(map[string]any)
				text := strings.TrimSpace(toStringValue(item["text"]))
				if strings.Contains(text, "<important_gateway_rules>") || strings.Contains(text, "[AllApiDeck 防投毒随机策略]") {
					return previewAdvancedProxyText(text, 2200)
				}
			}
		}
	}
	return ""
}

type advancedProxyBodyFeedbackContext struct {
	ToolCalls        []string
	ToolArgsPreview  []string
	AssistantPreview string
	LatestObserved   *advancedProxyObservedItem
}

func summarizeAdvancedProxyBodyFeedbackContext(raw []byte, observedFormat string) advancedProxyBodyFeedbackContext {
	if len(raw) == 0 {
		return advancedProxyBodyFeedbackContext{}
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return advancedProxyBodyFeedbackContext{}
	}
	switch normalizeAdvancedProxyObservedFormat(observedFormat) {
	case "chat":
		return summarizeAdvancedProxyChatBodyFeedbackContext(body)
	case "responses", "responses_compact":
		return summarizeAdvancedProxyResponsesBodyFeedbackContext(body)
	case "anthropic":
		return summarizeAdvancedProxyAnthropicBodyFeedbackContext(body)
	default:
		if output, ok := body["output"].([]any); ok && len(output) > 0 {
			return summarizeAdvancedProxyResponsesBodyFeedbackContext(body)
		}
		if content, ok := body["content"].([]any); ok && len(content) > 0 {
			return summarizeAdvancedProxyAnthropicBodyFeedbackContext(body)
		}
		if choices, ok := body["choices"].([]any); ok && len(choices) > 0 {
			return summarizeAdvancedProxyChatBodyFeedbackContext(body)
		}
	}
	return advancedProxyBodyFeedbackContext{}
}

func summarizeAdvancedProxyResponsesBodyFeedbackContext(body map[string]any) advancedProxyBodyFeedbackContext {
	toolNames := make([]string, 0, 8)
	toolArgs := make([]string, 0, 8)
	textParts := make([]string, 0, 8)
	var latest *advancedProxyObservedItem
	for _, rawItem := range anySliceValue(body["output"]) {
		item, _ := rawItem.(map[string]any)
		switch strings.TrimSpace(toStringValue(item["type"])) {
		case "function_call":
			name := strings.TrimSpace(toStringValue(item["name"]))
			if name != "" {
				toolNames = append(toolNames, name)
			}
			if arguments := strings.TrimSpace(toStringValue(item["arguments"])); arguments != "" {
				toolArgs = append(toolArgs, arguments)
			}
			latest = &advancedProxyObservedItem{
				Type:             "function_call",
				Name:             name,
				ArgumentsPreview: strings.TrimSpace(toStringValue(item["arguments"])),
				RawPreview:       stringifyJSON(item),
			}
		case "web_search_call":
			toolNames = append(toolNames, "web_search_call")
			action := stringifyJSON(item["action"])
			if strings.TrimSpace(action) != "" {
				toolArgs = append(toolArgs, action)
			}
			latest = &advancedProxyObservedItem{
				Type:             "web_search_call",
				Name:             "web_search_call",
				ArgumentsPreview: action,
				RawPreview:       stringifyJSON(item),
			}
		case "message":
			texts := make([]string, 0, 4)
			for _, rawContent := range anySliceValue(item["content"]) {
				contentMap, _ := rawContent.(map[string]any)
				text := firstNonEmptyExact(
					toStringValue(contentMap["text"]),
					toStringValue(contentMap["content"]),
				)
				text = strings.TrimSpace(text)
				if text != "" {
					textParts = append(textParts, text)
					texts = append(texts, text)
				}
			}
			latest = &advancedProxyObservedItem{
				Type:        "message",
				TextPreview: strings.TrimSpace(strings.Join(texts, " ")),
				RawPreview:  stringifyJSON(item),
			}
		}
	}
	return advancedProxyBodyFeedbackContext{
		ToolCalls:        toolNames,
		ToolArgsPreview:  toolArgs,
		AssistantPreview: previewAdvancedProxyText(strings.Join(textParts, " "), 1200),
		LatestObserved:   latest,
	}
}

func summarizeAdvancedProxyChatBodyFeedbackContext(body map[string]any) advancedProxyBodyFeedbackContext {
	toolNames := make([]string, 0, 8)
	toolArgs := make([]string, 0, 8)
	textParts := make([]string, 0, 8)
	var latest *advancedProxyObservedItem
	for _, rawChoice := range anySliceValue(body["choices"]) {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}
		if content := strings.TrimSpace(toStringValue(message["content"])); content != "" {
			textParts = append(textParts, content)
			latest = &advancedProxyObservedItem{
				Type:        "message",
				TextPreview: content,
				RawPreview:  stringifyJSON(message),
			}
		}
		if toolCalls, ok := message["tool_calls"].([]any); ok {
			for _, rawCall := range toolCalls {
				callMap, _ := rawCall.(map[string]any)
				functionMap, _ := callMap["function"].(map[string]any)
				name := strings.TrimSpace(toStringValue(functionMap["name"]))
				arguments := strings.TrimSpace(toStringValue(functionMap["arguments"]))
				if name != "" {
					toolNames = append(toolNames, name)
				}
				if arguments != "" {
					toolArgs = append(toolArgs, arguments)
				}
				latest = &advancedProxyObservedItem{
					Type:             "tool_call",
					Name:             name,
					ArgumentsPreview: arguments,
					RawPreview:       stringifyJSON(callMap),
				}
			}
		}
		if functionMap, ok := message["function_call"].(map[string]any); ok {
			name := strings.TrimSpace(toStringValue(functionMap["name"]))
			arguments := strings.TrimSpace(toStringValue(functionMap["arguments"]))
			if name != "" {
				toolNames = append(toolNames, name)
			}
			if arguments != "" {
				toolArgs = append(toolArgs, arguments)
			}
			latest = &advancedProxyObservedItem{
				Type:             "function_call",
				Name:             name,
				ArgumentsPreview: arguments,
				RawPreview:       stringifyJSON(functionMap),
			}
		}
	}
	return advancedProxyBodyFeedbackContext{
		ToolCalls:        toolNames,
		ToolArgsPreview:  toolArgs,
		AssistantPreview: previewAdvancedProxyText(strings.Join(textParts, " "), 1200),
		LatestObserved:   latest,
	}
}

func summarizeAdvancedProxyAnthropicBodyFeedbackContext(body map[string]any) advancedProxyBodyFeedbackContext {
	toolNames := make([]string, 0, 8)
	toolArgs := make([]string, 0, 8)
	textParts := make([]string, 0, 8)
	var latest *advancedProxyObservedItem
	for _, rawBlock := range anySliceValue(body["content"]) {
		block, _ := rawBlock.(map[string]any)
		blockType := strings.TrimSpace(toStringValue(block["type"]))
		switch blockType {
		case "tool_use":
			name := strings.TrimSpace(toStringValue(block["name"]))
			inputPreview := stringifyJSON(block["input"])
			if name != "" {
				toolNames = append(toolNames, name)
			}
			if strings.TrimSpace(inputPreview) != "" {
				toolArgs = append(toolArgs, inputPreview)
			}
			latest = &advancedProxyObservedItem{
				Type:             "tool_use",
				Name:             name,
				ArgumentsPreview: inputPreview,
				RawPreview:       stringifyJSON(block),
			}
		case "text":
			text := strings.TrimSpace(toStringValue(block["text"]))
			if text != "" {
				textParts = append(textParts, text)
			}
			latest = &advancedProxyObservedItem{
				Type:        "text",
				TextPreview: text,
				RawPreview:  stringifyJSON(block),
			}
		case "thinking", "redacted_thinking":
			text := strings.TrimSpace(toStringValue(block["thinking"]))
			if text == "" {
				text = strings.TrimSpace(toStringValue(block["text"]))
			}
			if text != "" {
				textParts = append(textParts, text)
			}
			latest = &advancedProxyObservedItem{
				Type:        blockType,
				TextPreview: text,
				RawPreview:  stringifyJSON(block),
			}
		}
	}
	return advancedProxyBodyFeedbackContext{
		ToolCalls:        toolNames,
		ToolArgsPreview:  toolArgs,
		AssistantPreview: previewAdvancedProxyText(strings.Join(textParts, " "), 1200),
		LatestObserved:   latest,
	}
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
