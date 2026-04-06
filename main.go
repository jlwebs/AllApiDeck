package main

import (
	"embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Batch API Check",
		Width:            1480,
		Height:           960,
		MinWidth:         1200,
		MinHeight:        760,
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows:       buildWindowsOptions(),
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func buildWindowsOptions() *windows.Options {
	return &windows.Options{
		WebviewUserDataPath: resolveWebviewUserDataPath(),
		WindowClassName:     "BatchApiCheckWindow",
	}
}

func resolveWebviewUserDataPath() string {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		return ""
	}

	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	exeName := strings.ToLower(filepath.Base(exePath))
	mode := "prod"
	if strings.Contains(exeName, "-dev") {
		mode = "dev"
	}

	webviewRoot := filepath.Join(root, "BatchApiCheck", "runtime", "webview2", mode)
	if mode == "dev" {
		webviewRoot = filepath.Join(webviewRoot, strconv.Itoa(os.Getpid()))
		_ = cleanupOldWebviewDevDirs(filepath.Dir(webviewRoot))
	}

	if err := os.MkdirAll(webviewRoot, 0o755); err != nil {
		return ""
	}
	return webviewRoot
}

func cleanupOldWebviewDevDirs(devRoot string) error {
	entries, err := os.ReadDir(devRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := strconv.Atoi(name); err != nil {
			continue
		}
		if name == strconv.Itoa(os.Getpid()) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(devRoot, name))
	}
	return nil
}
