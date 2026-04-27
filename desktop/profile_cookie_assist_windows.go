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
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	TargetURL    string   `json:"targetUrl,omitempty"`
	TargetName   string   `json:"targetName,omitempty"`
	Provider     string   `json:"provider,omitempty"`
	Method       string   `json:"method,omitempty"`
	Status       int      `json:"status,omitempty"`
	Detail       string   `json:"detail,omitempty"`
	Challenge    bool     `json:"challenge,omitempty"`
}

type profileAssistWebResourceRequestedEventArgsView struct {
	Vtbl *profileAssistWebResourceRequestedEventArgsVtbl
}

type profileAssistWebResourceRequestedEventArgsVtbl struct {
	QueryInterface     edge.ComProc
	AddRef             edge.ComProc
	Release            edge.ComProc
	GetRequest         edge.ComProc
	GetResponse        edge.ComProc
	PutResponse        edge.ComProc
	GetDeferral        edge.ComProc
	GetResourceContext edge.ComProc
}

var (
	user32DLL                       = windows.NewLazySystemDLL("user32.dll")
	gdi32DLL                        = windows.NewLazySystemDLL("gdi32.dll")
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
	procBeginPaint                  = user32DLL.NewProc("BeginPaint")
	procEndPaint                    = user32DLL.NewProc("EndPaint")
	procDrawTextW                   = user32DLL.NewProc("DrawTextW")
	procCreateSolidBrush            = gdi32DLL.NewProc("CreateSolidBrush")
	procSetBkMode                   = gdi32DLL.NewProc("SetBkMode")
	procSetTextColor                = gdi32DLL.NewProc("SetTextColor")
	profileAssistWindowClassName, _ = windows.UTF16PtrFromString("BatchApiCheckProfileAssistWindow")
	profileAssistWndProc            = windows.NewCallback(profileAssistWindowProc)
	profileAssistWindowClassOnce    sync.Once
	profileAssistWindowClassErr     error
	profileAssistWindowStateMu      sync.Mutex
	profileAssistWindowStates       = map[uintptr]*profileAssistWindowState{}
	profileAssistWindowHostMu       sync.Mutex
	profileAssistWindowHosts        = map[string]map[uintptr]struct{}{}
	profileAssistBackgroundBrush    windows.Handle
	crypt32DLL                      = syscall.NewLazyDLL("crypt32.dll")
	kernel32DLL                     = syscall.NewLazyDLL("kernel32.dll")
	procCryptUnprotectData          = crypt32DLL.NewProc("CryptUnprotectData")
	procLocalFree                   = kernel32DLL.NewProc("LocalFree")
)

const (
	profileAssistDisabled            = false
	profileAssistDisableInjection    = true
	profileAssistMinimalMode         = true
	profileAssistTraceMinimal        = false
	profileAssistWSOverlappedWindow  = 0x00CF0000
	profileAssistSWShow              = 5
	profileAssistWMDestroy           = 0x0002
	profileAssistWMSize              = 0x0005
	profileAssistWMPaint             = 0x000F
	profileAssistWMClose             = 0x0010
	profileAssistWMQuit              = 0x0012
	profileAssistWMApp               = 0x8000
	profileAssistWMRetryNavigate     = profileAssistWMApp + 1
	profileAssistSystemMetricsCxIcon = 11
	profileAssistSystemMetricsCyIcon = 12
	profileAssistLRLoadFromFile      = 0x00000010
	profileAssistLRDefaultSize       = 0x00000040
	profileAssistImageIcon           = 1
	profileAssistDTCenter            = 0x00000001
	profileAssistDTVCenter           = 0x00000004
	profileAssistDTSingleLine        = 0x00000020
	profileAssistDTWordBreak         = 0x00000010
	profileAssistBkModeTransparent   = 1
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
	chromium     *edge.Chromium
	loadingTitle string
	loadingHint  string
	host         string
	retryURL     string
}

type profileAssistPaintStruct struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     profileAssistRect
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
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

func profileAssistDetectProvider(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	switch {
	case strings.Contains(value, "github.com"), strings.Contains(value, "github"):
		return "github"
	case strings.Contains(value, "linux.do"), strings.Contains(value, "linuxdo"):
		return "linux.do"
	case strings.Contains(value, "google"), strings.Contains(value, "accounts.google.com"):
		return "google"
	case strings.Contains(value, "cloudflare"), strings.Contains(value, "turnstile"), strings.Contains(value, "captcha"), strings.Contains(value, "challenge"):
		return "challenge"
	default:
		return ""
	}
}

func profileAssistShouldTraceURL(siteURL string, rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}

	siteHost := ""
	if parsedSite, err := url.Parse(strings.TrimSpace(siteURL)); err == nil {
		siteHost = strings.ToLower(strings.TrimSpace(parsedSite.Hostname()))
	}

	parsed, err := url.Parse(rawURL)
	if err == nil {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if host == "" && parsed.Scheme == "" && parsed.Path != "" {
			return true
		}
		if siteHost != "" && (host == siteHost || strings.HasSuffix(host, "."+siteHost)) {
			return true
		}
		switch host {
		case "github.com", "www.github.com", "linux.do", "www.linux.do", "challenges.cloudflare.com":
			return true
		}
	}

	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, "github.com") ||
		strings.Contains(lower, "linux.do") ||
		strings.Contains(lower, "cloudflare") ||
		strings.Contains(lower, "turnstile") ||
		strings.Contains(lower, "challenge") ||
		strings.Contains(lower, "/login") ||
		strings.Contains(lower, "/signin") ||
		strings.Contains(lower, "/oauth") ||
		strings.Contains(lower, "/auth")
}

func profileAssistResourceContextName(context edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT) string {
	switch context {
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT:
		return "document"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_SCRIPT:
		return "script"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_XML_HTTP_REQUEST:
		return "xhr"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_FETCH:
		return "fetch"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_STYLESHEET:
		return "stylesheet"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_IMAGE:
		return "image"
	case edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_OTHER:
		return "other"
	default:
		return fmt.Sprintf("ctx-%d", uint32(context))
	}
}

func profileAssistGetWebResourceContext(args *edge.ICoreWebView2WebResourceRequestedEventArgs) (edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT, error) {
	if args == nil {
		return edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_OTHER, fmt.Errorf("nil args")
	}
	view := (*profileAssistWebResourceRequestedEventArgsView)(unsafe.Pointer(args))
	if view == nil || view.Vtbl == nil {
		return edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_OTHER, fmt.Errorf("missing args vtbl")
	}

	var context edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT
	hr, _, _ := view.Vtbl.GetResourceContext.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&context)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_OTHER, windows.Errno(hr)
	}
	return context, nil
}

func profileAssistGetHeaderValue(headers *edge.ICoreWebView2HttpRequestHeaders, name string) string {
	if headers == nil || strings.TrimSpace(name) == "" {
		return ""
	}
	value, err := headers.GetHeader(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
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
	if profileAssistDisabled {
		return nil, fmt.Errorf("profile assist disabled")
	}
	hostKey := ""
	if parsed, err := url.Parse(strings.TrimSpace(request.SiteURL)); err == nil {
		hostKey = strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	}
	if hostKey != "" {
		if reused, ok := focusProfileAssistWindowByHost(hostKey); ok && reused {
			return &desktopProfileAssistWindowResult{
				SiteName:            strings.TrimSpace(request.SiteName),
				SiteURL:             strings.TrimSpace(request.SiteURL),
				InjectedCookies:     0,
				InjectedCookieNames: []string{},
				StorageFields:       []string{},
				Message:             "Profile assist window already open, focused existing window",
			}, nil
		}
	}
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
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] start | site=%s name=%s type=%s", siteURL, strings.TrimSpace(request.SiteName), strings.TrimSpace(request.SiteType)))

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

	disableInjection := profileAssistDisableInjection || profileAssistMinimalMode
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] mode | site=%s minimal=%t disableInjection=%t", siteURL, profileAssistMinimalMode, disableInjection))
	cookies := []desktopProfileAssistCookie{}
	storageValues := map[string]string{}
	storageFields := []string{}
	if !disableInjection {
		var err error
		cookies, err = loadChromeCookiesForHost(host)
		if err != nil {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] %s cookies load failed | %v", siteURL, err))
		}

		if snapshot, _, snapshotErr := loadChromeProfileAuthSnapshot(); snapshotErr == nil && snapshot != nil {
			if rawValues := snapshot.entries[origin]; len(rawValues) > 0 {
				storageValues = cloneStringMap(rawValues)
				for key := range rawValues {
					storageFields = append(storageFields, key)
				}
			}
		}
	}

	shouldInjectStorage, storageSkipReason := shouldInjectProfileAssistStorage(request, host, storageValues, cookies)
	if disableInjection {
		shouldInjectStorage = false
		storageSkipReason = "disabled"
	}
	if !shouldInjectStorage && storageSkipReason != "" {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] storage injection skipped %s | reason=%s", siteURL, storageSkipReason))
		storageValues = map[string]string{}
		storageFields = []string{}
	}
	initialNavigateURL := resolveProfileAssistInitialURL(origin, request, host, len(cookies), shouldInjectStorage)
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] resolve | site=%s host=%s origin=%s initial=%s cookies=%d storage=%d inject=%t", siteURL, host, origin, initialNavigateURL, len(cookies), len(storageFields), shouldInjectStorage))

	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] com init | site=%s", siteURL))
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
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] window created | site=%s hwnd=0x%X", siteURL, hwnd))

	chromium := edge.NewChromium()
	loadingHint := fmt.Sprintf("Preparing %s login environment...", siteName)
	if disableInjection {
		loadingHint = fmt.Sprintf("Opening %s login page...", siteName)
	}
	state := &profileAssistWindowState{
		loadingTitle: "Profile Assist",
		loadingHint:  loadingHint,
		host:         host,
	}
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
	var navigationCount int32
	var documentRequestCount int32
	resourceRequestCounts := map[string]int{}
	traceEvents := !profileAssistMinimalMode || profileAssistTraceMinimal
	chromium.DataPath = buildProfileAssistSessionDataPath(host)
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] data path %s | %s", siteURL, chromium.DataPath))
	if !profileAssistMinimalMode {
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
	}
	if traceEvents {
		chromium.SetErrorCallback(func(webErr error) {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] webview error %s | %v", siteURL, webErr))
		})
		chromium.WebResourceRequestedCallback = func(request *edge.ICoreWebView2WebResourceRequest, args *edge.ICoreWebView2WebResourceRequestedEventArgs) {
			if request == nil {
				return
			}

			method, _ := request.GetMethod()
			uri, _ := request.GetUri()
			context, contextErr := profileAssistGetWebResourceContext(args)
			contextName := profileAssistResourceContextName(context)
			resourceRequestCounts[contextName]++
			if context == edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT {
				documentRequestCount++
			}

			if contextErr != nil {
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] resource context read failed %s | %v", siteURL, contextErr))
			}

			if !profileAssistShouldTraceURL(siteURL, uri) && context != edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT {
				return
			}

			headers, _ := request.GetHeaders()
			hasCookieHeader := false
			hasAuthorizationHeader := false
			originHeader := ""
			refererHeader := ""
			if headers != nil {
				hasCookieHeader = profileAssistGetHeaderValue(headers, "Cookie") != ""
				hasAuthorizationHeader = profileAssistGetHeaderValue(headers, "Authorization") != ""
				originHeader = truncateProfileAssistLogText(profileAssistGetHeaderValue(headers, "Origin"), 180)
				refererHeader = truncateProfileAssistLogText(profileAssistGetHeaderValue(headers, "Referer"), 180)
				_ = headers.Release()
			}

			appendLine(profileAssistLogPath(), fmt.Sprintf(
				"[ASSIST] resource request %s | ctx=%s count=%d docCount=%d auth=%t cookie=%t origin=%s referer=%s | %s %s",
				siteURL,
				contextName,
				resourceRequestCounts[contextName],
				documentRequestCount,
				hasAuthorizationHeader,
				hasCookieHeader,
				originHeader,
				refererHeader,
				strings.TrimSpace(method),
				truncateProfileAssistLogText(uri, 600),
			))
		}
		chromium.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
			_ = args
			count := atomic.AddInt32(&navigationCount, 1)
			currentURL := ""
			if sender != nil {
				currentURL, _ = sender.GetSource()
			}
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigation completed %s | count=%d | url=%s", siteURL, count, currentURL))
		}
	}

	registerProfileAssistWindowState(hwnd, state)
	registerProfileAssistHostWindow(host, hwnd)
	defer unregisterProfileAssistWindowState(hwnd)
	defer unregisterProfileAssistHostWindow(host, hwnd)

	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] embed start %s", siteURL))
	if ok := chromium.Embed(hwnd); !ok {
		_ = destroyProfileAssistWindow(hwnd)
		resultCh <- desktopProfileAssistWindowOpenResult{err: fmt.Errorf("webview2 embed failed")}
		return
	}
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] embed ready %s", siteURL))
	if controller := chromium.GetController(); controller == nil {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] controller nil after embed %s", siteURL))
	}
	state.chromium = chromium
	chromium.Resize()
	chromium.SetBackgroundColour(255, 255, 255, 255)
	if err := chromium.Show(); err != nil {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] show failed %s | %v", siteURL, err))
	}
	if traceEvents {
		chromium.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT)
		chromium.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_SCRIPT)
		chromium.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_XML_HTTP_REQUEST)
		chromium.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_FETCH)
		chromium.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_OTHER)
		logProfileAssistWebViewSettings(siteURL, chromium)
	}
	if !profileAssistMinimalMode {

		if !disableInjection {
			if script := buildProfileAssistStorageBootstrapScript(origin, storageValues); script != "" {
				chromium.Init(script)
			}
		}
		if traceScript := buildProfileAssistTraceScript(origin); traceScript != "" {
			chromium.Init(traceScript)
		}
	}

	injectedCookieNames := []string{}
	if !disableInjection && !profileAssistMinimalMode {
		var err error
		injectedCookieNames, err = injectChromeCookiesIntoWebView(chromium, cookies)
		if err != nil {
			appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] inject cookies failed %s | %v", siteURL, err))
		}
	} else if disableInjection && !profileAssistMinimalMode {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] injection disabled %s", siteURL))
	}

	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] open window %s | cookies=%d | storageFields=%d", siteURL, len(injectedCookieNames), len(storageFields)))
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigate start %s | %s", siteURL, initialNavigateURL))
	chromium.Navigate(initialNavigateURL)
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigate issued %s", siteURL))

	_, _, _ = procShowWindow.Call(hwnd, profileAssistSWShow)
	_, _, _ = procUpdateWindow.Call(hwnd)
	_, _, _ = procSetFocus.Call(hwnd)
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] window shown %s", siteURL))
	if traceEvents {
		go func(target string) {
			time.Sleep(6 * time.Second)
			if atomic.LoadInt32(&navigationCount) == 0 {
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigate stall %s | after=6s | target=%s | documentRequests=%d", siteURL, target, documentRequestCount))
				scheduleProfileAssistNavigateRetry(hwnd, target)
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigate retry scheduled %s | target=%s", siteURL, target))
			}
			time.Sleep(6 * time.Second)
			if atomic.LoadInt32(&navigationCount) == 0 {
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] navigate stall %s | after=12s | target=%s | documentRequests=%d", siteURL, target, documentRequestCount))
			}
		}(initialNavigateURL)
	}

	result := &desktopProfileAssistWindowResult{
		SiteName:            siteName,
		SiteURL:             initialNavigateURL,
		InjectedCookies:     len(injectedCookieNames),
		InjectedCookieNames: injectedCookieNames,
		StorageFields:       storageFields,
		Message:             fmt.Sprintf("Opened WebView2 (inject disabled=%t), cookies=%d storageFields=%d", disableInjection, len(injectedCookieNames), len(storageFields)),
	}
	resultCh <- desktopProfileAssistWindowOpenResult{result: result}
	runProfileAssistMessageLoop(hwnd, chromium)
}

func buildProfileAssistLoadingDocument(siteName string, targetURL string) string {
	siteLabel := strings.TrimSpace(siteName)
	if siteLabel == "" {
		siteLabel = "Profile Assist"
	}

	targetLiteral, err := json.Marshal(strings.TrimSpace(targetURL))
	if err != nil {
		targetLiteral = []byte(`""`)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  <style>
    :root {
      color-scheme: dark;
      --bg0: #09111b;
      --bg1: #0f1b29;
      --bg2: rgba(95, 160, 121, 0.22);
      --line: rgba(183, 255, 212, 0.16);
      --text: rgba(238, 246, 242, 0.96);
      --muted: rgba(194, 210, 203, 0.78);
      --accent: #8cf0b6;
      --accent-soft: rgba(140, 240, 182, 0.2);
    }
    * { box-sizing: border-box; }
    html, body {
      width: 100%%;
      height: 100%%;
      margin: 0;
      overflow: hidden;
      background:
        radial-gradient(circle at 18%% 20%%, rgba(122, 201, 154, 0.22), transparent 30%%),
        radial-gradient(circle at 82%% 18%%, rgba(84, 146, 112, 0.18), transparent 28%%),
        linear-gradient(160deg, var(--bg0) 0%%, var(--bg1) 100%%);
      color: var(--text);
      font-family: "Segoe UI", "Microsoft YaHei UI", sans-serif;
      user-select: none;
    }
    body {
      display: grid;
      place-items: center;
      padding: 28px;
    }
    .panel {
      width: min(560px, 100%%);
      border: 1px solid var(--line);
      border-radius: 24px;
      padding: 28px 30px 24px;
      background:
        linear-gradient(180deg, rgba(15, 25, 38, 0.96), rgba(10, 17, 27, 0.94)),
        var(--accent-soft);
      box-shadow: 0 22px 72px rgba(0, 0, 0, 0.32);
      backdrop-filter: blur(8px);
    }
    .eyebrow {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      font-size: 12px;
      letter-spacing: 0.22em;
      text-transform: uppercase;
      color: var(--muted);
    }
    .dot {
      width: 8px;
      height: 8px;
      border-radius: 999px;
      background: var(--accent);
      box-shadow: 0 0 0 6px rgba(140, 240, 182, 0.14);
      animation: pulse 1.3s ease-in-out infinite;
    }
    .title {
      margin: 16px 0 8px;
      font-size: 28px;
      line-height: 1.18;
      font-weight: 700;
    }
    .desc {
      margin: 0;
      font-size: 14px;
      line-height: 1.7;
      color: var(--muted);
      word-break: break-all;
    }
    .url {
      margin-top: 18px;
      padding: 12px 14px;
      border-radius: 14px;
      border: 1px solid rgba(183, 255, 212, 0.12);
      background: rgba(8, 14, 22, 0.5);
      color: rgba(225, 239, 233, 0.92);
      font-size: 12px;
      line-height: 1.6;
      word-break: break-all;
    }
    .bar {
      margin-top: 22px;
      height: 6px;
      border-radius: 999px;
      background: rgba(255, 255, 255, 0.08);
      overflow: hidden;
      position: relative;
    }
    .bar::before {
      content: "";
      position: absolute;
      inset: 0;
      width: 38%%;
      border-radius: inherit;
      background: linear-gradient(90deg, rgba(140, 240, 182, 0.22), rgba(140, 240, 182, 0.94));
      animation: loading 1.05s ease-in-out infinite alternate;
    }
    .foot {
      margin-top: 16px;
      font-size: 12px;
      color: rgba(190, 206, 198, 0.72);
    }
    @keyframes loading {
      from { transform: translateX(-8%%); }
      to { transform: translateX(168%%); }
    }
    @keyframes pulse {
      0%%, 100%% { transform: scale(1); opacity: 1; }
      50%% { transform: scale(0.82); opacity: 0.72; }
    }
  </style>
</head>
<body>
  <section class="panel">
    <div class="eyebrow"><span class="dot"></span><span>Profile Assist</span></div>
    <h1 class="title">%s</h1>
    <p class="desc">正在准备登录环境、注入本地已捕获的 Cookie / Storage，并立即跳转到目标站点。</p>
    <div class="url">%s</div>
    <div class="bar"></div>
    <div class="foot">如果目标站点响应较慢，这个过渡页会先顶住首屏，避免白屏闪烁。</div>
  </section>
  <script>
    (() => {
      const target = %s;
      const jump = () => {
        if (!target) return;
        window.location.replace(target);
      };
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setTimeout(jump, 72);
        });
      });
    })();
  </script>
</body>
</html>`, html.EscapeString(siteLabel), html.EscapeString(siteLabel), html.EscapeString(strings.TrimSpace(targetURL)), string(targetLiteral))
}

func runProfileAssistMessageLoop(hwnd uintptr, chromium *edge.Chromium) {
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] message loop start | hwnd=0x%X", hwnd))
	var msg profileAssistMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), hwnd, 0, 0)
		if ret == 0 || ret == ^uintptr(0) || msg.Message == profileAssistWMQuit {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] message loop end | hwnd=0x%X", hwnd))
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
		if profileAssistBackgroundBrush == 0 {
			brush, _, _ := procCreateSolidBrush.Call(0x00FFFFFF)
			profileAssistBackgroundBrush = windows.Handle(brush)
		}

		wc := profileAssistWndClassEx{
			CbSize:        uint32(unsafe.Sizeof(profileAssistWndClassEx{})),
			HInstance:     hinstance,
			LpszClassName: profileAssistWindowClassName,
			HIcon:         windows.Handle(icon),
			HIconSm:       windows.Handle(icon),
			LpfnWndProc:   profileAssistWndProc,
			HbrBackground: profileAssistBackgroundBrush,
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
	case profileAssistWMPaint:
		if chromium := getProfileAssistChromium(hwnd); chromium == nil {
			break
		}
	case profileAssistWMClose:
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] wm_close | hwnd=0x%X", hwnd))
		_ = destroyProfileAssistWindow(hwnd)
		return 0
	case profileAssistWMRetryNavigate:
		state := getProfileAssistWindowState(hwnd)
		if state != nil && state.chromium != nil {
			profileAssistWindowStateMu.Lock()
			target := strings.TrimSpace(state.retryURL)
			state.retryURL = ""
			profileAssistWindowStateMu.Unlock()
			if target != "" {
				if controller := state.chromium.GetController(); controller != nil {
					_ = controller.PutIsVisible(true)
				}
				state.chromium.Navigate(target)
				appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] retry navigate | hwnd=0x%X target=%s", hwnd, target))
			}
		}
		return 0
	case profileAssistWMDestroy:
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] wm_destroy | hwnd=0x%X", hwnd))
		if chromium := getProfileAssistChromium(hwnd); chromium != nil {
			chromium.ShuttingDown()
		}
		if state := getProfileAssistWindowState(hwnd); state != nil {
			unregisterProfileAssistHostWindow(state.host, hwnd)
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
}

func registerProfileAssistWindowState(hwnd uintptr, state *profileAssistWindowState) {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	profileAssistWindowStates[hwnd] = state
}

func unregisterProfileAssistWindowState(hwnd uintptr) {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	delete(profileAssistWindowStates, hwnd)
}

func registerProfileAssistHostWindow(host string, hwnd uintptr) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || hwnd == 0 {
		return
	}
	profileAssistWindowHostMu.Lock()
	defer profileAssistWindowHostMu.Unlock()
	hostMap := profileAssistWindowHosts[host]
	if hostMap == nil {
		hostMap = map[uintptr]struct{}{}
		profileAssistWindowHosts[host] = hostMap
	}
	hostMap[hwnd] = struct{}{}
}

func unregisterProfileAssistHostWindow(host string, hwnd uintptr) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || hwnd == 0 {
		return
	}
	profileAssistWindowHostMu.Lock()
	defer profileAssistWindowHostMu.Unlock()
	hostMap := profileAssistWindowHosts[host]
	if hostMap == nil {
		return
	}
	delete(hostMap, hwnd)
	if len(hostMap) == 0 {
		delete(profileAssistWindowHosts, host)
	}
}

func scheduleProfileAssistNavigateRetry(hwnd uintptr, target string) {
	if hwnd == 0 {
		return
	}
	state := getProfileAssistWindowState(hwnd)
	if state == nil {
		return
	}
	profileAssistWindowStateMu.Lock()
	state.retryURL = strings.TrimSpace(target)
	profileAssistWindowStateMu.Unlock()
	_, _, _ = procPostMessageW.Call(hwnd, profileAssistWMRetryNavigate, 0, 0)
}

func focusProfileAssistWindowByHost(host string) (bool, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false, false
	}
	profileAssistWindowHostMu.Lock()
	var hwnd uintptr
	if hostMap := profileAssistWindowHosts[host]; len(hostMap) > 0 {
		for existing := range hostMap {
			hwnd = existing
			break
		}
	}
	profileAssistWindowHostMu.Unlock()
	if hwnd == 0 {
		return false, false
	}
	_, _, _ = procShowWindow.Call(hwnd, profileAssistSWShow)
	_, _, _ = procUpdateWindow.Call(hwnd)
	_, _, _ = procSetFocus.Call(hwnd)
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] focus existing window | host=%s hwnd=0x%X", host, hwnd))
	return true, true
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

func renderProfileAssistPlaceholder(hwnd uintptr) {
	state := getProfileAssistWindowState(hwnd)
	if state == nil {
		return
	}

	var ps profileAssistPaintStruct
	ret, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if ret == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	_, _, _ = procSetBkMode.Call(ret, profileAssistBkModeTransparent)
	_, _, _ = procSetTextColor.Call(ret, 0x00D9F3E6)

	text := strings.TrimSpace(state.loadingTitle)
	if text == "" {
		text = "Profile Assist"
	}
	if strings.TrimSpace(state.loadingHint) != "" {
		text += "\n" + strings.TrimSpace(state.loadingHint)
	}
	textPtr, _ := windows.UTF16PtrFromString(text)
	rect := ps.RcPaint
	rect.Left += 48
	rect.Right -= 48
	rect.Top += 48
	rect.Bottom -= 48
	_, _, _ = procDrawTextW.Call(
		ret,
		uintptr(unsafe.Pointer(textPtr)),
		^uintptr(0),
		uintptr(unsafe.Pointer(&rect)),
		profileAssistDTCenter|profileAssistDTVCenter|profileAssistDTWordBreak,
	)
}

func getProfileAssistWindowState(hwnd uintptr) *profileAssistWindowState {
	profileAssistWindowStateMu.Lock()
	defer profileAssistWindowStateMu.Unlock()
	return profileAssistWindowStates[hwnd]
}

func destroyProfileAssistWindow(hwnd uintptr) error {
	ret, _, err := procDestroyWindow.Call(hwnd)
	if ret == 0 {
		return err
	}
	return nil
}

func closeProfileAssistWindowsByHosts(hosts []string) int {
	if len(hosts) == 0 {
		return 0
	}
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] close request | hosts=%v", hosts))
	uniqueHosts := map[string]struct{}{}
	for _, raw := range hosts {
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		host := ""
		if strings.HasPrefix(strings.ToLower(text), "http://") || strings.HasPrefix(strings.ToLower(text), "https://") {
			if parsed, err := url.Parse(text); err == nil {
				host = parsed.Hostname()
			}
		} else if strings.Contains(text, "/") {
			if parsed, err := url.Parse("https://" + text); err == nil {
				host = parsed.Hostname()
			}
		} else {
			host = text
		}
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		uniqueHosts[host] = struct{}{}
	}
	if len(uniqueHosts) == 0 {
		return 0
	}

	hwnds := make([]uintptr, 0, len(uniqueHosts))
	profileAssistWindowHostMu.Lock()
	for host := range uniqueHosts {
		for hwnd := range profileAssistWindowHosts[host] {
			hwnds = append(hwnds, hwnd)
		}
	}
	profileAssistWindowHostMu.Unlock()

	closed := 0
	for _, hwnd := range hwnds {
		ret, _, _ := procPostMessageW.Call(hwnd, profileAssistWMClose, 0, 0)
		if ret != 0 {
			closed++
		}
	}
	appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] close issued | count=%d", closed))
	return closed
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
    const CHALLENGE_PATTERNS = [/just a moment/i, /请稍候/i, /turnstile/i, /captcha/i, /challenge/i, /verify you are human/i];
    const PROVIDER_PATTERNS = [
      { provider: "github", pattern: /github/i },
      { provider: "linux.do", pattern: /linux\.do|linuxdo/i },
      { provider: "google", pattern: /google/i }
    ];
    let startedAt = Number(sessionStorage.getItem(START_KEY) || "0");
    if (!Number.isFinite(startedAt) || startedAt <= 0) {
      startedAt = Date.now();
      sessionStorage.setItem(START_KEY, String(startedAt));
    }
    const detectProvider = (input) => {
      const text = String(input || "").toLowerCase();
      for (const item of PROVIDER_PATTERNS) {
        if (item.pattern.test(text)) {
          return item.provider;
        }
      }
      return "";
    };
    const shouldTraceURL = (value) => {
      const text = String(value || "").trim();
      if (!text) return false;
      if (text.startsWith("/") || text.startsWith("#")) return true;
      return /github\.com|linux\.do|cloudflare|turnstile|captcha|challenge|oauth|auth|signin|login/i.test(text);
    };
    const shorten = (value, limit = 160) => {
      const text = String(value || "").replace(/\s+/g, " ").trim();
      if (!text) return "";
      return text.length > limit ? text.slice(0, limit) + "..." : text;
    };
    const readCookieNames = () => {
      try {
        return String(document.cookie || "")
          .split(";")
          .map(item => item.split("=")[0]?.trim())
          .filter(Boolean)
          .slice(0, 12);
      } catch {
        return [];
      }
    };
    const detectChallenge = () => {
      const title = String(document.title || "");
      const bodyText = shorten(document.body?.innerText || "", 320);
      return CHALLENGE_PATTERNS.some(pattern => pattern.test(title) || pattern.test(bodyText) || pattern.test(location.href));
    };
    const post = (type, extra = {}) => {
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
            challenge: detectChallenge(),
            provider: detectProvider([location.href, document.title, document.body?.innerText || ""].join(" ")),
            ...extra,
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
            challenge: detectChallenge(),
            provider: detectProvider([location.href, document.title, document.body?.innerText || ""].join(" ")),
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
      const cookieNames = readCookieNames();
      const challenge = detectChallenge();
      const loggedIn = (!isLoginPath() && (storageKeys.length > 0 || isDashboardPath()));
      postAuthState("auth-check", {
        loggedIn,
        storageKeys,
        detail: shorten("cookies=" + cookieNames.join(",") + " title=" + (document.title || "") + " body=" + (document.body?.innerText || ""), 260),
        reason: loggedIn ? "path_or_storage_ready" : "awaiting_login",
        challenge,
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
          detail: shorten("cookies=" + cookieNames.join(",") + " title=" + (document.title || ""), 220),
          reason: "timeout",
          challenge,
        });
        return true;
      }
      return false;
    };
    let lastHref = location.href;
    post("script-installed", {
      detail: shorten("ua=" + (navigator.userAgent || "") + " webdriver=" + String(navigator.webdriver) + " cookieEnabled=" + String(navigator.cookieEnabled) + " hasWebView=" + String(!!window.chrome?.webview), 240),
    });
    window.addEventListener("DOMContentLoaded", () => post("dom-content-loaded"));
    window.addEventListener("load", () => post("window-load"));
    window.addEventListener("error", (event) => {
      post("page-error", {
        detail: shorten(String(event.message || "") + " @ " + String(event.filename || "") + ":" + String(event.lineno || 0) + ":" + String(event.colno || 0), 220),
      });
    }, true);
    window.addEventListener("unhandledrejection", (event) => {
      const reason = event?.reason;
      post("unhandled-rejection", {
        detail: shorten(typeof reason === "string" ? reason : (reason?.message || JSON.stringify(reason || {})), 220),
      });
    });
    const originalOpen = window.open;
    if (typeof originalOpen === "function") {
      window.open = function (...args) {
        const targetUrl = String(args?.[0] || "");
        const targetName = String(args?.[1] || "");
        post("window-open", {
          targetUrl,
          targetName,
          provider: detectProvider(targetUrl + " " + targetName),
          detail: shorten(String(args?.[2] || ""), 120),
        });
        return originalOpen.apply(this, args);
      };
    }
    const originalFetch = window.fetch?.bind(window);
    if (originalFetch) {
      window.fetch = async (...args) => {
        const input = args?.[0];
        const requestUrl = typeof input === "string" ? input : (input?.url || "");
        if (shouldTraceURL(requestUrl)) {
          post("fetch-start", {
            method: String(args?.[1]?.method || input?.method || "GET").toUpperCase(),
            targetUrl: requestUrl,
            provider: detectProvider(requestUrl),
          });
        }
        try {
          const response = await originalFetch(...args);
          if (shouldTraceURL(requestUrl) || shouldTraceURL(response?.url || "")) {
            post("fetch-end", {
              method: String(args?.[1]?.method || input?.method || "GET").toUpperCase(),
              targetUrl: response?.url || requestUrl,
              status: Number(response?.status || 0),
              provider: detectProvider(requestUrl + " " + String(response?.url || "")),
              detail: shorten("ok=" + String(!!response?.ok) + " redirected=" + String(!!response?.redirected) + " type=" + String(response?.type || ""), 140),
            });
          }
          return response;
        } catch (error) {
          if (shouldTraceURL(requestUrl)) {
            post("fetch-error", {
              method: String(args?.[1]?.method || input?.method || "GET").toUpperCase(),
              targetUrl: requestUrl,
              provider: detectProvider(requestUrl),
              detail: shorten(error?.message || String(error || ""), 180),
            });
          }
          throw error;
        }
      };
    }
    if (window.XMLHttpRequest?.prototype) {
      const xhrOpen = window.XMLHttpRequest.prototype.open;
      const xhrSend = window.XMLHttpRequest.prototype.send;
      window.XMLHttpRequest.prototype.open = function (method, requestUrl, ...rest) {
        this.__batchApiCheckTrace = {
          method: String(method || "GET").toUpperCase(),
          url: String(requestUrl || ""),
        };
        return xhrOpen.call(this, method, requestUrl, ...rest);
      };
      window.XMLHttpRequest.prototype.send = function (...args) {
        const trace = this.__batchApiCheckTrace || {};
        const requestUrl = String(trace.url || "");
        if (shouldTraceURL(requestUrl)) {
          post("xhr-start", {
            method: String(trace.method || "GET"),
            targetUrl: requestUrl,
            provider: detectProvider(requestUrl),
          });
          this.addEventListener("loadend", () => {
            post("xhr-end", {
              method: String(trace.method || "GET"),
              targetUrl: requestUrl,
              status: Number(this.status || 0),
              provider: detectProvider(requestUrl),
              detail: shorten("readyState=" + String(this.readyState || "") + " responseURL=" + String(this.responseURL || ""), 160),
            });
          }, { once: true });
          this.addEventListener("error", () => {
            post("xhr-error", {
              method: String(trace.method || "GET"),
              targetUrl: requestUrl,
              provider: detectProvider(requestUrl),
              detail: "network-error",
            });
          }, { once: true });
        }
        return xhrSend.apply(this, args);
      };
    }
    document.addEventListener("click", (event) => {
      const element = event.target?.closest?.("a,button,[role='button'],input[type='button'],input[type='submit']");
      if (!element) return;
      const text = shorten(element.innerText || element.textContent || element.value || element.getAttribute?.("aria-label") || "", 60);
      const href = String(element.getAttribute?.("href") || "");
      const marker = [text, href, element.className || "", element.id || ""].join(" ");
      if (!shouldTraceURL(href) && !detectProvider(marker) && !/login|signin|oauth|授权|登录/i.test(marker)) {
        return;
      }
      post("ui-click", {
        targetUrl: href,
        targetName: element.tagName || "",
        provider: detectProvider(marker),
        detail: shorten(marker, 200),
      });
    }, true);
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
        post("href-changed", {
          targetUrl: location.href,
        });
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
	_ = request
	_ = host
	return ""
}

func shouldOpenProfileAssistLoginFirst(request desktopProfileAssistOpenRequest, host string, cookieCount int, storageInjected bool) bool {
	_ = request
	_ = host
	return cookieCount <= 0 && !storageInjected
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

func buildProfileAssistSessionDataPath(host string) string {
	hostSegment := sanitizeProfileAssistSegment(host)
	if hostSegment == "" {
		hostSegment = "unknown_host"
	}

	hostRoot := filepath.Join(resolveRuntimeRootDir(), "webview2-assist", hostSegment)
	_ = os.MkdirAll(hostRoot, 0o755)
	if profileAssistMinimalMode {
		cleanupOldProfileAssistSessions(hostRoot, 3)
		return filepath.Join(hostRoot, fmt.Sprintf("session-%d", time.Now().UnixMilli()))
	}
	return filepath.Join(hostRoot, "profile-default")
}

func cleanupOldProfileAssistSessions(hostRoot string, keep int) {
	if keep <= 0 {
		keep = 4
	}
	entries, err := os.ReadDir(hostRoot)
	if err != nil {
		return
	}

	type sessionDir struct {
		name    string
		modTime time.Time
	}
	dirs := make([]sessionDir, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		dirs = append(dirs, sessionDir{name: entry.Name(), modTime: info.ModTime()})
	}

	if len(dirs) <= keep {
		return
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})

	for _, entry := range dirs[keep:] {
		_ = os.RemoveAll(filepath.Join(hostRoot, entry.name))
	}
}

func logProfileAssistWebViewSettings(siteURL string, chromium *edge.Chromium) {
	if chromium == nil {
		return
	}

	settings, err := chromium.GetSettings()
	if err != nil || settings == nil {
		appendLine(profileAssistLogPath(), fmt.Sprintf("[ASSIST] settings unavailable %s | %v", siteURL, err))
		return
	}
	defer settings.Release()

	isScriptEnabled, _ := settings.GetIsScriptEnabled()
	isWebMessageEnabled, _ := settings.GetIsWebMessageEnabled()
	areDialogsEnabled, _ := settings.GetAreDefaultScriptDialogsEnabled()
	isStatusBarEnabled, _ := settings.GetIsStatusBarEnabled()
	areDevToolsEnabled, _ := settings.GetAreDevToolsEnabled()
	areContextMenusEnabled, _ := settings.GetAreDefaultContextMenusEnabled()
	areHostObjectsAllowed, _ := settings.GetAreHostObjectsAllowed()
	isZoomEnabled, _ := settings.GetIsZoomControlEnabled()
	isBuiltInErrorPageEnabled, _ := settings.GetIsBuiltInErrorPageEnabled()
	userAgent, _ := settings.GetUserAgent()
	areBrowserAcceleratorKeysEnabled, _ := settings.GetAreBrowserAcceleratorKeysEnabled()
	isPinchZoomEnabled, _ := settings.GetIsPinchZoomEnabled()
	isSwipeNavigationEnabled, _ := settings.GetIsSwipeNavigationEnabled()

	appendLine(profileAssistLogPath(), fmt.Sprintf(
		"[ASSIST] settings %s | script=%t webMessage=%t dialogs=%t statusBar=%t devTools=%t contextMenus=%t hostObjects=%t zoom=%t builtInError=%t accelKeys=%t pinchZoom=%t swipeNav=%t ua=%s",
		siteURL,
		isScriptEnabled,
		isWebMessageEnabled,
		areDialogsEnabled,
		isStatusBarEnabled,
		areDevToolsEnabled,
		areContextMenusEnabled,
		areHostObjectsAllowed,
		isZoomEnabled,
		isBuiltInErrorPageEnabled,
		areBrowserAcceleratorKeysEnabled,
		isPinchZoomEnabled,
		isSwipeNavigationEnabled,
		truncateProfileAssistLogText(userAgent, 260),
	))
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
