package main

import (
	"bufio"
	"bytes"
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

type normalizedCheckKeyPayload struct {
	URL       string
	Key       string
	Model     string
	UID       string
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

var checkEndpointStripPatterns = []*regexp.Regexp{
	regexp.MustCompile(`/v\d+/chat/completions$`),
	regexp.MustCompile(`/chat/completions$`),
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

func buildCheckEndpointCandidates(raw string) []string {
	input := normalizeCheckEndpointInput(raw)
	if input == "" {
		return nil
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

	client := &http.Client{Timeout: time.Duration(payload.TimeoutMs) * time.Millisecond}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return http.StatusGatewayTimeout, map[string]any{"error": map[string]any{"message": fmt.Sprintf("请求超时 (%ds)", payload.TimeoutMs/1000)}}
		}
		return http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}}
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	duration := time.Since(start).Round(10 * time.Millisecond)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if contentType := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(contentType, "json") {
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

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "json") {
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

	returnedModel, content, reasoningContent, usage, chunkCount := parseSSECompletion(rawBody)
	if chunkCount > 0 {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(sse) %s | %s | chunks=%d | %s", payload.URL, payload.Model, chunkCount, duration))
		messagePayload := map[string]any{
			"role":    "assistant",
			"content": nil,
		}
		if content != "" {
			messagePayload["content"] = content
		}
		if reasoningContent != "" {
			messagePayload["reasoning_content"] = reasoningContent
		}
		return http.StatusOK, map[string]any{
			"model": firstNonEmpty(returnedModel, payload.Model),
			"choices": []map[string]any{
				{
					"message": messagePayload,
				},
			},
			"usage":             usage,
			"isStreamAssembled": true,
			"message":           "success",
		}
	}

	appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] fail %s | %s | no valid SSE chunks | %s", payload.URL, payload.Model, duration))
	return http.StatusBadGateway, map[string]any{"error": map[string]any{"message": "流式响应无有效数据(0 chunks)"}}
}

func executeCheckKeyAttempt(payload normalizedCheckKeyPayload, targetURL string) checkExecutionResult {
	requestBody := map[string]any{
		"model":    payload.Model,
		"messages": payload.Messages,
		"stream":   true,
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

	client := &http.Client{Timeout: time.Duration(payload.TimeoutMs) * time.Millisecond}
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

	rawBody, _ := io.ReadAll(resp.Body)
	duration := time.Since(start).Round(10 * time.Millisecond)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMessage := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if contentType := strings.ToLower(resp.Header.Get("Content-Type")); strings.Contains(contentType, "json") {
			var errorPayload map[string]any
			if err := json.Unmarshal(rawBody, &errorPayload); err == nil {
				errMessage = firstNonEmpty(
					getNestedString(errorPayload, "error", "message"),
					strings.TrimSpace(toStringValue(errorPayload["message"])),
					errMessage,
				)
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

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "json") {
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
			appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(json) %s | %s | %s", payload.Model, targetURL, duration))
			return checkExecutionResult{
				ok:       true,
				endpoint: targetURL,
				status:   http.StatusOK,
				body: map[string]any{
					"model":   firstNonEmpty(strings.TrimSpace(toStringValue(responsePayload["model"])), payload.Model),
					"choices": responsePayload["choices"],
					"usage":   responsePayload["usage"],
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

	returnedModel, content, reasoningContent, usage, chunkCount := parseSSECompletion(rawBody)
	if chunkCount > 0 {
		appendLine(resolveCheckLogPath(), fmt.Sprintf("[CHECK] ok(sse) %s | %s | chunks=%d | %s", payload.Model, targetURL, chunkCount, duration))
		messagePayload := map[string]any{
			"role":    "assistant",
			"content": nil,
		}
		if content != "" {
			messagePayload["content"] = content
		}
		if reasoningContent != "" {
			messagePayload["reasoning_content"] = reasoningContent
		}
		return checkExecutionResult{
			ok:       true,
			endpoint: targetURL,
			status:   http.StatusOK,
			body: map[string]any{
				"model": firstNonEmpty(returnedModel, payload.Model),
				"choices": []map[string]any{
					{
						"message": messagePayload,
					},
				},
				"usage":             usage,
				"isStreamAssembled": true,
				"message":           "success",
			},
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

func executeCheckKeySmart(payload normalizedCheckKeyPayload) (int, map[string]any) {
	endpoints := buildCheckEndpointCandidates(payload.URL)
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

	for _, endpoint := range endpoints {
		result := executeCheckKeyAttempt(payload, endpoint)
		if result.ok {
			if result.body == nil {
				result.body = map[string]any{}
			}
			diagnostics := buildCheckDiagnostics(payload, attempts)
			diagnostics["resolvedEndpoint"] = result.endpoint
			result.body["diagnostics"] = diagnostics
			return result.status, result.body
		}

		lastFailure = &result
		if result.attempt != nil {
			attempts = append(attempts, *result.attempt)
		}
		if !result.retryable {
			return result.status, map[string]any{
				"error": map[string]any{
					"message":     result.message,
					"diagnostics": buildCheckDiagnostics(payload, attempts),
				},
			}
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

	client := &http.Client{Timeout: timeout}
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

func parseSSECompletion(raw []byte) (string, string, string, any, int) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	returnedModel := ""
	content := ""
	reasoningContent := ""
	var usage any
	chunkCount := 0

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
		chunkCount++
		if returnedModel == "" {
			returnedModel = strings.TrimSpace(toStringValue(chunk["model"]))
		}
		if chunk["usage"] != nil {
			usage = chunk["usage"]
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
		content += toStringValue(delta["content"])
		reasoningContent += toStringValue(delta["reasoning_content"]) + toStringValue(delta["thinking"])
	}

	return returnedModel, content, reasoningContent, usage, chunkCount
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
