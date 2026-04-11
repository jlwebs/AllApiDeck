package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	sidecarHost = "127.0.0.1"
	sidecarPort = 3000
)

type App struct {
	ctx        context.Context
	sidecarCmd *exec.Cmd
	sidecarLog *os.File
	sidecarMu  sync.Mutex
	mode       launchMode
	recordKey  string

	tray                 trayController
	windowMonitorStop    chan struct{}
	windowMonitorStopMux sync.Mutex

	panelCmd *exec.Cmd
	panelMu  sync.Mutex

	panelAutoStop    chan struct{}
	panelAutoStopMux sync.Mutex

	mainWindowGraceUntil atomic.Int64
	mainWindowSeenNormal atomic.Bool
}

func NewApp(mode launchMode, recordKey string) *App {
	return &App{mode: mode, recordKey: recordKey}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	debugLogf("startup begin")
	if a.isMainMode() {
		a.mainWindowSeenNormal.Store(false)
		a.mainWindowGraceUntil.Store(time.Now().Add(4 * time.Second).UnixMilli())
		a.terminateSiblingAppProcesses()
		a.stopPanelProcess()
		if err := a.initTray(); err != nil {
			debugLogf("init tray failed: %v", err)
		} else {
			debugLogf("tray initialised")
		}
		a.initWindowMonitor()
	}
	if a.isPanelMode() || !shouldStartDevSidecar() {
		debugLogf("startup skip dev sidecar: embedded bridge mode")
		return
	}
	if err := a.ensureSidecar(); err != nil {
		debugLogf("startup sidecar error: %v", err)
		fmt.Printf("failed to start local API sidecar: %v\n", err)
		return
	}
	debugLogf("startup sidecar ready")
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	debugLogf("shutdown begin")
	a.stopWindowMonitor()
	a.stopPanelAutoController()
	a.closeTray()
	a.stopPanelProcess()
	a.stopSidecar()
	debugLogf("shutdown complete")
}

func (a *App) domReady(ctx context.Context) {
	_ = ctx
	debugLogf("dom ready")
	if a.isPanelMode() {
		go a.initPanelWindow()
		return
	}
	if a.isMainMode() {
		go a.ensureMainWindowVisible()
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	_ = ctx
	debugLogf("before close")
	if a.isPanelMode() || a.isEditorMode() || a.isDesktopConfigMode() {
		return false
	}
	if a.isQuitRequested() {
		debugLogf("before close allowed: quit requested")
		return false
	}
	if err := a.HideToTray(); err != nil {
		debugLogf("before close hide to tray failed: %v", err)
		return false
	}
	debugLogf("before close intercepted: hidden to tray")
	return true
}

func (a *App) GetLaunchMode() string {
	return string(a.mode)
}

func (a *App) GetLaunchRecordKey() string {
	return a.recordKey
}

func (a *App) isPanelMode() bool {
	return a.mode == launchModePanel
}

func (a *App) isEditorMode() bool {
	return a.mode == launchModeEditor
}

func (a *App) isDesktopConfigMode() bool {
	return a.mode == launchModeDesktopConfig
}

func (a *App) isMainMode() bool {
	return a.mode == launchModeMain
}

func (a *App) terminateSiblingAppProcesses() {
	if runtime.GOOS != "windows" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}

	processName := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))
	if processName == "" {
		return
	}

	script := fmt.Sprintf(
		"Get-Process -Name '%s' -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne %d } | Stop-Process -Force",
		processName,
		os.Getpid(),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	configureBackgroundCmd(cmd)
	if err := cmd.Run(); err != nil {
		debugLogf("terminate sibling processes failed: %v", err)
		return
	}
	debugLogf("terminated sibling %s processes", processName)
}

func (a *App) ensureMainWindowVisible() {
	if a.ctx == nil || !a.isMainMode() {
		return
	}

	time.Sleep(220 * time.Millisecond)
	debugLogf("ensure main window visible")
	defaultWidth, defaultHeight, minWidth, minHeight := resolveMainWindowSize()
	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
	wruntime.WindowSetMinSize(a.ctx, minWidth, minHeight)
	wruntime.WindowSetSize(a.ctx, defaultWidth, defaultHeight)
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
	wruntime.Show(a.ctx)
	wruntime.WindowCenter(a.ctx)
	a.mainWindowSeenNormal.Store(true)
	a.mainWindowGraceUntil.Store(time.Now().Add(1800 * time.Millisecond).UnixMilli())
}

func (a *App) ensureSidecar() error {
	a.sidecarMu.Lock()
	defer a.sidecarMu.Unlock()

	debugLogf("ensureSidecar enter")

	if apiReady(sidecarHost, sidecarPort, 1200*time.Millisecond) {
		debugLogf("sidecar already ready on %s:%d", sidecarHost, sidecarPort)
		return nil
	}
	if tcpPortOpen(sidecarHost, sidecarPort, 600*time.Millisecond) {
		if err := waitForAPIReady(sidecarHost, sidecarPort, 8*time.Second); err == nil {
			debugLogf("sidecar became ready on occupied port %s:%d after waiting", sidecarHost, sidecarPort)
			return nil
		}
		debugLogf("port %d occupied by another process", sidecarPort)
		return fmt.Errorf("port %d is already occupied by another process", sidecarPort)
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		debugLogf("findProjectRoot failed: %v", err)
		return err
	}
	debugLogf("project root: %s", projectRoot)

	nodePath, err := exec.LookPath("node")
	if err != nil {
		debugLogf("node not found: %v", err)
		return fmt.Errorf("node executable not found in PATH: %w", err)
	}
	debugLogf("node path: %s", nodePath)

	viteBin := resolveViteExecutable(projectRoot)
	if viteBin == "" {
		debugLogf("vite executable missing: %s", viteBin)
		return fmt.Errorf("vite executable not found: %s", viteBin)
	}
	debugLogf("vite executable: %s", viteBin)

	logDir := filepath.Join(projectRoot, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		debugLogf("mkdir logs failed: %v", err)
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "EXE_BACKEND_DEBUG.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		debugLogf("open backend log failed: %v", err)
		return fmt.Errorf("failed to open backend log: %w", err)
	}

	cmd := exec.Command(nodePath, viteBin, "--host", sidecarHost, "--port", fmt.Sprintf("%d", sidecarPort), "--strictPort")
	configureBackgroundCmd(cmd)
	cmd.Dir = projectRoot
	cmd.Env = os.Environ()
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if _, err := io.WriteString(logFile, fmt.Sprintf("[%s] [EXE] starting vite sidecar in %s\n", time.Now().Format(time.RFC3339), projectRoot)); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("failed to write startup log: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		debugLogf("start vite sidecar failed: %v", err)
		return fmt.Errorf("failed to launch vite sidecar: %w", err)
	}
	debugLogf("vite sidecar process started pid=%d", cmd.Process.Pid)

	a.sidecarCmd = cmd
	a.sidecarLog = logFile

	go a.waitSidecar(cmd, logFile)

	if err := waitForAPIReady(sidecarHost, sidecarPort, 20*time.Second); err != nil {
		debugLogf("waitForAPIReady failed: %v", err)
		a.stopSidecarLocked()
		return fmt.Errorf("vite sidecar did not become ready: %w", err)
	}

	_, _ = io.WriteString(logFile, fmt.Sprintf("[%s] [EXE] vite sidecar ready at http://%s:%d\n", time.Now().Format(time.RFC3339), sidecarHost, sidecarPort))
	debugLogf("vite sidecar ready at http://%s:%d", sidecarHost, sidecarPort)
	return nil
}

func (a *App) waitSidecar(cmd *exec.Cmd, logFile *os.File) {
	err := cmd.Wait()

	a.sidecarMu.Lock()
	defer a.sidecarMu.Unlock()

	if a.sidecarCmd == cmd {
		a.sidecarCmd = nil
	}

	if logFile != nil {
		if err != nil {
			_, _ = io.WriteString(logFile, fmt.Sprintf("[%s] [EXE] vite sidecar exited with error: %v\n", time.Now().Format(time.RFC3339), err))
		} else {
			_, _ = io.WriteString(logFile, fmt.Sprintf("[%s] [EXE] vite sidecar exited cleanly\n", time.Now().Format(time.RFC3339)))
		}
	}

	if a.sidecarLog == logFile {
		_ = a.sidecarLog.Close()
		a.sidecarLog = nil
	}
}

func (a *App) stopSidecar() {
	a.sidecarMu.Lock()
	defer a.sidecarMu.Unlock()
	a.stopSidecarLocked()
}

func (a *App) stopSidecarLocked() {
	if a.sidecarCmd != nil && a.sidecarCmd.Process != nil {
		killProcessTree(a.sidecarCmd.Process.Pid)
		a.sidecarCmd = nil
	}
	if a.sidecarLog != nil {
		_, _ = io.WriteString(a.sidecarLog, fmt.Sprintf("[%s] [EXE] stopping vite sidecar\n", time.Now().Format(time.RFC3339)))
		_ = a.sidecarLog.Close()
		a.sidecarLog = nil
	}
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}

	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F").Run()
		return
	}

	_ = exec.Command("kill", "-TERM", fmt.Sprintf("%d", pid)).Run()
}

func findProjectRoot() (string, error) {
	exePath, _ := os.Executable()
	workingDir, _ := os.Getwd()

	candidates := []string{
		workingDir,
		filepath.Dir(exePath),
		filepath.Join(filepath.Dir(exePath), ".."),
		filepath.Join(filepath.Dir(exePath), "..", ".."),
	}

	for _, candidate := range candidates {
		if root := walkUpToProjectRoot(candidate); root != "" {
			return root, nil
		}
	}

	return "", fmt.Errorf("unable to locate project root from cwd=%s exe=%s", workingDir, exePath)
}

func walkUpToProjectRoot(start string) string {
	if start == "" {
		return ""
	}

	dir := filepath.Clean(start)
	for {
		if looksLikeProjectRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func looksLikeProjectRoot(dir string) bool {
	required := []string{
		filepath.Join(dir, "package.json"),
		filepath.Join(dir, "vite.config.js"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return resolveViteExecutable(dir) != ""
}

func shouldStartDevSidecar() bool {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return false
	}
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	return resolveViteExecutable(projectRoot) != ""
}

func resolveViteExecutable(projectRoot string) string {
	directPath := filepath.Join(projectRoot, "node_modules", "vite", "bin", "vite.js")
	if _, err := os.Stat(directPath); err == nil {
		return directPath
	}

	viteDir := filepath.Join(projectRoot, "node_modules", "vite")
	if realViteDir, err := filepath.EvalSymlinks(viteDir); err == nil {
		realBinPath := filepath.Join(realViteDir, "bin", "vite.js")
		if _, err := os.Stat(realBinPath); err == nil {
			return realBinPath
		}
	}

	pnpmRoot := filepath.Join(projectRoot, "node_modules", ".pnpm")
	entries, err := os.ReadDir(pnpmRoot)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() || len(entry.Name()) < 6 || entry.Name()[:5] != "vite@" {
			continue
		}
		pnpmBinPath := filepath.Join(pnpmRoot, entry.Name(), "node_modules", "vite", "bin", "vite.js")
		if _, err := os.Stat(pnpmBinPath); err == nil {
			return pnpmBinPath
		}
	}

	return ""
}

func waitForAPIReady(host string, port int, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		if apiReady(host, port, 1200*time.Millisecond) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

func apiReady(host string, port int, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/api/browser-session/browsers", host, port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 500
}

func tcpPortOpen(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func debugLogf(format string, args ...interface{}) {
	line := fmt.Sprintf("[%s] [EXE] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	for _, path := range candidateDebugLogPaths() {
		if path == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			continue
		}
		_, _ = io.WriteString(file, line)
		_ = file.Close()
		return
	}
}

func candidateDebugLogPaths() []string {
	var paths []string
	paths = append(paths, filepath.Join(resolveRuntimeLogDir(), "EXE_BACKEND_DEBUG.log"))
	if wd, err := os.Getwd(); err == nil && wd != "" {
		paths = append(paths, filepath.Join(wd, "logs", "EXE_BACKEND_DEBUG.log"))
	}
	if exePath, err := os.Executable(); err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		paths = append(paths, filepath.Join(exeDir, "logs", "EXE_BACKEND_DEBUG.log"))
		paths = append(paths, filepath.Join(exeDir, "..", "logs", "EXE_BACKEND_DEBUG.log"))
		paths = append(paths, filepath.Join(exeDir, "..", "..", "logs", "EXE_BACKEND_DEBUG.log"))
	}
	return paths
}
