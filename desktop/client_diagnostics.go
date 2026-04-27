package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (a *App) AppendClientLog(scope string, message string) {
	appendClientRuntimeLog(scope, message)
}

func appendClientRuntimeLog(scope string, message string) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "client"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	appendLine(filepath.Join(resolveRuntimeLogDir(), "client-runtime.log"), fmt.Sprintf("[%s] %s", scope, message))
}
