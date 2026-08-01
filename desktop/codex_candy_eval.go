package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	candyEvalExecutorCodex         = "codex_cli"
	candyEvalExecutorClaude        = "claude_cli"
	candyEvalExecutorRaw           = "raw_request"
	candyEvalMaxLineBytes          = 4 << 20
	candyEvalDefaultTests          = 5
)

const candyEvalPrompt = `不使用任何外部工具回答以下问题：

在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）

        苹果味  桃子味  西瓜味
圆形       7      9      8
五角星形   7      6      4
`

const candyEvalExpectedAnswer = 21

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
	Executor              string  `json:"executor,omitempty"`
	ProviderID            string  `json:"providerId,omitempty"`
	ProviderName          string  `json:"providerName,omitempty"`
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

// candyEvalTarget is deliberately kept in memory only.  It makes the target
// selected in the key panel explicit for every test run, instead of silently
// inheriting the user's normal Codex login or the proxy's active queue.
type candyEvalTarget struct {
	SiteURL      string
	APIKey       string
	ProviderID   string
	ProviderName string
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

func normalizeCandyEvalExecutor(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case candyEvalExecutorClaude:
		return candyEvalExecutorClaude
	case candyEvalExecutorRaw:
		return candyEvalExecutorRaw
	default:
		return candyEvalExecutorCodex
	}
}

func candyEvalExecutorLabel(executor string) string {
	switch normalizeCandyEvalExecutor(executor) {
	case candyEvalExecutorClaude:
		return "Claude CLI"
	case candyEvalExecutorRaw:
		return "Raw Request"
	default:
		return "Codex CLI"
	}
}

func candyEvalModeLabel(mode string) string {
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		return "代理网关"
	}
	return "直连"
}

// Candy tests must use the same OpenAI URL convention as the quick test. A
// saved site root such as https://www.cun.ai represents /v1 by default, not
// /responses directly.
func normalizeCandyEvalOpenAIBaseURL(siteURL string) string {
	endpoint := firstNonEmpty(buildResponsesEndpointCandidates(siteURL)...)
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	endpoint = strings.TrimSuffix(endpoint, "/responses")
	return strings.TrimRight(endpoint, "/")
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

func resolveCandyClaudeExecutable() (string, error) {
	candidates := []string{"claude"}
	if runtime.GOOS == "windows" {
		candidates = []string{"claude.cmd", "claude.exe", "claude"}
	}
	for _, candidate := range candidates {
		if executable, err := exec.LookPath(candidate); err == nil && strings.TrimSpace(executable) != "" {
			return executable, nil
		}
	}
	return "", errors.New("找不到 Claude CLI，请先安装 Claude Code 并确保它已加入 PATH")
}

func buildCandyCodexCommand(ctx context.Context, executable, model, effort, mode, gatewayURL string) *exec.Cmd {
	return buildCandyCodexCommandForTarget(ctx, executable, model, effort, mode, gatewayURL, candyEvalTarget{}, "")
}

func buildCandyCodexCommandForTarget(ctx context.Context, executable, model, effort, mode, gatewayURL string, target candyEvalTarget, runID string) *exec.Cmd {
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
		if strings.TrimSpace(target.ProviderID) != "" {
			args = append(args,
				"-c", fmt.Sprintf(`model_providers.%s.http_headers."X-AllApiDeck-Provider-ID"=%q`, candyEvalGatewayProvider, strings.TrimSpace(target.ProviderID)),
			)
		}
		if strings.TrimSpace(runID) != "" {
			args = append(args,
				"-c", fmt.Sprintf(`model_providers.%s.http_headers."X-AllApiDeck-Candy-Eval-Run"=%q`, candyEvalGatewayProvider, strings.TrimSpace(runID)),
			)
		}
	} else if strings.TrimSpace(target.SiteURL) != "" {
		// Direct tests use the selected row's endpoint and key, never the
		// operator's existing ~/.codex provider/login settings.
		args = append(args,
			"-c", "model_provider=allapideck_candy_target",
			"-c", "model_providers.allapideck_candy_target.name=AllApiDeck Candy Target",
			"-c", fmt.Sprintf("model_providers.allapideck_candy_target.base_url=%q", normalizeCandyEvalOpenAIBaseURL(target.SiteURL)),
			"-c", "model_providers.allapideck_candy_target.wire_api=responses",
			"-c", "model_providers.allapideck_candy_target.requires_openai_auth=true",
		)
	}
	if normalizedModel := strings.TrimSpace(model); normalizedModel != "" {
		args = append(args, "-m", normalizedModel)
	}

	var command *exec.Cmd
	if runtime.GOOS == "windows" && (strings.HasSuffix(strings.ToLower(executable), ".cmd") || strings.HasSuffix(strings.ToLower(executable), ".bat")) {
		cmdArgs := append([]string{"/d", "/c", "call", executable}, args...)
		command = exec.CommandContext(ctx, "cmd.exe", cmdArgs...)
	} else {
		command = exec.CommandContext(ctx, executable, args...)
	}
	command.Stdin = strings.NewReader(candyEvalPrompt)
	command.Env = candyEvalCommandEnvironment(target)
	configureBackgroundCmd(command)
	return command
}

func candyEvalCommandEnvironment(target candyEvalTarget) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		upper := strings.ToUpper(item)
		if strings.HasPrefix(upper, "CODEX_HOME=") || strings.HasPrefix(upper, "CODEX_API_KEY=") {
			continue
		}
		env = append(env, item)
	}
	if apiKey := strings.TrimSpace(target.APIKey); apiKey != "" {
		env = append(env, "CODEX_API_KEY="+apiKey)
	}
	return env
}

func prepareCandyEvalCodexHome(target candyEvalTarget, mode, gatewayURL, runID, effort string) (string, func(), error) {
	home, err := os.MkdirTemp("", "allapideck-candy-codex-")
	if err != nil {
		return "", nil, err
	}
	// CODEX_HOME makes this run self-contained.  It prevents a test from
	// reading, modifying, or persisting credentials/configuration in ~/.codex.
	providerName := "allapideck_candy_target"
	baseURL := normalizeCandyEvalOpenAIBaseURL(target.SiteURL)
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		providerName = candyEvalGatewayProvider
		baseURL = strings.TrimRight(strings.TrimSpace(gatewayURL), "/")
	}
	config := strings.Builder{}
	fmt.Fprintf(&config, "# transient AllApiDeck candy evaluation configuration\nmodel_provider = %q\nmodel_reasoning_effort = %q\n\n", providerName, normalizeCandyEvalEffort(effort))
	fmt.Fprintf(&config, "[model_providers.%s]\nname = %q\nbase_url = %q\nwire_api = %q\nrequires_openai_auth = true\n\n", providerName, "AllApiDeck Candy Evaluation", baseURL, "responses")
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		fmt.Fprintf(&config, "[model_providers.%s.http_headers]\n", providerName)
		fmt.Fprintf(&config, "%q = %q\n", "X-AllApiDeck-Provider-ID", strings.TrimSpace(target.ProviderID))
		fmt.Fprintf(&config, "%q = %q\n", "X-AllApiDeck-Candy-Eval-Run", strings.TrimSpace(runID))
		fmt.Fprintf(&config, "%q = %q\n", "X-AllApiDeck-Candy-Target-Base-URL", normalizeCandyEvalOpenAIBaseURL(target.SiteURL))
		fmt.Fprintf(&config, "%q = %q\n", "X-AllApiDeck-Candy-Target-API-Key", strings.TrimSpace(target.APIKey))
		fmt.Fprintf(&config, "%q = %q\n", "X-AllApiDeck-Candy-Target-Name", strings.TrimSpace(target.ProviderName))
	}
	configPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(configPath, []byte(config.String()), 0o600); err != nil {
		_ = os.RemoveAll(home)
		return "", nil, err
	}
	return home, func() { _ = os.RemoveAll(home) }, nil
}

func (a *App) StartCandyIntelligenceTest(runID, model, effort, mode, siteURL, apiKey, providerID, providerName, executor string) error {
	if a == nil || a.ctx == nil {
		return errors.New("桌面运行时不可用，无法启动糖果智力测试")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return errors.New("糖果智力测试缺少运行标识")
	}
	executor = normalizeCandyEvalExecutor(executor)
	if executor == candyEvalExecutorCodex {
		if _, err := resolveCandyCodexExecutable(); err != nil {
			return err
		}
	} else if executor == candyEvalExecutorClaude {
		if _, err := resolveCandyClaudeExecutable(); err != nil {
			return err
		}
	}
	mode = normalizeCandyEvalMode(mode)
	target := candyEvalTarget{
		SiteURL:      strings.TrimSpace(siteURL),
		APIKey:       strings.TrimSpace(apiKey),
		ProviderID:   strings.TrimSpace(providerID),
		ProviderName: strings.TrimSpace(providerName),
	}
	if target.APIKey == "" {
		return errors.New("糖果测试缺少右键选中的 API Key")
	}
	if mode == candyEvalModeDirect && target.SiteURL == "" {
		return errors.New("直连糖果测试缺少右键选中的站点地址")
	}
	gatewayURL := ""
	if mode == candyEvalModeGateway {
		if err := a.ensureBridgeServer(); err != nil {
			return fmt.Errorf("启动代理网关失败：%w", err)
		}
		gatewayPath := advancedProxyCodexBasePath
		if executor == candyEvalExecutorClaude {
			gatewayPath = advancedProxyClaudeBasePath
		}
		gatewayURL = currentBridgeServerURLWithPath(gatewayPath)
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
	if executor == candyEvalExecutorClaude && mode == candyEvalModeGateway {
		if a.candyEvalGatewayTargets == nil {
			a.candyEvalGatewayTargets = make(map[string]candyEvalTarget)
		}
		a.candyEvalGatewayTargets[runID] = target
	}
	a.candyEvalMu.Unlock()

	model = strings.TrimSpace(model)
	effort = normalizeCandyEvalEffort(effort)
	startedAt := time.Now().UnixMilli()
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
		RunID:        runID,
		Stage:        candyEvalStageStarting,
		Kind:         "system",
		Message:      fmt.Sprintf("正在启动 %s（链路：%s，目标：%s，模型：%s，思考量：%s）", candyEvalExecutorLabel(executor), candyEvalModeLabel(mode), firstNonEmpty(target.ProviderName, target.ProviderID, "右键选中密钥"), firstNonEmpty(model, "默认模型"), effort),
		Model:        model,
		Effort:       effort,
		Mode:         mode,
		GatewayURL:   gatewayURL,
		ProviderID:   target.ProviderID,
		ProviderName: target.ProviderName,
		Executor:     executor,
		TotalRuns:    candyEvalDefaultTests,
		Tests:        candyEvalDefaultTests,
		StartedAt:    startedAt,
	})

	go func() {
		defer a.unregisterCandyIntelligenceTest(runID)
		a.runCandyIntelligenceTest(ctx, runID, model, effort, mode, gatewayURL, target, executor, startedAt)
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
	delete(a.candyEvalGatewayTargets, strings.TrimSpace(runID))
	a.candyEvalMu.Unlock()
}

// findCandyEvalGatewayTarget identifies an active Claude gateway run without
// requiring Claude Code to support custom request headers.  The bridge is
// loopback-only and callers must still present the selected key; the key is
// used only for an in-memory equality check and is never logged.
func (a *App) findCandyEvalGatewayTarget(request *http.Request) (string, candyEvalTarget, bool) {
	if a == nil || request == nil {
		return "", candyEvalTarget{}, false
	}
	candidates := []string{strings.TrimSpace(request.Header.Get("x-api-key"))}
	if authorization := strings.TrimSpace(request.Header.Get("Authorization")); authorization != "" {
		candidates = append(candidates, strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")))
	}
	a.candyEvalMu.Lock()
	defer a.candyEvalMu.Unlock()
	for runID, target := range a.candyEvalGatewayTargets {
		for _, candidate := range candidates {
			if candidate != "" && target.APIKey != "" && candidate == target.APIKey {
				return runID, target, true
			}
		}
	}
	return "", candyEvalTarget{}, false
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

func (a *App) runCandyIntelligenceTest(ctx context.Context, runID, model, effort, mode, gatewayURL string, target candyEvalTarget, executor string, startedAt int64) {
	switch normalizeCandyEvalExecutor(executor) {
	case candyEvalExecutorRaw:
		a.runRawCandyIntelligenceTest(ctx, runID, model, effort, mode, gatewayURL, target, startedAt)
		return
	case candyEvalExecutorClaude:
		a.runClaudeCandyIntelligenceTest(ctx, runID, model, effort, mode, gatewayURL, target, startedAt)
		return
	}
	executable, err := resolveCandyCodexExecutable()
	if err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
			RunID: runID, Stage: candyEvalStageError, Kind: "error", Message: err.Error(), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, StartedAt: startedAt,
		})
		return
	}
	codexHome, cleanupCodexHome, err := prepareCandyEvalCodexHome(target, mode, gatewayURL, runID, effort)
	if err != nil {
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageError, Kind: "error", Message: fmt.Sprintf("创建临时 Codex 配置失败：%v", err), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, StartedAt: startedAt})
		return
	}
	defer cleanupCodexHome()
	a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "target", Message: fmt.Sprintf("已锁定右键选中密钥：%s（%s）；使用独立临时 Codex 配置，结束后自动清理", firstNonEmpty(target.ProviderName, target.ProviderID, "选中密钥"), candyEvalModeLabel(mode)), Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, ProviderID: target.ProviderID, ProviderName: target.ProviderName, TotalRuns: candyEvalDefaultTests, Tests: candyEvalDefaultTests, StartedAt: startedAt})

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

		result := a.runCandyEvalOne(ctx, executable, runID, runIndex, totalRuns, model, effort, mode, gatewayURL, target, codexHome, startedAt)
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

func (a *App) runCandyEvalOne(ctx context.Context, executable, runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, target candyEvalTarget, codexHome string, startedAt int64) candyEvalRunResult {
	started := time.Now()
	result := candyEvalRunResult{}
	command := buildCandyCodexCommandForTarget(ctx, executable, model, effort, mode, gatewayURL, target, runID)
	command.Env = append(command.Env, "CODEX_HOME="+codexHome)
	stopLiveProxyStatus := func() {}
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		stopLiveProxyStatus = a.startCandyEvalLiveProxyStatus(ctx, runID, runIndex, totalRuns, model, effort, mode, gatewayURL, startedAt)
	}
	defer stopLiveProxyStatus()
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
	if normalizeCandyEvalMode(mode) == candyEvalModeGateway {
		a.emitCandyEvalProxyTrace(runID, runIndex, totalRuns, model, effort, mode, gatewayURL, startedAt)
	}

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

func (a *App) emitCandyEvalProxyTrace(runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, startedAt int64) {
	raw, err := os.ReadFile(resolveAdvancedProxyLogPath())
	if err != nil {
		return
	}
	needle := "run=" + strings.TrimSpace(runID)
	if needle == "run=" {
		return
	}
	lines := strings.Split(string(raw), "\n")
	matched := make([]string, 0, 4)
	for index := len(lines) - 1; index >= 0 && len(matched) < 6; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" || !strings.Contains(line, needle) {
			continue
		}
		if strings.Contains(line, "[CANDY_EVAL_TARGET_LOCK]") || strings.Contains(line, "[CANDY_EVAL_PROTOCOL_LOCK]") || strings.Contains(line, "[CANDY_EVAL_EFFORT]") || strings.Contains(line, "[ANTI_CANDY_") {
			matched = append(matched, line)
		}
	}
	for index := len(matched) - 1; index >= 0; index-- {
		line := matched[index]
		message := compactCandyEvalText(line, 1000)
		if strings.Contains(line, "eligible=false") && strings.Contains(line, "model_not_allowed") {
			message = "反降智未执行：当前模型不在反降智白名单中（model_not_allowed）。"
		} else if strings.Contains(line, "[ANTI_CANDY_FOLD]") {
			message = "反降智已触发并完成续写：" + message
		} else if strings.Contains(line, "[ANTI_CANDY_RESULT]") {
			message = "反降智已检查，但本轮未命中截断条件：" + message
		} else if strings.Contains(line, "[CANDY_EVAL_EFFORT]") {
			message = "糖果诊断已确认实际出站推理强度：" + message
		} else if strings.Contains(line, "[CANDY_EVAL_TARGET_LOCK]") {
			message = "高级代理已锁定右键选中的 Provider：" + message
		} else if strings.Contains(line, "[CANDY_EVAL_PROTOCOL_LOCK]") {
			message = "糖果诊断已锁定 Responses 协议并延长首次响应等待：" + message
		}
		a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{RunID: runID, Stage: candyEvalStageStreaming, Kind: "anti-candy", Message: message, Model: model, Effort: effort, Mode: mode, GatewayURL: gatewayURL, Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt})
	}
}

// startCandyEvalLiveProxyStatus keeps one mutable tail line in the candy panel
// while Codex is waiting on the gateway. codexcomp exposes this as round logs;
// our desktop runner mirrors it so a long upstream wait is observable instead
// of looking like a stalled CLI process.
func (a *App) startCandyEvalLiveProxyStatus(ctx context.Context, runID string, runIndex, totalRuns int, model, effort, mode, gatewayURL string, startedAt int64) func() {
	stop := make(chan struct{})
	runStartedAt := time.Now()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		currentStep := "等待 Codex CLI 发起 Responses 请求"
		stepStartedAt := runStartedAt
		lastLine := ""
		emit := func() {
			a.emitCandyIntelligenceTestEvent(candyIntelligenceTestEvent{
				RunID: runID, Stage: candyEvalStageStreaming, Kind: "anti-candy-live",
				Message: fmt.Sprintf("反降智实时步骤：%s（本步骤 %.1fs，累计 %.1fs）", currentStep, time.Since(stepStartedAt).Seconds(), time.Since(runStartedAt).Seconds()),
				Model:   model, Effort: effort, Mode: mode, GatewayURL: gatewayURL,
				Run: runIndex, TotalRuns: totalRuns, Tests: totalRuns, StartedAt: startedAt,
			})
		}
		emit()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				if line := latestCandyEvalProxyStep(runID); line != "" && line != lastLine {
					lastLine = line
					currentStep = describeCandyEvalProxyStep(line)
					stepStartedAt = time.Now()
				}
				emit()
			}
		}
	}()
	return func() { close(stop) }
}

func latestCandyEvalProxyStep(runID string) string {
	raw, err := os.ReadFile(resolveAdvancedProxyLogPath())
	if err != nil {
		return ""
	}
	needle := "run=" + strings.TrimSpace(runID)
	if needle == "run=" {
		return ""
	}
	lines := strings.Split(string(raw), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if strings.Contains(line, needle) && (strings.Contains(line, "[ANTI_CANDY_STEP]") || strings.Contains(line, "[ANTI_CANDY_") || strings.Contains(line, "[CANDY_EVAL_")) {
			return line
		}
	}
	return ""
}

func describeCandyEvalProxyStep(line string) string {
	switch {
	case strings.Contains(line, "[CANDY_EVAL_EFFORT]"):
		return "已确认实际出站推理强度：" + compactCandyEvalText(line, 240)
	case strings.Contains(line, "[CANDY_EVAL_TARGET_LOCK]"):
		return "已锁定右键选中的 Provider，准备建立上游请求"
	case strings.Contains(line, "[CANDY_EVAL_PROTOCOL_LOCK]"):
		return "已锁定 Responses 协议，等待首轮响应终止事件"
	case strings.Contains(line, "stage=round_wait"):
		return "第 1 轮：正在接收上游 SSE，等待 response.completed / response.incomplete"
	case strings.Contains(line, "stage=round_received"):
		return "第 1 轮：已收到完整 SSE，正在解析 518n-2 截断指纹"
	case strings.Contains(line, "stage=continuation_request"):
		return "已命中截断：已重放 encrypted reasoning，正在等待续写轮终止事件"
	case strings.Contains(line, "stage=continuation_received"):
		return "续写轮已返回，正在解析 reasoning token 并决定继续或收尾"
	case strings.Contains(line, "[ANTI_CANDY_FOLD]"):
		return "续写与折叠已完成，正在等待 Codex CLI 解析最终回答"
	case strings.Contains(line, "[ANTI_CANDY_RESULT]"):
		return "已检查反降智条件，本轮未命中截断"
	case strings.Contains(line, "[ANTI_CANDY_CHECK]"):
		return "已收到首轮响应，正在检查模型、协议与截断资格"
	default:
		return compactCandyEvalText(line, 360)
	}
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
