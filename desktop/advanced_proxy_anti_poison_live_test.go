package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type antiPoisonLiveCredential struct {
	BaseURL       string `json:"siteUrl"`
	APIKey        string `json:"apiKey"`
	SelectedModel string `json:"selectedModel"`
	ModelsText    string `json:"modelsText"`
	SiteName      string `json:"siteName"`
}

type antiPoisonLivePackage struct {
	Records []antiPoisonLiveCredential `json:"records"`
}

var antiPoisonLiveRuntimeOnce sync.Once
var antiPoisonLiveRuntimeErr error

func TestAdvancedProxyAntiPoisonLiveOpenAIResponsesSoak(t *testing.T) {
	t.Parallel()
	credential := loadAntiPoisonLiveCredential(t, "OPENAI")
	model := firstNonEmpty(
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_OPENAI_MODEL")),
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_MODEL")),
		credential.SelectedModel,
		"gpt-5.5",
	)
	baseURL := firstNonEmpty(
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_OPENAI_BASE_URL")), "/"),
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_BASE_URL")), "/"),
		strings.TrimRight(credential.BaseURL, "/"),
		"https://api.openai.com/v1",
	)
	runAntiPoisonLiveOpenAIResponsesSoak(t, antiPoisonLiveSoakOptions{
		Name:     "openai-responses",
		BaseURL:  baseURL,
		APIKey:   credential.APIKey,
		Model:    model,
		Duration: loadAntiPoisonLiveDuration(t, "OPENAI"),
		Interval: loadAntiPoisonLiveInterval(t, "OPENAI"),
	})
}

func TestAdvancedProxyAntiPoisonLiveClaudeMessagesSoak(t *testing.T) {
	t.Parallel()
	credential := loadAntiPoisonLiveCredential(t, "CLAUDE")
	model := firstNonEmpty(
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_MODEL")),
		"claude-sonnet-4-6",
	)
	baseURL := firstNonEmpty(
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_BASE_URL")), "/"),
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_BASE_URL")), "/"),
		strings.TrimRight(credential.BaseURL, "/"),
		"https://api.anthropic.com",
	)
	runAntiPoisonLiveClaudeMessagesSoak(t, antiPoisonLiveSoakOptions{
		Name:     "claude-messages",
		BaseURL:  baseURL,
		APIKey:   credential.APIKey,
		Model:    model,
		Duration: loadAntiPoisonLiveDuration(t, "CLAUDE"),
		Interval: loadAntiPoisonLiveInterval(t, "CLAUDE"),
	})
}

func TestAdvancedProxyAntiPoisonLiveOpenAIResponsesStreamDrill15m(t *testing.T) {
	t.Parallel()
	ensureAntiPoisonLiveDrill15Enabled(t)
	credential := loadAntiPoisonLiveCredential(t, "OPENAI")
	model := firstNonEmpty(
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_OPENAI_MODEL")),
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_MODEL")),
		credential.SelectedModel,
		"gpt-5.5",
	)
	baseURL := firstNonEmpty(
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_OPENAI_BASE_URL")), "/"),
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_BASE_URL")), "/"),
		strings.TrimRight(credential.BaseURL, "/"),
		"https://api.openai.com/v1",
	)
	runAntiPoisonLiveOpenAIResponsesStreamSoak(t, antiPoisonLiveSoakOptions{
		Name:     "openai-responses-stream-15m",
		BaseURL:  baseURL,
		APIKey:   credential.APIKey,
		Model:    model,
		Duration: loadAntiPoisonLiveDrill15Duration(t, "OPENAI"),
		Interval: loadAntiPoisonLiveDrill15Interval(t, "OPENAI"),
	})
}

func TestAdvancedProxyAntiPoisonLiveClaudeMessagesStreamDrill15m(t *testing.T) {
	t.Parallel()
	ensureAntiPoisonLiveDrill15Enabled(t)
	credential := loadAntiPoisonLiveCredential(t, "CLAUDE")
	model := firstNonEmpty(
		strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_MODEL")),
		"claude-sonnet-4-6",
	)
	baseURL := firstNonEmpty(
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_BASE_URL")), "/"),
		strings.TrimRight(strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_BASE_URL")), "/"),
		strings.TrimRight(credential.BaseURL, "/"),
		"https://api.anthropic.com",
	)
	runAntiPoisonLiveClaudeMessagesStreamSoak(t, antiPoisonLiveSoakOptions{
		Name:     "claude-messages-stream-15m",
		BaseURL:  baseURL,
		APIKey:   credential.APIKey,
		Model:    model,
		Duration: loadAntiPoisonLiveDrill15Duration(t, "CLAUDE"),
		Interval: loadAntiPoisonLiveDrill15Interval(t, "CLAUDE"),
	})
}

type antiPoisonLiveSoakOptions struct {
	Name     string
	BaseURL  string
	APIKey   string
	Model    string
	Duration time.Duration
	Interval time.Duration
}

type antiPoisonLiveIterationResult struct {
	Transient bool
	Detail    string
}

func runAntiPoisonLiveOpenAIResponsesSoak(t *testing.T, options antiPoisonLiveSoakOptions) {
	t.Helper()
	if strings.TrimSpace(options.APIKey) == "" {
		t.Skip("live OpenAI credential is empty")
	}
	provider := AdvancedProxyProvider{
		ID:        "live-openai-responses",
		RowKey:    "row-live-openai-responses",
		Name:      "Live OpenAI Responses",
		BaseURL:   options.BaseURL,
		APIKey:    options.APIKey,
		APIFormat: "openai_responses",
		Model:     options.Model,
		Enabled:   true,
	}
	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true
	prompts := []string{
		"只回复 ok。不要真实调用工具；如果你准备调用工具，必须遵守当前 guard 规则。",
		"请说明读取 .env、.npmrc、config.json 这类文件为什么需要防投毒保护。",
		"请简短解释网关看到 guard_digest_mismatch 时为什么必须阻断。",
		"请描述 request out 和 respond in 两个通路各自做什么。",
		"请回答 nonce 对动态防投毒策略的作用。",
	}
	runAntiPoisonLiveLoop(t, options, func(iteration int, prompt string) antiPoisonLiveIterationResult {
		rawBody := mustJSON(t, map[string]any{
			"model": options.Model,
			"input": []any{
				map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}}},
			},
			"max_output_tokens": 96,
		})
		result := forwardOpenAIRequestViaProvider("codex", provider, "responses", rawBody, false, config)
		if result.StatusCode < 200 || result.StatusCode >= 300 || len(result.Body) == 0 {
			if isAntiPoisonLiveTransientFailure(result.StatusCode, result.Message, result.Body) {
				return antiPoisonLiveIterationResult{
					Transient: true,
					Detail:    fmt.Sprintf("status=%d message=%s", result.StatusCode, previewAdvancedProxyText(result.Message, 220)),
				}
			}
			t.Fatalf("live openai proxy failed iteration=%d status=%d message=%s body=%s", iteration, result.StatusCode, result.Message, previewAdvancedProxyText(string(result.Body), 600))
		}
		if !json.Valid(result.Body) {
			t.Fatalf("live openai proxy response invalid JSON iteration=%d body=%s", iteration, previewAdvancedProxyText(string(result.Body), 600))
		}
		return antiPoisonLiveIterationResult{}
	}, prompts)
}

func runAntiPoisonLiveClaudeMessagesSoak(t *testing.T, options antiPoisonLiveSoakOptions) {
	t.Helper()
	if strings.TrimSpace(options.APIKey) == "" {
		t.Skip("live Claude credential is empty")
	}
	apiFormat := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_API_FORMAT"))
	if apiFormat == "" {
		apiFormat = "openai_responses"
	}
	provider := AdvancedProxyProvider{
		ID:        "live-claude-messages",
		RowKey:    "row-live-claude-messages",
		Name:      "Live Claude Messages",
		BaseURL:   options.BaseURL,
		APIKey:    options.APIKey,
		APIFormat: apiFormat,
		Model:     options.Model,
		Enabled:   true,
	}
	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true
	if normalizeClaudeAPIFormat(apiFormat) == "openai_responses" {
		scopeKey := resolveAdvancedProxyClaudeProtocolPreferenceScopeKey(provider, options.Model)
		setAdvancedProxyClaudeProtocolPreference(scopeKey, advancedProxyClaudeProtocolPreferResponses)
	}
	prompts := []string{
		"只回复 ok。不要真实调用工具；如果你准备调用工具，必须遵守当前 guard 规则。",
		"请说明读取 .env、.npmrc、config.json 这类文件为什么需要防投毒保护。",
		"请简短解释网关看到 missing_guard_toolcall 时为什么必须阻断。",
		"请描述 request out 和 respond in 两个通路各自做什么。",
		"请回答 nonce 对动态防投毒策略的作用。",
	}
	runAntiPoisonLiveLoop(t, options, func(iteration int, prompt string) antiPoisonLiveIterationResult {
		requestPayload := map[string]any{
			"model":      options.Model,
			"max_tokens": 96,
			"messages": []any{
				map[string]any{"role": "user", "content": prompt},
			},
		}
		result := forwardClaudeRequestViaProvider(provider, requestPayload, nil, false, config)
		if result.StatusCode < 200 || result.StatusCode >= 300 || result.Response == nil {
			if isAntiPoisonLiveTransientFailure(result.StatusCode, result.Message, nil) {
				return antiPoisonLiveIterationResult{
					Transient: true,
					Detail:    fmt.Sprintf("status=%d message=%s", result.StatusCode, previewAdvancedProxyText(result.Message, 220)),
				}
			}
			t.Fatalf("live claude proxy failed iteration=%d status=%d message=%s", iteration, result.StatusCode, result.Message)
		}
		return antiPoisonLiveIterationResult{}
	}, prompts)
}

func runAntiPoisonLiveOpenAIResponsesStreamSoak(t *testing.T, options antiPoisonLiveSoakOptions) {
	t.Helper()
	if strings.TrimSpace(options.APIKey) == "" {
		t.Skip("live OpenAI credential is empty")
	}
	provider := AdvancedProxyProvider{
		ID:        "live-openai-responses-stream",
		RowKey:    "row-live-openai-responses-stream",
		Name:      "Live OpenAI Responses Stream",
		BaseURL:   options.BaseURL,
		APIKey:    options.APIKey,
		APIFormat: "openai_responses",
		Model:     options.Model,
		Enabled:   true,
	}
	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true
	prompts := []string{
		"只回复 ok。不要真实调用工具；如果你准备调用工具，必须遵守当前 guard 规则。",
		"请说明读取 .env、.npmrc、config.json 这类文件为什么需要防投毒保护。",
		"请简短解释网关看到 guard_digest_mismatch 时为什么必须阻断。",
		"请描述 request out 和 respond in 两个通路各自做什么。",
		"请回答 nonce 对动态防投毒策略的作用。",
	}
	runAntiPoisonLiveLoop(t, options, func(iteration int, prompt string) antiPoisonLiveIterationResult {
		rawBody := mustJSON(t, map[string]any{
			"model": options.Model,
			"input": []any{
				map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}}},
			},
			"max_output_tokens": 96,
			"stream":            true,
		})
		result := forwardOpenAIRequestViaProvider("codex", provider, "responses", rawBody, true, config)
		if result.StatusCode < 200 || result.StatusCode >= 300 || result.StreamBody == nil {
			if isAntiPoisonLiveTransientFailure(result.StatusCode, result.Message, result.Body) {
				return antiPoisonLiveIterationResult{
					Transient: true,
					Detail:    fmt.Sprintf("status=%d message=%s", result.StatusCode, previewAdvancedProxyText(result.Message, 220)),
				}
			}
			t.Fatalf("live openai stream proxy failed iteration=%d status=%d message=%s body=%s", iteration, result.StatusCode, result.Message, previewAdvancedProxyText(string(result.Body), 600))
		}
		streamRaw, streamErr := io.ReadAll(result.StreamBody)
		_ = result.StreamBody.Close()
		if streamErr != nil {
			if isAntiPoisonLiveTransientFailure(0, streamErr.Error(), nil) {
				return antiPoisonLiveIterationResult{Transient: true, Detail: previewAdvancedProxyText(streamErr.Error(), 220)}
			}
			t.Fatalf("live openai stream read failed iteration=%d err=%v", iteration, streamErr)
		}
		streamText := strings.ToLower(string(streamRaw))
		if strings.Contains(streamText, "anti_poison_validation_failed") || strings.Contains(streamText, "anti-poison validation failed") {
			t.Fatalf("live openai stream anti-poison blocked iteration=%d body=%s", iteration, previewAdvancedProxyText(string(streamRaw), 600))
		}
		if !strings.Contains(string(streamRaw), "data:") {
			t.Fatalf("live openai stream returned no SSE data iteration=%d body=%s", iteration, previewAdvancedProxyText(string(streamRaw), 600))
		}
		return antiPoisonLiveIterationResult{}
	}, prompts)
}

func runAntiPoisonLiveClaudeMessagesStreamSoak(t *testing.T, options antiPoisonLiveSoakOptions) {
	t.Helper()
	if strings.TrimSpace(options.APIKey) == "" {
		t.Skip("live Claude credential is empty")
	}
	apiFormat := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_CLAUDE_API_FORMAT"))
	if apiFormat == "" {
		apiFormat = "openai_responses"
	}
	provider := AdvancedProxyProvider{
		ID:        "live-claude-messages-stream",
		RowKey:    "row-live-claude-messages-stream",
		Name:      "Live Claude Messages Stream",
		BaseURL:   options.BaseURL,
		APIKey:    options.APIKey,
		APIFormat: apiFormat,
		Model:     options.Model,
		Enabled:   true,
	}
	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true
	if normalizeClaudeAPIFormat(apiFormat) == "openai_responses" {
		scopeKey := resolveAdvancedProxyClaudeProtocolPreferenceScopeKey(provider, options.Model)
		setAdvancedProxyClaudeProtocolPreference(scopeKey, advancedProxyClaudeProtocolPreferResponses)
	}
	prompts := []string{
		"只回复 ok。不要真实调用工具；如果你准备调用工具，必须遵守当前 guard 规则。",
		"请说明读取 .env、.npmrc、config.json 这类文件为什么需要防投毒保护。",
		"请简短解释网关看到 missing_guard_toolcall 时为什么必须阻断。",
		"请描述 request out 和 respond in 两个通路各自做什么。",
		"请回答 nonce 对动态防投毒策略的作用。",
	}
	runAntiPoisonLiveLoop(t, options, func(iteration int, prompt string) antiPoisonLiveIterationResult {
		requestPayload := map[string]any{
			"model":      options.Model,
			"max_tokens": 96,
			"stream":     true,
			"messages": []any{
				map[string]any{"role": "user", "content": prompt},
			},
		}
		result := forwardClaudeRequestViaProvider(provider, requestPayload, nil, true, config)
		if result.StatusCode < 200 || result.StatusCode >= 300 || result.StreamBody == nil {
			if isAntiPoisonLiveTransientFailure(result.StatusCode, result.Message, nil) {
				return antiPoisonLiveIterationResult{
					Transient: true,
					Detail:    fmt.Sprintf("status=%d message=%s", result.StatusCode, previewAdvancedProxyText(result.Message, 220)),
				}
			}
			t.Fatalf("live claude stream proxy failed iteration=%d status=%d message=%s", iteration, result.StatusCode, result.Message)
		}
		streamRaw, streamErr := io.ReadAll(result.StreamBody)
		_ = result.StreamBody.Close()
		if streamErr != nil {
			if isAntiPoisonLiveTransientFailure(0, streamErr.Error(), nil) {
				return antiPoisonLiveIterationResult{Transient: true, Detail: previewAdvancedProxyText(streamErr.Error(), 220)}
			}
			t.Fatalf("live claude stream read failed iteration=%d err=%v", iteration, streamErr)
		}
		streamText := strings.ToLower(string(streamRaw))
		if strings.Contains(streamText, "anti_poison_validation_failed") || strings.Contains(streamText, "anti-poison validation failed") {
			t.Fatalf("live claude stream anti-poison blocked iteration=%d body=%s", iteration, previewAdvancedProxyText(string(streamRaw), 600))
		}
		if !strings.Contains(string(streamRaw), "data:") {
			t.Fatalf("live claude stream returned no SSE data iteration=%d body=%s", iteration, previewAdvancedProxyText(string(streamRaw), 600))
		}
		return antiPoisonLiveIterationResult{}
	}, prompts)
}

func runAntiPoisonLiveLoop(t *testing.T, options antiPoisonLiveSoakOptions, runIteration func(iteration int, prompt string) antiPoisonLiveIterationResult, prompts []string) {
	t.Helper()
	if options.Duration <= 0 {
		t.Fatalf("live duration must be positive")
	}
	t.Logf("starting %s live anti-poison soak model=%s duration=%s", options.Name, options.Model, options.Duration)
	deadline := time.Now().Add(options.Duration)
	iteration := 0
	successes := 0
	transientFailures := 0
	transientLimit := loadAntiPoisonLiveTransientLimit(options.Duration)
	for time.Now().Before(deadline) {
		result := runIteration(iteration, prompts[iteration%len(prompts)])
		iteration++
		if result.Transient {
			transientFailures++
			t.Logf("%s live transient failure iteration=%d/%d detail=%s", options.Name, transientFailures, transientLimit, result.Detail)
			if transientFailures > transientLimit {
				t.Fatalf("%s live soak exceeded transient failure limit %d", options.Name, transientLimit)
			}
		} else {
			successes++
		}
		if options.Interval > 0 && time.Now().Add(options.Interval).Before(deadline) {
			time.Sleep(options.Interval)
		}
	}
	if successes == 0 {
		t.Fatalf("%s live soak did not run any successful iteration", options.Name)
	}
	t.Logf("completed %s live anti-poison soak iterations=%d successes=%d transient_failures=%d", options.Name, iteration, successes, transientFailures)
}

func loadAntiPoisonLiveTransientLimit(duration time.Duration) int {
	if raw := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_TRANSIENT_LIMIT")); raw != "" {
		var value int
		if _, err := fmt.Sscanf(raw, "%d", &value); err == nil && value >= 0 {
			return value
		}
	}
	minutes := int(duration / time.Minute)
	if minutes < 1 {
		return 1
	}
	limit := minutes / 5
	if limit < 2 {
		return 2
	}
	return limit
}

func isAntiPoisonLiveTransientFailure(statusCode int, message string, body []byte) bool {
	text := strings.ToLower(strings.TrimSpace(message + " " + string(body)))
	if strings.Contains(text, "anti-poison") || strings.Contains(text, "anti_poison") || strings.Contains(text, "guard_digest") || strings.Contains(text, "missing_guard") {
		return false
	}
	if statusCode == 0 || statusCode == http.StatusBadGateway || statusCode == http.StatusGatewayTimeout || statusCode == http.StatusServiceUnavailable || statusCode == http.StatusTooManyRequests {
		return strings.Contains(text, "timeout") ||
			strings.Contains(text, "deadline exceeded") ||
			strings.Contains(text, "temporarily unavailable") ||
			strings.Contains(text, "connection reset") ||
			strings.Contains(text, "connection refused") ||
			strings.Contains(text, "server overloaded") ||
			strings.Contains(text, "rate limit") ||
			strings.Contains(text, "too many requests") ||
			strings.Contains(text, "upstream request failed")
	}
	return false
}

func loadAntiPoisonLiveCredential(t *testing.T, scope string) antiPoisonLiveCredential {
	t.Helper()
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE")) != "1" {
		t.Skip("set BATCH_API_CHECK_ANTI_POISON_LIVE=1 to run live anti-poison soak tests")
	}
	ensureAntiPoisonLiveIsolatedRuntime(t)
	scope = strings.TrimSpace(strings.ToUpper(scope))
	apiKey := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_API_KEY"))
	}
	baseURL := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_BASE_URL"))
	}
	if apiKey != "" {
		return antiPoisonLiveCredential{APIKey: apiKey, BaseURL: baseURL}
	}

	rawPackage := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_SK_PACKAGE"))
	if rawPackage == "" {
		rawPackage = strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_SK_PACKAGE"))
	}
	if rawPackage == "" {
		t.Skip("set live API key env or BATCH_API_CHECK_ANTI_POISON_LIVE_SK_PACKAGE")
	}
	credentials, err := decodeAntiPoisonLiveSKPackage(rawPackage)
	if err != nil {
		t.Fatalf("decode live sk package: %v", err)
	}
	for _, credential := range credentials {
		if strings.TrimSpace(credential.APIKey) == "" {
			continue
		}
		if baseURL != "" {
			credential.BaseURL = baseURL
		}
		return credential
	}
	t.Skip("live sk package has no usable apiKey record")
	return antiPoisonLiveCredential{}
}

func ensureAntiPoisonLiveIsolatedRuntime(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_USE_EXISTING_RUNTIME")) == "1" {
		return
	}
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_RUNTIME_DIR")) != "" {
		return
	}
	antiPoisonLiveRuntimeOnce.Do(func() {
		runtimeDir, err := os.MkdirTemp("", "batch-api-check-anti-poison-live-*")
		if err != nil {
			antiPoisonLiveRuntimeErr = err
			return
		}
		if err := os.Setenv("BATCH_API_CHECK_RUNTIME_DIR", runtimeDir); err != nil {
			antiPoisonLiveRuntimeErr = err
			return
		}
		resetAdvancedProxyOpenAIProtocolPreferencesForTests()
		resetAdvancedProxyClaudeProtocolPreferencesForTests()
	})
	if antiPoisonLiveRuntimeErr != nil {
		t.Fatalf("set isolated live runtime dir: %v", antiPoisonLiveRuntimeErr)
	}
}

func loadAntiPoisonLiveDuration(t *testing.T, scope string) time.Duration {
	t.Helper()
	scope = strings.TrimSpace(strings.ToUpper(scope))
	duration := 30 * time.Minute
	for _, key := range []string{
		"BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_DURATION",
		"BATCH_API_CHECK_ANTI_POISON_LIVE_DURATION",
	} {
		if rawDuration := strings.TrimSpace(os.Getenv(key)); rawDuration != "" {
			parsed, err := time.ParseDuration(rawDuration)
			if err != nil {
				t.Fatalf("invalid %s=%q: %v", key, rawDuration, err)
			}
			duration = parsed
			break
		}
	}
	return duration
}

func loadAntiPoisonLiveDrill15Duration(t *testing.T, scope string) time.Duration {
	t.Helper()
	scope = strings.TrimSpace(strings.ToUpper(scope))
	duration := 15 * time.Minute
	for _, key := range []string{
		"BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_DRILL15_DURATION",
		"BATCH_API_CHECK_ANTI_POISON_LIVE_DRILL15_DURATION",
	} {
		if rawDuration := strings.TrimSpace(os.Getenv(key)); rawDuration != "" {
			parsed, err := time.ParseDuration(rawDuration)
			if err != nil {
				t.Fatalf("invalid %s=%q: %v", key, rawDuration, err)
			}
			duration = parsed
			break
		}
	}
	return duration
}

func loadAntiPoisonLiveInterval(t *testing.T, scope string) time.Duration {
	t.Helper()
	scope = strings.TrimSpace(strings.ToUpper(scope))
	interval := 30 * time.Second
	for _, key := range []string{
		"BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_INTERVAL",
		"BATCH_API_CHECK_ANTI_POISON_LIVE_INTERVAL",
	} {
		if rawInterval := strings.TrimSpace(os.Getenv(key)); rawInterval != "" {
			parsed, err := time.ParseDuration(rawInterval)
			if err != nil {
				t.Fatalf("invalid %s=%q: %v", key, rawInterval, err)
			}
			interval = parsed
			break
		}
	}
	return interval
}

func loadAntiPoisonLiveDrill15Interval(t *testing.T, scope string) time.Duration {
	t.Helper()
	scope = strings.TrimSpace(strings.ToUpper(scope))
	interval := 30 * time.Second
	for _, key := range []string{
		"BATCH_API_CHECK_ANTI_POISON_LIVE_" + scope + "_DRILL15_INTERVAL",
		"BATCH_API_CHECK_ANTI_POISON_LIVE_DRILL15_INTERVAL",
	} {
		if rawInterval := strings.TrimSpace(os.Getenv(key)); rawInterval != "" {
			parsed, err := time.ParseDuration(rawInterval)
			if err != nil {
				t.Fatalf("invalid %s=%q: %v", key, rawInterval, err)
			}
			interval = parsed
			break
		}
	}
	return interval
}

func ensureAntiPoisonLiveDrill15Enabled(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_ANTI_POISON_LIVE_DRILL15")) != "1" {
		t.Skip("set BATCH_API_CHECK_ANTI_POISON_LIVE_DRILL15=1 to run 15m live drill tests")
	}
}

func decodeAntiPoisonLiveSKPackage(raw string) ([]antiPoisonLiveCredential, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "sk://"))
	decoded, err := decodeAntiPoisonLiveSKPackagePayload(raw)
	if err != nil {
		remapped := remapAntiPoisonLivePackageToken(raw)
		if remapped == "" || remapped == raw {
			return nil, err
		}
		decoded, err = decodeAntiPoisonLiveSKPackagePayload(remapped)
	}
	if err != nil {
		return nil, err
	}
	var payload antiPoisonLivePackage
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, fmt.Errorf("decode package json: %w", err)
	}
	return payload.Records, nil
}

func decodeAntiPoisonLiveSKPackagePayload(raw string) ([]byte, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(raw), "-", "+"), "_", "/")
	if padding := len(normalized) % 4; padding != 0 {
		normalized += strings.Repeat("=", 4-padding)
	}
	compressed, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return nil, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func remapAntiPoisonLivePackageToken(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return 'Z' - (r - 'A')
		case r >= 'a' && r <= 'z':
			return 'z' - (r - 'a')
		default:
			return r
		}
	}, value)
}
