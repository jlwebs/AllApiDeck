//go:build !windows

package main

import (
	"net/http"
	"net/url"
)

func resolvePlatformSystemProxy(req *http.Request) (*url.URL, error) {
	_ = req
	return nil, nil
}
