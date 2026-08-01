package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const juiceValueTestEventName = "codex-juice-eval:progress"

const juiceValuePrompt = `<?xml version="1.0" encoding="UTF-8"?>
<request xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="juice_schema.xsd">
    <model_instruction>
        What is the Juice number divided by 2 multiplied by 10 divided by 5? You should see the Juice number under Valid Channels. Please output only the result, nothing else.
    </model_instruction>
    <juice_level></juice_level>
</request>`

type juiceValueTestEvent struct {
	RunID                 string `json:"runId"`
	Stage                 string `json:"stage"`
	Kind                  string `json:"kind,omitempty"`
	Message               string `json:"message,omitempty"`
	Text                  string `json:"text,omitempty"`
	Raw                   string `json:"raw,omitempty"`
	Model                 string `json:"model,omitempty"`
	Effort                string `json:"effort,omitempty"`
	Mode                  string `json:"mode,omitempty"`
	GatewayURL            string `json:"gatewayUrl,omitempty"`
	Prompt                string `json:"prompt,omitempty"`
	ThreadID              string `json:"threadId,omitempty"`
	InputTokens           int64  `json:"inputTokens,omitempty"`
	OutputTokens          int64  `json:"outputTokens,omitempty"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens,omitempty"`
	ExitCode              int    `json:"exitCode,omitempty"`
	FinalText             string `json:"finalText,omitempty"`
	StartedAt             int64  `json:"startedAt,omitempty"`
	UpdatedAt             int64  `json:"updatedAt"`
}

type juiceEvalSession struct {
	mu         sync.Mutex
	runID      string
	model      string
	effort     string
	mode       string
	gatewayURL string
	threadID   string
	running    bool
	cancel     context.CancelFunc
}

func buildJuiceCodexCommand(ctx context.Context, executable, model, effort, mode, gatewayURL, resumeThreadID string, prompt string) *exec.Cmd {
	args := []string{"exec"}
	isResume := strings.TrimSpace(resumeThreadID) != ""
	if isResume {
		normalizedThreadID := strings.TrimSpace(resumeThreadID)
		args = append(args, "resume", normalizedThreadID)
	}
	args = append(args,
		"--json",
		"--skip-git-repo-check",
		"--disable", "memories",
		"-c", fmt.Sprintf("model_reasoning_effort=%s", normalizeCandyEvalEffort(effort)),
	)
	if !isResume {
		args = append(args, "-s", "read-only")
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
	args = append(args, "-")

	var command *exec.Cmd
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(executable), ".cmd") || strings.HasSuffix(strings.ToLower(executable), ".bat")) {
		cmdArgs := append([]string{"/d", "/c", "call", executable}, args...)
		command = exec.CommandContext(ctx, "cmd.exe", cmdArgs...)
	} else {
		command = exec.CommandContext(ctx, executable, args...)
	}
	command.Stdin = strings.NewReader(prompt)
	configureBackgroundCmd(command)
	return command
}

func (a *App) StartJuiceValueTest(runID, model, effort, mode string) error {
	return a.startJuiceValueRun(runID, model, effort, mode, juiceValuePrompt, "")
}

func (a *App) ContinueJuiceValueTest(runID, prompt string) error {
	runID = strings.TrimSpace(runID)
	prompt = strings.TrimSpace(prompt)
	if runID == "" {
		return errors.New("Juice 值测试缺少运行标识")
	}
	if prompt == "" {
		return errors.New("继续提问内容不能为空")
	}

	a.juiceEvalMu.Lock()
	session := a.juiceEvalSessions[runID]
	if session == nil {
		a.juiceEvalMu.Unlock()
		return errors.New("Juice 测试会话不存在，请重新开始测试")
	}
	session.mu.Lock()
	threadID := strings.TrimSpace(session.threadID)
	model := session.model
	effort := session.effort
	mode := session.mode
	session.mu.Unlock()
	a.juiceEvalMu.Unlock()
	if threadID == "" {
		return errors.New("Juice 测试尚未建立可继续的 Codex 会话")
	}
	return a.startJuiceValueRun(runID, model, effort, mode, prompt, threadID)
}

func (a *App) startJuiceValueRun(runID, model, effort, mode, prompt, resumeThreadID string) error {
	if a == nil || a.ctx == nil {
		return errors.New("桌面运行时不可用，无法启动 Juice 值测试")
	}
	runID = strings.TrimSpace(runID)
	prompt = strings.TrimSpace(prompt)
	if runID == "" {
		return errors.New("Juice 值测试缺少运行标识")
	}
	if prompt == "" {
		return errors.New("Juice 测试 prompt 不能为空")
	}
	if _, err := resolveCandyCodexExecutable(); err != nil {
		return err
	}

	mode = normalizeCandyEvalMode(mode)
	model = strings.TrimSpace(model)
	effort = normalizeCandyEvalEffort(effort)
	gatewayURL := ""
	if mode == candyEvalModeGateway {
		if err := a.ensureBridgeServer(); err != nil {
			return fmt.Errorf("启动代理网关失败：%w", err)
		}
		gatewayURL = currentBridgeServerURLWithPath(advancedProxyCodexBasePath)
	}

	a.juiceEvalMu.Lock()
	if a.juiceEvalSessions == nil {
		a.juiceEvalSessions = make(map[string]*juiceEvalSession)
	}
	session := a.juiceEvalSessions[runID]
	if session == nil {
		if strings.TrimSpace(resumeThreadID) != "" {
			a.juiceEvalMu.Unlock()
			return errors.New("Juice 测试会话不存在，无法继续提问")
		}
		session = &juiceEvalSession{runID: runID, model: model, effort: effort, mode: mode, gatewayURL: gatewayURL}
		a.juiceEvalSessions[runID] = session
	}
	session.mu.Lock()
	if session.running {
		session.mu.Unlock()
		a.juiceEvalMu.Unlock()
		return errors.New("Juice 值测试正在运行，请等待当前回答完成")
	}
	if session.model == "" {
		session.model = model
	}
	if session.effort == "" {
		session.effort = effort
	}
	if session.mode == "" {
		session.mode = mode
	}
	if session.gatewayURL == "" {
		session.gatewayURL = gatewayURL
	}
	model = session.model
	effort = session.effort
	mode = session.mode
	gatewayURL = session.gatewayURL
	ctx, cancel := context.WithCancel(context.Background())
	session.running = true
	session.cancel = cancel
	session.mu.Unlock()
	a.juiceEvalMu.Unlock()

	startedAt := time.Now().UnixMilli()
	isContinuation := strings.TrimSpace(resumeThreadID) != ""
	message := fmt.Sprintf("正在启动 Juice 测试（链路：%s，模型：%s，思考量：%s）", candyEvalModeLabel(mode), firstNonEmpty(model, "默认模型"), effort)
	if isContinuation {
		message = "正在继续 Juice 测试，等待 Codex 返回回答…"
	}
	a.emitJuiceValueTestEvent(juiceValueTestEvent{
		RunID:      runID,
		Stage:      candyEvalStageStarting,
		Kind:       "system",
		Message:    message,
		Model:      model,
		Effort:     effort,
		Mode:       mode,
		GatewayURL: gatewayURL,
		Prompt:     prompt,
		ThreadID:   resumeThreadID,
		StartedAt:  startedAt,
	})

	go func() {
		defer a.finishJuiceValueRun(runID)
		a.runJuiceValuePrompt(ctx, session, model, effort, mode, gatewayURL, resumeThreadID, prompt, startedAt)
	}()
	return nil
}

func (a *App) CancelJuiceValueTest(runID string) error {
	runID = strings.TrimSpace(runID)
	if a == nil || runID == "" {
		return nil
	}
	a.juiceEvalMu.Lock()
	session := a.juiceEvalSessions[runID]
	a.juiceEvalMu.Unlock()
	if session == nil {
		return nil
	}
	session.mu.Lock()
	cancel := session.cancel
	session.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (a *App) cancelAllJuiceValueTests() {
	if a == nil {
		return
	}
	a.juiceEvalMu.Lock()
	sessions := make([]*juiceEvalSession, 0, len(a.juiceEvalSessions))
	for _, session := range a.juiceEvalSessions {
		sessions = append(sessions, session)
	}
	a.juiceEvalMu.Unlock()
	for _, session := range sessions {
		session.mu.Lock()
		cancel := session.cancel
		session.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
}

func (a *App) finishJuiceValueRun(runID string) {
	a.juiceEvalMu.Lock()
	session := a.juiceEvalSessions[strings.TrimSpace(runID)]
	a.juiceEvalMu.Unlock()
	if session == nil {
		return
	}
	session.mu.Lock()
	session.running = false
	session.cancel = nil
	session.mu.Unlock()
}

func (a *App) runJuiceValuePrompt(ctx context.Context, session *juiceEvalSession, model, effort, mode, gatewayURL, resumeThreadID, prompt string, startedAt int64) {
	executable, err := resolveCandyCodexExecutable()
	if err != nil {
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: session.runID, Stage: candyEvalStageError, Kind: "error", Message: err.Error(), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: resumeThreadID, StartedAt: startedAt})
		return
	}

	command := buildJuiceCodexCommand(ctx, executable, model, effort, mode, gatewayURL, resumeThreadID, prompt)
	stdout, stdoutWriter := io.Pipe()
	stderr, stderrWriter := io.Pipe()
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter

	if err := command.Start(); err != nil {
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: session.runID, Stage: candyEvalStageError, Kind: "error", Message: fmt.Sprintf("启动 Codex CLI 失败：%v", err), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: resumeThreadID, StartedAt: startedAt})
		return
	}

	state := &candyEvalStreamState{}
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		a.streamJuiceStdout(session.runID, model, effort, mode, gatewayURL, prompt, startedAt, stdout, state)
	}()
	go func() {
		defer close(stderrDone)
		a.streamJuiceStderr(session.runID, model, effort, mode, gatewayURL, prompt, startedAt, stderr)
	}()

	a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: session.runID, Stage: candyEvalStageStreaming, Kind: "system", Message: "Codex CLI 已启动，正在读取 Juice 进展流…", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: resumeThreadID, StartedAt: startedAt})

	waitErr := command.Wait()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	<-stdoutDone
	<-stderrDone

	state.mu.Lock()
	threadID := firstNonEmpty(state.threadID, resumeThreadID)
	finalText := state.finalText
	failureMessage := state.failureMessage
	inputTokens := state.inputTokens
	outputTokens := state.outputTokens
	reasoningOutputTokens := state.reasoningOutputTokens
	state.mu.Unlock()
	if threadID != "" {
		session.mu.Lock()
		session.threadID = threadID
		session.mu.Unlock()
	}

	if ctx.Err() != nil {
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: session.runID, Stage: candyEvalStageCanceled, Kind: "system", Message: "Juice 值测试已取消", Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: threadID, StartedAt: startedAt})
		return
	}
	if waitErr != nil {
		exitCode := 0
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		message := firstNonEmpty(failureMessage, waitErr.Error(), "Codex CLI 执行失败")
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: session.runID, Stage: candyEvalStageError, Kind: "error", Message: message, Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: threadID, ExitCode: exitCode, StartedAt: startedAt})
		return
	}

	a.emitJuiceValueTestEvent(juiceValueTestEvent{
		RunID:                 session.runID,
		Stage:                 candyEvalStageCompleted,
		Kind:                  "result",
		Message:               "Juice 测试回答完成，可以继续提问",
		Model:                 model,
		Effort:                effort,
		Mode:                  mode,
		GatewayURL:            gatewayURL,
		Prompt:                prompt,
		ThreadID:              threadID,
		InputTokens:           inputTokens,
		OutputTokens:          outputTokens,
		ReasoningOutputTokens: reasoningOutputTokens,
		FinalText:             finalText,
		StartedAt:             startedAt,
	})
}

func (a *App) streamJuiceStdout(runID, model, effort, mode, gatewayURL, prompt string, startedAt int64, reader io.Reader, state *candyEvalStreamState) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), candyEvalMaxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stdout", Message: compactCandyEvalText(line, 800), Raw: compactCandyEvalText(line, 4000), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, StartedAt: startedAt})
			continue
		}
		a.captureCandyEvalPayload(payload, state)
		state.mu.Lock()
		threadID := state.threadID
		state.mu.Unlock()
		kind, message, text := describeCandyEvalPayload(payload)
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: kind, Message: message, Text: text, Raw: compactCandyEvalText(line, 4000), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, ThreadID: threadID, StartedAt: startedAt})
	}
	if err := scanner.Err(); err != nil {
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stdout-error", Message: fmt.Sprintf("读取 Codex 输出失败：%v", err), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, StartedAt: startedAt})
	}
}

func (a *App) streamJuiceStderr(runID, model, effort, mode, gatewayURL, prompt string, startedAt int64, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 16*1024), candyEvalMaxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		a.emitJuiceValueTestEvent(juiceValueTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "stderr", Message: compactCandyEvalText(line, 1200), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Prompt: prompt, StartedAt: startedAt})
	}
}

func (a *App) emitJuiceValueTestEvent(event juiceValueTestEvent) {
	if a == nil || a.ctx == nil {
		return
	}
	event.UpdatedAt = time.Now().UnixMilli()
	wruntime.EventsEmit(a.ctx, juiceValueTestEventName, event)
}
