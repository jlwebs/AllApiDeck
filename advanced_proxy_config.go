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
	advancedProxyOpenCodePath   = "/advanced-proxy/opencode/v1"
	advancedProxyOpenClawPath   = "/advanced-proxy/openclaw/v1"
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

type AdvancedProxyConfig struct {
	Enabled    bool                    `json:"enabled"`
	ListenHost string                  `json:"listenHost"`
	ListenPort int                     `json:"listenPort"`
	Claude     ClaudeProxyCompatConfig `json:"claude"`
	Codex      AdvancedProxyAppConfig  `json:"codex"`
	OpenCode   AdvancedProxyAppConfig  `json:"opencode"`
	OpenClaw   AdvancedProxyAppConfig  `json:"openclaw"`
	Failover   AppFailoverConfig       `json:"failover"`
	Rectifier  RectifierConfig         `json:"rectifier"`
	Optimizer  OptimizerConfig         `json:"optimizer"`
	UpdatedAt  string                  `json:"updatedAt"`
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

func defaultAdvancedProxyConfig() AdvancedProxyConfig {
	return AdvancedProxyConfig{
		Enabled:    false,
		ListenHost: bridgeServerHost,
		ListenPort: bridgeServerPort,
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
			MaxRetries:                2,
			StreamingFirstByteTimeout: 25,
			StreamingIdleTimeout:      60,
			NonStreamingTimeout:       90,
			CircuitFailureThreshold:   3,
			CircuitSuccessThreshold:   2,
			CircuitTimeoutSeconds:     45,
			CircuitErrorRateThreshold: 0.6,
			CircuitMinRequests:        3,
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

	if strings.TrimSpace(config.ListenHost) == "" {
		config.ListenHost = defaults.ListenHost
	}
	if config.ListenPort <= 0 {
		config.ListenPort = defaults.ListenPort
	}

	if strings.TrimSpace(config.Claude.BasePath) == "" {
		config.Claude.BasePath = defaults.Claude.BasePath
	}
	config.Claude.BasePath = ensureLeadingSlash(strings.TrimSpace(config.Claude.BasePath))
	config.Claude.DefaultModel = strings.TrimSpace(config.Claude.DefaultModel)
	config.Claude.Providers = sanitizeAdvancedProxyProviders(config.Claude.Providers)
	config.Codex = sanitizeAdvancedProxyAppConfig(config.Codex, defaults.Codex)
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
	if strings.TrimSpace(config.Optimizer.CacheTTL) == "" {
		config.Optimizer.CacheTTL = defaults.Optimizer.CacheTTL
	}
	config.Enabled = advancedProxyAnyAppEnabled(config)
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

func isAdvancedProxySupportedAppType(appType string) bool {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude", "codex", "opencode", "openclaw":
		return true
	default:
		return false
	}
}

func advancedProxyAnyAppEnabled(config AdvancedProxyConfig) bool {
	return config.Claude.Enabled || config.Codex.Enabled || config.OpenCode.Enabled || config.OpenClaw.Enabled
}

func advancedProxyAppEnabled(config AdvancedProxyConfig, appType string) bool {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude":
		return config.Claude.Enabled
	case "codex":
		return config.Codex.Enabled
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
	case "opencode":
		return config.OpenCode.BasePath
	case "openclaw":
		return config.OpenClaw.BasePath
	default:
		return "/"
	}
}

func (a *App) GetAdvancedProxyConfig() (*AdvancedProxyConfig, error) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (a *App) GetAdvancedProxyConfigFilePath() string {
	return resolveAdvancedProxyConfigPath()
}

func (a *App) SetAdvancedProxyConfig(config AdvancedProxyConfig) (*AdvancedProxyConfig, error) {
	saved, err := saveAdvancedProxyConfig(config)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (a *App) GetFailoverQueue(appType string) ([]FailoverQueueItem, error) {
	config, err := loadAdvancedProxyConfig()
	if err != nil {
		return nil, err
	}
	if !isAdvancedProxySupportedAppType(appType) {
		return []FailoverQueueItem{}, nil
	}
	items := make([]FailoverQueueItem, 0, len(config.Claude.Providers))
	for index, provider := range config.Claude.Providers {
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
	if !isAdvancedProxySupportedAppType(appType) {
		return []FailoverQueueItem{}, nil
	}

	providersByID := make(map[string]AdvancedProxyProvider, len(config.Claude.Providers))
	for _, provider := range config.Claude.Providers {
		providersByID[provider.ID] = provider
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortIndex != items[j].SortIndex {
			return items[i].SortIndex < items[j].SortIndex
		}
		return items[i].ProviderID < items[j].ProviderID
	})

	reordered := make([]AdvancedProxyProvider, 0, len(config.Claude.Providers))
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
	for _, provider := range config.Claude.Providers {
		if _, exists := seen[provider.ID]; exists {
			continue
		}
		reordered = append(reordered, provider)
	}

	config.Claude.Providers = sanitizeAdvancedProxyProviders(reordered)
	saved, err := saveAdvancedProxyConfig(config)
	if err != nil {
		return nil, err
	}
	result := make([]FailoverQueueItem, 0, len(saved.Claude.Providers))
	for index, provider := range saved.Claude.Providers {
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
