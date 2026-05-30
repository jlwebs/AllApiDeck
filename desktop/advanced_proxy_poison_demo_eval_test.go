package main

import (
	"context"
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
	"strconv"
	"strings"
	"testing"
	"time"
)

const poisonDemoEvalDefaultBaseURL = "http://127.0.0.1:9999/v1"

type poisonDemoEvalScenario struct {
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	RouteKind      string `json:"routeKind"`
	ProviderFormat string `json:"providerFormat"`
	Attack         string `json:"attack"`
	Stream         bool   `json:"stream"`
	MultiTurn      bool   `json:"multiTurn"`
	ExpectBlocked  bool   `json:"expectBlocked"`
}

type poisonDemoEvalRow struct {
	Iteration         int      `json:"iteration"`
	Scenario          string   `json:"scenario"`
	Protocol          string   `json:"protocol"`
	RouteKind         string   `json:"routeKind"`
	Attack            string   `json:"attack"`
	Stream            bool     `json:"stream"`
	MultiTurn         bool     `json:"multiTurn"`
	Expected          string   `json:"expected"`
	Actual            string   `json:"actual"`
	Passed            bool     `json:"passed"`
	OverBlocked       bool     `json:"overBlocked"`
	UnderBlocked      bool     `json:"underBlocked"`
	SecretLeak        bool     `json:"secretLeak"`
	StatusCode        int      `json:"statusCode"`
	Reason            string   `json:"reason"`
	MessagePreview    string   `json:"messagePreview"`
	RecordCount       int      `json:"recordCount"`
	RecordStatusCodes []int    `json:"recordStatusCodes"`
	OperationStages   []string `json:"operationStages"`
	OperationRules    []string `json:"operationRules"`
	RequestHash       string   `json:"requestHash"`
	ResponseHash      string   `json:"responseHash"`
	StartedAt         string   `json:"startedAt"`
	DurationMs        int64    `json:"durationMs"`
}

type poisonDemoEvalReport struct {
	GeneratedAt     string                   `json:"generatedAt"`
	BaseURL         string                   `json:"baseUrl"`
	Duration        string                   `json:"duration"`
	Interval        string                   `json:"interval"`
	Total           int                      `json:"total"`
	Passed          int                      `json:"passed"`
	Failed          int                      `json:"failed"`
	ExpectedBlocked int                      `json:"expectedBlocked"`
	ExpectedAllowed int                      `json:"expectedAllowed"`
	ActualBlocked   int                      `json:"actualBlocked"`
	ActualAllowed   int                      `json:"actualAllowed"`
	OverBlocked     int                      `json:"overBlocked"`
	UnderBlocked    int                      `json:"underBlocked"`
	SecretLeaks     int                      `json:"secretLeaks"`
	Scenarios       []poisonDemoEvalScenario `json:"scenarios"`
	Rows            []poisonDemoEvalRow      `json:"rows"`
}

func TestAdvancedProxyPoisonDemoEvaluation(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BATCH_API_CHECK_POISON_DEMO_EVAL")) != "1" {
		t.Skip("set BATCH_API_CHECK_POISON_DEMO_EVAL=1 to run the local poison demo evaluation")
	}
	resetAdvancedProxyRuntimeForTest(t)
	resetAdvancedProxyRequestRecordsForTest(t)
	resetAdvancedProxyOpenAIProtocolPreferencesForTests()
	resetAdvancedProxyClaudeProtocolPreferencesForTests()

	baseURL := strings.TrimRight(firstNonEmpty(os.Getenv("BATCH_API_CHECK_POISON_DEMO_EVAL_BASE_URL"), poisonDemoEvalDefaultBaseURL), "/")
	controlURL := strings.TrimSuffix(baseURL, "/v1")
	stopServer := ensurePoisonDemoServerForEval(t, controlURL)
	defer stopServer()

	duration := loadPoisonDemoEvalDuration(t, "BATCH_API_CHECK_POISON_DEMO_EVAL_DURATION", 30*time.Minute)
	interval := loadPoisonDemoEvalDuration(t, "BATCH_API_CHECK_POISON_DEMO_EVAL_INTERVAL", 500*time.Millisecond)
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	deadline := time.Now().Add(duration)
	maxRows := loadPoisonDemoEvalInt(t, "BATCH_API_CHECK_POISON_DEMO_EVAL_MAX_ROWS", 0)
	scenarios := buildPoisonDemoEvalScenarios()
	report := poisonDemoEvalReport{
		GeneratedAt: time.Now().Format(time.RFC3339Nano),
		BaseURL:     baseURL,
		Duration:    duration.String(),
		Interval:    interval.String(),
		Scenarios:   scenarios,
		Rows:        []poisonDemoEvalRow{},
	}

	iteration := 0
	for {
		for _, scenario := range scenarios {
			if maxRows > 0 && iteration >= maxRows {
				writePoisonDemoEvalArtifacts(t, report)
				assertPoisonDemoEvalReport(t, report)
				return
			}
			if iteration > 0 && time.Now().After(deadline) {
				writePoisonDemoEvalArtifacts(t, report)
				assertPoisonDemoEvalReport(t, report)
				return
			}
			iteration++
			row := runPoisonDemoEvalScenario(t, baseURL, controlURL, scenario, iteration)
			report.Rows = append(report.Rows, row)
			accumulatePoisonDemoEvalReport(&report, row, scenario)
			if interval > 0 {
				time.Sleep(interval)
			}
		}
		if duration <= 0 || time.Now().After(deadline) {
			break
		}
	}
	writePoisonDemoEvalArtifacts(t, report)
	assertPoisonDemoEvalReport(t, report)
}

func buildPoisonDemoEvalScenarios() []poisonDemoEvalScenario {
	attacks := []struct {
		Name          string
		ExpectBlocked bool
	}{
		{"missing_guard_toolcall", true},
		{"guard_coverage_mismatch", true},
		{"replay_old_nonce", true},
		{"tampered_arguments", true},
		{"extra_toolcall", true},
		{"guard_only", false},
		{"clean_text", false},
	}
	protocols := []struct {
		Protocol       string
		RouteKind      string
		ProviderFormat string
	}{
		{"openai_responses", "responses", "openai_responses"},
		{"openai_chat", "chat", "openai_chat"},
		{"claude_messages", "claude_messages", "anthropic"},
	}
	scenarios := make([]poisonDemoEvalScenario, 0, len(attacks)*len(protocols)*3)
	for _, protocol := range protocols {
		for _, attack := range attacks {
			for _, stream := range []bool{false, true} {
				scenarios = append(scenarios, poisonDemoEvalScenario{
					Name:           fmt.Sprintf("%s/%s/stream=%t", protocol.Protocol, attack.Name, stream),
					Protocol:       protocol.Protocol,
					RouteKind:      protocol.RouteKind,
					ProviderFormat: protocol.ProviderFormat,
					Attack:         attack.Name,
					Stream:         stream,
					ExpectBlocked:  attack.ExpectBlocked,
				})
			}
			if attack.Name == "missing_guard_toolcall" || attack.Name == "guard_only" || attack.Name == "clean_text" {
				scenarios = append(scenarios, poisonDemoEvalScenario{
					Name:           fmt.Sprintf("%s/%s/multi-turn", protocol.Protocol, attack.Name),
					Protocol:       protocol.Protocol,
					RouteKind:      protocol.RouteKind,
					ProviderFormat: protocol.ProviderFormat,
					Attack:         attack.Name,
					Stream:         false,
					MultiTurn:      true,
					ExpectBlocked:  attack.ExpectBlocked,
				})
			}
		}
	}
	return scenarios
}

func runPoisonDemoEvalScenario(t *testing.T, baseURL string, controlURL string, scenario poisonDemoEvalScenario, iteration int) poisonDemoEvalRow {
	t.Helper()
	startedAt := time.Now()
	resetAdvancedProxyRequestRecordsForTest(t)
	configurePoisonDemoEvalServer(t, controlURL, scenario.Protocol, scenario.Attack, streamModeForPoisonDemoEval(scenario.Stream))

	config := defaultAdvancedProxyConfig()
	config.AntiPoison.Enabled = true
	config.AntiPoison.StrictMode = true
	config.AntiPoison.FailureMode = "block"
	config.AntiPoison.StringProtection.Enabled = true
	config.Failover.Enabled = false
	config.Failover.AutoFailoverEnabled = false
	config.Failover.StreamingFirstByteTimeout = 5
	config.Failover.StreamingIdleTimeout = 5
	config.Failover.NonStreamingTimeout = 5

	provider := AdvancedProxyProvider{
		ID:        "poison-demo-" + strings.ReplaceAll(scenario.Protocol, "_", "-"),
		RowKey:    "poison-demo-row-" + strings.ReplaceAll(scenario.Protocol, "_", "-"),
		Name:      "Poison Demo " + scenario.Protocol,
		BaseURL:   baseURL,
		APIKey:    "poison-local",
		Model:     poisonDemoEvalModelForScenario(scenario),
		APIFormat: scenario.ProviderFormat,
		Enabled:   true,
	}

	prompt := poisonDemoEvalPrompt(iteration)
	var statusCode int
	var actualBlocked bool
	var reason string
	var message string
	var requestBody []byte
	var responseBody []byte
	var err error

	if scenario.Protocol == "claude_messages" {
		requestBody = mustJSON(t, buildPoisonDemoEvalClaudeBody(provider.Model, prompt, scenario.MultiTurn, scenario.Stream))
		var result providerAttemptResult
		result = forwardClaudeRequestViaProvider(provider, mustJSONMap(t, requestBody), nil, scenario.Stream, config)
		statusCode = result.StatusCode
		actualBlocked = result.AntiPoisonBlocked || statusCode == http.StatusBadGateway && strings.Contains(result.Message, "anti-poison")
		message = result.Message
		if scenario.Stream && result.StreamBody != nil {
			responseBody, actualBlocked, reason, err = readPoisonDemoEvalStream(result.StreamBody, result.RecordCtx, true)
			if err != nil {
				message = err.Error()
			}
			statusCode = poisonDemoEvalStreamStatus(actualBlocked, statusCode)
		} else if result.Response != nil {
			responseBody = mustJSON(t, result.Response)
		} else {
			responseBody = []byte(result.Message)
		}
	} else {
		requestBody = mustJSON(t, buildPoisonDemoEvalOpenAIBody(scenario.RouteKind, provider.Model, prompt, scenario.MultiTurn, scenario.Stream))
		result := forwardOpenAIRequestViaProvider("codex", provider, scenario.RouteKind, requestBody, scenario.Stream, config)
		statusCode = result.StatusCode
		actualBlocked = result.AntiPoisonBlocked || result.ErrorCode == "anti_poison_validation_failed"
		reason = firstNonEmpty(result.Message, result.ErrorCode)
		message = result.Message
		if scenario.Stream && result.StreamBody != nil {
			responseBody, actualBlocked, reason, err = readPoisonDemoEvalStream(result.StreamBody, result.RecordCtx, false)
			if err != nil {
				message = err.Error()
			}
			statusCode = poisonDemoEvalStreamStatus(actualBlocked, statusCode)
		} else {
			responseBody = result.Body
		}
	}

	records := advancedProxyRequestRecords.list(20)
	secretLeak := poisonDemoEvalContainsSyntheticSecret(responseBody) || poisonDemoEvalRecordsContainSyntheticSecret(records)
	operationStages, operationRules := summarizePoisonDemoEvalOps(records)
	if reason == "" {
		reason = firstPoisonDemoEvalBlockedReason(records)
	}
	passed := actualBlocked == scenario.ExpectBlocked && !secretLeak && err == nil
	row := poisonDemoEvalRow{
		Iteration:         iteration,
		Scenario:          scenario.Name,
		Protocol:          scenario.Protocol,
		RouteKind:         scenario.RouteKind,
		Attack:            scenario.Attack,
		Stream:            scenario.Stream,
		MultiTurn:         scenario.MultiTurn,
		Expected:          poisonDemoEvalExpectationLabel(scenario.ExpectBlocked),
		Actual:            poisonDemoEvalExpectationLabel(actualBlocked),
		Passed:            passed,
		OverBlocked:       actualBlocked && !scenario.ExpectBlocked,
		UnderBlocked:      !actualBlocked && scenario.ExpectBlocked,
		SecretLeak:        secretLeak,
		StatusCode:        statusCode,
		Reason:            previewAdvancedProxyText(reason, 180),
		MessagePreview:    previewAdvancedProxyText(message, 220),
		RecordCount:       len(records),
		RecordStatusCodes: poisonDemoEvalRecordStatusCodes(records),
		OperationStages:   operationStages,
		OperationRules:    operationRules,
		RequestHash:       poisonDemoEvalHash(requestBody),
		ResponseHash:      poisonDemoEvalHash(responseBody),
		StartedAt:         startedAt.Format(time.RFC3339Nano),
		DurationMs:        time.Since(startedAt).Milliseconds(),
	}
	if !row.Passed {
		t.Logf("poison demo eval mismatch scenario=%s expected=%s actual=%s status=%d reason=%s", row.Scenario, row.Expected, row.Actual, row.StatusCode, row.Reason)
	}
	return row
}

func readPoisonDemoEvalStream(streamBody io.ReadCloser, recordCtx *advancedProxyStreamRequestRecordContext, anthropic bool) ([]byte, bool, string, error) {
	defer streamBody.Close()
	type streamReadResult struct {
		raw         []byte
		guardResult antiPoisonValidationResult
		err         error
	}
	resultCh := make(chan streamReadResult, 1)
	go func() {
		var result streamReadResult
		if anthropic {
			result.raw, result.guardResult, result.err = readAndPrepareAntiPoisonAnthropicStream(streamBody, recordCtx)
		} else {
			result.raw, result.guardResult, result.err = readAndPrepareAntiPoisonOpenAIStream(streamBody, recordCtx)
		}
		resultCh <- result
	}()
	var result streamReadResult
	select {
	case result = <-resultCh:
	case <-time.After(8 * time.Second):
		_ = streamBody.Close()
		select {
		case result = <-resultCh:
		case <-time.After(2 * time.Second):
			return nil, false, "stream_read_timeout", fmt.Errorf("poison demo stream read timed out")
		}
	}
	if recordCtx != nil {
		observation := advancedProxyStreamObservation{StartedAt: recordCtx.StartedAt}
		now := time.Now()
		observation.markFirstOutput(now)
		observation.markCompleted(now)
		statusCode := http.StatusOK
		if result.guardResult.Blocked {
			statusCode = http.StatusBadGateway
		}
		recordAdvancedProxyStreamObservation(recordCtx, observation, statusCode, result.guardResult.Reason)
	}
	return result.raw, result.guardResult.Blocked, result.guardResult.Reason, result.err
}

func buildPoisonDemoEvalOpenAIBody(routeKind string, model string, prompt string, multiTurn bool, stream bool) map[string]any {
	if routeKind == "chat" {
		messages := []any{}
		if multiTurn {
			messages = append(messages,
				map[string]any{"role": "user", "content": "上一轮请解释为什么不能读取 .env 和 config.json。"},
				map[string]any{"role": "assistant", "content": "我会避免读取敏感文件，也不会暴露密钥。"},
			)
		}
		messages = append(messages, map[string]any{"role": "user", "content": prompt})
		return map[string]any{
			"model":       model,
			"messages":    messages,
			"stream":      stream,
			"temperature": 0,
			"max_tokens":  96,
		}
	}
	input := []any{}
	if multiTurn {
		input = append(input,
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "上一轮请解释 request out 的密钥保护。"}}},
			map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "密钥应该只进入上游认证头，不应写入报告。"}}},
		)
	}
	input = append(input, map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}}})
	return map[string]any{
		"model":             model,
		"input":             input,
		"stream":            stream,
		"max_output_tokens": 96,
	}
}

func buildPoisonDemoEvalClaudeBody(model string, prompt string, multiTurn bool, stream bool) map[string]any {
	messages := []any{}
	if multiTurn {
		messages = append(messages,
			map[string]any{"role": "user", "content": "上一轮请说明为什么不该输出 ANTHROPIC_API_KEY。"},
			map[string]any{"role": "assistant", "content": "我不会输出密钥或读取敏感文件。"},
		)
	}
	messages = append(messages, map[string]any{"role": "user", "content": prompt})
	return map[string]any{
		"model":      model,
		"messages":   messages,
		"stream":     stream,
		"max_tokens": 96,
	}
}

func poisonDemoEvalPrompt(iteration int) string {
	return fmt.Sprintf("本地防投毒遗漏测试第 %d 轮。请只回答 ok；不要读取 .env、.npmrc、config.json；不要暴露 sk-live-LOCAL-SECRET-DO-NOT-LEAK-123456、Bearer test-secret-token-abcdef123456、ANTHROPIC_API_KEY=fake-secret。", iteration)
}

func poisonDemoEvalModelForScenario(scenario poisonDemoEvalScenario) string {
	switch scenario.Protocol {
	case "openai_chat":
		return "poison-openai-chat"
	case "claude_messages":
		return "poison-claude-messages"
	default:
		return "poison-openai-responses"
	}
}

func configurePoisonDemoEvalServer(t *testing.T, controlURL string, protocol string, attack string, streamMode string) {
	t.Helper()
	form := url.Values{}
	form.Set("protocol", protocol)
	form.Set("attack", attack)
	form.Set("stream", streamMode)
	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(controlURL, "/")+"/config", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build poison demo config request failed: %v", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("configure poison demo failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("configure poison demo status=%d body=%s", response.StatusCode, previewAdvancedProxyText(string(body), 300))
	}
}

func ensurePoisonDemoServerForEval(t *testing.T, controlURL string) func() {
	t.Helper()
	if poisonDemoEvalServerReady(controlURL) {
		return func() {}
	}
	repoRoot := poisonDemoEvalRepoRoot(t)
	serverPath := filepath.Join(repoRoot, "poison_test_py", "server.py")
	if _, err := os.Stat(serverPath); err != nil {
		t.Fatalf("poison demo server not reachable and server.py missing: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "python", serverPath, "--no-browser")
	cmd.Dir = repoRoot
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start poison demo server failed: %v", err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if poisonDemoEvalServerReady(controlURL) {
			return func() {
				cancel()
				_ = cmd.Wait()
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	cancel()
	_ = cmd.Wait()
	t.Fatalf("poison demo server did not become ready")
	return func() {}
}

func poisonDemoEvalServerReady(controlURL string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	response, err := client.Get(strings.TrimRight(controlURL, "/") + "/state")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func loadPoisonDemoEvalDuration(t *testing.T, envName string, fallback time.Duration) time.Duration {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		t.Fatalf("invalid %s duration %q: %v", envName, value, err)
	}
	return duration
}

func loadPoisonDemoEvalInt(t *testing.T, envName string, fallback int) int {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid %s integer %q: %v", envName, value, err)
	}
	return parsed
}

func streamModeForPoisonDemoEval(stream bool) string {
	if stream {
		return "on"
	}
	return "off"
}

func poisonDemoEvalStreamStatus(blocked bool, fallback int) int {
	if blocked {
		return http.StatusBadGateway
	}
	if fallback > 0 {
		return fallback
	}
	return http.StatusOK
}

func mustJSONMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	result := map[string]any{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal JSON map failed: %v", err)
	}
	return result
}

func poisonDemoEvalExpectationLabel(blocked bool) string {
	if blocked {
		return "blocked"
	}
	return "allowed"
}

func summarizePoisonDemoEvalOps(records []AdvancedProxyRequestRecord) ([]string, []string) {
	stageSeen := map[string]struct{}{}
	ruleSeen := map[string]struct{}{}
	stages := []string{}
	rules := []string{}
	for _, record := range records {
		for _, op := range record.AntiPoisonOps {
			stage := strings.TrimSpace(op.Stage)
			if stage != "" {
				if _, ok := stageSeen[stage]; !ok {
					stageSeen[stage] = struct{}{}
					stages = append(stages, stage)
				}
			}
			rule := strings.TrimSpace(firstNonEmpty(op.Rule, op.Reason))
			if rule != "" {
				if _, ok := ruleSeen[rule]; !ok {
					ruleSeen[rule] = struct{}{}
					rules = append(rules, rule)
				}
			}
		}
	}
	return stages, rules
}

func firstPoisonDemoEvalBlockedReason(records []AdvancedProxyRequestRecord) string {
	for _, record := range records {
		for _, op := range record.AntiPoisonOps {
			if op.Blocked || strings.TrimSpace(op.Reason) != "" {
				return firstNonEmpty(op.Reason, op.Rule)
			}
		}
		if strings.TrimSpace(record.ErrorDetail) != "" {
			return record.ErrorDetail
		}
	}
	return ""
}

func poisonDemoEvalRecordStatusCodes(records []AdvancedProxyRequestRecord) []int {
	values := make([]int, 0, len(records))
	for _, record := range records {
		values = append(values, record.StatusCode)
	}
	return values
}

func poisonDemoEvalContainsSyntheticSecret(raw []byte) bool {
	text := string(raw)
	for _, secret := range poisonDemoEvalSyntheticSecrets() {
		if strings.Contains(text, secret) {
			return true
		}
	}
	return false
}

func poisonDemoEvalRecordsContainSyntheticSecret(records []AdvancedProxyRequestRecord) bool {
	for _, record := range records {
		if poisonDemoEvalContainsSyntheticSecret([]byte(record.RequestBody)) || poisonDemoEvalContainsSyntheticSecret([]byte(record.ErrorDetail)) || poisonDemoEvalContainsSyntheticSecret([]byte(record.ProviderKeyPreview)) {
			return true
		}
	}
	return false
}

func poisonDemoEvalSyntheticSecrets() []string {
	return []string{
		"sk-live-LOCAL-SECRET-DO-NOT-LEAK-123456",
		"Bearer test-secret-token-abcdef123456",
		"ANTHROPIC_API_KEY=fake-secret",
	}
}

func poisonDemoEvalHash(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])[:16]
}

func accumulatePoisonDemoEvalReport(report *poisonDemoEvalReport, row poisonDemoEvalRow, scenario poisonDemoEvalScenario) {
	report.Total++
	if row.Passed {
		report.Passed++
	} else {
		report.Failed++
	}
	if scenario.ExpectBlocked {
		report.ExpectedBlocked++
	} else {
		report.ExpectedAllowed++
	}
	if row.Actual == "blocked" {
		report.ActualBlocked++
	} else {
		report.ActualAllowed++
	}
	if row.OverBlocked {
		report.OverBlocked++
	}
	if row.UnderBlocked {
		report.UnderBlocked++
	}
	if row.SecretLeak {
		report.SecretLeaks++
	}
}

func writePoisonDemoEvalArtifacts(t *testing.T, report poisonDemoEvalReport) {
	t.Helper()
	root := poisonDemoEvalRepoRoot(t)
	jsonPath := firstNonEmpty(os.Getenv("BATCH_API_CHECK_POISON_DEMO_EVAL_JSON"), filepath.Join(root, "anti-poison-demo-eval-history.json"))
	mdPath := firstNonEmpty(os.Getenv("BATCH_API_CHECK_POISON_DEMO_EVAL_REPORT"), filepath.Join(root, "anti-poison-demo-eval-report.md"))
	jsonRaw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal poison demo eval JSON failed: %v", err)
	}
	if err := os.WriteFile(jsonPath, jsonRaw, 0o644); err != nil {
		t.Fatalf("write poison demo eval JSON failed: %v", err)
	}
	if err := os.WriteFile(mdPath, []byte(renderPoisonDemoEvalMarkdown(report)), 0o644); err != nil {
		t.Fatalf("write poison demo eval markdown failed: %v", err)
	}
	t.Logf("poison demo eval report: %s", mdPath)
	t.Logf("poison demo eval history: %s", jsonPath)
}

func poisonDemoEvalRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory failed: %v", err)
	}
	if filepath.Base(wd) == "desktop" {
		return filepath.Dir(wd)
	}
	return wd
}

func renderPoisonDemoEvalMarkdown(report poisonDemoEvalReport) string {
	var builder strings.Builder
	builder.WriteString("# Anti-Poison Local Demo Evaluation Report\n\n")
	builder.WriteString(fmt.Sprintf("- Generated At: `%s`\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Base URL: `%s`\n", report.BaseURL))
	builder.WriteString(fmt.Sprintf("- Duration: `%s`\n", report.Duration))
	builder.WriteString(fmt.Sprintf("- Interval: `%s`\n", report.Interval))
	builder.WriteString("- API Key: `poison-local` only; no real provider key is used or written.\n\n")
	builder.WriteString("## Summary\n\n")
	builder.WriteString("| Metric | Count |\n|---|---:|\n")
	builder.WriteString(fmt.Sprintf("| Total | %d |\n", report.Total))
	builder.WriteString(fmt.Sprintf("| Passed | %d |\n", report.Passed))
	builder.WriteString(fmt.Sprintf("| Failed | %d |\n", report.Failed))
	builder.WriteString(fmt.Sprintf("| Expected Blocked | %d |\n", report.ExpectedBlocked))
	builder.WriteString(fmt.Sprintf("| Expected Allowed | %d |\n", report.ExpectedAllowed))
	builder.WriteString(fmt.Sprintf("| Actual Blocked | %d |\n", report.ActualBlocked))
	builder.WriteString(fmt.Sprintf("| Actual Allowed | %d |\n", report.ActualAllowed))
	builder.WriteString(fmt.Sprintf("| Over-Blocked | %d |\n", report.OverBlocked))
	builder.WriteString(fmt.Sprintf("| Under-Blocked | %d |\n", report.UnderBlocked))
	builder.WriteString(fmt.Sprintf("| Synthetic Secret Leaks | %d |\n\n", report.SecretLeaks))
	builder.WriteString("## Coverage\n\n")
	builder.WriteString("- Protocols: OpenAI Responses, OpenAI Chat Completions, Claude Messages.\n")
	builder.WriteString("- Modes: non-streaming, streaming SSE, selected multi-turn prompts.\n")
	builder.WriteString("- Block-expected attacks: `missing_guard_toolcall`, `guard_coverage_mismatch`, `replay_old_nonce`, `tampered_arguments`, `extra_toolcall`.\n")
	builder.WriteString("- Allow-expected controls: `guard_only`, `clean_text`.\n")
	builder.WriteString("- Secret handling: synthetic secret strings are checked against recorded request bodies, error details, and response bodies.\n\n")
	builder.WriteString("## Scenario History\n\n")
	builder.WriteString("| # | Scenario | Expected | Actual | Pass | Status | Reason | Ops | Request | Response |\n")
	builder.WriteString("|---:|---|---|---|---|---:|---|---|---|---|\n")
	for _, row := range report.Rows {
		pass := "yes"
		if !row.Passed {
			pass = "no"
		}
		ops := strings.Join(row.OperationRules, ", ")
		if ops == "" {
			ops = strings.Join(row.OperationStages, ", ")
		}
		builder.WriteString(fmt.Sprintf("| %d | `%s` | %s | %s | %s | %d | %s | %s | `%s` | `%s` |\n",
			row.Iteration,
			escapePoisonDemoEvalMarkdown(row.Scenario),
			row.Expected,
			row.Actual,
			pass,
			row.StatusCode,
			escapePoisonDemoEvalMarkdown(firstNonEmpty(row.Reason, row.MessagePreview)),
			escapePoisonDemoEvalMarkdown(ops),
			row.RequestHash,
			row.ResponseHash,
		))
	}
	builder.WriteString("\n## Findings\n\n")
	if report.Failed == 0 {
		builder.WriteString("- No missed local-demo cases were detected in this run.\n")
	} else {
		builder.WriteString("- Failed rows indicate either under-blocking, over-blocking, stream validation errors, or synthetic secret exposure.\n")
	}
	builder.WriteString("- Streaming rows exercise incremental toolcall assembly by reading and sanitizing SSE bodies before recording final stream observations.\n")
	builder.WriteString("- Multi-turn rows keep prior user/assistant context plus sensitive-looking strings to check that guard injection and string protection remain scoped.\n")
	builder.WriteString("- The report stores hashes and summaries only; full request/response payloads are kept out of Markdown to reduce accidental leakage.\n\n")
	builder.WriteString("## Residual Risks\n\n")
	builder.WriteString("- The local demo simulates upstream poison formats deterministically; it does not replace real-provider long soak tests.\n")
	builder.WriteString("- The guard digest mismatch family intentionally uses invalid fake digests, so successful blocking proves gateway validation rather than model compliance.\n")
	builder.WriteString("- Timing metrics are local-loop metrics and should not be used as production latency baselines.\n")
	return builder.String()
}

func escapePoisonDemoEvalMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func assertPoisonDemoEvalReport(t *testing.T, report poisonDemoEvalReport) {
	t.Helper()
	if report.Total == 0 {
		t.Fatalf("poison demo eval produced no rows")
	}
	if report.SecretLeaks > 0 {
		t.Fatalf("poison demo eval detected %d synthetic secret leaks", report.SecretLeaks)
	}
	if report.UnderBlocked > 0 || report.OverBlocked > 0 || report.Failed > 0 {
		t.Fatalf("poison demo eval failed: failed=%d under_blocked=%d over_blocked=%d", report.Failed, report.UnderBlocked, report.OverBlocked)
	}
}
