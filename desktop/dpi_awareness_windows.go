//go:build windows

package main

import "golang.org/x/sys/windows"

const (
	processPerMonitorDPIAware = 2
)

var (
	user32DPIDLL                      = windows.NewLazySystemDLL("user32.dll")
	shcoreDPIDLL                      = windows.NewLazySystemDLL("shcore.dll")
	procSetProcessDpiAwarenessContext = user32DPIDLL.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32DPIDLL.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness        = shcoreDPIDLL.NewProc("SetProcessDpiAwareness")
)

func enableProcessDPIAwareness() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if ret, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3)); ret != 0 {
		return
	}
	if ret, _, _ := procSetProcessDpiAwareness.Call(uintptr(processPerMonitorDPIAware)); ret == 0 {
		return
	}
	_, _, _ = procSetProcessDPIAware.Call()
}
