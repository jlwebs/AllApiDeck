package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetMCPSkillConfigSnapshotKeepsSkillsInOwningApp(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("ALL_API_DECK_CONFIG_DIR", filepath.Join(tempDir, "runtime"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "xdg-data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(tempDir, "local-app-data"))

	codexSkillDir := filepath.Join(tempDir, ".codex", "skills", "codex-only")
	geminiSkillDir := filepath.Join(tempDir, ".gemini", "skills", "agents-sdk")
	opencodeSkillDir := filepath.Join(tempDir, "xdg-data", "opencode", "skills", "agent-ssh-cli")
	writeSkill(t, codexSkillDir, "codex-only", "Codex skill")
	writeSkill(t, geminiSkillDir, "agents-sdk", "Gemini skill")
	writeSkill(t, opencodeSkillDir, "agent-ssh-cli", "OpenCode skill")

	staleConfig := mcpSkillConfigFile{
		Skills: []ManagedSkill{
			{
				ID:          stableSkillID(geminiSkillDir),
				Name:        "agents-sdk",
				Directory:   geminiSkillDir,
				ReadmePath:  filepath.Join(geminiSkillDir, "SKILL.md"),
				Apps:        ManagedAppToggles{Codex: true, Gemini: true},
				Source:      "stale",
				Description: "stale app binding should not win",
			},
			{
				ID:          stableSkillID(opencodeSkillDir),
				Name:        "agent-ssh-cli",
				Directory:   opencodeSkillDir,
				ReadmePath:  filepath.Join(opencodeSkillDir, "SKILL.md"),
				Apps:        ManagedAppToggles{Codex: true, OpenCode: true},
				Source:      "stale",
				Description: "stale app binding should not win",
			},
		},
	}
	writeMCPConfig(t, staleConfig)

	snapshot, err := (&App{}).GetMCPSkillConfigSnapshot()
	if err != nil {
		t.Fatalf("GetMCPSkillConfigSnapshot returned error: %v", err)
	}

	for _, skill := range snapshot.Skills {
		switch normalizeTextPath(skill.Directory) {
		case normalizeTextPath(codexSkillDir):
			if !skill.Apps.Codex || skill.Apps.Gemini || skill.Apps.OpenCode {
				t.Fatalf("codex skill has wrong apps: %#v", skill.Apps)
			}
		case normalizeTextPath(geminiSkillDir):
			if skill.Apps.Codex || !skill.Apps.Gemini || skill.Apps.OpenCode {
				t.Fatalf("gemini skill leaked into codex/opencode: %#v", skill.Apps)
			}
		case normalizeTextPath(opencodeSkillDir):
			if skill.Apps.Codex || skill.Apps.Gemini || !skill.Apps.OpenCode {
				t.Fatalf("opencode skill leaked into codex/gemini: %#v", skill.Apps)
			}
		}
	}
}

func TestGetMCPSkillConfigSnapshotKeepsMCPServersInOwningApp(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("ALL_API_DECK_CONFIG_DIR", filepath.Join(tempDir, "runtime"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tempDir, "xdg-data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(tempDir, "local-app-data"))

	geminiConfigPath := filepath.Join(tempDir, ".gemini", "settings.json")
	opencodeConfigPath := filepath.Join(tempDir, "xdg-data", "opencode", "opencode.json")
	writeMCPJSON(t, geminiConfigPath, map[string]any{
		"gemini-only": map[string]any{"command": "gemini-mcp"},
	})
	writeMCPJSON(t, opencodeConfigPath, map[string]any{
		"opencode-only": map[string]any{"command": "opencode-mcp"},
	})

	staleConfig := mcpSkillConfigFile{
		MCP: []ManagedMCPServer{
			{
				ID:      "gemini-only",
				Name:    "gemini-only",
				Command: "gemini-mcp",
				Apps:    ManagedAppToggles{Codex: true, Gemini: true},
				Source:  "stale",
			},
			{
				ID:      "opencode-only",
				Name:    "opencode-only",
				Command: "opencode-mcp",
				Apps:    ManagedAppToggles{Codex: true, OpenCode: true},
				Source:  "stale",
			},
		},
	}
	writeMCPConfig(t, staleConfig)

	snapshot, err := (&App{}).GetMCPSkillConfigSnapshot()
	if err != nil {
		t.Fatalf("GetMCPSkillConfigSnapshot returned error: %v", err)
	}

	for _, server := range snapshot.MCP {
		switch server.ID {
		case "gemini-only":
			if server.Apps.Codex || !server.Apps.Gemini || server.Apps.OpenCode {
				t.Fatalf("gemini MCP leaked into codex/opencode: %#v", server.Apps)
			}
		case "opencode-only":
			if server.Apps.Codex || server.Apps.Gemini || !server.Apps.OpenCode {
				t.Fatalf("opencode MCP leaked into codex/gemini: %#v", server.Apps)
			}
		}
	}
}

func TestGetMCPSkillConfigSnapshotDiscoversCodexMCPFromTOML(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("ALL_API_DECK_CONFIG_DIR", filepath.Join(tempDir, "runtime"))

	codexConfigPath := filepath.Join(tempDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(codexConfigPath), 0o755); err != nil {
		t.Fatalf("mkdir codex config dir: %v", err)
	}
	codexConfig := `[mcp_servers.codex_local]
command = "node"
args = [ "server.js", "--stdio" ]

[mcp_servers.codex_local.env]
TOKEN = "abc"
`
	if err := os.WriteFile(codexConfigPath, []byte(codexConfig), 0o644); err != nil {
		t.Fatalf("write codex config: %v", err)
	}

	snapshot, err := (&App{}).GetMCPSkillConfigSnapshot()
	if err != nil {
		t.Fatalf("GetMCPSkillConfigSnapshot returned error: %v", err)
	}

	var found *ManagedMCPServer
	for index := range snapshot.MCP {
		if snapshot.MCP[index].ID == "codex_local" {
			found = &snapshot.MCP[index]
			break
		}
	}
	if found == nil {
		t.Fatal("expected codex_local MCP server from .codex/config.toml")
	}
	if !found.Apps.Codex || found.Apps.Gemini || found.Apps.OpenCode {
		t.Fatalf("codex TOML MCP has wrong apps: %#v", found.Apps)
	}
	if found.Command != "node" || len(found.Args) != 2 || found.Args[0] != "server.js" || found.Env["TOKEN"] != "abc" {
		t.Fatalf("codex TOML MCP parsed incorrectly: %#v", *found)
	}
}

func writeSkill(t *testing.T, dir string, name string, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := "# " + name + "\n\ndescription: " + description + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
}

func writeMCPJSON(t *testing.T, path string, servers map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir MCP config dir: %v", err)
	}
	data, err := json.Marshal(map[string]any{"mcpServers": servers})
	if err != nil {
		t.Fatalf("marshal MCP config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write MCP config: %v", err)
	}
}

func writeMCPConfig(t *testing.T, config mcpSkillConfigFile) {
	t.Helper()
	path := mcpSkillConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
