package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

const (
	outboundProxyModeSystem = "system"
	outboundProxyModeDirect = "direct"
	outboundProxyModeCustom = "custom"
)

var outboundProxyConfigMu sync.Mutex

type OutboundProxyConfig struct {
	Mode      string `json:"mode"`
	CustomURL string `json:"customUrl"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

func defaultOutboundProxyConfig() OutboundProxyConfig {
	return OutboundProxyConfig{
		Mode: outboundProxyModeSystem,
	}
}

func resolveOutboundProxyConfigPath() string {
	dir := filepath.Join(resolveRuntimeRootDir(), "network")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "outbound-proxy.json")
}

func sanitizeOutboundProxyConfig(config OutboundProxyConfig) OutboundProxyConfig {
	switch strings.ToLower(strings.TrimSpace(config.Mode)) {
	case outboundProxyModeDirect:
		config.Mode = outboundProxyModeDirect
		config.CustomURL = ""
	case outboundProxyModeCustom:
		config.Mode = outboundProxyModeCustom
		config.CustomURL = strings.TrimSpace(config.CustomURL)
	default:
		config.Mode = outboundProxyModeSystem
		config.CustomURL = ""
	}
	return config
}

func validateOutboundProxyConfig(config OutboundProxyConfig) error {
	if config.Mode != outboundProxyModeCustom {
		return nil
	}
	if strings.TrimSpace(config.CustomURL) == "" {
		return fmt.Errorf("自定义代理地址不能为空")
	}
	parsed, err := url.Parse(strings.TrimSpace(config.CustomURL))
	if err != nil {
		return fmt.Errorf("代理地址格式无效: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	switch scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("代理协议不支持，仅支持 http / https / socks5 / socks5h")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("代理地址缺少 host:port")
	}
	return nil
}

func loadOutboundProxyConfig() (OutboundProxyConfig, error) {
	outboundProxyConfigMu.Lock()
	defer outboundProxyConfigMu.Unlock()

	config := defaultOutboundProxyConfig()
	raw, err := os.ReadFile(resolveOutboundProxyConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return defaultOutboundProxyConfig(), err
	}
	config = sanitizeOutboundProxyConfig(config)
	if err := validateOutboundProxyConfig(config); err != nil {
		debugLogf("outbound proxy config invalid, fallback to system: %v", err)
		return defaultOutboundProxyConfig(), nil
	}
	return config, nil
}

func saveOutboundProxyConfig(config OutboundProxyConfig) (OutboundProxyConfig, error) {
	outboundProxyConfigMu.Lock()
	defer outboundProxyConfigMu.Unlock()

	config = sanitizeOutboundProxyConfig(config)
	if err := validateOutboundProxyConfig(config); err != nil {
		return config, err
	}
	config.UpdatedAt = time.Now().Format(time.RFC3339)
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return config, err
	}
	if err := os.WriteFile(resolveOutboundProxyConfigPath(), raw, 0o644); err != nil {
		return config, err
	}
	debugLogf("outbound proxy config saved: mode=%s", config.Mode)
	return config, nil
}

func (a *App) GetOutboundProxyConfig() (*OutboundProxyConfig, error) {
	config, err := loadOutboundProxyConfig()
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (a *App) SetOutboundProxyConfig(config OutboundProxyConfig) (*OutboundProxyConfig, error) {
	saved, err := saveOutboundProxyConfig(config)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func newOutboundHTTPClient(timeout time.Duration) (*http.Client, error) {
	config, err := loadOutboundProxyConfig()
	if err != nil {
		return nil, err
	}
	transport, err := newOutboundHTTPTransport(config)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

func newOutboundHTTPTransport(config OutboundProxyConfig) (*http.Transport, error) {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default transport unavailable")
	}
	transport := baseTransport.Clone()
	config = sanitizeOutboundProxyConfig(config)

	switch config.Mode {
	case outboundProxyModeDirect:
		transport.Proxy = nil
	case outboundProxyModeCustom:
		parsed, err := url.Parse(strings.TrimSpace(config.CustomURL))
		if err != nil {
			return nil, fmt.Errorf("invalid proxy url: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
		case "http", "https":
			transport.Proxy = http.ProxyURL(parsed)
		case "socks5", "socks5h":
			socksURL := *parsed
			socksURL.Scheme = "socks5"
			dialer, err := proxy.FromURL(&socksURL, &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			})
			if err != nil {
				return nil, fmt.Errorf("invalid socks proxy: %w", err)
			}
			transport.Proxy = nil
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialProxyContext(ctx, dialer, network, address)
			}
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
		}
	default:
		transport.Proxy = resolveSystemProxyFunc()
	}

	return transport, nil
}

func dialProxyContext(ctx context.Context, dialer proxy.Dialer, network string, address string) (net.Conn, error) {
	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, network, address)
	}

	type dialResult struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan dialResult, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		resultCh <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.conn, result.err
	}
}

func resolveSystemProxyFunc() func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		if req == nil {
			return nil, nil
		}
		if envProxy, err := http.ProxyFromEnvironment(req); envProxy != nil || err != nil {
			return envProxy, err
		}
		return resolvePlatformSystemProxy(req)
	}
}
