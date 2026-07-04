package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ManagedAppToggles struct {
	Claude        bool `json:"claude"`
	ClaudeDesktop bool `json:"claudeDesktop"`
	Codex         bool `json:"codex"`
	Gemini        bool `json:"gemini"`
	OpenCode      bool `json:"opencode"`
	OpenClaw      bool `json:"openclaw"`
}

type ManagedMCPServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	URL         string            `json:"url"`
	Env         map[string]string `json:"env"`
	Raw         map[string]any    `json:"raw"`
	Apps        ManagedAppToggles `json:"apps"`
	Source      string            `json:"source"`
	UpdatedAt   int64             `json:"updatedAt"`
}

type ManagedSkill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Directory   string            `json:"directory"`
	ReadmePath  string            `json:"readmePath"`
	Apps        ManagedAppToggles `json:"apps"`
	Source      string            `json:"source"`
	UpdatedAt   int64             `json:"updatedAt"`
}

type MCPSkillConfigSnapshot struct {
	ConfigPath string             `json:"configPath"`
	MCP        []ManagedMCPServer `json:"mcp"`
	Skills     []ManagedSkill     `json:"skills"`
}

type mcpSkillConfigFile struct {
	MCP    []ManagedMCPServer `json:"mcp"`
	Skills []ManagedSkill     `json:"skills"`
}

func (a *App) GetMCPSkillConfigSnapshot() (MCPSkillConfigSnapshot, error) {
	config, _ := readMCPSkillConfigFile()
	discoveredMCP := discoverMCPServers()
	discoveredSkills := discoverSkills()

	mcpByID := map[string]ManagedMCPServer{}
	discoveredMCPApps := map[string]ManagedAppToggles{}
	for _, server := range discoveredMCP {
		mcpByID[server.ID] = server
		discoveredMCPApps[server.ID] = server.Apps
	}
	for _, server := range config.MCP {
		normalized := normalizeManagedMCPServer(server)
		if normalized.ID == "" {
			continue
		}
		existing := mcpByID[normalized.ID]
		if existing.Raw != nil && normalized.Raw == nil {
			normalized.Raw = existing.Raw
		}
		if normalized.Source == "" {
			normalized.Source = firstNonEmpty(existing.Source, "managed")
		}
		if discoveredApps, ok := discoveredMCPApps[normalized.ID]; ok {
			normalized.Apps = discoveredApps
		}
		mcpByID[normalized.ID] = normalized
	}

	skillByID := map[string]ManagedSkill{}
	discoveredSkillApps := map[string]ManagedAppToggles{}
	for _, skill := range discoveredSkills {
		key := stableSkillKey(skill)
		skillByID[key] = skill
		discoveredSkillApps[key] = skill.Apps
	}
	for _, skill := range config.Skills {
		normalized := normalizeManagedSkill(skill)
		if normalized.ID == "" {
			continue
		}
		key := stableSkillKey(normalized)
		existing := skillByID[key]
		if normalized.Directory == "" {
			normalized.Directory = existing.Directory
		}
		if normalized.ReadmePath == "" {
			normalized.ReadmePath = existing.ReadmePath
		}
		if normalized.Source == "" {
			normalized.Source = firstNonEmpty(existing.Source, "managed")
		}
		normalized.ID = key
		if discoveredApps, ok := discoveredSkillApps[key]; ok {
			normalized.Apps = discoveredApps
		} else {
			normalized.Apps = mergeToggles(existing.Apps, normalized.Apps)
		}
		skillByID[key] = normalized
	}

	return MCPSkillConfigSnapshot{
		ConfigPath: mcpSkillConfigPath(),
		MCP:        sortedManagedMCPServers(mcpByID),
		Skills:     sortedManagedSkills(skillByID),
	}, nil
}

func (a *App) SaveMCPSkillConfigSnapshot(snapshot MCPSkillConfigSnapshot) (bool, error) {
	previousConfig, _ := readMCPSkillConfigFile()
	config := mcpSkillConfigFile{
		MCP:    make([]ManagedMCPServer, 0, len(snapshot.MCP)),
		Skills: make([]ManagedSkill, 0, len(snapshot.Skills)),
	}
	now := time.Now().UnixMilli()
	for _, server := range snapshot.MCP {
		normalized := normalizeManagedMCPServer(server)
		if normalized.ID == "" {
			continue
		}
		if normalized.UpdatedAt == 0 {
			normalized.UpdatedAt = now
		}
		config.MCP = append(config.MCP, normalized)
	}
	for _, skill := range snapshot.Skills {
		normalized := normalizeManagedSkill(skill)
		if normalized.ID == "" {
			continue
		}
		if normalized.UpdatedAt == 0 {
			normalized.UpdatedAt = now
		}
		config.Skills = append(config.Skills, normalized)
	}
	sort.SliceStable(config.MCP, func(i, j int) bool { return strings.ToLower(config.MCP[i].ID) < strings.ToLower(config.MCP[j].ID) })
	sort.SliceStable(config.Skills, func(i, j int) bool {
		return strings.ToLower(config.Skills[i].ID) < strings.ToLower(config.Skills[j].ID)
	})

	path := mcpSkillConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return false, err
	}
	if err := applyMCPSkillConfigSnapshot(config, previousConfig); err != nil {
		return false, err
	}
	return true, nil
}

func applyMCPSkillConfigSnapshot(config mcpSkillConfigFile, previousConfig mcpSkillConfigFile) error {
	allMCP := mergeManagedMCPForApply(config.MCP, previousConfig.MCP)
	allSkills := mergeManagedSkillsForApply(config.Skills, previousConfig.Skills)
	if err := applyMCPServersForApp("claude", userHomeJoin(".claude.json"), allMCP); err != nil {
		return err
	}
	if err := applyMCPServersForApp("claude-desktop", claudeDesktopConfigPath(), allMCP); err != nil {
		return err
	}
	if err := applyCodexMCPServers(allMCP); err != nil {
		return err
	}
	if err := applyMCPServersForApp("gemini", userHomeJoin(".gemini", "settings.json"), allMCP); err != nil {
		return err
	}
	if err := applyMCPServersForApp("opencode", filepath.Join(openCodeBaseDir(), "opencode.json"), allMCP); err != nil {
		return err
	}
	if err := applyMCPServersForApp("openclaw", openClawConfigPath(), allMCP); err != nil {
		return err
	}
	if err := applySkillsForApp("claude", userHomeJoin(".claude", "skills"), allSkills); err != nil {
		return err
	}
	if err := applySkillsForApp("claude-desktop", userHomeJoin(".claude", "skills"), allSkills); err != nil {
		return err
	}
	if err := applySkillsForApp("codex", userHomeJoin(".codex", "skills"), allSkills); err != nil {
		return err
	}
	if err := applyCodexSkillConfig(allSkills); err != nil {
		return err
	}
	if err := applySkillsForApp("gemini", userHomeJoin(".gemini", "skills"), allSkills); err != nil {
		return err
	}
	if err := applySkillsForApp("opencode", filepath.Join(openCodeBaseDir(), "skills"), allSkills); err != nil {
		return err
	}
	if err := applySkillsForApp("openclaw", userHomeJoin(".openclaw", "skills"), allSkills); err != nil {
		return err
	}
	return nil
}

func mergeManagedMCPForApply(current []ManagedMCPServer, previous []ManagedMCPServer) []ManagedMCPServer {
	byID := map[string]ManagedMCPServer{}
	for _, item := range previous {
		normalized := normalizeManagedMCPServer(item)
		if normalized.ID == "" {
			continue
		}
		normalized.Apps = ManagedAppToggles{}
		byID[normalized.ID] = normalized
	}
	for _, item := range current {
		normalized := normalizeManagedMCPServer(item)
		if normalized.ID != "" {
			byID[normalized.ID] = normalized
		}
	}
	result := make([]ManagedMCPServer, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	return result
}

func mergeManagedSkillsForApply(current []ManagedSkill, previous []ManagedSkill) []ManagedSkill {
	byID := map[string]ManagedSkill{}
	for _, item := range previous {
		normalized := normalizeManagedSkill(item)
		if normalized.ID == "" {
			continue
		}
		normalized.Apps = ManagedAppToggles{}
		byID[stableSkillKey(normalized)] = normalized
	}
	for _, item := range current {
		normalized := normalizeManagedSkill(item)
		if normalized.ID != "" {
			byID[stableSkillKey(normalized)] = normalized
		}
	}
	result := make([]ManagedSkill, 0, len(byID))
	for _, item := range byID {
		result = append(result, item)
	}
	return result
}

func applyMCPServersForApp(app string, path string, servers []ManagedMCPServer) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &root)
	}
	rawServers := map[string]any{}
	if existing, ok := root["mcpServers"].(map[string]any); ok {
		for key, value := range existing {
			rawServers[key] = value
		}
	}
	for _, server := range servers {
		normalized := normalizeManagedMCPServer(server)
		if normalized.ID == "" {
			continue
		}
		if toggleForApp(normalized.Apps, app) {
			rawServers[normalized.ID] = rawMCPServerConfig(normalized)
		} else {
			delete(rawServers, normalized.ID)
		}
	}
	if len(rawServers) == 0 {
		delete(root, "mcpServers")
	} else {
		root["mcpServers"] = rawServers
	}
	return writeJSONFile(path, root, 0o600)
}

func applyCodexMCPServers(servers []ManagedMCPServer) error {
	path := userHomeJoin(".codex", "config.toml")
	text := ""
	if data, err := os.ReadFile(path); err == nil {
		text = string(data)
	}
	managedIDs := map[string]bool{}
	blocks := []string{}
	for _, server := range servers {
		normalized := normalizeManagedMCPServer(server)
		if normalized.ID == "" {
			continue
		}
		managedIDs[normalized.ID] = true
		if toggleForApp(normalized.Apps, "codex") {
			blocks = append(blocks, renderCodexMCPServerBlock(normalized))
		}
	}
	text = removeCodexMCPServerBlocks(text, managedIDs)
	if len(blocks) > 0 {
		text = strings.TrimRight(text, "\r\n")
		if text != "" {
			text += "\n\n"
		}
		text += strings.Join(blocks, "\n\n") + "\n"
	}
	if strings.TrimSpace(text) == "" {
		text = "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

func renderCodexMCPServerBlock(server ManagedMCPServer) string {
	lines := []string{fmt.Sprintf("[mcp_servers.%s]", tomlBareKey(server.ID))}
	if server.Command != "" {
		lines = append(lines, fmt.Sprintf("command = %s", tomlQuote(server.Command)))
	}
	if len(server.Args) > 0 {
		quoted := make([]string, 0, len(server.Args))
		for _, arg := range server.Args {
			quoted = append(quoted, tomlQuote(arg))
		}
		lines = append(lines, fmt.Sprintf("args = [ %s ]", strings.Join(quoted, ", ")))
	}
	if server.URL != "" {
		lines = append(lines, fmt.Sprintf("url = %s", tomlQuote(server.URL)))
	}
	if len(server.Env) > 0 {
		keys := make([]string, 0, len(server.Env))
		for key := range server.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines = append(lines, "", fmt.Sprintf("[mcp_servers.%s.env]", tomlBareKey(server.ID)))
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s = %s", tomlBareKey(key), tomlQuote(server.Env[key])))
		}
	}
	return strings.Join(lines, "\n")
}

func removeCodexMCPServerBlocks(text string, ids map[string]bool) string {
	if len(ids) == 0 || strings.TrimSpace(text) == "" {
		return text
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	skipID := ""
	for _, line := range lines {
		section, ok := parseTomlSection(line)
		if ok {
			if id, matched := codexMCPSectionID(section); matched && ids[id] {
				skipID = id
				continue
			}
			skipID = ""
		}
		if skipID != "" {
			continue
		}
		result = append(result, line)
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}

func codexMCPSectionID(section string) (string, bool) {
	if strings.HasPrefix(section, "mcp_servers.") {
		rest := strings.TrimPrefix(section, "mcp_servers.")
		return strings.TrimSuffix(rest, ".env"), true
	}
	return "", false
}

func applyCodexSkillConfig(skills []ManagedSkill) error {
	path := userHomeJoin(".codex", "config.toml")
	text := ""
	if data, err := os.ReadFile(path); err == nil {
		text = string(data)
	}
	managedPaths := map[string]bool{}
	blocks := []string{}
	for _, skill := range skills {
		normalized := normalizeManagedSkill(skill)
		if normalized.ID == "" {
			continue
		}
		targetPath := filepath.Join(userHomeJoin(".codex", "skills"), skillDirectoryName(normalized), "SKILL.md")
		if normalizeTextPath(targetPath) != "" {
			managedPaths[normalizeTextPath(targetPath)] = true
		}
		if normalizeTextPath(normalized.ReadmePath) != "" {
			managedPaths[normalizeTextPath(normalized.ReadmePath)] = true
		}
		sourceTargetPath := filepath.Join(filepath.Dir(normalized.ReadmePath), "SKILL.md")
		if normalizeTextPath(sourceTargetPath) != "" {
			managedPaths[normalizeTextPath(sourceTargetPath)] = true
		}
		if toggleForApp(normalized.Apps, "codex") {
			blocks = append(blocks,
				"[[skills.config]]\n"+
					fmt.Sprintf("path = %s\n", tomlQuote(targetPath))+
					"enabled = true",
			)
		}
	}
	text = removeCodexSkillBlocks(text, managedPaths)
	if len(blocks) > 0 {
		text = strings.TrimRight(text, "\r\n")
		if text != "" {
			text += "\n\n"
		}
		text += strings.Join(blocks, "\n\n") + "\n"
	}
	if strings.TrimSpace(text) == "" {
		text = "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

func removeCodexSkillBlocks(text string, managedPaths map[string]bool) string {
	if len(managedPaths) == 0 || strings.TrimSpace(text) == "" {
		return text
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		if strings.TrimSpace(lines[index]) != "[[skills.config]]" {
			result = append(result, lines[index])
			index++
			continue
		}
		blockStart := index
		block := []string{lines[index]}
		index++
		for index < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[index]), "[") {
			block = append(block, lines[index])
			index++
		}
		pathValue := ""
		for _, line := range block {
			key, value, ok := parseTomlKeyValue(line)
			if ok && key == "path" {
				pathValue = normalizeTextPath(unquoteTomlString(value))
				break
			}
		}
		if pathValue != "" && managedPaths[pathValue] {
			_ = blockStart
			continue
		}
		result = append(result, block...)
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}

func rawMCPServerConfig(server ManagedMCPServer) map[string]any {
	raw := map[string]any{}
	for key, value := range server.Raw {
		raw[key] = value
	}
	if server.Type != "" {
		raw["type"] = server.Type
	}
	if server.Command != "" {
		raw["command"] = server.Command
	}
	if len(server.Args) > 0 {
		raw["args"] = server.Args
	}
	if server.URL != "" {
		raw["url"] = server.URL
	}
	if len(server.Env) > 0 {
		raw["env"] = server.Env
	}
	return raw
}

func applySkillsForApp(app string, root string, skills []ManagedSkill) error {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	managedNames := map[string]bool{}
	for _, skill := range skills {
		normalized := normalizeManagedSkill(skill)
		name := skillDirectoryName(normalized)
		if normalized.ID == "" || name == "" {
			continue
		}
		managedNames[name] = true
		target := filepath.Join(root, name)
		if toggleForApp(normalized.Apps, app) {
			if sameCleanPath(normalized.Directory, target) {
				continue
			}
			if err := copyDirectoryReplace(normalized.Directory, target); err != nil {
				return err
			}
		} else if sameCleanPath(filepath.Dir(target), root) {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
	}
	_ = managedNames
	return nil
}

func toggleForApp(toggles ManagedAppToggles, app string) bool {
	switch strings.ToLower(strings.TrimSpace(app)) {
	case "claude":
		return toggles.Claude
	case "claude-desktop":
		return toggles.ClaudeDesktop
	case "codex":
		return toggles.Codex
	case "gemini":
		return toggles.Gemini
	case "opencode":
		return toggles.OpenCode
	case "openclaw":
		return toggles.OpenClaw
	default:
		return false
	}
}

func skillDirectoryName(skill ManagedSkill) string {
	if base := strings.TrimSpace(filepath.Base(skill.Directory)); base != "" && base != "." && base != string(filepath.Separator) {
		return sanitizePathSegment(base)
	}
	return sanitizePathSegment(firstNonEmpty(skill.Name, skill.ID))
}

func sanitizePathSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return strings.Trim(replacer.Replace(strings.TrimSpace(value)), ". ")
}

func sameCleanPath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func writeJSONFile(path string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), mode)
}

func copyDirectoryReplace(src string, dst string) error {
	if strings.TrimSpace(src) == "" {
		return nil
	}
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()
		targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer targetFile.Close()
		_, err = io.Copy(targetFile, sourceFile)
		return err
	})
}

func readMCPSkillConfigFile() (mcpSkillConfigFile, error) {
	path := mcpSkillConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mcpSkillConfigFile{}, nil
		}
		return mcpSkillConfigFile{}, err
	}
	var config mcpSkillConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return mcpSkillConfigFile{}, err
	}
	return config, nil
}

func mcpSkillConfigPath() string {
	return filepath.Join(appConfigDir(), "mcp-skill-config.json")
}

func appConfigDir() string {
	if value := strings.TrimSpace(os.Getenv("ALL_API_DECK_CONFIG_DIR")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_RUNTIME_DIR")); value != "" {
		return value
	}
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "AllApiDeck")
	}
	return userHomeJoin(".all-api-deck")
}

func discoverMCPServers() []ManagedMCPServer {
	result := []ManagedMCPServer{}
	result = append(result, discoverMCPFromJSONFile(userHomeJoin(".claude.json"), "claude")...)
	result = append(result, discoverMCPFromJSONFile(claudeDesktopConfigPath(), "claude-desktop")...)
	result = append(result, discoverMCPFromJSONFile(userHomeJoin(".codex", "config.json"), "codex")...)
	result = append(result, discoverCodexMCPFromTOML(userHomeJoin(".codex", "config.toml"))...)
	result = append(result, discoverMCPFromJSONFile(userHomeJoin(".gemini", "settings.json"), "gemini")...)
	result = append(result, discoverMCPFromJSONFile(filepath.Join(openCodeBaseDir(), "opencode.json"), "opencode")...)
	result = append(result, discoverMCPFromJSONFile(openClawConfigPath(), "openclaw")...)
	return dedupeMCPServers(result)
}

func claudeDesktopConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("CLAUDE_DESKTOP_CONFIG_PATH")); value != "" {
		return value
	}
	if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
		return filepath.Join(appData, "Claude", "claude_desktop_config.json")
	}
	if configDir, err := os.UserConfigDir(); err == nil && configDir != "" {
		return filepath.Join(configDir, "Claude", "claude_desktop_config.json")
	}
	return userHomeJoin(".config", "Claude", "claude_desktop_config.json")
}

func openClawConfigPath() string {
	if value := strings.TrimSpace(os.Getenv("OPENCLAW_CONFIG_PATH")); value != "" {
		return value
	}
	return userHomeJoin(".openclaw", "openclaw.json")
}

func discoverMCPFromJSONFile(path string, app string) []ManagedMCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	serversValue := root["mcpServers"]
	if serversValue == nil {
		serversValue = root["mcp_servers"]
	}
	servers, ok := serversValue.(map[string]any)
	if !ok {
		return nil
	}
	result := make([]ManagedMCPServer, 0, len(servers))
	for id, raw := range servers {
		rawMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		server := managedMCPFromRaw(id, rawMap, app, path)
		if server.ID != "" {
			result = append(result, server)
		}
	}
	return result
}

func managedMCPFromRaw(id string, raw map[string]any, app string, source string) ManagedMCPServer {
	serverType := firstNonEmpty(stringValue(raw["type"]), inferMCPServerType(raw))
	server := ManagedMCPServer{
		ID:      strings.TrimSpace(id),
		Name:    firstNonEmpty(stringValue(raw["name"]), strings.TrimSpace(id)),
		Type:    serverType,
		Command: stringValue(raw["command"]),
		URL:     firstNonEmpty(stringValue(raw["url"]), stringValue(raw["endpoint"])),
		Args:    stringSliceValue(raw["args"]),
		Env:     stringMapValue(raw["env"]),
		Raw:     raw,
		Source:  source,
	}
	server.Apps = togglesForApp(app)
	return normalizeManagedMCPServer(server)
}

func discoverCodexMCPFromTOML(path string) []ManagedMCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	servers := map[string]ManagedMCPServer{}
	currentID := ""
	inEnv := false
	for _, line := range lines {
		section, ok := parseTomlSection(line)
		if ok {
			if id, matched := codexMCPSectionID(section); matched {
				currentID = strings.TrimSpace(id)
				inEnv = strings.HasSuffix(section, ".env")
				if currentID != "" {
					server := servers[currentID]
					server.ID = currentID
					server.Name = firstNonEmpty(server.Name, currentID)
					server.Type = firstNonEmpty(server.Type, "stdio")
					server.Source = path
					server.Apps = togglesForApp("codex")
					if server.Env == nil {
						server.Env = map[string]string{}
					}
					servers[currentID] = server
				}
				continue
			}
			currentID = ""
			inEnv = false
			continue
		}
		if currentID == "" {
			continue
		}
		key, value, ok := parseTomlKeyValue(line)
		if !ok {
			continue
		}
		server := servers[currentID]
		if inEnv {
			if server.Env == nil {
				server.Env = map[string]string{}
			}
			server.Env[key] = unquoteTomlString(value)
		} else {
			switch key {
			case "command":
				server.Command = unquoteTomlString(value)
			case "args":
				server.Args = parseTomlStringArray(value)
			case "url":
				server.URL = unquoteTomlString(value)
				server.Type = "http"
			case "type":
				server.Type = unquoteTomlString(value)
			}
		}
		servers[currentID] = server
	}
	result := make([]ManagedMCPServer, 0, len(servers))
	for _, server := range servers {
		result = append(result, normalizeManagedMCPServer(server))
	}
	return result
}

func inferMCPServerType(raw map[string]any) string {
	if stringValue(raw["url"]) != "" || stringValue(raw["endpoint"]) != "" {
		return "http"
	}
	return "stdio"
}

func discoverSkills() []ManagedSkill {
	roots := []struct {
		path string
		app  string
	}{
		{userHomeJoin(".agents", "skills"), "codex"},
		{userHomeJoin(".codex", "skills"), "codex"},
		{userHomeJoin(".claude", "skills"), "claude"},
		{userHomeJoin(".gemini", "skills"), "gemini"},
		{filepath.Join(openCodeBaseDir(), "skills"), "opencode"},
		{userHomeJoin(".openclaw", "skills"), "openclaw"},
	}
	result := []ManagedSkill{}
	for _, root := range roots {
		result = append(result, discoverSkillsInRoot(root.path, root.app)...)
	}
	return dedupeSkills(result)
}

func discoverSkillsInRoot(root string, app string) []ManagedSkill {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	result := []ManagedSkill{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		readmePath := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(readmePath); err != nil {
			continue
		}
		name, description := parseSkillMetadata(readmePath, entry.Name())
		skill := normalizeManagedSkill(ManagedSkill{
			ID:          stableSkillID(dir),
			Name:        name,
			Description: description,
			Directory:   dir,
			ReadmePath:  readmePath,
			Apps:        togglesForApp(app),
			Source:      root,
		})
		if skill.ID != "" {
			result = append(result, skill)
		}
	}
	return result
}

func parseSkillMetadata(path string, fallback string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback, ""
	}
	lines := strings.Split(string(data), "\n")
	name := fallback
	description := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && name == fallback {
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			continue
		}
		lowered := strings.ToLower(trimmed)
		if strings.HasPrefix(lowered, "description:") {
			description = strings.TrimSpace(trimmed[len("description:"):])
			description = strings.Trim(description, `"'`)
			break
		}
		if description == "" && trimmed != "" && !strings.HasPrefix(trimmed, "---") && !strings.HasPrefix(trimmed, "#") {
			description = trimmed
		}
		if name != fallback && description != "" {
			break
		}
	}
	return firstNonEmpty(name, fallback), description
}

func normalizeManagedMCPServer(server ManagedMCPServer) ManagedMCPServer {
	server.ID = strings.TrimSpace(server.ID)
	server.Name = firstNonEmpty(strings.TrimSpace(server.Name), server.ID)
	server.Type = strings.ToLower(firstNonEmpty(strings.TrimSpace(server.Type), "stdio"))
	server.Command = strings.TrimSpace(server.Command)
	server.URL = strings.TrimSpace(server.URL)
	server.Description = strings.TrimSpace(server.Description)
	server.Source = strings.TrimSpace(server.Source)
	server.Args = compactStringSlice(server.Args)
	if server.Env == nil {
		server.Env = map[string]string{}
	}
	if server.Raw == nil {
		server.Raw = map[string]any{}
		if server.Type != "" {
			server.Raw["type"] = server.Type
		}
		if server.Command != "" {
			server.Raw["command"] = server.Command
		}
		if len(server.Args) > 0 {
			server.Raw["args"] = server.Args
		}
		if server.URL != "" {
			server.Raw["url"] = server.URL
		}
		if len(server.Env) > 0 {
			server.Raw["env"] = server.Env
		}
	}
	server.Apps = normalizeManagedAppToggles(server.Apps)
	return server
}

func normalizeManagedSkill(skill ManagedSkill) ManagedSkill {
	skill.ID = firstNonEmpty(strings.TrimSpace(skill.ID), stableSkillID(skill.Directory))
	skill.Name = firstNonEmpty(strings.TrimSpace(skill.Name), filepath.Base(skill.Directory), skill.ID)
	skill.Description = strings.TrimSpace(skill.Description)
	skill.Directory = strings.TrimSpace(skill.Directory)
	skill.ReadmePath = strings.TrimSpace(skill.ReadmePath)
	skill.Source = strings.TrimSpace(skill.Source)
	skill.Apps = normalizeManagedAppToggles(skill.Apps)
	return skill
}

func normalizeManagedAppToggles(toggles ManagedAppToggles) ManagedAppToggles {
	if toggles.ClaudeDesktop {
		toggles.Claude = true
	}
	return toggles
}

func togglesForApp(app string) ManagedAppToggles {
	app = strings.ToLower(strings.TrimSpace(app))
	return ManagedAppToggles{
		Claude:        app == "claude" || app == "claude-desktop",
		ClaudeDesktop: app == "claude-desktop",
		Codex:         app == "codex",
		Gemini:        app == "gemini",
		OpenCode:      app == "opencode",
		OpenClaw:      app == "openclaw",
	}
}

func dedupeMCPServers(items []ManagedMCPServer) []ManagedMCPServer {
	byID := map[string]ManagedMCPServer{}
	for _, item := range items {
		normalized := normalizeManagedMCPServer(item)
		if normalized.ID == "" {
			continue
		}
		existing := byID[normalized.ID]
		normalized.Apps = mergeToggles(existing.Apps, normalized.Apps)
		if existing.Source != "" && normalized.Source != existing.Source {
			normalized.Source = existing.Source + "; " + normalized.Source
		}
		byID[normalized.ID] = normalized
	}
	return sortedManagedMCPServers(byID)
}

func dedupeSkills(items []ManagedSkill) []ManagedSkill {
	byID := map[string]ManagedSkill{}
	for _, item := range items {
		normalized := normalizeManagedSkill(item)
		if normalized.ID == "" {
			continue
		}
		key := stableSkillKey(normalized)
		existing := byID[key]
		normalized.Apps = mergeToggles(existing.Apps, normalized.Apps)
		if normalized.Directory == "" {
			normalized.Directory = existing.Directory
		}
		if normalized.ReadmePath == "" {
			normalized.ReadmePath = existing.ReadmePath
		}
		if existing.Source != "" && normalized.Source != existing.Source {
			normalized.Source = existing.Source + "; " + normalized.Source
		}
		normalized.ID = key
		byID[key] = normalized
	}
	return sortedManagedSkills(byID)
}

func mergeToggles(left ManagedAppToggles, right ManagedAppToggles) ManagedAppToggles {
	return ManagedAppToggles{
		Claude:        left.Claude || right.Claude,
		ClaudeDesktop: left.ClaudeDesktop || right.ClaudeDesktop,
		Codex:         left.Codex || right.Codex,
		Gemini:        left.Gemini || right.Gemini,
		OpenCode:      left.OpenCode || right.OpenCode,
		OpenClaw:      left.OpenClaw || right.OpenClaw,
	}
}

func sortedManagedMCPServers(items map[string]ManagedMCPServer) []ManagedMCPServer {
	result := make([]ManagedMCPServer, 0, len(items))
	for _, item := range items {
		result = append(result, normalizeManagedMCPServer(item))
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func sortedManagedSkills(items map[string]ManagedSkill) []ManagedSkill {
	result := make([]ManagedSkill, 0, len(items))
	for _, item := range items {
		result = append(result, normalizeManagedSkill(item))
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func stableSkillID(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(filepath.ToSlash(normalized), "/", "::"))
}

func stableSkillKey(skill ManagedSkill) string {
	if key := stableSkillID(skill.Directory); key != "" {
		return key
	}
	if key := stableSkillID(filepath.Dir(skill.ReadmePath)); key != "" {
		return key
	}
	return stableSkillID(skill.ID)
}

func parseTomlSection(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "[["), "]]")), true
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")), true
	}
	return "", false
}

func parseTomlKeyValue(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	index := strings.Index(trimmed, "=")
	if index < 0 {
		return "", "", false
	}
	return strings.TrimSpace(trimmed[:index]), strings.TrimSpace(trimmed[index+1:]), true
}

func tomlBareKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return `""`
	}
	for _, r := range trimmed {
		if !(r == '_' || r == '-' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return tomlQuote(trimmed)
		}
	}
	return trimmed
}

func tomlQuote(value string) string {
	escaped, _ := json.Marshal(strings.TrimSpace(value))
	return string(escaped)
}

func unquoteTomlString(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") && len(trimmed) >= 2 {
		return strings.Trim(trimmed, "'")
	}
	var result string
	if err := json.Unmarshal([]byte(trimmed), &result); err == nil {
		return result
	}
	return strings.Trim(trimmed, `"`)
}

func parseTomlStringArray(value string) []string {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return stringSliceValue(unquoteTomlString(trimmed))
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
	if body == "" {
		return nil
	}
	result := []string{}
	current := strings.Builder{}
	quote := rune(0)
	escaped := false
	for _, r := range body {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if quote == '"' && r == '\\' {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			current.WriteRune(r)
			continue
		}
		if r == ',' {
			if text := unquoteTomlString(current.String()); text != "" {
				result = append(result, text)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if text := unquoteTomlString(current.String()); text != "" {
		result = append(result, text)
	}
	return result
}

func normalizeTextPath(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	abs, err := filepath.Abs(text)
	if err == nil {
		text = abs
	}
	return strings.ToLower(filepath.Clean(text))
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return compactStringSlice(typed)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringValue(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return strings.Fields(typed)
	default:
		return nil
	}
}

func compactStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func stringMapValue(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		if typed, ok := value.(map[string]string); ok {
			return typed
		}
		return map[string]string{}
	}
	result := map[string]string{}
	for key, item := range raw {
		if text := stringValue(item); text != "" {
			result[key] = text
		}
	}
	return result
}

func appToggleCount(toggles ManagedAppToggles) int {
	count := 0
	if toggles.Claude {
		count++
	}
	if toggles.ClaudeDesktop {
		count++
	}
	if toggles.Codex {
		count++
	}
	if toggles.Gemini {
		count++
	}
	if toggles.OpenCode {
		count++
	}
	if toggles.OpenClaw {
		count++
	}
	return count
}

func (toggles ManagedAppToggles) String() string {
	parts := []string{}
	if toggles.Claude {
		parts = append(parts, "claude")
	}
	if toggles.ClaudeDesktop {
		parts = append(parts, "claude-desktop")
	}
	if toggles.Codex {
		parts = append(parts, "codex")
	}
	if toggles.Gemini {
		parts = append(parts, "gemini")
	}
	if toggles.OpenCode {
		parts = append(parts, "opencode")
	}
	if toggles.OpenClaw {
		parts = append(parts, "openclaw")
	}
	return fmt.Sprintf("%d:%s", appToggleCount(toggles), strings.Join(parts, ","))
}
