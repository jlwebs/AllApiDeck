package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCandyCodexGatewayCommandPreservesArgumentsWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Codex .cmd invocation is Windows-specific")
	}

	executable := writeCodexArgumentCaptureScript(t)
	command := buildCandyCodexCommand(
		context.Background(),
		executable,
		"gpt-5.6-luna",
		"max",
		candyEvalModeGateway,
		"http://127.0.0.1:19876/advanced-proxy/codex/v1",
	)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	if err := command.Run(); err != nil {
		t.Fatalf("gateway command failed: %v\n%s", err, output.String())
	}

	const expected = "ARG=[model_providers.allapideck_gateway.name=AllApiDeck Advanced Proxy]"
	if !strings.Contains(output.String(), expected) {
		t.Fatalf("gateway provider name was split across argv: expected %q in:\n%s", expected, output.String())
	}
}

func TestJuiceCodexGatewayCommandPreservesArgumentsWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Codex .cmd invocation is Windows-specific")
	}

	executable := writeCodexArgumentCaptureScript(t)
	command := buildJuiceCodexCommand(
		context.Background(),
		executable,
		"gpt-5.6-luna",
		"max",
		candyEvalModeGateway,
		"http://127.0.0.1:19876/advanced-proxy/codex/v1",
		"",
		"test prompt",
	)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	if err := command.Run(); err != nil {
		t.Fatalf("gateway command failed: %v\n%s", err, output.String())
	}

	const expected = "ARG=[model_providers.allapideck_gateway.name=AllApiDeck Advanced Proxy]"
	if !strings.Contains(output.String(), expected) {
		t.Fatalf("gateway provider name was split across argv: expected %q in:\n%s", expected, output.String())
	}
}

func writeCodexArgumentCaptureScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture-codex-args.cmd")
	contents := "@echo off\r\n" +
		":next\r\n" +
		"if \"%~1\"==\"\" exit /b 0\r\n" +
		"echo ARG=[%~1]\r\n" +
		"shift\r\n" +
		"goto next\r\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("write capture script: %v", err)
	}
	return path
}
