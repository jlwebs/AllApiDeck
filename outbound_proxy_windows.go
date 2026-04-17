//go:build windows

package main

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func resolvePlatformSystemProxy(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}

	proxyValue, err := readWindowsInternetProxy()
	if err != nil || strings.TrimSpace(proxyValue) == "" {
		return nil, err
	}

	target := pickWindowsProxyForScheme(proxyValue, req.URL.Scheme)
	if strings.TrimSpace(target) == "" {
		return nil, nil
	}

	return normalizeSystemProxyURL(target, req.URL.Scheme)
}

func readWindowsInternetProxy() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return "", nil
	}

	value, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func pickWindowsProxyForScheme(proxyValue string, scheme string) string {
	normalizedScheme := strings.ToLower(strings.TrimSpace(scheme))
	if !strings.Contains(proxyValue, "=") {
		return strings.TrimSpace(proxyValue)
	}

	selected := ""
	pairs := strings.Split(proxyValue, ";")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		if selected == "" && key == "socks" {
			selected = "socks5://" + value
		}
		if key == normalizedScheme {
			if key == "socks" {
				return "socks5://" + value
			}
			return value
		}
	}
	return selected
}

func normalizeSystemProxyURL(raw string, requestScheme string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if strings.Contains(value, "://") {
		return url.Parse(value)
	}

	scheme := "http"
	if strings.EqualFold(strings.TrimSpace(requestScheme), "https") {
		scheme = "http"
	}
	return url.Parse(scheme + "://" + value)
}
