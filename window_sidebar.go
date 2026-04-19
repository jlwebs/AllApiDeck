package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	mainWindowWidth        = 760
	mainWindowHeight       = 460
	mainWindowMinWidth     = 720
	mainWindowMinHeight    = 460
	panelWindowWidth       = 192
	panelWideWindowWidth   = 520
	panelMaxWindowWidth    = 680
	panelWindowHeight      = 936
	manualPanelWindowWidth = 136
	manualPanelWideWidth   = 300
	manualPanelMaxWidth    = 420
	manualPanelHeight      = 671
	manualPanelMinHeight   = 406
	panelTriggerWidth      = 22
	panelExpandedEdgeGap   = 0
	panelDockThreshold     = 28
	panelWindowMarginY     = 20
	panelEdgeActivateGap   = 2
	panelRightDockShiftPct = 0
	panelHideGrace         = 500 * time.Millisecond
	panelEdgeCooldown      = 1000 * time.Millisecond
	panelAutoTickInterval  = 60 * time.Millisecond
	windowMonitorInterval  = 450 * time.Millisecond
	panelRestoreSignal     = "panel-restore.signal"
	panelManualShowSignal  = "panel-manual-show.signal"
	panelManualReadySignal = "panel-manual-ready.signal"
)

type panelDockEdge string

const (
	panelDockLeft  panelDockEdge = "left"
	panelDockRight panelDockEdge = "right"
	panelDockFree  panelDockEdge = "free"
)

type sidebarWindowBounds struct {
	Width  int
	Height int
	X      int
	Y      int
}

type trayController interface {
	Close()
}

type noopTrayController struct{}

func (noopTrayController) Close() {}

type panelWindowState struct {
	screenWidth              int
	screenHeight             int
	collapsed                bool
	expandedWidth            int
	expandedHeight           int
	lastExpandedHeight       int
	interactionLocked        bool
	superMiniActive          bool
	superMiniTransitionUntil int64
	preferredX               int
	hasPreferredX            bool
	preferredY               int
	hasPreferredY            bool
	dockEdge                 panelDockEdge
}

type sidebarWindowState struct {
	mu               sync.Mutex
	quitRequested    atomic.Bool
	lastNormalBounds sidebarWindowBounds
	hasNormalBounds  bool
	panel            panelWindowState
}

var appWindowState sidebarWindowState

func (a *App) initWindowMonitor() {
	a.windowMonitorStopMux.Lock()
	defer a.windowMonitorStopMux.Unlock()
	if a.windowMonitorStop != nil || a.isPanelMode() {
		return
	}
	stopCh := make(chan struct{})
	a.windowMonitorStop = stopCh
	go a.runWindowMonitor(stopCh)
}

func (a *App) stopWindowMonitor() {
	a.windowMonitorStopMux.Lock()
	defer a.windowMonitorStopMux.Unlock()
	if a.windowMonitorStop == nil {
		return
	}
	close(a.windowMonitorStop)
	a.windowMonitorStop = nil
}

func (a *App) startPanelAutoController() {
	if !a.isPanelMode() || !nativePanelControllerSupported() {
		return
	}
	a.panelAutoStopMux.Lock()
	defer a.panelAutoStopMux.Unlock()
	if a.panelAutoStop != nil {
		return
	}
	stopCh := make(chan struct{})
	a.panelAutoStop = stopCh
	go a.runPanelAutoController(stopCh)
}

func (a *App) stopPanelAutoController() {
	a.panelAutoStopMux.Lock()
	defer a.panelAutoStopMux.Unlock()
	if a.panelAutoStop == nil {
		return
	}
	close(a.panelAutoStop)
	a.panelAutoStop = nil
}

func (a *App) startPanelSignalWatcher() {
	if !a.isPanelMode() {
		return
	}
	a.panelSignalStopMux.Lock()
	defer a.panelSignalStopMux.Unlock()
	if a.panelSignalStop != nil {
		return
	}
	stopCh := make(chan struct{})
	a.panelSignalStop = stopCh
	go a.runPanelSignalWatcher(stopCh)
}

func (a *App) stopPanelSignalWatcher() {
	a.panelSignalStopMux.Lock()
	defer a.panelSignalStopMux.Unlock()
	if a.panelSignalStop == nil {
		return
	}
	close(a.panelSignalStop)
	a.panelSignalStop = nil
}

func (a *App) runPanelSignalWatcher(stopCh <-chan struct{}) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if !a.isPanelMode() || a.ctx == nil || a.isQuitRequested() {
				continue
			}
			if !consumePanelManualShowSignal() {
				continue
			}
			debugLogf("panel manual show signal consumed")
			if err := a.applyManualPanelWindowState(0, 0); err != nil {
				debugLogf("panel manual show failed: %v", err)
				continue
			}
			if err := writePanelManualReadySignal(); err != nil {
				debugLogf("panel manual ready ack write failed: %v", err)
				continue
			}
			debugLogf("panel manual ready ack written")
		}
	}
}

func (a *App) runWindowMonitor(stopCh <-chan struct{}) {
	ticker := time.NewTicker(windowMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if a.ctx == nil || a.isQuitRequested() || a.isPanelMode() {
				continue
			}
			if consumePanelRestoreSignal() {
				_ = a.ShowMainWindow()
				continue
			}
			if wruntime.WindowIsNormal(a.ctx) {
				a.mainWindowSeenNormal.Store(true)
				a.captureNormalWindowBounds()
				continue
			}
			if wruntime.WindowIsMinimised(a.ctx) {
				if !a.mainWindowSeenNormal.Load() {
					debugLogf("window monitor skip minimise handling: main window not yet normal")
					continue
				}
				if until := a.mainWindowGraceUntil.Load(); until > 0 && time.Now().UnixMilli() < until {
					debugLogf("window monitor skip minimise handling during startup grace window")
					continue
				}
				debugLogf("window monitor detected minimised main window; hiding to tray panel")
				if err := a.HideToTrayPanel(); err != nil {
					debugLogf("window monitor hide to tray panel failed: %v", err)
				}
				continue
			}
		}
	}
}

func (a *App) runPanelAutoController(stopCh <-chan struct{}) {
	ticker := time.NewTicker(panelAutoTickInterval)
	defer ticker.Stop()

	var lastInsideAt time.Time
	lastInsideAt = time.Now()
	var lastEdgeRevealAt time.Time

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if a.ctx == nil || !a.isPanelMode() || a.isQuitRequested() {
				continue
			}

			hwnd, err := findPanelWindowHandle()
			if err != nil || hwnd == 0 {
				continue
			}

			_ = hidePanelWindowFromTaskbar()

			if !lastEdgeRevealAt.IsZero() && time.Since(lastEdgeRevealAt) < panelEdgeCooldown {
				continue
			}

			cursorX, cursorY, err := getCursorPosition()
			if err != nil {
				continue
			}

			workArea, err := getMonitorWorkAreaForPoint(cursorX, cursorY)
			if err != nil || workArea.Width() <= 0 || workArea.Height() <= 0 {
				workArea = resolvePanelWorkArea(0, 0)
			}
			visible := isNativePanelWindowVisible(hwnd)
			state := a.getPanelStateSnapshot()
			if visible && (state.superMiniActive || (state.superMiniTransitionUntil > 0 && time.Now().UnixMilli() < state.superMiniTransitionUntil)) {
				lastInsideAt = time.Now()
				continue
			}
			currentRect := desktopRect{}
			if visible {
				if rect, rectErr := getNativePanelWindowRect(hwnd); rectErr == nil && rect.Width() > 0 && rect.Height() > 0 {
					currentRect = rect
					state = a.captureNativePanelPlacement(workArea, rect)
				}
			}
			bounds := a.resolveNativePanelBounds(workArea, state)
			activeRect := boundsToDesktopRect(bounds)
			if visible && currentRect.Width() > 0 && currentRect.Height() > 0 {
				activeRect = currentRect
			}
			cursorInside := isPointInsideRect(cursorX, cursorY, activeRect)
			locked := a.isPanelInteractionLocked()
			foreground := isNativePanelWindowForeground(hwnd)

			if visible && state.dockEdge == panelDockFree {
				lastInsideAt = time.Now()
				a.updatePanelRuntimeState(false, workArea, bounds, panelDockFree)
				continue
			}

			if visible && (cursorInside || locked || foreground) {
				lastInsideAt = time.Now()
				if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
					debugLogf("panel show failed: %v", err)
				}
				a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
				continue
			}

			withinMonitorY := cursorY >= int(workArea.Top) && cursorY < int(workArea.Bottom)
			switch state.dockEdge {
			case panelDockLeft:
				nearLeftEdge := cursorX >= int(workArea.Left) && cursorX <= int(workArea.Left)+panelEdgeActivateGap
				if nearLeftEdge && withinMonitorY {
					lastInsideAt = time.Now()
					if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
						debugLogf("panel edge-show failed: %v", err)
					}
					lastEdgeRevealAt = time.Now()
					a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
					continue
				}
			case panelDockRight:
				nearRightEdge := cursorX >= int(workArea.Right)-panelEdgeActivateGap && cursorX <= int(workArea.Right)
				if nearRightEdge && withinMonitorY {
					lastInsideAt = time.Now()
					if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
						debugLogf("panel edge-show failed: %v", err)
					}
					lastEdgeRevealAt = time.Now()
					a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
					continue
				}
			case panelDockFree:
				lastInsideAt = time.Now()
				a.updatePanelRuntimeState(false, workArea, bounds, panelDockFree)
				continue
			}

			if !visible {
				a.updatePanelRuntimeState(true, workArea, bounds, state.dockEdge)
				continue
			}

			if time.Since(lastInsideAt) < panelHideGrace {
				continue
			}
			if err := a.ensureNativePanelHidden(hwnd); err != nil {
				debugLogf("panel hide failed: %v", err)
			}
			a.updatePanelRuntimeState(true, workArea, bounds, state.dockEdge)
		}
	}
}

func (a *App) captureNormalWindowBounds() {
	isNormal := a.ctx != nil && wruntime.WindowIsNormal(a.ctx)
	if a.ctx == nil || a.isPanelMode() || !isNormal {
		debugLogf("capture normal bounds skipped: ctx=%t panel=%t normal=%t", a.ctx != nil, a.isPanelMode(), isNormal)
		return
	}

	width, height := wruntime.WindowGetSize(a.ctx)
	x, y := wruntime.WindowGetPosition(a.ctx)
	if width <= 0 || height <= 0 {
		debugLogf("capture normal bounds invalid size: size=%dx%d pos=(%d,%d)", width, height, x, y)
		return
	}

	appWindowState.mu.Lock()
	appWindowState.lastNormalBounds = sidebarWindowBounds{
		Width:  width,
		Height: height,
		X:      x,
		Y:      y,
	}
	appWindowState.hasNormalBounds = true
	appWindowState.mu.Unlock()
	debugLogf("capture normal bounds stored: size=%dx%d pos=(%d,%d)", width, height, x, y)
}

func (a *App) HideToTrayPanel() error {
	if a.ctx == nil || a.isPanelMode() {
		return nil
	}

	debugLogf("hide to tray panel requested")
	a.captureNormalWindowBounds()
	if err := a.startPanelProcessWithMode(panelStartAuto); err != nil {
		debugLogf("hide to tray panel start process failed: %v", err)
		return err
	}

	wruntime.WindowHide(a.ctx)
	wruntime.Hide(a.ctx)
	debugLogf("hide to tray panel completed")
	return nil
}

func (a *App) HideToTray() error {
	if a.ctx == nil || a.isPanelMode() {
		return nil
	}

	a.captureNormalWindowBounds()
	wruntime.WindowHide(a.ctx)
	wruntime.Hide(a.ctx)
	return nil
}

func (a *App) ShowMainWindow() error {
	if a.ctx == nil || a.isPanelMode() {
		return nil
	}

	debugLogf("show main window requested")
	a.stopPanelProcess()
	defaultWidth, defaultHeight, minWidth, minHeight := resolveMainWindowSize()

	appWindowState.mu.Lock()
	bounds := appWindowState.lastNormalBounds
	hasBounds := appWindowState.hasNormalBounds
	appWindowState.mu.Unlock()

	wruntime.WindowSetAlwaysOnTop(a.ctx, false)
	wruntime.WindowUnminimise(a.ctx)
	wruntime.WindowShow(a.ctx)
	wruntime.Show(a.ctx)
	wruntime.WindowSetMinSize(a.ctx, minWidth, minHeight)

	if hasBounds && bounds.Width > 0 && bounds.Height > 0 {
		width := clampWindowSize(bounds.Width, minWidth, defaultWidth)
		height := clampWindowSize(bounds.Height, minHeight, defaultHeight)
		debugLogf("show main window restoring bounds: size=%dx%d pos=(%d,%d)", width, height, bounds.X, bounds.Y)
		wruntime.WindowSetSize(a.ctx, width, height)
		wruntime.WindowSetPosition(a.ctx, bounds.X, bounds.Y)
		return nil
	}

	debugLogf("show main window falling back to center: default=%dx%d", defaultWidth, defaultHeight)
	wruntime.WindowSetSize(a.ctx, defaultWidth, defaultHeight)
	wruntime.WindowCenter(a.ctx)
	return nil
}

func (a *App) RequestMainWindowRestore() error {
	if !a.isPanelMode() {
		return a.ShowMainWindow()
	}
	if err := writePanelRestoreSignal(); err != nil {
		return err
	}
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
	return nil
}

func (a *App) EnterSidebarMode() error {
	return a.HideToTrayPanel()
}

func (a *App) OpenManualSidebarPanel() error {
	if a.ctx == nil || a.isPanelMode() {
		return nil
	}

	debugLogf("open manual sidebar panel requested")
	a.captureNormalWindowBounds()
	debugLogf("manual panel open requested")
	clearPanelManualReadySignal()

	if a.panelProcessRunning() {
		debugLogf("manual panel open: existing panel process detected, requesting visible state")
		if err := writePanelManualShowSignal(); err != nil {
			debugLogf("manual panel open: write show signal failed: %v", err)
		} else if waitForPanelManualReadySignal(2200 * time.Millisecond) {
			debugLogf("manual panel open: existing panel acknowledged visible state")
			wruntime.WindowHide(a.ctx)
			wruntime.Hide(a.ctx)
			return nil
		} else {
			debugLogf("manual panel open: existing panel did not acknowledge visible state in time")
		}

		debugLogf("manual panel open: restarting panel process after missed ack")
		a.stopPanelProcess()
		clearPanelManualReadySignal()
	}

	if err := a.startPanelProcessWithMode(panelStartManual); err != nil {
		debugLogf("manual panel open: start manual panel failed: %v", err)
		return err
	}
	if !waitForPanelManualReadySignal(3500 * time.Millisecond) {
		debugLogf("manual panel open: new manual panel did not become ready before timeout")
		return fmt.Errorf("manual panel ready timeout")
	}
	debugLogf("manual panel open: new manual panel acknowledged ready; hiding main window to tray")
	wruntime.WindowHide(a.ctx)
	wruntime.Hide(a.ctx)
	return nil
}

func (a *App) ExitSidebarMode() error {
	if a.isPanelMode() {
		return a.RequestMainWindowRestore()
	}
	return a.ShowMainWindow()
}

func (a *App) ToggleSidebarMode() error {
	if a.isPanelMode() {
		if appWindowState.panel.collapsed {
			return a.ExpandPanel()
		}
		return a.CollapsePanel()
	}
	return a.HideToTrayPanel()
}

func (a *App) GetSidebarMode() bool {
	return a.panelProcessRunning()
}

func (a *App) RequestQuit() {
	appWindowState.quitRequested.Store(true)
	a.stopPanelProcess()
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

func (a *App) isQuitRequested() bool {
	return appWindowState.quitRequested.Load()
}

func (a *App) isPanelInteractionLocked() bool {
	appWindowState.mu.Lock()
	defer appWindowState.mu.Unlock()
	return appWindowState.panel.interactionLocked
}

func (a *App) SetPanelInteractionLocked(locked bool) error {
	appWindowState.mu.Lock()
	appWindowState.panel.interactionLocked = locked
	appWindowState.mu.Unlock()
	return nil
}

func (a *App) SetPanelSuperMiniActive(active bool) error {
	appWindowState.mu.Lock()
	appWindowState.panel.superMiniActive = active
	appWindowState.panel.superMiniTransitionUntil = time.Now().Add(450 * time.Millisecond).UnixMilli()
	appWindowState.mu.Unlock()
	debugLogf("set panel super mini active: active=%t state=%+v", active, a.getPanelStateSnapshot())
	return nil
}

func (a *App) getPanelStateSnapshot() panelWindowState {
	appWindowState.mu.Lock()
	defer appWindowState.mu.Unlock()
	return appWindowState.panel
}

func (a *App) captureNativePanelPlacement(workArea desktopRect, rect desktopRect) panelWindowState {
	appWindowState.mu.Lock()
	defer appWindowState.mu.Unlock()

	debugLogf(
		"capture native panel placement entry: rect=(%d,%d)-(%d,%d) workArea=(%d,%d)-(%d,%d) state=%+v",
		rect.Left,
		rect.Top,
		rect.Right,
		rect.Bottom,
		workArea.Left,
		workArea.Top,
		workArea.Right,
		workArea.Bottom,
		appWindowState.panel,
	)
	dockEdge := resolveNativePanelDockEdge(int(rect.Left), rect.Width(), workArea)
	if rect.Width() > 0 {
		appWindowState.panel.expandedWidth = rect.Width()
	}
	if rect.Height() > 0 {
		appWindowState.panel.expandedHeight = rect.Height()
		if !appWindowState.panel.superMiniActive {
			appWindowState.panel.lastExpandedHeight = rect.Height()
		}
	}
	appWindowState.panel.screenWidth = workArea.Width()
	appWindowState.panel.screenHeight = workArea.Height()
	appWindowState.panel.collapsed = false
	appWindowState.panel.preferredY = clampPanelY(int(rect.Top), rect.Height(), workArea)
	appWindowState.panel.hasPreferredY = true
	appWindowState.panel.dockEdge = dockEdge

	if dockEdge == panelDockFree {
		appWindowState.panel.preferredX = clampPanelX(int(rect.Left), rect.Width(), workArea)
		appWindowState.panel.hasPreferredX = true
	} else {
		appWindowState.panel.preferredX = int(rect.Left)
		appWindowState.panel.hasPreferredX = false
	}

	debugLogf(
		"capture native panel placement stored: dock=%s expanded=%d preferred=(%d,%d) hasPreferred=(%t,%t)",
		appWindowState.panel.dockEdge,
		appWindowState.panel.expandedWidth,
		appWindowState.panel.preferredX,
		appWindowState.panel.preferredY,
		appWindowState.panel.hasPreferredX,
		appWindowState.panel.hasPreferredY,
	)

	return appWindowState.panel
}

func (a *App) resolveNativePanelBounds(workArea desktopRect, state panelWindowState) sidebarWindowBounds {
	width := panelWideWindowWidth
	if state.expandedWidth > panelWindowWidth {
		width = state.expandedWidth
	}

	if width < panelWindowWidth {
		width = panelWindowWidth
	}
	if width > panelMaxWindowWidth {
		width = panelMaxWindowWidth
	}

	height := panelWindowHeight
	if state.lastExpandedHeight > 0 {
		height = state.lastExpandedHeight
	} else if state.expandedHeight > 0 {
		height = state.expandedHeight
	}
	if maxHeight := workArea.Height() - panelWindowMarginY*2; maxHeight > 0 && height > maxHeight {
		height = maxInt(420, maxHeight)
	}
	if height <= 0 {
		height = panelWindowHeight
	}

	y := 0
	if state.hasPreferredY {
		y = clampPanelY(state.preferredY, height, workArea)
	}

	x := int(workArea.Right) - width - panelExpandedEdgeGap
	switch state.dockEdge {
	case panelDockLeft:
		x = int(workArea.Left)
	case panelDockFree:
		if state.hasPreferredX {
			x = clampPanelX(state.preferredX, width, workArea)
		}
	default:
		x = int(workArea.Right) - width - panelExpandedEdgeGap - ((width * panelRightDockShiftPct) / 100)
	}

	bounds := sidebarWindowBounds{
		Width:  width,
		Height: height,
		X:      x,
		Y:      y,
	}
	debugLogf(
		"resolve native panel bounds: dock=%s expanded=%d collapsed=%t workArea=(%d,%d)-(%d,%d) bounds=%dx%d pos=(%d,%d) state=%+v",
		state.dockEdge,
		state.expandedWidth,
		state.collapsed,
		workArea.Left,
		workArea.Top,
		workArea.Right,
		workArea.Bottom,
		bounds.Width,
		bounds.Height,
		bounds.X,
		bounds.Y,
		state,
	)
	return bounds
}

func (a *App) ensureNativePanelVisible(hwnd uintptr, bounds sidebarWindowBounds) error {
	rect, err := getNativePanelWindowRect(hwnd)
	if err == nil && isNativePanelWindowVisible(hwnd) && boundsMatchRect(bounds, rect) {
		return nil
	}
	return showNativePanelWindow(hwnd, bounds)
}

func (a *App) ensureNativePanelHidden(hwnd uintptr) error {
	if !isNativePanelWindowVisible(hwnd) {
		return nil
	}
	return hideNativePanelWindow(hwnd)
}

func (a *App) updatePanelRuntimeState(collapsed bool, workArea desktopRect, bounds sidebarWindowBounds, dockEdge panelDockEdge) {
	appWindowState.mu.Lock()
	appWindowState.panel.screenWidth = workArea.Width()
	appWindowState.panel.screenHeight = workArea.Height()
	appWindowState.panel.collapsed = collapsed
	appWindowState.panel.expandedHeight = bounds.Height
	if bounds.Height > 0 && !appWindowState.panel.superMiniActive {
		appWindowState.panel.lastExpandedHeight = bounds.Height
	}
	appWindowState.panel.preferredX = bounds.X
	appWindowState.panel.hasPreferredX = dockEdge == panelDockFree
	appWindowState.panel.preferredY = bounds.Y
	appWindowState.panel.hasPreferredY = true
	appWindowState.panel.dockEdge = dockEdge
	appWindowState.mu.Unlock()
	debugLogf(
		"update panel runtime state: collapsed=%t dock=%s workArea=(%d,%d)-(%d,%d) bounds=%dx%d pos=(%d,%d)",
		collapsed,
		dockEdge,
		workArea.Left,
		workArea.Top,
		workArea.Right,
		workArea.Bottom,
		bounds.Width,
		bounds.Height,
		bounds.X,
		bounds.Y,
	)
}

func isPointInsideBounds(x int, y int, bounds sidebarWindowBounds) bool {
	return x >= bounds.X && x < bounds.X+bounds.Width && y >= bounds.Y && y < bounds.Y+bounds.Height
}

func isPointInsideRect(x int, y int, rect desktopRect) bool {
	return x >= int(rect.Left) && x < int(rect.Right) && y >= int(rect.Top) && y < int(rect.Bottom)
}

func boundsToDesktopRect(bounds sidebarWindowBounds) desktopRect {
	return desktopRect{
		Left:   int32(bounds.X),
		Top:    int32(bounds.Y),
		Right:  int32(bounds.X + bounds.Width),
		Bottom: int32(bounds.Y + bounds.Height),
	}
}

func clampPanelX(x int, width int, workArea desktopRect) int {
	minX := int(workArea.Left)
	maxX := int(workArea.Right) - width
	if maxX < minX {
		maxX = minX
	}
	if x < minX {
		return minX
	}
	if x > maxX {
		return maxX
	}
	return x
}

func clampPanelY(y int, height int, workArea desktopRect) int {
	minY := int(workArea.Top)
	maxY := int(workArea.Bottom) - height
	if maxY < minY {
		maxY = minY
	}
	if y < minY {
		return minY
	}
	if y > maxY {
		return maxY
	}
	return y
}

func boundsMatchRect(bounds sidebarWindowBounds, rect desktopRect) bool {
	return bounds.X == int(rect.Left) &&
		bounds.Y == int(rect.Top) &&
		bounds.Width == rect.Width() &&
		bounds.Height == rect.Height()
}

func (a *App) InitPanelWindow(screenWidth int, screenHeight int) error {
	if !a.isPanelMode() || a.ctx == nil {
		return nil
	}
	debugLogf("panel init entry: manual=%t screen=%dx%d state=%+v", a.isManualPanelStart(), screenWidth, screenHeight, a.getPanelStateSnapshot())
	if a.isManualPanelStart() {
		debugLogf("panel init: manual panel start detected")
		if err := a.applyManualPanelWindowState(screenWidth, screenHeight); err != nil {
			debugLogf("panel init: manual panel apply state failed: %v", err)
			return err
		}
		if err := writePanelManualReadySignal(); err != nil {
			debugLogf("panel init: manual panel ready ack write failed: %v", err)
		} else {
			debugLogf("panel init: manual panel ready ack written")
		}
		return nil
	}
	debugLogf("panel init: auto panel start detected screen=%dx%d", screenWidth, screenHeight)
	workArea := resolvePanelWorkArea(screenWidth, screenHeight)
	appWindowState.mu.Lock()
	appWindowState.panel.screenWidth = maxInt(workArea.Width(), screenWidth)
	appWindowState.panel.screenHeight = maxInt(workArea.Height(), screenHeight)
	if appWindowState.panel.expandedWidth <= panelWindowWidth {
		appWindowState.panel.expandedWidth = panelWideWindowWidth
	}
	appWindowState.panel.collapsed = true
	appWindowState.panel.preferredX = 0
	appWindowState.panel.hasPreferredX = false
	appWindowState.panel.preferredY = 0
	appWindowState.panel.hasPreferredY = false
	appWindowState.panel.dockEdge = panelDockRight
	appWindowState.mu.Unlock()
	debugLogf("panel init: auto state reset complete workArea=(%d,%d)-(%d,%d)", workArea.Left, workArea.Top, workArea.Right, workArea.Bottom)
	if nativePanelControllerSupported() {
		a.startPanelAutoController()
		return nil
	}
	debugLogf("panel init: applying non-native panel state")
	return a.applyPanelWindowState(screenWidth, screenHeight, true)
}

func (a *App) CollapsePanel() error {
	if !a.isPanelMode() {
		return nil
	}
	debugLogf("collapse panel requested")
	if nativePanelControllerSupported() {
		appWindowState.mu.Lock()
		appWindowState.panel.collapsed = true
		appWindowState.mu.Unlock()
		hwnd, err := findPanelWindowHandle()
		if err == nil && hwnd != 0 {
			return a.ensureNativePanelHidden(hwnd)
		}
		return nil
	}
	appWindowState.mu.Lock()
	if appWindowState.panel.collapsed {
		appWindowState.mu.Unlock()
		return nil
	}
	screenWidth := appWindowState.panel.screenWidth
	screenHeight := appWindowState.panel.screenHeight
	appWindowState.mu.Unlock()
	a.capturePanelPlacement()
	return a.applyPanelWindowState(screenWidth, screenHeight, true)
}

func (a *App) ExpandPanel() error {
	if !a.isPanelMode() {
		return nil
	}
	debugLogf("expand panel requested")
	if nativePanelControllerSupported() {
		hwnd, err := findPanelWindowHandle()
		if err != nil || hwnd == 0 {
			debugLogf("expand panel skipped: hwnd unavailable err=%v", err)
			return nil
		}
		cursorX, cursorY, cursorErr := getCursorPosition()
		workArea := resolvePanelWorkArea(0, 0)
		if cursorErr == nil {
			if nextWorkArea, workErr := getMonitorWorkAreaForPoint(cursorX, cursorY); workErr == nil && nextWorkArea.Width() > 0 && nextWorkArea.Height() > 0 {
				workArea = nextWorkArea
			}
		}
		state := a.getPanelStateSnapshot()
		bounds := a.resolveNativePanelBounds(workArea, state)
		debugLogf("expand panel resolved bounds: workArea=(%d,%d)-(%d,%d) bounds=%dx%d pos=(%d,%d)", workArea.Left, workArea.Top, workArea.Right, workArea.Bottom, bounds.Width, bounds.Height, bounds.X, bounds.Y)
		if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
			debugLogf("expand panel ensure visible failed: %v", err)
			return err
		}
		a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
		debugLogf("expand panel applied state: dock=%s", state.dockEdge)
		return nil
	}
	appWindowState.mu.Lock()
	if !appWindowState.panel.collapsed {
		appWindowState.mu.Unlock()
		return nil
	}
	screenWidth := appWindowState.panel.screenWidth
	screenHeight := appWindowState.panel.screenHeight
	appWindowState.mu.Unlock()
	return a.applyPanelWindowState(screenWidth, screenHeight, false)
}

func (a *App) SetPanelCollapsed(screenWidth int, screenHeight int, collapsed bool) error {
	if !a.isPanelMode() {
		return nil
	}
	if nativePanelControllerSupported() {
		if collapsed {
			return a.CollapsePanel()
		}
		return a.ExpandPanel()
	}
	return a.applyPanelWindowState(screenWidth, screenHeight, collapsed)
}

func (a *App) GetPanelDockState() string {
	if !a.isPanelMode() {
		return string(panelDockRight)
	}
	appWindowState.mu.Lock()
	dockEdge := appWindowState.panel.dockEdge
	appWindowState.mu.Unlock()
	if dockEdge == "" {
		dockEdge = panelDockRight
	}
	debugLogf("get panel dock state: dock=%s state=%+v", dockEdge, a.getPanelStateSnapshot())
	return string(dockEdge)
}

func (a *App) GetPanelWindowBounds() (sidebarWindowBounds, error) {
	if !a.isPanelMode() {
		return sidebarWindowBounds{}, nil
	}

	hwnd, err := findPanelWindowHandle()
	state := a.getPanelStateSnapshot()
	workArea := resolvePanelWorkArea(state.screenWidth, state.screenHeight)
	debugLogf("get panel window bounds entry: hwnd=%d state=%+v workArea=(%d,%d)-(%d,%d)", hwnd, state, workArea.Left, workArea.Top, workArea.Right, workArea.Bottom)
	if err == nil && hwnd != 0 {
		if rect, rectErr := getNativePanelWindowRect(hwnd); rectErr == nil && rect.Width() > 0 && rect.Height() > 0 {
			if state.dockEdge == panelDockRight && int(rect.Left) <= int(workArea.Left)+panelEdgeActivateGap {
				debugLogf(
					"panel bounds fallback: native rect at left edge rect=(%d,%d)-(%d,%d) state=%+v",
					rect.Left,
					rect.Top,
					rect.Right,
					rect.Bottom,
					state,
				)
				bounds := a.resolveNativePanelBounds(workArea, state)
				debugLogf("get panel window bounds fallback resolved: bounds=%+v state=%+v", bounds, state)
				return bounds, nil
			}
			debugLogf("get panel window bounds native: rect=(%d,%d)-(%d,%d) state=%+v", rect.Left, rect.Top, rect.Right, rect.Bottom, state)
			return sidebarWindowBounds{
				Width:  rect.Width(),
				Height: rect.Height(),
				X:      int(rect.Left),
				Y:      int(rect.Top),
			}, nil
		}
	}

	bounds := a.resolveNativePanelBounds(workArea, state)
	debugLogf("get panel window bounds resolved: bounds=%+v state=%+v", bounds, state)
	return bounds, nil
}

func (a *App) applyPanelWindowState(screenWidth int, screenHeight int, collapsed bool) error {
	if a.ctx == nil {
		return nil
	}

	width := panelWindowWidth
	appWindowState.mu.Lock()
	storedScreenWidth := appWindowState.panel.screenWidth
	storedScreenHeight := appWindowState.panel.screenHeight
	expandedWidth := appWindowState.panel.expandedWidth
	currentState := appWindowState.panel
	appWindowState.mu.Unlock()
	debugLogf(
		"panel apply state entry: collapsed=%t screen=%dx%d storedScreen=%dx%d expanded=%d state=%+v",
		collapsed,
		screenWidth,
		screenHeight,
		storedScreenWidth,
		storedScreenHeight,
		expandedWidth,
		currentState,
	)
	if screenWidth <= 0 {
		screenWidth = storedScreenWidth
	}
	if screenHeight <= 0 {
		screenHeight = storedScreenHeight
	}
	if expandedWidth <= panelWindowWidth {
		expandedWidth = panelWideWindowWidth
	}
	dockEdge := panelDockRight
	workArea := resolvePanelWorkArea(screenWidth, screenHeight)
	if collapsed {
		width = panelTriggerWidth
	} else if expandedWidth > panelWindowWidth {
		width = expandedWidth
	}
	if width > panelMaxWindowWidth {
		width = panelMaxWindowWidth
	}
	height := panelWindowHeight
	if height > workArea.Height()-panelWindowMarginY*2 {
		height = maxInt(420, workArea.Height()-panelWindowMarginY*2)
	}
	if height <= 0 {
		height = panelWindowHeight
	}
	x := int(workArea.Right) - width - panelExpandedEdgeGap
	y := int(workArea.Top)

	appWindowState.mu.Lock()
	appWindowState.panel = panelWindowState{
		screenWidth:        screenWidth,
		screenHeight:       screenHeight,
		collapsed:          collapsed,
		expandedWidth:      expandedWidth,
		expandedHeight:     height,
		lastExpandedHeight: height,
		preferredX:         x,
		hasPreferredX:      false,
		preferredY:         0,
		hasPreferredY:      false,
		dockEdge:           panelDockRight,
	}
	appWindowState.mu.Unlock()
	debugLogf(
		"panel apply state: collapsed=%t dock=%s workArea=(%d,%d)-(%d,%d) size=%dx%d pos=(%d,%d) preferred=(%t,%t)",
		collapsed,
		dockEdge,
		workArea.Left,
		workArea.Top,
		workArea.Right,
		workArea.Bottom,
		width,
		height,
		x,
		y,
		false,
		false,
	)

	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetMinSize(a.ctx, panelTriggerWidth, 420)
	wruntime.WindowSetMaxSize(a.ctx, panelMaxWindowWidth, maxInt(panelWindowHeight, height))
	wruntime.WindowSetSize(a.ctx, width, height)
	wruntime.WindowSetPosition(a.ctx, x, y)
	wruntime.WindowShow(a.ctx)
	wruntime.Show(a.ctx)
	debugLogf("panel apply state committed: collapsed=%t size=%dx%d pos=(%d,%d)", collapsed, width, height, x, y)
	return nil
}

func (a *App) applyManualPanelWindowState(screenWidth int, screenHeight int) error {
	if a.ctx == nil {
		return nil
	}

	workArea := resolvePanelWorkArea(screenWidth, screenHeight)
	debugLogf("manual panel apply entry: screen=%dx%d workArea=(%d,%d)-(%d,%d) state=%+v", screenWidth, screenHeight, workArea.Left, workArea.Top, workArea.Right, workArea.Bottom, a.getPanelStateSnapshot())
	width := manualPanelWideWidth
	if width > manualPanelMaxWidth {
		width = manualPanelMaxWidth
	}

	height := manualPanelHeight
	if height > workArea.Height()-panelWindowMarginY*2 {
		height = maxInt(manualPanelMinHeight, workArea.Height()-panelWindowMarginY*2)
	}
	if height <= 0 {
		height = manualPanelHeight
	}

	x := int(workArea.Left) + maxInt((workArea.Width()-width)/2, 0)
	y := int(workArea.Top)
	dockEdge := panelDockFree
	hasPreferredX := true

	appWindowState.mu.Lock()
	appWindowState.panel = panelWindowState{
		screenWidth:        maxInt(workArea.Width(), screenWidth),
		screenHeight:       maxInt(workArea.Height(), screenHeight),
		collapsed:          false,
		expandedWidth:      width,
		expandedHeight:     height,
		lastExpandedHeight: height,
		preferredX:         x,
		hasPreferredX:      hasPreferredX,
		preferredY:         y,
		hasPreferredY:      true,
		dockEdge:           dockEdge,
	}
	appWindowState.mu.Unlock()

	debugLogf(
		"manual panel apply state: workArea=(%d,%d)-(%d,%d) size=%dx%d pos=(%d,%d)",
		workArea.Left,
		workArea.Top,
		workArea.Right,
		workArea.Bottom,
		width,
		height,
		x,
		y,
	)

	wruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wruntime.WindowSetMinSize(a.ctx, manualPanelWindowWidth, manualPanelMinHeight)
	wruntime.WindowSetMaxSize(a.ctx, manualPanelMaxWidth, maxInt(manualPanelHeight, height))
	wruntime.WindowSetSize(a.ctx, width, height)
	wruntime.WindowSetPosition(a.ctx, x, y)
	wruntime.WindowShow(a.ctx)
	wruntime.Show(a.ctx)
	if !a.isManualPanelStart() {
		_ = hidePanelWindowFromTaskbar()
	}
	return nil
}

func (a *App) capturePanelPlacement() {
	if !a.isPanelMode() || a.ctx == nil {
		return
	}
	appWindowState.mu.Lock()
	appWindowState.panel.preferredX = 0
	appWindowState.panel.hasPreferredX = false
	appWindowState.panel.preferredY = 0
	appWindowState.panel.hasPreferredY = false
	appWindowState.panel.dockEdge = panelDockRight
	appWindowState.mu.Unlock()
}

func resolvePanelDockEdge(x int, width int, workArea desktopRect) panelDockEdge {
	leftDistance := x - int(workArea.Left)
	rightDistance := int(workArea.Right) - (x + width)
	isNearLeft := leftDistance <= panelDockThreshold
	isNearRight := rightDistance <= panelDockThreshold

	switch {
	case isNearLeft && isNearRight:
		if leftDistance <= rightDistance {
			return panelDockLeft
		}
		return panelDockRight
	case isNearLeft:
		return panelDockLeft
	case isNearRight:
		return panelDockRight
	default:
		return panelDockFree
	}
}

func resolveNativePanelDockEdge(x int, width int, workArea desktopRect) panelDockEdge {
	leftDistance := x - int(workArea.Left)
	rightDistance := int(workArea.Right) - (x + width)
	rightDockX := int(workArea.Right) - width - panelExpandedEdgeGap - ((width * panelRightDockShiftPct) / 100)
	rightDockDistance := absInt(x - rightDockX)

	isNearLeft := leftDistance <= panelDockThreshold
	isNearRight := rightDistance <= panelDockThreshold || rightDockDistance <= panelDockThreshold

	switch {
	case isNearLeft && isNearRight:
		if leftDistance <= rightDistance {
			return panelDockLeft
		}
		return panelDockRight
	case isNearLeft:
		return panelDockLeft
	case isNearRight:
		return panelDockRight
	default:
		return panelDockFree
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func resolvePanelMonitorProbe(x int, y int, width int, height int, dockEdge panelDockEdge, collapsed bool) (int, int) {
	probeX := x + width/2
	if collapsed {
		switch dockEdge {
		case panelDockLeft:
			probeX = x + width - maxInt(panelTriggerWidth/2, 1)
		case panelDockRight:
			probeX = x + maxInt(panelTriggerWidth/2, 1)
		}
	}
	return probeX, y + height/2
}

func (a *App) SetPanelExpandedWidth(width int) error {
	if !a.isPanelMode() {
		return nil
	}
	debugLogf("set panel expanded width requested: width=%d state=%+v", width, a.getPanelStateSnapshot())
	if width < panelWindowWidth {
		width = panelWindowWidth
	}
	if width > panelMaxWindowWidth {
		width = panelMaxWindowWidth
	}
	appWindowState.mu.Lock()
	screenWidth := appWindowState.panel.screenWidth
	screenHeight := appWindowState.panel.screenHeight
	collapsed := appWindowState.panel.collapsed
	appWindowState.panel.expandedWidth = width
	appWindowState.mu.Unlock()
	debugLogf("set panel expanded width stored: width=%d collapsed=%t", width, collapsed)
	if nativePanelControllerSupported() {
		if collapsed {
			return nil
		}
		hwnd, err := findPanelWindowHandle()
		if err != nil || hwnd == 0 {
			return nil
		}
		workArea := resolvePanelWorkArea(screenWidth, screenHeight)
		state := a.getPanelStateSnapshot()
		bounds := a.resolveNativePanelBounds(workArea, state)
		if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
			return err
		}
		a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
		debugLogf("set panel expanded width applied native: width=%d bounds=%dx%d pos=(%d,%d) dock=%s", width, bounds.Width, bounds.Height, bounds.X, bounds.Y, state.dockEdge)
		return nil
	}
	return a.applyPanelWindowState(screenWidth, screenHeight, collapsed)
}

func (a *App) ResetPanelExpandedWidth() error {
	if !a.isPanelMode() {
		return nil
	}
	debugLogf("reset panel expanded width requested: state=%+v", a.getPanelStateSnapshot())
	appWindowState.mu.Lock()
	screenWidth := appWindowState.panel.screenWidth
	screenHeight := appWindowState.panel.screenHeight
	collapsed := appWindowState.panel.collapsed
	appWindowState.panel.expandedWidth = panelWideWindowWidth
	appWindowState.mu.Unlock()
	debugLogf("reset panel expanded width stored: width=%d collapsed=%t", panelWideWindowWidth, collapsed)
	if nativePanelControllerSupported() {
		if collapsed {
			return nil
		}
		hwnd, err := findPanelWindowHandle()
		if err != nil || hwnd == 0 {
			return nil
		}
		workArea := resolvePanelWorkArea(screenWidth, screenHeight)
		state := a.getPanelStateSnapshot()
		bounds := a.resolveNativePanelBounds(workArea, state)
		if err := a.ensureNativePanelVisible(hwnd, bounds); err != nil {
			return err
		}
		a.updatePanelRuntimeState(false, workArea, bounds, state.dockEdge)
		debugLogf("reset panel expanded width applied native: bounds=%dx%d pos=(%d,%d) dock=%s", bounds.Width, bounds.Height, bounds.X, bounds.Y, state.dockEdge)
		return nil
	}
	return a.applyPanelWindowState(screenWidth, screenHeight, collapsed)
}

func (a *App) initPanelWindow() {
	if !a.isPanelMode() || a.ctx == nil {
		return
	}
	if a.isManualPanelStart() {
		_ = a.applyManualPanelWindowState(0, 0)
		return
	}
	appWindowState.mu.Lock()
	appWindowState.panel.preferredX = 0
	appWindowState.panel.hasPreferredX = false
	appWindowState.panel.preferredY = 0
	appWindowState.panel.hasPreferredY = false
	appWindowState.panel.dockEdge = panelDockRight
	appWindowState.mu.Unlock()
	if nativePanelControllerSupported() {
		a.startPanelAutoController()
		delays := []time.Duration{
			60 * time.Millisecond,
			180 * time.Millisecond,
			420 * time.Millisecond,
			900 * time.Millisecond,
		}
		for _, delay := range delays {
			time.Sleep(delay)
			hwnd, err := findPanelWindowHandle()
			if err == nil && hwnd != 0 {
				_ = hideNativePanelWindow(hwnd)
			}
			_ = hidePanelWindowFromTaskbar()
		}
		return
	}
	_ = hidePanelWindowFromTaskbar()
	delays := []time.Duration{
		120 * time.Millisecond,
		420 * time.Millisecond,
		900 * time.Millisecond,
	}
	for _, delay := range delays {
		time.Sleep(delay)
		_ = hidePanelWindowFromTaskbar()
	}
}

func (a *App) startPanelProcess() error {
	return a.startPanelProcessWithMode(panelStartAuto)
}

func (a *App) startPanelProcessWithMode(panelStart panelStartMode) error {
	if a.isPanelMode() {
		return nil
	}

	a.panelMu.Lock()
	defer a.panelMu.Unlock()

	if a.panelCmd != nil && a.panelCmd.Process != nil {
		if a.panelCmd.ProcessState == nil || !a.panelCmd.ProcessState.Exited() {
			return nil
		}
		a.panelCmd = nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}

	args := []string{"--panel"}
	if panelStart == panelStartManual {
		args = append(args, "--panel-manual")
	}
	debugLogf("start panel process requested: mode=%s args=%v", panelStart, args)

	cmd := exec.Command(exePath, args...)
	configureWindowedAppCmd(cmd)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", webviewGroupPIDEnvKey, strconv.Itoa(resolveWebviewGroupPID(a.mode))))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start panel failed: %w", err)
	}
	debugLogf("panel child launched pid=%d mode=%s", cmd.Process.Pid, panelStart)
	a.panelCmd = cmd

	go func(process *exec.Cmd) {
		waitErr := process.Wait()
		debugLogf("panel child exited pid=%d err=%v", process.Process.Pid, waitErr)
		a.panelMu.Lock()
		if a.panelCmd == process {
			a.panelCmd = nil
		}
		a.panelMu.Unlock()
	}(cmd)

	return nil
}

func (a *App) OpenKeyEditor(rowKey string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}

	args := []string{"--editor"}
	if rowKey != "" {
		args = append(args, "--row-key", rowKey)
	}

	cmd := exec.Command(exePath, args...)
	configureWindowedAppCmd(cmd)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", webviewGroupPIDEnvKey, strconv.Itoa(resolveWebviewGroupPID(a.mode))))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start key editor failed: %w", err)
	}
	debugLogf("key editor child launched pid=%d rowKey=%q", cmd.Process.Pid, rowKey)

	go func(process *exec.Cmd) {
		_ = process.Wait()
	}(cmd)

	return nil
}

func (a *App) OpenDesktopConfigWindow(rowKey string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}

	args := []string{"--desktop-config"}
	if rowKey != "" {
		args = append(args, "--row-key", rowKey)
	}

	cmd := exec.Command(exePath, args...)
	configureWindowedAppCmd(cmd)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", webviewGroupPIDEnvKey, strconv.Itoa(resolveWebviewGroupPID(a.mode))))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start desktop config failed: %w", err)
	}
	debugLogf("desktop config child launched pid=%d rowKey=%q", cmd.Process.Pid, rowKey)

	go func(process *exec.Cmd) {
		_ = process.Wait()
	}(cmd)

	return nil
}

func (a *App) stopPanelProcess() {
	a.panelMu.Lock()
	defer a.panelMu.Unlock()

	if a.panelCmd == nil || a.panelCmd.Process == nil {
		return
	}
	killProcessTree(a.panelCmd.Process.Pid)
	a.panelCmd = nil
}

func (a *App) panelProcessRunning() bool {
	a.panelMu.Lock()
	defer a.panelMu.Unlock()
	return a.panelCmd != nil && a.panelCmd.Process != nil && (a.panelCmd.ProcessState == nil || !a.panelCmd.ProcessState.Exited())
}

func panelSignalPath() string {
	return filepath.Join(resolveRuntimeRootDir(), panelRestoreSignal)
}

func panelManualShowSignalPath() string {
	return filepath.Join(resolveRuntimeRootDir(), panelManualShowSignal)
}

func panelManualReadySignalPath() string {
	return filepath.Join(resolveRuntimeRootDir(), panelManualReadySignal)
}

func writePanelRestoreSignal() error {
	path := panelSignalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0o644)
}

func consumePanelRestoreSignal() bool {
	path := panelSignalPath()
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_ = os.Remove(path)
	return true
}

func writePanelManualShowSignal() error {
	path := panelManualShowSignalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0o644)
}

func consumePanelManualShowSignal() bool {
	path := panelManualShowSignalPath()
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_ = os.Remove(path)
	return true
}

func writePanelManualReadySignal() error {
	path := panelManualReadySignalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0o644)
}

func clearPanelManualReadySignal() {
	_ = os.Remove(panelManualReadySignalPath())
}

func waitForPanelManualReadySignal(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	path := panelManualReadySignalPath()
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			_ = os.Remove(path)
			return true
		}
		time.Sleep(80 * time.Millisecond)
	}
	return false
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func resolvePanelWorkArea(screenWidth int, screenHeight int) desktopRect {
	if workArea, err := getCursorWorkArea(); err == nil && workArea.Width() > 0 && workArea.Height() > 0 {
		return workArea
	}
	if workArea, err := getDesktopWorkArea(); err == nil && workArea.Width() > 0 && workArea.Height() > 0 {
		return workArea
	}
	if screenWidth <= 0 {
		screenWidth = panelWideWindowWidth + panelTriggerWidth + 12
	}
	if screenHeight <= 0 {
		screenHeight = panelWindowHeight + panelWindowMarginY*2
	}
	return desktopRect{
		Left:   0,
		Top:    0,
		Right:  int32(screenWidth),
		Bottom: int32(screenHeight),
	}
}

func resolvePanelDockWorkArea(screenWidth int, screenHeight int, dockEdge panelDockEdge) desktopRect {
	if dockEdge != panelDockFree && screenWidth > 0 && screenHeight > 0 {
		return desktopRect{
			Left:   0,
			Top:    0,
			Right:  int32(screenWidth),
			Bottom: int32(screenHeight),
		}
	}
	return resolvePanelWorkArea(screenWidth, screenHeight)
}
