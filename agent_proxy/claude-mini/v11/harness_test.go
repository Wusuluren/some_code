package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"claude-code-go/v11/api"
	"claude-code-go/v11/engine"
	"claude-code-go/v11/tools"
)

// TestFullHarness is the v7 capstone: one prompt exercises the whole
// agent harness at once — workspace tools (Filesystem), a gated Task
// call (Human-in-the-loop -> Subagent), and Memory (experience layer).
// Each mechanism was introduced alone in v1..v6; here they must compose.
func TestFullHarness(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AGENT_HOME", t.TempDir())

	var subagentRan int32
	tools.SubagentRunner = func(ctx context.Context, task string, maxTurns int) (string, error) {
		atomic.StoreInt32(&subagentRan, 1)
		return "analysis done: 3 hotspots", nil
	}
	defer func() { tools.SubagentRunner = nil }()

	// The fake model: delegate via Task, save a memory, then finish.
	var reqs []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqs = append(reqs, string(body))
		calls++
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "m", "role": "assistant", "model": "test", "stop_reason": "tool_use",
			"usage": map[string]int{"input_tokens": 1, "output_tokens": 1},
		}
		switch calls {
		case 1:
			resp["content"] = []map[string]interface{}{{"type": "tool_use", "id": "tu_1", "name": "Task",
				"input": map[string]interface{}{"task": "analyze the repo"}}}
		case 2:
			resp["content"] = []map[string]interface{}{{"type": "tool_use", "id": "tu_2", "name": "Memory",
				"input": map[string]interface{}{"action": "save", "fact": "repo has 3 hotspots"}}}
		default:
			resp["stop_reason"] = "end_turn"
			resp["content"] = []map[string]interface{}{{"type": "text", "text": "all mechanisms used"}}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	approved := 0
	eng := engine.New(engine.Config{Model: "test", MaxTurns: 6, Tools: tools.NewRegistry(),
		Client: api.NewClient(api.Config{APIKey: "k", BaseURL: server.URL + "/v1"}),
		Guard: func(ctx context.Context, name string, input json.RawMessage) (bool, string) {
			if name == "Task" || name == "Write" || name == "Edit" || name == "Bash" {
				approved++ // the human clicks "allow"
			}
			return true, ""
		}})

	var final string
	for ev := range eng.Run(context.Background(), "analyze the repo and remember the findings") {
		if ev.Type == "result" {
			final = ev.Text
		}
	}
	if final != "" {
		t.Fatalf("harness run errored: %s", final)
	}
	if atomic.LoadInt32(&subagentRan) != 1 {
		t.Fatal("subagent (Task) never ran")
	}
	if approved != 1 { // only the Task call was in the guarded set
		t.Fatalf("guard approvals: %d", approved)
	}
	// Memory persisted the lesson for the next session.
	if mem := tools.ReadMemory(); !strings.Contains(mem, "3 hotspots") {
		t.Fatalf("memory not persisted: %q", mem)
	}
	// The last request carried the tool results back to the model.
	if len(reqs) < 2 || !strings.Contains(reqs[len(reqs)-1], "analysis done") {
		t.Fatalf("subagent answer missing from follow-up request")
	}
}
