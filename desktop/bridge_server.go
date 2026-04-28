package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed plugin-bridge-js/bridge.user.js
var embeddedBridgeUserScript string

const (
	bridgeServerHost               = "127.0.0.1"
	bridgeServerPort               = 8888
	bridgeServerVersion            = "0.2.11"
	bridgeScriptManagerExtensionID = "gcalenpjmijncebpfijmoaglllgpjagf"
	bridgeScriptManagerInstallURL  = "https://chrome.google.com/webstore/detail/tampermonkey/gcalenpjmijncebpfijmoaglllgpjagf"
)

type bridgeScriptManagerInstallStatus struct {
	Installed    bool     `json:"installed"`
	Source       string   `json:"source"`
	Browser      string   `json:"browser,omitempty"`
	Profile      string   `json:"profile,omitempty"`
	Version      string   `json:"version,omitempty"`
	Path         string   `json:"path,omitempty"`
	ScannedRoots []string `json:"scannedRoots,omitempty"`
}

type bridgeBrowserUserDataRoot struct {
	Browser string
	Root    string
}

type BridgeImportRecord struct {
	ID            string         `json:"id"`
	ReceivedAt    string         `json:"receivedAt"`
	RemoteAddr    string         `json:"remoteAddr"`
	Type          string         `json:"type"`
	SourceURL     string         `json:"sourceUrl"`
	SourceOrigin  string         `json:"sourceOrigin"`
	Title         string         `json:"title"`
	UserAgent     string         `json:"userAgent"`
	SiteType      string         `json:"siteType"`
	ResolvedUser  string         `json:"resolvedUser"`
	TokenPreview  string         `json:"tokenPreview"`
	TokenCount    int            `json:"tokenCount"`
	TokenEndpoint string         `json:"tokenEndpoint"`
	Ready         bool           `json:"ready"`
	ReadyReason   string         `json:"readyReason"`
	Payload       map[string]any `json:"payload"`
}

type BridgeImportSnapshot struct {
	Records        []BridgeImportRecord `json:"records"`
	TotalCount     int                  `json:"totalCount"`
	ReadyCount     int                  `json:"readyCount"`
	LastReceivedAt string               `json:"lastReceivedAt"`
	LastStoredAt   string               `json:"lastStoredAt"`
	LogPath        string               `json:"logPath"`
	ServerURL      string               `json:"serverUrl"`
	LastLogs       []string             `json:"lastLogs"`
	SessionActive  bool                 `json:"sessionActive"`
	ClientReady    bool                 `json:"clientReady"`
	LastClientPing string               `json:"lastClientPing"`
}

func resolveBridgeImportDir() string {
	dir := filepath.Join(resolveRuntimeRootDir(), "bridge-import")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func resolveBridgeImportLastPath() string {
	return filepath.Join(resolveBridgeImportDir(), "last-import.json")
}

func resolveBridgeImportHistoryPath() string {
	return filepath.Join(resolveBridgeImportDir(), "history.jsonl")
}

func resolveBridgeImportLogPath() string {
	return filepath.Join(resolveRuntimeLogDir(), "bridge-import.log")
}

func appendBridgeImportLogf(format string, args ...any) {
	appendLine(resolveBridgeImportLogPath(), fmt.Sprintf(format, args...))
}

func previewBridgeText(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\n", "\\n")
	if limit <= 0 || len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "...(truncated)"
}

func shouldIgnoreBridgeImportSource(sourceURL, sourceOrigin, title string) bool {
	sourceURL = strings.TrimSpace(sourceURL)
	sourceOrigin = strings.TrimSpace(sourceOrigin)
	title = strings.TrimSpace(title)

	checkURL := func(raw string) bool {
		if raw == "" {
			return false
		}

		parsed, err := url.Parse(raw)
		if err != nil {
			return false
		}

		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		path := strings.ToLower(strings.TrimSpace(parsed.Path))

		if host == bridgeServerHost && strings.HasPrefix(path, "/bridge/") {
			return true
		}
		if host == "localhost" && strings.HasPrefix(path, "/bridge/") {
			return true
		}
		if strings.HasSuffix(host, "tampermonkey.net") {
			if path == "/script_installation.php" || path == "/userscript.php" {
				return true
			}
		}
		if (path == "/oauth2/authorize" || path == "/authorize") && parsed.Query().Get("client_id") != "" {
			return true
		}

		return false
	}

	if checkURL(sourceURL) || checkURL(sourceOrigin) {
		return true
	}

	normalizedTitle := strings.ToLower(title)
	if strings.Contains(normalizedTitle, "tampermonkey") && strings.Contains(normalizedTitle, "script installation") {
		return true
	}

	return false
}

func readBridgePayloadMapAt(payload map[string]any, keys ...string) map[string]any {
	current := any(payload)
	for _, key := range keys {
		target, ok := current.(map[string]any)
		if !ok || len(target) == 0 {
			return map[string]any{}
		}
		current = target[key]
	}
	result, _ := current.(map[string]any)
	if len(result) == 0 {
		return map[string]any{}
	}
	return result
}

func readBridgePayloadArrayAt(payload map[string]any, keys ...string) []any {
	current := any(payload)
	for _, key := range keys {
		target, ok := current.(map[string]any)
		if !ok || len(target) == 0 {
			return nil
		}
		current = target[key]
	}
	list, _ := current.([]any)
	return list
}

func readBridgePayloadBoolAt(payload map[string]any, keys ...string) bool {
	current := any(payload)
	for _, key := range keys {
		target, ok := current.(map[string]any)
		if !ok || len(target) == 0 {
			return false
		}
		current = target[key]
	}
	switch value := current.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func bridgeTokenCandidatesAreCookieOnly(payload map[string]any) bool {
	candidates := readBridgePayloadArrayAt(payload, "diagnostics", "token_candidates")
	if len(candidates) == 0 {
		return false
	}
	for _, item := range candidates {
		target, _ := item.(map[string]any)
		source := strings.ToLower(strings.TrimSpace(fmt.Sprint(target["source"])))
		storage := strings.ToLower(strings.TrimSpace(fmt.Sprint(target["storage"])))
		if storage != "cookie" && !strings.Contains(source, "cookie") {
			return false
		}
	}
	return true
}

func shouldIgnoreBridgeImportPayload(payload map[string]any, extracted map[string]any, tokenCount int) (bool, string) {
	sourceURL := readBridgePayloadString(payload, "source_url", "sourceUrl")
	title := strings.ToLower(readBridgePayloadString(payload, "title"))
	accessToken := readBridgePayloadString(extracted, "resolved_access_token", "resolvedAccessToken", "access_token", "accessToken")
	userID := readBridgePayloadString(extracted, "resolved_user_id", "resolvedUserId", "user_id", "userId")
	endpoint := readBridgePayloadString(extracted, "endpoint")
	selfProbeOK := readBridgePayloadBoolAt(payload, "diagnostics", "self_probe", "ok")
	tokenProbeOK := readBridgePayloadBoolAt(payload, "diagnostics", "token_probe", "ok")
	observedAuthCount := len(readBridgePayloadArrayAt(payload, "diagnostics", "observed_auth_candidates"))
	observedSnapshotCount := len(readBridgePayloadArrayAt(payload, "diagnostics", "observed_token_snapshots"))
	cookieOnlyCandidates := bridgeTokenCandidatesAreCookieOnly(payload)

	if tokenCount > 0 {
		return false, ""
	}
	if strings.TrimSpace(accessToken) == "" && strings.TrimSpace(userID) == "" && !selfProbeOK && !tokenProbeOK && observedAuthCount == 0 && observedSnapshotCount == 0 {
		return true, "no_bridge_signal"
	}
	if (strings.Contains(strings.ToLower(sourceURL), "/oauth2/authorize") || strings.Contains(title, "authorize -")) &&
		strings.TrimSpace(userID) == "" &&
		strings.TrimSpace(endpoint) == "" &&
		!selfProbeOK &&
		!tokenProbeOK &&
		observedAuthCount == 0 &&
		observedSnapshotCount == 0 {
		return true, "oauth_surface"
	}
	if strings.TrimSpace(accessToken) != "" &&
		strings.TrimSpace(userID) == "" &&
		strings.TrimSpace(endpoint) == "" &&
		!selfProbeOK &&
		!tokenProbeOK &&
		observedAuthCount == 0 &&
		observedSnapshotCount == 0 &&
		cookieOnlyCandidates {
		return true, "cookie_only_nonrelay"
	}
	return false, ""
}

func bridgePayloadKeys(payload map[string]any) string {
	if len(payload) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return "-"
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func readBridgeLogTail(limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	raw, err := os.ReadFile(resolveBridgeImportLogPath())
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	result := make([]string, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
	}
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

func maskBridgeTokenPreview(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 12 {
		return token[:3] + "***"
	}
	return token[:10] + "..." + token[len(token)-4:]
}

func extractBridgePayloadMap(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return map[string]any{}
	}
	if extracted, ok := payload["extracted"].(map[string]any); ok && len(extracted) > 0 {
		return extracted
	}
	if nested, ok := payload["data"].(map[string]any); ok {
		if extracted, ok := nested["extracted"].(map[string]any); ok && len(extracted) > 0 {
			return extracted
		}
	}
	return payload
}

func readBridgePayloadArrayLength(payload map[string]any, keys ...string) int {
	readLen := func(target map[string]any, key string) int {
		if len(target) == 0 {
			return 0
		}
		if list, ok := target[key].([]any); ok {
			return len(list)
		}
		return 0
	}

	for _, key := range keys {
		if size := readLen(payload, key); size > 0 {
			return size
		}
	}
	extracted := extractBridgePayloadMap(payload)
	for _, key := range keys {
		if size := readLen(extracted, key); size > 0 {
			return size
		}
	}
	nested, _ := payload["data"].(map[string]any)
	for _, key := range keys {
		if size := readLen(nested, key); size > 0 {
			return size
		}
	}
	return 0
}

func computeBridgeImportReadyReason(payload map[string]any, extracted map[string]any, tokenCount int) (bool, string) {
	siteURL := readBridgePayloadString(extracted, "site_url", "siteUrl", "source_origin", "sourceOrigin")
	accessToken := readBridgePayloadString(extracted, "resolved_access_token", "resolvedAccessToken", "access_token", "accessToken")
	userID := readBridgePayloadString(extracted, "resolved_user_id", "resolvedUserId", "user_id", "userId")
	endpoint := readBridgePayloadString(extracted, "endpoint")
	extractedError := strings.ToLower(readBridgePayloadString(extracted, "error"))
	selfProbeOK := readBridgePayloadBoolAt(payload, "diagnostics", "self_probe", "ok")
	tokenProbeOK := readBridgePayloadBoolAt(payload, "diagnostics", "token_probe", "ok")
	observedAuthCount := len(readBridgePayloadArrayAt(payload, "diagnostics", "observed_auth_candidates"))
	observedSnapshotCount := len(readBridgePayloadArrayAt(payload, "diagnostics", "observed_token_snapshots"))
	if strings.TrimSpace(siteURL) == "" {
		return false, "missing_site_url"
	}
	if extractedError == "token_expired" || extractedError == "token_expired_local" {
		return false, extractedError
	}
	if extractedError == "not_logged_in" {
		return false, extractedError
	}
	if tokenCount > 0 {
		return true, "prefetched_tokens"
	}
	if strings.TrimSpace(accessToken) != "" {
		if strings.TrimSpace(userID) != "" || strings.TrimSpace(endpoint) != "" || selfProbeOK || tokenProbeOK || observedAuthCount > 0 || observedSnapshotCount > 0 {
			return true, "access_token_contextual"
		}
		return false, "weak_access_token"
	}
	return false, "missing_access_token_and_tokens"
}

func (a *App) ensureBridgeServer() error {
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()

	if a.bridgeServer != nil && a.bridgeListener != nil {
		return nil
	}

	address := fmt.Sprintf("%s:%d", bridgeServerHost, bridgeServerPort)
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return fmt.Errorf("listen %s failed: %w", address, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/bridge/ping", a.handleBridgePing)
	mux.HandleFunc("/bridge/import", a.handleBridgeImport)
	mux.HandleFunc("/bridge/install", a.handleBridgeInstallPage)
	mux.HandleFunc("/bridge/install/status", a.handleBridgeInstallStatus)
	mux.HandleFunc("/bridge/script.user.js", a.handleBridgeUserScript)
	mux.HandleFunc("/advanced-proxy/ping", a.handleAdvancedProxyPing)
	mux.HandleFunc("/advanced-proxy/claude/v1/messages", a.handleAdvancedProxyClaude)
	mux.HandleFunc("/advanced-proxy/claude/messages", a.handleAdvancedProxyClaude)
	mux.HandleFunc("/advanced-proxy/codex/v1/chat/completions", a.handleAdvancedProxyCodex)
	mux.HandleFunc("/advanced-proxy/codex/v1/responses", a.handleAdvancedProxyCodex)
	mux.HandleFunc("/advanced-proxy/codex/v1/responses/compact", a.handleAdvancedProxyCodex)
	mux.HandleFunc("/advanced-proxy/opencode/v1/chat/completions", a.handleAdvancedProxyOpenCode)
	mux.HandleFunc("/advanced-proxy/opencode/v1/responses", a.handleAdvancedProxyOpenCode)
	mux.HandleFunc("/advanced-proxy/opencode/v1/responses/compact", a.handleAdvancedProxyOpenCode)
	mux.HandleFunc("/advanced-proxy/openclaw/v1/chat/completions", a.handleAdvancedProxyOpenClaw)
	mux.HandleFunc("/advanced-proxy/openclaw/v1/responses", a.handleAdvancedProxyOpenClaw)
	mux.HandleFunc("/advanced-proxy/openclaw/v1/responses/compact", a.handleAdvancedProxyOpenClaw)

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	a.bridgeListener = listener
	a.bridgeServer = server

	appendBridgeImportLogf("[BRIDGE_START] addr=%s mode=%s", address, a.mode)

	go func(server *http.Server, listener net.Listener) {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			appendBridgeImportLogf("[BRIDGE_STOP_ERROR] err=%v", serveErr)
			debugLogf("bridge server serve failed: %v", serveErr)
		}
	}(server, listener)

	return nil
}

func (a *App) OpenBridgeScriptInstallPage() error {
	if err := a.ensureBridgeServer(); err != nil {
		return err
	}
	if a.ctx == nil {
		return fmt.Errorf("wails context not ready")
	}
	wruntime.BrowserOpenURL(a.ctx, fmt.Sprintf("http://%s:%d/bridge/install", bridgeServerHost, bridgeServerPort))
	return nil
}

func (a *App) StartBridgeImportSession() (*BridgeImportSnapshot, error) {
	if err := a.ensureBridgeServer(); err != nil {
		return nil, err
	}
	return a.ResetBridgeImportSession()
}

func (a *App) ResetBridgeImportSession() (*BridgeImportSnapshot, error) {
	paths := []string{
		resolveBridgeImportLastPath(),
		resolveBridgeImportHistoryPath(),
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	a.bridgeSessionActive.Store(true)
	a.bridgeClientPingAt.Store(0)
	appendBridgeImportLogf("[SESSION_RESET] cleared bridge import session")
	return &BridgeImportSnapshot{Records: []BridgeImportRecord{}, SessionActive: true}, nil
}

func (a *App) GetBridgeImportSnapshot() (*BridgeImportSnapshot, error) {
	snapshot, err := readBridgeImportSnapshot()
	if err != nil {
		return nil, err
	}
	snapshot.SessionActive = a.bridgeSessionActive.Load()
	lastPingAt := a.bridgeClientPingAt.Load()
	if lastPingAt > 0 {
		snapshot.LastClientPing = time.UnixMilli(lastPingAt).Format(time.RFC3339Nano)
		snapshot.ClientReady = time.Since(time.UnixMilli(lastPingAt)) <= 15*time.Second
	}
	return snapshot, nil
}

func (a *App) CloseBridgeImportSession() (*BridgeImportSnapshot, error) {
	a.bridgeSessionActive.Store(false)
	a.bridgeClientPingAt.Store(0)
	appendBridgeImportLogf("[SESSION_CLOSE] bridge session closed by ui")
	snapshot, err := a.GetBridgeImportSnapshot()
	if err != nil {
		return nil, err
	}
	if !a.shouldAutoStartBridgeServer() {
		a.stopBridgeServer()
	}
	return snapshot, nil
}

func (a *App) stopBridgeServer() {
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()

	if a.bridgeServer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	appendBridgeImportLogf("[BRIDGE_STOP] requested")
	_ = a.bridgeServer.Shutdown(ctx)
	a.bridgeServer = nil
	a.bridgeListener = nil
}

func (a *App) handleBridgePing(writer http.ResponseWriter, request *http.Request) {
	appendBridgeImportLogf("[PING_REQUEST] method=%s remote=%s ua=%s origin=%s", request.Method, request.RemoteAddr, previewBridgeText(request.UserAgent(), 120), previewBridgeText(request.Header.Get("Origin"), 120))
	if request.Method == http.MethodOptions {
		writeBridgeJSON(writer, http.StatusOK, map[string]any{
			"ok": true,
		})
		return
	}
	if request.Method != http.MethodGet {
		writeBridgeJSON(writer, http.StatusMethodNotAllowed, map[string]any{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}

	clientKind := strings.ToLower(strings.TrimSpace(request.Header.Get("X-AllApiDeck-Bridge-Client")))
	sessionActive := a.bridgeSessionActive.Load()
	if clientKind == "userscript" {
		if !sessionActive {
			appendBridgeImportLogf("[PING_REJECT] remote=%s reason=session_inactive", request.RemoteAddr)
			writeBridgeJSON(writer, http.StatusConflict, map[string]any{
				"ok":            false,
				"sessionActive": false,
				"message":       "bridge session inactive",
			})
			return
		}
		a.bridgeClientPingAt.Store(time.Now().UnixMilli())
	}

	writeBridgeJSON(writer, http.StatusOK, map[string]any{
		"ok":            true,
		"host":          bridgeServerHost,
		"port":          bridgeServerPort,
		"mode":          string(a.mode),
		"version":       bridgeServerVersion,
		"serverUrl":     fmt.Sprintf("http://%s:%d", bridgeServerHost, bridgeServerPort),
		"sessionActive": sessionActive,
	})
}

func (a *App) handleBridgeImport(writer http.ResponseWriter, request *http.Request) {
	appendBridgeImportLogf("[IMPORT_REQUEST] method=%s remote=%s contentLength=%d ua=%s origin=%s referer=%s", request.Method, request.RemoteAddr, request.ContentLength, previewBridgeText(request.UserAgent(), 120), previewBridgeText(request.Header.Get("Origin"), 120), previewBridgeText(request.Header.Get("Referer"), 160))
	if request.Method == http.MethodOptions {
		writeBridgeJSON(writer, http.StatusOK, map[string]any{
			"ok": true,
		})
		return
	}
	if request.Method != http.MethodPost {
		writeBridgeJSON(writer, http.StatusMethodNotAllowed, map[string]any{
			"ok":      false,
			"message": "method not allowed",
		})
		return
	}
	if !a.bridgeSessionActive.Load() {
		appendBridgeImportLogf("[IMPORT_IGNORE] remote=%s reason=session_inactive", request.RemoteAddr)
		writeBridgeJSON(writer, http.StatusConflict, map[string]any{
			"ok":      false,
			"ignored": true,
			"reason":  "session_inactive",
		})
		return
	}

	remoteIP := extractBridgeRemoteIP(request.RemoteAddr)
	if !isLoopbackBridgeRemote(remoteIP) {
		appendBridgeImportLogf("[IMPORT_REJECT] remote=%s reason=non_loopback", request.RemoteAddr)
		writeBridgeJSON(writer, http.StatusForbidden, map[string]any{
			"ok":      false,
			"message": "bridge only accepts loopback requests",
		})
		return
	}

	body, err := io.ReadAll(io.LimitReader(request.Body, 1024*1024))
	if err != nil {
		appendBridgeImportLogf("[IMPORT_FAIL] read body err=%v", err)
		writeBridgeJSON(writer, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "failed to read request body",
		})
		return
	}

	rawText := strings.TrimSpace(string(body))
	appendBridgeImportLogf("[IMPORT_BODY] remote=%s preview=%s", request.RemoteAddr, previewBridgeText(rawText, 360))
	if rawText == "" {
		appendBridgeImportLogf("[IMPORT_FAIL] stage=empty_body remote=%s", request.RemoteAddr)
		writeBridgeJSON(writer, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "request body is empty",
		})
		return
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rawText), &payload); err != nil {
		appendBridgeImportLogf("[IMPORT_FAIL] invalid json err=%v", err)
		writeBridgeJSON(writer, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"message": "invalid json payload",
		})
		return
	}

	receivedAt := time.Now().Format(time.RFC3339Nano)
	recordID := fmt.Sprintf("bridge-%d", time.Now().UnixNano())
	payloadType := strings.TrimSpace(fmt.Sprint(payload["type"]))
	sourceURL := readBridgePayloadString(payload, "source_url", "sourceUrl")
	sourceOrigin := readBridgePayloadString(payload, "source_origin", "sourceOrigin")
	title := readBridgePayloadString(payload, "title")
	if shouldIgnoreBridgeImportSource(sourceURL, sourceOrigin, title) {
		appendBridgeImportLogf(
			"[IMPORT_IGNORE] source=%s origin=%s title=%s reason=bootstrap_page",
			previewBridgeText(sourceURL, 160),
			previewBridgeText(sourceOrigin, 160),
			previewBridgeText(title, 96),
		)
		writeBridgeJSON(writer, http.StatusOK, map[string]any{
			"ok":      true,
			"ignored": true,
			"reason":  "bootstrap_page",
		})
		return
	}
	extracted := extractBridgePayloadMap(payload)
	tokenCount := readBridgePayloadArrayLength(payload, "tokens")
	if ignored, ignoreReason := shouldIgnoreBridgeImportPayload(payload, extracted, tokenCount); ignored {
		appendBridgeImportLogf(
			"[IMPORT_IGNORE] source=%s origin=%s title=%s reason=%s",
			previewBridgeText(sourceURL, 160),
			previewBridgeText(sourceOrigin, 160),
			previewBridgeText(title, 96),
			ignoreReason,
		)
		writeBridgeJSON(writer, http.StatusOK, map[string]any{
			"ok":      true,
			"ignored": true,
			"reason":  ignoreReason,
		})
		return
	}
	ready, readyReason := computeBridgeImportReadyReason(payload, extracted, tokenCount)
	appendBridgeImportLogf(
		"[IMPORT_PARSE] id=%s type=%s title=%s source=%s origin=%s siteType=%s user=%s tokenCount=%d ready=%t readyReason=%s keys=%s",
		recordID,
		previewBridgeText(payloadType, 64),
		previewBridgeText(title, 96),
		previewBridgeText(sourceURL, 160),
		previewBridgeText(sourceOrigin, 160),
		previewBridgeText(readBridgePayloadString(extracted, "site_type", "siteType"), 40),
		previewBridgeText(readBridgePayloadString(extracted, "resolved_user_id", "resolvedUserId", "user_id", "userId"), 40),
		tokenCount,
		ready,
		readyReason,
		bridgePayloadKeys(payload),
	)
	record := map[string]any{
		"id":         recordID,
		"receivedAt": receivedAt,
		"remoteAddr": request.RemoteAddr,
		"payload":    payload,
	}

	lastRaw, _ := json.MarshalIndent(record, "", "  ")
	historyRaw, _ := json.Marshal(record)
	lastPath := resolveBridgeImportLastPath()
	historyPath := resolveBridgeImportHistoryPath()

	if err := os.WriteFile(lastPath, lastRaw, 0o644); err != nil {
		appendBridgeImportLogf("[IMPORT_FAIL] write last snapshot path=%s err=%v", lastPath, err)
		writeBridgeJSON(writer, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "failed to persist latest bridge import",
		})
		return
	}
	if err := appendBridgeHistory(historyPath, historyRaw); err != nil {
		appendBridgeImportLogf("[IMPORT_FAIL] append history path=%s err=%v", historyPath, err)
		writeBridgeJSON(writer, http.StatusInternalServerError, map[string]any{
			"ok":      false,
			"message": "failed to persist bridge import history",
		})
		return
	}

	appendBridgeImportLogf("[IMPORT_OK] id=%s type=%s source=%s remote=%s last=%s history=%s", recordID, previewBridgeText(payloadType, 64), previewBridgeText(sourceURL, 160), request.RemoteAddr, lastPath, historyPath)

	writeBridgeJSON(writer, http.StatusOK, map[string]any{
		"ok":          true,
		"id":          recordID,
		"receivedAt":  receivedAt,
		"storedAt":    lastPath,
		"historyPath": historyPath,
		"type":        payloadType,
		"sourceUrl":   sourceURL,
		"logPath":     resolveBridgeImportLogPath(),
	})
}

func (a *App) handleBridgeInstallPage(writer http.ResponseWriter, request *http.Request) {
	appendBridgeImportLogf("[INSTALL_PAGE] method=%s remote=%s ua=%s", request.Method, request.RemoteAddr, previewBridgeText(request.UserAgent(), 120))
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(renderBridgeInstallPageHTML()))
}

func (a *App) handleBridgeInstallStatus(writer http.ResponseWriter, request *http.Request) {
	appendBridgeImportLogf("[INSTALL_STATUS] method=%s remote=%s ua=%s", request.Method, request.RemoteAddr, previewBridgeText(request.UserAgent(), 120))
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(detectBridgeScriptManagerInstall(bridgeScriptManagerExtensionID))
}

func detectBridgeScriptManagerInstall(extensionID string) bridgeScriptManagerInstallStatus {
	status := bridgeScriptManagerInstallStatus{
		Installed: false,
		Source:    "not_found",
	}

	for _, candidate := range bridgeBrowserUserDataRoots() {
		if !isDirectory(candidate.Root) {
			continue
		}
		status.ScannedRoots = append(status.ScannedRoots, candidate.Root)

		entries, err := os.ReadDir(candidate.Root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			profileName := strings.TrimSpace(entry.Name())
			if profileName == "" {
				continue
			}
			extensionRoot := filepath.Join(candidate.Root, profileName, "Extensions", extensionID)
			if !isDirectory(extensionRoot) {
				continue
			}

			version := ""
			versionEntries, err := os.ReadDir(extensionRoot)
			if err == nil {
				versionNames := make([]string, 0, len(versionEntries))
				for _, versionEntry := range versionEntries {
					if versionEntry.IsDir() {
						versionNames = append(versionNames, versionEntry.Name())
					}
				}
				sort.Strings(versionNames)
				if len(versionNames) > 0 {
					version = versionNames[len(versionNames)-1]
				}
			}

			status.Installed = true
			status.Source = "filesystem"
			status.Browser = candidate.Browser
			status.Profile = profileName
			status.Version = version
			status.Path = extensionRoot
			return status
		}
	}

	return status
}

func bridgeBrowserUserDataRoots() []bridgeBrowserUserDataRoot {
	var roots []bridgeBrowserUserDataRoot
	add := func(browser string, root string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		roots = append(roots, bridgeBrowserUserDataRoot{
			Browser: browser,
			Root:    root,
		})
	}

	switch runtime.GOOS {
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		add("Chrome", filepath.Join(localAppData, "Google", "Chrome", "User Data"))
		add("Edge", filepath.Join(localAppData, "Microsoft", "Edge", "User Data"))
		add("Chromium", filepath.Join(localAppData, "Chromium", "User Data"))
		add("Brave", filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data"))
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		add("Chrome", filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome"))
		add("Edge", filepath.Join(homeDir, "Library", "Application Support", "Microsoft Edge"))
		add("Chromium", filepath.Join(homeDir, "Library", "Application Support", "Chromium"))
		add("Brave", filepath.Join(homeDir, "Library", "Application Support", "BraveSoftware", "Brave-Browser"))
	default:
		homeDir, _ := os.UserHomeDir()
		add("Chrome", filepath.Join(homeDir, ".config", "google-chrome"))
		add("Edge", filepath.Join(homeDir, ".config", "microsoft-edge"))
		add("Chromium", filepath.Join(homeDir, ".config", "chromium"))
		add("Brave", filepath.Join(homeDir, ".config", "BraveSoftware", "Brave-Browser"))
	}

	return roots
}

func renderBridgeInstallPageHTML() string {
	html := `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>All API Deck Bridge Installer</title>
  <style>
    :root{
      --ink:#223122;
      --muted:#62725d;
      --line:rgba(76,101,69,.14);
      --line-strong:rgba(76,101,69,.22);
      --card:rgba(255,255,255,.82);
      --gold:#caa347;
      --gold-deep:#8b6819;
      --green:#4f6c3e;
      --green-deep:#38512c;
      --ok:#1f8f52;
      --danger:#d9485f;
      --shadow:0 24px 52px rgba(58,77,50,.12);
    }
    *{box-sizing:border-box}
    body{
      margin:0;
      min-height:100vh;
      font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
      color:var(--ink);
      background:
        radial-gradient(circle at top right, rgba(230,209,146,.34), transparent 26%),
        radial-gradient(circle at left 20% bottom 10%, rgba(177,208,152,.26), transparent 25%),
        linear-gradient(135deg,#fbf7ea,#eef5e2 55%,#edf2e8);
    }
    .shell{max-width:1160px;margin:0 auto;padding:42px 22px 48px}
    .hero{
      position:relative;
      overflow:hidden;
      border-radius:32px;
      padding:30px 30px 26px;
      background:
        linear-gradient(120deg, rgba(255,255,255,.20), rgba(255,255,255,0) 28%),
        linear-gradient(145deg, rgba(249,233,181,.96), rgba(242,220,160,.92) 35%, rgba(236,243,228,.94) 100%);
      border:1px solid rgba(185,150,63,.18);
      box-shadow:var(--shadow), inset 0 1px 0 rgba(255,255,255,.72);
    }
    .hero::before{
      content:"";
      position:absolute;
      right:-60px;
      top:-60px;
      width:220px;
      height:220px;
      border-radius:50%;
      background:radial-gradient(circle, rgba(255,255,255,.54), rgba(255,255,255,0));
      pointer-events:none;
    }
    .hero-kicker{
      display:inline-flex;
      align-items:center;
      gap:8px;
      margin-bottom:14px;
      padding:7px 12px;
      border-radius:999px;
      font-size:12px;
      font-weight:800;
      letter-spacing:.16em;
      color:var(--gold-deep);
      background:rgba(255,255,255,.56);
      border:1px solid rgba(185,150,63,.18);
      text-transform:uppercase;
    }
    h1{
      margin:0;
      font:700 38px/1.04 Georgia,'Times New Roman',serif;
      color:#2d3f2a;
      max-width:760px;
    }
    .hero-copy{
      margin:14px 0 0;
      max-width:760px;
      color:#586755;
      font-size:15px;
      line-height:1.75;
    }
    .hero-actions{display:flex;gap:12px;flex-wrap:wrap;margin-top:20px}
    .hero-grid{display:grid;grid-template-columns:minmax(0,1.35fr) minmax(320px,.75fr);gap:18px;margin-top:22px}
    .panel{border-radius:28px;border:1px solid var(--line);background:var(--card);box-shadow:var(--shadow);backdrop-filter:blur(8px)}
    .panel-main{padding:24px;display:flex;flex-direction:column}
    .process-block{order:2;margin-top:18px}
    .process-label{
      display:inline-flex;align-items:center;min-height:34px;padding:0 14px;
      border-radius:999px;background:rgba(255,246,220,.92);border:1px solid rgba(202,163,71,.24);
      color:#74540d;font-size:12px;font-weight:800;letter-spacing:.08em
    }
    .steps{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin-bottom:18px}
    .step{padding:14px 14px 12px;border-radius:18px;background:linear-gradient(180deg, rgba(255,255,255,.92), rgba(244,249,239,.86));border:1px solid var(--line);min-height:92px;transition:transform .18s ease, border-color .18s ease, box-shadow .18s ease}
    .step.is-active{border-color:rgba(202,163,71,.44);box-shadow:0 12px 24px rgba(202,163,71,.12);transform:translateY(-1px)}
    .step.is-done{border-color:rgba(31,143,82,.28);background:linear-gradient(180deg, rgba(245,255,248,.98), rgba(236,247,239,.92))}
    .step-no{display:inline-flex;align-items:center;justify-content:center;width:26px;height:26px;border-radius:999px;margin-bottom:10px;background:rgba(79,108,62,.10);color:var(--green-deep);font:800 12px/1 ui-monospace,SFMono-Regular,Menlo,monospace}
    .step-title{color:#2f402d;font-size:14px;font-weight:700}
    .step-desc{margin-top:6px;color:var(--muted);font-size:12px;line-height:1.55}
    .state-card{border-radius:24px;padding:22px 22px 20px;background:linear-gradient(180deg, rgba(255,255,255,.92), rgba(245,249,239,.86));border:1px solid var(--line)}
    .state-head{display:flex;align-items:flex-start;gap:14px}
    .state-icon{width:56px;height:56px;flex:0 0 auto;border-radius:18px;display:flex;align-items:center;justify-content:center;background:rgba(79,108,62,.08);color:var(--green-deep)}
    .state-title{margin:2px 0 6px;font-size:22px;line-height:1.2;font-weight:800;color:#2f402d}
    .state-copy{margin:0;color:var(--muted);font-size:14px;line-height:1.7}
    .status-chip{display:inline-flex;align-items:center;gap:8px;margin-top:14px;padding:8px 12px;border-radius:999px;background:rgba(79,108,62,.08);color:#344a31;border:1px solid rgba(79,108,62,.10);font-size:12px;font-weight:700}
    .status-dot{width:10px;height:10px;border-radius:999px;background:currentColor;box-shadow:0 0 0 4px rgba(79,108,62,.10)}
    .actions{display:flex;gap:12px;flex-wrap:wrap;margin-top:18px}
    .button{appearance:none;border:none;cursor:pointer;text-decoration:none;display:inline-flex;align-items:center;justify-content:center;min-height:48px;padding:0 22px;border-radius:999px;font-size:14px;font-weight:800;letter-spacing:.01em;transition:transform .16s ease, box-shadow .16s ease, background .16s ease, border-color .16s ease}
    .button:hover{transform:translateY(-1px)}
    .button-primary{color:#fff;background:linear-gradient(180deg, var(--green), var(--green-deep));box-shadow:0 12px 24px rgba(56,81,44,.18)}
    .button-ghost{color:#314230;background:#fff;border:1px solid var(--line-strong)}
    .button-gold{color:#5f4300;background:linear-gradient(180deg, rgba(255,246,220,.96), rgba(246,229,183,.94));border:1px solid rgba(202,163,71,.28);box-shadow:0 10px 24px rgba(202,163,71,.12)}
    .helper{margin-top:14px;color:#768571;font-size:12px;line-height:1.7}
    .helper strong{color:#3e5139}
    .env-tags{display:flex;flex-wrap:wrap;gap:8px;margin-top:14px}
    .tag{display:inline-flex;align-items:center;min-height:32px;padding:0 12px;border-radius:999px;background:rgba(79,108,62,.07);border:1px solid rgba(79,108,62,.09);color:#42513e;font-size:12px;font-weight:700}
    .side{padding:20px;display:grid;gap:14px;align-content:start}
    .meta-card{padding:18px;border-radius:22px;background:linear-gradient(180deg, rgba(255,255,255,.88), rgba(248,251,245,.80));border:1px solid var(--line)}
    .meta-title{margin:0 0 10px;font-size:15px;font-weight:800;color:#314230}
    .meta-row{display:grid;gap:6px;margin-top:10px}
    .meta-label{font-size:12px;color:#7a8875;font-weight:700;text-transform:uppercase;letter-spacing:.08em}
    code{display:block;overflow:auto;padding:10px 12px;border-radius:14px;background:rgba(36,49,34,.92);color:#eef7e8;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;line-height:1.55}
    .list{display:grid;gap:10px;margin:0;padding:0;list-style:none}
    .list li{position:relative;padding-left:18px;color:#5f6e5b;font-size:13px;line-height:1.7}
    .list li::before{content:"";position:absolute;left:0;top:.68em;width:8px;height:8px;border-radius:999px;background:linear-gradient(180deg, rgba(202,163,71,.92), rgba(79,108,62,.92))}
    .spinner{width:18px;height:18px;border-radius:999px;border:2px solid rgba(79,108,62,.18);border-top-color:var(--gold);animation:spin .9s linear infinite}
    .hidden{display:none!important}
    .tone-checking .state-icon{background:rgba(202,163,71,.10);color:var(--gold-deep)}
    .tone-checking .status-chip{background:rgba(202,163,71,.10);color:#79570a;border-color:rgba(202,163,71,.14)}
    .tone-missing .state-icon{background:rgba(217,72,95,.08);color:var(--danger)}
    .tone-missing .status-chip{background:rgba(217,72,95,.08);color:#8f2334;border-color:rgba(217,72,95,.14)}
    .tone-installed .state-icon{background:rgba(31,143,82,.10);color:var(--ok)}
    .tone-installed .status-chip{background:rgba(31,143,82,.10);color:#166b3b;border-color:rgba(31,143,82,.14)}
    .bridge-toast-mask{
      position:fixed;inset:0;z-index:99999;background:rgba(24,33,20,.24);
      display:flex;align-items:center;justify-content:center;padding:18px
    }
    .bridge-toast{
      width:min(420px,100%%);padding:22px 22px 18px;border-radius:24px;
      background:linear-gradient(180deg, rgba(255,255,255,.98), rgba(246,250,241,.96));
      border:1px solid rgba(76,101,69,.14);box-shadow:0 24px 48px rgba(58,77,50,.18)
    }
    .bridge-toast-title{margin:0;font:700 28px/1.08 Georgia,'Times New Roman',serif;color:#2d3f2a}
    .bridge-toast-copy{margin:10px 0 0;color:#667760;font-size:14px;line-height:1.7}
    .bridge-toast-actions{display:flex;justify-content:flex-end;gap:10px;margin-top:18px}
    @keyframes spin{to{transform:rotate(360deg)}}
    @media (max-width:960px){.hero-grid{grid-template-columns:1fr}}
    @media (max-width:720px){.shell{padding:24px 14px 32px}.hero{padding:22px 18px 18px}h1{font-size:30px}.panel-main,.side{padding:16px}.steps{grid-template-columns:1fr}.actions .button{width:100%}.bridge-toast-actions .button{width:auto}}
  </style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <div class="hero-kicker">Bridge Install Flow</div>
      <h1>浏览器桥接脚本安装</h1>
      <p class="hero-copy">这个页面用于把浏览器标签页和桌面端 All API Deck 连接起来。你可以先检测当前浏览器是否已安装篡改猴，也可以直接安装桥接脚本；脚本装好后，回到桌面端导入窗口并保持目标站点标签页打开即可。</p>
      <div class="hero-actions">
        <a class="button button-primary" href="__SCRIPT_URL__">安装桥接脚本</a>
        <button id="test-bridge-btn" class="button button-ghost" type="button">测试本地桥接</button>
      </div>
      <div class="hero-grid">
        <section class="panel panel-main">
          <div class="process-block">
            <div class="process-label">接入流程</div>
            <div class="steps">
            <div class="step">
              <div class="step-no">01</div>
              <div class="step-title">检测篡改猴</div>
              <div class="step-desc">检查此浏览器中是否已经安装并启用了 Tampermonkey。</div>
            </div>
            <div class="step">
              <div class="step-no">02</div>
              <div class="step-title">安装桥接脚本</div>
              <div class="step-desc">桥接脚本可以直接安装，不必等待环境检测完成后再继续。</div>
            </div>
            <div class="step">
              <div class="step-no">03</div>
              <div class="step-title">返回桌面端</div>
              <div class="step-desc">打开所有中转站确保已经登录，会自动识别并追加到程序内部。</div>
            </div>
          </div>
          </div>

          <div id="state-checking" class="state-card tone-checking">
            <div class="state-head">
              <div class="state-icon"><div class="spinner"></div></div>
              <div>
                <div class="state-title">正在检测浏览器环境...</div>
                <p class="state-copy">页面正在确认当前浏览器配置中是否可用 Tampermonkey。这个检测只用于提示，不影响你直接安装桥接脚本。</p>
                <div class="status-chip"><span class="status-dot"></span> 环境检测中</div>
              </div>
            </div>
            <div class="helper">如果你刚装好插件，稍等片刻后点击“重新检测”即可。</div>
            <div class="actions">
              <button id="retry-checking-btn" class="button button-ghost" type="button">重新检测</button>
            </div>
          </div>

          <div id="state-missing" class="state-card tone-missing hidden">
            <div class="state-head">
              <div class="state-icon" aria-hidden="true">
                <svg viewBox="0 0 1024 1024" width="30" height="30" fill="currentColor"><path d="M512 85.333333c-235.648 0-426.666667 191.018667-426.666667 426.666667s191.018667 426.666667 426.666667 426.666667 426.666667-191.018667 426.666667-426.666667S747.648 85.333333 512 85.333333z m186.282667 274.602667c26.112 0 47.445333 21.290667 47.445333 47.445333 0 26.112-21.333333 47.445333-47.445333 47.445333-26.154667 0-47.445333-21.333333-47.445333-47.445333 0-26.154667 21.290667-47.445333 47.445333-47.445333z m-372.650667 0c26.154667 0 47.445333 21.290667 47.445333 47.445333 0 26.112-21.290667 47.445333-47.445333 47.445333-26.112 0-47.402667-21.333333-47.402667-47.445333 0-26.154667 21.290667-47.445333 47.402667-47.445333z m186.368 412.330667c-107.562667 0-198.613333-68.522667-230.144-162.816H742.144c-31.530667 94.293333-122.581333 162.816-230.144 162.816z"/></svg>
              </div>
              <div>
                <div class="state-title">未检测到 Tampermonkey</div>
                <p class="state-copy">请先安装并启用 Tampermonkey，然后刷新本页或点击“重新检测”。如果你使用的是其他脚本管理器，也可以直接继续安装桥接脚本。</p>
                <div class="status-chip"><span class="status-dot"></span> 需要脚本管理器</div>
              </div>
            </div>
            <div class="actions">
              <a class="button button-primary" href="__SCRIPT_MANAGER_URL__" target="_blank" rel="noreferrer">安装篡改猴</a>
              <button id="retry-missing-btn" class="button button-ghost" type="button">重新检测</button>
              <a class="button button-gold" href="__SCRIPT_URL__">仍然继续安装桥接脚本</a>
            </div>
            <div id="state-missing-helper" class="helper"><strong>提示：</strong> 当前检测会同时尝试本机浏览器扩展目录和前端资源探测；如果你确认已经安装脚本管理器，也可以直接继续安装桥接脚本。</div>
          </div>

          <div id="state-installed" class="state-card tone-installed hidden">
            <div class="state-head">
              <div class="state-icon" aria-hidden="true">
                <svg viewBox="0 0 1024 1024" width="30" height="30" fill="currentColor"><path d="M512 85.333333c-235.648 0-426.666667 191.018667-426.666667 426.666667s191.018667 426.666667 426.666667 426.666667 426.666667-191.018667 426.666667-426.666667S747.648 85.333333 512 85.333333z m226.901333 323.072l-289.450666 289.493334c-12.458667 12.458667-32.725333 12.458667-45.226667 0L285.056 578.688c-12.501333-12.458667-12.501333-32.725333 0-45.226667 12.501333-12.501333 32.768-12.501333 45.269333 0l96.554667 96.554667 266.837333-266.837333c12.501333-12.501333 32.768-12.501333 45.269334 0 12.458667 12.458667 12.458667 32.725333-0.085334 45.226666z"/></svg>
              </div>
              <div>
                <div class="state-title">环境就绪，可以继续安装</div>
                <p id="state-installed-copy" class="state-copy">当前浏览器已经识别到 Tampermonkey。现在可以安装桥接脚本，然后切回桌面端 All API Deck 导入窗口，保持目标站点标签页打开等待自动处理。</p>
                <div class="status-chip"><span class="status-dot"></span> 已可安装桥接脚本</div>
              </div>
            </div>
            <div id="state-installed-tags" class="env-tags">
              <span class="tag">Tampermonkey 已检测到</span>
              <span class="tag">桥接版本 __VERSION__</span>
            </div>
            <div class="actions">
              <a class="button button-primary" href="__SCRIPT_URL__">安装桥接脚本</a>
              <button id="retry-installed-bridge-btn" class="button button-ghost" type="button">测试本地桥接</button>
              <button id="retry-installed-btn" class="button button-ghost" type="button">重新检测</button>
            </div>
            <div id="state-installed-helper" class="helper"><strong>安装后：</strong> 不需要停留在此页面，直接去目标站点标签页，再回桌面端窗口即可。</div>
          </div>
        </section>

        <aside class="panel side">
          <section class="meta-card">
            <h2 class="meta-title">本地桥接信息</h2>
            <div class="meta-row">
              <div class="meta-label">Bridge Ping</div>
              <code>__PING_URL__</code>
            </div>
            <div class="meta-row">
              <div class="meta-label">Bridge Import</div>
              <code>__IMPORT_URL__</code>
            </div>
            <div class="meta-row">
              <div class="meta-label">Userscript</div>
              <code>__SCRIPT_URL__</code>
            </div>
          </section>

          <section class="meta-card">
            <h2 class="meta-title">使用提示</h2>
            <ul class="list">
              <li>建议优先使用 Chrome 或 Edge 内核浏览器访问这个安装页。</li>
              <li>即使环境检测失败，只要你确认已经有脚本管理器，也可以直接安装桥接脚本。</li>
              <li>桥接脚本安装完成后，不需要停留在本页，直接回目标站点标签页即可。</li>
              <li>桌面端导入窗口关闭后，油猴侧栏会自动隐藏，不会继续打扰。</li>
            </ul>
          </section>
        </aside>
      </div>
    </section>
  </main>

  <div id="bridge-toast-mask" class="bridge-toast-mask hidden">
    <div class="bridge-toast">
      <h2 id="bridge-toast-title" class="bridge-toast-title">联通</h2>
      <p id="bridge-toast-copy" class="bridge-toast-copy">本地桥接服务可访问。</p>
      <div class="bridge-toast-actions">
        <button id="bridge-toast-close" class="button button-primary" type="button">知道了</button>
      </div>
    </div>
  </div>

  <script>
    const extensionId = '__SCRIPT_MANAGER_EXTENSION_ID__';
    const extensionStatusUrl = '__EXTENSION_STATUS_URL__';
    const testCandidates = [
      'images/icon_grey16.png',
      'images/icon16.png',
      'images/icon.png',
      'images/icon48.png',
      'icon.png'
    ].map(path => 'chrome-extension://' + extensionId + '/' + path);

    const toastMask = document.getElementById('bridge-toast-mask');
    const toastTitle = document.getElementById('bridge-toast-title');
    const toastCopy = document.getElementById('bridge-toast-copy');
    const stateMissingHelper = document.getElementById('state-missing-helper');
    const stateInstalledCopy = document.getElementById('state-installed-copy');
    const stateInstalledHelper = document.getElementById('state-installed-helper');
    const stateInstalledTags = document.getElementById('state-installed-tags');

    function updateState(stateId) {
      ['state-checking', 'state-missing', 'state-installed'].forEach(id => {
        document.getElementById(id).classList.add('hidden');
      });
      document.getElementById(stateId).classList.remove('hidden');
    }

    function showBridgeToast(title, copy) {
      toastTitle.textContent = title;
      toastCopy.textContent = copy;
      toastMask.classList.remove('hidden');
    }

    function hideBridgeToast() {
      toastMask.classList.add('hidden');
    }

    function escapeHTML(value) {
      return String(value || '').replace(/[&<>\"']/g, ch => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '\"': '&quot;',
        '\'': '&#39;'
      }[ch] || ch));
    }

    function applyInstalledState(status, sourceLabel) {
      updateState('state-installed');

      const browser = status?.browser ? String(status.browser) : '当前浏览器';
      const profile = status?.profile ? String(status.profile) : '';
      const version = status?.version ? String(status.version) : '';
      const path = status?.path ? String(status.path) : '';
      const source = status?.source === 'filesystem'
        ? '已从本机浏览器扩展目录识别'
        : (sourceLabel || '已从当前浏览器识别');

      const locationLabel = [browser, profile].filter(Boolean).join(' / ');
      const detailBits = [source, locationLabel, version ? ('版本 ' + version) : ''].filter(Boolean).join(' · ');

      if (stateInstalledCopy) {
        stateInstalledCopy.textContent = detailBits
          ? (detailBits + '。现在可以安装桥接脚本，然后切回桌面端 All API Deck 导入窗口，保持目标站点标签页打开等待自动处理。')
          : '当前浏览器已经识别到 Tampermonkey。现在可以安装桥接脚本，然后切回桌面端 All API Deck 导入窗口，保持目标站点标签页打开等待自动处理。';
      }
      if (stateInstalledHelper) {
        stateInstalledHelper.innerHTML = path
          ? ('<strong>检测位置：</strong> <code>' + escapeHTML(path) + '</code>')
          : '<strong>安装后：</strong> 不需要停留在此页面，直接去目标站点标签页，再回桌面端窗口即可。';
      }
      if (stateInstalledTags) {
        const tags = [
          'Tampermonkey 已检测到',
          '桥接版本 __VERSION__'
        ];
        if (status?.source === 'filesystem') {
          tags.unshift('本机目录命中');
        }
        if (locationLabel) {
          tags.push(locationLabel);
        }
        stateInstalledTags.innerHTML = tags.map(tag => '<span class="tag">' + escapeHTML(tag) + '</span>').join('');
      }
    }

    function applyMissingState(status) {
      updateState('state-missing');
      if (!stateMissingHelper) {
        return;
      }
      const scannedRoots = Array.isArray(status?.scannedRoots) ? status.scannedRoots.filter(Boolean) : [];
      if (scannedRoots.length > 0) {
        stateMissingHelper.innerHTML = '<strong>提示：</strong> 当前未在本机浏览器扩展目录中命中测试版 Tampermonkey。已扫描这些目录：<code>' + escapeHTML(scannedRoots.join('\n')) + '</code>';
        return;
      }
      stateMissingHelper.innerHTML = '<strong>提示：</strong> 当前检测会同时尝试本机浏览器扩展目录和前端资源探测；如果你确认已经安装脚本管理器，也可以直接继续安装桥接脚本。';
    }

    function tryLoadCandidate(url) {
      return new Promise(resolve => {
        const img = new Image();
        img.onload = () => resolve(true);
        img.onerror = () => resolve(false);
        img.src = url + '?t=' + Date.now();
      });
    }

    async function fetchLocalExtensionStatus() {
      try {
        const response = await fetch(extensionStatusUrl + '?t=' + Date.now(), { cache: 'no-store' });
        if (!response.ok) {
          return null;
        }
        return await response.json();
      } catch (error) {
        return null;
      }
    }

    async function checkTampermonkey() {
      updateState('state-checking');
      const localStatus = await fetchLocalExtensionStatus();
      if (localStatus?.installed) {
        applyInstalledState(localStatus, '已从本机桌面端识别');
        return;
      }
      for (const candidate of testCandidates) {
        const ok = await tryLoadCandidate(candidate);
        if (ok) {
          applyInstalledState({ source: 'browser_probe' }, '已从当前浏览器探测到');
          return;
        }
      }
      applyMissingState(localStatus);
    }

    async function testLocalBridgeConnectivity() {
      const trigger = document.getElementById('test-bridge-btn');
      if (trigger) {
        trigger.disabled = true;
        trigger.textContent = '检测中...';
      }
      try {
        const response = await fetch('__PING_URL__?t=' + Date.now(), { cache: 'no-store' });
        if (response.ok) {
          showBridgeToast('联通', '本地桥接服务已可访问，可以继续安装桥接脚本。');
        } else {
          showBridgeToast('未联通', '当前没有连接到本地桥接服务，请先在 All API Deck 中打开当前标签导入窗口。');
        }
      } catch (error) {
        showBridgeToast('未联通', '当前没有连接到本地桥接服务，请先在 All API Deck 中打开当前标签导入窗口。');
      } finally {
        if (trigger) {
          trigger.disabled = false;
          trigger.textContent = '测试本地桥接';
        }
      }
    }

    ['retry-checking-btn', 'retry-missing-btn', 'retry-installed-btn'].forEach(id => {
      const node = document.getElementById(id);
      if (node) {
        node.addEventListener('click', checkTampermonkey);
      }
    });

    document.getElementById('test-bridge-btn')?.addEventListener('click', testLocalBridgeConnectivity);
    document.getElementById('retry-installed-bridge-btn')?.addEventListener('click', testLocalBridgeConnectivity);
    document.getElementById('bridge-toast-close')?.addEventListener('click', hideBridgeToast);
    toastMask?.addEventListener('click', event => {
      if (event.target === toastMask) hideBridgeToast();
    });

    window.addEventListener('load', () => {
      window.setTimeout(checkTampermonkey, 720);
    });
  </script>
</body>
</html>`

	return strings.NewReplacer(
		"__VERSION__", bridgeServerVersion,
		"__PING_URL__", fmt.Sprintf("http://%s:%d/bridge/ping", bridgeServerHost, bridgeServerPort),
		"__EXTENSION_STATUS_URL__", fmt.Sprintf("http://%s:%d/bridge/install/status", bridgeServerHost, bridgeServerPort),
		"__IMPORT_URL__", fmt.Sprintf("http://%s:%d/bridge/import", bridgeServerHost, bridgeServerPort),
		"__SCRIPT_URL__", fmt.Sprintf("http://%s:%d/bridge/script.user.js", bridgeServerHost, bridgeServerPort),
		"__SCRIPT_MANAGER_EXTENSION_ID__", bridgeScriptManagerExtensionID,
		"__SCRIPT_MANAGER_URL__", bridgeScriptManagerInstallURL,
	).Replace(html)
}

func (a *App) handleBridgeUserScript(writer http.ResponseWriter, request *http.Request) {
	appendBridgeImportLogf("[SCRIPT_REQUEST] method=%s remote=%s ua=%s", request.Method, request.RemoteAddr, previewBridgeText(request.UserAgent(), 120))
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := ""
	source := "embedded"
	if diskPath, err := resolveBridgeUserScriptPath(); err == nil && strings.TrimSpace(diskPath) != "" {
		if fileRaw, readErr := os.ReadFile(diskPath); readErr == nil {
			candidate := strings.TrimSpace(string(fileRaw))
			if candidate != "" {
				raw = candidate
				source = "disk"
			} else {
				appendBridgeImportLogf("[SCRIPT_WARN] source=disk path=%s err=empty_disk_script", diskPath)
			}
		} else {
			appendBridgeImportLogf("[SCRIPT_WARN] source=disk path=%s err=%v", diskPath, readErr)
		}
	}

	if raw == "" {
		raw = strings.TrimSpace(embeddedBridgeUserScript)
	}
	if raw == "" {
		appendBridgeImportLogf("[SCRIPT_FAIL] source=%s err=empty_script", source)
		http.Error(writer, "bridge script not found", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	appendBridgeImportLogf("[SCRIPT_OK] source=%s bytes=%d", source, len(raw))
	_, _ = writer.Write([]byte(raw))
}

func writeBridgeJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1")
	writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func extractBridgeRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(remoteAddr)
}

func isLoopbackBridgeRemote(remoteIP string) bool {
	ip := net.ParseIP(strings.TrimSpace(remoteIP))
	return ip != nil && ip.IsLoopback()
}

func appendBridgeHistory(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(raw); err != nil {
		return err
	}
	_, err = file.WriteString("\n")
	return err
}

func resolveBridgeUserScriptPath() (string, error) {
	baseDir := ""
	if root, err := findProjectRoot(); err == nil && strings.TrimSpace(root) != "" {
		baseDir = root
	}
	if baseDir == "" {
		exePath, err := os.Executable()
		if err == nil && strings.TrimSpace(exePath) != "" {
			baseDir = filepath.Dir(exePath)
		}
	}
	if baseDir == "" {
		return "", fmt.Errorf("unable to resolve bridge script base directory")
	}
	return filepath.Join(baseDir, "plugin-bridge-js", "bridge.user.js"), nil
}

func readBridgeImportSnapshot() (*BridgeImportSnapshot, error) {
	snapshot := &BridgeImportSnapshot{
		Records:  []BridgeImportRecord{},
		LastLogs: []string{},
	}

	lastPath := resolveBridgeImportLastPath()
	historyPath := resolveBridgeImportHistoryPath()
	snapshot.LastStoredAt = lastPath
	snapshot.LogPath = resolveBridgeImportLogPath()
	snapshot.ServerURL = fmt.Sprintf("http://%s:%d", bridgeServerHost, bridgeServerPort)
	snapshot.LastLogs = readBridgeLogTail(24)

	raw, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	dedupedRecords := make(map[string]BridgeImportRecord)
	recordOrder := make([]string, 0)
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var envelope map[string]any
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			appendBridgeImportLogf("[SNAPSHOT_SKIP] invalid history line index=%d err=%v", index, err)
			continue
		}

		payload, _ := envelope["payload"].(map[string]any)
		sourceURL := readBridgePayloadString(payload, "source_url", "sourceUrl")
		sourceOrigin := readBridgePayloadString(payload, "source_origin", "sourceOrigin")
		title := readBridgePayloadString(payload, "title")
		if shouldIgnoreBridgeImportSource(sourceURL, sourceOrigin, title) {
			appendBridgeImportLogf(
				"[SNAPSHOT_SKIP] index=%d source=%s title=%s reason=bootstrap_page",
				index,
				previewBridgeText(sourceURL, 160),
				previewBridgeText(title, 96),
			)
			continue
		}

		extracted := extractBridgePayloadMap(payload)
		tokenCount := readBridgePayloadArrayLength(payload, "tokens")
		if ignored, ignoreReason := shouldIgnoreBridgeImportPayload(payload, extracted, tokenCount); ignored {
			appendBridgeImportLogf(
				"[SNAPSHOT_SKIP] index=%d source=%s title=%s reason=%s",
				index,
				previewBridgeText(sourceURL, 160),
				previewBridgeText(title, 96),
				ignoreReason,
			)
			continue
		}

		record := normalizeBridgeImportRecord(envelope, index)
		recordKey := buildBridgeImportRecordDedupKey(record)
		if recordKey == "" {
			recordKey = fmt.Sprintf("record-%d", index)
		}
		if _, exists := dedupedRecords[recordKey]; exists {
			nextOrder := make([]string, 0, len(recordOrder))
			for _, item := range recordOrder {
				if item != recordKey {
					nextOrder = append(nextOrder, item)
				}
			}
			recordOrder = nextOrder
		}
		dedupedRecords[recordKey] = record
		recordOrder = append(recordOrder, recordKey)
	}
	for _, recordKey := range recordOrder {
		record, ok := dedupedRecords[recordKey]
		if !ok {
			continue
		}
		snapshot.Records = append(snapshot.Records, record)
	}

	snapshot.TotalCount = len(snapshot.Records)
	snapshot.ReadyCount = 0
	for _, record := range snapshot.Records {
		if record.Ready {
			snapshot.ReadyCount += 1
		}
	}
	if len(snapshot.Records) > 0 {
		snapshot.LastReceivedAt = snapshot.Records[len(snapshot.Records)-1].ReceivedAt
	}
	return snapshot, nil
}

func normalizeBridgeImportRecord(envelope map[string]any, index int) BridgeImportRecord {
	payload, _ := envelope["payload"].(map[string]any)
	extracted := extractBridgePayloadMap(payload)
	receivedAt := strings.TrimSpace(fmt.Sprint(envelope["receivedAt"]))
	if receivedAt == "" {
		receivedAt = time.Now().Format(time.RFC3339Nano)
	}

	sourceURL := readBridgePayloadString(extracted, "site_url", "siteUrl", "source_url", "sourceUrl")
	if sourceURL == "" {
		sourceURL = readBridgePayloadString(payload, "source_url", "sourceUrl")
	}
	sourceOrigin := readBridgePayloadString(extracted, "storage_origin", "storageOrigin", "source_origin", "sourceOrigin")
	if sourceOrigin == "" {
		sourceOrigin = readBridgePayloadString(payload, "source_origin", "sourceOrigin")
	}

	recordID := strings.TrimSpace(fmt.Sprint(envelope["id"]))
	if recordID == "" {
		recordID = fmt.Sprintf("bridge-%d-%d", index, time.Now().UnixMilli())
	}

	tokenCount := readBridgePayloadArrayLength(payload, "tokens")
	accessToken := readBridgePayloadString(extracted, "resolved_access_token", "resolvedAccessToken", "access_token", "accessToken")
	ready, readyReason := computeBridgeImportReadyReason(payload, extracted, tokenCount)

	return BridgeImportRecord{
		ID:            recordID,
		ReceivedAt:    receivedAt,
		RemoteAddr:    strings.TrimSpace(fmt.Sprint(envelope["remoteAddr"])),
		Type:          strings.TrimSpace(fmt.Sprint(payload["type"])),
		SourceURL:     sourceURL,
		SourceOrigin:  sourceOrigin,
		Title:         readBridgePayloadString(extracted, "site_name", "siteName", "title"),
		UserAgent:     readBridgePayloadString(payload, "user_agent", "userAgent"),
		SiteType:      readBridgePayloadString(extracted, "site_type", "siteType"),
		ResolvedUser:  readBridgePayloadString(extracted, "resolved_user_id", "resolvedUserId", "user_id", "userId"),
		TokenPreview:  maskBridgeTokenPreview(accessToken),
		TokenCount:    tokenCount,
		TokenEndpoint: readBridgePayloadString(extracted, "endpoint"),
		Ready:         ready,
		ReadyReason:   readyReason,
		Payload:       payload,
	}
}

func buildBridgeImportRecordDedupKey(record BridgeImportRecord) string {
	title := strings.TrimSpace(strings.ToLower(record.Title))
	siteType := strings.TrimSpace(strings.ToLower(record.SiteType))
	sourceURL := strings.TrimSpace(strings.ToLower(record.SourceURL))
	sourceURL = strings.TrimRight(sourceURL, "/")
	if siteType == "hub_linux_do" && sourceURL != "" && title != "" {
		return "url-title:" + sourceURL + "|" + title
	}
	if sourceURL != "" {
		return "url:" + sourceURL
	}

	sourceOrigin := strings.TrimSpace(strings.ToLower(record.SourceOrigin))
	sourceOrigin = strings.TrimRight(sourceOrigin, "/")
	if siteType == "hub_linux_do" && sourceOrigin != "" && title != "" {
		return "origin-title:" + sourceOrigin + "|" + title
	}
	if sourceOrigin != "" {
		return "origin:" + sourceOrigin
	}

	if title != "" {
		return "title:" + title
	}

	recordID := strings.TrimSpace(record.ID)
	if recordID != "" {
		return "id:" + recordID
	}
	return ""
}

func readBridgePayloadString(payload map[string]any, keys ...string) string {
	if len(payload) == 0 {
		return ""
	}

	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(payload[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}

	extracted := extractBridgePayloadMap(payload)
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(extracted[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}

	nested, _ := payload["data"].(map[string]any)
	for _, key := range keys {
		value := strings.TrimSpace(fmt.Sprint(nested[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}

	return ""
}
