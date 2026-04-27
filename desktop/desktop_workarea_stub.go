//go:build !windows

package main

import "fmt"

type desktopRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func (r desktopRect) Width() int {
	return int(r.Right - r.Left)
}

func (r desktopRect) Height() int {
	return int(r.Bottom - r.Top)
}

func getDesktopWorkArea() (desktopRect, error) {
	return desktopRect{}, fmt.Errorf("desktop work area is only implemented on windows")
}

func getCursorWorkArea() (desktopRect, error) {
	return getDesktopWorkArea()
}

func getMonitorWorkAreaForPoint(x int, y int) (desktopRect, error) {
	return getDesktopWorkArea()
}

func getCursorPosition() (int, int, error) {
	return 0, 0, fmt.Errorf("cursor position is only implemented on windows")
}
