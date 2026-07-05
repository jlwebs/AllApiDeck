package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LocalTokenUsageAnalytics struct {
	Source          string                       `json:"source"`
	SourceLabel     string                       `json:"sourceLabel"`
	SessionsPath    string                       `json:"sessionsPath"`
	SessionCount    int                          `json:"sessionCount"`
	TotalTokens     int64                        `json:"totalTokens"`
	InputTokens     int64                        `json:"inputTokens"`
	OutputTokens    int64                        `json:"outputTokens"`
	ReasoningTokens int64                        `json:"reasoningTokens"`
	Series          []LocalTokenUsageSeriesPoint `json:"series"`
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
	Source          string `json:"source"`
	SourceLabel     string `json:"sourceLabel"`
	SessionCount    int    `json:"sessionCount"`
	TotalTokens     int64  `json:"totalTokens"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ReasoningTokens int64  `json:"reasoningTokens"`
}

type LocalTokenUsageSeriesPoint struct {
	Date            string `json:"date"`
	Hour            string `json:"hour,omitempty"`
	Source          string `json:"source"`
	SourceLabel     string `json:"sourceLabel"`
	SessionCount    int    `json:"sessionCount"`
	TotalTokens     int64  `json:"totalTokens"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ReasoningTokens int64  `json:"reasoningTokens"`
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
	SessionCounted  bool
	TurnCount       int
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	TotalTokens     int64
	ToolCounts      map[string]int
}

type codexSessionJSONLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Payload   struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Info struct {
			TotalTokenUsage *codexTokenUsagePayload `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

type codexTokenUsagePayload struct {
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

func (a *App) GetLocalTokenUsageAnalytics() (LocalTokenUsageAnalytics, error) {
	return buildLocalTokenUsageAnalytics(resolveCodexSessionsDir())
}

func buildLocalTokenUsageAnalytics(sessionsDir string) (LocalTokenUsageAnalytics, error) {
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
			seriesKey := dayKey + "-" + hourKey
			point := tokenSeries[seriesKey]
			if point == nil {
				point = &LocalTokenUsageSeriesPoint{
					Date:        dayKey,
					Hour:        hourKey,
					Source:      "codex",
					SourceLabel: "Codex",
				}
				tokenSeries[seriesKey] = point
			}
			point.SessionCount += 1
			point.InputTokens += session.InputTokens
			point.OutputTokens += session.OutputTokens
			point.ReasoningTokens += session.ReasoningTokens
			point.TotalTokens += session.TotalTokens
			analytics.InputTokens += session.InputTokens
			analytics.OutputTokens += session.OutputTokens
			analytics.ReasoningTokens += session.ReasoningTokens
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
		}}
	}
	return analytics, nil
}

func collectCodexSessionAnalytics(sessionsDir string) ([]codexSessionAnalytics, error) {
	sessions := []codexSessionAnalytics{}
	err := filepath.WalkDir(sessionsDir, func(path string, entry os.DirEntry, walkErr error) error {
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
	})
	return sessions, err
}

func readCodexSessionAnalytics(path string) (codexSessionAnalytics, bool) {
	file, err := os.Open(path)
	if err != nil {
		return codexSessionAnalytics{}, false
	}
	defer file.Close()

	usage := codexSessionAnalytics{ToolCounts: map[string]int{}}
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
	if tokenUsage == nil || tokenUsage.TotalTokens <= 0 {
		return
	}
	usage.InputTokens = tokenUsage.InputTokens
	usage.OutputTokens = tokenUsage.OutputTokens
	usage.ReasoningTokens = tokenUsage.ReasoningOutputTokens
	usage.TotalTokens = tokenUsage.TotalTokens
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
