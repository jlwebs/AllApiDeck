package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type fetchKeysProgressState struct {
	mu            sync.Mutex
	Active        bool   `json:"active"`
	Stage         string `json:"stage"`
	Detail        string `json:"detail"`
	Total         int    `json:"total"`
	Completed     int    `json:"completed"`
	SuccessSites  int    `json:"successSites"`
	LastSiteName  string `json:"lastSiteName"`
	StartedAt     int64  `json:"startedAt"`
	LastUpdatedAt int64  `json:"lastUpdatedAt"`
}

type fetchKeysProgressSnapshot struct {
	Active        bool   `json:"active"`
	Stage         string `json:"stage"`
	Detail        string `json:"detail"`
	Total         int    `json:"total"`
	Completed     int    `json:"completed"`
	SuccessSites  int    `json:"successSites"`
	LastSiteName  string `json:"lastSiteName"`
	StartedAt     int64  `json:"startedAt"`
	LastUpdatedAt int64  `json:"lastUpdatedAt"`
}

var localFetchKeysProgress fetchKeysProgressState

func (a *App) PerformHttpRequestRaw(payloadJSON string) string {
	type rawBridgeRequest struct {
		Method    string            `json:"method"`
		URL       string            `json:"url"`
		Headers   map[string]string `json:"headers"`
		Body      string            `json:"body"`
		TimeoutMs int               `json:"timeoutMs"`
	}

	request := rawBridgeRequest{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(payloadJSON)), &request); err != nil {
		appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("error raw payload decode | %v", err))
		data, _ := json.Marshal(&bridgeHTTPResponse{
			Status: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: fmt.Sprintf(`{"message":"invalid bridge payload: %s"}`, escapeJSONText(err.Error())),
		})
		return string(data)
	}

	result, err := a.performHTTPRequest(request.Method, request.URL, request.Headers, request.Body, request.TimeoutMs)
	if err != nil {
		appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("error raw request | %v", err))
		data, _ := json.Marshal(&bridgeHTTPResponse{
			Status: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: fmt.Sprintf(`{"message":"bridge request failed: %s"}`, escapeJSONText(err.Error())),
		})
		return string(data)
	}
	return result
}

func (a *App) PerformHttpRequest(method string, targetURL string, headersJSON string, body string, timeoutMs int) (string, error) {
	return a.performHTTPRequestWithJSONHeaders(method, targetURL, headersJSON, body, timeoutMs)
}

func (a *App) GetChromeProfileExtractProgress() fetchKeysProgressSnapshot {
	return localFetchKeysProgressSnapshot()
}

func (a *App) OpenDesktopProfileAssist(sites []desktopProfileAssistOpenRequest) map[string]any {
	results := make([]*desktopProfileAssistWindowResult, 0, len(sites))
	openErrors := make([]string, 0, len(sites))
	opened := 0
	if len(sites) > 0 {
		appendLine(filepath.Join(resolveRuntimeLogDir(), "profile-assist.log"), fmt.Sprintf("[ASSIST] api open | sites=%d", len(sites)))
	}

	for _, site := range sites {
		result, err := openDesktopProfileAssistWindow(site)
		if err != nil {
			messageText := fmt.Sprintf("%s(%s): %v", strings.TrimSpace(site.SiteName), strings.TrimSpace(site.SiteURL), err)
			openErrors = append(openErrors, strings.TrimSpace(messageText))
			appendLine(filepath.Join(resolveRuntimeLogDir(), "profile-assist.log"), fmt.Sprintf("[ASSIST] open failed | %s", messageText))
			continue
		}
		opened++
		results = append(results, result)
	}

	return map[string]any{
		"success": opened > 0,
		"opened":  opened,
		"results": results,
		"errors":  openErrors,
	}
}

func (a *App) CloseDesktopProfileAssist(hosts []string) map[string]any {
	if len(hosts) > 0 {
		appendLine(filepath.Join(resolveRuntimeLogDir(), "profile-assist.log"), fmt.Sprintf("[ASSIST] api close | hosts=%v", hosts))
	}
	closed := closeProfileAssistWindowsByHosts(hosts)
	return map[string]any{
		"success": closed > 0,
		"closed":  closed,
	}
}

func (a *App) performHTTPRequestWithJSONHeaders(method string, targetURL string, headersJSON string, body string, timeoutMs int) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return "", fmt.Errorf("request url is empty")
	}
	headers := map[string]string{}
	if strings.TrimSpace(headersJSON) != "" {
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("error %s %s | invalid headers json: %v", method, targetURL, err))
			return "", fmt.Errorf("invalid headers json: %w", err)
		}
	}

	return a.performHTTPRequest(method, targetURL, headers, body, timeoutMs)
}

func (a *App) performHTTPRequest(method string, targetURL string, headers map[string]string, body string, timeoutMs int) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}

	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return "", fmt.Errorf("request url is empty")
	}
	appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("request %s %s", method, targetURL))

	if strings.HasPrefix(targetURL, "/api/") {
		resp, err := a.handleLocalAPIRequest(method, targetURL, headers, body)
		if err != nil {
			appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("error %s %s | %v", method, targetURL, err))
			return "", err
		}
		appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("response %s %s | %d", method, targetURL, resp.Status))
		return encodeBridgeResponse(resp)
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("invalid request url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported request url: %s", targetURL)
	}

	timeout := time.Duration(clampInt(timeoutMs, 0, 180000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	resp, err := performExternalHTTPRequest(method, targetURL, headers, body, timeout)
	if err != nil {
		appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("error %s %s | %v", method, targetURL, err))
		return "", err
	}
	appendLine(filepath.Join(resolveRuntimeLogDir(), "bridge-http.log"), fmt.Sprintf("response %s %s | %d", method, targetURL, resp.Status))
	return encodeBridgeResponse(resp)
}

func (a *App) handleLocalAPIRequest(method string, rawURL string, headers map[string]string, body string) (*bridgeHTTPResponse, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"message": err.Error()}), nil
	}

	switch parsed.Path {
	case "/api/alive":
		return jsonBridgeResponse(http.StatusOK, map[string]any{
			"ok":        true,
			"mode":      "wails-bridge",
			"timestamp": time.Now().UnixMilli(),
			"runtime":   "desktop",
		}), nil
	case "/api/proxy-get":
		return handleLocalProxyGet(parsed, headers)
	case "/api/check-key":
		return handleLocalCheckKey(method, headers, body)
	case "/api/fetch-keys":
		return handleLocalFetchKeys(method, body)
	case "/api/fetch-keys/progress":
		return jsonBridgeResponse(http.StatusOK, localFetchKeysProgressSnapshot()), nil
	case "/api/profile-assist/open":
		return handleLocalProfileAssistOpen(method, body)
	case "/api/profile-assist/close":
		return handleLocalProfileAssistClose(method, body)
	case "/api/clear-logs":
		return handleLocalClearLogs(method, parsed)
	case "/api/browser-session/browsers":
		return jsonBridgeResponse(http.StatusOK, detectInstalledBrowsersGo()), nil
	case "/api/browser-session/status":
		query := parsed.Query()
		browserType := normalizeBrowserType(query.Get("browserType"))
		return jsonBridgeResponse(http.StatusOK, map[string]any{
			"success":     true,
			"browserType": browserType,
			"running":     isBrowserProcessRunningGo(browserType),
			"attached":    false,
			"launching":   false,
			"managed":     false,
		}), nil
	default:
		if strings.HasPrefix(parsed.Path, "/api/browser-session/") {
			return jsonBridgeResponse(http.StatusNotImplemented, map[string]any{
				"success": false,
				"code":    "BROWSER_SESSION_UNAVAILABLE",
				"message": "当前桌面 release 未内置受控浏览器模式，请切换到“Profile 文件模式”继续使用。",
			}), nil
		}
		return jsonBridgeResponse(http.StatusNotFound, map[string]any{"message": "Not Found"}), nil
	}
}

func handleLocalProxyGet(parsed *url.URL, headers map[string]string) (*bridgeHTTPResponse, error) {
	targetURL := strings.TrimSpace(parsed.Query().Get("url"))
	if targetURL == "" {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"message": "missing url"}), nil
	}

	baseURL := ""
	if targetParsed, err := url.Parse(targetURL); err == nil {
		baseURL = targetParsed.Scheme + "://" + targetParsed.Host
	}

	reqHeaders := map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) BatchApiCheck/1.0",
		"X-Requested-With": "XMLHttpRequest",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"Cache-Control":    "no-cache",
		"Pragma":           "no-cache",
	}
	if baseURL != "" {
		reqHeaders["Referer"] = baseURL + "/"
	}
	if auth := getHeaderIgnoreCase(headers, "Authorization"); auth != "" {
		reqHeaders["Authorization"] = auth
	}
	for key, value := range buildCompatHeaders(parsed.Query().Get("uid")) {
		reqHeaders[key] = value
	}

	resp, err := performExternalHTTPRequest(http.MethodGet, targetURL, reqHeaders, "", 15*time.Second)
	if err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[PROXY-GET] error: %s | %v", targetURL, err))
		return jsonBridgeResponse(http.StatusInternalServerError, map[string]any{"message": err.Error()}), nil
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[PROXY-GET] %d %s", resp.Status, targetURL))
	return resp, nil
}

func handleLocalCheckKey(method string, headers map[string]string, body string) (*bridgeHTTPResponse, error) {
	if method != http.MethodPost {
		return &bridgeHTTPResponse{Status: http.StatusMethodNotAllowed, Body: ""}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid json body"}}), nil
	}

	if truthy(payload["_isFirst"]) {
		_ = os.WriteFile(resolveCheckLogPath(), []byte{}, 0o644)
	}

	normalized, err := normalizeCheckKeyPayload(payload)
	if err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"error": map[string]any{"message": err.Error()}}), nil
	}

	status, responseBody := executeCheckKeySmart(normalized)
	resolvedEndpoint := ""
	if bodyMap, ok := responseBody["diagnostics"].(map[string]any); ok {
		resolvedEndpoint = strings.TrimSpace(toStringValue(bodyMap["resolvedEndpoint"]))
	} else if errorMap, ok := responseBody["error"].(map[string]any); ok {
		if diagnostics, ok := errorMap["diagnostics"].(map[string]any); ok {
			resolvedEndpoint = strings.TrimSpace(toStringValue(diagnostics["resolvedEndpoint"]))
		}
	}
	if resolvedEndpoint != "" {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] %s | %s | status=%d | endpoint=%s", normalized.URL, normalized.Model, status, resolvedEndpoint))
	} else {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] %s | %s | status=%d", normalized.URL, normalized.Model, status))
	}
	return jsonBridgeResponse(status, responseBody), nil
}

func handleLocalFetchKeys(method string, body string) (*bridgeHTTPResponse, error) {
	if method != http.MethodPost {
		return &bridgeHTTPResponse{Status: http.StatusMethodNotAllowed, Body: ""}, nil
	}

	var request struct {
		Accounts []ChromeProfileAccount `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"message": "invalid json body"}), nil
	}
	if len(request.Accounts) == 0 {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"message": "accounts cannot be empty"}), nil
	}

	localFetchKeysProgressReset(len(request.Accounts))
	appendLine(resolveFetchLogPath(), fmt.Sprintf("[BATCH] start fetch accounts=%d", len(request.Accounts)))

	results := make([]ChromeProfileTokenResult, len(request.Accounts))
	workerCount := minInt(10, len(request.Accounts))
	indexCh := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				result := fetchTokensForAccountViaAccessToken(request.Accounts[idx])
				results[idx] = result
				localFetchKeysProgressMark(result)
			}
		}()
	}

	for idx := range request.Accounts {
		indexCh <- idx
	}
	close(indexCh)
	wg.Wait()
	localFetchKeysProgressFinish()

	successSites := 0
	for _, item := range results {
		if len(item.Tokens) > 0 {
			successSites++
		}
	}
	appendLine(resolveFetchLogPath(), fmt.Sprintf("[BATCH] complete successSites=%d/%d", successSites, len(results)))
	return jsonBridgeResponse(http.StatusOK, map[string]any{"results": results}), nil
}

func handleLocalClearLogs(method string, parsed *url.URL) (*bridgeHTTPResponse, error) {
	if method != http.MethodPost {
		return &bridgeHTTPResponse{Status: http.StatusMethodNotAllowed, Body: ""}, nil
	}

	logType := strings.TrimSpace(parsed.Query().Get("type"))
	switch logType {
	case "fetch":
		if err := os.WriteFile(resolveFetchLogPath(), []byte{}, 0o644); err != nil {
			return jsonBridgeResponse(http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()}), nil
		}
	case "check":
		if err := os.WriteFile(resolveCheckLogPath(), []byte{}, 0o644); err != nil {
			return jsonBridgeResponse(http.StatusInternalServerError, map[string]any{"success": false, "message": err.Error()}), nil
		}
	default:
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{"success": false, "message": "unsupported log type"}), nil
	}

	return jsonBridgeResponse(http.StatusOK, map[string]any{"success": true}), nil
}

func handleLocalProfileAssistOpen(method string, body string) (*bridgeHTTPResponse, error) {
	if method != http.MethodPost {
		return &bridgeHTTPResponse{Status: http.StatusMethodNotAllowed, Body: ""}, nil
	}

	var request struct {
		Sites []desktopProfileAssistOpenRequest `json:"sites"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "invalid json body",
		}), nil
	}
	if len(request.Sites) == 0 {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "sites cannot be empty",
		}), nil
	}

	payload := (&App{}).OpenDesktopProfileAssist(request.Sites)

	status := http.StatusOK
	success, _ := payload["success"].(bool)
	if !success {
		status = http.StatusInternalServerError
	}

	return jsonBridgeResponse(status, payload), nil
}

func handleLocalProfileAssistClose(method string, body string) (*bridgeHTTPResponse, error) {
	if method != http.MethodPost {
		return &bridgeHTTPResponse{Status: http.StatusMethodNotAllowed, Body: ""}, nil
	}

	var request struct {
		Hosts []string `json:"hosts"`
		Sites []string `json:"sites"`
	}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "invalid json body",
		}), nil
	}
	hosts := request.Hosts
	if len(hosts) == 0 {
		hosts = request.Sites
	}
	if len(hosts) == 0 {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "hosts cannot be empty",
		}), nil
	}

	payload := (&App{}).CloseDesktopProfileAssist(hosts)
	return jsonBridgeResponse(http.StatusOK, payload), nil
}

type normalizedCheckKeyPayload struct {
	URL       string
	Key       string
	Model     string
	UID       string
	SiteType  string
	TimeoutMs int
	Messages  any
}

type checkEndpointAttempt struct {
	Endpoint  string `json:"endpoint"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type checkExecutionResult struct {
	ok        bool
	endpoint  string
	status    int
	message   string
	retryable bool
	attempt   *checkEndpointAttempt
	body      map[string]any
}

type checkEndpointPhase struct {
	protocol  string
	endpoints []string
}

type checkProtocolPreferenceState struct {
	mu     sync.Mutex
	loaded bool
	values map[string]int
}

type sseCompletionParseResult struct {
	ReturnedModel    string
	Content          string
	ReasoningContent string
	Usage            any
	ChunkCount       int
	TTFTMs           int64
	HasTTFT          bool
}

const (
	checkProtocolPreferCompletions = 0
	checkProtocolPreferResponses   = 1
)

var localCheckProtocolPreferences checkProtocolPreferenceState

var checkEndpointStripPatterns = []*regexp.Regexp{
	regexp.MustCompile(`/v\d+/chat/completions$`),
	regexp.MustCompile(`/chat/completions$`),
	regexp.MustCompile(`/v\d+/responses$`),
	regexp.MustCompile(`/responses$`),
	regexp.MustCompile(`/v\d+/messages$`),
	regexp.MustCompile(`/messages$`),
	regexp.MustCompile(`/api/user/models$`),
	regexp.MustCompile(`/api/models$`),
	regexp.MustCompile(`/api/v\d+/models$`),
	regexp.MustCompile(`/v\d+/models$`),
	regexp.MustCompile(`/models$`),
	regexp.MustCompile(`/api/v\d+$`),
	regexp.MustCompile(`/v\d+$`),
	regexp.MustCompile(`/api$`),
}

func normalizeCheckKeyPayload(payload map[string]any) (normalizedCheckKeyPayload, error) {
	normalized := normalizedCheckKeyPayload{
		URL:       strings.TrimRight(strings.TrimSpace(toStringValue(payload["url"])), "/"),
		Key:       strings.TrimSpace(toStringValue(payload["key"])),
		Model:     strings.TrimSpace(toStringValue(payload["model"])),
		UID:       strings.TrimSpace(toStringValue(payload["uid"])),
		SiteType:  strings.ToLower(strings.TrimSpace(toStringValue(payload["siteType"]))),
		TimeoutMs: int(toFloat64OrZero(payload["timeoutMs"])),
		Messages:  payload["messages"],
	}

	if normalized.TimeoutMs <= 0 {
		normalized.TimeoutMs = 55000
	} else {
		normalized.TimeoutMs = clampInt(normalized.TimeoutMs, 5000, 180000)
	}

	if (normalized.URL == "" || normalized.Key == "") && payload["site"] != nil {
		siteMap, ok := payload["site"].(map[string]any)
		if !ok {
			return normalized, fmt.Errorf("invalid legacy site payload")
		}
		normalized.URL = strings.TrimRight(strings.TrimSpace(toStringValue(siteMap["site_url"])), "/")
		if rawAPI := strings.TrimSpace(toStringValue(siteMap["api_key"])); strings.HasPrefix(strings.ToLower(rawAPI), "http") {
			normalized.URL = strings.TrimRight(rawAPI, "/")
		}
		normalized.Key = strings.TrimSpace(toStringValue(payload["tokenKey"]))
		normalized.Model = strings.TrimSpace(toStringValue(payload["model"]))
		if accountInfo, ok := siteMap["account_info"].(map[string]any); ok && normalized.UID == "" {
			normalized.UID = strings.TrimSpace(toStringValue(accountInfo["id"]))
		}
		if normalized.SiteType == "" {
			normalized.SiteType = strings.ToLower(strings.TrimSpace(toStringValue(siteMap["site_type"])))
		}
		if normalized.Messages == nil {
			normalized.Messages = payload["messages"]
		}
	}

	if normalized.URL == "" || normalized.Key == "" || normalized.Model == "" {
		return normalized, fmt.Errorf("url, key and model are required")
	}

	return normalized, nil
}

func normalizeCheckEndpointInput(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func stripKnownCheckEndpointSuffix(input string) string {
	normalized := normalizeCheckEndpointInput(input)
	for _, pattern := range checkEndpointStripPatterns {
		if pattern.MatchString(normalized) {
			return pattern.ReplaceAllString(normalized, "")
		}
	}
	return normalized
}

func addCheckEndpointCandidate(candidates *[]string, seen map[string]struct{}, candidate string) {
	normalized := normalizeCheckEndpointInput(candidate)
	if normalized == "" {
		return
	}
	if _, exists := seen[normalized]; exists {
		return
	}
	seen[normalized] = struct{}{}
	*candidates = append(*candidates, normalized)
}

func addAnthropicCheckEndpointCandidates(candidates *[]string, seen map[string]struct{}, base string) {
	lowerBase := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lowerBase, "/messages"):
		addCheckEndpointCandidate(candidates, seen, base)
	case regexp.MustCompile(`/api/v\d+$`).MatchString(lowerBase) || regexp.MustCompile(`/v\d+$`).MatchString(lowerBase):
		addCheckEndpointCandidate(candidates, seen, base+"/messages")
	case strings.HasSuffix(lowerBase, "/api"):
		addCheckEndpointCandidate(candidates, seen, base+"/v1/messages")
		addCheckEndpointCandidate(candidates, seen, base+"/messages")
	default:
		addCheckEndpointCandidate(candidates, seen, base+"/v1/messages")
		addCheckEndpointCandidate(candidates, seen, base+"/messages")
	}
}

func isAnyrouterCheckPayload(payload normalizedCheckKeyPayload) bool {
	if strings.EqualFold(strings.TrimSpace(payload.SiteType), "anyrouter") {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(payload.URL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "anyrouter.top" || strings.HasSuffix(host, ".anyrouter.top")
}

func shouldTryResponsesCheck(payload normalizedCheckKeyPayload) bool {
	hostKey := resolveCheckProtocolPreferenceHostKey(payload.URL)
	if hostKey != "" {
		return true
	}
	return isAnyrouterCheckPayload(payload)
}

func shouldPreferAnthropicFallback(payload normalizedCheckKeyPayload) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(payload.Model)), "claude")
}

func shouldTryAnthropicCheck(payload normalizedCheckKeyPayload) bool {
	if shouldPreferAnthropicFallback(payload) {
		return true
	}
	if isAnyrouterCheckPayload(payload) {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(payload.URL))
	if err == nil && strings.EqualFold(strings.TrimSpace(parsed.Hostname()), "api.anthropic.com") {
		return true
	}
	return false
}

func buildOpenAIChatCheckEndpointCandidates(raw string) []string {
	input := normalizeCheckEndpointInput(raw)
	if input == "" {
		return nil
	}

	if parsed, err := url.Parse(input); err == nil {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if host == "anyrouter.top" || strings.HasSuffix(host, ".anyrouter.top") {
			return []string{strings.TrimRight(input, "/") + "/v1/chat/completions"}
		}
	}

	bases := []string{input}
	stripped := stripKnownCheckEndpointSuffix(input)
	if stripped != "" && stripped != input {
		bases = append(bases, stripped)
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, 6)

	for _, base := range bases {
		lowerBase := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lowerBase, "/chat/completions"):
			addCheckEndpointCandidate(&candidates, seen, base)
		case regexp.MustCompile(`/api/v\d+$`).MatchString(lowerBase) || regexp.MustCompile(`/v\d+$`).MatchString(lowerBase):
			addCheckEndpointCandidate(&candidates, seen, base+"/chat/completions")
		case strings.HasSuffix(lowerBase, "/api"):
			addCheckEndpointCandidate(&candidates, seen, base+"/v1/chat/completions")
			addCheckEndpointCandidate(&candidates, seen, base+"/chat/completions")
		default:
			addCheckEndpointCandidate(&candidates, seen, base+"/v1/chat/completions")
			addCheckEndpointCandidate(&candidates, seen, base+"/chat/completions")
			addCheckEndpointCandidate(&candidates, seen, base+"/api/v1/chat/completions")
		}
	}

	return candidates
}

func buildResponsesCheckCandidates(payload normalizedCheckKeyPayload) []string {
	input := normalizeCheckEndpointInput(payload.URL)
	if input == "" {
		return nil
	}

	bases := []string{input}
	stripped := stripKnownCheckEndpointSuffix(input)
	if stripped != "" && stripped != input {
		bases = append(bases, stripped)
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, 4)
	for _, base := range bases {
		for _, candidate := range buildResponsesEndpointCandidates(base) {
			addCheckEndpointCandidate(&candidates, seen, candidate)
		}
	}
	return candidates
}

func buildAnthropicCheckCandidates(payload normalizedCheckKeyPayload) []string {
	if !shouldTryAnthropicCheck(payload) {
		return nil
	}

	input := normalizeCheckEndpointInput(payload.URL)
	if input == "" {
		return nil
	}

	bases := []string{input}
	stripped := stripKnownCheckEndpointSuffix(input)
	if stripped != "" && stripped != input {
		bases = append(bases, stripped)
	}

	if isAnyrouterCheckPayload(payload) {
		base := input
		if stripped != "" {
			base = stripped
		}
		return []string{normalizeCheckEndpointInput(base + "/v1/messages")}
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, 2)
	for _, base := range bases {
		addAnthropicCheckEndpointCandidates(&candidates, seen, base)
	}
	return candidates
}

func buildCheckEndpointPhases(payload normalizedCheckKeyPayload) []checkEndpointPhase {
	openAICandidates := buildOpenAIChatCheckEndpointCandidates(payload.URL)
	responsesCandidates := buildResponsesCheckCandidates(payload)
	anthropicCandidates := buildAnthropicCheckCandidates(payload)

	phases := make([]checkEndpointPhase, 0, 3)
	preferResponses := getCheckProtocolPreference(payload) == checkProtocolPreferResponses
	preferAnthropicFallback := shouldPreferAnthropicFallback(payload)

	addPhase := func(protocol string, endpoints []string) {
		if len(endpoints) == 0 {
			return
		}
		phases = append(phases, checkEndpointPhase{
			protocol:  protocol,
			endpoints: endpoints,
		})
	}

	if preferResponses {
		addPhase("responses", responsesCandidates)
		addPhase("chat", openAICandidates)
		addPhase("messages", anthropicCandidates)
	} else {
		addPhase("chat", openAICandidates)
		if preferAnthropicFallback {
			addPhase("messages", anthropicCandidates)
			addPhase("responses", responsesCandidates)
		} else {
			addPhase("responses", responsesCandidates)
			addPhase("messages", anthropicCandidates)
		}
	}

	return phases
}

func buildCheckEndpointCandidates(payload normalizedCheckKeyPayload) []string {
	phases := buildCheckEndpointPhases(payload)
	candidates := make([]string, 0, 6)
	for _, phase := range phases {
		candidates = append(candidates, phase.endpoints...)
	}
	return candidates
}

func shouldContinueCheckPhaseAfterFailure(result checkExecutionResult) bool {
	endpoint := strings.ToLower(strings.TrimSpace(result.endpoint))
	message := strings.ToLower(strings.TrimSpace(result.message))

	switch {
	case strings.HasSuffix(endpoint, "/chat/completions"):
		if strings.Contains(message, "stream response did not contain valid chunks") {
			return true
		}
		if result.status == http.StatusNotFound || result.status == http.StatusMethodNotAllowed {
			return true
		}
	case strings.HasSuffix(endpoint, "/responses"):
		if result.status == http.StatusNotFound || result.status == http.StatusMethodNotAllowed {
			return true
		}
		if strings.Contains(message, "不支持所选模型") || strings.Contains(message, "does not support selected model") {
			return true
		}
		if strings.Contains(message, "invalid json") || strings.Contains(message, "(html)") {
			return true
		}
	case strings.HasSuffix(endpoint, "/messages"):
		if result.status == http.StatusNotFound || result.status == http.StatusMethodNotAllowed {
			return true
		}
	}

	if strings.HasSuffix(endpoint, "/messages") {
		return false
	}
	if strings.Contains(message, "stream response did not contain valid chunks") {
		return true
	}
	if result.status == http.StatusNotFound || result.status == http.StatusMethodNotAllowed {
		return true
	}

	return false
}

func shouldAdvanceCheckPhase(payload normalizedCheckKeyPayload, currentProtocol string, lastFailure *checkExecutionResult, nextProtocol string) bool {
	if currentProtocol == "" || nextProtocol == "" || lastFailure == nil {
		return false
	}

	if currentProtocol == "chat" && nextProtocol == "responses" {
		return true
	}

	if currentProtocol == "chat" && nextProtocol == "messages" {
		return shouldTryAnthropicCheck(payload)
	}

	if currentProtocol == "messages" && nextProtocol == "responses" {
		return shouldTryResponsesCheck(payload)
	}

	if currentProtocol == "responses" && nextProtocol == "chat" {
		return shouldContinueCheckPhaseAfterFailure(*lastFailure)
	}

	if nextProtocol != "messages" || !shouldTryAnthropicCheck(payload) {
		return false
	}

	endpoint := strings.ToLower(strings.TrimSpace(lastFailure.endpoint))
	message := strings.ToLower(strings.TrimSpace(lastFailure.message))
	if strings.HasSuffix(endpoint, "/messages") {
		return false
	}
	if strings.Contains(message, "stream response did not contain valid chunks") {
		return true
	}
	if lastFailure.status == http.StatusNotFound || lastFailure.status == http.StatusMethodNotAllowed {
		return true
	}
	if currentProtocol == "responses" {
		if strings.Contains(message, "不支持所选模型") || strings.Contains(message, "does not support selected model") {
			return true
		}
		if strings.Contains(message, "invalid json") || strings.Contains(message, "(html)") {
			return true
		}
	}
	return false
}

func isAnyrouterClaude1MErrorMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(text, "1m 上下文") || strings.Contains(text, "请启用 1m 上下文")
}

func buildAnyrouterClaudeUpgradeHint(model string) string {
	modelText := strings.TrimSpace(model)
	if modelText == "" {
		return "Any Router 的 Claude 协议要求 1m 上下文；请改用 Opus 4.7 1m 后再测"
	}
	return fmt.Sprintf("Any Router 的 Claude 协议要求 1m 上下文；当前模型 %s 不可直接测活，请改用 Opus 4.7 1m 后再测", modelText)
}

func extractCheckErrorMessage(payload map[string]any, fallback string) string {
	return firstNonEmpty(
		getNestedString(payload, "error", "message"),
		strings.TrimSpace(toStringValue(payload["message"])),
		strings.TrimSpace(toStringValue(payload["error"])),
		fallback,
	)
}

func isRetryableCheckStatus(status int) bool {
	return status == http.StatusNotFound || status == http.StatusMethodNotAllowed
}

func buildCheckDiagnostics(payload normalizedCheckKeyPayload, attempts []checkEndpointAttempt) map[string]any {
	items := make([]map[string]any, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, map[string]any{
			"endpoint":  attempt.Endpoint,
			"status":    attempt.Status,
			"message":   attempt.Message,
			"retryable": attempt.Retryable,
		})
	}

	return map[string]any{
		"inputUrl":  payload.URL,
		"model":     payload.Model,
		"timeoutMs": payload.TimeoutMs,
		"attempts":  items,
	}
}

func resolveCheckProtocolPreferenceHostKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Host))
}

func buildCheckProtocolPreferenceKeyFingerprint(rawKey string) string {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func resolveCheckProtocolPreferenceScopeKey(payload normalizedCheckKeyPayload) string {
	hostKey := resolveCheckProtocolPreferenceHostKey(payload.URL)
	if hostKey == "" {
		return ""
	}

	keyFingerprint := buildCheckProtocolPreferenceKeyFingerprint(payload.Key)
	modelKey := strings.ToLower(strings.TrimSpace(payload.Model))
	if keyFingerprint == "" || modelKey == "" {
		return ""
	}

	return fmt.Sprintf(
		"host=%s&key=%s&model=%s",
		url.QueryEscape(hostKey),
		url.QueryEscape(keyFingerprint),
		url.QueryEscape(modelKey),
	)
}

func resolveCheckProtocolPreferencePath() string {
	return filepath.Join(resolveRuntimeRootDir(), "check-protocol-preferences.json")
}

func loadCheckProtocolPreferencesLocked() {
	if localCheckProtocolPreferences.loaded {
		return
	}
	localCheckProtocolPreferences.loaded = true
	localCheckProtocolPreferences.values = map[string]int{}

	raw, err := os.ReadFile(resolveCheckProtocolPreferencePath())
	if err != nil {
		return
	}

	var decoded map[string]int
	if err := json.Unmarshal(raw, &decoded); err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] protocol-preference decode failed | %v", err))
		return
	}

	for hostKey, value := range decoded {
		if strings.TrimSpace(hostKey) == "" {
			continue
		}
		switch value {
		case checkProtocolPreferCompletions, checkProtocolPreferResponses:
			localCheckProtocolPreferences.values[strings.ToLower(strings.TrimSpace(hostKey))] = value
		}
	}
}

func getCheckProtocolPreference(payload normalizedCheckKeyPayload) int {
	scopeKey := resolveCheckProtocolPreferenceScopeKey(payload)
	if scopeKey == "" {
		return checkProtocolPreferCompletions
	}

	localCheckProtocolPreferences.mu.Lock()
	defer localCheckProtocolPreferences.mu.Unlock()
	loadCheckProtocolPreferencesLocked()

	if value, ok := localCheckProtocolPreferences.values[scopeKey]; ok {
		return value
	}
	return checkProtocolPreferCompletions
}

func setCheckProtocolPreference(payload normalizedCheckKeyPayload, value int) {
	scopeKey := resolveCheckProtocolPreferenceScopeKey(payload)
	if scopeKey == "" {
		return
	}
	if value != checkProtocolPreferResponses {
		value = checkProtocolPreferCompletions
	}

	localCheckProtocolPreferences.mu.Lock()
	loadCheckProtocolPreferencesLocked()
	current := localCheckProtocolPreferences.values[scopeKey]
	if current == value {
		localCheckProtocolPreferences.mu.Unlock()
		return
	}
	localCheckProtocolPreferences.values[scopeKey] = value
	snapshot := make(map[string]int, len(localCheckProtocolPreferences.values))
	for key, item := range localCheckProtocolPreferences.values {
		snapshot[key] = item
	}
	localCheckProtocolPreferences.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] protocol-preference encode failed | scope=%s | %v", scopeKey, err))
		return
	}
	if err := os.MkdirAll(filepath.Dir(resolveCheckProtocolPreferencePath()), 0o755); err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] protocol-preference mkdir failed | scope=%s | %v", scopeKey, err))
		return
	}
	if err := os.WriteFile(resolveCheckProtocolPreferencePath(), data, 0o644); err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] protocol-preference write failed | scope=%s | %v", scopeKey, err))
		return
	}

	protocolName := "chat"
	if value == checkProtocolPreferResponses {
		protocolName = "responses"
	}
	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] protocol-preference saved | scope=%s | prefer=%s", scopeKey, protocolName))
}

func resetCheckProtocolPreferencesForTests() {
	localCheckProtocolPreferences.mu.Lock()
	defer localCheckProtocolPreferences.mu.Unlock()
	localCheckProtocolPreferences.loaded = false
	localCheckProtocolPreferences.values = nil
}

func executeCheckKey(payload normalizedCheckKeyPayload) (int, map[string]any) {
	targetURL := payload.URL + "/v1/chat/completions"
	requestBody := map[string]any{
		"model":    payload.Model,
		"messages": payload.Messages,
		"stream":   true,
	}
	if requestBody["messages"] == nil {
		requestBody["messages"] = []map[string]any{{"role": "user", "content": "hi"}}
	}

	bodyBytes, _ := json.Marshal(requestBody)
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return http.StatusBadRequest, map[string]any{"error": map[string]any{"message": err.Error()}}
	}

	req.Header.Set("Authorization", "Bearer "+payload.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 BatchApiCheck/1.0")
	for key, value := range buildCompatHeaders(payload.UID) {
		req.Header.Set(key, value)
	}

	client, err := newOutboundHTTPClient(time.Duration(payload.TimeoutMs) * time.Millisecond)
	if err != nil {
		return http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}}
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return http.StatusGatewayTimeout, map[string]any{"error": map[string]any{"message": fmt.Sprintf("请求超时 (%ds)", payload.TimeoutMs/1000)}}
		}
		return http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}}
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawBody, _ := io.ReadAll(resp.Body)
		duration := time.Since(start).Round(10 * time.Millisecond)
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if strings.Contains(contentType, "json") {
			var payload map[string]any
			if err := json.Unmarshal(rawBody, &payload); err == nil {
				errMessage = firstNonEmpty(
					getNestedString(payload, "error", "message"),
					strings.TrimSpace(toStringValue(payload["message"])),
					errMessage,
				)
			}
		} else if title := extractHTMLTitle(string(rawBody)); title != "" {
			errMessage = "(HTML) " + title
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | %s | %s", payload.URL, payload.Model, errMessage, duration))
		return resp.StatusCode, map[string]any{"error": map[string]any{"message": errMessage}}
	}

	if strings.Contains(contentType, "json") {
		rawBody, _ := io.ReadAll(resp.Body)
		duration := time.Since(start).Round(10 * time.Millisecond)
		var responsePayload map[string]any
		if err := json.Unmarshal(rawBody, &responsePayload); err != nil {
			return http.StatusBadGateway, map[string]any{"error": map[string]any{"message": "Invalid JSON"}}
		}
		if _, ok := responsePayload["choices"]; ok {
			appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(json) %s | %s | %s", payload.URL, payload.Model, duration))
			return http.StatusOK, map[string]any{
				"model":   firstNonEmpty(strings.TrimSpace(toStringValue(responsePayload["model"])), payload.Model),
				"choices": responsePayload["choices"],
				"usage":   responsePayload["usage"],
				"message": "success",
			}
		}
		return http.StatusBadRequest, map[string]any{"error": map[string]any{"message": firstNonEmpty(getNestedString(responsePayload, "error", "message"), strings.TrimSpace(toStringValue(responsePayload["message"])), "Unknown error")}}
	}

	parseResult := parseSSECompletionStream(resp.Body, start)
	duration := time.Since(start).Round(10 * time.Millisecond)
	if parseResult.ChunkCount > 0 {
		ttftLog := "ttft=-"
		if parseResult.HasTTFT {
			ttftLog = fmt.Sprintf("ttft=%dms", parseResult.TTFTMs)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(sse) %s | %s | chunks=%d | %s | %s", payload.URL, payload.Model, parseResult.ChunkCount, duration, ttftLog))
		messagePayload := map[string]any{
			"role":    "assistant",
			"content": nil,
		}
		if parseResult.Content != "" {
			messagePayload["content"] = parseResult.Content
		}
		if parseResult.ReasoningContent != "" {
			messagePayload["reasoning_content"] = parseResult.ReasoningContent
		}
		body := map[string]any{
			"model": firstNonEmpty(parseResult.ReturnedModel, payload.Model),
			"choices": []map[string]any{
				{
					"message": messagePayload,
				},
			},
			"usage":             parseResult.Usage,
			"isStreamAssembled": true,
			"message":           "success",
		}
		if parseResult.HasTTFT {
			body["ttftMs"] = parseResult.TTFTMs
		}
		return http.StatusOK, body
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | no valid SSE chunks | %s", payload.URL, payload.Model, duration))
	return http.StatusBadGateway, map[string]any{"error": map[string]any{"message": "流式响应无有效数据(0 chunks)"}}
}

func executeCheckKeyAttempt(payload normalizedCheckKeyPayload, targetURL string) checkExecutionResult {
	lowerTargetURL := strings.ToLower(strings.TrimSpace(targetURL))
	if strings.HasSuffix(lowerTargetURL, "/messages") {
		return executeAnthropicCheckAttempt(payload, targetURL)
	}
	if strings.HasSuffix(lowerTargetURL, "/responses") {
		return executeResponsesCheckAttempt(payload, targetURL)
	}

	requestBody := map[string]any{
		"model":    payload.Model,
		"messages": payload.Messages,
		"stream":   true,
	}
	requestBody["stream_options"] = map[string]any{
		"include_usage": true,
	}
	if requestBody["messages"] == nil {
		requestBody["messages"] = []map[string]any{{"role": "user", "content": "hi"}}
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] try %s | %s", payload.Model, targetURL))

	bodyBytes, _ := json.Marshal(requestBody)
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}

	req.Header.Set("Authorization", "Bearer "+payload.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 BatchApiCheck/1.0")
	for key, value := range buildCompatHeaders(payload.UID) {
		req.Header.Set(key, value)
	}

	client, err := newOutboundHTTPClient(time.Duration(payload.TimeoutMs) * time.Millisecond)
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusInternalServerError,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusInternalServerError,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			status = http.StatusGatewayTimeout
			message = fmt.Sprintf("Request timed out (%ds)", payload.TimeoutMs/1000)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error %s | %s | %s", payload.Model, targetURL, message))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    status,
			message:   message,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    status,
				Message:   message,
				Retryable: false,
			},
		}
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawBody, _ := io.ReadAll(resp.Body)
		duration := time.Since(start).Round(10 * time.Millisecond)
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if strings.Contains(contentType, "json") {
			var errorPayload map[string]any
			if err := json.Unmarshal(rawBody, &errorPayload); err == nil {
				errMessage = extractCheckErrorMessage(errorPayload, errMessage)
			}
		} else if title := extractHTMLTitle(string(rawBody)); title != "" {
			errMessage = "(HTML) " + title
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    resp.StatusCode,
			message:   errMessage,
			retryable: isRetryableCheckStatus(resp.StatusCode),
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    resp.StatusCode,
				Message:   errMessage,
				Retryable: isRetryableCheckStatus(resp.StatusCode),
			},
		}
	}

	if strings.Contains(contentType, "json") {
		rawBody, _ := io.ReadAll(resp.Body)
		duration := time.Since(start).Round(10 * time.Millisecond)
		var responsePayload map[string]any
		if err := json.Unmarshal(rawBody, &responsePayload); err != nil {
			appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error %s | %s | invalid JSON", payload.Model, targetURL))
			return checkExecutionResult{
				ok:        false,
				endpoint:  targetURL,
				status:    http.StatusBadGateway,
				message:   "Invalid JSON",
				retryable: false,
				attempt: &checkEndpointAttempt{
					Endpoint:  targetURL,
					Status:    http.StatusBadGateway,
					Message:   "Invalid JSON",
					Retryable: false,
				},
			}
		}
		if _, ok := responsePayload["choices"]; ok {
			usage := normalizeUsageTokenTotals(responsePayload["usage"])
			appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(json) %s | %s | %s", payload.Model, targetURL, duration))
			return checkExecutionResult{
				ok:       true,
				endpoint: targetURL,
				status:   http.StatusOK,
				body: map[string]any{
					"model":   firstNonEmpty(strings.TrimSpace(toStringValue(responsePayload["model"])), payload.Model),
					"choices": responsePayload["choices"],
					"usage":   usage,
					"message": "success",
				},
			}
		}

		errMessage := firstNonEmpty(
			getNestedString(responsePayload, "error", "message"),
			strings.TrimSpace(toStringValue(responsePayload["message"])),
			"Unknown error",
		)
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   errMessage,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   errMessage,
				Retryable: false,
			},
		}
	}

	parseResult := parseSSECompletionStream(resp.Body, start)
	duration := time.Since(start).Round(10 * time.Millisecond)
	if parseResult.ChunkCount > 0 {
		ttftLog := "ttft=-"
		if parseResult.HasTTFT {
			ttftLog = fmt.Sprintf("ttft=%dms", parseResult.TTFTMs)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(sse) %s | %s | chunks=%d | %s | %s", payload.Model, targetURL, parseResult.ChunkCount, duration, ttftLog))
		messagePayload := map[string]any{
			"role":    "assistant",
			"content": nil,
		}
		if parseResult.Content != "" {
			messagePayload["content"] = parseResult.Content
		}
		if parseResult.ReasoningContent != "" {
			messagePayload["reasoning_content"] = parseResult.ReasoningContent
		}
		usage := normalizeUsageTokenTotals(parseResult.Usage)
		body := map[string]any{
			"model": firstNonEmpty(parseResult.ReturnedModel, payload.Model),
			"choices": []map[string]any{
				{
					"message": messagePayload,
				},
			},
			"usage":             usage,
			"isStreamAssembled": true,
			"message":           "success",
		}
		if parseResult.HasTTFT {
			body["ttftMs"] = parseResult.TTFTMs
		}
		return checkExecutionResult{
			ok:       true,
			endpoint: targetURL,
			status:   http.StatusOK,
			body:     body,
		}
	}

	errMessage := "Stream response did not contain valid chunks (0 chunks)"
	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
	return checkExecutionResult{
		ok:        false,
		endpoint:  targetURL,
		status:    http.StatusBadGateway,
		message:   errMessage,
		retryable: false,
		attempt: &checkEndpointAttempt{
			Endpoint:  targetURL,
			Status:    http.StatusBadGateway,
			Message:   errMessage,
			Retryable: false,
		},
	}
}

func buildResponsesCheckInput(raw any) any {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return "hi"
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(toStringValue(msg["role"]))
		contentText := ""
		switch content := msg["content"].(type) {
		case string:
			contentText = strings.TrimSpace(content)
		case []any:
			parts := make([]string, 0, len(content))
			for _, rawPart := range content {
				partMap, ok := rawPart.(map[string]any)
				if !ok {
					continue
				}
				partType := strings.TrimSpace(toStringValue(partMap["type"]))
				if partType != "" && partType != "text" && partType != "input_text" && partType != "output_text" {
					continue
				}
				text := strings.TrimSpace(toStringValue(partMap["text"]))
				if text != "" {
					parts = append(parts, text)
				}
			}
			contentText = strings.TrimSpace(strings.Join(parts, "\n"))
		default:
			contentText = strings.TrimSpace(toStringValue(msg["content"]))
		}
		if contentText == "" {
			continue
		}
		if role == "" {
			role = "user"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, contentText))
	}
	if len(lines) == 0 {
		return "hi"
	}
	return strings.Join(lines, "\n\n")
}

func extractResponsesOutputText(responsePayload map[string]any) string {
	if text := strings.TrimSpace(toStringValue(responsePayload["output_text"])); text != "" {
		return text
	}

	outputs, ok := responsePayload["output"].([]any)
	if !ok {
		return ""
	}

	parts := make([]string, 0, len(outputs))
	for _, rawOutput := range outputs {
		outputMap, ok := rawOutput.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(toStringValue(outputMap["type"])) != "message" {
			continue
		}
		contents, ok := outputMap["content"].([]any)
		if !ok {
			continue
		}
		for _, rawContent := range contents {
			contentMap, ok := rawContent.(map[string]any)
			if !ok {
				continue
			}
			contentType := strings.TrimSpace(toStringValue(contentMap["type"]))
			switch contentType {
			case "output_text", "text":
				text := strings.TrimSpace(toStringValue(contentMap["text"]))
				if text != "" {
					parts = append(parts, text)
				}
			case "refusal":
				text := strings.TrimSpace(toStringValue(contentMap["refusal"]))
				if text != "" {
					parts = append(parts, text)
				}
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func executeResponsesCheckAttempt(payload normalizedCheckKeyPayload, targetURL string) checkExecutionResult {
	requestBody := map[string]any{
		"model":   payload.Model,
		"include": []string{"reasoning.encrypted_content"},
		"input":   buildResponsesCheckInput(payload.Messages),
		"stream":  true,
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] try(responses) %s | %s", payload.Model, targetURL))

	bodyBytes, _ := json.Marshal(requestBody)
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}

	req.Header.Set("Authorization", "Bearer "+payload.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "Mozilla/5.0 BatchApiCheck/1.0")
	for key, value := range buildCompatHeaders(payload.UID) {
		req.Header.Set(key, value)
	}

	client, err := newOutboundHTTPClient(time.Duration(payload.TimeoutMs) * time.Millisecond)
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusInternalServerError,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusInternalServerError,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			status = http.StatusGatewayTimeout
			message = fmt.Sprintf("Request timed out (%ds)", payload.TimeoutMs/1000)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error(responses) %s | %s | %s", payload.Model, targetURL, message))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    status,
			message:   message,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    status,
				Message:   message,
				Retryable: false,
			},
		}
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	duration := time.Since(start).Round(10 * time.Millisecond)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if strings.Contains(contentType, "json") {
			var errorPayload map[string]any
			if err := json.Unmarshal(rawBody, &errorPayload); err == nil {
				errMessage = extractCheckErrorMessage(errorPayload, errMessage)
			}
		} else if title := extractHTMLTitle(string(rawBody)); title != "" {
			errMessage = "(HTML) " + title
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail(responses) %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    resp.StatusCode,
			message:   errMessage,
			retryable: isRetryableCheckStatus(resp.StatusCode),
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    resp.StatusCode,
				Message:   errMessage,
				Retryable: isRetryableCheckStatus(resp.StatusCode),
			},
		}
	}

	if strings.Contains(contentType, "text/event-stream") || strings.HasPrefix(strings.TrimSpace(string(rawBody)), "data:") {
		parseResult := parseSSECompletionStream(bytes.NewReader(rawBody), start)
		if parseResult.ChunkCount > 0 {
			ttftLog := "ttft=-"
			if parseResult.HasTTFT {
				ttftLog = fmt.Sprintf("ttft=%dms", parseResult.TTFTMs)
			}
			usage := normalizeUsageTokenTotals(parseResult.Usage)
			appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(responses-sse) %s | %s | chunks=%d | %s | %s", payload.Model, targetURL, parseResult.ChunkCount, duration, ttftLog))
			messagePayload := map[string]any{
				"role":    "assistant",
				"content": parseResult.Content,
			}
			if parseResult.ReasoningContent != "" {
				messagePayload["reasoning_content"] = parseResult.ReasoningContent
			}
			body := map[string]any{
				"model": firstNonEmpty(parseResult.ReturnedModel, payload.Model),
				"choices": []map[string]any{
					{
						"message": messagePayload,
					},
				},
				"usage":             usage,
				"isStreamAssembled": true,
				"message":           "success",
			}
			if parseResult.HasTTFT {
				body["ttftMs"] = parseResult.TTFTMs
			}
			return checkExecutionResult{
				ok:       true,
				endpoint: targetURL,
				status:   http.StatusOK,
				body:     body,
			}
		}
	}

	var responsePayload map[string]any
	if err := json.Unmarshal(rawBody, &responsePayload); err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error(responses) %s | %s | invalid JSON", payload.Model, targetURL))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadGateway,
			message:   "Invalid JSON",
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadGateway,
				Message:   "Invalid JSON",
				Retryable: false,
			},
		}
	}

	textContent := extractResponsesOutputText(responsePayload)
	if textContent == "" {
		errMessage := firstNonEmpty(
			getNestedString(responsePayload, "error", "message"),
			strings.TrimSpace(toStringValue(responsePayload["message"])),
			strings.TrimSpace(toStringValue(responsePayload["error"])),
			"Unknown error",
		)
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail(responses) %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   errMessage,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   errMessage,
				Retryable: false,
			},
		}
	}

	usage := normalizeUsageTokenTotals(responsePayload["usage"])
	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(responses) %s | %s | %s", payload.Model, targetURL, duration))
	return checkExecutionResult{
		ok:       true,
		endpoint: targetURL,
		status:   http.StatusOK,
		body: map[string]any{
			"model": firstNonEmpty(strings.TrimSpace(toStringValue(responsePayload["model"])), payload.Model),
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": textContent,
					},
				},
			},
			"usage":   usage,
			"message": "success",
		},
	}
}

func normalizeAnthropicMessages(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return []map[string]any{{"role": "user", "content": "hi"}}
	}

	messages := make([]map[string]any, 0, len(items))
	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.TrimSpace(toStringValue(msg["role"]))
		if role == "" || strings.EqualFold(role, "system") {
			continue
		}
		content := msg["content"]
		if content == nil {
			content = ""
		}
		messages = append(messages, map[string]any{
			"role":    role,
			"content": content,
		})
	}
	if len(messages) == 0 {
		return []map[string]any{{"role": "user", "content": "hi"}}
	}
	return messages
}

func extractAnthropicTextContent(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(toStringValue(block["type"])) != "text" {
				continue
			}
			text := strings.TrimSpace(toStringValue(block["text"]))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func executeAnthropicCheckAttempt(payload normalizedCheckKeyPayload, targetURL string) checkExecutionResult {
	requestBody := map[string]any{
		"model":      payload.Model,
		"messages":   normalizeAnthropicMessages(payload.Messages),
		"max_tokens": 32,
		"stream":     false,
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] try(anthropic) %s | %s", payload.Model, targetURL))

	bodyBytes, _ := json.Marshal(requestBody)
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}

	req.Header.Set("Authorization", "Bearer "+payload.Key)
	req.Header.Set("x-api-key", payload.Key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 BatchApiCheck/1.0")

	client, err := newOutboundHTTPClient(time.Duration(payload.TimeoutMs) * time.Millisecond)
	if err != nil {
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusInternalServerError,
			message:   err.Error(),
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusInternalServerError,
				Message:   err.Error(),
				Retryable: false,
			},
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			status = http.StatusGatewayTimeout
			message = fmt.Sprintf("Request timed out (%ds)", payload.TimeoutMs/1000)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error(anthropic) %s | %s | %s", payload.Model, targetURL, message))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    status,
			message:   message,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    status,
				Message:   message,
				Retryable: false,
			},
		}
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	duration := time.Since(start).Round(10 * time.Millisecond)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if strings.Contains(contentType, "json") {
			var errorPayload map[string]any
			if err := json.Unmarshal(rawBody, &errorPayload); err == nil {
				errMessage = extractCheckErrorMessage(errorPayload, errMessage)
			}
		} else if title := extractHTMLTitle(string(rawBody)); title != "" {
			errMessage = "(HTML) " + title
		}
		if isAnyrouterCheckPayload(payload) && isAnyrouterClaude1MErrorMessage(errMessage) {
			errMessage = buildAnyrouterClaudeUpgradeHint(payload.Model)
		}
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail(anthropic) %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    resp.StatusCode,
			message:   errMessage,
			retryable: isRetryableCheckStatus(resp.StatusCode),
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    resp.StatusCode,
				Message:   errMessage,
				Retryable: isRetryableCheckStatus(resp.StatusCode),
			},
		}
	}

	var responsePayload map[string]any
	if err := json.Unmarshal(rawBody, &responsePayload); err != nil {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] error(anthropic) %s | %s | invalid JSON", payload.Model, targetURL))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadGateway,
			message:   "Invalid JSON",
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadGateway,
				Message:   "Invalid JSON",
				Retryable: false,
			},
		}
	}

	textContent := extractAnthropicTextContent(responsePayload["content"])
	if textContent == "" {
		errMessage := extractCheckErrorMessage(responsePayload, "Unknown error")
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail(anthropic) %s | %s | %s | %s", payload.Model, targetURL, errMessage, duration))
		return checkExecutionResult{
			ok:        false,
			endpoint:  targetURL,
			status:    http.StatusBadRequest,
			message:   errMessage,
			retryable: false,
			attempt: &checkEndpointAttempt{
				Endpoint:  targetURL,
				Status:    http.StatusBadRequest,
				Message:   errMessage,
				Retryable: false,
			},
		}
	}

	usage := normalizeUsageTokenTotals(responsePayload["usage"])
	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(anthropic) %s | %s | %s", payload.Model, targetURL, duration))
	return checkExecutionResult{
		ok:       true,
		endpoint: targetURL,
		status:   http.StatusOK,
		body: map[string]any{
			"model": firstNonEmpty(strings.TrimSpace(toStringValue(responsePayload["model"])), payload.Model),
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": textContent,
					},
				},
			},
			"usage":   usage,
			"message": "success",
		},
	}
}

func executeCheckKeySmart(payload normalizedCheckKeyPayload) (int, map[string]any) {
	phases := buildCheckEndpointPhases(payload)
	endpoints := make([]string, 0, 6)
	for _, phase := range phases {
		endpoints = append(endpoints, phase.endpoints...)
	}
	if len(endpoints) == 0 {
		return http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "API URL is empty or invalid",
			},
		}
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] start %s | %s | endpoints=%d | timeout=%dms", payload.URL, payload.Model, len(endpoints), payload.TimeoutMs))

	attempts := make([]checkEndpointAttempt, 0, len(endpoints))
	var lastFailure *checkExecutionResult

	for phaseIndex, phase := range phases {
		var phaseLastFailure *checkExecutionResult
		for _, endpoint := range phase.endpoints {
			result := executeCheckKeyAttempt(payload, endpoint)
			if result.ok {
				if result.body == nil {
					result.body = map[string]any{}
				}
				if phase.protocol == "responses" {
					setCheckProtocolPreference(payload, checkProtocolPreferResponses)
				}
				diagnostics := buildCheckDiagnostics(payload, attempts)
				diagnostics["resolvedEndpoint"] = result.endpoint
				result.body["diagnostics"] = diagnostics
				return result.status, result.body
			}

			lastFailure = &result
			phaseLastFailure = &result
			if result.attempt != nil {
				attempts = append(attempts, *result.attempt)
			}
			if shouldContinueCheckPhaseAfterFailure(result) {
				continue
			}
			break
		}

		if phaseIndex >= len(phases)-1 {
			continue
		}
		if !shouldAdvanceCheckPhase(payload, phase.protocol, phaseLastFailure, phases[phaseIndex+1].protocol) {
			break
		}
	}

	fallbackMessage := "No compatible chat completion endpoint found"
	fallbackStatus := http.StatusNotFound
	if lastFailure != nil {
		if strings.TrimSpace(lastFailure.message) != "" {
			fallbackMessage = lastFailure.message
		}
		if lastFailure.status > 0 {
			fallbackStatus = lastFailure.status
		}
	}

	return fallbackStatus, map[string]any{
		"error": map[string]any{
			"message":     fallbackMessage,
			"diagnostics": buildCheckDiagnostics(payload, attempts),
		},
	}
}

func fetchTokensForAccountViaAccessToken(account ChromeProfileAccount) ChromeProfileTokenResult {
	result := ChromeProfileTokenResult{
		ID:       account.ID,
		SiteName: account.SiteName,
		SiteURL:  account.SiteURL,
		Tokens:   []map[string]interface{}{},
	}

	baseURL := strings.TrimRight(strings.TrimSpace(account.SiteURL), "/")
	accessToken := strings.TrimSpace(account.AccountInfo.AccessToken)
	if baseURL == "" || accessToken == "" {
		result.Error = "missing_access_token_or_site_url"
		return result
	}

	userID := normalizeUserID(account.AccountInfo.ID)
	baseHeaders := map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"X-Requested-With": "XMLHttpRequest",
	}
	for key, value := range buildCompatHeaders(userID) {
		baseHeaders[key] = value
	}

	var lastErr error
	deadline := time.Now().Add(profileFetchTotalTimeout)
	attempts := 0
	for _, endpoint := range getProfileTokenEndpoints(account.SiteType) {
		if attempts >= profileFetchAttemptLimit {
			break
		}
		if time.Now().After(deadline) {
			lastErr = fmt.Errorf("profile_fetch_timeout")
			break
		}
		attempts += 1
		tokens, err := requestTokenListEndpoint(baseURL, endpoint, accessToken, baseHeaders, deadline)
		if err != nil {
			lastErr = err
			continue
		}
		if len(tokens) == 0 {
			lastErr = fmt.Errorf("token_list_empty")
			continue
		}
		result.Tokens = tokens
		result.ResolvedAccessToken = accessToken
		result.ResolvedUserID = userID
		appendLine(resolveFetchLogPath(), fmt.Sprintf("[FETCH] %s | %s -> %d tokens", account.SiteName, endpoint, len(tokens)))
		return result
	}

	if lastErr != nil {
		result.Error = lastErr.Error()
	} else {
		result.Error = "all_endpoints_failed"
	}
	appendLine(resolveFetchLogPath(), fmt.Sprintf("[FETCH] fail %s | %s", account.SiteName, result.Error))
	return result
}

func performExternalHTTPRequest(method string, targetURL string, headers map[string]string, body string, timeout time.Duration) (*bridgeHTTPResponse, error) {
	var bodyReader io.Reader
	if method != http.MethodGet && method != http.MethodHead && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	client, err := newOutboundHTTPClient(timeout)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	responseHeaders := map[string]string{}
	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		responseHeaders[key] = values[0]
	}

	return &bridgeHTTPResponse{
		Status:  resp.StatusCode,
		Headers: responseHeaders,
		Body:    string(data),
	}, nil
}

type bridgeHTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func jsonBridgeResponse(status int, payload any) *bridgeHTTPResponse {
	data, _ := json.Marshal(payload)
	return &bridgeHTTPResponse{
		Status: status,
		Headers: map[string]string{
			"Content-Type": "application/json; charset=utf-8",
		},
		Body: string(data),
	}
}

func encodeBridgeResponse(resp *bridgeHTTPResponse) (string, error) {
	if resp == nil {
		resp = &bridgeHTTPResponse{
			Status: 500,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: `{"message":"empty bridge response"}`,
		}
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func escapeJSONText(value string) string {
	data, _ := json.Marshal(value)
	if len(data) >= 2 {
		return string(data[1 : len(data)-1])
	}
	return ""
}

func localFetchKeysProgressReset(total int) {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()

	now := time.Now().UnixMilli()
	localFetchKeysProgress.Active = total > 0
	localFetchKeysProgress.Stage = ""
	localFetchKeysProgress.Detail = ""
	localFetchKeysProgress.Total = total
	localFetchKeysProgress.Completed = 0
	localFetchKeysProgress.SuccessSites = 0
	localFetchKeysProgress.LastSiteName = ""
	localFetchKeysProgress.StartedAt = now
	localFetchKeysProgress.LastUpdatedAt = now
}

func localFetchKeysProgressSetStage(stage string, detail string) {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()

	localFetchKeysProgress.Stage = strings.TrimSpace(stage)
	localFetchKeysProgress.Detail = strings.TrimSpace(detail)
	localFetchKeysProgress.LastUpdatedAt = time.Now().UnixMilli()
}

func localFetchKeysProgressSetCurrentSite(siteName string) {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()

	localFetchKeysProgress.LastSiteName = strings.TrimSpace(siteName)
	localFetchKeysProgress.LastUpdatedAt = time.Now().UnixMilli()
}

func localFetchKeysProgressMark(result ChromeProfileTokenResult) {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()

	localFetchKeysProgress.Stage = "extract_site"
	localFetchKeysProgress.Detail = "逐站点提取 Token"
	localFetchKeysProgress.Completed += 1
	localFetchKeysProgress.LastSiteName = strings.TrimSpace(result.SiteName)
	if len(result.Tokens) > 0 {
		localFetchKeysProgress.SuccessSites += 1
	}
	localFetchKeysProgress.LastUpdatedAt = time.Now().UnixMilli()
}

func localFetchKeysProgressFinish() {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()

	localFetchKeysProgress.Active = false
	localFetchKeysProgress.Stage = "done"
	localFetchKeysProgress.Detail = "提取完成，正在整理结果"
	localFetchKeysProgress.LastUpdatedAt = time.Now().UnixMilli()
}

func localFetchKeysProgressSnapshot() fetchKeysProgressSnapshot {
	localFetchKeysProgress.mu.Lock()
	defer localFetchKeysProgress.mu.Unlock()
	return fetchKeysProgressSnapshot{
		Active:        localFetchKeysProgress.Active,
		Stage:         localFetchKeysProgress.Stage,
		Detail:        localFetchKeysProgress.Detail,
		Total:         localFetchKeysProgress.Total,
		Completed:     localFetchKeysProgress.Completed,
		SuccessSites:  localFetchKeysProgress.SuccessSites,
		LastSiteName:  localFetchKeysProgress.LastSiteName,
		StartedAt:     localFetchKeysProgress.StartedAt,
		LastUpdatedAt: localFetchKeysProgress.LastUpdatedAt,
	}
}

func parseSSECompletionStream(reader io.Reader, startedAt time.Time) sseCompletionParseResult {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	result := sseCompletionParseResult{
		TTFTMs: -1,
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" || payload == "" {
			continue
		}

		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		result.ChunkCount++
		if result.ReturnedModel == "" {
			result.ReturnedModel = strings.TrimSpace(toStringValue(chunk["model"]))
		}
		if chunk["usage"] != nil {
			result.Usage = chunk["usage"]
		}

		eventType := strings.TrimSpace(toStringValue(chunk["type"]))
		switch eventType {
		case "response.output_text.delta", "response.refusal.delta":
			textPiece := firstNonEmpty(toStringValue(chunk["delta"]), toStringValue(chunk["text"]))
			if !result.HasTTFT && strings.TrimSpace(textPiece) != "" {
				elapsedMs := time.Since(startedAt).Milliseconds()
				if elapsedMs < 0 {
					elapsedMs = 0
				}
				result.TTFTMs = elapsedMs
				result.HasTTFT = true
			}
			result.Content += textPiece
			continue
		case "response.output_text.done", "response.refusal.done":
			if result.Content == "" {
				result.Content = toStringValue(chunk["text"])
			}
			continue
		case "response.completed":
			responseMap, _ := chunk["response"].(map[string]any)
			if responseMap != nil {
				if result.ReturnedModel == "" {
					result.ReturnedModel = strings.TrimSpace(toStringValue(responseMap["model"]))
				}
				if responseMap["usage"] != nil {
					result.Usage = responseMap["usage"]
				}
				if text := extractResponsesOutputText(responseMap); text != "" && result.Content == "" {
					result.Content = text
				}
			}
			continue
		}

		choices, _ := chunk["choices"].([]any)
		if len(choices) == 0 {
			continue
		}
		choiceMap, _ := choices[0].(map[string]any)
		if choiceMap == nil {
			continue
		}
		delta, _ := choiceMap["delta"].(map[string]any)
		if delta == nil {
			continue
		}
		contentPiece := toStringValue(delta["content"])
		reasoningPiece := toStringValue(delta["reasoning_content"]) + toStringValue(delta["thinking"])
		if !result.HasTTFT && strings.TrimSpace(contentPiece+reasoningPiece) != "" {
			elapsedMs := time.Since(startedAt).Milliseconds()
			if elapsedMs < 0 {
				elapsedMs = 0
			}
			result.TTFTMs = elapsedMs
			result.HasTTFT = true
		}
		result.Content += contentPiece
		result.ReasoningContent += reasoningPiece
	}

	return result
}

func normalizeUsageTokenTotals(raw any) any {
	usageMap, ok := raw.(map[string]any)
	if !ok || usageMap == nil {
		return raw
	}

	normalized := make(map[string]any, len(usageMap)+1)
	for key, value := range usageMap {
		normalized[key] = value
	}

	if total := usageTotalTokens(normalized); total > 0 {
		normalized["total_tokens"] = total
		return normalized
	}

	input := usageFirstPositive(
		normalized["input_tokens"],
		normalized["prompt_tokens"],
		usageNestedValue(normalized, "input_tokens_details", "input_tokens"),
		usageNestedValue(normalized, "prompt_tokens_details", "input_tokens"),
	)
	output := usageFirstPositive(
		normalized["output_tokens"],
		normalized["completion_tokens"],
		usageNestedValue(normalized, "output_tokens_details", "output_tokens"),
		usageNestedValue(normalized, "completion_tokens_details", "completion_tokens"),
	)
	reasoning := usageFirstPositive(
		normalized["reasoning_tokens"],
		usageNestedValue(normalized, "completion_tokens_details", "reasoning_tokens"),
		usageNestedValue(normalized, "output_tokens_details", "reasoning_tokens"),
	)
	cached := usageFirstPositive(
		normalized["cached_tokens"],
		usageNestedValue(normalized, "prompt_tokens_details", "cached_tokens"),
		usageNestedValue(normalized, "input_tokens_details", "cached_tokens"),
		usageNestedValue(normalized, "cache_read_input_tokens"),
		usageNestedValue(normalized, "cache_creation_input_tokens"),
	)

	total := input + output + reasoning
	if total <= 0 {
		total = input + output + reasoning + cached
	}
	if total > 0 {
		normalized["total_tokens"] = int64(total)
		return normalized
	}

	return normalized
}

func usageTotalTokens(usage map[string]any) int64 {
	if usage == nil {
		return 0
	}
	return int64(usageFirstPositive(usage["total_tokens"]))
}

func usageFirstPositive(candidates ...any) float64 {
	for _, candidate := range candidates {
		value := toFloat64OrZero(candidate)
		if value > 0 {
			return value
		}
	}
	return 0
}

func usageNestedValue(root map[string]any, path ...string) any {
	current := any(root)
	for _, key := range path {
		if current == nil {
			return nil
		}
		nextMap, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		nextValue, exists := nextMap[key]
		if !exists {
			return nil
		}
		current = nextValue
	}
	return current
}

func extractHTMLTitle(body string) string {
	start := strings.Index(strings.ToLower(body), "<title>")
	end := strings.Index(strings.ToLower(body), "</title>")
	if start < 0 || end <= start+7 {
		return ""
	}
	return strings.TrimSpace(body[start+7 : end])
}

func getHeaderIgnoreCase(headers map[string]string, name string) string {
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func toFloat64OrZero(value any) float64 {
	number, ok := toFloat64(value)
	if !ok {
		return 0
	}
	return number
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func resolveRuntimeRootDir() string {
	explicitDir := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_RUNTIME_DIR"))
	if explicitDir != "" {
		if abs, err := filepath.Abs(explicitDir); err == nil {
			return abs
		}
		return explicitDir
	}

	if runtime.GOOS == "windows" {
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if localAppData != "" {
			return filepath.Join(localAppData, "BatchApiCheck", "runtime")
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(homeDir, "Library", "Application Support", "BatchApiCheck", "runtime")
		}
		return filepath.Join(homeDir, ".cache", "batch-api-check", "runtime")
	}

	return filepath.Join(os.TempDir(), "batch-api-check", "runtime")
}

func resolveRuntimeLogDir() string {
	dir := filepath.Join(resolveRuntimeRootDir(), "logs")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func resolveFetchLogPath() string {
	return filepath.Join(resolveRuntimeLogDir(), "fetch-keys.log")
}

func resolveCheckLogPath() string {
	return filepath.Join(resolveRuntimeLogDir(), "check-keys.log")
}

func appendLine(path string, line string) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(line) == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), line))
}

func normalizeBrowserType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "edge") {
		return "edge"
	}
	return "chrome"
}

func detectInstalledBrowsersGo() map[string]any {
	browsers := make([]map[string]string, 0, 2)
	defaultBrowser := ""

	if path := findLocalBrowserExecutableGo("chrome"); path != "" {
		browsers = append(browsers, map[string]string{"type": "chrome", "path": path})
		defaultBrowser = "chrome"
	}
	if path := findLocalBrowserExecutableGo("edge"); path != "" {
		browsers = append(browsers, map[string]string{"type": "edge", "path": path})
		if defaultBrowser == "" {
			defaultBrowser = "edge"
		}
	}

	return map[string]any{
		"success":        true,
		"browsers":       browsers,
		"defaultBrowser": defaultBrowser,
	}
}

func findLocalBrowserExecutableGo(browserType string) string {
	candidates := map[string][]string{
		"chrome": {
			filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		},
		"edge": {
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		},
	}

	for _, candidate := range candidates[normalizeBrowserType(browserType)] {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func isBrowserProcessRunningGo(browserType string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	imageName := "chrome.exe"
	if normalizeBrowserType(browserType) == "edge" {
		imageName = "msedge.exe"
	}
	cmd := exec.Command("tasklist.exe", "/FI", fmt.Sprintf("IMAGENAME eq %s", imageName))
	configureBackgroundCmd(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(output)), strings.ToLower(imageName))
}
