package main

import (
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:dist
var assets embed.FS

type launchMode string
type panelStartMode string

const (
	launchModeMain          launchMode     = "main"
	launchModePanel         launchMode     = "panel"
	launchModeEditor        launchMode     = "editor"
	launchModeDesktopConfig launchMode     = "desktop-config"
	webviewGroupPIDEnvKey   string         = "BATCH_API_CHECK_WEBVIEW_GROUP_PID"
	panelStartAuto          panelStartMode = "auto"
	panelStartManual        panelStartMode = "manual"
)

func main() {
	enableProcessDPIAwareness()
	mode, recordKey, panelStart := resolveLaunchContext(os.Args[1:])
	applyMacActivationPolicy(mode)
	app := NewApp(mode, recordKey, panelStart)

	err := wails.Run(buildAppOptions(app, mode))

	if err != nil {
		println("Error:", err.Error())
	}
}

func resolveLaunchContext(args []string) (launchMode, string, panelStartMode) {
	mode := launchModeMain
	recordKey := ""
	panelStart := panelStartAuto
	for index := 0; index < len(args); index += 1 {
		arg := strings.TrimSpace(args[index])
		if strings.EqualFold(arg, "--panel") {
			mode = launchModePanel
			continue
		}
		if strings.EqualFold(arg, "--editor") {
			mode = launchModeEditor
			continue
		}
		if strings.EqualFold(arg, "--desktop-config") {
			mode = launchModeDesktopConfig
			continue
		}
		if strings.EqualFold(arg, "--panel-manual") {
			panelStart = panelStartManual
			continue
		}
		if strings.EqualFold(arg, "--row-key") && index+1 < len(args) {
			recordKey = args[index+1]
			index += 1
		}
	}
	return mode, recordKey, panelStart
}

func buildAppOptions(app *App, mode launchMode) *options.App {
	mainWidth, mainHeight, mainMinWidth, mainMinHeight := resolveMainWindowSize()
	appOptions := &options.App{
		Title:             "All API Deck",
		Width:             mainWidth,
		Height:            mainHeight,
		MinWidth:          mainMinWidth,
		MinHeight:         mainMinHeight,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows:       buildWindowsOptions(mode),
		Mac:           buildMacOptions(mode),
		Linux:         buildLinuxOptions(mode),
		OnStartup:     app.startup,
		OnDomReady:    app.domReady,
		OnBeforeClose: app.beforeClose,
		OnShutdown:    app.shutdown,
		Bind: []interface{}{
			app,
		},
	}

	if mode == launchModeMain && runtime.GOOS == "darwin" {
		appOptions.HideWindowOnClose = false
		appOptions.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "allapideck-main",
			OnSecondInstanceLaunch: func(_ options.SecondInstanceData) {
				go func() {
					_ = app.ShowMainWindow()
				}()
			},
		}
	}

	if mode == launchModePanel {
		appOptions.Title = "All API Deck Panel"
		if app.isManualPanelStart() {
			appOptions.Width = manualPanelWideWidth
			appOptions.Height = manualPanelHeight
			appOptions.MinWidth = manualPanelWindowWidth
			appOptions.MaxWidth = manualPanelMaxWidth
			appOptions.MinHeight = manualPanelMinHeight
		} else {
			appOptions.Width = panelWideWindowWidth
			appOptions.Height = panelWindowHeight
			appOptions.MinWidth = panelWindowWidth
			appOptions.MaxWidth = panelMaxWindowWidth
			appOptions.MinHeight = 560
		}
		appOptions.DisableResize = true
		appOptions.Frameless = true
		appOptions.AlwaysOnTop = true
		appOptions.HideWindowOnClose = false
		appOptions.StartHidden = true
		appOptions.BackgroundColour = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	if mode == launchModeEditor {
		appOptions.Title = "Key Editor"
		appOptions.Frameless = true
		appOptions.Width = 720
		appOptions.Height = 350
		appOptions.MinWidth = 680
		appOptions.MinHeight = 320
		appOptions.HideWindowOnClose = false
		appOptions.AlwaysOnTop = true
		appOptions.BackgroundColour = &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	if mode == launchModeDesktopConfig {
		appOptions.Title = "Desktop Config"
		appOptions.Width = 840
		appOptions.Height = 800
		appOptions.MinWidth = 760
		appOptions.MinHeight = 760
		appOptions.HideWindowOnClose = false
		appOptions.AlwaysOnTop = true
	}

	return appOptions
}

func resolveMainWindowSize() (width int, height int, minWidth int, minHeight int) {
	width = 800
	height = 460
	minWidth = 720
	minHeight = 460

	workArea, err := getDesktopWorkArea()
	if err != nil || workArea.Width() <= 0 || workArea.Height() <= 0 {
		return
	}

	width = clampWindowSize(int(float64(workArea.Width())*0.35), minWidth, 860)
	height = clampWindowSize(int(float64(workArea.Height())*0.6), minHeight, 600)
	return
}

func clampWindowSize(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func buildWindowsOptions(mode launchMode) *windows.Options {
	windowOptions := &windows.Options{
		WebviewUserDataPath: resolveWebviewUserDataPath(mode),
		WindowClassName:     "AllApiDeckWindow",
	}
	if mode == launchModePanel {
		windowOptions.WebviewIsTransparent = true
		windowOptions.WindowIsTranslucent = true
		windowOptions.DisableFramelessWindowDecorations = true
	}
	if mode == launchModeEditor {
		windowOptions.WebviewIsTransparent = true
		windowOptions.WindowIsTranslucent = true
		windowOptions.DisableFramelessWindowDecorations = true
	}
	return windowOptions
}

func buildMacOptions(mode launchMode) *mac.Options {
	if mode != launchModePanel && mode != launchModeEditor {
		return nil
	}
	return &mac.Options{
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
	}
}

func buildLinuxOptions(mode launchMode) *linux.Options {
	if mode != launchModePanel && mode != launchModeEditor {
		return &linux.Options{}
	}
	return &linux.Options{
		WindowIsTranslucent: true,
	}
}

func resolveWebviewUserDataPath(appMode launchMode) string {
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
		groupPID := resolveWebviewGroupPID(appMode)
		webviewRoot = filepath.Join(webviewRoot, strconv.Itoa(groupPID))
		if appMode == launchModeMain {
			_ = cleanupOldWebviewDevDirs(filepath.Dir(webviewRoot))
		}
	}

	if err := os.MkdirAll(webviewRoot, 0o755); err != nil {
		return ""
	}
	cleanupStaleWebviewLocks(webviewRoot)
	return webviewRoot
}

func resolveWebviewGroupPID(appMode launchMode) int {
	if raw := strings.TrimSpace(os.Getenv(webviewGroupPIDEnvKey)); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return value
		}
	}

	if appMode != launchModeMain {
		if parentPID := os.Getppid(); parentPID > 0 {
			return parentPID
		}
	}

	if pid := os.Getpid(); pid > 0 {
		return pid
	}
	return 1
}

func cleanupStaleWebviewLocks(webviewRoot string) {
	if runtime.GOOS == "windows" && hasAnotherBatchApiCheckProcess() {
		return
	}

	lockPaths := []string{
		filepath.Join(webviewRoot, "Default", "LOCK"),
		filepath.Join(webviewRoot, "LOCK"),
		filepath.Join(webviewRoot, "SingletonLock"),
		filepath.Join(webviewRoot, "SingletonCookie"),
	}

	for _, path := range lockPaths {
		if _, err := os.Stat(path); err == nil {
			_ = os.Remove(path)
		}
	}
}

func hasAnotherBatchApiCheckProcess() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	imageName := filepath.Base(exePath)
	if imageName == "" {
		return false
	}

	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+imageName, "/FO", "CSV", "/NH")
	configureBackgroundCmd(cmd)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToUpper(line), "INFO:") {
			continue
		}
		count += 1
	}
	return count > 1
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
