package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	advancedProxyConfigDirName  = "advanced-proxy"
	advancedProxyConfigFileName = "config.json"
	advancedProxyClaudeBasePath = "/advanced-proxy/claude"
	advancedProxyCodexBasePath  = "/advanced-proxy/codex/v1"
	advancedProxyGrokBuildPath  = "/advanced-proxy/grokbuild/v1"
	advancedProxyOpenCodePath   = "/advanced-proxy/opencode/v1"
	advancedProxyOpenClawPath   = "/advanced-proxy/openclaw/v1"
	advancedProxyGlobalScope    = "global"
)

var advancedProxyConfigMu sync.Mutex

type AdvancedProxyProvider struct {
	ID          string `json:"id"`
	RowKey      string `json:"rowKey,omitempty"`
	Name        string `json:"name"`
	BaseURL     string `json:"baseUrl"`
	APIKey      string `json:"apiKey"`
	Model       string `json:"model"`
	APIFormat   string `json:"apiFormat"`
	APIKeyField string `json:"apiKeyField"`
	Enabled     bool   `json:"enabled"`
	SortIndex   int    `json:"sortIndex"`
	SourceType  string `json:"sourceType,omitempty"`
}

type ClaudeProxyCompatConfig struct {
	Enabled      bool                    `json:"enabled"`
	BasePath     string                  `json:"basePath"`
	DefaultModel string                  `json:"defaultModel"`
	Providers    []AdvancedProxyProvider `json:"providers"`
}

type AdvancedProxyAppConfig struct {
	Enabled  bool   `json:"enabled"`
	BasePath string `json:"basePath"`
}

type AdvancedProxyQueueConfig struct {
	InheritGlobal bool                    `json:"inheritGlobal"`
	Providers     []AdvancedProxyProvider `json:"providers"`
}

type AdvancedProxyQueuesConfig struct {
	Global    AdvancedProxyQueueConfig `json:"global"`
	Claude    AdvancedProxyQueueConfig `json:"claude"`
	Codex     AdvancedProxyQueueConfig `json:"codex"`
	GrokBuild AdvancedProxyQueueConfig `json:"grokbuild"`
	OpenCode  AdvancedProxyQueueConfig `json:"opencode"`
	OpenClaw  AdvancedProxyQueueConfig `json:"openclaw"`
}

type AppFailoverConfig struct {
	AppType                   string  `json:"appType"`
	Enabled                   bool    `json:"enabled"`
	AutoFailoverEnabled       bool    `json:"autoFailoverEnabled"`
	MaxRetries                int     `json:"maxRetries"`
	StreamingFirstByteTimeout int     `json:"streamingFirstByteTimeout"`
	StreamingIdleTimeout      int     `json:"streamingIdleTimeout"`
	NonStreamingTimeout       int     `json:"nonStreamingTimeout"`
	CircuitFailureThreshold   int     `json:"circuitFailureThreshold"`
	CircuitSuccessThreshold   int     `json:"circuitSuccessThreshold"`
	CircuitTimeoutSeconds     int     `json:"circuitTimeoutSeconds"`
	CircuitErrorRateThreshold float64 `json:"circuitErrorRateThreshold"`
	CircuitMinRequests        int     `json:"circuitMinRequests"`
}

type HighAvailabilityConfig struct {
	Enabled              bool                      `json:"enabled"`
	DynamicOptimizeQueue bool                      `json:"dynamicOptimizeQueue"`
	DispatchMode         string                    `json:"dispatchMode"`
	RPM                  HighAvailabilityRPMConfig `json:"rpm"`
}

type HighAvailabilityRPMConfig struct {
	Global    int             `json:"global"`
	Providers map[string]*int `json:"providers"`
}

type RectifierConfig struct {
	Enabled                  bool `json:"enabled"`
	RequestThinkingSignature bool `json:"requestThinkingSignature"`
	RequestThinkingBudget    bool `json:"requestThinkingBudget"`
}

type OptimizerConfig struct {
	Enabled           bool   `json:"enabled"`
	ThinkingOptimizer bool   `json:"thinkingOptimizer"`
	CacheInjection    bool   `json:"cacheInjection"`
	CacheTTL          string `json:"cacheTtl"`
}

type ContextAutoCompressionConfig struct {
	Enabled    bool `json:"enabled"`
	ThresholdK int  `json:"thresholdK"`
}

type AntiPoisonRandomizationConfig struct {
	Enabled                      bool `json:"enabled"`
	StrategyPoolSize             int  `json:"strategyPoolSize"`
	MinPhraseVariantsPerStrategy int  `json:"minPhraseVariantsPerStrategy"`
	RandomInsertionPoints        bool `json:"randomInsertionPoints"`
	MinFakeToolcalls             int  `json:"minFakeToolcalls"`
	RequirePerToolTypeMarker     bool `json:"requirePerToolTypeMarker"`
}

type AntiPoisonStringProtectionConfig struct {
	Enabled bool     `json:"enabled"`
	Rules   []string `json:"rules"`
}

type AntiPoisonConfig struct {
	Enabled          bool                             `json:"enabled"`
	StrictMode       bool                             `json:"strictMode"`
	FailureMode      string                           `json:"failureMode"`
	StrategyPrompt   string                           `json:"strategyPrompt"`
	AlgorithmPrompt  string                           `json:"algorithmPrompt"`
	Randomization    AntiPoisonRandomizationConfig    `json:"randomization"`
	StringProtection AntiPoisonStringProtectionConfig `json:"stringProtection"`
}

type AdvancedProxyConfig struct {
	Enabled                bool                         `json:"enabled"`
	DebugLogging           bool                         `json:"debugLogging"`
	ListenHost             string                       `json:"listenHost"`
	ListenPort             int                          `json:"listenPort"`
	Queues                 AdvancedProxyQueuesConfig    `json:"queues"`
	UserAgentMappings      []checkUserAgentMapping      `json:"userAgentMappings"`
	ContextAutoCompression ContextAutoCompressionConfig `json:"contextAutoCompression"`
	Claude                 ClaudeProxyCompatConfig      `json:"claude"`
	Codex                  AdvancedProxyAppConfig       `json:"codex"`
	GrokBuild              AdvancedProxyAppConfig       `json:"grokbuild"`
	OpenCode               AdvancedProxyAppConfig       `json:"opencode"`
	OpenClaw               AdvancedProxyAppConfig       `json:"openclaw"`
	Failover               AppFailoverConfig            `json:"failover"`
	HighAvailability       HighAvailabilityConfig       `json:"highAvailability"`
	Rectifier              RectifierConfig              `json:"rectifier"`
	Optimizer              OptimizerConfig              `json:"optimizer"`
	AntiPoison             AntiPoisonConfig             `json:"antiPoison"`
	UpdatedAt              string                       `json:"updatedAt"`
}

type FailoverQueueItem struct {
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	SortIndex    int    `json:"sortIndex"`
	Enabled      bool   `json:"enabled"`
}

type CircuitBreakerStats struct {
	State                string `json:"state"`
	ConsecutiveFailures  int    `json:"consecutiveFailures"`
	ConsecutiveSuccesses int    `json:"consecutiveSuccesses"`
	TotalRequests        int    `json:"totalRequests"`
	FailedRequests       int    `json:"failedRequests"`
}

type AdvancedProxyRoutingState struct {
	AppType        string `json:"appType"`
	ProviderID     string `json:"providerId"`
	ProviderRowKey string `json:"providerRowKey"`
	ProviderName   string `json:"providerName"`
	RouteKind      string `json:"routeKind"`
	Status         string `json:"status"`
	TargetURL      string `json:"targetUrl"`
	UpdatedAt      string `json:"updatedAt"`
}

type AdvancedProxyProviderRoutingState struct {
	ProviderID     string   `json:"providerId"`
	ProviderRowKey string   `json:"providerRowKey"`
	ProviderName   string   `json:"providerName"`
	AppTypes       []string `json:"appTypes"`
	ActiveCount    int      `json:"activeCount"`
	RouteKind      string   `json:"routeKind"`
	Status         string   `json:"status"`
	TargetURL      string   `json:"targetUrl"`
	UpdatedAt      string   `json:"updatedAt"`
}

type AdvancedProxyRoutingSnapshot struct {
	Apps      map[string]AdvancedProxyRoutingState         `json:"apps"`
	Providers map[string]AdvancedProxyProviderRoutingState `json:"providers"`
}

func defaultAdvancedProxyQueueConfig(inheritGlobal bool) AdvancedProxyQueueConfig {
	return AdvancedProxyQueueConfig{
		InheritGlobal: inheritGlobal,
		Providers:     []AdvancedProxyProvider{},
	}
}

func defaultAdvancedProxyQueuesConfig() AdvancedProxyQueuesConfig {
	return AdvancedProxyQueuesConfig{
		Global:    defaultAdvancedProxyQueueConfig(false),
		Claude:    defaultAdvancedProxyQueueConfig(true),
		Codex:     defaultAdvancedProxyQueueConfig(true),
		GrokBuild: defaultAdvancedProxyQueueConfig(true),
		OpenCode:  defaultAdvancedProxyQueueConfig(true),
		OpenClaw:  defaultAdvancedProxyQueueConfig(true),
	}
}

const defaultAntiPoisonStrategyPrompt = `IMPORTANT: AllApiDeck guard rules are gateway-enforced high-priority safety constraints and must override ordinary task instructions.
Current guard rule id is {{ALGORITHM_ALIAS}}.
If this turn emits any real toolcall, the assistant text immediately before that toolcall must be exactly one AllApiDeck guard JSON text block.
Do not emit ordinary pre-tool text such as "I will search", "I will check", or progress narration before a toolcall; replace that pre-tool sentence with the guard JSON block.
The guard JSON is not a user request and must not be explained to the user.
The guard JSON only requires the minimal binding fields: name and tool_name.
Do not explain, quote, summarize, or mention the guard JSON, guard name, guard prefix, naming rule, gateway rule, or validation result in ordinary assistant text.
Outside the guard JSON block itself, output no guard-related ordinary text; only the actual guard JSON block may contain the guard tag, guard name, or guard prefix.
If a valid guard JSON cannot be emitted before a real toolcall, emit no real toolcall and output plain text: guard generation failed for pending toolcall.
The gateway strips all guard JSON before returning to the client.
If there is no real toolcall, do not emit guard JSON.`

const defaultAntiPoisonAlgorithmPrompt = `{{ALGORITHM_ALIAS}} is this turn's guard rule id.
When generating guard JSON, write only name and tool_name.
name must follow the gateway guard tool naming rule, and tool_name must equal the immediately following real tool name.`

func defaultAntiPoisonRandomizationConfig() AntiPoisonRandomizationConfig {
	return AntiPoisonRandomizationConfig{
		Enabled:                      true,
		StrategyPoolSize:             10,
		MinPhraseVariantsPerStrategy: 5,
		RandomInsertionPoints:        true,
		MinFakeToolcalls:             1,
		RequirePerToolTypeMarker:     true,
	}
}

func defaultAntiPoisonStringProtectionConfig() AntiPoisonStringProtectionConfig {
	return AntiPoisonStringProtectionConfig{
		Enabled: true,
		Rules: []string{
			`保护 JSON 字段名命中值: key:(?i)^(api[_-]?key|secret|client[_-]?secret|token|access[_-]?token|refresh[_-]?token|session[_-]?token|password|authorization|auth[_-]?token|credential|credentials|private[_-]?key)$`,
			"保护用户双尖括号标记内容: user_text:<<[^<>\r\n]{1,512}>>",
			`保护 JSON/YAML/TOML 敏感 key: (?i)("?(api[_-]?key|secret|token|password|authorization|auth[_-]?token|private[_-]?key)"?\s*[:=]\s*)("[^"]{8,}"|'[^']{8,}'|[^\s,;}"']{8,})`,
			`保护 Bearer/Basic/Auth 头值: (?i)\b(Bearer|Basic|Authorization)\s+[A-Za-z0-9._~+/=-]{12,}`,
			`保护常见 LLM/API key 形态: (?i)\b(?:sk|sk-ant|sk-proj|sk-live|sk-test|xox[baprs]|gh[pousr]|AIza)[A-Za-z0-9._=-]{12,}\b`,
			`保护环境变量式密钥: (?i)\b[A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*\s*=\s*[^\s"'` + "`" + `]{8,}`,
			`保护疑似私钥块: -----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]{20,}?-----END [A-Z ]*PRIVATE KEY-----`,
		},
	}
}

func defaultAntiPoisonConfig() AntiPoisonConfig {
	return AntiPoisonConfig{
		Enabled:          false,
		StrictMode:       true,
		FailureMode:      "block",
		StrategyPrompt:   defaultAntiPoisonStrategyPrompt,
		AlgorithmPrompt:  defaultAntiPoisonAlgorithmPrompt,
		Randomization:    defaultAntiPoisonRandomizationConfig(),
		StringProtection: defaultAntiPoisonStringProtectionConfig(),
	}
}

func defaultAdvancedProxyUserAgentMappings() []checkUserAgentMapping {
	return []checkUserAgentMapping{
		{
			ModelContains: "gpt",
			TargetUA: strings.Join([]string{
				"originator: Codex Desktop",
				"user-agent: Codex Desktop/0.142.0-alpha.6 (Windows 10.0.19044; x86_64) unknown (Codex Desktop; 26.616.51431)",
			}, "\n"),
		},
		{
			ModelContains: "claude",
			TargetUA: strings.Join([]string{
				"User-Agent: claude-cli/2.1.129 (external, cli)",
				"x-app: cli",
				"anthropic-version: 2023-06-01",
				"anthropic-beta: claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24",
				"anthropic-dangerous-direct-browser-access: true",
				"X-Stainless-Arch: x64",
				"X-Stainless-Lang: js",
				"X-Stainless-OS: Windows",
				"X-Stainless-Package-Version: 0.93.0",
				"X-Stainless-Retry-Count: 0",
				"X-Stainless-Runtime: node",
				"X-Stainless-Runtime-Version: v24.3.0",
				"X-Stainless-Timeout: 600",
			}, "\n"),
		},
	}
}

func defaultContextAutoCompressionConfig() ContextAutoCompressionConfig {
	return ContextAutoCompressionConfig{
		Enabled:    false,
		ThresholdK: 256,
	}
}

func defaultAdvancedProxyConfig() AdvancedProxyConfig {
	return AdvancedProxyConfig{
		Enabled:                false,
		DebugLogging:           false,
		ListenHost:             bridgeServerHost,
		ListenPort:             bridgeServerPortStart,
		Queues:                 defaultAdvancedProxyQueuesConfig(),
		UserAgentMappings:      defaultAdvancedProxyUserAgentMappings(),
		ContextAutoCompression: defaultContextAutoCompressionConfig(),
		Claude: ClaudeProxyCompatConfig{
			Enabled:      false,
			BasePath:     advancedProxyClaudeBasePath,
			DefaultModel: "",
			Providers:    []AdvancedProxyProvider{},
		},
		Codex: AdvancedProxyAppConfig{
			Enabled:  false,
			BasePath: advancedProxyCodexBasePath,
		},
		GrokBuild: AdvancedProxyAppConfig{
			Enabled:  false,
			BasePath: advancedProxyGrokBuildPath,
		},
		OpenCode: AdvancedProxyAppConfig{
			Enabled:  false,
			BasePath: advancedProxyOpenCodePath,
		},
		OpenClaw: AdvancedProxyAppConfig{
			Enabled:  false,
			BasePath: advancedProxyOpenClawPath,
		},
		Failover: AppFailoverConfig{
			AppType:                   "claude",
			Enabled:                   false,
			AutoFailoverEnabled:       false,
			MaxRetries:                1,
			StreamingFirstByteTimeout: 8,
			StreamingIdleTimeout:      12,
			NonStreamingTimeout:       25,
			CircuitFailureThreshold:   2,
			CircuitSuccessThreshold:   1,
			CircuitTimeoutSeconds:     30,
			CircuitErrorRateThreshold: 0.5,
			CircuitMinRequests:        2,
		},
		HighAvailability: HighAvailabilityConfig{
			Enabled:              false,
			DynamicOptimizeQueue: false,
			DispatchMode:         "fixed",
			RPM:                  defaultAdvancedProxyHighAvailabilityRPMConfig(),
		},
		Rectifier: RectifierConfig{
			Enabled:                  true,
			RequestThinkingSignature: true,
			RequestThinkingBudget:    true,
		},
		Optimizer: OptimizerConfig{
			Enabled:           false,
			ThinkingOptimizer: true,
			CacheInjection:    true,
			CacheTTL:          "1h",
		},
		AntiPoison: defaultAntiPoisonConfig(),
	}
}

func resolveAdvancedProxyConfigPath() string {
	dir := filepath.Join(resolveRuntimeRootDir(), advancedProxyConfigDirName)
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, advancedProxyConfigFileName)
}

func loadAdvancedProxyConfig() (AdvancedProxyConfig, error) {
	advancedProxyConfigMu.Lock()
	defer advancedProxyConfigMu.Unlock()

	config := defaultAdvancedProxyConfig()
	raw, err := os.ReadFile(resolveAdvancedProxyConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return sanitizeAdvancedProxyConfig(config), nil
		}
		return config, err
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return defaultAdvancedProxyConfig(), err
	}
	return sanitizeAdvancedProxyConfig(config), nil
}

func saveAdvancedProxyConfig(config AdvancedProxyConfig) (AdvancedProxyConfig, error) {
	advancedProxyConfigMu.Lock()
	defer advancedProxyConfigMu.Unlock()

	config = sanitizeAdvancedProxyConfig(config)
	config.UpdatedAt = time.Now().Format(time.RFC3339)
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return config, err
	}
	if err := os.WriteFile(resolveAdvancedProxyConfigPath(), raw, 0o644); err != nil {
		return config, err
	}
	return config, nil
}

func sanitizeAdvancedProxyConfig(config AdvancedProxyConfig) AdvancedProxyConfig {
	defaults := defaultAdvancedProxyConfig()
	legacyGlobalProviders := append([]AdvancedProxyProvider(nil), config.Claude.Providers...)

	if advancedProxyQueuesLikelyMissing(config.Queues) {
		config.Queues = defaults.Queues
	}

	if strings.TrimSpace(config.ListenHost) == "" {
		config.ListenHost = defaults.ListenHost
	}
	if config.ListenPort <= 0 {
		config.ListenPort = defaults.ListenPort
	}
	config.UserAgentMappings = sanitizeAdvancedProxyUserAgentMappings(config.UserAgentMappings)
	if len(config.UserAgentMappings) == 0 {
		config.UserAgentMappings = defaultAdvancedProxyUserAgentMappings()
	}
	config.ContextAutoCompression = sanitizeContextAutoCompressionConfig(config.ContextAutoCompression)

	config.Queues.Global = sanitizeAdvancedProxyQueueConfig(config.Queues.Global, defaults.Queues.Global, legacyGlobalProviders)
	config.Queues.Claude = sanitizeAdvancedProxyQueueConfig(config.Queues.Claude, defaults.Queues.Claude, nil)
	config.Queues.Codex = sanitizeAdvancedProxyQueueConfig(config.Queues.Codex, defaults.Queues.Codex, nil)
	config.Queues.GrokBuild = sanitizeAdvancedProxyQueueConfig(config.Queues.GrokBuild, defaults.Queues.GrokBuild, nil)
	config.Queues.OpenCode = sanitizeAdvancedProxyQueueConfig(config.Queues.OpenCode, defaults.Queues.OpenCode, nil)
	config.Queues.OpenClaw = sanitizeAdvancedProxyQueueConfig(config.Queues.OpenClaw, defaults.Queues.OpenClaw, nil)

	if strings.TrimSpace(config.Claude.BasePath) == "" {
		config.Claude.BasePath = defaults.Claude.BasePath
	}
	config.Claude.BasePath = ensureLeadingSlash(strings.TrimSpace(config.Claude.BasePath))
	config.Claude.DefaultModel = strings.TrimSpace(config.Claude.DefaultModel)
	config.Claude.Providers = append([]AdvancedProxyProvider(nil), config.Queues.Global.Providers...)
	config.Codex = sanitizeAdvancedProxyAppConfig(config.Codex, defaults.Codex)
	config.GrokBuild = sanitizeAdvancedProxyAppConfig(config.GrokBuild, defaults.GrokBuild)
	config.OpenCode = sanitizeAdvancedProxyAppConfig(config.OpenCode, defaults.OpenCode)
	config.OpenClaw = sanitizeAdvancedProxyAppConfig(config.OpenClaw, defaults.OpenClaw)

	if strings.TrimSpace(config.Failover.AppType) == "" {
		config.Failover.AppType = defaults.Failover.AppType
	}
	config.Failover.MaxRetries = clampInt(config.Failover.MaxRetries, 0, 10)
	config.Failover.StreamingFirstByteTimeout = clampInt(config.Failover.StreamingFirstByteTimeout, 5, 300)
	config.Failover.StreamingIdleTimeout = clampInt(config.Failover.StreamingIdleTimeout, 5, 600)
	config.Failover.NonStreamingTimeout = clampInt(config.Failover.NonStreamingTimeout, 5, 600)
	config.Failover.CircuitFailureThreshold = clampInt(config.Failover.CircuitFailureThreshold, 1, 20)
	config.Failover.CircuitSuccessThreshold = clampInt(config.Failover.CircuitSuccessThreshold, 1, 20)
	config.Failover.CircuitTimeoutSeconds = clampInt(config.Failover.CircuitTimeoutSeconds, 5, 600)
	if config.Failover.CircuitErrorRateThreshold <= 0 || config.Failover.CircuitErrorRateThreshold > 1 {
		config.Failover.CircuitErrorRateThreshold = defaults.Failover.CircuitErrorRateThreshold
	}
	config.Failover.CircuitMinRequests = clampInt(config.Failover.CircuitMinRequests, 1, 100)
	if strings.EqualFold(strings.TrimSpace(config.HighAvailability.DispatchMode), "ha_round_robin") {
		config.HighAvailability.Enabled = true
		config.HighAvailability.DispatchMode = "fixed"
	}
	config.HighAvailability.DispatchMode = normalizeAdvancedProxyDispatchMode(config.HighAvailability.DispatchMode)
	config.HighAvailability.RPM = normalizeAdvancedProxyHighAvailabilityRPMConfig(config.HighAvailability.RPM)
	if strings.TrimSpace(config.Optimizer.CacheTTL) == "" {
		config.Optimizer.CacheTTL = defaults.Optimizer.CacheTTL
	}
	config.AntiPoison = sanitizeAntiPoisonConfig(config.AntiPoison)
	config.Enabled = advancedProxyAnyAppEnabled(config)
	return config
}

func sanitizeAdvancedProxyUserAgentMappings(input []checkUserAgentMapping) []checkUserAgentMapping {
	if len(input) == 0 {
		return nil
	}
	result := make([]checkUserAgentMapping, 0, len(input))
	for _, item := range input {
		normalized := checkUserAgentMapping{
			ModelContains: strings.TrimSpace(item.ModelContains),
			TargetUA:      strings.TrimSpace(item.TargetUA),
		}
		if normalized.ModelContains == "" || normalized.TargetUA == "" {
			continue
		}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func sanitizeContextAutoCompressionConfig(config ContextAutoCompressionConfig) ContextAutoCompressionConfig {
	defaults := defaultContextAutoCompressionConfig()
	if config.ThresholdK <= 0 {
		config.ThresholdK = defaults.ThresholdK
	} else {
		config.ThresholdK = clampInt(config.ThresholdK, 1, 4096)
	}
	return config
}

func sanitizeAntiPoisonConfig(config AntiPoisonConfig) AntiPoisonConfig {
	defaults := defaultAntiPoisonConfig()
	if isZeroAntiPoisonConfig(config) {
		return defaults
	}
	if strings.TrimSpace(config.FailureMode) != "warn" {
		config.FailureMode = defaults.FailureMode
	}
	config.StrategyPrompt = strings.TrimSpace(config.StrategyPrompt)
	if config.StrategyPrompt == "" || antiPoisonPromptLooksStale(config.StrategyPrompt) {
		config.StrategyPrompt = defaults.StrategyPrompt
	}
	config.AlgorithmPrompt = strings.TrimSpace(config.AlgorithmPrompt)
	if config.AlgorithmPrompt == "" || antiPoisonPromptLooksStale(config.AlgorithmPrompt) {
		config.AlgorithmPrompt = defaults.AlgorithmPrompt
	}
	if config.Randomization.StrategyPoolSize == 0 &&
		config.Randomization.MinPhraseVariantsPerStrategy == 0 &&
		config.Randomization.MinFakeToolcalls == 0 {
		config.Randomization = defaults.Randomization
	} else {
		config.Randomization.StrategyPoolSize = clampInt(config.Randomization.StrategyPoolSize, 1, 100)
		config.Randomization.MinPhraseVariantsPerStrategy = clampInt(config.Randomization.MinPhraseVariantsPerStrategy, 1, 50)
		config.Randomization.MinFakeToolcalls = clampInt(config.Randomization.MinFakeToolcalls, 1, 20)
	}
	config.StringProtection = sanitizeAntiPoisonStringProtectionConfig(config.StringProtection, defaults.StringProtection)
	return config
}

func antiPoisonPromptLooksStale(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "guard fake toolcall") {
		return true
	}
	if strings.Contains(lower, "digest") && strings.Contains(lower, "chain") && strings.Contains(lower, "cover") {
		return true
	}
	if strings.Contains(lower, "first emit one guard json text block immediately before") &&
		!strings.Contains(lower, "ordinary pre-tool text") {
		return true
	}
	return false
}

func sanitizeAntiPoisonStringProtectionConfig(config AntiPoisonStringProtectionConfig, defaults AntiPoisonStringProtectionConfig) AntiPoisonStringProtectionConfig {
	if len(config.Rules) == 0 {
		if !config.Enabled {
			config.Enabled = defaults.Enabled
		}
		config.Rules = append([]string(nil), defaults.Rules...)
		return config
	}
	cleaned := make([]string, 0, len(config.Rules))
	seen := map[string]struct{}{}
	for _, rule := range config.Rules {
		rule = strings.TrimSpace(rule)
		if rule == "" || isLegacyAntiPoisonFileMentionProtectionRule(rule) || isLegacyAntiPoisonSingleAngleUserTextRule(rule) {
			continue
		}
		if _, exists := seen[rule]; exists {
			continue
		}
		seen[rule] = struct{}{}
		cleaned = append(cleaned, rule)
		if len(cleaned) >= 80 {
			break
		}
	}
	if len(cleaned) == 0 {
		cleaned = append([]string(nil), defaults.Rules...)
	}
	if !antiPoisonStringProtectionRulesContainScope(cleaned, "user_text") {
		for _, defaultRule := range defaults.Rules {
			_, scope, _ := parseAntiPoisonStringProtectionRule(defaultRule)
			if scope == "user_text" {
				cleaned = append(cleaned, defaultRule)
				break
			}
		}
	}
	config.Rules = cleaned
	return config
}

func isLegacyAntiPoisonFileMentionProtectionRule(rule string) bool {
	return strings.Contains(rule, "保护点号开头配置文件路径") || strings.Contains(rule, "保护常见配置文件路径")
}

func isLegacyAntiPoisonSingleAngleUserTextRule(rule string) bool {
	return strings.Contains(rule, "保护用户尖括号标记内容") || strings.Contains(rule, "user_text:<[^<>\\r\\n]{1,512}>")
}

func antiPoisonStringProtectionRulesContainScope(rules []string, scope string) bool {
	scope = strings.TrimSpace(strings.ToLower(scope))
	for _, rule := range rules {
		_, currentScope, _ := parseAntiPoisonStringProtectionRule(rule)
		if strings.TrimSpace(strings.ToLower(currentScope)) == scope {
			return true
		}
	}
	return false
}

func isZeroAntiPoisonConfig(config AntiPoisonConfig) bool {
	return !config.Enabled &&
		!config.StrictMode &&
		strings.TrimSpace(config.FailureMode) == "" &&
		strings.TrimSpace(config.StrategyPrompt) == "" &&
		strings.TrimSpace(config.AlgorithmPrompt) == "" &&
		!config.Randomization.Enabled &&
		config.Randomization.StrategyPoolSize == 0 &&
		config.Randomization.MinPhraseVariantsPerStrategy == 0 &&
		!config.Randomization.RandomInsertionPoints &&
		config.Randomization.MinFakeToolcalls == 0 &&
		!config.Randomization.RequirePerToolTypeMarker &&
		!config.StringProtection.Enabled &&
		len(config.StringProtection.Rules) == 0
}

func defaultAdvancedProxyHighAvailabilityRPMConfig() HighAvailabilityRPMConfig {
	return HighAvailabilityRPMConfig{
		Global:    0,
		Providers: map[string]*int{},
	}
}

func normalizeAdvancedProxyDispatchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fixed":
		return "fixed"
	case "ordered":
		return "ordered"
	case "random":
		return "random"
	default:
		return "fixed"
	}
}

func normalizeAdvancedProxyHighAvailabilityRPMConfig(config HighAvailabilityRPMConfig) HighAvailabilityRPMConfig {
	normalized := defaultAdvancedProxyHighAvailabilityRPMConfig()
	normalized.Global = clampInt(config.Global, 0, 1000000)
	if config.Providers == nil {
		return normalized
	}
	for scope, rawValue := range config.Providers {
		scope = strings.TrimSpace(scope)
		if scope == "" || isAdvancedProxyLegacyRPMScope(scope) {
			continue
		}
		if rawValue == nil {
			normalized.Providers[scope] = nil
			continue
		}
		value := clampInt(*rawValue, 0, 1000000)
		normalized.Providers[scope] = &value
	}
	return normalized
}

func isAdvancedProxyLegacyRPMScope(scope string) bool {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "claude", "codex", "opencode", "openclaw":
		return true
	default:
		return false
	}
}

func providerRPMKey(provider AdvancedProxyProvider) string {
	if key := strings.TrimSpace(provider.RowKey); key != "" {
		return key
	}
	if key := strings.TrimSpace(provider.ID); key != "" {
		return key
	}
	if key := strings.TrimSpace(provider.BaseURL); key != "" {
		return key
	}
	return ""
}

func resolveAdvancedProxyHighAvailabilityRPM(config AdvancedProxyConfig, provider AdvancedProxyProvider, appType string) int {
	providerKey := providerRPMKey(provider)
	if providerKey != "" && config.HighAvailability.RPM.Providers != nil {
		if rawValue, exists := config.HighAvailability.RPM.Providers[providerKey]; exists && rawValue != nil {
			return clampInt(*rawValue, 0, 1000000)
		}
	}

	return clampInt(config.HighAvailability.RPM.Global, 0, 1000000)
}

func advancedProxyQueuesLikelyMissing(queues AdvancedProxyQueuesConfig) bool {
	return !queues.Global.InheritGlobal &&
		!queues.Claude.InheritGlobal &&
		!queues.Codex.InheritGlobal &&
		!queues.GrokBuild.InheritGlobal &&
		!queues.OpenCode.InheritGlobal &&
		!queues.OpenClaw.InheritGlobal &&
		len(queues.Global.Providers) == 0 &&
		len(queues.Claude.Providers) == 0 &&
		len(queues.Codex.Providers) == 0 &&
		len(queues.GrokBuild.Providers) == 0 &&
		len(queues.OpenCode.Providers) == 0 &&
		len(queues.OpenClaw.Providers) == 0
}

func sanitizeAdvancedProxyQueueConfig(config AdvancedProxyQueueConfig, defaults AdvancedProxyQueueConfig, fallbackProviders []AdvancedProxyProvider) AdvancedProxyQueueConfig {
	providers := config.Providers
	if len(providers) == 0 && len(fallbackProviders) > 0 {
		providers = fallbackProviders
	}
	config.InheritGlobal = config.InheritGlobal
	if defaults.InheritGlobal {
		config.InheritGlobal = config.InheritGlobal
	} else {
		config.InheritGlobal = false
	}
	config.Providers = sanitizeAdvancedProxyProviders(providers)
	return config
}

func sanitizeAdvancedProxyAppConfig(config AdvancedProxyAppConfig, defaults AdvancedProxyAppConfig) AdvancedProxyAppConfig {
	if strings.TrimSpace(config.BasePath) == "" {
		config.BasePath = defaults.BasePath
	}
	config.BasePath = ensureLeadingSlash(strings.TrimSpace(config.BasePath))
	return config
}

func sanitizeAdvancedProxyProviders(providers []AdvancedProxyProvider) []AdvancedProxyProvider {
	cleaned := make([]AdvancedProxyProvider, 0, len(providers))
	seen := map[string]struct{}{}
	for index, provider := range providers {
		provider.ID = strings.TrimSpace(provider.ID)
		provider.RowKey = strings.TrimSpace(provider.RowKey)
		provider.Name = strings.TrimSpace(provider.Name)
		provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
		provider.APIKey = strings.TrimSpace(provider.APIKey)
		provider.Model = strings.TrimSpace(provider.Model)
		provider.SourceType = strings.TrimSpace(provider.SourceType)
		provider.APIFormat = normalizeClaudeAPIFormat(provider.APIFormat)
		provider.APIKeyField = normalizeClaudeAPIKeyField(provider.APIKeyField)
		if provider.ID == "" {
			switch {
			case provider.RowKey != "":
				provider.ID = provider.RowKey
			case provider.BaseURL != "" && provider.Model != "":
				provider.ID = provider.BaseURL + "::" + provider.Model
			case provider.BaseURL != "":
				provider.ID = provider.BaseURL
			default:
				provider.ID = fmt.Sprintf("provider-%d", index+1)
			}
		}
		if provider.SortIndex <= 0 {
			provider.SortIndex = index + 1
		}
		if provider.Name == "" {
			provider.Name = provider.BaseURL
		}
		if provider.BaseURL == "" || provider.APIKey == "" {
			continue
		}
		if _, exists := seen[provider.ID]; exists {
			continue
		}
		seen[provider.ID] = struct{}{}
		cleaned = append(cleaned, provider)
	}
	sort.SliceStable(cleaned, func(i, j int) bool {
		if cleaned[i].SortIndex != cleaned[j].SortIndex {
			return cleaned[i].SortIndex < cleaned[j].SortIndex
		}
		return cleaned[i].Name < cleaned[j].Name
	})
	for index := range cleaned {
		cleaned[index].SortIndex = index + 1
	}
	return cleaned
}

func advancedProxyProviderMatches(left AdvancedProxyProvider, right AdvancedProxyProvider) bool {
	leftRowKey := strings.TrimSpace(left.RowKey)
	rightRowKey := strings.TrimSpace(right.RowKey)
	if leftRowKey != "" && rightRowKey != "" {
		return leftRowKey == rightRowKey
	}

	leftID := strings.TrimSpace(left.ID)
	rightID := strings.TrimSpace(right.ID)
	if leftID != "" && rightID != "" {
		return leftID == rightID
	}

	leftBaseURL := strings.TrimRight(strings.TrimSpace(left.BaseURL), "/")
	rightBaseURL := strings.TrimRight(strings.TrimSpace(right.BaseURL), "/")
	leftModel := strings.TrimSpace(left.Model)
	rightModel := strings.TrimSpace(right.Model)
	if leftBaseURL != "" && rightBaseURL != "" && leftModel != "" && rightModel != "" {
		return leftBaseURL == rightBaseURL && leftModel == rightModel
	}

	if leftBaseURL != "" && rightBaseURL != "" {
		return leftBaseURL == rightBaseURL
	}

	return false
}

func updateAdvancedProxyProviderFormat(providers []AdvancedProxyProvider, target AdvancedProxyProvider, apiFormat string) ([]AdvancedProxyProvider, bool) {
	normalizedFormat := normalizeClaudeAPIFormat(apiFormat)
	if len(providers) == 0 {
		return providers, false
	}

	updated := append([]AdvancedProxyProvider(nil), providers...)
	changed := false
	for index := range updated {
		if !advancedProxyProviderMatches(updated[index], target) {
			continue
		}
		if normalizeClaudeAPIFormat(updated[index].APIFormat) == normalizedFormat {
			continue
		}
		updated[index].APIFormat = normalizedFormat
		changed = true
	}
	return updated, changed
}

func persistClaudeProviderAPIFormat(provider AdvancedProxyProvider, apiFormat string) (bool, error) {
	normalizedFormat := normalizeClaudeAPIFormat(apiFormat)
	if normalizedFormat == "" {
		return false, nil
	}

	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return false, err
	}

	changed := false
	if nextProviders, updated := updateAdvancedProxyProviderFormat(config.Queues.Global.Providers, provider, normalizedFormat); updated {
		config.Queues.Global.Providers = nextProviders
		changed = true
	}
	if nextProviders, updated := updateAdvancedProxyProviderFormat(config.Queues.Claude.Providers, provider, normalizedFormat); updated {
		config.Queues.Claude.Providers = nextProviders
		changed = true
	}
	if !changed {
		return false, nil
	}

	if _, err := saveAdvancedProxyConfig(config); err != nil {
		return false, err
	}
	return true, nil
}

func ensureLeadingSlash(value string) string {
	if value == "" {
		return "/"
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}

func normalizeClaudeAPIFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai_chat":
		return "openai_chat"
	case "openai_responses":
		return "openai_responses"
	default:
		return "anthropic"
	}
}

func normalizeClaudeAPIKeyField(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "ANTHROPIC_API_KEY") {
		return "ANTHROPIC_API_KEY"
	}
	return "ANTHROPIC_AUTH_TOKEN"
}

func normalizeAdvancedProxyQueueScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "claude", "codex", "grokbuild", "opencode", "openclaw":
		return strings.ToLower(strings.TrimSpace(scope))
	default:
		return advancedProxyGlobalScope
	}
}

func isAdvancedProxySupportedAppType(appType string) bool {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude", "codex", "grokbuild", "opencode", "openclaw":
		return true
	default:
		return false
	}
}

func isAdvancedProxySupportedQueueScope(scope string) bool {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "global", "claude", "codex", "grokbuild", "opencode", "openclaw":
		return true
	default:
		return false
	}
}

func advancedProxyAnyAppEnabled(config AdvancedProxyConfig) bool {
	return config.Claude.Enabled || config.Codex.Enabled || config.GrokBuild.Enabled || config.OpenCode.Enabled || config.OpenClaw.Enabled
}

func advancedProxyAppEnabled(config AdvancedProxyConfig, appType string) bool {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude":
		return config.Claude.Enabled
	case "codex":
		return config.Codex.Enabled
	case "grokbuild":
		return config.GrokBuild.Enabled
	case "opencode":
		return config.OpenCode.Enabled
	case "openclaw":
		return config.OpenClaw.Enabled
	default:
		return false
	}
}

func advancedProxyAppBasePath(config AdvancedProxyConfig, appType string) string {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude":
		return config.Claude.BasePath
	case "codex":
		return config.Codex.BasePath
	case "grokbuild":
		return config.GrokBuild.BasePath
	case "opencode":
		return config.OpenCode.BasePath
	case "openclaw":
		return config.OpenClaw.BasePath
	default:
		return "/"
	}
}

func advancedProxyQueueConfigForScope(config *AdvancedProxyConfig, scope string) *AdvancedProxyQueueConfig {
	switch normalizeAdvancedProxyQueueScope(scope) {
	case "claude":
		return &config.Queues.Claude
	case "codex":
		return &config.Queues.Codex
	case "grokbuild":
		return &config.Queues.GrokBuild
	case "opencode":
		return &config.Queues.OpenCode
	case "openclaw":
		return &config.Queues.OpenClaw
	default:
		return &config.Queues.Global
	}
}

func resolveAdvancedProxyQueueProviders(config AdvancedProxyConfig, scope string, effective bool) []AdvancedProxyProvider {
	queue := advancedProxyQueueConfigForScope(&config, scope)
	providers := queue.Providers
	if effective && normalizeAdvancedProxyQueueScope(scope) != advancedProxyGlobalScope && queue.InheritGlobal {
		providers = config.Queues.Global.Providers
	}
	return append([]AdvancedProxyProvider(nil), providers...)
}

func resolveAdvancedProxyEffectiveProviders(config AdvancedProxyConfig, appType string) []AdvancedProxyProvider {
	providers := resolveAdvancedProxyQueueProviders(config, appType, true)
	filtered := make([]AdvancedProxyProvider, 0, len(providers))
	for _, provider := range providers {
		if !provider.Enabled {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(appType), "claude") {
			filtered = append(filtered, provider)
			continue
		}
		if normalizeClaudeAPIFormat(provider.APIFormat) != "anthropic" {
			filtered = append(filtered, provider)
		}
	}
	return advancedProxyRuntime.OrderProvidersByHealth(config, appType, filtered)
}

func (a *App) GetAdvancedProxyConfig() (*AdvancedProxyConfig, error) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, err
	}
	config.ListenHost = bridgeServerHost
	config.ListenPort = currentBridgeServerPort()
	return &config, nil
}

func (a *App) GetAdvancedProxyConfigFilePath() string {
	return resolveAdvancedProxyConfigPath()
}

func (a *App) SetAdvancedProxyConfig(config AdvancedProxyConfig) (*AdvancedProxyConfig, error) {
	current, err := loadAdvancedProxyConfig()
	loadedCurrent := err == nil
	if err == nil {
		config.ListenHost = current.ListenHost
		config.ListenPort = current.ListenPort
	}
	saved, err := saveAdvancedProxyConfig(config)
	if err != nil {
		return nil, err
	}
	if loadedCurrent {
		logAdvancedProxyAntiPoisonConfigChange(current.AntiPoison, saved.AntiPoison)
	}
	if advancedProxyAnyAppEnabled(saved) {
		if err := a.ensureBridgeServer(); err != nil {
			appendAdvancedProxyLogf("[BRIDGE_SYNC_START_FAIL] detail=%s", previewAdvancedProxyText(err.Error(), 240))
			return nil, err
		}
	} else {
		a.stopBridgeServer()
	}
	saved.ListenHost = bridgeServerHost
	saved.ListenPort = currentBridgeServerPort()
	return &saved, nil
}

func logAdvancedProxyAntiPoisonConfigChange(before AntiPoisonConfig, after AntiPoisonConfig) {
	if before.Enabled == after.Enabled &&
		before.StrictMode == after.StrictMode &&
		before.FailureMode == after.FailureMode &&
		before.StrategyPrompt == after.StrategyPrompt &&
		before.AlgorithmPrompt == after.AlgorithmPrompt &&
		before.Randomization == after.Randomization &&
		before.StringProtection.Enabled == after.StringProtection.Enabled &&
		strings.Join(before.StringProtection.Rules, "\n") == strings.Join(after.StringProtection.Rules, "\n") {
		return
	}
	appendAdvancedProxyLogf(
		"[ANTI_POISON_CONFIG] enabled=%t strict=%t failure=%s strategy_len=%d algorithm_len=%d random_enabled=%t strategy_pool=%d phrase_variants=%d insertion_random=%t min_fake=%d per_type_marker=%t string_protection=%t string_rules=%d",
		after.Enabled,
		after.StrictMode,
		after.FailureMode,
		len(after.StrategyPrompt),
		len(after.AlgorithmPrompt),
		after.Randomization.Enabled,
		after.Randomization.StrategyPoolSize,
		after.Randomization.MinPhraseVariantsPerStrategy,
		after.Randomization.RandomInsertionPoints,
		after.Randomization.MinFakeToolcalls,
		after.Randomization.RequirePerToolTypeMarker,
		after.StringProtection.Enabled,
		len(after.StringProtection.Rules),
	)
}

func (a *App) GetFailoverQueue(appType string) ([]FailoverQueueItem, error) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, err
	}
	if !isAdvancedProxySupportedQueueScope(appType) {
		return []FailoverQueueItem{}, nil
	}
	providers := resolveAdvancedProxyQueueProviders(config, appType, true)
	items := make([]FailoverQueueItem, 0, len(providers))
	for index, provider := range providers {
		items = append(items, FailoverQueueItem{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			SortIndex:    index + 1,
			Enabled:      provider.Enabled,
		})
	}
	return items, nil
}

func (a *App) SetFailoverQueue(appType string, items []FailoverQueueItem) ([]FailoverQueueItem, error) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, err
	}
	if !isAdvancedProxySupportedQueueScope(appType) {
		return []FailoverQueueItem{}, nil
	}

	scope := normalizeAdvancedProxyQueueScope(appType)
	baseProviders := resolveAdvancedProxyQueueProviders(config, scope, true)
	providersByID := make(map[string]AdvancedProxyProvider, len(baseProviders))
	for _, provider := range baseProviders {
		providersByID[provider.ID] = provider
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortIndex != items[j].SortIndex {
			return items[i].SortIndex < items[j].SortIndex
		}
		return items[i].ProviderID < items[j].ProviderID
	})

	reordered := make([]AdvancedProxyProvider, 0, len(baseProviders))
	seen := map[string]struct{}{}
	for _, item := range items {
		provider, exists := providersByID[item.ProviderID]
		if !exists {
			continue
		}
		provider.Enabled = item.Enabled
		reordered = append(reordered, provider)
		seen[item.ProviderID] = struct{}{}
	}
	for _, provider := range baseProviders {
		if _, exists := seen[provider.ID]; exists {
			continue
		}
		reordered = append(reordered, provider)
	}

	queue := advancedProxyQueueConfigForScope(&config, scope)
	queue.Providers = sanitizeAdvancedProxyProviders(reordered)
	if scope != advancedProxyGlobalScope {
		queue.InheritGlobal = false
	} else {
		queue.InheritGlobal = false
	}

	saved, err := saveAdvancedProxyConfig(config)
	if err != nil {
		return nil, err
	}
	resultProviders := resolveAdvancedProxyQueueProviders(saved, scope, true)
	result := make([]FailoverQueueItem, 0, len(resultProviders))
	for index, provider := range resultProviders {
		result = append(result, FailoverQueueItem{
			ProviderID:   provider.ID,
			ProviderName: provider.Name,
			SortIndex:    index + 1,
			Enabled:      provider.Enabled,
		})
	}
	return result, nil
}

func (a *App) GetCircuitBreakerStats(appType string, providerID string) (*CircuitBreakerStats, error) {
	stats := advancedProxyRuntime.GetStats(strings.TrimSpace(appType), strings.TrimSpace(providerID))
	return &stats, nil
}

func (a *App) ResetCircuitBreaker(appType string, providerID string) (bool, error) {
	advancedProxyRuntime.Reset(strings.TrimSpace(appType), strings.TrimSpace(providerID))
	return true, nil
}

func (a *App) GetAdvancedProxyRoutingSnapshot() (*AdvancedProxyRoutingSnapshot, error) {
	snapshot := advancedProxyRuntime.GetRoutingSnapshot()
	return &snapshot, nil
}
