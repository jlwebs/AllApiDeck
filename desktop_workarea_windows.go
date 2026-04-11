//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

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

var (
	workAreaUser32DLL                = windows.NewLazySystemDLL("user32.dll")
	procWorkAreaSystemParametersInfo = workAreaUser32DLL.NewProc("SystemParametersInfoW")
	procGetCursorPos                 = workAreaUser32DLL.NewProc("GetCursorPos")
	procMonitorFromPoint             = workAreaUser32DLL.NewProc("MonitorFromPoint")
	procGetMonitorInfoW              = workAreaUser32DLL.NewProc("GetMonitorInfoW")
)

const (
	spiGetWorkArea          = 0x0030
	monitorDefaultToNearest = 0x00000002
)

type desktopPoint struct {
	X int32
	Y int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor desktopRect
	RcWork    desktopRect
	DwFlags   uint32
}

func getDesktopWorkArea() (desktopRect, error) {
	rect := desktopRect{}
	ret, _, err := procWorkAreaSystemParametersInfo.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&rect)),
		0,
	)
	if ret == 0 {
		return desktopRect{}, fmt.Errorf("SystemParametersInfoW failed: %w", err)
	}
	return rect, nil
}

func getCursorWorkArea() (desktopRect, error) {
	x, y, err := getCursorPosition()
	if err != nil {
		return desktopRect{}, err
	}
	return getMonitorWorkAreaForPoint(x, y)
}

func getCursorPosition() (int, int, error) {
	point := desktopPoint{}
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if ret == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos failed: %w", err)
	}
	return int(point.X), int(point.Y), nil
}

func getMonitorWorkAreaForPoint(x int, y int) (desktopRect, error) {
	point := desktopPoint{X: int32(x), Y: int32(y)}
	monitor, _, monitorErr := procMonitorFromPoint.Call(
		*(*uintptr)(unsafe.Pointer(&point)),
		uintptr(monitorDefaultToNearest),
	)
	if monitor == 0 {
		return desktopRect{}, fmt.Errorf("MonitorFromPoint failed: %w", monitorErr)
	}

	info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	ret, _, infoErr := procGetMonitorInfoW.Call(
		monitor,
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return desktopRect{}, fmt.Errorf("GetMonitorInfoW failed: %w", infoErr)
	}
	if info.RcWork.Width() <= 0 || info.RcWork.Height() <= 0 {
		return desktopRect{}, fmt.Errorf("monitor work area is empty")
	}
	return info.RcWork, nil
}
