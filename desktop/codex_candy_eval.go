package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/width"
)

const (
	candyIntelligenceTestEventName = "codex-candy-eval:progress"
	candyEvalStageStarting         = "starting"
	candyEvalStageStreaming        = "streaming"
	candyEvalStageCompleted        = "completed"
	candyEvalStageError            = "error"
	candyEvalStageCanceled         = "canceled"
	candyEvalModeDirect            = "direct"
	candyEvalModeGateway           = "gateway"
	candyEvalGatewayProvider       = "allapideck_gateway"
	candyEvalMaxLineBytes          = 4 << 20
	candyEvalDefaultTests          = 5
)

const candyEvalPrompt = `不使用任何外部工具回答以下问题：

在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

        苹果味  桃子味  西瓜味
圆形       7      9      8
五角星形   7      6      4
`

var candyEvalAnswerPattern = regexp.MustCompile(`(^|[^0-9])21([^0-9]|$)`)

type candyIntelligenceTestEvent struct {
	RunID                 string  `json:"runId"`
	Stage                 string  `json:"stage"`
	Kind                  string  `json:"kind,omitempty"`
	Message               string  `json:"message,omitempty"`
	Text                  string  `json:"text,omitempty"`
	Raw                   string  `json:"raw,omitempty"`
	Model                 string  `json:"model,omitempty"`
	Effort                string  `json:"effort,omitempty"`
	Mode                  string  `json:"mode,omitempty"`
	GatewayURL            string  `json:"gatewayUrl,omitempty"`
	Run                   int     `json:"run,omitempty"`
	TotalRuns             int     `json:"totalRuns,omitempty"`
	Tests                 int     `json:"tests,omitempty"`
	InputTokens           int64   `json:"inputTokens,omitempty"`
	OutputTokens          int64   `json:"outputTokens,omitempty"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens,omitempty"`
	ElapsedSeconds        float64 `json:"elapsedSeconds,omitempty"`
	TPS                   float64 `json:"tps,omitempty"`
	ExitCode              int     `json:"exitCode,omitempty"`
	Correct               *bool   `json:"correct,omitempty"`
	FinalText             string  `json:"finalText,omitempty"`
	FinalOutput           string  `json:"finalOutput,omitempty"`
	Table                 string  `json:"table,omitempty"`
	Graded                int     `json:"graded,omitempty"`
	CorrectCount          int     `json:"correctCount,omitempty"`
	Accuracy              float64 `json:"accuracy,omitempty"`
	StartedAt             int64   `json:"startedAt,omitempty"`
	UpdatedAt             int64   `json:"updatedAt"`
}

type candyEvalStreamState struct {
	mu                    sync.Mutex
	threadID              string
	finalText             string
	failureMessage        string
	inputTokens           int64
	outputTokens          int64
	reasoningOutputTokens int64
	inputTokensSet        bool
	outputTokensSet       bool
	reasoningTokensSet    bool
}

func normalizeCandyEvalEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high", "xhigh", "max", "ultra":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "high"
	}
}

func normalizeCandyEvalMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), candyEvalModeGateway) {
		return candyEvalModeGateway
	}
	return candyEvalModeDirect
}

func candyEvalModeLabel(mode string) string {
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		return "代理网关"
	}
	return "直连"
}

func resolveCandyCodexExecutable() (string, error) {
	candidates := []string{"codex"}
	if runtime.GOOS == "windows" {
		candidates = []string{"codex.cmd", "codex.exe", "codex"}
	}
	for _, candidate := range candidates {
		if executable, err := exec.LookPath(candidate); err == nil && strings.TrimSpace(executable) != "" {
			return executable, nil
		}
	}
	return "", errors.New("找不到 codex CLI，请先安装 Codex CLI 并确保它已加入 PATH")
}

func buildCandyCodexCommand(ctx context.Context, executable, model, effort, mode, gatewayURL string) *exec.Cmd {
	args := []string{
		"exec", "--json",
		"--skip-git-repo-check",
		"--ephemeral",
		"-s", "read-only",
		"--disable", "memories",
		"-c", fmt.Sprintf("model_reasoning_effort=%s", normalizeCandyEvalEffort(effort)),
	}
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway && strings.TrimSpace(gatewayURL) != "" {
		args = append(args,
			"-c", fmt.Sprintf("model_provider=%s", candyEvalGatewayProvider),
			"-c", fmt.Sprintf("model_providers.%s.name=AllApiDeck Advanced Proxy", candyEvalGatewayProvider),
			"-c", fmt.Sprintf("model_providers.%s.base_url=%s", candyEvalGatewayProvider, strings.TrimSpace(gatewayURL)),
			"-c", fmt.Sprintf("model_providers.%s.wire_api=responses", candyEvalGatewayProvider),
			"-c", fmt.Sprintf("model_providers.%s.requires_openai_auth=true", candyEvalGatewayProvider),
		)
	}
	if normalizedModel := strings.TrimSpace(model); normalizedModel != "" {
		args = append(args, "-m", normalizedModel)
	}

	var command *exec.Cmd
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(executable), ".cmd") || strings.HasSuffix(strings.ToLower(executable), ".bat")) {
		commandLine := quoteCandyCommandArg(executable)
		for _, arg := range args {
			commandLine += " " + quoteCandyCommandArg(arg)
		}
		command = exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", commandLine)
	} else {
		command = exec.CommandContext(ctx, executable, args...)
	}
	command.Stdin = strings.NewReader(candyEvalPrompt)
	configureBackgroundCmd(command)
	return command
}

func quoteCandyCommandArg(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"&|<>^()") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func (a *App) StartCandyIntelligenceTest(runID, model, effort, mode string) error {
	if a == nil || a.ctx == nil {
		return errors.New("桌面运行时不可用，无法启动糖果智力测试")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return errors.New("糖果智力测试缺少运行标识")
	}
	if _, err := resolveCandyCodexExecutable(); err != nil {
		return err
	}
	mode = normalizeCandyEvalMode(mode)
	gatewayURL := ""
	if mode == candyEvalModeGateway {
		if err := a.ensureBridgeServer(); err != nil {
			return fmt.Errorf("启动代理网关失败：%w", err)
		}
		gatewayURL = currentBridgeServerURLWithPath(advancedProxyCodexBasePath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.candyEvalMu.Lock()
	if a.candyEvalCancels == nil {
		a.candyEvalCancels = make(map[string]context.CancelFunc)
	}
	if _, exists := a.candyEvalCancels[runID]; exists {
		a.candyEvalMu.Unlock()
		cancel()
		return fmt.Errorf("糖果智力测试已在运行：%s", runID)
	}
	a.candyEvalCancels[runID] = cancel
	a.candyEvalMu.Unlock()

	model = strings.TrimSpace(model)
	effort = normalizeCandyEvalEffort(effort)
	startedAt := time.Now().UnixMilli()
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
		RunID:      runID,
		Stage:      candyEvalStageStarting,
		Kind:       "system",
		Message:    fmt.Sprintf("正在启动 Codex CLI（链路：%s，模型：%s，思考量：%s）", candyEvalModeLabel(mode), firstNonEmpty(model, "默认模型"), effort),
		Model:      model,
		Effort:     effort,
		Mode:       mode,
		GatewayURL: gatewayURL,
		TotalRuns:  candyEvalDefaultTests,
		Tests:      candyEvalDefaultTests,
		StartedAt:  startedAt,
	})

	go func() {
		defer a.unregisterCandyIntelligenceTest(runID)
		a.runCandyIntelligenceTest(ctx, runID, model, effort, mode, gatewayURL, startedAt)
	}()
	return nil
}

func (a *App) CancelCandyIntelligenceTest(runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" || a == nil {
		return nil
	}
	a.candyEvalMu.Lock()
	cancel := a.candyEvalCancels[runID]
	a.candyEvalMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (a *App) cancelAllCandyIntelligenceTests() {
	if a == nil {
		return
	}
	a.candyEvalMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.candyEvalCancels))
	for _, cancel := range a.candyEvalCancels {
		cancels = append(cancels, cancel)
	}
	a.candyEvalCancels = make(map[string]context.CancelFunc)
	a.candyEvalMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (a *App) unregisterCandyIntelligenceTest(runID string) {
	a.candyEvalMu.Lock()
	delete(a.candyEvalCancels, strings.TrimSpace(runID))
	a.candyEvalMu.Unlock()
}

type candyEvalRunResult struct {
	finalText       string
	failureMessage  string
	inputTokens     int64
	outputTokens    int64
	reasoningTokens int64
	inputTokensSet  bool
	outputTokensSet bool
	reasoningSet    bool
	elapsedSeconds  float64
	tps             float64
	exitCode        int
	correct         *bool
	canceled        bool
	row             []string
}

func (a *App) runCandyIntelligenceTest(ctx context.Context, runID, model, effort, mode, gatewayURL string, startedAt int64) {
	executable, err := resolveCandyCodexExecutable()
	if err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
			RunID: runID, Stage: candyEvalStageError, Kind: "error", Message: err.Error(), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, StartedAt: startedAt,
		})
		return
	}

	totalRuns := candyEvalDefaultTests
	rows := make([][]string, 0, totalRuns)
	graded := 0
	correctCount := 0
	var latestFinalText string

	for runIndex := 1; runIndex <= totalRuns; runIndex++ {
		if ctx.Err() != nil {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
				RunID: runID, Stage: candyEvalStageCanceled, Kind: "system", Message: "糖果智力测试已取消", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt,
			})
			return
		}

		result := a.runCandyEvalOne(ctx, executable, runID, runIndex, totalRuns, model, effort, mode, gatewayURL, startedAt)
		if result.canceled {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
				RunID: runID, Stage: candyEvalStageCanceled, Kind: "system", Message: "糖果智力测试已取消", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt,
			})
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

		table := renderCandyEvalTable(rows)
		kind := "run-result"
		message := fmt.Sprintf("第 %d/%d 轮完成：%s", runIndex, totalRuns, "答案未命中 21")
		if result.correct != nil && *result.correct {
			message = fmt.Sprintf("第 %d/%d 轮完成：答案命中 21", runIndex, totalRuns)
		}
		if result.correct == nil {
			kind = "run-error"
			message = fmt.Sprintf("第 %d/%d 轮失败：%s", runIndex, totalRuns, firstNonEmpty(result.failureMessage, "Codex CLI 执行失败"))
		}

		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
			RunID:                 runID,
			Stage:                 candyEvalStageStreaming,
			Kind:                  kind,
			Message:               message,
			Model:                 model,
			Effort:                effort,
			Mode:                  mode,
			GatewayURL:            gatewayURL,
			Run:                   runIndex,
			TotalRuns:             totalRuns,
			Tests:                 totalRuns,
			InputTokens:           result.inputTokens,
			OutputTokens:          result.outputTokens,
			ReasoningOutputTokens: result.reasoningTokens,
			ElapsedSeconds:        result.elapsedSeconds,
			TPS:                   result.tps,
			ExitCode:              result.exitCode,
			Correct:               result.correct,
			FinalText:             result.finalText,
			Table:                 table,
			StartedAt:             startedAt,
		})
	}

	accuracy := 0.0
	if graded > 0 {
		accuracy = float64(correctCount) / float64(graded) * 100
	}
	summary := fmt.Sprintf("Graded %d/%d  correct=%d  accuracy=%.1f%%", graded, totalRuns, correctCount, accuracy)
	table := renderCandyEvalTable(rows)
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
		RunID:        runID,
		Stage:        candyEvalStageCompleted,
		Kind:         "summary",
		Message:      summary,
		Model:        model,
		Effort:       effort,
		Mode:         mode,
		GatewayURL:   gatewayURL,
		TotalRuns:    totalRuns,
		Tests:        totalRuns,
		FinalText:    latestFinalText,
		FinalOutput:  table + "\n\n" + summary,
		Table:        table,
		Graded:       graded,
		CorrectCount: correctCount,
		Accuracy:     accuracy,
		StartedAt:    startedAt,
	})
}

func (a *App) runCandyEvalOne(ctx context.Context, executable, runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, startedAt int64) candyEvalRunResult {
	started := time.Now()
	result := candyEvalRunResult{}
	command := buildCandyCodexCommand(ctx, executable, model, effort, mode, gatewayURL)
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter

	if err := command.Start(); err != nil {
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		result.failureMessage = fmt.Sprintf("启动 Codex CLI 失败：%v", err)
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}

	state := &candyEvalStreamState{}
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		a.streamCandyStdout(runID, runIndex, totalRuns, model, effort, mode, gatewayURL, startedAt, stdout, state)
	}()
	go func() {
		defer close(stderrDone)
		a.streamCandyStderr(runID, runIndex, totalRuns, model, effort, mode, gatewayURL, startedAt, stderr)
	}()

	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
		RunID: runID, Stage: candyEvalStageStreaming, Kind: "system", Message: fmt.Sprintf("第 %d/%d 轮：Codex CLI 已启动，正在读取进展流…", runIndex, totalRuns), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt,
	})

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
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.exitCode = exitErr.ExitCode()
		}
		state.mu.Lock()
		failureMessage := state.failureMessage
		state.mu.Unlock()
		result.failureMessage = firstNonEmpty(failureMessage, waitErr.Error(), "Codex CLI 执行失败")
		result.row = candyEvalErrorRow(runIndex, result.failureMessage)
		return result
	}

	state.mu.Lock()
	result.finalText = state.finalText
	result.inputTokens = state.inputTokens
	result.outputTokens = state.outputTokens
	result.reasoningTokens = state.reasoningOutputTokens
	result.inputTokensSet = state.inputTokensSet
	result.outputTokensSet = state.outputTokensSet
	result.reasoningSet = state.reasoningTokensSet
	state.mu.Unlock()
	correct := candyEvalAnswerPattern.MatchString(result.finalText)
	result.correct = &correct
	if result.outputTokens > 0 && result.elapsedSeconds > 0 {
		result.tps = float64(result.outputTokens) / result.elapsedSeconds
	}
	result.row = []string{
		strconv.Itoa(runIndex),
		previewCandyEvalText(result.finalText),
		candyEvalTokenCell(result.inputTokens, result.inputTokensSet),
		candyEvalTokenCell(result.outputTokens, result.outputTokensSet),
		candyEvalTokenCell(result.reasoningTokens, result.reasoningSet),
		fmt.Sprintf("%.1f", result.elapsedSeconds),
		candyEvalTPSCell(result.tps),
		map[bool]string{true: "✓", false: "✗"}[correct],
	}
	return result
}

func candyEvalErrorRow(runIndex int, message string) []string {
	return []string{strconv.Itoa(runIndex), "ERROR: " + previewCandyEvalText(message), "-", "-", "-", "-", "-", "-"}
}

func candyEvalTPSCell(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", value)
}

func candyEvalTokenCell(value int64, present bool) string {
	if !present {
		return "None"
	}
	return strconv.FormatInt(value, 10)
}

func (a *App) streamCandyStdout(runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, startedAt int64, reader io.Reader, state *candyEvalStreamState) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), candyEvalMaxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stdout", Message: compactCandyEvalText(line, 800), Raw: compactCandyEvalText(line, 4000), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt})
			continue
		}
		a.captureCandyEvalPayload(payload, state)
		kind, message, text := describeCandyEvalPayload(payload)
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: kind, Message: message, Text: text, Raw: compactCandyEvalText(line, 4000), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt})
	}
	if err := scanner.Err(); err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stdout-error", Message: fmt.Sprintf("读取 Codex 输出失败：%v", err), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt})
	}
}

func (a *App) streamCandyStderr(runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, startedAt int64, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 16*1024), candyEvalMaxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stderr", Message: compactCandyEvalText(line, 1200), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt})
	}
}

func (a *App) captureCandyEvalPayload(payload map[string]any, state *candyEvalStreamState) {
	eventType := candyEvalMapString(payload, "type")
	if eventType == "thread.started" {
		threadID := firstNonEmpty(candyEvalMapString(payload, "thread_id"), candyEvalMapString(payload, "threadId"))
		if threadID != "" {
			state.mu.Lock()
			state.threadID = threadID
			state.mu.Unlock()
		}
	}
	item, _ := payload["item"].(map[string]any)
	itemType := candyEvalMapString(item, "type")
	if eventType == "item.completed" && itemType == "agent_message" {
		finalText := candyEvalPayloadText(item)
		state.mu.Lock()
		state.finalText = finalText
		state.mu.Unlock()
	}
	if eventType == "turn.failed" || eventType == "error" {
		state.mu.Lock()
		state.failureMessage = firstNonEmpty(candyEvalPayloadText(payload), "Codex CLI 返回错误")
		state.mu.Unlock()
	}
	if eventType != "turn.completed" {
		return
	}
	usage, _ := payload["usage"].(map[string]any)
	state.mu.Lock()
	state.inputTokens = candyEvalMapInt64(usage, "input_tokens")
	state.outputTokens = candyEvalMapInt64(usage, "output_tokens")
	state.reasoningOutputTokens = candyEvalMapInt64(usage, "reasoning_output_tokens")
	_, state.inputTokensSet = usage["input_tokens"]
	_, state.outputTokensSet = usage["output_tokens"]
	_, state.reasoningTokensSet = usage["reasoning_output_tokens"]
	state.mu.Unlock()
}

func describeCandyEvalPayload(payload map[string]any) (kind, message, text string) {
	eventType := candyEvalMapString(payload, "type")
	item, _ := payload["item"].(map[string]any)
	itemType := candyEvalMapString(item, "type")
	text = candyEvalPayloadText(item)
	if text == "" {
		text = candyEvalPayloadText(payload)
	}

	switch {
	case eventType == "thread.started":
		return "thread", "Codex 会话已启动", text
	case eventType == "turn.started":
		return "turn", "开始推理", text
	case eventType == "turn.completed":
		return "turn", "推理回合完成，正在读取统计", text
	case eventType == "turn.failed" || eventType == "error":
		return "error", firstNonEmpty(text, "Codex 返回错误"), text
	case itemType == "reasoning" && strings.HasSuffix(eventType, ".started"):
		return "reasoning", "正在思考…", text
	case itemType == "reasoning" && strings.HasSuffix(eventType, ".completed"):
		return "reasoning", "思考阶段完成", text
	case itemType == "agent_message" && eventType == "item.completed":
		return "answer", "模型回答已生成", text
	case itemType != "":
		return itemType, fmt.Sprintf("收到 %s（%s）", eventType, itemType), text
	case eventType != "":
		return eventType, fmt.Sprintf("收到事件：%s", eventType), text
	default:
		return "event", "收到 Codex 进展", text
	}
}

func candyEvalPayloadText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	for _, key := range []string{"text", "message", "error", "output"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if summary, ok := payload["summary"]; ok {
		if value, ok := summary.(string); ok {
			return strings.TrimSpace(value)
		}
		if raw, err := json.Marshal(summary); err == nil {
			return string(raw)
		}
	}
	return ""
}

func candyEvalMapString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func candyEvalMapInt64(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	switch value := payload[key].(type) {
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed
	default:
		return 0
	}
}

func candyEvalCharWidth(char rune) int {
	if unicode.Is(unicode.Mn, char) || unicode.Is(unicode.Mc, char) || unicode.Is(unicode.Me, char) {
		return 0
	}
	switch width.LookupRune(char).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

func candyEvalDisplayWidth(text string) int {
	total := 0
	for _, char := range text {
		total += candyEvalCharWidth(char)
	}
	return total
}

func candyEvalPad(text string, targetWidth int, align string) string {
	gap := targetWidth - candyEvalDisplayWidth(text)
	if gap <= 0 {
		return text
	}
	switch align {
	case "right":
		return strings.Repeat(" ", gap) + text
	case "center":
		left := gap / 2
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", gap-left)
	default:
		return text + strings.Repeat(" ", gap)
	}
}

func renderCandyEvalTable(rows [][]string) string {
	headers := []string{"Run", "Codex", "In Tok", "Out Tok", "Reason Tok", "Time(s)", "TPS", "OK"}
	aligns := []string{"right", "left", "right", "right", "right", "right", "right", "center"}
	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = candyEvalDisplayWidth(header)
	}
	for _, row := range rows {
		for index := range headers {
			cell := ""
			if index < len(row) {
				cell = row[index]
			}
			if cellWidth := candyEvalDisplayWidth(cell); cellWidth > widths[index] {
				widths[index] = cellWidth
			}
		}
	}

	formatRow := func(row []string) string {
		cells := make([]string, len(headers))
		for index := range headers {
			cell := ""
			if index < len(row) {
				cell = row[index]
			}
			cells[index] = candyEvalPad(cell, widths[index], aligns[index])
		}
		return strings.Join(cells, "  ")
	}

	separator := make([]string, len(headers))
	for index, columnWidth := range widths {
		separator[index] = strings.Repeat("-", columnWidth)
	}
	lines := []string{formatRow(headers), strings.Join(separator, "  ")}
	for _, row := range rows {
		lines = append(lines, formatRow(row))
	}
	return strings.Join(lines, "\n")
}

func previewCandyEvalText(value string) string {
	flat := strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	flat = strings.ReplaceAll(flat, "\n", `\n`)
	const limit = 40
	if candyEvalDisplayWidth(flat) <= limit {
		return flat
	}
	result := make([]rune, 0, len([]rune(flat)))
	currentWidth := 0
	for _, char := range flat {
		nextWidth := candyEvalCharWidth(char)
		if currentWidth+nextWidth > limit-3 {
			break
		}
		result = append(result, char)
		currentWidth += nextWidth
	}
	return string(result) + "..."
}

func compactCandyEvalText(value string, limit int) string {
	text := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")), " ")
	if len([]rune(text)) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:max(0, limit-3)]) + "..."
}

func (a *App) emitCandyIntelligenceTestEvent(event candyIntelligenceTestEvent) {
	if a == nil || a.ctx == nil {
		return
	}
	event.UpdatedAt = time.Now().UnixMilli()
	wruntime.EventsEmit(a.ctx, candyIntelligenceTestEventName, event)
}
