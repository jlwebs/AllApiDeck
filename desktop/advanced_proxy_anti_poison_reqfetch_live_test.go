package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type antiPoisonReqFetchVariant struct {
	Name            string
	StrategyPrompt  string
	AlgorithmPrompt string
	Placement       string
	PromptMode      string
}

type antiPoisonReqFetchExperimentSummary struct {
	Variant             string                     `json:"variant"`
	Attempt             int                        `json:"attempt"`
	StartedAt           string                     `json:"startedAt"`
	DurationMs          int64                      `json:"durationMs"`
	StatusCode          int                        `json:"statusCode"`
	TargetURL           string                     `json:"targetUrl"`
	Model               string                     `json:"model"`
	Alias               string                     `json:"alias"`
	Nonce               string                     `json:"nonce"`
	PromptApplied       bool                       `json:"promptApplied"`
	ValidationApplied   bool                       `json:"validationApplied"`
	ValidationValid     bool                       `json:"validationValid"`
	ValidationBlocked   bool                       `json:"validationBlocked"`
	ValidationReason    string                     `json:"validationReason"`
	RealCount           int                        `json:"realCount"`
	GuardCount          int                        `json:"guardCount"`
	RemovedGuards       int                        `json:"removedGuards"`
	UpstreamToolCalls   []string                   `json:"upstreamToolCalls,omitempty"`
	UpstreamToolArgs    []string                   `json:"upstreamToolArgs,omitempty"`
	AssistantPreview    string                     `json:"assistantPreview,omitempty"`
	LatestObserved      *advancedProxyObservedItem `json:"latestObserved,omitempty"`
	StreamEventCount    int                        `json:"streamEventCount"`
	RawGuardBlockCount  int                        `json:"rawGuardBlockCount"`
	PromptEchoGuardRefs int                        `json:"promptEchoGuardRefs"`
	SanitizedPath       string                     `json:"sanitizedPath,omitempty"`
	RawPath             string                     `json:"rawPath,omitempty"`
	InstructionsPath    string                     `json:"instructionsPath,omitempty"`
	RequestBodyPath     string                     `json:"requestBodyPath,omitempty"`
	Failure             string                     `json:"failure,omitempty"`
}

type antiPoisonReqFetchProxySummary struct {
	Variant                 string                      `json:"variant"`
	StartedAt               string                      `json:"startedAt"`
	TargetURL               string                      `json:"targetUrl"`
	Model                   string                      `json:"model"`
	Stream                  bool                        `json:"stream"`
	StatusCode              int                         `json:"statusCode"`
	AntiPoisonBlocked       bool                        `json:"antiPoisonBlocked"`
	ResponsePath            string                      `json:"responsePath,omitempty"`
	RequestBodyPath         string                      `json:"requestBodyPath,omitempty"`
	RecordCount             int                         `json:"recordCount"`
	HasRetryRecord          bool                        `json:"hasRetryRecord"`
	HasInitialFailure       bool                        `json:"hasInitialFailure"`
	HasMissingGuardEvidence bool                        `json:"hasMissingGuardEvidence"`
	RetryRecordSource       string                      `json:"retryRecordSource,omitempty"`
	RetryRecordStatusCode   int                         `json:"retryRecordStatusCode"`
	RetryRecordErrorDetail  string                      `json:"retryRecordErrorDetail,omitempty"`
	RetryUpstreamToolCalls  []string                    `json:"retryUpstreamToolCalls,omitempty"`
	RetryLatestObserved     *advancedProxyObservedItem  `json:"retryLatestObserved,omitempty"`
	RetryOps                []antiPoisonOperationRecord `json:"retryOps,omitempty"`
	Failure                 string                      `json:"failure,omitempty"`
}

func TestAdvancedProxyAntiPoisonReqFetchLiveVariants(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_REQFETCH_LIVE")) == "" {
		t.Skip("set BATCH_API_CHECK_REQFETCH_LIVE=1 to run real req-fetch upstream experiments")
	}

	rawFetchPath := resolveReqFetchFixturePath()
	rawFetch, err := os.ReadFile(rawFetchPath)
	if err != nil {
		t.Fatalf("read req-fetch.txt: %v", err)
	}
	baseURL, apiKey, requestBody, err := parseReqFetchRequest(rawFetch)
	if err != nil {
		t.Fatalf("parse req-fetch.txt: %v", err)
	}

	provider := AdvancedProxyProvider{
		ID:        "req-fetch-live",
		RowKey:    "req-fetch-live",
		Name:      "req-fetch-live",
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APIFormat: "openai_responses",
		Model:     strings.TrimSpace(toStringValue(requestBody["model"])),
		Enabled:   true,
	}
	if provider.Model == "" {
		provider.Model = "gpt-5.4"
	}

	attempts := clampInt(envInt("BATCH_API_CHECK_REQFETCH_ATTEMPTS", 3), 1, 20)
	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true

	outDir := filepath.Join("build", "bin", "guard-experiments", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}

	variants := []antiPoisonReqFetchVariant{
		{
			Name:      "default_current",
			Placement: "instructions_prepend",
		},
		{
			Name:       "compact_guard_name_replace",
			Placement:  "instructions_replace",
			PromptMode: "compact_guard_name",
		},
		{
			Name:       "compact_guard_name_system",
			Placement:  "input_system",
			PromptMode: "compact_guard_name",
		},
		{
			Name:       "compact_guard_name_user_tail",
			Placement:  "last_user_append",
			PromptMode: "compact_guard_name",
		},
		{
			Name:       "compact_guard_name_prepend",
			Placement:  "instructions_prepend",
			PromptMode: "compact_guard_name",
		},
		{
			Name:       "exact_guard_template_replace",
			Placement:  "instructions_replace",
			PromptMode: "exact_guard_template",
		},
		{
			Name:       "exact_guard_template_system",
			Placement:  "input_system",
			PromptMode: "exact_guard_template",
		},
	}
	variants = filterReqFetchVariants(variants, os.Getenv("BATCH_API_CHECK_REQFETCH_VARIANTS"))
	if len(variants) == 0 {
		t.Fatalf("no req-fetch variants selected")
	}

	summaries := make([]antiPoisonReqFetchExperimentSummary, 0, len(variants)*attempts)
	for _, variant := range variants {
		variantConfig := config
		if strings.TrimSpace(variant.StrategyPrompt) != "" {
			variantConfig.AntiPoison.StrategyPrompt = variant.StrategyPrompt
		}
		if strings.TrimSpace(variant.AlgorithmPrompt) != "" {
			variantConfig.AntiPoison.AlgorithmPrompt = variant.AlgorithmPrompt
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			summary := runAntiPoisonReqFetchLiveAttempt(t, provider, requestBody, variantConfig, variant, attempt, outDir)
			summaries = append(summaries, summary)
			t.Logf(
				"variant=%s attempt=%d status=%d blocked=%t valid=%t real=%d guard=%d reason=%s",
				summary.Variant,
				summary.Attempt,
				summary.StatusCode,
				summary.ValidationBlocked,
				summary.ValidationValid,
				summary.RealCount,
				summary.GuardCount,
				summary.ValidationReason,
			)
		}
	}

	reportPath := filepath.Join(outDir, "summary.json")
	reportRaw, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if err := os.WriteFile(reportPath, reportRaw, 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	t.Logf("req-fetch live experiment report: %s", reportPath)
}

func TestAdvancedProxyAntiPoisonReqFetchLiveProxyResponsesRetry(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_REQFETCH_LIVE")) == "" {
		t.Skip("set BATCH_API_CHECK_REQFETCH_LIVE=1 to run real req-fetch proxy experiments")
	}

	resetAdvancedProxyRuntimeForTest(t)
	resetAdvancedProxyRequestRecordsForTest(t)

	provider, requestBody := loadReqFetchLiveProviderAndBody(t)
	requestBody["stream"] = false

	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal req-fetch body: %v", err)
	}

	outDir := filepath.Join("build", "bin", "guard-experiments", time.Now().Format("20060102-150405")+"-proxy-responses")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}

	summary := antiPoisonReqFetchProxySummary{
		Variant:         "proxy_responses_retry",
		StartedAt:       time.Now().Format(time.RFC3339Nano),
		TargetURL:       firstNonEmpty(buildResponsesEndpointCandidates(provider.BaseURL)...),
		Model:           provider.Model,
		Stream:          false,
		RequestBodyPath: filepath.Join(outDir, "request.json"),
	}
	if err := os.WriteFile(summary.RequestBodyPath, rawBody, 0o644); err != nil {
		t.Fatalf("write request body snapshot: %v", err)
	}

	result := forwardOpenAIRequestViaProvider("codex", provider, "responses", rawBody, false, config)
	summary.StatusCode = result.StatusCode
	summary.AntiPoisonBlocked = result.AntiPoisonBlocked
	if len(result.Body) > 0 {
		summary.ResponsePath = filepath.Join(outDir, "response.json")
		if err := os.WriteFile(summary.ResponsePath, result.Body, 0o644); err != nil {
			t.Fatalf("write response snapshot: %v", err)
		}
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		summary.Failure = firstNonEmpty(result.Message, string(result.Body))
	}

	records := advancedProxyRequestRecords.list(10)
	summary.RecordCount = len(records)
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.ErrorDetail), "missing_guard_toolcall") {
			summary.HasInitialFailure = true
		}
		if advancedProxyRecordHasMissingGuardEvidence(record) {
			summary.HasMissingGuardEvidence = true
		}
		if record.Source == "anti_poison_exact_retry" {
			summary.HasRetryRecord = true
			summary.RetryRecordSource = record.Source
			summary.RetryRecordStatusCode = record.StatusCode
			summary.RetryRecordErrorDetail = record.ErrorDetail
			summary.RetryUpstreamToolCalls = append([]string(nil), record.UpstreamToolCalls...)
			summary.RetryLatestObserved = normalizeAdvancedProxyObservedItem(record.UpstreamLatestObserved)
			summary.RetryOps = append([]antiPoisonOperationRecord(nil), record.AntiPoisonOps...)
			break
		}
	}

	writeReqFetchProxySummary(t, outDir, "proxy_responses_retry", summary)

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected proxy retry to succeed, got status=%d blocked=%t body=%s", result.StatusCode, result.AntiPoisonBlocked, string(result.Body))
	}
	if !summary.HasRetryRecord {
		t.Fatalf("expected retry record in request records, got %#v", records)
	}
	if summary.RetryRecordSource != "anti_poison_exact_retry" {
		t.Fatalf("expected retry record source anti_poison_exact_retry, got %#v", summary)
	}
	if summary.RetryRecordStatusCode != http.StatusOK {
		t.Fatalf("expected retry record status 200, got %#v", summary)
	}
}

func TestAdvancedProxyAntiPoisonReqFetchLiveProxyResponsesStreamRetry(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_REQFETCH_LIVE")) == "" {
		t.Skip("set BATCH_API_CHECK_REQFETCH_LIVE=1 to run real req-fetch proxy stream experiments")
	}

	resetAdvancedProxyRuntimeForTest(t)
	resetAdvancedProxyRequestRecordsForTest(t)

	provider, requestBody := loadReqFetchLiveProviderAndBody(t)
	requestBody["stream"] = true

	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal req-fetch body: %v", err)
	}

	outDir := filepath.Join("build", "bin", "guard-experiments", time.Now().Format("20060102-150405")+"-proxy-stream")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}

	summary := antiPoisonReqFetchProxySummary{
		Variant:         "proxy_stream_retry",
		StartedAt:       time.Now().Format(time.RFC3339Nano),
		TargetURL:       firstNonEmpty(buildResponsesEndpointCandidates(provider.BaseURL)...),
		Model:           provider.Model,
		Stream:          true,
		RequestBodyPath: filepath.Join(outDir, "request.json"),
	}
	if err := os.WriteFile(summary.RequestBodyPath, rawBody, 0o644); err != nil {
		t.Fatalf("write request body snapshot: %v", err)
	}

	result := forwardOpenAIRequestViaProvider("codex", provider, "responses", rawBody, true, config)
	if result.StreamBody == nil {
		summary.StatusCode = result.StatusCode
		summary.AntiPoisonBlocked = result.AntiPoisonBlocked
		summary.Failure = firstNonEmpty(result.Message, string(result.Body), "missing stream body")
		writeReqFetchProxySummary(t, outDir, "proxy_stream_retry", summary)
		t.Fatalf("expected stream body, got %#v", result)
	}

	recorder := httptest.NewRecorder()
	if err := proxyOpenAIStreamToClientWithMetrics(recorder, result.StreamBody, result.RecordCtx); err != nil {
		summary.Failure = err.Error()
		writeReqFetchProxySummary(t, outDir, "proxy_stream_retry", summary)
		t.Fatalf("proxy stream failed: %v", err)
	}

	responseBody := recorder.Body.Bytes()
	summary.StatusCode = recorder.Code
	summary.ResponsePath = filepath.Join(outDir, "response.sse")
	if err := os.WriteFile(summary.ResponsePath, responseBody, 0o644); err != nil {
		t.Fatalf("write stream response snapshot: %v", err)
	}
	if bytesContains(responseBody, []byte("anti_poison_validation_failed")) {
		summary.AntiPoisonBlocked = true
		summary.Failure = string(responseBody)
	}

	records := advancedProxyRequestRecords.list(10)
	summary.RecordCount = len(records)
	for _, record := range records {
		if strings.Contains(strings.ToLower(record.ErrorDetail), "missing_guard_toolcall") {
			summary.HasInitialFailure = true
		}
		if advancedProxyRecordHasMissingGuardEvidence(record) {
			summary.HasMissingGuardEvidence = true
		}
		if record.Source == "anti_poison_exact_retry" {
			summary.HasRetryRecord = true
			summary.RetryRecordSource = record.Source
			summary.RetryRecordStatusCode = record.StatusCode
			summary.RetryRecordErrorDetail = record.ErrorDetail
			summary.RetryUpstreamToolCalls = append([]string(nil), record.UpstreamToolCalls...)
			summary.RetryLatestObserved = normalizeAdvancedProxyObservedItem(record.UpstreamLatestObserved)
			summary.RetryOps = append([]antiPoisonOperationRecord(nil), record.AntiPoisonOps...)
			break
		}
	}

	writeReqFetchProxySummary(t, outDir, "proxy_stream_retry", summary)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected stream proxy to succeed, got status=%d body=%s", recorder.Code, string(responseBody))
	}
	if strings.Contains(string(responseBody), antiPoisonGuardJSONOpenTag) {
		t.Fatalf("expected delivered stream to strip guard json, got %s", string(responseBody))
	}
	if !summary.HasRetryRecord {
		t.Fatalf("expected stream retry record in request records, got %#v", records)
	}
}

func resolveReqFetchFixturePath() string {
	if envPath := strings.TrimSpace(os.Getenv("BATCH_API_CHECK_REQFETCH_PATH")); envPath != "" {
		return envPath
	}
	candidates := []string{
		filepath.Join("build", "bin", "req-fetch.txt"),
		filepath.Join("desktop", "build", "bin", "req-fetch.txt"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}

func runAntiPoisonReqFetchLiveAttempt(t *testing.T, provider AdvancedProxyProvider, baseRequest map[string]any, config AdvancedProxyConfig, variant antiPoisonReqFetchVariant, attempt int, outDir string) antiPoisonReqFetchExperimentSummary {
	t.Helper()

	requestCopy := deepCopyJSONMap(baseRequest)
	requestCopy["instructions"] = stripExistingAntiPoisonPrompt(strings.TrimSpace(toStringValue(requestCopy["instructions"])))
	appliedBody, ctx, appliedPrompt := applyReqFetchVariantPrompt(t, requestCopy, config.AntiPoison, variant)
	protectedBody, protectCtx, err := applyAntiPoisonStringProtectionToJSONBody(appliedBody, config.AntiPoison, "responses", provider.Name, "openai")
	if err != nil {
		t.Fatalf("apply string protection: %v", err)
	}

	startedAt := time.Now()
	targetURL := firstNonEmpty(buildResponsesEndpointCandidates(provider.BaseURL)...)
	statusCode, _, _, streamBody, elapsed, err := performRawUpstreamRequest(
		http.MethodPost,
		targetURL,
		buildOpenAIProviderHeaders(provider),
		protectedBody,
		computeAdvancedProxyTimeoutSeconds(true, false, config.Failover),
		true,
	)
	summary := antiPoisonReqFetchExperimentSummary{
		Variant:       variant.Name,
		Attempt:       attempt,
		StartedAt:     startedAt.Format(time.RFC3339Nano),
		DurationMs:    elapsed.Milliseconds(),
		StatusCode:    statusCode,
		TargetURL:     targetURL,
		Model:         provider.Model,
		Alias:         ctx.Alias,
		Nonce:         ctx.Seed,
		PromptApplied: ctx.Enabled,
	}

	baseName := fmt.Sprintf("%02d_%s", attempt, sanitizeReqFetchVariantFileName(variant.Name))
	requestBodyPath := filepath.Join(outDir, baseName+"_request.json")
	summary.RequestBodyPath = requestBodyPath
	if err := os.WriteFile(requestBodyPath, protectedBody, 0o644); err != nil {
		t.Fatalf("write request body snapshot: %v", err)
	}
	instructionsPath := filepath.Join(outDir, baseName+"_instructions.txt")
	summary.InstructionsPath = instructionsPath
	if err := os.WriteFile(instructionsPath, []byte(appliedPrompt), 0o644); err != nil {
		t.Fatalf("write instructions snapshot: %v", err)
	}

	if err != nil {
		summary.Failure = err.Error()
		writeReqFetchAttemptSummary(t, outDir, baseName, summary)
		return summary
	}
	if streamBody == nil {
		summary.Failure = "upstream returned nil stream body"
		writeReqFetchAttemptSummary(t, outDir, baseName, summary)
		return summary
	}
	defer streamBody.Close()

	streamRaw, readErr := io.ReadAll(streamBody)
	if readErr != nil {
		summary.Failure = readErr.Error()
		writeReqFetchAttemptSummary(t, outDir, baseName, summary)
		return summary
	}
	if protectCtx.Enabled {
		streamRaw = restoreAntiPoisonStringProtectionInSSEBody(streamRaw, &protectCtx, "responses", provider.Name, "openai")
	}

	rawPath := filepath.Join(outDir, baseName+"_raw.sse")
	if err := os.WriteFile(rawPath, streamRaw, 0o644); err != nil {
		t.Fatalf("write raw sse: %v", err)
	}
	summary.RawPath = rawPath
	summary.RawGuardBlockCount = strings.Count(string(streamRaw), antiPoisonGuardJSONOpenTag)

	events, parseErr := parseAdvancedProxySSEEvents(streamRaw)
	if parseErr == nil {
		summary.StreamEventCount = len(events)
	}
	summary.PromptEchoGuardRefs = countPromptEchoGuardRefs(events)

	feedbackTools, feedbackArgs, assistantPreview, latest := summarizeAdvancedProxyRawStreamFeedbackContext(streamRaw, "responses")
	summary.UpstreamToolCalls = feedbackTools
	summary.UpstreamToolArgs = normalizeAdvancedProxyPreviewList(feedbackArgs, 12, 400)
	summary.AssistantPreview = previewAdvancedProxyText(assistantPreview, 1200)
	summary.LatestObserved = normalizeAdvancedProxyObservedItem(latest)

	sanitized, validation, sanitizeErr := sanitizeAntiPoisonOpenAIStreamBody(streamRaw, "responses", "responses", ctx)
	summary.ValidationApplied = validation.Applied
	summary.ValidationValid = validation.Valid
	summary.ValidationBlocked = validation.Blocked
	summary.ValidationReason = validation.Reason
	summary.RealCount = validation.RealCount
	summary.GuardCount = validation.GuardCount
	summary.RemovedGuards = validation.RemovedGuards
	if sanitizeErr != nil && summary.Failure == "" {
		summary.Failure = sanitizeErr.Error()
	}

	sanitizedPath := filepath.Join(outDir, baseName+"_sanitized.sse")
	if len(sanitized) > 0 {
		if err := os.WriteFile(sanitizedPath, sanitized, 0o644); err != nil {
			t.Fatalf("write sanitized sse: %v", err)
		}
		summary.SanitizedPath = sanitizedPath
	}

	writeReqFetchAttemptSummary(t, outDir, baseName, summary)
	return summary
}

func applyReqFetchVariantPrompt(t *testing.T, requestBody map[string]any, config AntiPoisonConfig, variant antiPoisonReqFetchVariant) ([]byte, antiPoisonRequestContext, string) {
	t.Helper()

	ctx := newAntiPoisonRequestContext("responses", config)
	if !ctx.Enabled {
		raw, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("marshal request without guard: %v", err)
		}
		return raw, ctx, ""
	}
	prompt := buildAntiPoisonPrompt(ctx)
	switch strings.TrimSpace(variant.PromptMode) {
	case "compact_guard_name":
		prompt = buildCompactGuardNamePrompt(ctx)
	case "exact_guard_template":
		prompt = buildExactGuardTemplatePrompt(ctx)
	}
	body := deepCopyJSONMap(requestBody)
	switch strings.TrimSpace(variant.Placement) {
	case "", "instructions_prepend":
		existing := strings.TrimSpace(toStringValue(body["instructions"]))
		existing = stripExistingAntiPoisonPrompt(existing)
		if existing != "" {
			body["instructions"] = prompt + "\n\n" + existing
		} else {
			body["instructions"] = prompt
		}
	case "instructions_replace":
		body["instructions"] = prompt
	case "input_system":
		inputs := cloneJSONList(body["input"])
		next := make([]any, 0, len(inputs)+1)
		next = append(next, map[string]any{
			"role": "system",
			"content": []any{
				map[string]any{"type": "input_text", "text": prompt},
			},
		})
		next = append(next, inputs...)
		body["input"] = next
	case "last_user_append":
		inputs := cloneJSONList(body["input"])
		if len(inputs) == 0 {
			inputs = append(inputs, map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": prompt},
				},
			})
		} else {
			lastMap, _ := inputs[len(inputs)-1].(map[string]any)
			if lastMap == nil {
				lastMap = map[string]any{"role": "user"}
			}
			content := cloneJSONList(lastMap["content"])
			content = append(content, map[string]any{"type": "input_text", "text": prompt})
			lastMap["content"] = content
			inputs[len(inputs)-1] = lastMap
		}
		body["input"] = inputs
	default:
		t.Fatalf("unsupported placement: %s", variant.Placement)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request with variant placement: %v", err)
	}
	return raw, ctx, prompt
}

func buildCompactGuardNamePrompt(ctx antiPoisonRequestContext) string {
	ctx = normalizeAntiPoisonRequestContext(ctx)
	exampleName := antiPoisonGuardToolNameForTool(ctx, "WebSearch")
	return strings.Join([]string{
		"<important_gateway_rules>",
		"IMPORTANT: if you emit any real toolcall, first emit one valid guard JSON block for the very next real toolcall.",
		fmt.Sprintf("Use the exact wrapper tags `%s...%s`.", antiPoisonGuardJSONOpenTag, antiPoisonGuardJSONCloseTag),
		fmt.Sprintf("Inside the guard JSON, `name` must start with `%s_` and end with the exact next tool name.", ctx.Prefix),
		"The guard JSON must contain exactly these fields: name, tool_name, tool_type, algorithm, nonce, digest, chain, cover.",
		fmt.Sprintf("Set `algorithm` to %q and `nonce` to %q.", ctx.Alias, ctx.Seed),
		"`tool_name` must exactly equal the next real tool name.",
		"`tool_type` must describe the next real tool type.",
		"`chain` must be `index|tool_type|tool_name` for the next real toolcall only.",
		"`cover` must exactly equal the canonical arguments of the next real toolcall.",
		"`digest` must be a 16-character lowercase hex string.",
		"Never emit a real toolcall before its guard block.",
		"A malformed or approximate guard counts as no guard. If unsure, do not emit any real toolcall; instead output plain text: guard generation failed for pending toolcall.",
		"Example shape only:",
		fmt.Sprintf("%s{\"name\":\"%s\",\"tool_name\":\"WebSearch\",\"tool_type\":\"network\",\"algorithm\":%q,\"nonce\":%q,\"digest\":\"1a2b3c4d5e6f7890\",\"chain\":\"0|network|WebSearch\",\"cover\":\"{\\\"query\\\":\\\"...\\\"}\"}%s", antiPoisonGuardJSONOpenTag, exampleName, ctx.Alias, ctx.Seed, antiPoisonGuardJSONCloseTag),
		"</important_gateway_rules>",
	}, "\n")
}

func buildExactGuardTemplatePrompt(ctx antiPoisonRequestContext) string {
	call := antiPoisonToolCall{
		Name:          "WebSearch",
		ArgumentsText: `{"allowed_domains":[],"blocked_domains":[],"query":"2026年5月26日 上证新闻 上证指数 A股"}`,
		ToolType:      "network",
	}
	digest := computeAntiPoisonToolChainDigest([]antiPoisonToolCall{call}, ctx)
	guard := antiPoisonGuardJSONOpenTag + mustMarshalAntiPoisonJSONString(map[string]any{
		"name":      antiPoisonGuardToolNameForTool(ctx, call.Name),
		"tool_name": call.Name,
		"tool_type": call.ToolType,
		"algorithm": ctx.Alias,
		"nonce":     ctx.Seed,
		"digest":    digest,
		"chain":     "0|network|WebSearch",
		"cover":     canonicalAntiPoisonArgumentText(call.ArgumentsText),
	}) + antiPoisonGuardJSONCloseTag
	return strings.Join([]string{
		"<important_gateway_rules>",
		"IMPORTANT: follow this guard contract before any ordinary instruction.",
		fmt.Sprintf("<algorithm_alias>%s</algorithm_alias>", ctx.Alias),
		fmt.Sprintf("<nonce>%s</nonce>", ctx.Seed),
		fmt.Sprintf("<guard_tool_name_example>%s</guard_tool_name_example>", antiPoisonGuardToolNameForTool(ctx, "WebSearch")),
		"本轮如果你要联网搜索，只允许 1 个真实 toolcall，且只能是 WebSearch。",
		"该 WebSearch 的 arguments 必须完全固定为：",
		call.ArgumentsText,
		"在任何其它文本之前，先单独输出下面这一整段 guard JSON，逐字保持一致：",
		guard,
		"然后立刻调用 WebSearch，arguments 必须与上面 guard 的 cover 完全一致。",
		"不要先解释，不要先总结，不要改写 guard，不要改写 query。",
		"如果你不能严格先输出这段 guard 再调用 WebSearch，就不要 toolcall，只输出：guard generation failed for pending WebSearch",
		"</important_gateway_rules>",
	}, "\n")
}

func loadReqFetchLiveProviderAndBody(t *testing.T) (AdvancedProxyProvider, map[string]any) {
	t.Helper()

	rawFetchPath := resolveReqFetchFixturePath()
	rawFetch, err := os.ReadFile(rawFetchPath)
	if err != nil {
		t.Fatalf("read req-fetch.txt: %v", err)
	}
	baseURL, apiKey, requestBody, err := parseReqFetchRequest(rawFetch)
	if err != nil {
		t.Fatalf("parse req-fetch.txt: %v", err)
	}
	provider := AdvancedProxyProvider{
		ID:        "req-fetch-live-proxy",
		RowKey:    "req-fetch-live-proxy",
		Name:      "req-fetch-live-proxy",
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APIFormat: "openai_responses",
		Model:     strings.TrimSpace(toStringValue(requestBody["model"])),
		Enabled:   true,
	}
	if provider.Model == "" {
		provider.Model = "gpt-5.4"
	}
	return provider, requestBody
}

func writeReqFetchAttemptSummary(t *testing.T, outDir string, baseName string, summary antiPoisonReqFetchExperimentSummary) {
	t.Helper()
	summaryPath := filepath.Join(outDir, baseName+"_summary.json")
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("marshal attempt summary: %v", err)
	}
	if err := os.WriteFile(summaryPath, raw, 0o644); err != nil {
		t.Fatalf("write attempt summary: %v", err)
	}
}

func writeReqFetchProxySummary(t *testing.T, outDir string, baseName string, summary antiPoisonReqFetchProxySummary) {
	t.Helper()
	summaryPath := filepath.Join(outDir, baseName+"_summary.json")
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("marshal proxy summary: %v", err)
	}
	if err := os.WriteFile(summaryPath, raw, 0o644); err != nil {
		t.Fatalf("write proxy summary: %v", err)
	}
}

func bytesContains(data []byte, needle []byte) bool {
	return strings.Contains(string(data), string(needle))
}

func advancedProxyRecordHasMissingGuardEvidence(record AdvancedProxyRequestRecord) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(record.ErrorDetail)), "missing_guard_toolcall") {
		return true
	}
	for _, op := range record.AntiPoisonOps {
		if strings.Contains(strings.ToLower(strings.TrimSpace(op.Reason)), "missing_guard_toolcall") {
			return true
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(op.Before)), "missing_guard_toolcall") {
			return true
		}
	}
	return false
}

func parseReqFetchRequest(raw []byte) (string, string, map[string]any, error) {
	text := string(raw)
	urlMatch := regexp.MustCompile(`fetch\("([^"]+)"`).FindStringSubmatch(text)
	if len(urlMatch) != 2 {
		return "", "", nil, fmt.Errorf("fetch url not found")
	}
	keyMatch := regexp.MustCompile(`Authorization":\s*"Bearer\s+([^"]+)"`).FindStringSubmatch(text)
	if len(keyMatch) != 2 {
		return "", "", nil, fmt.Errorf("authorization bearer token not found")
	}
	bodyIndex := strings.Index(text, `body: JSON.stringify(`)
	if bodyIndex < 0 {
		return "", "", nil, fmt.Errorf("JSON.stringify body not found")
	}
	start := strings.Index(text[bodyIndex:], "{")
	if start < 0 {
		return "", "", nil, fmt.Errorf("request body object start not found")
	}
	start += bodyIndex
	bodyText, err := extractBalancedBraces(text[start:])
	if err != nil {
		return "", "", nil, err
	}
	body := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(bodyText))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return "", "", nil, fmt.Errorf("decode request body: %w", err)
	}
	return urlMatch[1], keyMatch[1], body, nil
}

func extractBalancedBraces(text string) (string, error) {
	depth := 0
	inString := false
	escaped := false
	for index, r := range text {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[:index+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated JSON object")
}

func sanitizeReqFetchVariantFileName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "variant"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func filterReqFetchVariants(variants []antiPoisonReqFetchVariant, raw string) []antiPoisonReqFetchVariant {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return variants
	}
	allowed := map[string]struct{}{}
	for _, item := range strings.Split(raw, ",") {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		allowed[name] = struct{}{}
	}
	if len(allowed) == 0 {
		return variants
	}
	next := make([]antiPoisonReqFetchVariant, 0, len(variants))
	for _, variant := range variants {
		if _, ok := allowed[variant.Name]; ok {
			next = append(next, variant)
		}
	}
	return next
}

func countPromptEchoGuardRefs(events []advancedProxySSEEvent) int {
	count := 0
	for _, event := range events {
		data, ok := advancedProxySSEJSONPayload(event)
		if !ok {
			continue
		}
		switch firstNonEmpty(strings.TrimSpace(event.Event), strings.TrimSpace(toStringValue(data["type"]))) {
		case "response.created", "response.completed":
			responseMap, _ := data["response"].(map[string]any)
			if responseMap == nil {
				continue
			}
			instructions := toStringValue(responseMap["instructions"])
			if instructions == "" {
				continue
			}
			count += strings.Count(instructions, antiPoisonGuardJSONTagName)
			count += strings.Count(instructions, "<guard_tool_name>")
		}
	}
	return count
}
