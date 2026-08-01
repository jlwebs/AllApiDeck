package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFoldAntiCandyResponsesStream(t *testing.T) {
	baseBody := []byte(`{"model":"gpt-5.5","stream":true,"input":[{"type":"message","role":"user","content":"solve it"}],"previous_response_id":null}`)
	firstReasoning := map[string]any{
		"type":              "reasoning",
		"id":                "rs_1",
		"encrypted_content": "enc-1",
		"summary":           []any{},
	}
	intermediateTool := map[string]any{
		"type":      "function_call",
		"id":        "fc_intermediate",
		"call_id":   "call_intermediate",
		"name":      "search",
		"arguments": "{}",
	}
	firstRaw := strings.Join([]string{
		antiCandyTestSSEEvent("response.created", map[string]any{
			"type": "response.created",
			"response": map[string]any{
				"id":     "resp_1",
				"model":  "gpt-5.5",
				"status": "in_progress",
			},
		}),
		antiCandyTestSSEEvent("response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item":         firstReasoning,
		}),
		antiCandyTestSSEEvent("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item":         firstReasoning,
		}),
		antiCandyTestSSEEvent("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": 1,
			"item":         intermediateTool,
		}),
		antiCandyTestSSEEvent("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_1",
				"model":  "gpt-5.5",
				"status": "completed",
				"output": []any{firstReasoning, intermediateTool},
				"usage": map[string]any{
					"input_tokens":  100,
					"output_tokens": 516,
					"total_tokens":  616,
					"output_tokens_details": map[string]any{
						"reasoning_tokens": 516,
					},
				},
			},
		}),
		"data: [DONE]\n\n",
	}, "")

	secondReasoning := map[string]any{
		"type":              "reasoning",
		"id":                "rs_2",
		"encrypted_content": "enc-2",
		"summary":           []any{},
	}
	finalMessage := map[string]any{
		"type": "message",
		"id":   "msg_1",
		"role": "assistant",
		"content": []any{map[string]any{
			"type": "output_text",
			"text": "done",
		}},
	}
	secondRaw := strings.Join([]string{
		antiCandyTestSSEEvent("response.created", map[string]any{
			"type": "response.created",
			"response": map[string]any{
				"id":     "resp_2",
				"model":  "gpt-5.5",
				"status": "in_progress",
			},
		}),
		antiCandyTestSSEEvent("response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": 0,
			"item":         secondReasoning,
		}),
		antiCandyTestSSEEvent("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": 0,
			"item":         secondReasoning,
		}),
		antiCandyTestSSEEvent("response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": 1,
			"item":         finalMessage,
		}),
		antiCandyTestSSEEvent("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": 1,
			"item":         finalMessage,
		}),
		antiCandyTestSSEEvent("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_2",
				"model":  "gpt-5.5",
				"status": "completed",
				"output": []any{secondReasoning, finalMessage},
				"usage": map[string]any{
					"input_tokens":  120,
					"output_tokens": 120,
					"total_tokens":  240,
					"output_tokens_details": map[string]any{
						"reasoning_tokens": 100,
					},
				},
			},
		}),
		"data: [DONE]\n\n",
	}, "")

	var continuationBody map[string]any
	folded, stats, err := foldAntiCandyResponsesStreamBytes([]byte(firstRaw), baseBody, AntiCandyConfig{
		Enabled:     true,
		Models:      []string{"gpt-5.5"},
		MaxContinue: 3,
		MaxTierN:    6,
		MarkerText:  "Continue thinking...",
	}, func(rawBody []byte) (int, http.Header, []byte, error) {
		if err := json.Unmarshal(rawBody, &continuationBody); err != nil {
			return http.StatusBadRequest, nil, nil, err
		}
		return http.StatusOK, http.Header{"Content-Type": []string{"text/event-stream"}}, []byte(secondRaw), nil
	})
	if err != nil {
		t.Fatalf("fold returned error: %v", err)
	}
	if !stats.Folded || stats.Continuations != 1 || stats.Rounds != 2 {
		t.Fatalf("unexpected fold stats: %+v", stats)
	}
	if got := len(continuationBody["input"].([]any)); got != 3 {
		t.Fatalf("continuation input length = %d, want 3", got)
	}
	if _, ok := continuationBody["previous_response_id"]; ok {
		t.Fatal("continuation unexpectedly retained previous_response_id")
	}
	if include := continuationBody["include"].([]any); len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Fatalf("continuation include = %#v", include)
	}
	nudge, ok := continuationBody["input"].([]any)[2].(map[string]any)
	if !ok || nudge["phase"] != "commentary" {
		t.Fatalf("continuation nudge = %#v", continuationBody["input"].([]any)[2])
	}

	events := parseAntiCandySSEEvents(folded)
	var terminal map[string]any
	for _, event := range events {
		if event.Type == "response.completed" {
			terminal = event.Payload["response"].(map[string]any)
		}
	}
	if terminal == nil {
		t.Fatal("folded stream has no terminal response")
	}
	if terminal["id"] != "resp_1" || terminal["status"] != "completed" {
		t.Fatalf("terminal identity/status = %v/%v", terminal["id"], terminal["status"])
	}
	output, ok := terminal["output"].([]any)
	if !ok || len(output) != 3 {
		t.Fatalf("terminal output = %#v", terminal["output"])
	}
	if output[0].(map[string]any)["id"] != "rs_1" || output[1].(map[string]any)["id"] != "rs_2" || output[2].(map[string]any)["id"] != "msg_1" {
		t.Fatalf("terminal output order = %#v", output)
	}
	metadata := terminal["metadata"].(map[string]any)
	if len(metadata["proxy_rounds"].([]any)) != 2 {
		t.Fatalf("proxy_rounds = %#v", metadata["proxy_rounds"])
	}
	if metadata["proxy_stopped_reason"] != nil {
		t.Fatalf("unexpected stop reason: %#v", metadata["proxy_stopped_reason"])
	}
	billed := metadata["proxy_billed_usage"].(map[string]any)
	if billed["total_tokens"] != float64(856) {
		t.Fatalf("billed total_tokens = %#v, want 856", billed["total_tokens"])
	}
	usage := terminal["usage"].(map[string]any)
	if usage["output_tokens"] != float64(636) {
		t.Fatalf("agent output_tokens = %#v, want 636", usage["output_tokens"])
	}
	if strings.Contains(string(folded), "fc_intermediate") {
		t.Fatal("intermediate tool call leaked into folded stream")
	}
}

func TestFoldAntiCandyResponsesStreamLeavesNormalResponseUntouched(t *testing.T) {
	baseBody := []byte(`{"model":"gpt-5.5","stream":true,"input":[]}`)
	raw := strings.Join([]string{
		antiCandyTestSSEEvent("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"status": "completed",
				"usage": map[string]any{
					"output_tokens_details": map[string]any{"reasoning_tokens": 500},
				},
			},
		}),
		"data: [DONE]\n\n",
	}, "")
	called := false
	folded, stats, err := foldAntiCandyResponsesStreamBytes([]byte(raw), baseBody, AntiCandyConfig{Enabled: true, Models: []string{"gpt-5.5"}}, func([]byte) (int, http.Header, []byte, error) {
		called = true
		return http.StatusOK, nil, nil, nil
	})
	if err != nil {
		t.Fatalf("fold returned error: %v", err)
	}
	if stats.Folded || called || string(folded) != string(raw) {
		t.Fatalf("normal response changed: folded=%+v called=%t", stats, called)
	}
}

func antiCandyTestSSEEvent(eventType string, payload map[string]any) string {
	encoded, _ := json.Marshal(payload)
	return "event: " + eventType + "\ndata: " + string(encoded) + "\n\n"
}
