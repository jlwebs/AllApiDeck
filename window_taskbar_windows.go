//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	panelWindowTitleTaskbar = "All API Dock Panel"
	wsExToolWindowTaskbar   = 0x00000080
	wsExAppWindowTaskbar    = 0x00040000
	swpNoSizeTaskbar        = 0x0001
	swpNoMoveTaskbar        = 0x0002
	swpNoZOrderTaskbar      = 0x0004
	swpNoActivateTaskbar    = 0x0010
	swpFrameChangedTaskbar  = 0x0020
)

var (
	gwlExStyleTaskbar         int32 = -20
	taskbarUser32DLL          = windows.NewLazySystemDLL("user32.dll")
	procFindWindowWTaskbar    = taskbarUser32DLL.NewProc("FindWindowW")
	procGetWindowLongWTaskbar = taskbarUser32DLL.NewProc("GetWindowLongW")
	procSetWindowLongWTaskbar = taskbarUser32DLL.NewProc("SetWindowLongW")
	procSetWindowPosTaskbar   = taskbarUser32DLL.NewProc("SetWindowPos")
)

func hidePanelWindowFromTaskbar() error {
	titlePtr, err := windows.UTF16PtrFromString(panelWindowTitleTaskbar)
	if err != nil {
		return err
	}
	hwnd, _, findErr := procFindWindowWTaskbar.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
	)
	if hwnd == 0 {
		return fmt.Errorf("FindWindowW failed: %v", findErr)
	}

	exStyleValue, _, getErr := procGetWindowLongWTaskbar.Call(hwnd, uintptr(gwlExStyleTaskbar))
	if exStyleValue == 0 && getErr != windows.ERROR_SUCCESS && getErr != nil {
		return fmt.Errorf("GetWindowLongW failed: %v", getErr)
	}

	nextStyle := (uint32(exStyleValue) | wsExToolWindowTaskbar) &^ wsExAppWindowTaskbar
	_, _, setErr := procSetWindowLongWTaskbar.Call(
		hwnd,
		uintptr(gwlExStyleTaskbar),
		uintptr(nextStyle),
	)
	if setErr != windows.ERROR_SUCCESS && setErr != nil {
		return fmt.Errorf("SetWindowLongW failed: %v", setErr)
	}

	_, _, posErr := procSetWindowPosTaskbar.Call(
		hwnd,
		0,
		0,
		0,
		0,
		0,
		uintptr(swpNoSizeTaskbar|swpNoMoveTaskbar|swpNoZOrderTaskbar|swpNoActivateTaskbar|swpFrameChangedTaskbar),
	)
	if posErr != windows.ERROR_SUCCESS && posErr != nil {
		return fmt.Errorf("SetWindowPos failed: %v", posErr)
	}

	return nil
}
