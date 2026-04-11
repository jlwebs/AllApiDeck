//go:build !windows

package main

func nativePanelControllerSupported() bool {
	return false
}

func findPanelWindowHandle() (uintptr, error) {
	return 0, nil
}

func hideNativePanelWindow(hwnd uintptr) error {
	return nil
}

func showNativePanelWindow(hwnd uintptr, bounds sidebarWindowBounds) error {
	return nil
}

func isNativePanelWindowVisible(hwnd uintptr) bool {
	return false
}

func isNativePanelWindowForeground(hwnd uintptr) bool {
	return false
}

func getNativePanelWindowRect(hwnd uintptr) (desktopRect, error) {
	return desktopRect{}, nil
}
