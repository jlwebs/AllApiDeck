//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	swHidePanel              = 0
	swShowNoActivatePanel    = 4
	hwndTopmostPanel         = ^uintptr(0)
	swpNoActivatePanel       = 0x0010
	swpShowWindowPanel       = 0x0040
	dwmwaExtendedFrameBounds = 9
)

var (
	procShowWindowPanel       = taskbarUser32DLL.NewProc("ShowWindow")
	procIsWindowVisiblePanel  = taskbarUser32DLL.NewProc("IsWindowVisible")
	procGetWindowRectPanel    = taskbarUser32DLL.NewProc("GetWindowRect")
	procGetForegroundWindow   = taskbarUser32DLL.NewProc("GetForegroundWindow")
	dwmapiDLL                 = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmGetWindowAttribute = dwmapiDLL.NewProc("DwmGetWindowAttribute")
)

func nativePanelControllerSupported() bool {
	return true
}

func findPanelWindowHandle() (uintptr, error) {
	titlePtr, err := windows.UTF16PtrFromString(panelWindowTitleTaskbar)
	if err != nil {
		return 0, err
	}
	hwnd, _, findErr := procFindWindowWTaskbar.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("FindWindowW failed: %v", findErr)
	}
	return hwnd, nil
}

func hideNativePanelWindow(hwnd uintptr) error {
	if hwnd == 0 {
		return fmt.Errorf("panel hwnd is empty")
	}
	_, _, err := procShowWindowPanel.Call(hwnd, uintptr(swHidePanel))
	if err != windows.ERROR_SUCCESS && err != nil {
		return fmt.Errorf("ShowWindow hide failed: %v", err)
	}
	return nil
}

func showNativePanelWindow(hwnd uintptr, bounds sidebarWindowBounds) error {
	if hwnd == 0 {
		return fmt.Errorf("panel hwnd is empty")
	}
	ret, _, err := procSetWindowPosTaskbar.Call(
		hwnd,
		hwndTopmostPanel,
		uintptr(bounds.X),
		uintptr(bounds.Y),
		uintptr(bounds.Width),
		uintptr(bounds.Height),
		uintptr(swpNoActivatePanel|swpShowWindowPanel),
	)
	if ret == 0 {
		return fmt.Errorf("SetWindowPos failed: %v", err)
	}
	_, _, showErr := procShowWindowPanel.Call(hwnd, uintptr(swShowNoActivatePanel))
	if showErr != windows.ERROR_SUCCESS && showErr != nil {
		return fmt.Errorf("ShowWindow show failed: %v", showErr)
	}
	return nil
}

func isNativePanelWindowVisible(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindowVisiblePanel.Call(hwnd)
	return ret != 0
}

func isNativePanelWindowForeground(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	fg, _, _ := procGetForegroundWindow.Call()
	return fg == hwnd
}

func getNativePanelWindowRect(hwnd uintptr) (desktopRect, error) {
	if hwnd == 0 {
		return desktopRect{}, fmt.Errorf("panel hwnd is empty")
	}

	rect := desktopRect{}
	if ret, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaExtendedFrameBounds),
		uintptr(unsafe.Pointer(&rect)),
		unsafe.Sizeof(rect),
	); ret == 0 && rect.Width() > 0 && rect.Height() > 0 {
		return rect, nil
	}

	rect = desktopRect{}
	ret, _, err := procGetWindowRectPanel.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return desktopRect{}, fmt.Errorf("GetWindowRect failed: %v", err)
	}
	return rect, nil
}
