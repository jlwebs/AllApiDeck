//go:build !windows

package main

import "fmt"

type desktopProfileAssistOpenRequest struct {
	SiteName string `json:"siteName"`
	SiteURL  string `json:"siteUrl"`
	SiteType string `json:"siteType"`
}

type desktopProfileAssistWindowResult struct {
	SiteName            string   `json:"siteName"`
	SiteURL             string   `json:"siteUrl"`
	InjectedCookies     int      `json:"injectedCookies"`
	InjectedCookieNames []string `json:"injectedCookieNames,omitempty"`
	StorageFields       []string `json:"storageFields,omitempty"`
	Message             string   `json:"message,omitempty"`
}

func openDesktopProfileAssistWindow(request desktopProfileAssistOpenRequest) (*desktopProfileAssistWindowResult, error) {
	return nil, fmt.Errorf("profile assist window is only available on Windows")
}

func closeProfileAssistWindowsByHosts(hosts []string) int {
	_ = hosts
	return 0
}
