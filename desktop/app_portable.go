package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func appendPortableDataLogf(format string, args ...any) {
	appendLine(filepath.Join(resolveRuntimeLogDir(), "portable-data.log"), fmt.Sprintf(format, args...))
}

type PortableDataPackageResult struct {
	BackupDir              string `json:"backupDir"`
	RuntimeSourceDir       string `json:"runtimeSourceDir"`
	RuntimeBackupDir       string `json:"runtimeBackupDir"`
	LocalStorageBackupPath string `json:"localStorageBackupPath"`
	LocalStorageKeyCount   int    `json:"localStorageKeyCount"`
}

type PortableDataUnpackResult struct {
	BackupDir              string `json:"backupDir"`
	RuntimeBackupDir       string `json:"runtimeBackupDir"`
	LocalStorageBackupPath string `json:"localStorageBackupPath"`
	LocalStorageJSON       string `json:"localStorageJson"`
	LocalStorageKeyCount   int    `json:"localStorageKeyCount"`
}

func (a *App) PackagePortableData(localStorageJSON string) (*PortableDataPackageResult, error) {
	backupDir, err := resolvePortableBackupDir()
	if err != nil {
		appendPortableDataLogf("[PACKAGE_FAIL] resolve backup dir | err=%v", err)
		return nil, err
	}

	runtimeSourceDir := resolveRuntimeRootDir()
	runtimeBackupDir := filepath.Join(backupDir, "runtime")
	localStorageBackupPath := filepath.Join(backupDir, "localstorage.json")
	metadataPath := filepath.Join(backupDir, "metadata.json")
	appendPortableDataLogf("[PACKAGE_START] backupDir=%s runtimeSourceDir=%s runtimeBackupDir=%s localStorageBackupPath=%s", backupDir, runtimeSourceDir, runtimeBackupDir, localStorageBackupPath)

	if err := os.RemoveAll(backupDir); err != nil {
		appendPortableDataLogf("[PACKAGE_FAIL] reset backup dir | backupDir=%s err=%v", backupDir, err)
		return nil, fmt.Errorf("failed to reset backup directory: %w", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		appendPortableDataLogf("[PACKAGE_FAIL] create backup dir | backupDir=%s err=%v", backupDir, err)
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	if isDirectory(runtimeSourceDir) {
		if err := copyPortableRuntimeDirectory(runtimeSourceDir, runtimeBackupDir); err != nil {
			appendPortableDataLogf("[PACKAGE_FAIL] copy runtime dir | src=%s dst=%s err=%v", runtimeSourceDir, runtimeBackupDir, err)
			return nil, fmt.Errorf("failed to package runtime data: %w", err)
		}
		appendPortableDataLogf("[PACKAGE_RUNTIME_OK] src=%s dst=%s", runtimeSourceDir, runtimeBackupDir)
	} else {
		appendPortableDataLogf("[PACKAGE_RUNTIME_SKIP] runtime source dir missing | path=%s", runtimeSourceDir)
	}

	trimmedLocalStorageJSON := strings.TrimSpace(localStorageJSON)
	if trimmedLocalStorageJSON == "" {
		trimmedLocalStorageJSON = "{}"
	}
	if err := os.WriteFile(localStorageBackupPath, []byte(trimmedLocalStorageJSON), 0o644); err != nil {
		appendPortableDataLogf("[PACKAGE_FAIL] write localstorage snapshot | path=%s err=%v", localStorageBackupPath, err)
		return nil, fmt.Errorf("failed to write localstorage snapshot: %w", err)
	}

	keyCount := countLocalStorageKeys(trimmedLocalStorageJSON)
	metadata := map[string]any{
		"timestamp":              time.Now().UnixMilli(),
		"runtimeSourceDir":       runtimeSourceDir,
		"runtimeBackupDir":       runtimeBackupDir,
		"localStorageBackupPath": localStorageBackupPath,
		"localStorageKeyCount":   keyCount,
	}
	if metadataRaw, err := json.MarshalIndent(metadata, "", "  "); err == nil {
		_ = os.WriteFile(metadataPath, metadataRaw, 0o644)
	}
	appendPortableDataLogf("[PACKAGE_OK] backupDir=%s localStorageKeys=%d", backupDir, keyCount)

	return &PortableDataPackageResult{
		BackupDir:              backupDir,
		RuntimeSourceDir:       runtimeSourceDir,
		RuntimeBackupDir:       runtimeBackupDir,
		LocalStorageBackupPath: localStorageBackupPath,
		LocalStorageKeyCount:   keyCount,
	}, nil
}

func (a *App) UnpackPortableData() (*PortableDataUnpackResult, error) {
	backupDir, err := resolvePortableBackupDir()
	if err != nil {
		appendPortableDataLogf("[UNPACK_FAIL] resolve backup dir | err=%v", err)
		return nil, err
	}

	runtimeBackupDir := filepath.Join(backupDir, "runtime")
	localStorageBackupPath := filepath.Join(backupDir, "localstorage.json")
	runtimeTargetDir := resolveRuntimeRootDir()
	appendPortableDataLogf("[UNPACK_START] backupDir=%s runtimeBackupDir=%s runtimeTargetDir=%s localStorageBackupPath=%s", backupDir, runtimeBackupDir, runtimeTargetDir, localStorageBackupPath)

	if !isDirectory(runtimeBackupDir) && !isRegularFile(localStorageBackupPath) {
		appendPortableDataLogf("[UNPACK_FAIL] backup empty | backupDir=%s", backupDir)
		return nil, fmt.Errorf("backup directory is empty or missing: %s", backupDir)
	}

	if isDirectory(runtimeBackupDir) {
		if err := clearPortableRuntimeTarget(runtimeTargetDir); err != nil {
			appendPortableDataLogf("[UNPACK_FAIL] clear runtime target | target=%s err=%v", runtimeTargetDir, err)
			return nil, fmt.Errorf("failed to clear current runtime data: %w", err)
		}
		if err := copyDirectory(runtimeBackupDir, runtimeTargetDir); err != nil {
			appendPortableDataLogf("[UNPACK_FAIL] restore runtime dir | src=%s dst=%s err=%v", runtimeBackupDir, runtimeTargetDir, err)
			return nil, fmt.Errorf("failed to restore runtime data: %w", err)
		}
		appendPortableDataLogf("[UNPACK_RUNTIME_OK] src=%s dst=%s", runtimeBackupDir, runtimeTargetDir)
	}

	localStorageJSON := "{}"
	if isRegularFile(localStorageBackupPath) {
		raw, readErr := os.ReadFile(localStorageBackupPath)
		if readErr != nil {
			appendPortableDataLogf("[UNPACK_FAIL] read localstorage backup | path=%s err=%v", localStorageBackupPath, readErr)
			return nil, fmt.Errorf("failed to read localstorage backup: %w", readErr)
		}
		localStorageJSON = strings.TrimSpace(string(raw))
		if localStorageJSON == "" {
			localStorageJSON = "{}"
		}
	}
	appendPortableDataLogf("[UNPACK_OK] backupDir=%s localStorageKeys=%d", backupDir, countLocalStorageKeys(localStorageJSON))

	return &PortableDataUnpackResult{
		BackupDir:              backupDir,
		RuntimeBackupDir:       runtimeBackupDir,
		LocalStorageBackupPath: localStorageBackupPath,
		LocalStorageJSON:       localStorageJSON,
		LocalStorageKeyCount:   countLocalStorageKeys(localStorageJSON),
	}, nil
}

func resolvePortableBackupDir() (string, error) {
	baseDir := ""
	if root, err := findProjectRoot(); err == nil && strings.TrimSpace(root) != "" {
		baseDir = root
	}
	if baseDir == "" {
		if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
			baseDir = filepath.Dir(exePath)
		}
	}
	if baseDir == "" {
		if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
			baseDir = wd
		}
	}
	if baseDir == "" {
		return "", fmt.Errorf("unable to resolve portable backup base directory")
	}
	return filepath.Join(baseDir, "backup"), nil
}

func countLocalStorageKeys(raw string) int {
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return 0
	}
	return len(decoded)
}

func copyPortableRuntimeDirectory(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if strings.EqualFold(filepath.Base(path), "LOCK") {
				return nil
			}
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if shouldSkipPortableRuntimeRelPath(relPath) {
			if info.IsDir() {
				appendPortableDataLogf("[PACKAGE_RUNTIME_SKIP] skip dir | path=%s", path)
				return filepath.SkipDir
			}
			appendPortableDataLogf("[PACKAGE_RUNTIME_SKIP] skip file | path=%s", path)
			return nil
		}

		targetPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if strings.EqualFold(info.Name(), "LOCK") {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			if strings.EqualFold(info.Name(), "LOCK") {
				return nil
			}
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func shouldSkipPortableRuntimeRelPath(relPath string) bool {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" || trimmed == "." {
		return false
	}

	parts := strings.Split(filepath.Clean(trimmed), string(filepath.Separator))
	return len(parts) > 0 && strings.EqualFold(parts[0], "webview2")
}

func clearPortableRuntimeTarget(runtimeTargetDir string) error {
	if err := os.MkdirAll(runtimeTargetDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(runtimeTargetDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		if strings.EqualFold(name, "webview2") {
			appendPortableDataLogf("[UNPACK_RUNTIME_SKIP] preserve dir | path=%s", filepath.Join(runtimeTargetDir, name))
			continue
		}

		targetPath := filepath.Join(runtimeTargetDir, name)
		if removeErr := os.RemoveAll(targetPath); removeErr != nil {
			return removeErr
		}
	}

	return nil
}

func isRegularFile(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Mode().IsRegular()
}
