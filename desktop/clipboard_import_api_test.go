package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeClipboardImportAPIRequestSupportsAliases(t *testing.T) {
	request, err := decodeClipboardImportAPIRequest(strings.NewReader(`{
		"目标分组名":"福利组",
		"剪贴板文本":"https://api.example.com\nsk-example1234567890"
	}`))
	if err != nil {
		t.Fatalf("decode clipboard import request: %v", err)
	}
	if request.TargetGroupName != "福利组" {
		t.Fatalf("expected Chinese target group alias, got %q", request.TargetGroupName)
	}
	if !strings.Contains(request.ClipboardText, "sk-example") {
		t.Fatalf("expected clipboard text preserved, got %q", request.ClipboardText)
	}
}

func TestHandleClipboardImportHTTPDefaultsToAllGroups(t *testing.T) {
	var captured clipboardImportEventRequest
	app := &App{
		clipboardImportDispatchOverride: func(request clipboardImportEventRequest) (clipboardImportResult, error) {
			captured = request
			return clipboardImportResult{
				Success:       true,
				Mode:          "smart",
				ImportedCount: 1,
				CreatedCount:  1,
			}, nil
		},
	}

	request := httptest.NewRequest(http.MethodPost, "/api/key-management/clipboard-import", strings.NewReader(`{
		"clipboardText":"https://api.example.com\nsk-example1234567890"
	}`))
	recorder := httptest.NewRecorder()
	app.handleClipboardImportHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected success, got status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if captured.TargetGroupName != "" {
		t.Fatalf("expected omitted target group to remain empty, got %q", captured.TargetGroupName)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := toStringValue(response["targetGroupName"]); got != "全部分组" {
		t.Fatalf("expected all-groups response, got %q in %#v", got, response)
	}
	if got := toIntValue(response["importedCount"]); got != 1 {
		t.Fatalf("expected importedCount=1, got %d in %#v", got, response)
	}
}

func TestHandleClipboardImportHTTPPassesTargetGroupAndFrontendFailure(t *testing.T) {
	var captured clipboardImportEventRequest
	app := &App{
		clipboardImportDispatchOverride: func(request clipboardImportEventRequest) (clipboardImportResult, error) {
			captured = request
			return clipboardImportResult{
				Success: false,
				Error:   "未识别到 URL 与 API Key 组合",
			}, nil
		},
	}

	request := httptest.NewRequest(http.MethodPost, "/api/key-management/clipboard-import", strings.NewReader(`{
		"targetGroupName":"Grok 福利",
		"clipboardText":"not importable"
	}`))
	recorder := httptest.NewRecorder()
	app.handleClipboardImportHTTP(recorder, request)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if captured.TargetGroupName != "Grok 福利" {
		t.Fatalf("expected target group forwarded, got %q", captured.TargetGroupName)
	}
	if !strings.Contains(recorder.Body.String(), "未识别到") {
		t.Fatalf("expected frontend error in response, got %s", recorder.Body.String())
	}
}

func TestHandleClipboardImportHTTPValidatesMethodBodyAndAvailability(t *testing.T) {
	app := &App{}

	methodRecorder := httptest.NewRecorder()
	app.handleClipboardImportHTTP(methodRecorder, httptest.NewRequest(http.MethodGet, "/api/key-management/clipboard-import", nil))
	if methodRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", methodRecorder.Code)
	}

	bodyRecorder := httptest.NewRecorder()
	app.handleClipboardImportHTTP(bodyRecorder, httptest.NewRequest(http.MethodPost, "/api/key-management/clipboard-import", strings.NewReader(`{}`)))
	if bodyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got status=%d body=%s", bodyRecorder.Code, bodyRecorder.Body.String())
	}

	app.clipboardImportDispatchOverride = func(request clipboardImportEventRequest) (clipboardImportResult, error) {
		return clipboardImportResult{}, errors.New("frontend unavailable")
	}
	unavailableRecorder := httptest.NewRecorder()
	app.handleClipboardImportHTTP(unavailableRecorder, httptest.NewRequest(http.MethodPost, "/api/key-management/clipboard-import", strings.NewReader(`{"clipboardText":"text"}`)))
	if unavailableRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got status=%d body=%s", unavailableRecorder.Code, unavailableRecorder.Body.String())
	}
}
