//go:build windows

package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"sync"
)

//go:embed build/windows/icon.ico
var embeddedWindowsAppIcon []byte

var (
	runtimeWindowsAppIconPath string
	runtimeWindowsAppIconErr  error
	runtimeWindowsAppIconOnce sync.Once
)

func ensureRuntimeWindowsAppIconPath() (string, error) {
	runtimeWindowsAppIconOnce.Do(func() {
		root := os.Getenv("LOCALAPPDATA")
		if root == "" {
			root = os.TempDir()
		}

		targetDir := filepath.Join(root, "BatchApiCheck", "runtime", "assets")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			runtimeWindowsAppIconErr = err
			return
		}

		targetPath := filepath.Join(targetDir, "app-icon.ico")
		if info, err := os.Stat(targetPath); err == nil && info.Size() == int64(len(embeddedWindowsAppIcon)) {
			runtimeWindowsAppIconPath = targetPath
			return
		}

		if err := os.WriteFile(targetPath, embeddedWindowsAppIcon, 0o644); err != nil {
			runtimeWindowsAppIconErr = err
			return
		}
		runtimeWindowsAppIconPath = targetPath
	})

	return runtimeWindowsAppIconPath, runtimeWindowsAppIconErr
}
