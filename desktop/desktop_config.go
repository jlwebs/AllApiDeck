package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ManagedAppConfigFile struct {
	AppID   string `json:"appId"`
	AppName string `json:"appName"`
	FileID  string `json:"fileId"`
	Label   string `json:"label"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
}

type ManagedAppConfigSnapshot struct {
	Files []ManagedAppConfigFile `json:"files"`
}

type ManagedAppConfigWrite struct {
	AppID   string `json:"appId"`
	FileID  string `json:"fileId"`
	Content string `json:"content"`
}

type ManagedAppConfigApplyRequest struct {
	Files []ManagedAppConfigWrite `json:"files"`
}

type ManagedAppConfigAppliedFile struct {
	AppID      string `json:"appId"`
	FileID     string `json:"fileId"`
	Path       string `json:"path"`
	BackupPath string `json:"backupPath"`
}

type ManagedAppConfigApplyResult struct {
	Applied []ManagedAppConfigAppliedFile `json:"applied"`
}

type managedConfigTarget struct {
	appID   string
	appName string
	fileID  string
	label   string
	path    string
}

func (a *App) ReadManagedAppConfigFiles(appIDs []string) (*ManagedAppConfigSnapshot, error) {
	targets, err := resolveManagedConfigTargets(appIDs)
	if err != nil {
		return nil, err
	}

	files := make([]ManagedAppConfigFile, 0, len(targets))
	for _, target := range targets {
		content, exists, err := readOptionalTextFile(target.path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s (%s): %w", target.label, target.path, err)
		}

		files = append(files, ManagedAppConfigFile{
			AppID:   target.appID,
			AppName: target.appName,
			FileID:  target.fileID,
			Label:   target.label,
			Path:    target.path,
			Exists:  exists,
			Content: content,
		})
	}

	return &ManagedAppConfigSnapshot{Files: files}, nil
}

func (a *App) ApplyManagedAppConfigFiles(request ManagedAppConfigApplyRequest) (*ManagedAppConfigApplyResult, error) {
	if len(request.Files) == 0 {
		return nil, fmt.Errorf("no files to apply")
	}

	applied := make([]ManagedAppConfigAppliedFile, 0, len(request.Files))
	for _, file := range request.Files {
		target, err := resolveManagedConfigTarget(file.AppID, file.FileID)
		if err != nil {
			return nil, err
		}

		backupPath, err := writeManagedConfigFile(target.path, file.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to write %s (%s): %w", target.label, target.path, err)
		}

		applied = append(applied, ManagedAppConfigAppliedFile{
			AppID:      target.appID,
			FileID:     target.fileID,
			Path:       target.path,
			BackupPath: backupPath,
		})
	}

	return &ManagedAppConfigApplyResult{Applied: applied}, nil
}

func resolveManagedConfigTargets(appIDs []string) ([]managedConfigTarget, error) {
	requested := map[string]bool{}
	if len(appIDs) == 0 {
		requested["claude"] = true
		requested["codex"] = true
		requested["opencode"] = true
		requested["openclaw"] = true
	} else {
		for _, appID := range appIDs {
			requested[appID] = true
		}
	}

	targets := make([]managedConfigTarget, 0, 5)
	for _, appID := range []string{"claude", "codex", "opencode", "openclaw"} {
		if !requested[appID] {
			continue
		}

		switch appID {
		case "claude":
			target, err := resolveManagedConfigTarget("claude", "settings")
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		case "codex":
			authTarget, err := resolveManagedConfigTarget("codex", "auth")
			if err != nil {
				return nil, err
			}
			configTarget, err := resolveManagedConfigTarget("codex", "config")
			if err != nil {
				return nil, err
			}
			targets = append(targets, authTarget, configTarget)
		case "opencode":
			target, err := resolveManagedConfigTarget("opencode", "config")
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		case "openclaw":
			target, err := resolveManagedConfigTarget("openclaw", "config")
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		default:
			return nil, fmt.Errorf("unsupported app: %s", appID)
		}
	}

	return targets, nil
}

func resolveManagedConfigTarget(appID string, fileID string) (managedConfigTarget, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return managedConfigTarget{}, fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	switch appID {
	case "claude":
		if fileID != "settings" {
			return managedConfigTarget{}, fmt.Errorf("unsupported Claude file: %s", fileID)
		}

		configDir := filepath.Join(homeDir, ".claude")
		settingsPath := filepath.Join(configDir, "settings.json")
		legacyPath := filepath.Join(configDir, "claude.json")
		if fileExists(legacyPath) && !fileExists(settingsPath) {
			settingsPath = legacyPath
		}

		return managedConfigTarget{
			appID:   "claude",
			appName: "Claude",
			fileID:  "settings",
			label:   "settings.json",
			path:    settingsPath,
		}, nil
	case "codex":
		configDir := filepath.Join(homeDir, ".codex")
		switch fileID {
		case "auth":
			return managedConfigTarget{
				appID:   "codex",
				appName: "Codex",
				fileID:  "auth",
				label:   "auth.json",
				path:    filepath.Join(configDir, "auth.json"),
			}, nil
		case "config":
			return managedConfigTarget{
				appID:   "codex",
				appName: "Codex",
				fileID:  "config",
				label:   "config.toml",
				path:    filepath.Join(configDir, "config.toml"),
			}, nil
		default:
			return managedConfigTarget{}, fmt.Errorf("unsupported Codex file: %s", fileID)
		}
	case "opencode":
		if fileID != "config" {
			return managedConfigTarget{}, fmt.Errorf("unsupported OpenCode file: %s", fileID)
		}
		return managedConfigTarget{
			appID:   "opencode",
			appName: "OpenCode",
			fileID:  "config",
			label:   "opencode.json",
			path:    filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
		}, nil
	case "openclaw":
		if fileID != "config" {
			return managedConfigTarget{}, fmt.Errorf("unsupported OpenClaw file: %s", fileID)
		}
		return managedConfigTarget{
			appID:   "openclaw",
			appName: "OpenClaw",
			fileID:  "config",
			label:   "openclaw.json",
			path:    filepath.Join(homeDir, ".openclaw", "openclaw.json"),
		}, nil
	default:
		return managedConfigTarget{}, fmt.Errorf("unsupported app: %s", appID)
	}
}

func readOptionalTextFile(path string) (string, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	return string(content), true, nil
}

func writeManagedConfigFile(path string, content string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	backupPath := ""
	if fileExists(path) {
		backupPath = fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
		if err := copySingleFile(path, backupPath); err != nil {
			return "", err
		}
	}

	if err := atomicWriteTextFile(path, content); err != nil {
		return backupPath, err
	}

	return backupPath, nil
}

func atomicWriteTextFile(path string, content string) error {
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return err
	}

	if fileExists(path) {
		if err := os.Remove(path); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

func copySingleFile(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
