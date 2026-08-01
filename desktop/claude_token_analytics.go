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

const (
	claudeLocalSource      = "claude"
	claudeLocalSourceLabel = "Claude"
)

type claudeSessionAnalytics struct {
	StartedAt       time.Time
	UpdatedAt       time.Time
	SessionID       string
	InputTokens     int64
	CacheReadTokens int64
	OutputTokens    int64
	TotalTokens     int64
	Model           string
	ModelUsages     map[string]claudeModelUsage
	TurnCount       int
}

type claudeModelUsage struct {
	InputTokens     int64
	CacheReadTokens int64
	OutputTokens    int64
	TotalTokens     int64
}

type claudeSessionJSONLine struct {
	Type        string         `json:"type"`
	Timestamp   string         `json:"timestamp"`
	SessionID   string         `json:"sessionId"`
	SessionID2  string         `json:"session_id"`
	UUID        string         `json:"uuid"`
	IsSidechain bool           `json:"isSidechain"`
	Message     *claudeMessage `json:"message"`
}

type claudeMessage struct {
	ID         string            `json:"id"`
	Role       string            `json:"role"`
	Model      string            `json:"model"`
	StopReason string            `json:"stop_reason"`
	Usage      *claudeTokenUsage `json:"usage"`
}

type claudeTokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
}

func resolveClaudeProjectsDir() string {
	root := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if root == "" {
		if home, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(home, ".claude")
		}
	}
	if root == "" {
		return ""
	}
	if strings.EqualFold(filepath.Base(filepath.Clean(root)), "projects") {
		return root
	}
	return filepath.Join(root, "projects")
}

func collectClaudeSessionAnalytics(projectsDir string) ([]claudeSessionAnalytics, error) {
	sessions := []claudeSessionAnalytics{}
	projectsDir = strings.TrimSpace(projectsDir)
	if projectsDir == "" {
		return sessions, nil
	}
	info, statErr := os.Stat(projectsDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return sessions, nil
		}
		return sessions, statErr
	}
	if !info.IsDir() {
		return sessions, nil
	}

	err := filepath.WalkDir(projectsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry == nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
			return nil
		}
		usage, ok := readClaudeSessionAnalytics(path)
		if ok {
			sessions = append(sessions, usage)
		}
		return nil
	})
	return sessions, err
}

func readClaudeSessionAnalytics(path string) (claudeSessionAnalytics, bool) {
	file, err := os.Open(path)
	if err != nil {
		return claudeSessionAnalytics{}, false
	}
	defer file.Close()

	usage := claudeSessionAnalytics{
		SessionID:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		ModelUsages: map[string]claudeModelUsage{},
	}
	seenMessages := map[string]struct{}{}
	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			readClaudeAnalyticsLine(line, &usage, seenMessages)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	if usage.TotalTokens <= 0 {
		return claudeSessionAnalytics{}, false
	}
	if usage.UpdatedAt.IsZero() {
		if info, statErr := file.Stat(); statErr == nil {
			usage.UpdatedAt = info.ModTime()
		}
	}
	if usage.StartedAt.IsZero() {
		usage.StartedAt = usage.UpdatedAt
	}
	return usage, true
}

func readClaudeAnalyticsLine(line string, usage *claudeSessionAnalytics, seenMessages map[string]struct{}) {
	if usage == nil || !strings.Contains(line, `"assistant"`) || !strings.Contains(line, `"usage"`) {
		return
	}
	var entry claudeSessionJSONLine
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return
	}
	if timestamp := parseClaudeSessionTimestamp(entry.Timestamp); !timestamp.IsZero() {
		if usage.StartedAt.IsZero() {
			usage.StartedAt = timestamp
		}
		usage.UpdatedAt = timestamp
	}
	if sessionID := firstNonEmpty(strings.TrimSpace(entry.SessionID), strings.TrimSpace(entry.SessionID2)); sessionID != "" {
		usage.SessionID = sessionID
	}
	if entry.Type != "assistant" || entry.Message == nil || entry.Message.Usage == nil {
		return
	}
	if role := strings.TrimSpace(entry.Message.Role); role != "" && !strings.EqualFold(role, "assistant") {
		return
	}
	model := strings.TrimSpace(entry.Message.Model)
	if model == "" || strings.EqualFold(model, "<synthetic>") {
		return
	}

	messageKey := strings.TrimSpace(entry.Message.ID)
	if messageKey == "" {
		messageKey = strings.TrimSpace(entry.UUID)
	}
	if messageKey != "" {
		if _, exists := seenMessages[messageKey]; exists {
			return
		}
		seenMessages[messageKey] = struct{}{}
	}

	inputTokens := maxInt64(0, entry.Message.Usage.InputTokens)
	cacheCreationTokens := maxInt64(0, entry.Message.Usage.CacheCreationInputTokens)
	cacheReadTokens := maxInt64(0, entry.Message.Usage.CacheReadInputTokens)
	outputTokens := maxInt64(0, entry.Message.Usage.OutputTokens)
	inputTokens += cacheCreationTokens
	if inputTokens == 0 && cacheReadTokens == 0 && outputTokens == 0 {
		return
	}

	usage.InputTokens += inputTokens
	usage.CacheReadTokens += cacheReadTokens
	usage.OutputTokens += outputTokens
	usage.TotalTokens += inputTokens + cacheReadTokens + outputTokens
	modelUsage := usage.ModelUsages[model]
	modelUsage.InputTokens += inputTokens
	modelUsage.CacheReadTokens += cacheReadTokens
	modelUsage.OutputTokens += outputTokens
	modelUsage.TotalTokens += inputTokens + cacheReadTokens + outputTokens
	usage.ModelUsages[model] = modelUsage
	usage.TurnCount++
	if usage.Model == "" {
		usage.Model = model
	} else if usage.Model != model {
		usage.Model = "Multiple models"
	}
}

func parseClaudeSessionTimestamp(value string) time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05Z07:00"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func mergeClaudeLocalTokenUsageAnalytics(analytics *LocalTokenUsageAnalytics, projectsDir string) error {
	if analytics == nil {
		return nil
	}
	sessions, err := collectClaudeSessionAnalytics(projectsDir)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}

	for _, session := range sessions {
		date := session.UpdatedAt
		if date.IsZero() {
			date = session.StartedAt
		}
		if date.IsZero() {
			date = time.Now()
		}
		analytics.Sessions = append(analytics.Sessions, LocalTokenUsageSession{
			ID:              session.SessionID,
			Timestamp:       date.UTC().Format(time.RFC3339Nano),
			AppType:         "claude",
			Source:          "claude_session",
			SourceLabel:     claudeLocalSourceLabel,
			Model:           session.Model,
			ModelName:       session.Model,
			InputTokens:     session.InputTokens,
			OutputTokens:    session.OutputTokens,
			CacheReadTokens: session.CacheReadTokens,
			TotalTokens:     session.TotalTokens,
			ModelCosts:      calculateClaudeSessionModelCosts(session),
		})
		analytics.SessionCount++
		analytics.TotalTurns += session.TurnCount
		analytics.InputTokens += session.InputTokens
		analytics.OutputTokens += session.OutputTokens
		analytics.CacheReadTokens += session.CacheReadTokens
		analytics.TotalTokens += session.TotalTokens
	}

	mergeClaudeSeries(analytics, sessions)
	analytics.TotalTurns = maxInt(analytics.TotalTurns, 0)
	if analytics.SessionCount > 0 {
		analytics.AvgTurns = float64(analytics.TotalTurns) / float64(analytics.SessionCount)
	}
	rebuildLocalUsageSources(analytics)
	return nil
}

func mergeClaudeSeries(analytics *LocalTokenUsageAnalytics, sessions []claudeSessionAnalytics) {
	series := map[string]*LocalTokenUsageSeriesPoint{}
	for _, existing := range analytics.Series {
		appType := strings.TrimSpace(existing.AppType)
		if appType == "" {
			appType = "codex"
		}
		key := strings.Join([]string{appType, existing.Date, existing.Hour}, "|")
		point := existing
		point.AppType = appType
		series[key] = &point
	}
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
		key := strings.Join([]string{"claude", dayKey, hourKey}, "|")
		point := series[key]
		if point == nil {
			point = &LocalTokenUsageSeriesPoint{
				Date:        dayKey,
				Hour:        hourKey,
				AppType:     "claude",
				Source:      claudeLocalSource,
				SourceLabel: claudeLocalSourceLabel,
			}
			series[key] = point
		}
		point.SessionCount++
		point.InputTokens += session.InputTokens
		point.OutputTokens += session.OutputTokens
		point.CacheReadTokens += session.CacheReadTokens
		point.TotalTokens += session.TotalTokens
		if session.Model != "" {
			if point.Model == "" {
				point.Model = session.Model
			} else if point.Model != session.Model {
				point.Model = "Multiple models"
			}
		}
		point.ModelCosts = mergeClaudeModelCosts(point.ModelCosts, calculateClaudeSessionModelCosts(session))
	}
	analytics.Series = make([]LocalTokenUsageSeriesPoint, 0, len(series))
	for _, point := range series {
		analytics.Series = append(analytics.Series, *point)
	}
	sort.Slice(analytics.Series, func(i, j int) bool {
		if analytics.Series[i].Date != analytics.Series[j].Date {
			return analytics.Series[i].Date < analytics.Series[j].Date
		}
		if analytics.Series[i].Hour != analytics.Series[j].Hour {
			return analytics.Series[i].Hour < analytics.Series[j].Hour
		}
		return analytics.Series[i].Source < analytics.Series[j].Source
	})
}

func calculateClaudeSessionModelCosts(session claudeSessionAnalytics) []LocalTokenUsageModelCost {
	if session.TotalTokens <= 0 {
		return nil
	}
	if len(session.ModelUsages) == 0 {
		model := strings.TrimSpace(session.Model)
		if model == "" || model == "Multiple models" {
			return nil
		}
		return []LocalTokenUsageModelCost{{
			Model:     model,
			ModelName: model,
			Tokens:    session.TotalTokens,
		}}
	}

	modelCosts := make([]LocalTokenUsageModelCost, 0, len(session.ModelUsages))
	for model, usage := range session.ModelUsages {
		model = strings.TrimSpace(model)
		if model == "" || usage.TotalTokens <= 0 {
			continue
		}
		modelCosts = append(modelCosts, LocalTokenUsageModelCost{
			Model:     model,
			ModelName: model,
			Tokens:    usage.TotalTokens,
		})
	}
	sort.SliceStable(modelCosts, func(left, right int) bool {
		if modelCosts[left].Tokens != modelCosts[right].Tokens {
			return modelCosts[left].Tokens > modelCosts[right].Tokens
		}
		return modelCosts[left].Model < modelCosts[right].Model
	})
	return modelCosts
}

func mergeClaudeModelCosts(existing, additions []LocalTokenUsageModelCost) []LocalTokenUsageModelCost {
	if len(additions) == 0 {
		return existing
	}
	merged := make(map[string]LocalTokenUsageModelCost, len(existing)+len(additions))
	for _, item := range existing {
		key := strings.ToLower(strings.TrimSpace(item.Model))
		if key == "" {
			continue
		}
		if strings.TrimSpace(item.ModelName) == "" {
			item.ModelName = item.Model
		}
		merged[key] = item
	}
	for _, item := range additions {
		key := strings.ToLower(strings.TrimSpace(item.Model))
		if key == "" {
			continue
		}
		current := merged[key]
		current.Model = item.Model
		if strings.TrimSpace(item.ModelName) != "" {
			current.ModelName = item.ModelName
		}
		current.Tokens += item.Tokens
		current.Cost += item.Cost
		current.CostKnown = current.CostKnown || item.CostKnown
		merged[key] = current
	}
	result := make([]LocalTokenUsageModelCost, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Tokens != result[right].Tokens {
			return result[left].Tokens > result[right].Tokens
		}
		return result[left].Model < result[right].Model
	})
	return result
}

func rebuildLocalUsageSources(analytics *LocalTokenUsageAnalytics) {
	if analytics == nil {
		return
	}
	sources := map[string]*LocalTokenUsageSource{}
	for _, session := range analytics.Sessions {
		if session.TotalTokens <= 0 {
			continue
		}
		source := strings.TrimSpace(session.AppType)
		if source == "" {
			source = strings.TrimSpace(session.Source)
		}
		if source == "" {
			source = "local"
		}
		label := strings.TrimSpace(session.SourceLabel)
		if label == "" {
			label = source
		}
		item := sources[source]
		if item == nil {
			item = &LocalTokenUsageSource{Source: source, SourceLabel: label}
			sources[source] = item
		}
		item.SessionCount++
		item.TotalTokens += session.TotalTokens
		item.InputTokens += session.InputTokens
		item.OutputTokens += session.OutputTokens
		item.ReasoningTokens += session.ReasoningTokens
		item.CacheReadTokens += session.CacheReadTokens
	}
	analytics.Sources = make([]LocalTokenUsageSource, 0, len(sources))
	for _, source := range sources {
		analytics.Sources = append(analytics.Sources, *source)
	}
	sort.Slice(analytics.Sources, func(i, j int) bool {
		if analytics.Sources[i].TotalTokens != analytics.Sources[j].TotalTokens {
			return analytics.Sources[i].TotalTokens > analytics.Sources[j].TotalTokens
		}
		return analytics.Sources[i].Source < analytics.Sources[j].Source
	})
	if len(analytics.Sources) == 1 {
		analytics.Source = analytics.Sources[0].Source
		analytics.SourceLabel = analytics.Sources[0].SourceLabel
	} else if len(analytics.Sources) > 1 {
		analytics.Source = "local_sessions"
		analytics.SourceLabel = "Local sessions"
	}
	activeDays := map[string]struct{}{}
	for _, session := range analytics.Sessions {
		if timestamp := parseClaudeSessionTimestamp(session.Timestamp); !timestamp.IsZero() {
			activeDays[timestamp.Local().Format("2006-01-02")] = struct{}{}
		}
	}
	analytics.ActiveDays = len(activeDays)
}
