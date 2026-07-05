package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const trayPreferencesFileName = "tray-preferences.json"

type trayPreferences struct {
	MinimizeActivatesSidebar *bool `json:"minimizeActivatesSidebar,omitempty"`
}

var trayPreferencesMu sync.Mutex

func trayPreferencesPath() string {
	return filepath.Join(resolveRuntimeRootDir(), trayPreferencesFileName)
}

func readTrayPreferences() trayPreferences {
	path := trayPreferencesPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return trayPreferences{}
	}
	var prefs trayPreferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return trayPreferences{}
	}
	return prefs
}

func isMinimizeActivatesSidebarEnabled() bool {
	trayPreferencesMu.Lock()
	defer trayPreferencesMu.Unlock()

	prefs := readTrayPreferences()
	if prefs.MinimizeActivatesSidebar == nil {
		return true
	}
	return *prefs.MinimizeActivatesSidebar
}

func setMinimizeActivatesSidebarEnabled(enabled bool) error {
	trayPreferencesMu.Lock()
	defer trayPreferencesMu.Unlock()

	value := enabled
	prefs := readTrayPreferences()
	prefs.MinimizeActivatesSidebar = &value
	raw, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	path := trayPreferencesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicWriteTextFile(path, string(raw))
}

func toggleMinimizeActivatesSidebarEnabled() bool {
	next := !isMinimizeActivatesSidebarEnabled()
	if err := setMinimizeActivatesSidebarEnabled(next); err != nil {
		debugLogf("toggle minimize sidebar preference failed: %v", err)
		return !next
	}
	return next
}
