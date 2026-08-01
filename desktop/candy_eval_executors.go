package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Raw Request is deliberately a first-class executor rather than a fallback:
// it supplies the Responses API baseline with no agent system prompt, CLI
// formatter, or client-side reconnect behaviour in the way.
func (a *App) runRawCandyIntelligenceTest(ctx context.Context, runID, model, effort, mode, gatewayURL string, target candyEvalTarget, startedAt int64) {
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "target", Message: fmt.Sprintf("已锁定右键选中密钥：%s（%s）；Raw Request 不读取或修改 Codex/Claude 配置", firstNonEmpty(target.ProviderName, target.ProviderID, "选中密钥"), candyEvalModeLabel(mode)), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, ProviderID: target.ProviderID, ProviderName: target.ProviderName, Executor: candyEvalExecutorRaw, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	a.runCandyEvalBatch(ctx, runID, model, effort, mode, gatewayURL, startedAt, candyEvalExecutorRaw, func(runIndex int) candyEvalRunResult {
		return a.runRawCandyEvalOne(ctx, runID, runIndex, model, effort, mode, gatewayURL, target, startedAt)
	})
}

func (a *App) runRawCandyEvalOne(ctx context.Context, runID string, runIndex int, model, effort, mode, gatewayURL string, target candyEvalTarget, startedAt int64) candyEvalRunResult {
	result := candyEvalRunResult{}
	endpoint := firstNonEmpty(buildResponsesEndpointCandidates(target.SiteURL)...)
	headers := map[string]string{
		"Authorization": "Bearer " + target.APIKey,
		"Content-Type":  "application/json",
		"Accept":        "text/event-stream",
	}
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		endpoint = strings.TrimRight(gatewayURL, "/") + "/responses"
		headers[advancedProxyCandyEvalProviderHeader] = target.ProviderID
		headers[advancedProxyCandyEvalRunHeader] = runID
		headers[advancedProxyCandyEvalTargetURL] = normalizeCandyEvalOpenAIBaseURL(target.SiteURL)
		headers[advancedProxyCandyEvalTargetKey] = target.APIKey
		headers[advancedProxyCandyEvalTargetName] = target.ProviderName
	}
	body := map[string]any{
		"model":  strings.TrimSpace(model),
		"stream": true,
		"input": []any{map[string]any{
			"type": "message", "role": "user",
			"content": []any{map[string]any{"type": "input_text", "text": candyEvalPrompt}},
		}},
		"reasoning": map[string]any{"effort": normalizeCandyEvalEffort(effort)},
		"include":   []string{"reasoning.encrypted_content"},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		result.failureMessage = fmt.Sprintf("构建 Raw Request 失败：%v", err)
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "request", Message: fmt.Sprintf("第 %d/%d 轮：Raw Request -> %s", runIndex, candyEvalDefaultTests, endpoint), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorRaw, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	status, responseHeaders, responseBody, streamBody, elapsed, err := performRawUpstreamRequest("POST", endpoint, headers, rawBody, 90, true)
	result.elapsedSeconds = elapsed.Seconds()
	if streamBody != nil {
		responseBody, err = io.ReadAll(io.LimitReader(streamBody, 16*1024*1024))
		_ = streamBody.Close()
	}
	if ctx.Err() != nil {
		result.canceled = true
		return result
	}
	if err != nil {
		result.failureMessage = fmt.Sprintf("Raw Request 网络错误：%v", err)
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}
	if status < 200 || status >= 300 {
		result.failureMessage = fmt.Sprintf("Raw Request HTTP %d：%s", status, compactCandyEvalText(string(responseBody), 600))
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}

	events := parseAntiCandySSEEvents(responseBody)
	terminal := false
	done := false
	var delta strings.Builder
	for _, event := range events {
		if event.Done {
			done = true
			continue
		}
		if event.Type == "response.completed" || event.Type == "response.failed" || event.Type == "response.incomplete" {
			terminal = true
		}
		if text := candyEvalRawEventText(event.Payload); text != "" {
			delta.WriteString(text)
		}
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "raw-event", Message: fmt.Sprintf("Raw SSE：%s", firstNonEmpty(event.Type, event.EventName, "event")), Text: candyEvalRawEventText(event.Payload), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorRaw, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	}
	if round, ok := parseAntiCandyRound(responseBody); ok {
		for _, item := range round.NonReasoningItems {
			if text := candyEvalResponseItemText(item); text != "" {
				result.finalText = text
			}
		}
		result.inputTokens = candyEvalMapInt64(round.Usage, "input_tokens")
		result.outputTokens = candyEvalMapInt64(round.Usage, "output_tokens")
		result.reasoningTokens = firstNonZeroCandyTokens(candyEvalMapInt64(round.Usage, "reasoning_tokens"), candyEvalMapInt64(round.Usage, "reasoning_output_tokens"))
		_, result.inputTokensSet = round.Usage["input_tokens"]
		_, result.outputTokensSet = round.Usage["output_tokens"]
		_, result.reasoningSet = round.Usage["reasoning_tokens"]
		if !result.reasoningSet {
			_, result.reasoningSet = round.Usage["reasoning_output_tokens"]
		}
	}
	if result.finalText == "" {
		result.finalText = strings.TrimSpace(delta.String())
	}
	if result.finalText == "" {
		payload := map[string]any{}
		if json.Unmarshal(responseBody, &payload) == nil {
			result.finalText = extractResponsesOutputText(payload)
		}
	}
	if !terminal && !done && result.finalText == "" {
		result.failureMessage = "上游未返回可识别的 SSE 终止事件或回答文本"
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}
	if !terminal && (done || result.finalText != "") {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "notice", Message: "上游未发送 response.completed；已保留已收到的回答用于对照 Codex CLI 重连行为", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorRaw, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	}
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		a.emitCandyEvalProxyTrace(runID, runIndex, candyEvalDefaultTests, model, effort, mode, gatewayURL, startedAt)
	}
	correct := candyEvalAnswerPattern.MatchString(result.finalText)
	result.correct = &correct
	if result.outputTokens > 0 && result.elapsedSeconds > 0 {
		result.tps = float64(result.outputTokens) / result.elapsedSeconds
	}
	contentType := strings.TrimSpace(responseHeaders.Get("Content-Type"))
	if contentType != "" {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "transport", Message: fmt.Sprintf("Raw Request HTTP %d，%s", status, compactCandyEvalText(contentType, 100)), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorRaw, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	}
	result.row = []string{strconv.Itoa(runIndex), previewCandyEvalText(result.finalText), candyEvalTokenCell(result.inputTokens, result.inputTokensSet), candyEvalTokenCell(result.outputTokens, result.outputTokensSet), candyEvalTokenCell(result.reasoningTokens, result.reasoningSet), fmt.Sprintf("%.1f", result.elapsedSeconds), candyEvalTPSCell(result.tps), map[bool]string{true: "✓", false: "✗"}[correct]}
	return result
}

func candyEvalRawEventText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if delta := strings.TrimSpace(candyEvalMapString(payload, "delta")); delta != "" {
		return delta
	}
	if response, ok := payload["response"].(map[string]any); ok {
		if text := strings.TrimSpace(candyEvalMapString(response, "output_text")); text != "" {
			return text
		}
	}
	if item, ok := payload["item"].(map[string]any); ok {
		return candyEvalResponseItemText(item)
	}
	return ""
}

func candyEvalResponseItemText(item map[string]any) string {
	if item == nil {
		return ""
	}
	if text := strings.TrimSpace(candyEvalMapString(item, "text")); text != "" {
		return text
	}
	content, _ := item["content"].([]any)
	parts := make([]string, 0, len(content))
	for _, raw := range content {
		part, _ := raw.(map[string]any)
		if text := strings.TrimSpace(candyEvalMapString(part, "text")); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

func firstNonZeroCandyTokens(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func (a *App) runClaudeCandyIntelligenceTest(ctx context.Context, runID, model, effort, mode, gatewayURL string, target candyEvalTarget, startedAt int64) {
	executable, err := resolveCandyClaudeExecutable()
	if err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageError, Kind: "error", Message: err.Error(), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorClaude, StartedAt: startedAt})
		return
	}
	claudeHome, err := os.MkdirTemp("", "allapideck-candy-claude-")
	if err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageError, Kind: "error", Message: fmt.Sprintf("创建临时 Claude 配置失败：%v", err), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorClaude, StartedAt: startedAt})
		return
	}
	defer os.RemoveAll(claudeHome)
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "target", Message: fmt.Sprintf("已锁定右键选中密钥：%s（%s）；使用独立临时 Claude 配置，结束后自动清理", firstNonEmpty(target.ProviderName, target.ProviderID, "选中密钥"), candyEvalModeLabel(mode)), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, ProviderID: target.ProviderID, ProviderName: target.ProviderName, Executor: candyEvalExecutorClaude, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	a.runCandyEvalBatch(ctx, runID, model, effort, mode, gatewayURL, startedAt, candyEvalExecutorClaude, func(runIndex int) candyEvalRunResult {
		return a.runClaudeCandyEvalOne(ctx, executable, claudeHome, runID, runIndex, model, effort, mode, gatewayURL, target, startedAt)
	})
}

func buildCandyClaudeCommand(ctx context.Context, executable, model, effort, mode, gatewayURL string, target candyEvalTarget, claudeHome string) *exec.Cmd {
	args := []string{"-p", candyEvalPrompt, "--bare", "--no-session-persistence", "--output-format", "stream-json", "--include-partial-messages", "--tools", "", "--effort", normalizeCandyEvalEffort(effort)}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", strings.TrimSpace(model))
	}
	command := exec.CommandContext(ctx, executable, args...)
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(executable), ".cmd") || strings.HasSuffix(strings.ToLower(executable), ".bat")) {
		command = exec.CommandContext(ctx, "cmd.exe", append([]string{"/d", "/c", "call", executable}, args...)...)
	}
	command.Env = candyEvalClaudeCommandEnvironment(target, mode, gatewayURL, claudeHome)
	configureBackgroundCmd(command)
	return command
}

func candyEvalClaudeCommandEnvironment(target candyEvalTarget, mode, gatewayURL, claudeHome string) []string {
	env := make([]string, 0, len(os.Environ())+5)
	for _, item := range os.Environ() {
		upper := strings.ToUpper(item)
		if strings.HasPrefix(upper, "ANTHROPIC_API_KEY=") || strings.HasPrefix(upper, "ANTHROPIC_AUTH_TOKEN=") || strings.HasPrefix(upper, "ANTHROPIC_BASE_URL=") || strings.HasPrefix(upper, "CLAUDE_CONFIG_DIR=") {
			continue
		}
		env = append(env, item)
	}
	baseURL := normalizeCandyEvalClaudeBaseURL(target.SiteURL)
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		baseURL = strings.TrimRight(gatewayURL, "/")
	}
	env = append(env, "ANTHROPIC_API_KEY="+target.APIKey, "ANTHROPIC_AUTH_TOKEN="+target.APIKey, "ANTHROPIC_BASE_URL="+baseURL, "CLAUDE_CONFIG_DIR="+filepath.Clean(claudeHome))
	return env
}

func normalizeCandyEvalClaudeBaseURL(siteURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(siteURL), "/")
	lower := strings.ToLower(baseURL)
	switch {
	case strings.HasSuffix(lower, "/v1/messages"):
		return strings.TrimSuffix(baseURL, "/v1/messages")
	case strings.HasSuffix(lower, "/messages"):
		return strings.TrimSuffix(baseURL, "/messages")
	case strings.HasSuffix(lower, "/v1"):
		return strings.TrimSuffix(baseURL, "/v1")
	default:
		return baseURL
	}
}

func (a *App) runClaudeCandyEvalOne(ctx context.Context, executable, claudeHome, runID string, runIndex int, model, effort, mode, gatewayURL string, target candyEvalTarget, startedAt int64) candyEvalRunResult {
	started := time.Now()
	result := candyEvalRunResult{}
	command := buildCandyClaudeCommand(ctx, executable, model, effort, mode, gatewayURL, target, claudeHome)
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	command.Stdout, command.Stderr = stdoutWriter, stderrWriter
	if err := command.Start(); err != nil {
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		result.failureMessage = fmt.Sprintf("启动 Claude CLI 失败：%v", err)
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}
	state := &candyEvalStreamState{}
	stdoutDone, stderrDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stdoutDone)
		a.streamClaudeCandyStdout(runID, runIndex, model, effort, mode, gatewayURL, startedAt, stdout, state)
	}()
	go func() {
		defer close(stderrDone)
		a.streamCandyStderr(runID, runIndex, candyEvalDefaultTests, model, effort, mode, gatewayURL, startedAt, stderr)
	}()
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "system", Message: fmt.Sprintf("第 %d/%d 轮：Claude CLI 已启动，正在读取 stream-json…", runIndex, candyEvalDefaultTests), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorClaude, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	waitErr := command.Wait()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	<-stdoutDone
	<-stderrDone
	result.elapsedSeconds = time.Since(started).Seconds()
	if ctx.Err() != nil {
		result.canceled = true
		return result
	}
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		a.emitCandyEvalProxyTrace(runID, runIndex, candyEvalDefaultTests, model, effort, mode, gatewayURL, startedAt)
	}
	state.mu.Lock()
	result.finalText, result.inputTokens, result.outputTokens, result.reasoningTokens = state.finalText, state.inputTokens, state.outputTokens, state.reasoningOutputTokens
	result.inputTokensSet, result.outputTokensSet, result.reasoningSet = state.inputTokensSet, state.outputTokensSet, state.reasoningTokensSet
	failure := state.failureMessage
	state.mu.Unlock()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.exitCode = exitErr.ExitCode()
		}
		result.failureMessage = firstNonEmpty(failure, waitErr.Error(), "Claude CLI 执行失败")
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}
	correct := candyEvalAnswerPattern.MatchString(result.finalText)
	result.correct = &correct
	if result.outputTokens > 0 && result.elapsedSeconds > 0 {
		result.tps = float64(result.outputTokens) / result.elapsedSeconds
	}
	result.row = []string{strconv.Itoa(runIndex), previewCandyEvalText(result.finalText), candyEvalTokenCell(result.inputTokens, result.inputTokensSet), candyEvalTokenCell(result.outputTokens, result.outputTokensSet), candyEvalTokenCell(result.reasoningTokens, result.reasoningSet), fmt.Sprintf("%.1f", result.elapsedSeconds), candyEvalTPSCell(result.tps), map[bool]string{true: "✓", false: "✗"}[correct]}
	return result
}

func (a *App) streamClaudeCandyStdout(runID string, runIndex int, model, effort, mode, gatewayURL string, startedAt int64, reader io.Reader, state *candyEvalStreamState) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), candyEvalMaxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stdout", Message: compactCandyEvalText(line, 800), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorClaude, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
			continue
		}
		kind, text := describeClaudeCandyPayload(payload)
		if text != "" {
			state.mu.Lock()
			if kind == "answer" {
				state.finalText = text
			} else if kind == "partial" {
				state.finalText += text
			}
			state.mu.Unlock()
		}
		if kind == "error" {
			state.mu.Lock()
			state.failureMessage = firstNonEmpty(text, "Claude CLI 返回错误")
			state.mu.Unlock()
		}
		if usage, ok := payload["usage"].(map[string]any); ok {
			state.mu.Lock()
			state.inputTokens = candyEvalMapInt64(usage, "input_tokens")
			state.outputTokens = candyEvalMapInt64(usage, "output_tokens")
			state.reasoningOutputTokens = firstNonZeroCandyTokens(candyEvalMapInt64(usage, "reasoning_tokens"), candyEvalMapInt64(usage, "thinking_tokens"))
			_, state.inputTokensSet = usage["input_tokens"]
			_, state.outputTokensSet = usage["output_tokens"]
			_, state.reasoningTokensSet = usage["reasoning_tokens"]
			state.mu.Unlock()
		}
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: kind, Message: "Claude stream-json：" + kind, Text: text, Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: candyEvalExecutorClaude, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
	}
}

func describeClaudeCandyPayload(payload map[string]any) (string, string) {
	typeName := strings.TrimSpace(candyEvalMapString(payload, "type"))
	if typeName == "result" {
		return "answer", firstNonEmpty(candyEvalMapString(payload, "result"), candyEvalMapString(payload, "text"))
	}
	if typeName == "error" {
		return "error", firstNonEmpty(candyEvalMapString(payload, "error"), candyEvalMapString(payload, "message"))
	}
	if event, ok := payload["event"].(map[string]any); ok {
		if delta, ok := event["delta"].(map[string]any); ok {
			if text := candyEvalMapString(delta, "text"); text != "" {
				return "partial", text
			}
		}
		if message, ok := event["message"].(map[string]any); ok {
			if content, ok := message["content"].([]any); ok {
				for _, raw := range content {
					if part, ok := raw.(map[string]any); ok {
						if text := candyEvalMapString(part, "text"); text != "" {
							return "answer", text
						}
					}
				}
			}
		}
	}
	return firstNonEmpty(typeName, "event"), ""
}

func (a *App) runCandyEvalBatch(ctx context.Context, runID, model, effort, mode, gatewayURL string, startedAt int64, executor string, runner func(int) candyEvalRunResult) {
	rows := make([][]string, 0, candyEvalDefaultTests)
	graded, correctCount := 0, 0
	latestFinalText := ""
	for runIndex := 1; runIndex <= candyEvalDefaultTests; runIndex++ {
		if ctx.Err() != nil {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageCanceled, Kind: "system", Message: "糖果智力测试已取消", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: executor, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
			return
		}
		result := runner(runIndex)
		if result.canceled {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageCanceled, Kind: "system", Message: "糖果智力测试已取消", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: executor, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})
			return
		}
		rows = append(rows, result.row)
		if result.correct != nil {
			graded++
			if *result.correct {
				correctCount++
			}
			latestFinalText = result.finalText
		}
		kind, resultMessage := "run-result", fmt.Sprintf("第 %d/%d 轮完成：答案未命中 21", runIndex, candyEvalDefaultTests)
		if result.correct != nil && *result.correct {
			resultMessage = fmt.Sprintf("第 %d/%d 轮完成：答案命中 21", runIndex, candyEvalDefaultTests)
		}
		if result.correct == nil {
			kind = "run-error"
			resultMessage = fmt.Sprintf("第 %d/%d 轮失败：%s", runIndex, candyEvalDefaultTests, firstNonEmpty(result.failureMessage, candyEvalExecutorLabel(executor)+" 执行失败"))
		}
		table := renderCandyEvalTable(rows)
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: kind, Message: resultMessage, Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: executor, Run: runIndex, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, InputTokens: result.inputTokens, OutputTokens: result.outputTokens, ReasoningOutputTokens: result.reasoningTokens, ElapsedSeconds: result.elapsedSeconds, TPS: result.tps, ExitCode: result.exitCode, Correct: result.correct, FinalText: result.finalText, Table: table, StartedAt: startedAt})
	}
	accuracy := 0.0
	if graded > 0 {
		accuracy = float64(correctCount) / float64(graded) * 100
	}
	summary := fmt.Sprintf("Graded %d/%d  correct=%d  accuracy=%.1f%%", graded, candyEvalDefaultTests, correctCount, accuracy)
	table := renderCandyEvalTable(rows)
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageCompleted, Kind: "summary", Message: summary, Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Executor: executor, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, FinalText: latestFinalText, FinalOutput: table + "\n\n" + summary, Table: table, Graded: graded, CorrectCount: correctCount, Accuracy: accuracy, StartedAt: startedAt})
}
