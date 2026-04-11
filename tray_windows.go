//go:build windows

package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	trayWMAppCallback = 0x8000 + 7
	trayWMCommand     = 0x0111
	trayWMClose       = 0x0010
	trayWMDestroy     = 0x0002
	trayWMNull        = 0x0000
	trayWMRButtonUp   = 0x0205
	trayWMLButtonUp   = 0x0202
	trayWMLButtonDbl  = 0x0203
	trayWMContextMenu = 0x007B

	trayWSOverlapped = 0

	trayNIMAdd    = 0x00000000
	trayNIMDelete = 0x00000002

	trayNIFMessage = 0x00000001
	trayNIFIcon    = 0x00000002
	trayNIFTip     = 0x00000004
	trayNIFShowTip = 0x00000080

	trayMFString    = 0x00000000
	trayMFSeparator = 0x00000800

	trayTPMLeftAlign   = 0x0000
	trayTPMBottomAlign = 0x0020
	trayTPMRightButton = 0x0002

	trayImageIcon      = 1
	trayLRLoadFromFile = 0x00000010
	trayLRDefaultSize  = 0x00000040
	trayIDIApplication = 32512
	trayIDCArrow       = 32512
	trayColorWindow    = 5

	trayMenuShow = 1001
	trayMenuQuit = 1002
)

type trayPoint struct {
	X int32
	Y int32
}

type trayMsg struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       trayPoint
	LPrivate uint32
}

type trayWndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type trayNotifyIconData struct {
	CbSize            uint32
	HWnd              uintptr
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	HIcon             uintptr
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          windows.GUID
	HBalloonIcon      uintptr
}

type windowsTray struct {
	showWindow func()
	quitApp    func()

	className string
	instance  uintptr
	hwnd      uintptr
	menu      uintptr
	icon      uintptr

	once sync.Once
}

var (
	trayUser32          = windows.NewLazySystemDLL("user32.dll")
	trayShell32         = windows.NewLazySystemDLL("shell32.dll")
	trayKernel32        = windows.NewLazySystemDLL("kernel32.dll")
	trayRegisterClassEx = trayUser32.NewProc("RegisterClassExW")
	trayCreateWindowEx  = trayUser32.NewProc("CreateWindowExW")
	trayDefWindowProc   = trayUser32.NewProc("DefWindowProcW")
	trayDestroyWindow   = trayUser32.NewProc("DestroyWindow")
	trayGetMessage      = trayUser32.NewProc("GetMessageW")
	trayTranslateMsg    = trayUser32.NewProc("TranslateMessage")
	trayDispatchMsg     = trayUser32.NewProc("DispatchMessageW")
	trayPostQuitMessage = trayUser32.NewProc("PostQuitMessage")
	trayPostMessage     = trayUser32.NewProc("PostMessageW")
	trayCreatePopup     = trayUser32.NewProc("CreatePopupMenu")
	trayAppendMenu      = trayUser32.NewProc("AppendMenuW")
	trayTrackPopup      = trayUser32.NewProc("TrackPopupMenu")
	traySetForeground   = trayUser32.NewProc("SetForegroundWindow")
	trayGetCursorPos    = trayUser32.NewProc("GetCursorPos")
	trayLoadIcon        = trayUser32.NewProc("LoadIconW")
	trayLoadCursor      = trayUser32.NewProc("LoadCursorW")
	trayLoadImage       = trayUser32.NewProc("LoadImageW")
	trayDestroyMenu     = trayUser32.NewProc("DestroyMenu")
	trayDestroyIcon     = trayUser32.NewProc("DestroyIcon")
	trayGetModuleHandle = trayKernel32.NewProc("GetModuleHandleW")
	trayNotifyIcon      = trayShell32.NewProc("Shell_NotifyIconW")
)

var trayInstances sync.Map

func (a *App) initTray() error {
	tray, err := newWindowsTray(
		func() {
			_ = a.ShowMainWindow()
		},
		func() {
			a.RequestQuit()
		},
	)
	if err != nil {
		return err
	}
	a.tray = tray
	return nil
}

func (a *App) closeTray() {
	if a.tray != nil {
		a.tray.Close()
	}
}

func newWindowsTray(showWindow func(), quitApp func()) (*windowsTray, error) {
	instance, _, _ := trayGetModuleHandle.Call(0)
	tray := &windowsTray{
		showWindow: showWindow,
		quitApp:    quitApp,
		instance:   instance,
		className:  "AllApiDockTrayWindowClass",
	}

	errCh := make(chan error, 1)
	go tray.run(errCh)
	if err := <-errCh; err != nil {
		return nil, err
	}
	return tray, nil
}

func (t *windowsTray) run(errCh chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	wndProc := syscall.NewCallback(trayWindowProc)
	classNamePtr, err := windows.UTF16PtrFromString(t.className)
	if err != nil {
		errCh <- err
		return
	}

	icon := loadTrayIcon(t.instance)
	cursor, _, _ := trayLoadCursor.Call(0, uintptr(trayIDCArrow))
	class := trayWndClassEx{
		CbSize:        uint32(unsafe.Sizeof(trayWndClassEx{})),
		LpfnWndProc:   wndProc,
		HInstance:     t.instance,
		HIcon:         icon,
		HCursor:       cursor,
		HbrBackground: trayColorWindow + 1,
		LpszClassName: classNamePtr,
		HIconSm:       icon,
	}

	result, _, classErr := trayRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))
	if result == 0 {
		errCh <- classErr
		return
	}

	hwnd, _, createErr := trayCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		0,
		trayWSOverlapped,
		0,
		0,
		0,
		0,
		0,
		0,
		t.instance,
		0,
	)
	if hwnd == 0 {
		errCh <- createErr
		return
	}

	menu, _, _ := trayCreatePopup.Call()
	trayAppendMenu.Call(menu, trayMFString, trayMenuShow, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Show Window"))))
	trayAppendMenu.Call(menu, trayMFSeparator, 0, 0)
	trayAppendMenu.Call(menu, trayMFString, trayMenuQuit, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Quit App"))))

	t.hwnd = hwnd
	t.menu = menu
	t.icon = icon
	trayInstances.Store(hwnd, t)

	if err := t.addNotifyIcon(); err != nil {
		trayInstances.Delete(hwnd)
		trayDestroyMenu.Call(menu)
		trayDestroyWindow.Call(hwnd)
		errCh <- err
		return
	}

	errCh <- nil

	var msg trayMsg
	for {
		next, _, _ := trayGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(next) <= 0 {
			break
		}
		trayTranslateMsg.Call(uintptr(unsafe.Pointer(&msg)))
		trayDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (t *windowsTray) addNotifyIcon() error {
	notify := trayNotifyIconData{
		CbSize:           uint32(unsafe.Sizeof(trayNotifyIconData{})),
		HWnd:             t.hwnd,
		UID:              1,
		UFlags:           trayNIFMessage | trayNIFIcon | trayNIFTip | trayNIFShowTip,
		UCallbackMessage: trayWMAppCallback,
		HIcon:            t.icon,
	}
	copyTrayUTF16(notify.SzTip[:], "All API Dock")
	result, _, err := trayNotifyIcon.Call(trayNIMAdd, uintptr(unsafe.Pointer(&notify)))
	if result == 0 {
		return err
	}
	return nil
}

func (t *windowsTray) removeNotifyIcon() {
	notify := trayNotifyIconData{
		CbSize: uint32(unsafe.Sizeof(trayNotifyIconData{})),
		HWnd:   t.hwnd,
		UID:    1,
	}
	trayNotifyIcon.Call(trayNIMDelete, uintptr(unsafe.Pointer(&notify)))
}

func (t *windowsTray) Close() {
	t.once.Do(func() {
		if t.hwnd != 0 {
			trayPostMessage.Call(t.hwnd, trayWMClose, 0, 0)
		}
	})
}

func (t *windowsTray) showContextMenu() {
	if t.hwnd == 0 || t.menu == 0 {
		return
	}
	var pt trayPoint
	trayGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	traySetForeground.Call(t.hwnd)
	trayTrackPopup.Call(
		t.menu,
		trayTPMLeftAlign|trayTPMBottomAlign|trayTPMRightButton,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		t.hwnd,
		0,
	)
	trayPostMessage.Call(t.hwnd, trayWMNull, 0, 0)
}

func trayWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	rawTray, ok := trayInstances.Load(hwnd)
	if !ok {
		ret, _, _ := trayDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
	tray := rawTray.(*windowsTray)

	switch msg {
	case trayWMAppCallback:
		switch uint32(lParam) {
		case trayWMLButtonUp, trayWMLButtonDbl:
			if tray.showWindow != nil {
				go tray.showWindow()
			}
		case trayWMRButtonUp, trayWMContextMenu:
			tray.showContextMenu()
		}
		return 0
	case trayWMCommand:
		switch trayLowWord(uint32(wParam)) {
		case trayMenuShow:
			if tray.showWindow != nil {
				go tray.showWindow()
			}
		case trayMenuQuit:
			if tray.quitApp != nil {
				go tray.quitApp()
			}
		}
		return 0
	case trayWMClose:
		tray.removeNotifyIcon()
		trayInstances.Delete(hwnd)
		if tray.menu != 0 {
			trayDestroyMenu.Call(tray.menu)
		}
		if tray.icon != 0 {
			trayDestroyIcon.Call(tray.icon)
		}
		trayDestroyWindow.Call(hwnd)
		return 0
	case trayWMDestroy:
		trayPostQuitMessage.Call(0)
		return 0
	default:
		ret, _, _ := trayDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

func loadTrayIcon(instance uintptr) uintptr {
	if path, err := ensureRuntimeWindowsAppIconPath(); err == nil && path != "" {
		ptr, err := windows.UTF16PtrFromString(path)
		if err == nil {
			icon, _, _ := trayLoadImage.Call(
				0,
				uintptr(unsafe.Pointer(ptr)),
				trayImageIcon,
				0,
				0,
				trayLRLoadFromFile|trayLRDefaultSize,
			)
			if icon != 0 {
				return icon
			}
		}
	}

	icon, _, _ := trayLoadIcon.Call(instance, uintptr(trayIDIApplication))
	if icon != 0 {
		return icon
	}
	icon, _, _ = trayLoadIcon.Call(0, uintptr(trayIDIApplication))
	return icon
}

func trayLowWord(value uint32) uintptr {
	return uintptr(value & 0xffff)
}

func copyTrayUTF16(target []uint16, text string) {
	copy(target, windows.StringToUTF16(text))
}
