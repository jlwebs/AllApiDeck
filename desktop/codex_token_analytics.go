package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type LocalTokenUsageAnalytics struct {
	CacheReadTokens int64                        `json:"cacheReadTokens"`
	Source          string                       `json:"source"`
	SourceLabel     string                       `json:"sourceLabel"`
	SessionsPath    string                       `json:"sessionsPath"`
	SessionCount    int                          `json:"sessionCount"`
	TotalTokens     int64                        `json:"totalTokens"`
	InputTokens     int64                        `json:"inputTokens"`
	OutputTokens    int64                        `json:"outputTokens"`
	ReasoningTokens int64                        `json:"reasoningTokens"`
	Series          []LocalTokenUsageSeriesPoint `json:"series"`
	Sessions        []LocalTokenUsageSession     `json:"sessions"`
	Sources         []LocalTokenUsageSource      `json:"sources"`
	SessionSeries   []LocalSessionSeriesPoint    `json:"sessionSeries"`
	ToolCalls       []LocalToolCallSeriesPoint   `json:"toolCalls"`
	ToolRanking     []LocalToolRankingItem       `json:"toolRanking"`
	ActiveDays      int                          `json:"activeDays"`
	AvgTurns        float64                      `json:"avgTurns"`
	TotalTurns      int                          `json:"totalTurns"`
	ToolCallCount   int                          `json:"toolCallCount"`
}

type LocalTokenUsageSource struct {
	CacheReadTokens int64  `json:"cacheReadTokens"`
	Source          string `json:"source"`
	SourceLabel     string `json:"sourceLabel"`
	SessionCount    int    `json:"sessionCount"`
	TotalTokens     int64  `json:"totalTokens"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ReasoningTokens int64  `json:"reasoningTokens"`
}

type LocalTokenUsageSeriesPoint struct {
	CacheReadTokens int64                      `json:"cacheReadTokens"`
	AppType         string                     `json:"appType,omitempty"`
	Model           string                     `json:"model,omitempty"`
	ModelCosts      []LocalTokenUsageModelCost `json:"modelCosts,omitempty"`
	Cost            *float64                   `json:"cost,omitempty"`
	Date            string                     `json:"date"`
	Hour            string                     `json:"hour,omitempty"`
	Source          string                     `json:"source"`
	SourceLabel     string                     `json:"sourceLabel"`
	SessionCount    int                        `json:"sessionCount"`
	TotalTokens     int64                      `json:"totalTokens"`
	InputTokens     int64                      `json:"inputTokens"`
	OutputTokens    int64                      `json:"outputTokens"`
	ReasoningTokens int64                      `json:"reasoningTokens"`
}

// LocalTokenUsageModelCost keeps the model-level split for an aggregated
// session or time bucket. Cost is optional because some local session formats
// expose token usage without a reliable pricing catalog.
type LocalTokenUsageModelCost struct {
	Model     string  `json:"model"`
	ModelName string  `json:"modelName,omitempty"`
	Cost      float64 `json:"cost"`
	Tokens    int64   `json:"tokens,omitempty"`
	CostKnown bool    `json:"costKnown"`
}

type LocalTokenUsageSession struct {
	ID              string                     `json:"id"`
	Timestamp       string                     `json:"timestamp"`
	AppType         string                     `json:"appType,omitempty"`
	Source          string                     `json:"source,omitempty"`
	SourceLabel     string                     `json:"sourceLabel,omitempty"`
	Model           string                     `json:"model,omitempty"`
	ModelName       string                     `json:"modelName,omitempty"`
	ModelCosts      []LocalTokenUsageModelCost `json:"modelCosts,omitempty"`
	InputTokens     int64                      `json:"inputTokens"`
	OutputTokens    int64                      `json:"outputTokens"`
	ReasoningTokens int64                      `json:"reasoningTokens"`
	CacheReadTokens int64                      `json:"cacheReadTokens"`
	TotalTokens     int64                      `json:"totalTokens"`
	Cost            *float64                   `json:"cost,omitempty"`
}

type LocalSessionSeriesPoint struct {
	Date         string `json:"date"`
	Source       string `json:"source"`
	SourceLabel  string `json:"sourceLabel"`
	SessionCount int    `json:"sessionCount"`
	TurnCount    int    `json:"turnCount"`
}

type LocalToolCallSeriesPoint struct {
	Date        string `json:"date"`
	Hour        string `json:"hour,omitempty"`
	Source      string `json:"source"`
	SourceLabel string `json:"sourceLabel"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Count       int    `json:"count"`
}

type LocalToolRankingItem struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type codexSessionAnalytics struct {
	StartedAt       time.Time
	UpdatedAt       time.Time
	SessionID       string
	SessionCounted  bool
	TurnCount       int
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CacheReadTokens int64
	TotalTokens     int64
	Model           string
	PricingModel    string
	ModelUsages     map[string]codexModelUsage
	PreviousTotals  *codexTokenUsagePayload
	ToolCounts      map[string]int
}

type codexModelUsage struct {
	InputTokens     int64
	CacheReadTokens int64
	OutputTokens    int64
	ReasoningTokens int64
	TotalTokens     int64
}

type codexSessionJSONLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Payload   struct {
		ID        string `json:"id"`
		SessionID string `json:"session_id"`
		ThreadID  string `json:"thread_id"`
		Type      string `json:"type"`
		Name      string `json:"name"`
		Model     string `json:"model"`
		ModelName string `json:"model_name"`
		Info      struct {
			Model           string                  `json:"model"`
			ModelName       string                  `json:"model_name"`
			TotalTokenUsage *codexTokenUsagePayload `json:"total_token_usage"`
			LastTokenUsage  *codexTokenUsagePayload `json:"last_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

type codexTokenUsagePayload struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

func (a *App) GetLocalTokenUsageAnalytics() (LocalTokenUsageAnalytics, error) {
	sessionsDir := resolveCodexSessionsDir()
	codexSessions, err := collectCodexSessionAnalytics(sessionsDir)
	if err != nil {
		return LocalTokenUsageAnalytics{}, err
	}
	claudeProjectsDir := resolveClaudeProjectsDir()
	claudeSessions, err := collectClaudeSessionAnalytics(claudeProjectsDir)
	if err != nil {
		return LocalTokenUsageAnalytics{}, err
	}

	pricing := defaultCodexPricingCatalog()
	analytics, err := buildLocalTokenUsageAnalyticsWithPricing(sessionsDir, pricing)
	if err != nil {
		return analytics, err
	}
	if analyticsNeedsRemotePricing(analytics) || codexSessionsNeedRemotePricing(codexSessions, pricing) || claudeSessionsNeedRemotePricing(claudeSessions, pricing) {
		pricing = loadCodexPricingCatalog()
		analytics, err = buildLocalTokenUsageAnalyticsWithPricing(sessionsDir, pricing)
		if err != nil {
			return analytics, err
		}
	}
	if err := mergeClaudeLocalTokenUsageAnalyticsWithPricing(&analytics, claudeProjectsDir, pricing); err != nil {
		return analytics, err
	}
	return analytics, nil
}

func codexSessionsNeedRemotePricing(sessions []codexSessionAnalytics, pricing codexPricingCatalog) bool {
	for _, session := range sessions {
		if len(session.ModelUsages) > 0 {
			for model, usage := range session.ModelUsages {
				if usage.TotalTokens > 0 && modelNeedsRemotePricing(model, pricing) {
					return true
				}
			}
			continue
		}
		if session.TotalTokens > 0 && modelNeedsRemotePricing(firstNonEmpty(session.PricingModel, session.Model), pricing) {
			return true
		}
	}
	return false
}

func analyticsNeedsRemotePricing(analytics LocalTokenUsageAnalytics) bool {
	for _, session := range analytics.Sessions {
		model := strings.TrimSpace(session.Model)
		if session.TotalTokens > 0 && model != "" && !isPlaceholderCodexModel(model) && session.Cost == nil {
			return true
		}
	}
	return false
}

func modelNeedsRemotePricing(model string, pricing codexPricingCatalog) bool {
	model = strings.TrimSpace(model)
	if isPlaceholderCodexModel(model) {
		return false
	}
	if isExplicitPricingChannelModel(model) {
		for _, candidate := range codexPricingChannelCandidates(model) {
			if _, ok := pricing[candidate]; ok {
				return false
			}
		}
		return true
	}
	_, ok := pricing.resolve(model)
	return !ok
}

func isExplicitPricingChannelModel(raw string) bool {
	model := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(model, "/") {
		return true
	}
	for _, prefix := range []string{
		"openai.", "anthropic.", "google.", "azure.", "azure-openai.", "xai.", "deepseek.",
		"moonshot.", "moonshotai.", "alibaba.", "zai.", "minimax.", "bedrock.",
		"amazon-bedrock.", "vertex.", "google-vertex.", "global.",
	} {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

func claudeSessionsNeedRemotePricing(sessions []claudeSessionAnalytics, pricing codexPricingCatalog) bool {
	for _, session := range sessions {
		if len(session.ModelUsages) > 0 {
			for model, usage := range session.ModelUsages {
				if usage.TotalTokens > 0 && modelNeedsRemotePricing(model, pricing) {
					return true
				}
			}
			continue
		}
		if session.TotalTokens > 0 && modelNeedsRemotePricing(session.Model, pricing) {
			return true
		}
	}
	return false
}

func buildLocalTokenUsageAnalytics(sessionsDir string) (LocalTokenUsageAnalytics, error) {
	return buildLocalTokenUsageAnalyticsWithPricing(sessionsDir, defaultCodexPricingCatalog())
}

func buildLocalTokenUsageAnalyticsWithPricing(sessionsDir string, pricing codexPricingCatalog) (LocalTokenUsageAnalytics, error) {
	analytics := LocalTokenUsageAnalytics{
		Source:       "codex",
		SourceLabel:  "Codex",
		SessionsPath: sessionsDir,
	}
	if strings.TrimSpace(sessionsDir) == "" {
		return analytics, nil
	}
	sessions, err := collectCodexSessionAnalytics(sessionsDir)
	if err != nil {
		return analytics, err
	}

	tokenSeries := map[string]*LocalTokenUsageSeriesPoint{}
	sessionSeries := map[string]*LocalSessionSeriesPoint{}
	toolSeries := map[string]*LocalToolCallSeriesPoint{}
	toolRanking := map[string]*LocalToolRankingItem{}
	localSessions := make([]LocalTokenUsageSession, 0, len(sessions))
	for _, session := range sessions {
		date := session.UpdatedAt
		if date.IsZero() {
			date = session.StartedAt
		}
		if date.IsZero() {
			date = time.Now()
		}
		localDate := date.Local()
		dayKey := localDate.Format("2006-01-02")
		hourKey := localDate.Format("15")
		if session.SessionCounted {
			point := sessionSeries[dayKey]
			if point == nil {
				point = &LocalSessionSeriesPoint{
					Date:        dayKey,
					Source:      "codex",
					SourceLabel: "Codex",
				}
				sessionSeries[dayKey] = point
			}
			point.SessionCount += 1
			point.TurnCount += session.TurnCount
			analytics.SessionCount += 1
			analytics.TotalTurns += session.TurnCount
		}
		if session.TotalTokens > 0 {
			cost := calculateCodexSessionCost(session, pricing)
			modelCosts := calculateCodexSessionModelCosts(session, pricing)
			model := codexSessionDisplayModel(session)
			localSessions = append(localSessions, LocalTokenUsageSession{
				ID:              session.SessionID,
				Timestamp:       date.UTC().Format(time.RFC3339Nano),
				AppType:         "codex",
				Source:          "codex_session",
				SourceLabel:     "Codex",
				Model:           model,
				ModelName:       codexSessionDisplayName(session, pricing),
				InputTokens:     session.InputTokens,
				OutputTokens:    session.OutputTokens,
				ReasoningTokens: session.ReasoningTokens,
				CacheReadTokens: session.CacheReadTokens,
				TotalTokens:     session.TotalTokens,
				Cost:            cost,
				ModelCosts:      modelCosts,
			})
			seriesKey := dayKey + "-" + hourKey
			point := tokenSeries[seriesKey]
			if point == nil {
				point = &LocalTokenUsageSeriesPoint{
					Date:        dayKey,
					Hour:        hourKey,
					AppType:     "codex",
					Source:      "codex",
					SourceLabel: "Codex",
				}
				tokenSeries[seriesKey] = point
			}
			point.SessionCount += 1
			point.InputTokens += session.InputTokens
			point.OutputTokens += session.OutputTokens
			point.ReasoningTokens += session.ReasoningTokens
			point.CacheReadTokens += session.CacheReadTokens
			point.TotalTokens += session.TotalTokens
			if model := codexSessionDisplayModel(session); model != "" {
				if point.Model == "" {
					point.Model = model
				} else if point.Model != model {
					point.Model = "Multiple models"
				}
			}
			if cost != nil {
				if point.Cost == nil {
					point.Cost = new(float64)
				}
				*point.Cost += *cost
			}
			point.ModelCosts = mergeCodexModelCosts(point.ModelCosts, modelCosts)
			analytics.InputTokens += session.InputTokens
			analytics.OutputTokens += session.OutputTokens
			analytics.ReasoningTokens += session.ReasoningTokens
			analytics.CacheReadTokens += session.CacheReadTokens
			analytics.TotalTokens += session.TotalTokens
		}
		for name, count := range session.ToolCounts {
			if count <= 0 {
				continue
			}
			category := categorizeCodexToolCall(name)
			seriesKey := dayKey + "-" + hourKey + "-" + name
			point := toolSeries[seriesKey]
			if point == nil {
				point = &LocalToolCallSeriesPoint{
					Date:        dayKey,
					Hour:        hourKey,
					Source:      "codex",
					SourceLabel: "Codex",
					Name:        name,
					Category:    category,
				}
				toolSeries[seriesKey] = point
			}
			point.Count += count
			ranking := toolRanking[name]
			if ranking == nil {
				ranking = &LocalToolRankingItem{Name: name, Category: category}
				toolRanking[name] = ranking
			}
			ranking.Count += count
			analytics.ToolCallCount += count
		}
	}

	analytics.ActiveDays = len(sessionSeries)
	sort.Slice(localSessions, func(i, j int) bool { return localSessions[i].Timestamp < localSessions[j].Timestamp })
	analytics.Sessions = localSessions
	if analytics.SessionCount > 0 {
		analytics.AvgTurns = float64(analytics.TotalTurns) / float64(analytics.SessionCount)
	}

	analytics.Series = make([]LocalTokenUsageSeriesPoint, 0, len(tokenSeries))
	for _, point := range tokenSeries {
		analytics.Series = append(analytics.Series, *point)
	}
	sort.Slice(analytics.Series, func(i, j int) bool {
		if analytics.Series[i].Date == analytics.Series[j].Date {
			return analytics.Series[i].Hour < analytics.Series[j].Hour
		}
		return analytics.Series[i].Date < analytics.Series[j].Date
	})

	analytics.SessionSeries = make([]LocalSessionSeriesPoint, 0, len(sessionSeries))
	for _, point := range sessionSeries {
		analytics.SessionSeries = append(analytics.SessionSeries, *point)
	}
	sort.Slice(analytics.SessionSeries, func(i, j int) bool {
		return analytics.SessionSeries[i].Date < analytics.SessionSeries[j].Date
	})

	analytics.ToolCalls = make([]LocalToolCallSeriesPoint, 0, len(toolSeries))
	for _, point := range toolSeries {
		analytics.ToolCalls = append(analytics.ToolCalls, *point)
	}
	sort.Slice(analytics.ToolCalls, func(i, j int) bool {
		if analytics.ToolCalls[i].Date == analytics.ToolCalls[j].Date {
			if analytics.ToolCalls[i].Hour == analytics.ToolCalls[j].Hour {
				return analytics.ToolCalls[i].Name < analytics.ToolCalls[j].Name
			}
			return analytics.ToolCalls[i].Hour < analytics.ToolCalls[j].Hour
		}
		return analytics.ToolCalls[i].Date < analytics.ToolCalls[j].Date
	})

	analytics.ToolRanking = make([]LocalToolRankingItem, 0, len(toolRanking))
	for _, item := range toolRanking {
		analytics.ToolRanking = append(analytics.ToolRanking, *item)
	}
	sort.Slice(analytics.ToolRanking, func(i, j int) bool {
		if analytics.ToolRanking[i].Count == analytics.ToolRanking[j].Count {
			return analytics.ToolRanking[i].Name < analytics.ToolRanking[j].Name
		}
		return analytics.ToolRanking[i].Count > analytics.ToolRanking[j].Count
	})

	if analytics.TotalTokens > 0 {
		analytics.Sources = []LocalTokenUsageSource{{
			Source:          "codex",
			SourceLabel:     "Codex",
			SessionCount:    analytics.SessionCount,
			TotalTokens:     analytics.TotalTokens,
			InputTokens:     analytics.InputTokens,
			OutputTokens:    analytics.OutputTokens,
			ReasoningTokens: analytics.ReasoningTokens,
			CacheReadTokens: analytics.CacheReadTokens,
		}}
	}
	return analytics, nil
}

func collectCodexSessionAnalytics(sessionsDir string) ([]codexSessionAnalytics, error) {
	sessions := []codexSessionAnalytics{}
	directories := []string{sessionsDir}
	if strings.EqualFold(filepath.Base(filepath.Clean(sessionsDir)), "sessions") {
		directories = append(directories, filepath.Join(filepath.Dir(sessionsDir), "archived_sessions"))
	}
	for _, directory := range directories {
		info, statErr := os.Stat(directory)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return sessions, statErr
		}
		if !info.IsDir() {
			continue
		}
		if err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry == nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			usage, ok := readCodexSessionAnalytics(path)
			if ok {
				sessions = append(sessions, usage)
			}
			return nil
		}); err != nil {
			return sessions, err
		}
	}
	return sessions, nil
}

func readCodexSessionAnalytics(path string) (codexSessionAnalytics, bool) {
	file, err := os.Open(path)
	if err != nil {
		return codexSessionAnalytics{}, false
	}
	defer file.Close()

	usage := codexSessionAnalytics{
		SessionID:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		ModelUsages: map[string]codexModelUsage{},
		ToolCounts:  map[string]int{},
	}
	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			readCodexAnalyticsLine(line, &usage)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	if !usage.SessionCounted && usage.TotalTokens <= 0 && usage.TurnCount <= 0 && len(usage.ToolCounts) == 0 {
		return codexSessionAnalytics{}, false
	}
	if usage.UpdatedAt.IsZero() {
		if info, err := file.Stat(); err == nil {
			usage.UpdatedAt = info.ModTime()
		}
	}
	if usage.StartedAt.IsZero() {
		usage.StartedAt = usage.UpdatedAt
	}
	return usage, true
}

func readCodexAnalyticsLine(line string, usage *codexSessionAnalytics) {
	if !strings.Contains(line, `"type"`) {
		return
	}
	var entry codexSessionJSONLine
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}
	timestamp := parseCodexSessionTimestamp(entry.Timestamp)
	if !timestamp.IsZero() {
		if usage.StartedAt.IsZero() {
			usage.StartedAt = timestamp
		}
		usage.UpdatedAt = timestamp
	}
	if entry.Type == "session_meta" {
		usage.SessionCounted = true
		if id := firstCodexModel(entry.Payload.ID, entry.Payload.SessionID, entry.Payload.ThreadID); id != "" {
			usage.SessionID = id
		}
		return
	}
	if entry.Type == "turn_context" {
		if model := firstCodexModel(entry.Payload.Model, entry.Payload.ModelName, entry.Payload.Info.Model, entry.Payload.Info.ModelName); model != "" {
			usage.Model = normalizeCodexModel(model)
			usage.PricingModel = normalizeCodexModelPreservingNamespace(model)
		}
		return
	}
	payloadType := strings.TrimSpace(entry.Payload.Type)
	switch payloadType {
	case "task_started":
		usage.TurnCount += 1
	case "function_call", "custom_tool_call":
		if name := normalizeCodexToolCallName(entry.Payload.Name); name != "" {
			usage.ToolCounts[name] += 1
		}
	case "web_search_call":
		usage.ToolCounts["web_search"] += 1
	case "tool_search_call":
		usage.ToolCounts["tool_search"] += 1
	}
	tokenUsage := entry.Payload.Info.TotalTokenUsage
	isTotal := true
	if tokenUsage == nil {
		tokenUsage = entry.Payload.Info.LastTokenUsage
		isTotal = false
	}
	if tokenUsage == nil {
		return
	}
	if model := firstCodexModel(entry.Payload.Info.Model, entry.Payload.Info.ModelName, entry.Payload.Model, entry.Payload.ModelName); model != "" {
		usage.Model = normalizeCodexModel(model)
		usage.PricingModel = normalizeCodexModelPreservingNamespace(model)
	}
	current := codexModelUsage{
		InputTokens:     tokenUsage.InputTokens,
		CacheReadTokens: codexCachedInputTokens(tokenUsage),
		OutputTokens:    tokenUsage.OutputTokens,
		ReasoningTokens: tokenUsage.ReasoningOutputTokens,
		TotalTokens:     tokenUsage.TotalTokens,
	}
	if current.TotalTokens <= 0 {
		current.TotalTokens = current.InputTokens + current.OutputTokens
	}
	if current.CacheReadTokens > current.InputTokens {
		current.CacheReadTokens = current.InputTokens
	}
	delta := current
	if isTotal {
		if usage.PreviousTotals != nil {
			delta = codexModelUsage{
				InputTokens:     maxInt64(0, current.InputTokens-usage.PreviousTotals.InputTokens),
				CacheReadTokens: maxInt64(0, current.CacheReadTokens-codexCachedInputTokens(usage.PreviousTotals)),
				OutputTokens:    maxInt64(0, current.OutputTokens-usage.PreviousTotals.OutputTokens),
				ReasoningTokens: maxInt64(0, current.ReasoningTokens-usage.PreviousTotals.ReasoningOutputTokens),
				TotalTokens:     maxInt64(0, current.TotalTokens-usage.PreviousTotals.TotalTokens),
			}
		}
		copyOfCurrent := codexTokenUsagePayload{
			InputTokens:           current.InputTokens,
			CachedInputTokens:     current.CacheReadTokens,
			CacheReadInputTokens:  current.CacheReadTokens,
			OutputTokens:          current.OutputTokens,
			ReasoningOutputTokens: current.ReasoningTokens,
			TotalTokens:           current.TotalTokens,
		}
		usage.PreviousTotals = &copyOfCurrent
		usage.InputTokens = tokenUsage.InputTokens
		usage.CacheReadTokens = current.CacheReadTokens
		usage.OutputTokens = tokenUsage.OutputTokens
		usage.ReasoningTokens = tokenUsage.ReasoningOutputTokens
		usage.TotalTokens = current.TotalTokens
	} else {
		usage.InputTokens += delta.InputTokens
		usage.CacheReadTokens += delta.CacheReadTokens
		usage.OutputTokens += delta.OutputTokens
		usage.ReasoningTokens += delta.ReasoningTokens
		usage.TotalTokens += delta.TotalTokens
	}
	if delta.CacheReadTokens > delta.InputTokens {
		delta.CacheReadTokens = delta.InputTokens
	}
	if delta.TotalTokens > 0 || delta.InputTokens > 0 || delta.OutputTokens > 0 {
		model := firstNonEmpty(usage.PricingModel, usage.Model)
		if model == "" {
			model = "unknown"
		}
		modelUsage := usage.ModelUsages[model]
		modelUsage.InputTokens += delta.InputTokens
		modelUsage.CacheReadTokens += delta.CacheReadTokens
		modelUsage.OutputTokens += delta.OutputTokens
		modelUsage.ReasoningTokens += delta.ReasoningTokens
		modelUsage.TotalTokens += delta.TotalTokens
		usage.ModelUsages[model] = modelUsage
	}
}

func codexCachedInputTokens(payload *codexTokenUsagePayload) int64 {
	if payload == nil {
		return 0
	}
	if payload.CachedInputTokens > 0 {
		return payload.CachedInputTokens
	}
	return payload.CacheReadInputTokens
}

func maxInt64(value, floor int64) int64 {
	if value < floor {
		return floor
	}
	return value
}

func firstCodexModel(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// codexModelPricing stores prices in USD per million tokens, matching the
// models.dev/cc-switch convention.
type codexModelPricing struct {
	DisplayName          string
	InputPerMillion      float64
	OutputPerMillion     float64
	CacheReadPerMillion  float64
	CacheWritePerMillion float64
	Source               string
}

type codexPricingCatalog map[string]codexModelPricing

type modelsDevCost struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

type modelsDevModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ReleaseDate string `json:"release_date"`
	Status      string `json:"status"`
	Modalities  struct {
		Output []string `json:"output"`
	} `json:"modalities"`
	Cost *modelsDevCost `json:"cost"`
}

type modelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Models map[string]modelsDevModel `json:"models"`
}

var codexPricingCache struct {
	sync.Mutex
	loadedAt time.Time
	catalog  codexPricingCatalog
}

const (
	modelsDevPricingURL = "https://models.dev/api.json"
	modelsDevPricingTTL = 6 * time.Hour
)

func defaultCodexPricingCatalog() codexPricingCatalog {
	return codexPricingCatalog{
		"gpt-5":               {DisplayName: "GPT-5", InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125, CacheWritePerMillion: 1.25, Source: "builtin"},
		"gpt-5-codex":         {DisplayName: "GPT-5 Codex", InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125, CacheWritePerMillion: 1.25, Source: "builtin"},
		"gpt-5-mini":          {DisplayName: "GPT-5 Mini", InputPerMillion: 0.25, OutputPerMillion: 2, CacheReadPerMillion: 0.025, CacheWritePerMillion: 0.25, Source: "builtin"},
		"gpt-5-nano":          {DisplayName: "GPT-5 Nano", InputPerMillion: 0.05, OutputPerMillion: 0.4, CacheReadPerMillion: 0.005, Source: "builtin"},
		"gpt-5-pro":           {DisplayName: "GPT-5 Pro", InputPerMillion: 15, OutputPerMillion: 120, Source: "builtin"},
		"gpt-5.1":             {DisplayName: "GPT-5.1", InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125, Source: "builtin"},
		"gpt-5.1-codex":       {DisplayName: "GPT-5.1 Codex", InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125, CacheWritePerMillion: 1.25, Source: "builtin"},
		"gpt-5.1-codex-max":   {DisplayName: "GPT-5.1 Codex Max", InputPerMillion: 1.25, OutputPerMillion: 10, CacheReadPerMillion: 0.125, Source: "builtin"},
		"gpt-5.1-codex-mini":  {DisplayName: "GPT-5.1 Codex Mini", InputPerMillion: 0.25, OutputPerMillion: 2, CacheReadPerMillion: 0.025, Source: "builtin"},
		"gpt-5.2":             {DisplayName: "GPT-5.2", InputPerMillion: 1.75, OutputPerMillion: 14, CacheReadPerMillion: 0.175, CacheWritePerMillion: 1.75, Source: "builtin"},
		"gpt-5.2-codex":       {DisplayName: "GPT-5.2 Codex", InputPerMillion: 1.75, OutputPerMillion: 14, CacheReadPerMillion: 0.175, CacheWritePerMillion: 1.75, Source: "builtin"},
		"gpt-5.2-pro":         {DisplayName: "GPT-5.2 Pro", InputPerMillion: 21, OutputPerMillion: 168, Source: "builtin"},
		"gpt-5.3-codex":       {DisplayName: "GPT-5.3 Codex", InputPerMillion: 1.75, OutputPerMillion: 14, CacheReadPerMillion: 0.175, Source: "builtin"},
		"gpt-5.3-codex-spark": {DisplayName: "GPT-5.3 Codex Spark", InputPerMillion: 1.75, OutputPerMillion: 14, CacheReadPerMillion: 0.175, Source: "builtin"},
		"gpt-5.4":             {DisplayName: "GPT-5.4", InputPerMillion: 2.5, OutputPerMillion: 15, CacheReadPerMillion: 0.25, CacheWritePerMillion: 2.5, Source: "builtin"},
		"gpt-5.4-mini":        {DisplayName: "GPT-5.4 Mini", InputPerMillion: 0.75, OutputPerMillion: 4.5, CacheReadPerMillion: 0.075, CacheWritePerMillion: 0.75, Source: "builtin"},
		"gpt-5.4-nano":        {DisplayName: "GPT-5.4 Nano", InputPerMillion: 0.2, OutputPerMillion: 1.25, CacheReadPerMillion: 0.02, Source: "builtin"},
		"gpt-5.4-pro":         {DisplayName: "GPT-5.4 Pro", InputPerMillion: 30, OutputPerMillion: 180, Source: "builtin"},
		"gpt-5.5":             {DisplayName: "GPT-5.5", InputPerMillion: 5, OutputPerMillion: 30, CacheReadPerMillion: 0.5, CacheWritePerMillion: 5, Source: "builtin"},
		"gpt-5.5-pro":         {DisplayName: "GPT-5.5 Pro", InputPerMillion: 30, OutputPerMillion: 180, Source: "builtin"},
		"gpt-5.6":             {DisplayName: "GPT-5.6", InputPerMillion: 5, OutputPerMillion: 30, CacheReadPerMillion: 0.5, CacheWritePerMillion: 6.25, Source: "builtin"},
		"gpt-5.6-luna":        {DisplayName: "GPT-5.6 Luna", InputPerMillion: 0.2, OutputPerMillion: 1.2, CacheReadPerMillion: 0.02, CacheWritePerMillion: 0.25, Source: "builtin"},
		"gpt-5.6-sol":         {DisplayName: "GPT-5.6 Sol", InputPerMillion: 5, OutputPerMillion: 30, CacheReadPerMillion: 0.5, CacheWritePerMillion: 6.25, Source: "builtin"},
		"gpt-5.6-terra":       {DisplayName: "GPT-5.6 Terra", InputPerMillion: 2, OutputPerMillion: 12, CacheReadPerMillion: 0.2, CacheWritePerMillion: 2.5, Source: "builtin"},
		"claude-haiku-4-5":    {DisplayName: "Claude Haiku 4.5", InputPerMillion: 1, OutputPerMillion: 5, CacheReadPerMillion: 0.1, CacheWritePerMillion: 1.25, Source: "builtin"},
		"claude-sonnet-4":     {DisplayName: "Claude Sonnet 4", InputPerMillion: 3, OutputPerMillion: 15, CacheReadPerMillion: 0.3, CacheWritePerMillion: 3.75, Source: "builtin"},
		"claude-sonnet-4-5":   {DisplayName: "Claude Sonnet 4.5", InputPerMillion: 3, OutputPerMillion: 15, CacheReadPerMillion: 0.3, CacheWritePerMillion: 3.75, Source: "builtin"},
		"claude-sonnet-4-6":   {DisplayName: "Claude Sonnet 4.6", InputPerMillion: 3, OutputPerMillion: 15, CacheReadPerMillion: 0.3, CacheWritePerMillion: 3.75, Source: "builtin"},
		"claude-opus-4":       {DisplayName: "Claude Opus 4", InputPerMillion: 15, OutputPerMillion: 75, CacheReadPerMillion: 1.5, CacheWritePerMillion: 18.75, Source: "builtin"},
		"claude-opus-4-5":     {DisplayName: "Claude Opus 4.5", InputPerMillion: 5, OutputPerMillion: 25, CacheReadPerMillion: 0.5, CacheWritePerMillion: 6.25, Source: "builtin"},
		"claude-opus-4-6":     {DisplayName: "Claude Opus 4.6", InputPerMillion: 5, OutputPerMillion: 25, CacheReadPerMillion: 0.5, CacheWritePerMillion: 6.25, Source: "builtin"},
	}
}

func loadCodexPricingCatalog() codexPricingCatalog {
	codexPricingCache.Lock()
	if codexPricingCache.catalog != nil && time.Since(codexPricingCache.loadedAt) < modelsDevPricingTTL {
		catalog := codexPricingCache.catalog
		codexPricingCache.Unlock()
		return catalog
	}
	codexPricingCache.Unlock()

	catalog := defaultCodexPricingCatalog()
	if client, err := newOutboundHTTPClient(4 * time.Second); err == nil {
		if request, err := http.NewRequest(http.MethodGet, modelsDevPricingURL, nil); err == nil {
			if response, err := client.Do(request); err == nil {
				if response.Body != nil {
					defer response.Body.Close()
				}
				if response.StatusCode >= 200 && response.StatusCode < 300 {
					var payload map[string]modelsDevProvider
					if json.NewDecoder(response.Body).Decode(&payload) == nil {
						mergeModelsDevPricing(catalog, payload)
					}
				}
			}
		}
	}

	codexPricingCache.Lock()
	codexPricingCache.catalog = catalog
	codexPricingCache.loadedAt = time.Now()
	codexPricingCache.Unlock()
	return catalog
}

func mergeModelsDevPricing(catalog codexPricingCatalog, payload map[string]modelsDevProvider) {
	for providerID, provider := range payload {
		providerID = normalizePricingChannel(firstNonEmpty(strings.TrimSpace(provider.ID), providerID))
		for modelID, model := range provider.Models {
			if !isTextCodexPricingModel(modelID, model) {
				continue
			}
			if model.Cost == nil || (model.Cost.Input == nil && model.Cost.Output == nil) {
				continue
			}
			normalized := normalizeCodexModel(modelID)
			if normalized == "" {
				continue
			}
			candidate := codexModelPricing{DisplayName: strings.TrimSpace(model.Name), Source: providerID}
			if model.Cost.Input != nil {
				candidate.InputPerMillion = *model.Cost.Input
			}
			if model.Cost.Output != nil {
				candidate.OutputPerMillion = *model.Cost.Output
			}
			if model.Cost.CacheRead != nil {
				candidate.CacheReadPerMillion = *model.Cost.CacheRead
			}
			if model.Cost.CacheWrite != nil {
				candidate.CacheWritePerMillion = *model.Cost.CacheWrite
			}
			for _, channel := range pricingChannelAliases(providerID) {
				if channelKey := pricingChannelModelKey(channel, modelID, normalized); channelKey != "" {
					mergeCodexPricingEntry(catalog, channelKey, candidate)
				}
			}
			mergeCodexPricingEntry(catalog, normalized, candidate)
		}
	}
}

func normalizePricingChannel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func pricingChannelAliases(channel string) []string {
	normalized := normalizePricingChannel(channel)
	if normalized == "" {
		return nil
	}
	aliases := []string{normalized}
	switch normalized {
	case "amazon-bedrock":
		aliases = append(aliases, "bedrock", "aws")
	case "google-vertex":
		aliases = append(aliases, "vertex")
	case "azure":
		aliases = append(aliases, "azure-openai")
	}
	return aliases
}

func normalizeCodexModelPreservingNamespace(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		name = name[:colon]
	}
	name = strings.ReplaceAll(name, "@", "-")
	name = strings.TrimSpace(strings.TrimSuffix(name, "[1m]"))
	return name
}

func pricingChannelModelKey(providerID, modelID, normalized string) string {
	channel := normalizePricingChannel(providerID)
	model := normalizeCodexModelPreservingNamespace(modelID)
	if model == "" {
		model = normalized
	}
	if channel == "" || model == "" {
		return ""
	}
	return channel + "/" + model
}

func pricingProviderRank(source string) int {
	switch normalizePricingChannel(source) {
	case "openai":
		return 0
	case "anthropic":
		return 1
	case "google":
		return 2
	case "xai":
		return 3
	case "deepseek":
		return 4
	case "moonshotai":
		return 5
	case "alibaba", "alibaba-cn":
		return 6
	case "zai":
		return 7
	case "minimax", "minimax-cn":
		return 8
	case "builtin":
		return 1000
	default:
		return 100
	}
}

func mergeCodexPricingEntry(catalog codexPricingCatalog, key string, candidate codexModelPricing) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return
	}
	existing, ok := catalog[key]
	if candidate.DisplayName == "" && ok {
		candidate.DisplayName = existing.DisplayName
	}
	candidateRank := pricingProviderRank(candidate.Source)
	existingRank := pricingProviderRank(existing.Source)
	if !ok || candidateRank < existingRank || (candidateRank == existingRank && candidate.Source < existing.Source) {
		catalog[key] = candidate
	}
}

func isTextCodexPricingModel(modelID string, model modelsDevModel) bool {
	if strings.EqualFold(strings.TrimSpace(model.Status), "deprecated") {
		return false
	}
	searchable := strings.ToLower(modelID + " " + model.Name)
	for _, marker := range []string{"audio", "embedding", "image", "moderation", "realtime", "transcribe", "tts", "video"} {
		if strings.Contains(searchable, marker) {
			return false
		}
	}
	if len(model.Modalities.Output) > 0 {
		containsText := false
		for _, modality := range model.Modalities.Output {
			normalized := strings.ToLower(strings.TrimSpace(modality))
			if normalized == "text" {
				containsText = true
			}
			if normalized == "audio" || normalized == "image" || normalized == "video" {
				return false
			}
		}
		if !containsText {
			return false
		}
	}
	return true
}

func (catalog codexPricingCatalog) resolve(model string) (codexModelPricing, bool) {
	for _, candidate := range codexPricingChannelCandidates(model) {
		if pricing, ok := catalog[candidate]; ok {
			return pricing, true
		}
	}
	for _, candidate := range codexModelPricingCandidates(model) {
		if pricing, ok := catalog[candidate]; ok {
			return pricing, true
		}
	}
	return codexModelPricing{}, false
}

func codexPricingChannelCandidates(raw string) []string {
	cleaned := normalizeCodexModelPreservingNamespace(raw)
	if isPlaceholderCodexModel(cleaned) {
		return nil
	}
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 5)
	appendCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	appendCandidate(cleaned)
	channel, explicit := pricingChannelFromModel(cleaned)
	if !explicit {
		return candidates
	}
	model := normalizeCodexPricingModel(raw)
	if model == "" {
		return candidates
	}
	for _, alias := range pricingChannelAliases(channel) {
		appendCandidate(alias + "/" + model)
	}
	return candidates
}

func pricingChannelFromModel(model string) (string, bool) {
	model = normalizePricingChannel(model)
	if slash := strings.IndexByte(model, '/'); slash > 0 {
		return model[:slash], true
	}
	for _, prefix := range []struct {
		prefix  string
		channel string
	}{
		{"global.openai.", "openai"},
		{"openai.", "openai"},
		{"anthropic.", "anthropic"},
		{"google.", "google"},
		{"azure.", "azure"},
		{"azure-openai.", "azure"},
		{"xai.", "xai"},
		{"deepseek.", "deepseek"},
		{"moonshot.", "moonshotai"},
		{"moonshotai.", "moonshotai"},
		{"alibaba.", "alibaba"},
		{"zai.", "zai"},
		{"minimax.", "minimax"},
		{"bedrock.", "amazon-bedrock"},
		{"amazon-bedrock.", "amazon-bedrock"},
		{"vertex.", "google-vertex"},
		{"google-vertex.", "google-vertex"},
	} {
		if strings.HasPrefix(model, prefix.prefix) {
			return prefix.channel, true
		}
	}
	return "", false
}

func normalizeCodexPricingModel(raw string) string {
	model := normalizeCodexModel(raw)
	for {
		next := firstNonEmpty(
			stripKnownCodexNamespace(model),
			stripCodexModelDateSuffix(model),
			stripCodexReasoningSuffix(model),
		)
		if next == "" || next == model {
			return model
		}
		model = next
	}
}

func codexSessionDisplayModel(session codexSessionAnalytics) string {
	if len(session.ModelUsages) > 1 {
		return "Multiple models"
	}
	for model := range session.ModelUsages {
		if model != "unknown" {
			return normalizeCodexModel(model)
		}
	}
	return session.Model
}

func codexSessionDisplayName(session codexSessionAnalytics, catalog codexPricingCatalog) string {
	if len(session.ModelUsages) > 1 {
		return "Multiple models"
	}
	for model := range session.ModelUsages {
		if model == "unknown" {
			continue
		}
		if pricing, ok := catalog.resolve(model); ok && strings.TrimSpace(pricing.DisplayName) != "" {
			return pricing.DisplayName
		}
		return model
	}
	if pricing, ok := catalog.resolve(session.Model); ok && strings.TrimSpace(pricing.DisplayName) != "" {
		return pricing.DisplayName
	}
	return session.Model
}

func codexModelDisplayName(model string, pricing codexModelPricing) string {
	if displayName := strings.TrimSpace(pricing.DisplayName); displayName != "" {
		return displayName
	}
	return model
}

func calculateCodexSessionModelCosts(session codexSessionAnalytics, catalog codexPricingCatalog) []LocalTokenUsageModelCost {
	if session.TotalTokens <= 0 {
		return nil
	}
	if len(session.ModelUsages) == 0 {
		pricing, ok := catalog.resolve(session.Model)
		if !ok {
			return nil
		}
		inputTokens := maxInt64(0, session.InputTokens-session.CacheReadTokens)
		cost := (float64(inputTokens)*pricing.InputPerMillion +
			float64(session.CacheReadTokens)*pricing.CacheReadPerMillion +
			float64(session.OutputTokens)*pricing.OutputPerMillion) / 1_000_000
		return []LocalTokenUsageModelCost{{
			Model:     normalizeCodexModel(session.Model),
			ModelName: codexModelDisplayName(normalizeCodexModel(session.Model), pricing),
			Cost:      cost,
			Tokens:    session.TotalTokens,
			CostKnown: true,
		}}
	}

	modelCosts := make([]LocalTokenUsageModelCost, 0, len(session.ModelUsages))
	for model, usage := range session.ModelUsages {
		pricing, ok := catalog.resolve(model)
		if !ok {
			return nil
		}
		inputTokens := maxInt64(0, usage.InputTokens-usage.CacheReadTokens)
		cost := (float64(inputTokens)*pricing.InputPerMillion +
			float64(usage.CacheReadTokens)*pricing.CacheReadPerMillion +
			float64(usage.OutputTokens)*pricing.OutputPerMillion) / 1_000_000
		modelCosts = append(modelCosts, LocalTokenUsageModelCost{
			Model:     normalizeCodexModel(model),
			ModelName: codexModelDisplayName(normalizeCodexModel(model), pricing),
			Cost:      cost,
			Tokens:    usage.TotalTokens,
			CostKnown: true,
		})
	}
	sort.SliceStable(modelCosts, func(left, right int) bool {
		if modelCosts[left].Cost != modelCosts[right].Cost {
			return modelCosts[left].Cost > modelCosts[right].Cost
		}
		return modelCosts[left].Model < modelCosts[right].Model
	})
	return modelCosts
}

func mergeCodexModelCosts(existing, additions []LocalTokenUsageModelCost) []LocalTokenUsageModelCost {
	if len(additions) == 0 {
		return existing
	}
	merged := make(map[string]LocalTokenUsageModelCost, len(existing)+len(additions))
	for _, item := range existing {
		key := normalizeCodexModel(item.Model)
		if key == "" {
			continue
		}
		item.Model = key
		if strings.TrimSpace(item.ModelName) == "" {
			item.ModelName = key
		}
		merged[key] = item
	}
	for _, item := range additions {
		key := normalizeCodexModel(item.Model)
		if key == "" {
			continue
		}
		item.Model = key
		if strings.TrimSpace(item.ModelName) == "" {
			item.ModelName = key
		}
		current := merged[key]
		current.Model = key
		current.ModelName = item.ModelName
		current.Cost += item.Cost
		current.Tokens += item.Tokens
		current.CostKnown = current.CostKnown || item.CostKnown
		merged[key] = current
	}
	result := make([]LocalTokenUsageModelCost, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Cost != result[right].Cost {
			return result[left].Cost > result[right].Cost
		}
		return result[left].Model < result[right].Model
	})
	return result
}

func calculateCodexSessionCost(session codexSessionAnalytics, catalog codexPricingCatalog) *float64 {
	modelCosts := calculateCodexSessionModelCosts(session, catalog)
	if len(modelCosts) == 0 {
		return nil
	}
	var totalCost float64
	for _, modelCost := range modelCosts {
		totalCost += modelCost.Cost
	}
	return &totalCost
}

func isPlaceholderCodexModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "", "unknown", "null", "none":
		return true
	default:
		return false
	}
}

func codexModelPricingCandidates(raw string) []string {
	cleaned := normalizeCodexModel(raw)
	if isPlaceholderCodexModel(cleaned) {
		return nil
	}
	seen := map[string]struct{}{}
	queue := []string{cleaned}
	candidates := make([]string, 0, 8)
	for len(queue) > 0 {
		candidate := queue[0]
		queue = queue[1:]
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
		if stripped := stripKnownCodexNamespace(candidate); stripped != "" {
			queue = append(queue, stripped)
		}
		if stripped := stripCodexModelDateSuffix(candidate); stripped != "" {
			queue = append(queue, stripped)
		}
		if stripped := stripCodexReasoningSuffix(candidate); stripped != "" {
			queue = append(queue, stripped)
		}
		if strings.HasPrefix(candidate, "claude-") && strings.Contains(candidate, ".") {
			queue = append(queue, strings.ReplaceAll(candidate, ".", "-"))
		}
		if base, suffix, ok := strings.Cut(candidate, "-v"); ok && base != "" && suffix != "" && allASCIIDigits(suffix) {
			queue = append(queue, base)
		}
	}
	return candidates
}

func stripKnownCodexNamespace(model string) string {
	if index := strings.LastIndex(model, "claude-"); index > 0 {
		return model[index:]
	}
	for _, marker := range []string{"openai.", "anthropic.", "google.", "moonshot.", "moonshotai.", "bedrock.", "global."} {
		if strings.HasPrefix(model, marker) {
			return strings.TrimPrefix(model, marker)
		}
	}
	if strings.HasPrefix(model, "claude-") {
		rest := strings.TrimPrefix(model, "claude-")
		for _, marker := range []string{"abab", "ark-code", "arctic", "astron", "codex", "command-r", "deepseek", "doubao", "ernie", "gemini", "gemma", "glm", "gpt", "grok", "hermes", "hy3", "hunyuan", "jamba", "kimi", "llama", "longcat", "mercury", "mimo", "minimax", "mistral", "mixtral", "moonshot", "nemotron", "nova-", "openai", "qianfan", "qwen", "seed-", "solar", "stepfun"} {
			if strings.HasPrefix(rest, marker) {
				return rest
			}
		}
	}
	return ""
}

func stripCodexModelDateSuffix(model string) string {
	if len(model) > 11 {
		suffix := model[len(model)-11:]
		if suffix[0] == '-' && suffix[5] == '-' && suffix[8] == '-' && allASCIIDigits(suffix[1:5]) && allASCIIDigits(suffix[6:8]) && allASCIIDigits(suffix[9:11]) {
			return model[:len(model)-11]
		}
	}
	lastDash := strings.LastIndexByte(model, '-')
	if lastDash <= 0 || lastDash == len(model)-1 {
		return ""
	}
	base := model[:lastDash]
	suffix := model[lastDash+1:]
	if len(suffix) == 8 && allASCIIDigits(suffix) {
		return base
	}
	if len(suffix) == 6 && allASCIIDigits(suffix) {
		month := 0
		day := 0
		for _, digit := range suffix[2:4] {
			month = month*10 + int(digit-'0')
		}
		for _, digit := range suffix[4:6] {
			day = day*10 + int(digit-'0')
		}
		if month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			return base
		}
	}
	return ""
}

func stripCodexReasoningSuffix(model string) string {
	for _, suffix := range []string{"-minimal", "-low", "-medium", "-high", "-xhigh"} {
		if stripped, ok := strings.CutSuffix(model, suffix); ok && stripped != "" {
			return stripped
		}
	}
	return ""
}

// normalizeCodexModel mirrors cc-switch's matching rules: remove provider
// prefixes, route suffixes, and dated snapshots before looking up pricing.
func normalizeCodexModel(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		name = name[:colon]
	}
	name = strings.ReplaceAll(name, "@", "-")
	name = strings.TrimSpace(strings.TrimSuffix(name, "[1m]"))
	if len(name) > 11 {
		suffix := name[len(name)-11:]
		if suffix[0] == '-' && suffix[5] == '-' && suffix[8] == '-' &&
			allASCIIDigits(suffix[1:5]) && allASCIIDigits(suffix[6:8]) && allASCIIDigits(suffix[9:11]) {
			name = name[:len(name)-11]
		}
	}
	if len(name) > 9 {
		suffix := name[len(name)-8:]
		if len(name) > 9 && name[len(name)-9] == '-' && allASCIIDigits(suffix) {
			name = name[:len(name)-9]
		}
	}
	return strings.TrimSpace(name)
}

func allASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func normalizeCodexToolCallName(name string) string {
	text := strings.TrimSpace(name)
	if text == "" {
		return ""
	}
	if strings.Contains(text, ".") {
		parts := strings.Split(text, ".")
		text = parts[len(parts)-1]
	}
	return strings.TrimSpace(text)
}

func categorizeCodexToolCall(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case normalized == "web_search" || normalized == "tool_search" || strings.Contains(normalized, "search") || strings.Contains(normalized, "find"):
		return "search"
	case normalized == "apply_patch" || strings.Contains(normalized, "edit") || strings.Contains(normalized, "write") || strings.Contains(normalized, "update"):
		return "edit"
	default:
		return "other"
	}
}

func parseCodexSessionTimestamp(value string) time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15-04-05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func resolveCodexSessionsDir() string {
	home := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".codex")
		}
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, "sessions")
}
