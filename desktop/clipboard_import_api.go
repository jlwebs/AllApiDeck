package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	clipboardImportRequestEvent = "batch-api-check:clipboard-import-request"
	clipboardImportResultEvent  = "batch-api-check:clipboard-import-result"
	clipboardImportBodyLimit    = 4 << 20
	clipboardImportTimeout      = 15 * time.Second
)

type clipboardImportAPIRequest struct {
	TargetGroupName        string `json:"targetGroupName"`
	TargetGroupNameSnake   string `json:"target_group_name"`
	TargetGroupNameChinese string `json:"目标分组名"`
	ClipboardText          string `json:"clipboardText"`
	ClipboardTextSnake     string `json:"clipboard_text"`
	ClipboardTextChinese   string `json:"剪贴板文本"`
}

type clipboardImportEventRequest struct {
	RequestID       string `json:"requestId"`
	TargetGroupName string `json:"targetGroupName,omitempty"`
	ClipboardText   string `json:"clipboardText"`
}

type clipboardImportResult struct {
	RequestID       string `json:"requestId"`
	Success         bool   `json:"success"`
	Mode            string `json:"mode,omitempty"`
	ImportedCount   int    `json:"importedCount,omitempty"`
	CreatedCount    int    `json:"createdCount,omitempty"`
	UpdatedCount    int    `json:"updatedCount,omitempty"`
	TargetGroupName string `json:"targetGroupName,omitempty"`
	GroupCreated    bool   `json:"groupCreated,omitempty"`
	Error           string `json:"error,omitempty"`
}

func (request clipboardImportAPIRequest) normalize() (clipboardImportEventRequest, error) {
	clipboardText := firstNonEmptyExact(
		request.ClipboardText,
		request.ClipboardTextSnake,
		request.ClipboardTextChinese,
	)
	if strings.TrimSpace(clipboardText) == "" {
		return clipboardImportEventRequest{}, errors.New("clipboardText is required")
	}
	targetGroupName := firstNonEmpty(
		strings.TrimSpace(request.TargetGroupName),
		strings.TrimSpace(request.TargetGroupNameSnake),
		strings.TrimSpace(request.TargetGroupNameChinese),
	)
	return clipboardImportEventRequest{
		TargetGroupName: targetGroupName,
		ClipboardText:   clipboardText,
	}, nil
}

func decodeClipboardImportAPIRequest(reader io.Reader) (clipboardImportEventRequest, error) {
	request := clipboardImportAPIRequest{}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&request); err != nil {
		return clipboardImportEventRequest{}, fmt.Errorf("invalid JSON body: %w", err)
	}
	return request.normalize()
}

func (a *App) initClipboardImportResultListener() {
	if a == nil || a.ctx == nil {
		return
	}
	a.clipboardImportMu.Lock()
	defer a.clipboardImportMu.Unlock()
	if a.clipboardImportEventsOff != nil {
		return
	}
	if a.clipboardImportPending == nil {
		a.clipboardImportPending = make(map[string]chan clipboardImportResult)
	}
	a.clipboardImportEventsOff = wruntime.EventsOn(a.ctx, clipboardImportResultEvent, func(optionalData ...interface{}) {
		if len(optionalData) == 0 || optionalData[0] == nil {
			return
		}
		raw, err := json.Marshal(optionalData[0])
		if err != nil {
			return
		}
		result := clipboardImportResult{}
		if err := json.Unmarshal(raw, &result); err != nil || strings.TrimSpace(result.RequestID) == "" {
			return
		}
		a.clipboardImportMu.Lock()
		pending := a.clipboardImportPending[result.RequestID]
		a.clipboardImportMu.Unlock()
		if pending == nil {
			return
		}
		select {
		case pending <- result:
		default:
		}
	})
}

func (a *App) stopClipboardImportResultListener() {
	if a == nil {
		return
	}
	a.clipboardImportMu.Lock()
	cancel := a.clipboardImportEventsOff
	a.clipboardImportEventsOff = nil
	a.clipboardImportPending = nil
	a.clipboardImportMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) dispatchClipboardImport(request clipboardImportEventRequest) (clipboardImportResult, error) {
	if a == nil {
		return clipboardImportResult{}, errors.New("desktop app is unavailable")
	}
	if a.clipboardImportDispatchOverride != nil {
		return a.clipboardImportDispatchOverride(request)
	}
	if a.ctx == nil {
		return clipboardImportResult{}, errors.New("desktop runtime is unavailable")
	}

	request.RequestID = fmt.Sprintf(
		"clipboard_%d_%d",
		time.Now().UnixNano(),
		a.clipboardImportSequence.Add(1),
	)
	pending := make(chan clipboardImportResult, 1)
	a.clipboardImportMu.Lock()
	if a.clipboardImportPending == nil {
		a.clipboardImportPending = make(map[string]chan clipboardImportResult)
	}
	a.clipboardImportPending[request.RequestID] = pending
	a.clipboardImportMu.Unlock()
	defer func() {
		a.clipboardImportMu.Lock()
		delete(a.clipboardImportPending, request.RequestID)
		a.clipboardImportMu.Unlock()
	}()

	wruntime.EventsEmit(a.ctx, clipboardImportRequestEvent, request)
	timer := time.NewTimer(clipboardImportTimeout)
	defer timer.Stop()
	select {
	case result := <-pending:
		return result, nil
	case <-timer.C:
		return clipboardImportResult{}, errors.New("clipboard import frontend did not respond")
	}
}

func clipboardImportResponsePayload(result clipboardImportResult) map[string]any {
	return map[string]any{
		"success":         true,
		"mode":            result.Mode,
		"importedCount":   result.ImportedCount,
		"createdCount":    result.CreatedCount,
		"updatedCount":    result.UpdatedCount,
		"targetGroupName": firstNonEmpty(strings.TrimSpace(result.TargetGroupName), "全部分组"),
		"groupCreated":    result.GroupCreated,
	}
}

func (a *App) executeClipboardImport(request clipboardImportEventRequest) (int, map[string]any) {
	result, err := a.dispatchClipboardImport(request)
	if err != nil {
		return http.StatusServiceUnavailable, map[string]any{
			"success": false,
			"code":    "CLIPBOARD_IMPORT_FRONTEND_UNAVAILABLE",
			"message": err.Error(),
		}
	}
	if !result.Success {
		return http.StatusUnprocessableEntity, map[string]any{
			"success": false,
			"code":    "CLIPBOARD_IMPORT_FAILED",
			"message": firstNonEmpty(strings.TrimSpace(result.Error), "clipboard import failed"),
		}
	}
	return http.StatusOK, clipboardImportResponsePayload(result)
}

func (a *App) handleLocalClipboardImport(method string, body string) *bridgeHTTPResponse {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return jsonBridgeResponse(http.StatusMethodNotAllowed, map[string]any{
			"success": false,
			"message": "Method Not Allowed",
		})
	}
	if len(body) > clipboardImportBodyLimit {
		return jsonBridgeResponse(http.StatusRequestEntityTooLarge, map[string]any{
			"success": false,
			"message": "request body is too large",
		})
	}
	request, err := decodeClipboardImportAPIRequest(strings.NewReader(body))
	if err != nil {
		return jsonBridgeResponse(http.StatusBadRequest, map[string]any{
			"success": false,
			"message": err.Error(),
		})
	}
	status, payload := a.executeClipboardImport(request)
	return jsonBridgeResponse(status, payload)
}

func (a *App) handleClipboardImportHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Allow", "POST")
	if request == nil || request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": false,
			"message": "Method Not Allowed",
		})
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, clipboardImportBodyLimit)
	decoded, err := decodeClipboardImportAPIRequest(request.Body)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			status = http.StatusRequestEntityTooLarge
		}
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	status, payload := a.executeClipboardImport(decoded)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
