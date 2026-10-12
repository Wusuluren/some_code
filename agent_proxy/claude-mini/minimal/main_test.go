package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"claude-code-go/minimal/api"
	"claude-code-go/minimal/engine"
	"claude-code-go/minimal/tools"
)

// TestAgentLoopEndToEnd drives the executeQueryLoop against a fake
// Messages API: turn 1 asks for a Write tool call, turn 2 finishes.
func TestAgentLoopEndToEnd(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "msg_1", "role": "assistant", "model": "test",
				"stop_reason": "tool_use",
				"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
				"content": []map[string]interface{}{
					{"type": "text", "text": "I will create the file."},
					{"type": "tool_use", "id": "tu_1", "name": "Write",
						"input": map[string]string{
							"file_path": filepath.Join(cwd, "hello.txt"),
							"content":   "hello from agent\n",
						}},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "msg_2", "role": "assistant", "model": "test",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 20, "output_tokens": 3},
			"content":     []map[string]interface{}{{"type": "text", "text": "Done."}},
		})
	}))
	defer server.Close()

	eng := engine.New(engine.Config{
		Model:    "test-model",
		MaxTurns: 5,
		Tools:    tools.NewRegistry(),
		Client:   api.NewClient(api.Config{APIKey: "test-key", BaseURL: server.URL + "/v1"}),
	})

	var events []engine.Event
	for ev := range eng.Run(context.Background(), "create hello.txt") {
		events = append(events, ev)
	}

	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}

	data, err := os.ReadFile(filepath.Join(cwd, "hello.txt"))
	if err != nil {
		t.Fatalf("tool did not write file: %v", err)
	}
	if string(data) != "hello from agent\n" {
		t.Fatalf("unexpected file content: %q", data)
	}

	var sawText, sawToolResult bool
	var last engine.Event
	for _, ev := range events {
		switch ev.Type {
		case "assistant_text":
			sawText = true
		case "tool_result":
			sawToolResult = true
		}
		last = ev
	}
	if !sawText || !sawToolResult {
		t.Fatalf("missing events: assistant_text=%v tool_result=%v", sawText, sawToolResult)
	}
	if last.Type != "result" || last.Text != "" {
		t.Fatalf("expected clean result event, got %+v", last)
	}
	if last.Usage.InputTokens != 30 || last.Usage.OutputTokens != 8 {
		t.Fatalf("usage not accumulated: %+v", last.Usage)
	}
}
