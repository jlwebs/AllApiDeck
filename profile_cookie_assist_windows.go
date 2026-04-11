//go:build windows

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
	_ "modernc.org/sqlite"
)

type desktopProfileAssistOpenRequest struct {
	SiteName string `json:"siteName"`
	SiteURL  string `json:"siteUrl"`
	SiteType string `json:"siteType"`
}

type desktopProfileAssistCookie struct {
	HostKey    string
	Name       string
	Value      string
	Path       string
	IsSecure   bool
	IsHTTPOnly bool
	SameSite   int32
}

type desktopProfileAssistWindowResult struct {
	SiteName            string   `json:"siteName"`
	SiteURL             string   `json:"siteUrl"`
	InjectedCookies     int      `json:"injectedCookies"`
	InjectedCookieNames []string `json:"injectedCookieNames,omitempty"`
	StorageFields       []string `json:"storageFields,omitempty"`
	Message             string   `json:"message,omitempty"`
}

type desktopProfileAssistWindowOpenResult struct {
	result *desktopProfileAssistWindowResult
	err    error
}

type profileAssistWebMessage struct {
	Source       string   `json:"source"`
	Type         string   `json:"type"`
	Origin       string   `json:"origin,omitempty"`
	Href         string   `json:"href,omitempty"`
	Title        string   `json:"title,omitempty"`
	ReadyState   string   `json:"readyState,omitempty"`
	InjectedKeys []string `json:"injectedKeys,omitempty"`
	LoggedIn     bool     `json:"loggedIn,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	ElapsedMs    int64    `json:"elapsedMs,omitempty"`
	StorageKeys  []string `json:"storageKeys,omitempty"`
	Pathname     string   `json:"pathname,omitempty"`
}

var (
	user32DLL                       = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW            = user32DLL.NewProc("RegisterClassExW")
	procCreateWindowExW             = user32DLL.NewProc("CreateWindowExW")
	procShowWindow                  = user32DLL.NewProc("ShowWindow")
	procUpdateWindow                = user32DLL.NewProc("UpdateWindow")
	procSetFocus                    = user32DLL.NewProc("SetFocus")
	procGetMessageW                 = user32DLL.NewProc("GetMessageW")
	procTranslateMessage            = user32DLL.NewProc("TranslateMessage")
	procDispatchMessageW            = user32DLL.NewProc("DispatchMessageW")
	procDefWindowProcW              = user32DLL.NewProc("DefWindowProcW")
	procDestroyWindow               = user32DLL.NewProc("DestroyWindow")
	procPostMessageW                = user32DLL.NewProc("PostMessageW")
	procPostQuitMessage             = user32DLL.NewProc("PostQuitMessage")
	procGetClientRect               = user32DLL.NewProc("GetClientRect")
	procLoadImageW                  = user32DLL.NewProc("LoadImageW")
	procGetSystemMetrics            = user32DLL.NewProc("GetSystemMetrics")
	profileAssistWindowClassName, _ = windows.UTF16PtrFromString("BatchApiCheckProfileAssistWindow")
	profileAssistWndProc            = windows.NewCallback(profileAssistWindowProc)
	profileAssistWindowClassOnce    sync.Once
	profileAssistWindowClassErr     error
	profileAssistWindowStateMu      sync.Mutex
	profileAssistWindowStates       = map[uintptr]*profileAssistWindowState{}
	crypt32DLL                      = syscall.NewLazyDLL("crypt32.dll")
	kernel32DLL                     = syscall.NewLazyDLL("kernel32.dll")
	procCryptUnprotectData          = crypt32DLL.NewProc("CryptUnprotectData")
	procLocalFree                   = kernel32DLL.NewProc("LocalFree")
)

const (
	profileAssistWSOverlappedWindow  = 0x00CF0000
	profileAssistSWShow              = 5
	profileAssistWMDestroy           = 0x0002
	profileAssistWMSize              = 0x0005
	profileAssistWMClose             = 0x0010
	profileAssistWMQuit              = 0x0012
	profileAssistSystemMetricsCxIcon = 11
	profileAssistSystemMetricsCyIcon = 12
	profileAssistLRLoadFromFile      = 0x00000010
	profileAssistLRDefaultSize       = 0x00000040
	profileAssistImageIcon           = 1
)

type profileAssistWndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type profileAssistRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type profileAssistMsg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct {
		X int32
		Y int32
	}
	LPrivate uint32
}

type profileAssistWindowState struct {
	chromium *edge.Chromium
}

type chromeLocalStateFile struct {
	OSCrypt struct {
		EncryptedKey string `json:"encrypted_key"`
	} `json:"os_crypt"`
}

type dpapiDataBlob struct {
	cbData uint32
	pbData *byte
}

func profileAssistLogPath() string {
	return filepath.Join(resolveRuntimeLogDir(), "profile-assist.log")
}

func truncateProfileAssistLogText(input string, limit int) string {
	input = strings.TrimSpace(input)
	if limit <= 0 || len(input) <= limit {
		return input
	}
	return input[:limit] + "..."
}

func parseProfileAssistWebMessage(raw string) *profileAssistWebMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" || (!strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[")) {
		return nil
	}
	var payload profileAssistWebMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return &payload
}

func openDesktopProfileAssistWindow(request desktopProfileAssistOpenRequest) (*desktopProfileAssistWindowResult, error) {
	resultCh := make(chan desktopProfileAssistWindowOpenResult, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		runDesktopProfileAssistWindow(request, resultCh)
	}()

	select {
	case outcome := <-resultCh:
		return outcome.result, outcome.err
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("profile assist window start timeout")
	}
}

func runDesktopProfileAssistWindow(request desktopProfileAssistOpenRequest, resultCh chan<- desktopProfileAssistWindowOpenResult) {
	siteURL := strings.TrimSpace(request.SiteURL)
	if siteURL == "" {
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("site url is empty")}
		return
	}

	origin, err := normalizeURLOrigin(siteURL)
	if err != nil {
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("invalid site url: %w", err)}
		return
	}

	parsedURL, err := url.Parse(siteURL)
	if err != nil {
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("parse site url failed: %w", err)}
		return
	}
	host := strings.ToLower(parsedURL.Hostname())
	siteName := strings.TrimSpace(request.SiteName)
	if siteName == "" {
		siteName = host
	}

	cookies, err := loadChromeCookiesForHost(host)
	if err != nil {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] %s cookies load failed | %v", siteURL, err))
	}

	storageValues := map[string]string{}
	storageFields := []string{}
	if snapshot, _, snapshotErr := loadChromeProfileAuthSnapshot(); snapshotErr == nil && snapshot != nil {
		if rawValues := snapshot.entries[origin]; len(rawValues) > 0 {
			storageValues = cloneStringMap(rawValues)
			for key := range rawValues {
				storageFields = append(storageFields, key)
			}
		}
	}

	shouldInjectStorage, storageSkipReason := shouldInjectProfileAssistStorage(request, host, storageValues, cookies)
	if !shouldInjectStorage && storageSkipReason != "" {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] storage injection skipped %s | reason=%s", siteURL, storageSkipReason))
		storageValues = map[string]string{}
		storageFields = []string{}
	}
	initialNavigateURL := resolveProfileAssistInitialURL(origin, request, host, len(cookies), shouldInjectStorage)

	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && !errors.Is(err, syscall.Errno(0x00000001)) {
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("CoInitializeEx failed: %w", err)}
		return
	}
	defer windows.CoUninitialize()

	if err := ensureProfileAssistWindowClass(); err != nil {
		resultCh <- desktopProfileAssistWindowOpenResult{err: err}
		return
	}

	var hinstance windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &hinstance)

	windowTitle, _ := windows.UTF16PtrFromString(fmt.Sprintf("Batch API Check - %s Profile Assist", siteName))
	hwnd, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(profileAssistWindowClassName)),
		uintptr(unsafe.Pointer(windowTitle)),
		profileAssistWSOverlappedWindow,
		120,
		120,
		1280,
		900,
		0,
		0,
		uintptr(hinstance),
		0,
	)
	if hwnd == 0 {
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("CreateWindowExW failed: %v", createErr)}
		return
	}

	_, _, _ = procShowWindow.Call(hwnd, profileAssistSWShow)
	_, _, _ = procUpdateWindow.Call(hwnd)
	_, _, _ = procSetFocus.Call(hwnd)

	chromium := edge.NewChromium()
	var closeWindowOnce sync.Once
	closeWindow := func(reason string) {
		closeWindowOnce.Do(func() {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] closing window %s | reason=%s", siteURL, strings.TrimSpace(reason)))
			if controller := chromium.GetController(); controller != nil {
				_ = controller.PutIsVisible(false)
			}
			ret, _, err := procPostMessageW.Call(hwnd, profileAssistWMClose, 0, 0)
			if ret == 0 {
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] post close failed %s | %v", siteURL, err))
			}
		})
	}
	chromium.MessageCallback = func(message string, sender *edge.ICoreWebView2, args *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		_ = sender
		_ = args
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] web message %s | %s", siteURL, truncateProfileAssistLogText(message, 800)))
		payload := parseProfileAssistWebMessage(message)
		if payload == nil || !strings.EqualFold(strings.TrimSpace(payload.Source), "profile-assist") {
			return
		}
		switch strings.TrimSpace(payload.Type) {
		case "auth-ready":
			closeWindow("auth-ready")
		case "assist-timeout":
			closeWindow("assist-timeout")
		}
	}
	chromium.SetErrorCallback(func(webErr error) {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] webview error %s | %v", siteURL, webErr))
	})
	chromium.DataPath = filepath.Join(resolveRuntimeRootDir(), "webview2-assist", sanitizeProfileAssistSegment(host))
	navigationCount := 0
	chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
		_ = args
		navigationCount++
		currentURL := ""
		if sender != nil {
			currentURL, _ = sender.GetSource()
		}
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigation completed %s | count=%d | url=%s", siteURL, navigationCount, currentURL))
	}

	registerProfileAssistWindowState(hwnd, chromium)
	defer unregisterProfileAssistWindowState(hwnd)

	if ok := chromium.Embed(hwnd); !ok {
		_ = destroyProfileAssistWindow(hwnd)
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("webview2 embed failed")}
		return
	}
	chromium.Resize()

	if script := buildProfileAssistStorageBootstrapScript(origin, storageValues); script != "" {
		chromium.Init(script)
	}
	if traceScript := buildProfileAssistTraceScript(origin); traceScript != "" {
		chromium.Init(traceScript)
	}

	injectedCookieNames, err := injectChromeCookiesIntoWebView(chromium, cookies)
	if err != nil {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] inject cookies failed %s | %v", siteURL, err))
	}

	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] open window %s | cookies=%d | storageFields=%d", siteURL, len(injectedCookieNames), len(storageFields)))
	chromium.Navigate(initialNavigateURL)

	result := &desktopProfileAssistWindowResult{
		SiteName:            siteName,
		SiteURL:             initialNavigateURL,
		InjectedCookies:     len(injectedCookieNames),
		InjectedCookieNames: injectedCookieNames,
		StorageFields:       storageFields,
		Message:             fmt.Sprintf("Opened WebView2 and attempted to inject %d cookies and %d storage fields", len(injectedCookieNames), len(storageFields)),
	}
	resultCh <- desktopProfileAssistWindowOpenResult{result: result}
	runProfileAssistMessageLoop(hwnd, chromium)
}

func runProfileAssistMessageLoop(hwnd uintptr, chromium *edge.Chromium) {
	var msg profileAssistMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), hwnd, 0, 0)
		if ret == 0 || ret == ^uintptr(0) || msg.Message == profileAssistWMQuit {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	if chromium != nil {
		chromium.ShuttingDown()
	}
}

func ensureProfileAssistWindowClass() error {
	profileAssistWindowClassOnce.Do(func() {
		var hinstance windows.Handle
		_ = windows.GetModuleHandleEx(0, nil, &hinstance)

		icow, _, _ := procGetSystemMetrics.Call(profileAssistSystemMetricsCxIcon)
		icoh, _, _ := procGetSystemMetrics.Call(profileAssistSystemMetricsCyIcon)
		var icon uintptr
		if iconPath, err := ensureRuntimeWindowsAppIconPath(); err == nil && iconPath != "" {
			if iconPathPtr, pathErr := windows.UTF16PtrFromString(iconPath); pathErr == nil {
				icon, _, _ = procLoadImageW.Call(
					0,
					uintptr(unsafe.Pointer(iconPathPtr)),
					profileAssistImageIcon,
					icow,
					icoh,
					profileAssistLRLoadFromFile|profileAssistLRDefaultSize,
				)
			}
		}
		if icon == 0 {
			icon, _, _ = procLoadImageW.Call(uintptr(hinstance), 32512, icow, icoh, 0)
		}

		wc := profileAssistWndClassEx{
			CbSize:        uint32(unsafe.Sizeof(profileAssistWndClassEx{})),
			HInstance:     hinstance,
			LpszClassName: profileAssistWindowClassName,
			HIcon:         windows.Handle(icon),
			HIconSm:       windows.Handle(icon),
			LpfnWndProc:   profileAssistWndProc,
		}
		ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		if ret == 0 && !errors.Is(err, windows.ERROR_CLASS_ALREADY_EXISTS) {
			profileAssistWindowClassErr = fmt.Errorf("RegisterClassExW failed: %w", err)
		}
	})
	return profileAssistWindowClassErr
}

func profileAssistWindowProc(hwnd uintptr, msg uint32, wparam uintptr, lparam uintptr) uintptr {
	switch msg {
	case profileAssistWMSize:
		if chromium := getProfileAssistChromium(hwnd); chromium != nil {
			chromium.Resize()
		}
	case profileAssistWMClose:
		_ = destroyProfileAssistWindow(hwnd)
		return 0
	case profileAssistWMDestroy:
		if chromium := getProfileAssistChromium(hwnd); chromium != nil {
			chromium.ShuttingDown()
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
}

func registerProfileAssistWindowState(hwnd uintptr, chromium *edge.Chromium) {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	profileAssistWindowStates[hwnd] = &profileAssistWindowState{chromium: chromium}
}

func unregisterProfileAssistWindowState(hwnd uintptr) {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	delete(profileAssistWindowStates, hwnd)
}

func getProfileAssistChromium(hwnd uintptr) *edge.Chromium {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	state := profileAssistWindowStates[hwnd]
	if state == nil {
		return nil
	}
	return state.chromium
}

func destroyProfileAssistWindow(hwnd uintptr) error {
	ret, _, err := procDestroyWindow.Call(hwnd)
	if ret == 0 {
		return err
	}
	return nil
}

func injectChromeCookiesIntoWebView(chromium *edge.Chromium, cookies []desktopProfileAssistCookie) ([]string, error) {
	if chromium == nil {
		return nil, fmt.Errorf("chromium is nil")
	}
	if len(cookies) == 0 {
		return nil, nil
	}

	manager, err := chromium.GetCookieManager()
	if err != nil {
		return nil, err
	}
	defer manager.Release()

	injectedNames := make([]string, 0, len(cookies))
	for _, item := range cookies {
		domain := strings.TrimSpace(strings.TrimPrefix(item.HostKey, "."))
		if domain == "" {
			continue
		}
		pathValue := strings.TrimSpace(item.Path)
		if pathValue == "" {
			pathValue = "/"
		}
		cookie, err := manager.CreateCookie(item.Name, item.Value, domain, pathValue)
		if err != nil {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] create cookie failed %s@%s | %v", item.Name, domain, err))
			continue
		}

		if item.IsSecure {
			_ = cookie.PutIsSecure(true)
		}
		if item.IsHTTPOnly {
			_ = cookie.PutIsHttpOnly(true)
		}
		if item.SameSite >= 0 {
			_ = cookie.PutSameSite(item.SameSite)
		}

		if err := manager.AddOrUpdateCookie(cookie); err != nil {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] add cookie failed %s@%s | %v", item.Name, domain, err))
			cookie.Release()
			continue
		}
		cookie.Release()
		injectedNames = append(injectedNames, item.Name)
	}

	return dedupeStrings(injectedNames), nil
}

func buildProfileAssistStorageBootstrapScript(origin string, storageValues map[string]string) string {
	if strings.TrimSpace(origin) == "" || len(storageValues) == 0 {
		return ""
	}

	payload, err := json.Marshal(storageValues)
	if err != nil {
		return ""
	}

	return fmt.Sprintf(`(() => {
  try {
    if (location.origin !== %q) {
      return;
    }
    const storageValues = %s;
    for (const [key, value] of Object.entries(storageValues)) {
      try {
        localStorage.setItem(key, String(value ?? ""));
      } catch {}
    }
    window.__batchApiCheckProfileAssist = {
      origin: location.origin,
      injectedKeys: Object.keys(storageValues),
      injectedAt: new Date().toISOString(),
    };
    if (window.chrome?.webview?.postMessage) {
      window.chrome.webview.postMessage(JSON.stringify({
        source: "profile-assist",
        type: "storage-injected",
        origin: location.origin,
        href: location.href,
        injectedKeys: Object.keys(storageValues),
      }));
    }
    console.info("[BatchApiCheck] profile assist storage injected", window.__batchApiCheckProfileAssist);
  } catch (error) {
    console.error("[BatchApiCheck] profile assist storage inject failed", error);
  }
})();`, origin, string(payload))
}

func buildProfileAssistTraceScript(origin string) string {
	if strings.TrimSpace(origin) == "" {
		return ""
	}

	return fmt.Sprintf(`(() => {
  try {
    if (window.__batchApiCheckProfileAssistTraceInstalled) {
      return;
    }
    window.__batchApiCheckProfileAssistTraceInstalled = true;
    const START_KEY = "__batchApiCheckProfileAssistStartedAt";
    const CHECK_INTERVAL_MS = 5000;
    const MAX_WAIT_MS = 20000;
    const LOGIN_PATTERNS = [/\/login(?:[\/?#]|$)/i, /\/signin(?:[\/?#]|$)/i, /\/auth(?:[\/?#]|$)/i];
    const DASHBOARD_PATTERNS = [/\/console/i, /\/dashboard/i, /\/panel/i, /\/token/i, /\/keys/i, /\/setting/i, /\/profile/i];
    const STORAGE_KEYS = ["auth_token", "access_token", "token", "authToken", "refresh_token", "user", "auth_user"];
    let startedAt = Number(sessionStorage.getItem(START_KEY) || "0");
    if (!Number.isFinite(startedAt) || startedAt <= 0) {
      startedAt = Date.now();
      sessionStorage.setItem(START_KEY, String(startedAt));
    }
    const post = (type) => {
      try {
        if (location.origin !== %q) {
          return;
        }
        if (window.chrome?.webview?.postMessage) {
          window.chrome.webview.postMessage(JSON.stringify({
            source: "profile-assist",
            type,
            href: location.href,
            origin: location.origin,
            title: document.title || "",
            readyState: document.readyState || "",
          }));
        }
      } catch {}
    };
    const postAuthState = (type, extra = {}) => {
      try {
        if (location.origin !== %q) {
          return;
        }
        if (window.chrome?.webview?.postMessage) {
          window.chrome.webview.postMessage(JSON.stringify({
            source: "profile-assist",
            type,
            href: location.href,
            origin: location.origin,
            pathname: location.pathname || "",
            elapsedMs: Date.now() - startedAt,
            ...extra,
          }));
        }
      } catch {}
    };
    const readStorageKeys = () => {
      const keys = [];
      for (const key of STORAGE_KEYS) {
        try {
          const value = localStorage.getItem(key);
          if (typeof value === "string" && value.trim()) {
            keys.push(key);
          }
        } catch {}
      }
      return keys;
    };
    const isLoginPath = () => LOGIN_PATTERNS.some(pattern => pattern.test(location.pathname || ""));
    const isDashboardPath = () => DASHBOARD_PATTERNS.some(pattern => pattern.test(location.pathname || ""));
    const evaluateAuthReady = () => {
      const storageKeys = readStorageKeys();
      const loggedIn = (!isLoginPath() && (storageKeys.length > 0 || isDashboardPath()));
      postAuthState("auth-check", {
        loggedIn,
        storageKeys,
        reason: loggedIn ? "path_or_storage_ready" : "awaiting_login",
      });
      if (loggedIn) {
        postAuthState("auth-ready", {
          loggedIn: true,
          storageKeys,
          reason: "path_or_storage_ready",
        });
        return true;
      }
      const elapsedMs = Date.now() - startedAt;
      if (elapsedMs >= MAX_WAIT_MS) {
        postAuthState("assist-timeout", {
          loggedIn: false,
          storageKeys,
          reason: "timeout",
        });
        return true;
      }
      return false;
    };
    let lastHref = location.href;
    post("script-installed");
    window.addEventListener("DOMContentLoaded", () => post("dom-content-loaded"));
    window.addEventListener("load", () => post("window-load"));
    setTimeout(() => {
      evaluateAuthReady();
    }, 1200);
    setInterval(() => {
      if (location.origin !== %q) {
        return;
      }
      evaluateAuthReady();
    }, CHECK_INTERVAL_MS);
    setInterval(() => {
      if (location.origin !== %q) {
        return;
      }
      if (location.href !== lastHref) {
        lastHref = location.href;
        post("href-changed");
      }
    }, 500);
  } catch {}
})();`, origin, origin, origin, origin)
}

func shouldInjectProfileAssistStorage(request desktopProfileAssistOpenRequest, host string, storageValues map[string]string, cookies []desktopProfileAssistCookie) (bool, string) {
	if len(storageValues) == 0 {
		return false, "no_storage_values"
	}

	if disabledReason := getProfileAssistStorageDisableReason(request, host); disabledReason != "" {
		return false, disabledReason
	}

	return true, ""
}

func resolveProfileAssistInitialURL(origin string, request desktopProfileAssistOpenRequest, host string, cookieCount int, storageInjected bool) string {
	if shouldOpenProfileAssistLoginFirst(request, host, cookieCount, storageInjected) {
		return strings.TrimRight(origin, "/") + "/login"
	}
	return origin
}

func getProfileAssistStorageDisableReason(request desktopProfileAssistOpenRequest, host string) string {
	if isAnyrouterProfileAssistRequest(request, host) {
		return "anyrouter_storage_disabled"
	}
	if isElysiverProfileAssistHost(host) {
		return "elysiver_storage_disabled"
	}
	return ""
}

func shouldOpenProfileAssistLoginFirst(request desktopProfileAssistOpenRequest, host string, cookieCount int, storageInjected bool) bool {
	if isAnyrouterProfileAssistRequest(request, host) {
		return true
	}
	if isElysiverProfileAssistHost(host) {
		return true
	}
	_ = cookieCount
	_ = storageInjected
	return false
}

func isAnyrouterProfileAssistRequest(request desktopProfileAssistOpenRequest, host string) bool {
	if strings.EqualFold(strings.TrimSpace(request.SiteType), "anyrouter") {
		return true
	}
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "anyrouter.top" || strings.HasSuffix(host, ".anyrouter.top")
}

func isElysiverProfileAssistHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "elysiver.h-e.top" || strings.HasSuffix(host, ".elysiver.h-e.top")
}

func loadChromeCookiesForHost(host string) ([]desktopProfileAssistCookie, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return nil, fmt.Errorf("host is empty")
	}

	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return nil, fmt.Errorf("LOCALAPPDATA is empty")
	}

	localStatePath := filepath.Join(localAppData, "Google", "Chrome", "User Data", "Local State")
	cookiesPath := filepath.Join(localAppData, "Google", "Chrome", "User Data", "Default", "Network", "Cookies")

	masterKey, err := readChromeMasterKey(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read chrome master key failed: %w", err)
	}

	tmpPath, cleanup, err := copySharedFileToTemp(cookiesPath)
	if err != nil {
		return nil, fmt.Errorf("copy cookies db failed: %w", err)
	}
	defer cleanup()

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, fmt.Errorf("open cookies db failed: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT host_key, name, path, value, encrypted_value, is_secure, is_httponly, samesite
		FROM cookies
		WHERE host_key = ? OR host_key = ? OR host_key LIKE ? OR host_key LIKE ?
		ORDER BY host_key, name
	`, host, "."+host, "%."+host, "%"+host)
	if err != nil {
		return nil, fmt.Errorf("query cookies failed: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	cookies := make([]desktopProfileAssistCookie, 0, 16)
	for rows.Next() {
		var (
			hostKey    string
			name       string
			pathValue  string
			plainValue string
			encrypted  []byte
			isSecure   int64
			isHTTPOnly int64
			sameSite   int64
		)

		if err := rows.Scan(&hostKey, &name, &pathValue, &plainValue, &encrypted, &isSecure, &isHTTPOnly, &sameSite); err != nil {
			continue
		}

		value := plainValue
		if value == "" && len(encrypted) > 0 {
			decrypted, err := decryptChromeCookieValue(masterKey, encrypted)
			if err != nil {
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] cookie decrypt failed %s@%s | %v", name, hostKey, err))
				continue
			}
			value = string(decrypted)
		}
		if strings.TrimSpace(value) == "" {
			continue
		}

		key := hostKey + "|" + name + "|" + pathValue
		if seen[key] {
			continue
		}
		seen[key] = true

		cookies = append(cookies, desktopProfileAssistCookie{
			HostKey:    hostKey,
			Name:       name,
			Value:      value,
			Path:       pathValue,
			IsSecure:   isSecure != 0,
			IsHTTPOnly: isHTTPOnly != 0,
			SameSite:   int32(sameSite),
		})
	}

	return cookies, nil
}

func readChromeMasterKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}

	var state chromeLocalStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	encoded := strings.TrimSpace(state.OSCrypt.EncryptedKey)
	if encoded == "" {
		return nil, fmt.Errorf("os_crypt.encrypted_key missing")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(raw, []byte("DPAPI")) {
		raw = raw[len("DPAPI"):]
	}
	return decryptDPAPI(raw)
}

func decryptChromeCookieValue(masterKey []byte, encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, fmt.Errorf("empty encrypted value")
	}
	if bytes.HasPrefix(encrypted, []byte("v20")) {
		return nil, fmt.Errorf("unsupported chrome app-bound cookie (v20)")
	}
	if bytes.HasPrefix(encrypted, []byte("v10")) || bytes.HasPrefix(encrypted, []byte("v11")) {
		if len(encrypted) < 3+12+16 {
			return nil, fmt.Errorf("encrypted value too short")
		}
		block, err := aes.NewCipher(masterKey)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		nonce := encrypted[3 : 3+12]
		ciphertext := encrypted[3+12:]
		return gcm.Open(nil, nonce, ciphertext, nil)
	}
	return decryptDPAPI(encrypted)
}

func copySharedFileToTemp(src string) (string, func(), error) {
	data, err := readWindowsSharedFile(src)
	if err != nil {
		return "", nil, err
	}

	tmpDir, err := os.MkdirTemp("", "batch-api-check-cookie-assist-*")
	if err != nil {
		return "", nil, err
	}
	dst := filepath.Join(tmpDir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}
	return dst, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func readWindowsSharedFile(path string) ([]byte, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)

	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, fmt.Errorf("wrap file handle failed: %s", path)
	}
	defer file.Close()

	return io.ReadAll(file)
}

func decryptDPAPI(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty dpapi payload")
	}

	var in dpapiDataBlob
	in.cbData = uint32(len(data))
	in.pbData = &data[0]
	var out dpapiDataBlob

	ret, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, callErr
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	buf := unsafe.Slice(out.pbData, out.cbData)
	return append([]byte(nil), buf...), nil
}

func sanitizeProfileAssistSegment(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return "default"
	}
	var builder strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	output := strings.Trim(builder.String(), "_")
	if output == "" {
		return "default"
	}
	return output
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func dedupeStrings(input []string) []string {
	seen := map[string]bool{}
	output := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		output = append(output, item)
	}
	return output
}
