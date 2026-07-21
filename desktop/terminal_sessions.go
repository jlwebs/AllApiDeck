package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const terminalSessionDefaultPageSize = 15
const terminalSessionMaxPageSize = 30
const terminalSessionMaxFileCandidates = 450
const terminalSessionDefaultMessageLimit = 80
const terminalSessionMaxMessageLimit = 200

var (
	terminalAnsiEscapePattern    = regexp.MustCompile("\x1b(?:\\[[0-?]*[ -/]*[@-~]|\\][^\a]*(?:\a|\x1b\\\\)|[@-Z\\\\-_])")
	terminalAnsiC1Pattern        = regexp.MustCompile("\u009b[0-?]*[ -/]*[@-~]")
	terminalAnsiOrphanPattern    = regexp.MustCompile(`\[[0-9;?]{1,16}[A-Za-z]`)
	terminalRepeatedBlankPattern = regexp.MustCompile(`\n{3,}`)
)

type TerminalSessionMeta struct {
	ProviderID    string `json:"providerId"`
	SessionID     string `json:"sessionId"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	ProjectDir    string `json:"projectDir"`
	CreatedAt     int64  `json:"createdAt"`
	LastActiveAt  int64  `json:"lastActiveAt"`
	SourcePath    string `json:"sourcePath"`
	ResumeCommand string `json:"resumeCommand"`
}

type TerminalSessionProviderSummary struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Total int    `json:"total"`
}

type TerminalSessionPage struct {
	ProviderID string                           `json:"providerId"`
	Page       int                              `json:"page"`
	PageSize   int                              `json:"pageSize"`
	Total      int                              `json:"total"`
	HasMore    bool                             `json:"hasMore"`
	Providers  []TerminalSessionProviderSummary `json:"providers"`
	Sessions   []TerminalSessionMeta            `json:"sessions"`
}

type TerminalSessionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Ts      int64  `json:"ts"`
}

type terminalSessionCandidate struct {
	path    string
	modTime int64
}

type terminalSessionProviderDef struct {
	id       string
	label    string
	scanner  func(page int, pageSize int) ([]TerminalSessionMeta, int)
	command  func(sessionID string) string
	roots    func() []string
	fileExt  string
	parse    func(string) (TerminalSessionMeta, bool)
	deepScan bool
}

var terminalSessionProviders = []terminalSessionProviderDef{
	{
		id:      "codex",
		label:   "Codex",
		command: func(sessionID string) string { return "codex resume " + sessionID },
		roots: func() []string {
			base := userHomeJoin(".codex")
			return []string{filepath.Join(base, "sessions"), filepath.Join(base, "archived_sessions")}
		},
		fileExt: ".jsonl",
		parse:   parseCodexTerminalSession,
	},
	{
		id:      "claude",
		label:   "Claude",
		command: func(sessionID string) string { return "claude --resume " + sessionID },
		roots:   func() []string { return []string{userHomeJoin(".claude", "projects")} },
		fileExt: ".jsonl",
		parse:   parseClaudeTerminalSession,
	},
	{
		id:      "grok",
		label:   "Grok",
		command: func(sessionID string) string { return "grok --resume " + sessionID },
		scanner: scanGrokTerminalSessions,
	},
	{
		id:      "opencode",
		label:   "OpenCode",
		command: func(sessionID string) string { return "opencode -s " + sessionID },
		scanner: scanOpenCodeTerminalSessions,
	},
	{
		id:      "openclaw",
		label:   "OpenClaw",
		command: func(string) string { return "" },
		roots:   func() []string { return []string{userHomeJoin(".openclaw", "agents")} },
		fileExt: ".jsonl",
		parse:   parseOpenClawTerminalSession,
	},
	{
		id:      "gemini",
		label:   "Gemini",
		command: func(sessionID string) string { return "gemini --resume " + sessionID },
		roots:   func() []string { return []string{userHomeJoin(".gemini", "tmp")} },
		fileExt: ".json",
		parse:   parseGeminiTerminalSession,
	},
}

func (a *App) GetTerminalSessions(providerID string, page int, pageSize int) (TerminalSessionPage, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(providerID))
	if normalizedProvider == "" {
		normalizedProvider = "codex"
	}
	provider, ok := findTerminalSessionProvider(normalizedProvider)
	if !ok {
		return TerminalSessionPage{}, fmt.Errorf("unsupported terminal session provider: %s", providerID)
	}
	page = clampPositiveInt(page, 1)
	pageSize = terminalClampInt(pageSize, 1, terminalSessionMaxPageSize, terminalSessionDefaultPageSize)

	sessions, total := scanTerminalSessionsForProvider(provider, page, pageSize)
	return TerminalSessionPage{
		ProviderID: provider.id,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		HasMore:    page*pageSize < total,
		Providers:  buildTerminalSessionProviderSummaries(provider.id, total),
		Sessions:   sessions,
	}, nil
}

func (a *App) LaunchTerminalSession(command string, cwd string) (bool, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return false, errors.New("resume command is empty")
	}
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cwd = home
		}
	}
	if cwd != "" {
		if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
			cwd = ""
		}
	}
	if err := launchTerminalCommand(command, cwd); err != nil {
		return false, err
	}
	return true, nil
}

func (a *App) GetTerminalSessionMessages(providerID string, sourcePath string, limit int) ([]TerminalSessionMessage, error) {
	normalizedProvider := strings.ToLower(strings.TrimSpace(providerID))
	sourcePath = strings.TrimSpace(sourcePath)
	if normalizedProvider == "" {
		return nil, errors.New("terminal session provider is required")
	}
	if sourcePath == "" {
		return nil, errors.New("terminal session source path is required")
	}
	limit = terminalClampInt(limit, 1, terminalSessionMaxMessageLimit, terminalSessionDefaultMessageLimit)

	var (
		messages []TerminalSessionMessage
		err      error
	)
	switch normalizedProvider {
	case "codex":
		messages, err = loadCodexTerminalSessionMessages(sourcePath)
	case "claude":
		messages, err = loadClaudeTerminalSessionMessages(sourcePath)
	case "opencode":
		messages, err = loadOpenCodeTerminalSessionMessages(sourcePath)
	case "openclaw":
		messages, err = loadOpenClawTerminalSessionMessages(sourcePath)
	case "gemini":
		messages, err = loadGeminiTerminalSessionMessages(sourcePath)
	case "grok":
		messages, err = loadGrokTerminalSessionMessages(sourcePath)
	default:
		return nil, fmt.Errorf("unsupported terminal session provider: %s", providerID)
	}
	if err != nil {
		return nil, err
	}
	return limitTerminalMessages(messages, limit), nil
}

func findTerminalSessionProvider(providerID string) (terminalSessionProviderDef, bool) {
	for _, provider := range terminalSessionProviders {
		if provider.id == providerID {
			return provider, true
		}
	}
	return terminalSessionProviderDef{}, false
}

func scanTerminalSessionsForProvider(provider terminalSessionProviderDef, page int, pageSize int) ([]TerminalSessionMeta, int) {
	if provider.scanner != nil {
		return provider.scanner(page, pageSize)
	}
	candidates := collectTerminalSessionCandidates(provider.roots(), provider.fileExt, terminalSessionMaxFileCandidates)
	allSessions := make([]TerminalSessionMeta, 0, len(candidates))
	for _, candidate := range candidates {
		session, ok := provider.parse(candidate.path)
		if !ok {
			continue
		}
		session.ProviderID = provider.id
		completeTerminalSessionMeta(&session, provider, candidate)
		allSessions = append(allSessions, session)
	}
	sortTerminalSessions(allSessions)
	start := (page - 1) * pageSize
	if start >= len(allSessions) {
		return []TerminalSessionMeta{}, len(allSessions)
	}
	end := start + pageSize
	if end > len(allSessions) {
		end = len(allSessions)
	}
	return append([]TerminalSessionMeta(nil), allSessions[start:end]...), len(allSessions)
}

func completeTerminalSessionMeta(session *TerminalSessionMeta, provider terminalSessionProviderDef, candidate terminalSessionCandidate) {
	if session == nil {
		return
	}
	if session.LastActiveAt == 0 {
		session.LastActiveAt = candidate.modTime
	}
	if session.CreatedAt == 0 {
		session.CreatedAt = candidate.modTime
	}
	if session.ResumeCommand == "" && provider.command != nil && session.SessionID != "" {
		session.ResumeCommand = provider.command(session.SessionID)
	}
	if session.SourcePath == "" {
		session.SourcePath = candidate.path
	}
}

func buildTerminalSessionProviderSummaries(activeProvider string, activeTotal int) []TerminalSessionProviderSummary {
	result := make([]TerminalSessionProviderSummary, 0, len(terminalSessionProviders))
	for _, provider := range terminalSessionProviders {
		total := 0
		if provider.id == activeProvider {
			total = activeTotal
		}
		result = append(result, TerminalSessionProviderSummary{ID: provider.id, Label: provider.label, Total: total})
	}
	return result
}

func collectTerminalSessionCandidates(roots []string, ext string, limit int) []terminalSessionCandidate {
	candidates := make([]terminalSessionCandidate, 0, 128)
	normalizedExt := strings.ToLower(strings.TrimSpace(ext))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if normalizedExt != "" && strings.ToLower(filepath.Ext(path)) != normalizedExt {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			candidates = append(candidates, terminalSessionCandidate{
				path:    path,
				modTime: info.ModTime().UnixMilli(),
			})
			return nil
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})
	if limit > 0 && len(candidates) > limit {
		return append([]terminalSessionCandidate(nil), candidates[:limit]...)
	}
	return candidates
}

func parseCodexTerminalSession(path string) (TerminalSessionMeta, bool) {
	head, tail, err := readHeadTailLines(path, 12, 28)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	session := TerminalSessionMeta{ProviderID: "codex", SourcePath: path}
	var firstUser string
	for _, line := range head {
		value := parseJSONObjectLine(line)
		if value == nil {
			continue
		}
		if session.CreatedAt == 0 {
			session.CreatedAt = parseTimestampMs(value["timestamp"])
		}
		if stringValue(value["type"]) == "session_meta" {
			payload := objectValue(value["payload"])
			if objectValue(payload["source"])["subagent"] != nil {
				return TerminalSessionMeta{}, false
			}
			session.SessionID = firstNonEmpty(session.SessionID, stringValue(payload["id"]))
			session.ProjectDir = firstNonEmpty(session.ProjectDir, stringValue(payload["cwd"]))
			if ts := parseTimestampMs(payload["timestamp"]); ts > 0 && session.CreatedAt == 0 {
				session.CreatedAt = ts
			}
		}
		if firstUser == "" && stringValue(value["type"]) == "response_item" {
			payload := objectValue(value["payload"])
			if stringValue(payload["type"]) == "message" && stringValue(payload["role"]) == "user" {
				firstUser = titleCandidateFromCodexUserMessage(extractTerminalText(payload["content"]))
			}
		}
	}
	var summary string
	for index := len(tail) - 1; index >= 0; index-- {
		value := parseJSONObjectLine(tail[index])
		if value == nil {
			continue
		}
		if session.LastActiveAt == 0 {
			session.LastActiveAt = parseTimestampMs(value["timestamp"])
		}
		if summary == "" && stringValue(value["type"]) == "response_item" {
			payload := objectValue(value["payload"])
			if stringValue(payload["type"]) == "message" {
				summary = extractTerminalText(payload["content"])
			}
		}
	}
	session.SessionID = firstNonEmpty(session.SessionID, inferSessionIDFromFilename(path))
	if session.SessionID == "" {
		return TerminalSessionMeta{}, false
	}
	session.Title = firstNonEmpty(truncateTerminalText(firstUser, 90), filepath.Base(session.ProjectDir), session.SessionID)
	session.Summary = truncateTerminalText(summary, 180)
	session.ResumeCommand = "codex resume " + session.SessionID
	return session, true
}

func parseClaudeTerminalSession(path string) (TerminalSessionMeta, bool) {
	if strings.HasPrefix(filepath.Base(path), "agent-") {
		return TerminalSessionMeta{}, false
	}
	head, tail, err := readHeadTailLines(path, 12, 28)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	session := TerminalSessionMeta{ProviderID: "claude", SourcePath: path}
	var firstUser string
	for _, line := range head {
		value := parseJSONObjectLine(line)
		if value == nil {
			continue
		}
		session.SessionID = firstNonEmpty(session.SessionID, stringValue(value["sessionId"]))
		session.ProjectDir = firstNonEmpty(session.ProjectDir, stringValue(value["cwd"]))
		if session.CreatedAt == 0 {
			session.CreatedAt = parseTimestampMs(value["timestamp"])
		}
		if firstUser == "" {
			message := objectValue(value["message"])
			if stringValue(value["type"]) == "user" || stringValue(message["role"]) == "user" {
				text := strings.TrimSpace(extractTerminalText(message["content"]))
				if text != "" && !strings.Contains(text, "<local-command-caveat>") && !strings.HasPrefix(text, "<command-name>") {
					firstUser = text
				}
			}
		}
	}
	var summary string
	var customTitle string
	for index := len(tail) - 1; index >= 0; index-- {
		value := parseJSONObjectLine(tail[index])
		if value == nil {
			continue
		}
		if session.LastActiveAt == 0 {
			session.LastActiveAt = parseTimestampMs(value["timestamp"])
		}
		if customTitle == "" && stringValue(value["type"]) == "custom-title" {
			customTitle = strings.TrimSpace(stringValue(value["customTitle"]))
		}
		if summary == "" && boolValue(value["isMeta"]) != true {
			message := objectValue(value["message"])
			summary = extractTerminalText(message["content"])
		}
	}
	session.SessionID = firstNonEmpty(session.SessionID, inferSessionIDFromFilename(path))
	if session.SessionID == "" {
		return TerminalSessionMeta{}, false
	}
	session.Title = firstNonEmpty(truncateTerminalText(customTitle, 90), truncateTerminalText(firstUser, 90), filepath.Base(session.ProjectDir), session.SessionID)
	session.Summary = truncateTerminalText(summary, 180)
	session.ResumeCommand = "claude --resume " + session.SessionID
	return session, true
}

func parseOpenClawTerminalSession(path string) (TerminalSessionMeta, bool) {
	head, tail, err := readHeadTailLines(path, 12, 28)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	session := TerminalSessionMeta{ProviderID: "openclaw", SourcePath: path}
	var firstUser string
	var summary string
	for _, line := range head {
		value := parseJSONObjectLine(line)
		if value == nil {
			continue
		}
		if session.CreatedAt == 0 {
			session.CreatedAt = parseTimestampMs(value["timestamp"])
		}
		eventType := stringValue(value["type"])
		if eventType == "session" {
			session.SessionID = firstNonEmpty(session.SessionID, stringValue(value["id"]))
			session.ProjectDir = firstNonEmpty(session.ProjectDir, stringValue(value["cwd"]))
		}
		if eventType == "message" {
			message := objectValue(value["message"])
			text := stripOpenClawMessageIDSuffix(extractTerminalText(message["content"]))
			if summary == "" {
				summary = text
			}
			if firstUser == "" && stringValue(message["role"]) == "user" {
				firstUser = text
			}
		}
	}
	for index := len(tail) - 1; index >= 0; index-- {
		value := parseJSONObjectLine(tail[index])
		if value == nil {
			continue
		}
		if ts := parseTimestampMs(value["timestamp"]); ts > 0 {
			session.LastActiveAt = ts
			break
		}
	}
	session.SessionID = firstNonEmpty(session.SessionID, inferSessionIDFromFilename(path))
	if session.SessionID == "" {
		return TerminalSessionMeta{}, false
	}
	session.Title = firstNonEmpty(truncateTerminalText(firstUser, 90), filepath.Base(session.ProjectDir), session.SessionID)
	session.Summary = truncateTerminalText(summary, 180)
	return session, true
}

func parseGeminiTerminalSession(path string) (TerminalSessionMeta, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return TerminalSessionMeta{}, false
	}
	sessionID := stringValue(value["sessionId"])
	if sessionID == "" {
		return TerminalSessionMeta{}, false
	}
	session := TerminalSessionMeta{
		ProviderID:    "gemini",
		SessionID:     sessionID,
		CreatedAt:     parseTimestampMs(value["startTime"]),
		LastActiveAt:  parseTimestampMs(value["lastUpdated"]),
		SourcePath:    path,
		ResumeCommand: "gemini --resume " + sessionID,
	}
	if messages, ok := value["messages"].([]any); ok {
		for _, item := range messages {
			message := objectValue(item)
			if stringValue(message["type"]) == "user" {
				session.Title = truncateTerminalText(extractTerminalText(message["content"]), 90)
				session.Summary = truncateTerminalText(session.Title, 180)
				break
			}
		}
	}
	session.Title = firstNonEmpty(session.Title, session.SessionID)
	return session, true
}

func scanOpenCodeTerminalSessions(page int, pageSize int) ([]TerminalSessionMeta, int) {
	if sessions, total := scanOpenCodeSQLiteTerminalSessions(page, pageSize); total > 0 {
		return sessions, total
	}
	storage := openCodeStorageDir()
	candidates := collectTerminalSessionCandidates([]string{filepath.Join(storage, "session")}, ".json", terminalSessionMaxFileCandidates)
	allSessions := make([]TerminalSessionMeta, 0, len(candidates))
	for _, candidate := range candidates {
		if session, ok := parseOpenCodeJSONTerminalSession(storage, candidate.path); ok {
			if session.LastActiveAt == 0 {
				session.LastActiveAt = candidate.modTime
			}
			if session.CreatedAt == 0 {
				session.CreatedAt = candidate.modTime
			}
			if session.SourcePath == "" {
				session.SourcePath = candidate.path
			}
			allSessions = append(allSessions, session)
		}
	}
	sortTerminalSessions(allSessions)
	start := (page - 1) * pageSize
	if start >= len(allSessions) {
		return []TerminalSessionMeta{}, len(allSessions)
	}
	end := start + pageSize
	if end > len(allSessions) {
		end = len(allSessions)
	}
	return append([]TerminalSessionMeta(nil), allSessions[start:end]...), len(allSessions)
}

func scanOpenCodeSQLiteTerminalSessions(page int, pageSize int) ([]TerminalSessionMeta, int) {
	dbPath := filepath.Join(openCodeBaseDir(), "opencode.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, 0
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, 0
	}
	defer db.Close()

	total := 0
	_ = db.QueryRow("SELECT COUNT(*) FROM session").Scan(&total)
	if total <= 0 {
		return nil, 0
	}
	offset := (page - 1) * pageSize
	rows, err := db.Query("SELECT id, title, directory, time_created, time_updated FROM session ORDER BY time_updated DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	sessions := make([]TerminalSessionMeta, 0, pageSize)
	for rows.Next() {
		var id, title, directory string
		var created, updated int64
		if err := rows.Scan(&id, &title, &directory, &created, &updated); err != nil {
			continue
		}
		displayTitle := firstNonEmpty(title, filepath.Base(directory), id)
		sessions = append(sessions, TerminalSessionMeta{
			ProviderID:    "opencode",
			SessionID:     id,
			Title:         displayTitle,
			Summary:       displayTitle,
			ProjectDir:    directory,
			CreatedAt:     normalizeUnixMaybeMs(created),
			LastActiveAt:  normalizeUnixMaybeMs(updated),
			SourcePath:    fmt.Sprintf("sqlite:%s:%s", dbPath, id),
			ResumeCommand: "opencode -s " + id,
		})
	}
	return sessions, total
}

func parseOpenCodeJSONTerminalSession(storage string, path string) (TerminalSessionMeta, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return TerminalSessionMeta{}, false
	}
	sessionID := stringValue(value["id"])
	if sessionID == "" {
		return TerminalSessionMeta{}, false
	}
	timeValue := objectValue(value["time"])
	directory := stringValue(value["directory"])
	title := firstNonEmpty(stringValue(value["title"]), filepath.Base(directory), sessionID)
	sourcePath := filepath.Join(storage, "message", sessionID)
	return TerminalSessionMeta{
		ProviderID:    "opencode",
		SessionID:     sessionID,
		Title:         title,
		Summary:       title,
		ProjectDir:    directory,
		CreatedAt:     parseTimestampMs(timeValue["created"]),
		LastActiveAt:  parseTimestampMs(timeValue["updated"]),
		SourcePath:    sourcePath,
		ResumeCommand: "opencode -s " + sessionID,
	}, true
}

func loadCodexTerminalSessionMessages(path string) ([]TerminalSessionMessage, error) {
	lines, err := readAllLines(path)
	if err != nil {
		return nil, err
	}
	messages := make([]TerminalSessionMessage, 0, len(lines)/2)
	for _, line := range lines {
		value := parseJSONObjectLine(line)
		if value == nil || stringValue(value["type"]) != "response_item" {
			continue
		}
		payload := objectValue(value["payload"])
		payloadType := stringValue(payload["type"])
		message := TerminalSessionMessage{Ts: parseTimestampMs(value["timestamp"])}
		switch payloadType {
		case "message":
			message.Role = firstNonEmpty(stringValue(payload["role"]), "unknown")
			message.Content = extractTerminalText(payload["content"])
		case "function_call":
			message.Role = "assistant"
			message.Content = "[Tool: " + firstNonEmpty(stringValue(payload["name"]), "unknown") + "]"
		case "function_call_output":
			message.Role = "tool"
			message.Content = extractTerminalText(payload["output"])
		default:
			continue
		}
		if normalized := normalizeTerminalMessage(message); normalized.Content != "" {
			messages = append(messages, normalized)
		}
	}
	return messages, nil
}

func loadClaudeTerminalSessionMessages(path string) ([]TerminalSessionMessage, error) {
	lines, err := readAllLines(path)
	if err != nil {
		return nil, err
	}
	messages := make([]TerminalSessionMessage, 0, len(lines)/2)
	for _, line := range lines {
		value := parseJSONObjectLine(line)
		if value == nil || boolValue(value["isMeta"]) {
			continue
		}
		messageObject := objectValue(value["message"])
		role := firstNonEmpty(stringValue(messageObject["role"]), stringValue(value["type"]), "unknown")
		content := extractTerminalText(messageObject["content"])
		if content == "" {
			content = extractTerminalText(value["content"])
		}
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    role,
			Content: content,
			Ts:      parseTimestampMs(value["timestamp"]),
		})
		if message.Content != "" {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func loadOpenClawTerminalSessionMessages(path string) ([]TerminalSessionMessage, error) {
	lines, err := readAllLines(path)
	if err != nil {
		return nil, err
	}
	messages := make([]TerminalSessionMessage, 0, len(lines)/2)
	for _, line := range lines {
		value := parseJSONObjectLine(line)
		if value == nil || stringValue(value["type"]) != "message" {
			continue
		}
		messageObject := objectValue(value["message"])
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    firstNonEmpty(stringValue(messageObject["role"]), "unknown"),
			Content: stripOpenClawMessageIDSuffix(extractTerminalText(messageObject["content"])),
			Ts:      parseTimestampMs(value["timestamp"]),
		})
		if message.Content != "" {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func loadGeminiTerminalSessionMessages(path string) ([]TerminalSessionMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	rawMessages, _ := value["messages"].([]any)
	messages := make([]TerminalSessionMessage, 0, len(rawMessages))
	for _, item := range rawMessages {
		messageObject := objectValue(item)
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    firstNonEmpty(stringValue(messageObject["role"]), stringValue(messageObject["type"]), "unknown"),
			Content: extractTerminalText(messageObject["content"]),
			Ts:      parseTimestampMs(firstNonEmptyAny(messageObject["timestamp"], messageObject["time"])),
		})
		if message.Content != "" {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func scanGrokTerminalSessions(page int, pageSize int) ([]TerminalSessionMeta, int) {
	root := userHomeJoin(".grok", "sessions")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return []TerminalSessionMeta{}, 0
	}

	allSessions := make([]TerminalSessionMeta, 0, 64)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || !entry.IsDir() {
			return nil
		}
		// Session dirs are second-level: sessions/<encoded_cwd>/<session_id>
		if path == root {
			return nil
		}
		summaryPath := filepath.Join(path, "summary.json")
		if _, err := os.Stat(summaryPath); err != nil {
			return nil
		}
		// Skip if parent is root (encoded cwd folders may also accidentally hold files)
		parent := filepath.Dir(path)
		if parent == root {
			return nil
		}
		session, ok := parseGrokTerminalSession(path)
		if !ok {
			return nil
		}
		allSessions = append(allSessions, session)
		return nil
	})

	sortTerminalSessions(allSessions)
	start := (page - 1) * pageSize
	if start >= len(allSessions) {
		return []TerminalSessionMeta{}, len(allSessions)
	}
	end := start + pageSize
	if end > len(allSessions) {
		end = len(allSessions)
	}
	return append([]TerminalSessionMeta(nil), allSessions[start:end]...), len(allSessions)
}

func parseGrokTerminalSession(dir string) (TerminalSessionMeta, bool) {
	summaryPath := filepath.Join(dir, "summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return TerminalSessionMeta{}, false
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return TerminalSessionMeta{}, false
	}
	info := objectValue(value["info"])
	sessionID := firstNonEmpty(stringValue(info["id"]), filepath.Base(dir))
	if sessionID == "" {
		return TerminalSessionMeta{}, false
	}
	projectDir := stringValue(info["cwd"])
	title := firstNonEmpty(
		stringValue(value["generated_title"]),
		stringValue(value["session_summary"]),
		filepath.Base(projectDir),
		sessionID,
	)
	createdAt := parseTimestampMs(value["created_at"])
	lastActiveAt := parseTimestampMs(firstNonEmptyAny(value["last_active_at"], value["updated_at"]))
	if lastActiveAt == 0 {
		if st, err := os.Stat(summaryPath); err == nil {
			lastActiveAt = st.ModTime().UnixMilli()
		}
	}
	if createdAt == 0 {
		createdAt = lastActiveAt
	}
	sourcePath := dir
	if _, err := os.Stat(filepath.Join(dir, "chat_history.jsonl")); err == nil {
		sourcePath = filepath.Join(dir, "chat_history.jsonl")
	}
	return TerminalSessionMeta{
		ProviderID:    "grok",
		SessionID:     sessionID,
		Title:         truncateTerminalText(title, 90),
		Summary:       truncateTerminalText(firstNonEmpty(stringValue(value["session_summary"]), title), 180),
		ProjectDir:    projectDir,
		CreatedAt:     createdAt,
		LastActiveAt:  lastActiveAt,
		SourcePath:    sourcePath,
		ResumeCommand: "grok --resume " + sessionID,
	}, true
}

func loadGrokTerminalSessionMessages(sourcePath string) ([]TerminalSessionMessage, error) {
	dir := sourcePath
	if strings.HasSuffix(strings.ToLower(sourcePath), ".jsonl") {
		dir = filepath.Dir(sourcePath)
	}
	chatPath := filepath.Join(dir, "chat_history.jsonl")
	if _, err := os.Stat(chatPath); err != nil {
		chatPath = sourcePath
	}
	// Stream large chat_history files; keep only the most recent messages.
	const retainCap = 400
	messages := make([]TerminalSessionMessage, 0, 64)
	file, err := os.Open(chatPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1<<20)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			value := parseJSONObjectLine(strings.TrimRight(line, "\r\n"))
			if value != nil {
				if message, ok := parseGrokChatHistoryMessage(value); ok {
					messages = append(messages, message)
					if len(messages) > retainCap {
						messages = append([]TerminalSessionMessage(nil), messages[len(messages)-retainCap:]...)
					}
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return messages, readErr
		}
	}
	// If chat_history has almost no assistant text, supplement from updates.jsonl
	assistantCount := 0
	for _, m := range messages {
		if m.Role == "assistant" {
			assistantCount++
		}
	}
	if assistantCount == 0 {
		if updateMessages := loadGrokUpdateMessageChunks(dir); len(updateMessages) > 0 {
			messages = append(messages, updateMessages...)
			sort.SliceStable(messages, func(i, j int) bool {
				return messages[i].Ts < messages[j].Ts
			})
		}
	}
	return messages, nil
}

func parseGrokChatHistoryMessage(value map[string]any) (TerminalSessionMessage, bool) {
	msgType := strings.ToLower(stringValue(value["type"]))
	switch msgType {
	case "user":
		if stringValue(value["synthetic_reason"]) != "" {
			return TerminalSessionMessage{}, false
		}
		raw := extractTerminalText(value["content"])
		content := extractGrokUserQuery(raw)
		if content == "" {
			content = raw
		}
		trimmed := strings.TrimSpace(content)
		if strings.HasPrefix(trimmed, "<user_info>") || strings.HasPrefix(trimmed, "<system-reminder>") {
			return TerminalSessionMessage{}, false
		}
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    "user",
			Content: content,
			Ts:      parseTimestampMs(firstNonEmptyAny(value["timestamp"], value["created_at"])),
		})
		if message.Content == "" {
			return TerminalSessionMessage{}, false
		}
		return message, true
	case "assistant":
		content := extractTerminalText(value["content"])
		if content == "" {
			return TerminalSessionMessage{}, false
		}
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    "assistant",
			Content: content,
			Ts:      parseTimestampMs(firstNonEmptyAny(value["timestamp"], value["created_at"])),
		})
		if message.Content == "" {
			return TerminalSessionMessage{}, false
		}
		return message, true
	default:
		return TerminalSessionMessage{}, false
	}
}

func loadGrokUpdateMessageChunks(dir string) []TerminalSessionMessage {
	updatePath := filepath.Join(dir, "updates.jsonl")
	lines, err := readAllLines(updatePath)
	if err != nil {
		return nil
	}
	var (
		messages   []TerminalSessionMessage
		builder    strings.Builder
		lastTs     int64
		collecting bool
	)
	flush := func() {
		text := strings.TrimSpace(builder.String())
		if text == "" {
			return
		}
		messages = append(messages, normalizeTerminalMessage(TerminalSessionMessage{
			Role:    "assistant",
			Content: text,
			Ts:      lastTs,
		}))
		builder.Reset()
		collecting = false
	}
	for _, line := range lines {
		value := parseJSONObjectLine(line)
		if value == nil {
			continue
		}
		params := objectValue(value["params"])
		update := objectValue(params["update"])
		sessionUpdate := stringValue(update["sessionUpdate"])
		ts := parseTimestampMs(firstNonEmptyAny(value["timestamp"], update["timestamp"]))
		switch sessionUpdate {
		case "agent_message_chunk":
			content := objectValue(update["content"])
			text := firstNonEmpty(stringValue(content["text"]), extractTerminalText(update["content"]))
			if text == "" {
				continue
			}
			builder.WriteString(text)
			if ts > 0 {
				lastTs = ts
			}
			collecting = true
		case "user_message_chunk", "tool_call", "tool_call_update", "agent_thought_chunk":
			if collecting {
				flush()
			}
		default:
			if collecting && sessionUpdate != "" {
				flush()
			}
		}
	}
	if collecting {
		flush()
	}
	return messages
}

func extractGrokUserQuery(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	const openTag = "<user_query>"
	const closeTag = "</user_query>"
	start := strings.Index(trimmed, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(trimmed[start:], closeTag)
	if end < 0 {
		return strings.TrimSpace(trimmed[start:])
	}
	return strings.TrimSpace(trimmed[start : start+end])
}

func loadOpenCodeTerminalSessionMessages(sourcePath string) ([]TerminalSessionMessage, error) {
	if strings.HasPrefix(sourcePath, "sqlite:") {
		return loadOpenCodeSQLiteTerminalSessionMessages(sourcePath)
	}
	return loadOpenCodeJSONTerminalSessionMessages(sourcePath)
}

func loadOpenCodeSQLiteTerminalSessionMessages(sourcePath string) ([]TerminalSessionMessage, error) {
	dbPath, sessionID, ok := parseOpenCodeSQLiteSource(sourcePath)
	if !ok {
		return nil, fmt.Errorf("invalid OpenCode SQLite source reference: %s", sourcePath)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	partRows, err := db.Query("SELECT message_id, data FROM part WHERE session_id = ? ORDER BY time_created ASC", sessionID)
	if err != nil {
		return nil, err
	}
	partsByMessage := map[string][]string{}
	for partRows.Next() {
		var messageID, data string
		if err := partRows.Scan(&messageID, &data); err != nil {
			continue
		}
		if text := extractOpenCodePartText(data); text != "" {
			partsByMessage[messageID] = append(partsByMessage[messageID], text)
		}
	}
	_ = partRows.Close()

	rows, err := db.Query("SELECT id, time_created, data FROM message WHERE session_id = ? ORDER BY time_created ASC", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []TerminalSessionMessage{}
	for rows.Next() {
		var id, data string
		var ts int64
		if err := rows.Scan(&id, &ts, &data); err != nil {
			continue
		}
		value := map[string]any{}
		_ = json.Unmarshal([]byte(data), &value)
		content := strings.TrimSpace(strings.Join(partsByMessage[id], "\n"))
		if content == "" {
			content = extractTerminalText(value["content"])
		}
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    firstNonEmpty(stringValue(value["role"]), "unknown"),
			Content: content,
			Ts:      normalizeUnixMaybeMs(ts),
		})
		if message.Content != "" {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func loadOpenCodeJSONTerminalSessionMessages(path string) ([]TerminalSessionMessage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("OpenCode message directory not found: %s", path)
	}
	storage := filepath.Dir(filepath.Dir(path))
	candidates := collectTerminalSessionCandidates([]string{path}, ".json", terminalSessionMaxFileCandidates)
	entries := make([]TerminalSessionMessage, 0, len(candidates))
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		value := map[string]any{}
		if err := json.Unmarshal(data, &value); err != nil {
			continue
		}
		messageID := stringValue(value["id"])
		partsText := collectOpenCodePartTexts(filepath.Join(storage, "part", messageID))
		content := strings.TrimSpace(strings.Join(partsText, "\n"))
		if content == "" {
			content = extractTerminalText(value["content"])
		}
		message := normalizeTerminalMessage(TerminalSessionMessage{
			Role:    firstNonEmpty(stringValue(value["role"]), "unknown"),
			Content: content,
			Ts:      parseTimestampMs(objectValue(value["time"])["created"]),
		})
		if message.Ts == 0 {
			message.Ts = candidate.modTime
		}
		if message.Content != "" {
			entries = append(entries, message)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Ts < entries[j].Ts
	})
	return entries, nil
}

func parseOpenCodeSQLiteSource(sourcePath string) (string, string, bool) {
	rest, ok := strings.CutPrefix(sourcePath, "sqlite:")
	if !ok {
		return "", "", false
	}
	separator := strings.LastIndex(rest, ":ses_")
	if separator < 0 {
		separator = strings.LastIndex(rest, ":")
	}
	if separator <= 0 || separator >= len(rest)-1 {
		return "", "", false
	}
	return rest[:separator], rest[separator+1:], true
}

func collectOpenCodePartTexts(partDir string) []string {
	candidates := collectTerminalSessionCandidates([]string{partDir}, ".json", terminalSessionMaxFileCandidates)
	texts := make([]string, 0, len(candidates))
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].modTime < candidates[j].modTime
	})
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		if text := extractOpenCodePartText(string(data)); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func extractOpenCodePartText(raw string) string {
	value := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return ""
	}
	for _, key := range []string{"text", "content", "output", "input"} {
		if text := extractTerminalText(value[key]); text != "" {
			return text
		}
	}
	if nested := objectValue(value["data"]); len(nested) > 0 {
		for _, key := range []string{"text", "content", "output", "input"} {
			if text := extractTerminalText(nested[key]); text != "" {
				return text
			}
		}
	}
	return extractTerminalTextFromObject(value)
}

func sortTerminalSessions(sessions []TerminalSessionMeta) {
	sort.SliceStable(sessions, func(i, j int) bool {
		left := sessions[i].LastActiveAt
		if left == 0 {
			left = sessions[i].CreatedAt
		}
		right := sessions[j].LastActiveAt
		if right == 0 {
			right = sessions[j].CreatedAt
		}
		return left > right
	})
}

func readHeadTailLines(path string, headLimit int, tailLimit int) ([]string, []string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	head := make([]string, 0, headLimit)
	tail := make([]string, 0, tailLimit)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if len(head) < headLimit {
				head = append(head, line)
			}
			if tailLimit > 0 {
				if len(tail) >= tailLimit {
					copy(tail, tail[1:])
					tail[len(tail)-1] = line
				} else {
					tail = append(tail, line)
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return head, tail, err
		}
	}
	return head, tail, nil
}

func readAllLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	lines := []string{}
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lines = append(lines, strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return lines, err
		}
	}
	return lines, nil
}

func parseJSONObjectLine(line string) map[string]any {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(line), &value); err != nil {
		return nil
	}
	return value
}

func normalizeTerminalMessage(message TerminalSessionMessage) TerminalSessionMessage {
	message.Role = strings.ToLower(strings.TrimSpace(message.Role))
	if message.Role == "" {
		message.Role = "unknown"
	}
	message.Content = sanitizeTerminalDisplayText(message.Content)
	return message
}

func limitTerminalMessages(messages []TerminalSessionMessage, limit int) []TerminalSessionMessage {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return append([]TerminalSessionMessage(nil), messages[len(messages)-limit:]...)
}

func firstNonEmptyAny(values ...any) any {
	for _, value := range values {
		if stringValue(value) != "" {
			return value
		}
	}
	return nil
}

func extractTerminalText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := extractTerminalTextFromObject(objectValue(item))
			if text == "" {
				text = extractTerminalText(item)
			}
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		return extractTerminalTextFromObject(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func extractTerminalTextFromObject(value map[string]any) string {
	if len(value) == 0 {
		return ""
	}
	for _, key := range []string{"text", "content", "input", "output"} {
		if text := extractTerminalText(value[key]); text != "" {
			return text
		}
	}
	if name := stringValue(value["name"]); name != "" {
		return "[Tool: " + name + "]"
	}
	return ""
}

func parseTimestampMs(value any) int64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case float64:
		return normalizeUnixMaybeMs(int64(typed))
	case int64:
		return normalizeUnixMaybeMs(typed)
	case int:
		return normalizeUnixMaybeMs(int64(typed))
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return normalizeUnixMaybeMs(n)
		}
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed.UnixMilli()
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", text); err == nil {
			return parsed.UnixMilli()
		}
	}
	return 0
}

func normalizeUnixMaybeMs(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value < 100000000000 {
		return value * 1000
	}
	return value
}

func objectValue(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok && mapped != nil {
		return mapped
	}
	return map[string]any{}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func boolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func truncateTerminalText(value string, limit int) string {
	text := strings.Join(strings.Fields(sanitizeTerminalDisplayText(value)), " ")
	if limit <= 0 || len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "..."
}

func sanitizeTerminalDisplayText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = terminalAnsiEscapePattern.ReplaceAllString(text, "")
	text = terminalAnsiC1Pattern.ReplaceAllString(text, "")
	text = terminalAnsiOrphanPattern.ReplaceAllString(text, "")
	text = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, text)
	text = terminalRepeatedBlankPattern.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func titleCandidateFromCodexUserMessage(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.HasPrefix(trimmed, "# AGENTS.md") || strings.HasPrefix(trimmed, "<environment_context>") {
		return ""
	}
	const prefix = "# Context from my IDE setup:"
	if strings.HasPrefix(trimmed, prefix) {
		lowered := strings.ToLower(trimmed)
		marker := "my request for codex"
		if index := strings.LastIndex(lowered, marker); index >= 0 {
			rest := trimmed[index+len(marker):]
			rest = strings.TrimLeft(rest, "#: \r\n\t")
			return strings.TrimSpace(rest)
		}
	}
	return trimmed
}

func stripOpenClawMessageIDSuffix(text string) string {
	if index := strings.LastIndex(text, "\n[message_id:"); index >= 0 {
		return strings.TrimSpace(text[:index])
	}
	return strings.TrimSpace(text)
}

func inferSessionIDFromFilename(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.TrimSpace(stem)
}

func userHomeJoin(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(parts...)
	}
	all := append([]string{home}, parts...)
	return filepath.Join(all...)
}

func openCodeBaseDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	candidates := []string{}
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		candidates = append(candidates, filepath.Join(local, "opencode"))
	}
	candidates = append(candidates, userHomeJoin(".local", "share", "opencode"))
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func openCodeStorageDir() string {
	return filepath.Join(openCodeBaseDir(), "storage")
}

func clampPositiveInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func terminalClampInt(value int, min int, max int, fallback int) int {
	if value <= 0 {
		value = fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func launchTerminalCommand(command string, cwd string) error {
	switch runtime.GOOS {
	case "windows":
		if err := launchWindowsTerminal(command, cwd); err == nil {
			return nil
		}
		return launchWindowsCmd(command, cwd)
	case "darwin":
		return launchMacTerminal(command, cwd)
	default:
		return launchLinuxTerminal(command, cwd)
	}
}

func launchWindowsTerminal(command string, cwd string) error {
	args := []string{}
	if cwd != "" {
		args = append(args, "-d", cwd)
	}
	args = append(args, "cmd", "/k", command)
	return exec.Command("wt.exe", args...).Start()
}

func launchWindowsCmd(command string, cwd string) error {
	args := []string{"/C", "start", ""}
	if cwd != "" {
		args = append(args, "/D", cwd)
	}
	args = append(args, "cmd", "/K", command)
	return exec.Command("cmd", args...).Start()
}

func launchMacTerminal(command string, cwd string) error {
	fullCommand := command
	if cwd != "" {
		fullCommand = "cd " + shellQuote(cwd) + " && " + command
	}
	script := fmt.Sprintf(`tell application "Terminal"
activate
do script "%s"
end tell`, escapeAppleScript(fullCommand))
	return exec.Command("osascript", "-e", script).Start()
}

func launchLinuxTerminal(command string, cwd string) error {
	candidates := [][]string{
		{"x-terminal-emulator", "-e", "sh", "-lc", command},
		{"gnome-terminal", "--", "sh", "-lc", command},
		{"konsole", "-e", "sh", "-lc", command},
		{"xterm", "-e", "sh", "-lc", command},
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err != nil {
			continue
		}
		cmd := exec.Command(candidate[0], candidate[1:]...)
		if cwd != "" {
			cmd.Dir = cwd
		}
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	return errors.New("no supported terminal found")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func escapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
