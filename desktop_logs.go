package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DesktopLogFileInfo struct {
	GroupKey    string `json:"groupKey"`
	GroupLabel  string `json:"groupLabel"`
	SourceKey   string `json:"sourceKey"`
	SourceLabel string `json:"sourceLabel"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type DesktopLogSnapshot struct {
	Files []DesktopLogFileInfo `json:"files"`
}

type DesktopLogContent struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updatedAt"`
}

func (a *App) ListDesktopLogFiles() (*DesktopLogSnapshot, error) {
	dirs, err := resolveDesktopLogDirs()
	if err != nil {
		return nil, err
	}

	files := make([]DesktopLogFileInfo, 0, 16)
	seen := map[string]bool{}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read log dir %s failed: %w", dir.path, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !isDesktopLogFile(name) {
				continue
			}

			fullPath := filepath.Join(dir.path, name)
			if seen[fullPath] {
				continue
			}
			seen[fullPath] = true

			info, err := entry.Info()
			if err != nil {
				continue
			}
			groupKey, groupLabel := classifyDesktopLogGroup(name)
			files = append(files, DesktopLogFileInfo{
				GroupKey:    groupKey,
				GroupLabel:  groupLabel,
				SourceKey:   dir.sourceKey,
				SourceLabel: dir.sourceLabel,
				Name:        name,
				Path:        fullPath,
				Size:        info.Size(),
				UpdatedAt:   info.ModTime().UnixMilli(),
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].GroupKey != files[j].GroupKey {
			return files[i].GroupKey < files[j].GroupKey
		}
		if files[i].UpdatedAt != files[j].UpdatedAt {
			return files[i].UpdatedAt > files[j].UpdatedAt
		}
		return files[i].Name < files[j].Name
	})

	return &DesktopLogSnapshot{Files: files}, nil
}

func (a *App) ReadDesktopLogFile(path string) (*DesktopLogContent, error) {
	allowedDirs, err := resolveDesktopLogDirs()
	if err != nil {
		return nil, err
	}

	targetPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("invalid log path: %w", err)
	}
	if !isPathWithinAllowedDirs(targetPath, allowedDirs) {
		return nil, fmt.Errorf("log path out of allowed directories")
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("log path is a directory")
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}

	return &DesktopLogContent{
		Path:      targetPath,
		Name:      filepath.Base(targetPath),
		Content:   string(data),
		Size:      info.Size(),
		UpdatedAt: info.ModTime().UnixMilli(),
	}, nil
}

type desktopLogDir struct {
	path        string
	sourceKey   string
	sourceLabel string
}

func resolveDesktopLogDirs() ([]desktopLogDir, error) {
	dirs := make([]desktopLogDir, 0, 5)
	seen := map[string]bool{}

	addDir := func(path string, sourceKey string, sourceLabel string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return
		}
		if seen[absPath] {
			return
		}
		seen[absPath] = true
		dirs = append(dirs, desktopLogDir{
			path:        absPath,
			sourceKey:   sourceKey,
			sourceLabel: sourceLabel,
		})
	}

	if projectRoot, err := findProjectRoot(); err == nil && projectRoot != "" {
		addDir(filepath.Join(projectRoot, "logs"), "project", "项目 logs")
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		addDir(filepath.Join(wd, "logs"), "cwd", "当前目录 logs")
	}
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		addDir(filepath.Join(exeDir, "logs"), "exe", "EXE 同级 logs")
		addDir(filepath.Join(exeDir, "..", "logs"), "exe_parent", "EXE 上级 logs")
	}
	addDir(resolveRuntimeLogDir(), "runtime", "运行时 logs")

	return dirs, nil
}

func resolveRuntimeRootDirForDesktop() string {
	explicitDir := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_RUNTIME_DIR"))
	if explicitDir != "" {
		if abs, err := filepath.Abs(explicitDir); err == nil {
			return abs
		}
		return explicitDir
	}

	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "BatchApiCheck", "runtime")
}

func isDesktopLogFile(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	return strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".txt")
}

func classifyDesktopLogGroup(name string) (string, string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "fetch"), strings.Contains(lower, "check-keys"), strings.Contains(lower, "proxy"), strings.Contains(lower, "models"), strings.Contains(lower, "nih"):
		return "fetch", "抓取与检测"
	case strings.Contains(lower, "wails"), strings.Contains(lower, "exe_backend"), strings.Contains(lower, "render"):
		return "runtime", "核心运行"
	case strings.Contains(lower, "test"), strings.Contains(lower, "debug"):
		return "debug", "调试与样本"
	default:
		return "other", "其他日志"
	}
}

func isPathWithinAllowedDirs(targetPath string, dirs []desktopLogDir) bool {
	for _, dir := range dirs {
		base := dir.path
		if base == "" {
			continue
		}
		baseAbs, err := filepath.Abs(base)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(baseAbs, targetPath)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func formatDesktopLogUpdatedAt(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.UnixMilli(ts).Format(time.RFC3339)
}
