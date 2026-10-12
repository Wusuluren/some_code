package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"claude-code-go/v14/api"
	"claude-code-go/v14/engine"
	"claude-code-go/v14/tools"
)

// TestSubagent covers the v5 addition: the Task tool delegates to an
// isolated agent session and only its final answer enters the main
// conversation.
func TestSubagent(t *testing.T) {
	// Worker registry must not allow nested spawning or experience writes.
	worker := tools.NewWorkerRegistry()
	for _, forbidden := range []string{"Task", "Memory", "Skill"} {
		if worker.Find(forbidden) != nil {
			t.Fatalf("worker registry should not expose %s", forbidden)
		}
	}
	if worker.Find("Read") == nil || len(worker.List()) != 7 {
		t.Fatalf("worker registry wrong: %d tools", len(worker.List()))
	}

	// End-to-end: main agent asks for a Task; the runner is a fake
	// sub-session that returns a summary; the tool_result carries it back.
	runnerCalled := 0
	tools.SubagentRunner = func(ctx context.Context, task string, maxTurns int) (string, error) {
		runnerCalled++
		if !strings.Contains(task, "analyze") || maxTurns != 15 {
			t.Errorf("unexpected subtask: %q, turns %d", task, maxTurns)
		}
		return "module internal/query has ~250 lines", nil
	}
	defer func() { tools.SubagentRunner = nil }()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "m1", "role": "assistant", "model": "test", "stop_reason": "tool_use",
				"usage": map[string]int{"input_tokens": 5, "output_tokens": 2},
				"content": []map[string]interface{}{{"type": "tool_use", "id": "tu_1", "name": "Task",
					"input": map[string]interface{}{"task": "analyze the query module"}},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "m2", "role": "assistant", "model": "test", "stop_reason": "end_turn",
			"usage":   map[string]int{"input_tokens": 6, "output_tokens": 1},
			"content": []map[string]interface{}{{"type": "text", "text": "delegated"}},
		})
	}))
	defer server.Close()

	eng := engine.New(engine.Config{Model: "test", MaxTurns: 4, Tools: tools.NewRegistry(),
		Client: api.NewClient(api.Config{APIKey: "k", BaseURL: server.URL + "/v1"})})
	var toolResult string
	for ev := range eng.Run(context.Background(), "split the work") {
		if ev.Type == "tool_result" {
			toolResult = ev.Text
		}
	}
	if runnerCalled != 1 {
		t.Fatalf("subagent runner calls: %d", runnerCalled)
	}
	if !strings.Contains(toolResult, "Subagent result") || !strings.Contains(toolResult, "250 lines") {
		t.Fatalf("subagent answer not relayed: %q", toolResult)
	}
}
