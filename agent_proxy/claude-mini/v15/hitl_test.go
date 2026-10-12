package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-code-go/v15/api"
	"claude-code-go/v15/engine"
	"claude-code-go/v15/tools"
)

// TestHumanInTheLoop covers the v6 addition: the engine consults the
// guard before each tool call; denials reach the model as errored
// tool_results and the tool never runs.
func TestHumanInTheLoop(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	target := filepath.Join(cwd, "out.txt")

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "m1", "role": "assistant", "model": "test", "stop_reason": "tool_use",
				"usage": map[string]int{"input_tokens": 1, "output_tokens": 1},
				"content": []map[string]interface{}{{"type": "tool_use", "id": "tu_1", "name": "Write",
					"input": map[string]string{"file_path": target, "content": "should not exist"}}},
			})
			return
		}
		if calls == 2 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "m2", "role": "assistant", "model": "test", "stop_reason": "tool_use",
				"usage": map[string]int{"input_tokens": 1, "output_tokens": 1},
				"content": []map[string]interface{}{{"type": "tool_use", "id": "tu_2", "name": "Ls",
					"input": map[string]string{"path": "."}}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "m3", "role": "assistant", "model": "test", "stop_reason": "end_turn",
			"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
			"content": []map[string]interface{}{{"type": "text", "text": "ok"}},
		})
	}))
	defer server.Close()

	guardCalls := 0
	eng := engine.New(engine.Config{Model: "test", MaxTurns: 5, Tools: tools.NewRegistry(),
		Client: api.NewClient(api.Config{APIKey: "k", BaseURL: server.URL + "/v1"}),
		Guard: func(ctx context.Context, name string, input json.RawMessage) (bool, string) {
			guardCalls++
			if name == "Write" {
				return false, "deleting/overwriting files needs review"
			}
			return true, ""
		}})

	var results []engine.Event
	for ev := range eng.Run(context.Background(), "write the file") {
		if ev.Type == "tool_result" {
			results = append(results, ev)
		}
	}

	if guardCalls != 2 {
		t.Fatalf("guard should be consulted for both calls, got %d", guardCalls)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("rejected Write must not touch the filesystem")
	}
	if len(results) != 2 || !strings.Contains(results[0].Text, "user rejected") {
		t.Fatalf("rejection not relayed: %+v", results)
	}
	if len(results) == 2 && strings.HasPrefix(results[1].Text, "user rejected") {
		t.Fatal("allowed Ls should have run")
	}
	if calls != 3 {
		t.Fatalf("loop did not continue after rejection: %d API calls", calls)
	}
}
