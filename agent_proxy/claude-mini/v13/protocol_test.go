package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"claude-code-go/v13/api"
	"claude-code-go/v13/engine"
	"claude-code-go/v13/tools"
)

// TestOpenAIProtocol is the v8 mechanism: the same engine and neutral
// ContentBlock model drive an OpenAI Chat Completions endpoint. The wire
// translation (tools as functions, tool results as role:"tool" messages)
// lives entirely behind the client port, so nothing above it changes.
func TestOpenAIProtocol(t *testing.T) {
	t.Chdir(t.TempDir())

	var bodies []string
	var paths []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		paths = append(paths, r.URL.Path)
		calls++
		w.Header().Set("Content-Type", "application/json")
		var resp map[string]interface{}
		if calls == 1 {
			// Ask for a tool through the Chat Completions dialect.
			resp = map[string]interface{}{"choices": []map[string]interface{}{{
				"finish_reason": "tool_calls",
				"message": map[string]interface{}{"tool_calls": []map[string]interface{}{{
					"id": "call_1", "type": "function",
					"function": map[string]interface{}{"name": "Write",
						"arguments": `{"file_path":"out.txt","content":"hi from openai"}`},
				}}},
			}}, "usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 3}}
		} else {
			resp = map[string]interface{}{"choices": []map[string]interface{}{{
				"finish_reason": "stop",
				"message":       map[string]interface{}{"content": "done"},
			}}, "usage": map[string]int{"prompt_tokens": 8, "completion_tokens": 1}}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(api.Config{APIKey: "sk-test", BaseURL: server.URL + "/v1", Protocol: "openai"})
	eng := engine.New(engine.Config{Model: "gpt-4o-mini", MaxTurns: 4, Tools: tools.NewRegistry(), Client: client})

	var final, errText string
	for ev := range eng.Run(context.Background(), "write out.txt") {
		switch ev.Type {
		case "assistant_text":
			final = ev.Text
		case "result":
			errText = ev.Text // result.Text carries a run error, empty means success
		}
	}
	if errText != "" {
		t.Fatalf("run errored: %s", errText)
	}
	if final != "done" {
		t.Fatalf("expected final 'done', got %q", final)
	}

	// The tool actually ran on the local filesystem.
	data, err := os.ReadFile("out.txt")
	if err != nil || string(data) != "hi from openai" {
		t.Fatalf("Write tool did not run through the openai wire: %v %q", err, data)
	}

	// Every request went to /chat/completions, not /messages.
	for _, p := range paths {
		if p != "/v1/chat/completions" {
			t.Fatalf("wrong endpoint for openai protocol: %s", p)
		}
	}
	// Request 1 advertises tools in the OpenAI function shape.
	if !strings.Contains(bodies[0], `"type":"function"`) || !strings.Contains(bodies[0], `"parameters"`) {
		t.Fatalf("first request not in Chat Completions tool shape: %s", bodies[0])
	}
	// Request 2 feeds the result back as a role:"tool" message keyed by call id.
	if !strings.Contains(bodies[1], `"role":"tool"`) || !strings.Contains(bodies[1], `"tool_call_id":"call_1"`) {
		t.Fatalf("follow-up missing tool-result message: %s", bodies[1])
	}
	// It must NOT leak the Anthropic-shaped tool_result block.
	if strings.Contains(bodies[1], `"tool_use_id"`) {
		t.Fatalf("anthropic block leaked into openai wire: %s", bodies[1])
	}
}
