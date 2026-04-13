package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/syndtr/goleveldb/leveldb"
)

var profileStorageKeys = map[string]bool{
	"auth_user":        true,
	"user":             true,
	"auth_token":       true,
	"access_token":     true,
	"token":            true,
	"authToken":        true,
	"refresh_token":    true,
	"token_expires_at": true,
}

const (
	profileFetchTotalTimeout = 7 * time.Second
	profileFetchAttemptLimit = 2
	profilePerRequestTimeout = 3 * time.Second
	profileMinRequestTimeout = 250 * time.Millisecond
	profileTokenExpiryLeeway = 5 * time.Second
)

type ChromeProfileTokenRequest struct {
	Accounts []ChromeProfileAccount `json:"accounts"`
}

type ChromeProfileAccount struct {
	ID          string                        `json:"id"`
	SiteName    string                        `json:"site_name"`
	SiteURL     string                        `json:"site_url"`
	SiteType    string                        `json:"site_type"`
	APIKey      string                        `json:"api_key"`
	AccountInfo ChromeProfileAccountInfoInput `json:"account_info"`
}

type ChromeProfileAccountInfoInput struct {
	ID          interface{} `json:"id"`
	AccessToken string      `json:"access_token"`
}

type ChromeProfileTokenResponse struct {
	Results  []ChromeProfileTokenResult `json:"results"`
	Warnings []string                   `json:"warnings,omitempty"`
}

type ChromeProfileTokenResult struct {
	ID                  string                   `json:"id"`
	SiteName            string                   `json:"site_name"`
	SiteURL             string                   `json:"site_url"`
	Tokens              []map[string]interface{} `json:"tokens"`
	Error               string                   `json:"error,omitempty"`
	ResolvedAccessToken string                   `json:"resolved_access_token,omitempty"`
	ResolvedUserID      string                   `json:"resolved_user_id,omitempty"`
	StorageFields       []string                 `json:"storage_fields,omitempty"`
	StorageOrigin       string                   `json:"storage_origin,omitempty"`
}

type profileAuthSnapshot struct {
	entries map[string]map[string]string
}

func (a *App) ExtractChromeProfileTokens(request ChromeProfileTokenRequest) (*ChromeProfileTokenResponse, error) {
	if len(request.Accounts) == 0 {
		return &ChromeProfileTokenResponse{Results: []ChromeProfileTokenResult{}}, nil
	}

	localFetchKeysProgressReset(len(request.Accounts))
	localFetchKeysProgressSetStage("profile_prepare", "准备读取 Chrome Default Profile")
	localFetchKeysProgressSetCurrentSite("Chrome Default Profile")

	snapshot, warnings, err := loadChromeProfileAuthSnapshot()
	if err != nil {
		localFetchKeysProgressFinish()
		return nil, err
	}

	results := make([]ChromeProfileTokenResult, len(request.Accounts))
	jobCh := make(chan int)
	var wg sync.WaitGroup
	workerCount := minInt(10, len(request.Accounts))

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobCh {
				localFetchKeysProgressSetStage("extract_site", "逐站点提取 Token")
				localFetchKeysProgressSetCurrentSite(request.Accounts[idx].SiteName)
				results[idx] = snapshot.extractSiteTokens(request.Accounts[idx])
				localFetchKeysProgressMark(results[idx])
			}
		}()
	}

	for idx := range request.Accounts {
		jobCh <- idx
	}
	close(jobCh)
	wg.Wait()

	successSites := 0
	for _, result := range results {
		if len(result.Tokens) > 0 {
			successSites++
		}
	}
	debugLogf("profile auth extraction complete: successSites=%d/%d warnings=%d", successSites, len(results), len(warnings))
	localFetchKeysProgressFinish()

	return &ChromeProfileTokenResponse{
		Results:  results,
		Warnings: warnings,
	}, nil
}

func loadChromeProfileAuthSnapshot() (*profileAuthSnapshot, []string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(localAppData) == "" {
		return nil, nil, fmt.Errorf("LOCALAPPDATA is empty")
	}

	leveldbDir := filepath.Join(localAppData, "Google", "Chrome", "User Data", "Default", "Local Storage", "leveldb")
	localFetchKeysProgressSetStage("profile_copy", "复制 Chrome Local Storage 文件")
	tmpDir, cleanup, err := copyDirToTemp(leveldbDir)
	if err != nil {
		return nil, nil, fmt.Errorf("copy Chrome Local Storage failed: %w", err)
	}
	defer cleanup()

	localFetchKeysProgressSetStage("profile_scan", "扫描 Local Storage 键值")
	db, err := leveldb.OpenFile(tmpDir, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("open Chrome Local Storage failed: %w", err)
	}
	defer db.Close()

	entries := make(map[string]map[string]string)
	warnings := []string{}
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		origin, storageKey, ok := parseProfileStorageKey(iter.Key())
		if !ok || !profileStorageKeys[storageKey] {
			continue
		}

		value := decodeProfileStorageValue(iter.Value())
		if value == "" {
			continue
		}

		origin = normalizeStorageOrigin(origin)
		if origin == "" {
			continue
		}
		if _, exists := entries[origin]; !exists {
			entries[origin] = map[string]string{}
		}
		entries[origin][storageKey] = value
	}

	if err := iter.Error(); err != nil {
		warnings = append(warnings, fmt.Sprintf("iterate Chrome Local Storage failed: %v", err))
	}

	debugLogf("loaded Chrome Local Storage auth snapshot: origins=%d", len(entries))
	return &profileAuthSnapshot{entries: entries}, warnings, nil
}

func (s *profileAuthSnapshot) extractSiteTokens(account ChromeProfileAccount) ChromeProfileTokenResult {
	result := ChromeProfileTokenResult{
		ID:       account.ID,
		SiteName: account.SiteName,
		SiteURL:  account.SiteURL,
		Tokens:   []map[string]interface{}{},
	}

	origin, err := normalizeURLOrigin(account.SiteURL)
	if err != nil {
		result.Error = "site_url_invalid"
		return result
	}
	result.StorageOrigin = origin

	storageValues := s.entries[origin]
	if storageValues == nil {
		storageValues = map[string]string{}
	}

	storageFields := make([]string, 0, len(storageValues))
	for key := range storageValues {
		storageFields = append(storageFields, key)
	}
	sort.Strings(storageFields)
	result.StorageFields = storageFields

	authUserObj := parseJSONMap(storageValues["auth_user"])
	userObj := parseJSONMap(storageValues["user"])
	resolvedUserID := firstNonEmpty(
		extractUserID(authUserObj),
		extractUserID(userObj),
		normalizeUserID(account.AccountInfo.ID),
	)
	result.ResolvedUserID = resolvedUserID

	tokenCandidates := buildTokenCandidates(storageValues, authUserObj, userObj, strings.TrimSpace(account.AccountInfo.AccessToken))
	if len(storageValues) == 0 && len(tokenCandidates) == 0 {
		result.Error = "profile_storage_not_found"
		return result
	}
	if len(tokenCandidates) == 0 {
		result.Error = "profile_token_not_found"
		return result
	}

	refreshToken := strings.TrimSpace(storageValues["refresh_token"])
	tokenExpiresAt := parseInt64Loose(storageValues["token_expires_at"])
	deadline := time.Now().Add(profileFetchTotalTimeout)
	if refreshedToken := tryRefreshProfileToken(account, tokenCandidates, resolvedUserID, refreshToken, tokenExpiresAt, deadline); refreshedToken != "" {
		tokenCandidates = prependUnique(tokenCandidates, refreshedToken)
	}

	tokens, usedToken, err := fetchSiteTokenListByProfileAuth(account, tokenCandidates, resolvedUserID, deadline)
	if err != nil {
		result.Error = err.Error()
		result.ResolvedAccessToken = usedToken
		return result
	}

	result.Tokens = tokens
	result.ResolvedAccessToken = usedToken
	return result
}

func fetchSiteTokenListByProfileAuth(account ChromeProfileAccount, tokenCandidates []string, resolvedUserID string, deadline time.Time) ([]map[string]interface{}, string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(account.SiteURL), "/")
	if baseURL == "" {
		return nil, "", fmt.Errorf("site_url_missing")
	}

	endpoints := getProfileTokenEndpoints(account.SiteType)
	baseHeaders := map[string]string{
		"Accept":           "application/json, text/plain, */*",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"X-Requested-With": "XMLHttpRequest",
	}
	for key, value := range buildCompatHeaders(resolvedUserID) {
		baseHeaders[key] = value
	}

	var bestErr error
	bestErrRank := -1
	bestErrToken := ""
	attempts := 0
outer:
	for _, token := range tokenCandidates {
		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		if token == "" {
			continue
		}
		if localErr := classifyProfileTokenCandidate(token); localErr != nil {
			rank := scoreProfileFetchError("", localErr)
			if rank >= bestErrRank {
				bestErr = localErr
				bestErrRank = rank
				bestErrToken = token
			}
			continue
		}
		for _, endpoint := range endpoints {
			if attempts >= profileFetchAttemptLimit {
				break outer
			}
			if time.Now().After(deadline) {
				bestErr = fmt.Errorf("profile_fetch_timeout")
				bestErrToken = token
				break outer
			}
			attempts += 1
			items, err := requestTokenListEndpoint(baseURL, endpoint, token, baseHeaders, deadline)
			if err != nil {
				rank := scoreProfileFetchError(endpoint, err)
				if rank >= bestErrRank {
					bestErr = err
					bestErrRank = rank
					bestErrToken = token
				}
				if strings.EqualFold(strings.TrimSpace(err.Error()), "token_expired") {
					break
				}
				continue
			}
			if len(items) == 0 {
				err = fmt.Errorf("token_list_empty")
				rank := scoreProfileFetchError(endpoint, err)
				if rank >= bestErrRank {
					bestErr = err
					bestErrRank = rank
					bestErrToken = token
				}
				continue
			}
			return items, token, nil
		}
	}

	if bestErr == nil {
		if time.Now().After(deadline) {
			bestErr = fmt.Errorf("profile_fetch_timeout")
		} else {
			bestErr = fmt.Errorf("profile_fetch_no_tokens")
		}
	}
	return nil, bestErrToken, bestErr
}

func requestTokenListEndpoint(baseURL string, endpoint string, token string, baseHeaders map[string]string, deadline time.Time) ([]map[string]interface{}, error) {
	urlValue := baseURL + endpoint
	headers := cloneHeaderMap(baseHeaders)
	headers["Authorization"] = "Bearer " + token

	body, err := doProfileJSONRequest(http.MethodGet, urlValue, headers, nil, profileRequestTimeout(deadline))
	if err != nil {
		return nil, err
	}

	items := extractProfileListItems(body)
	if len(items) == 0 {
		return nil, fmt.Errorf("token_list_empty")
	}

	resolvedItems := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		rawKey := extractProfileTokenValue(item)
		resolvedKey := rawKey
		unresolved := false
		tokenID := toStringValue(item["id"])

		if isMaskedProfileToken(resolvedKey) && tokenID != "" && time.Now().Before(deadline) {
			if fullKey := resolveMaskedProfileKey(baseURL, tokenID, token, baseHeaders, deadline); fullKey != "" {
				resolvedKey = fullKey
			}
		}
		unresolved = isMaskedProfileToken(resolvedKey)

		normalized := cloneAnyMap(item)
		if strings.TrimSpace(resolvedKey) != "" {
			normalized["key"] = strings.TrimSpace(resolvedKey)
		}
		if unresolved {
			normalized["unresolved"] = true
			normalized["masked"] = true
		}
		resolvedItems = append(resolvedItems, normalized)
	}

	return resolvedItems, nil
}

func resolveMaskedProfileKey(baseURL string, tokenID string, token string, baseHeaders map[string]string, deadline time.Time) string {
	endpoints := []struct {
		Path   string
		Method string
	}{
		{Path: fmt.Sprintf("/api/token/%s/key", tokenID), Method: http.MethodPost},
		{Path: fmt.Sprintf("/api/token/%s/key", tokenID), Method: http.MethodGet},
		{Path: fmt.Sprintf("/api/token/%s", tokenID), Method: http.MethodGet},
		{Path: fmt.Sprintf("/api/v1/keys/%s", tokenID), Method: http.MethodGet},
	}

	for _, endpoint := range endpoints {
		if time.Now().After(deadline) {
			return ""
		}
		headers := cloneHeaderMap(baseHeaders)
		headers["Authorization"] = "Bearer " + token
		if endpoint.Method != http.MethodGet {
			headers["Content-Type"] = "application/json"
		}
		body, err := doProfileJSONRequest(endpoint.Method, baseURL+endpoint.Path, headers, nil, profileRequestTimeout(deadline))
		if err != nil {
			continue
		}
		if key := extractSecretKeyFromBody(body); key != "" {
			return key
		}
	}
	return ""
}

func tryRefreshProfileToken(account ChromeProfileAccount, tokenCandidates []string, userID string, refreshToken string, tokenExpiresAt int64, deadline time.Time) string {
	if account.SiteType != "sub2api" {
		return ""
	}
	if strings.TrimSpace(refreshToken) == "" {
		return ""
	}
	if !shouldTryRefreshProfileToken(tokenCandidates, tokenExpiresAt) {
		return ""
	}

	baseURL := strings.TrimRight(strings.TrimSpace(account.SiteURL), "/")
	if baseURL == "" {
		return ""
	}
	headers := map[string]string{
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Content-Type":    "application/json",
	}
	for key, value := range buildCompatHeaders(userID) {
		headers[key] = value
	}
	if len(tokenCandidates) > 0 {
		headers["Authorization"] = "Bearer " + strings.TrimSpace(strings.TrimPrefix(tokenCandidates[0], "Bearer "))
	}

	payload := map[string]string{"refresh_token": refreshToken}
	bodyBytes, _ := json.Marshal(payload)
	body, err := doProfileJSONRequest(http.MethodPost, baseURL+"/api/v1/auth/refresh", headers, bodyBytes, profileRequestTimeout(deadline))
	if err != nil {
		debugLogf("profile token refresh failed: site=%s err=%v", account.SiteURL, err)
		return ""
	}

	if refreshed := firstNonEmpty(
		getNestedString(body, "data", "access_token"),
		getNestedString(body, "access_token"),
	); refreshed != "" {
		debugLogf("profile token refresh succeeded: site=%s", account.SiteURL)
		return strings.TrimSpace(strings.TrimPrefix(refreshed, "Bearer "))
	}
	return ""
}

func getProfileTokenEndpoints(siteType string) []string {
	if siteType == "anyrouter" {
		return []string{
			"/api/token/?p=0&size=100",
			"/api/token?p=0&size=100",
		}
	}
	if siteType == "sub2api" {
		return []string{
			"/api/v1/keys?page=1&page_size=100",
			"/api/v1/keys?p=0&size=100",
			"/api/token/?p=0&size=100",
			"/api/token?p=0&size=100",
		}
	}
	return []string{
		"/api/token/?p=0&size=100",
		"/api/token?p=0&size=100",
		"/api/v1/keys?page=1&page_size=100",
		"/api/v1/keys?p=0&size=100",
	}
}

func buildTokenCandidates(storageValues map[string]string, authUserObj map[string]interface{}, userObj map[string]interface{}, fallbackToken string) []string {
	ordered := []string{
		storageValues["auth_token"],
		storageValues["access_token"],
		storageValues["token"],
		storageValues["authToken"],
		getStringValue(userObj["access_token"]),
		getStringValue(authUserObj["access_token"]),
		fallbackToken,
	}

	seen := map[string]bool{}
	candidates := make([]string, 0, len(ordered))
	for _, raw := range ordered {
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		candidates = append(candidates, token)
	}
	return candidates
}

func buildCompatHeaders(userID string) map[string]string {
	if !isDigitsOnly(userID) {
		return map[string]string{}
	}
	return map[string]string{
		"one-api-user": userID,
		"New-API-User": userID,
		"Veloera-User": userID,
		"voapi-user":   userID,
		"User-id":      userID,
		"Rix-Api-User": userID,
		"neo-api-user": userID,
	}
}

func doProfileJSONRequest(method string, urlValue string, headers map[string]string, body []byte, timeout time.Duration) (interface{}, error) {
	var bodyReader *bytes.Reader
	if body == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, urlValue, bodyReader)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	if timeout <= 0 {
		timeout = profileMinRequestTimeout
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "html") {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("http_%d", resp.StatusCode)
		}
		return nil, fmt.Errorf("html_response")
	}

	var payload interface{}
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("http_%d", resp.StatusCode)
			}
			return nil, fmt.Errorf("json_decode_failed")
		}
	} else {
		payload = map[string]interface{}{}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if bodyMap, ok := payload.(map[string]interface{}); ok {
			if classified := classifyProfileHTTPError(resp.StatusCode, bodyMap); classified != nil {
				return nil, classified
			}
		}
		return nil, fmt.Errorf("http_%d", resp.StatusCode)
	}

	if bodyMap, ok := payload.(map[string]interface{}); ok {
		if code, exists := bodyMap["code"]; exists {
			if codeNumber, ok := toFloat64(code); ok && codeNumber != 0 {
				return nil, fmt.Errorf("business_code_%v", code)
			}
		}
		if success, exists := bodyMap["success"]; exists {
			if successBool, ok := success.(bool); ok && !successBool {
				return nil, classifyProfileBusinessError(bodyMap)
			}
		}
	}

	return payload, nil
}

func profileRequestTimeout(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining <= profileMinRequestTimeout {
		return profileMinRequestTimeout
	}
	if remaining < profilePerRequestTimeout {
		return remaining
	}
	return profilePerRequestTimeout
}

func extractProfileListItems(body interface{}) []map[string]interface{} {
	items := []interface{}{}
	switch value := body.(type) {
	case []interface{}:
		items = value
	case map[string]interface{}:
		items = toInterfaceSlice(value["items"])
		if len(items) == 0 {
			items = toInterfaceSlice(value["data"])
		}
		if len(items) == 0 {
			if nested, ok := value["data"].(map[string]interface{}); ok {
				items = toInterfaceSlice(nested["items"])
				if len(items) == 0 {
					items = toInterfaceSlice(nested["data"])
				}
			}
		}
	}

	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case map[string]interface{}:
			results = append(results, value)
		case string:
			results = append(results, map[string]interface{}{"key": value})
		}
	}
	return results
}

func extractProfileTokenValue(item map[string]interface{}) string {
	for _, key := range []string{"key", "access_token", "token", "api_key", "apikey"} {
		if value := strings.TrimSpace(toStringValue(item[key])); value != "" {
			return value
		}
	}
	return ""
}

func extractSecretKeyFromBody(body interface{}) string {
	for _, path := range [][]string{
		{"key"},
		{"data", "key"},
		{"data"},
		{"result", "key"},
		{"result", "data", "key"},
		{"token"},
	} {
		if value := getNestedString(body, path...); value != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseProfileStorageKey(raw []byte) (string, string, bool) {
	if len(raw) < 3 || raw[0] != '_' {
		return "", "", false
	}
	body := raw[1:]
	sep := bytes.IndexByte(body, 0)
	if sep <= 0 || sep+1 >= len(body) {
		return "", "", false
	}

	origin := strings.TrimSpace(string(body[:sep]))
	keyBytes := body[sep+1:]
	if len(keyBytes) > 0 && keyBytes[0] == 1 {
		keyBytes = keyBytes[1:]
	}
	storageKey := strings.TrimSpace(string(keyBytes))
	if origin == "" || storageKey == "" {
		return "", "", false
	}
	return origin, storageKey, true
}

func decodeProfileStorageValue(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == 1 {
		raw = raw[1:]
	}
	if len(raw) == 0 {
		return ""
	}
	if utf8.Valid(raw) {
		return strings.TrimSpace(string(raw))
	}
	if text, ok := decodeUTF16LE(raw); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(bytes.ToValidUTF8(raw, nil)))
}

func decodeUTF16LE(raw []byte) (string, bool) {
	if len(raw) < 2 || len(raw)%2 != 0 {
		return "", false
	}
	u16 := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		u16 = append(u16, uint16(raw[i])|uint16(raw[i+1])<<8)
	}
	text := strings.TrimSpace(string(utf16.Decode(u16)))
	if text == "" {
		return "", false
	}
	return text, true
}

func normalizeStorageOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	if parsed, err := url.Parse(origin); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host
	}
	return origin
}

func normalizeURLOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("missing scheme or host")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func parseJSONMap(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func extractUserID(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"id", "user_id", "userId"} {
		if value := normalizeUserID(payload[key]); value != "" {
			return value
		}
	}
	if nested, ok := payload["user"].(map[string]interface{}); ok {
		if value := normalizeUserID(nested["id"]); value != "" {
			return value
		}
	}
	return ""
}

func normalizeUserID(value interface{}) string {
	text := strings.TrimSpace(toStringValue(value))
	if !isDigitsOnly(text) {
		return ""
	}
	return text
}

func isDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func toInterfaceSlice(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if items, ok := value.([]interface{}); ok {
		return items
	}
	return nil
}

func cloneAnyMap(input map[string]interface{}) map[string]interface{} {
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneHeaderMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func getNestedString(root interface{}, path ...string) string {
	current := root
	for _, key := range path {
		object, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = object[key]
	}
	return strings.TrimSpace(toStringValue(current))
}

func toStringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		f := float64(v)
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(value)
	}
}

func getStringValue(value interface{}) string {
	return strings.TrimSpace(toStringValue(value))
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func parseInt64Loose(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		return number
	}
	return 0
}

func normalizeProfileTokenExpiryMillis(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func shouldTryRefreshProfileToken(tokenCandidates []string, tokenExpiresAt int64) bool {
	nowMillis := time.Now().UnixMilli()
	normalizedExpiry := normalizeProfileTokenExpiryMillis(tokenExpiresAt)
	if normalizedExpiry > 0 {
		return normalizedExpiry-nowMillis <= 120000
	}

	for _, token := range tokenCandidates {
		if expiryMillis := extractJWTExpiryMillis(token); expiryMillis > 0 {
			return expiryMillis-nowMillis <= 120000
		}
	}

	return false
}

func extractJWTExpiryMillis(token string) int64 {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return 0
	}

	expValue, ok := payload["exp"]
	if !ok {
		return 0
	}

	switch value := expValue.(type) {
	case float64:
		return int64(value * 1000)
	case int64:
		return value * 1000
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return parsed * 1000
		}
	}

	return 0
}

func classifyProfileTokenCandidate(token string) error {
	expiryMillis := extractJWTExpiryMillis(token)
	if expiryMillis <= 0 {
		return nil
	}
	if time.Now().UnixMilli() > expiryMillis+profileTokenExpiryLeeway.Milliseconds() {
		return fmt.Errorf("token_expired_local")
	}
	return nil
}

func scoreProfileFetchError(endpoint string, err error) int {
	if err == nil {
		return -1
	}

	code := strings.TrimSpace(strings.ToLower(err.Error()))
	switch code {
	case "token_expired_local":
		return 145
	case "token_expired":
		return 140
	case "user_banned":
		return 130
	case "http_401":
		return 120
	case "http_403":
		return 110
	case "business_success_false":
		return 100
	case "html_response":
		return 90
	case "token_list_empty":
		return 80
	case "http_404":
		if strings.Contains(endpoint, "/api/v1/keys") {
			return 70
		}
		return 20
	}

	if strings.HasPrefix(code, "business_code_") {
		return 85
	}
	if strings.HasPrefix(code, "http_5") {
		return 60
	}
	if strings.HasPrefix(code, "http_4") {
		return 50
	}

	return 40
}

func classifyProfileBusinessError(bodyMap map[string]interface{}) error {
	messageText := firstNonEmpty(
		toStringValue(bodyMap["message"]),
		toStringValue(bodyMap["msg"]),
		getNestedString(bodyMap, "error", "message"),
	)
	lowerMessage := strings.ToLower(strings.TrimSpace(messageText))
	codeText := strings.ToLower(strings.TrimSpace(toStringValue(bodyMap["code"])))
	switch {
	case codeText == "token_expired" || strings.Contains(lowerMessage, "token has expired") || strings.Contains(messageText, "令牌已过期") || strings.Contains(messageText, "token已过期"):
		return fmt.Errorf("token_expired")
	case strings.Contains(messageText, "封禁") || strings.Contains(lowerMessage, "banned"):
		return fmt.Errorf("user_banned")
	case strings.Contains(messageText, "余额不足") || strings.Contains(lowerMessage, "insufficient balance"):
		return fmt.Errorf("insufficient_balance")
	case strings.Contains(messageText, "无权") || strings.Contains(lowerMessage, "unauthorized"):
		return fmt.Errorf("http_401")
	}
	return fmt.Errorf("business_success_false")
}

func classifyProfileHTTPError(statusCode int, bodyMap map[string]interface{}) error {
	codeText := strings.ToLower(strings.TrimSpace(toStringValue(bodyMap["code"])))
	messageText := firstNonEmpty(
		toStringValue(bodyMap["message"]),
		toStringValue(bodyMap["msg"]),
		getNestedString(bodyMap, "error", "message"),
	)
	lowerMessage := strings.ToLower(strings.TrimSpace(messageText))

	switch {
	case codeText == "token_expired" || strings.Contains(lowerMessage, "token has expired") || strings.Contains(messageText, "令牌已过期") || strings.Contains(messageText, "token已过期"):
		return fmt.Errorf("token_expired")
	case strings.Contains(messageText, "封禁") || strings.Contains(lowerMessage, "banned"):
		return fmt.Errorf("user_banned")
	}

	return fmt.Errorf("http_%d", statusCode)
}

func prependUnique(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	output := []string{value}
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			continue
		}
		output = append(output, item)
	}
	return output
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isMaskedProfileToken(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && strings.Contains(value, "*")
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func copyDirToTemp(src string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "chrome-profile-auth-*")
	if err != nil {
		return "", nil, err
	}

	dst := filepath.Join(tmpDir, filepath.Base(src))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o600); err != nil {
			continue
		}
	}

	return dst, func() { _ = os.RemoveAll(tmpDir) }, nil
}
