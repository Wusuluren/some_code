package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"claude-code-go/v11/api"
	"claude-code-go/v11/engine"
	"claude-code-go/v11/tools"
)

// TestStreamingSSE is the v9 mechanism: the reply arrives as Server-Sent
// Events and text deltas stream out as they land, while a tool_use split
// across several input_json_delta chunks is still reassembled whole so the
// engine executes it normally.
func TestStreamingSSE(t *testing.T) {
	t.Chdir(t.TempDir())

	var bodies []string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		sse := func(lines ...string) {
			for _, l := range lines {
				fmt.Fprintf(w, "data: %s\n\n", l)
			}
		}
		if calls == 1 {
			// A Write call whose JSON input arrives in two fragments.
			sse(
				`{"type":"message_start","message":{"usage":{"input_tokens":3}}}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu1","name":"Write"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"s.txt\""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":",\"content\":\"streamed\"}"}}`,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`,
				`{"type":"message_stop"}`,
			)
		} else {
			// Final answer split into two text deltas.
			sse(
				`{"type":"message_start","message":{"usage":{"input_tokens":7}}}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"done"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}`,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
				`{"type":"message_stop"}`,
			)
		}
	}))
	defer server.Close()

	client := api.NewClient(api.Config{APIKey: "k", BaseURL: server.URL + "/v1"})
	eng := engine.New(engine.Config{Model: "test", MaxTurns: 4, Stream: true, Tools: tools.NewRegistry(), Client: client})

	var deltas []string
	var errText string
	for ev := range eng.Run(context.Background(), "write s.txt") {
		switch ev.Type {
		case "assistant_text":
			deltas = append(deltas, ev.Text)
		case "result":
			errText = ev.Text
		}
	}
	if errText != "" {
		t.Fatalf("stream run errored: %s", errText)
	}
	// Streaming proves itself: the final turn came out in >1 text delta, not one blob.
	if len(deltas) < 2 || strings.Join(deltas, "") != "done!" {
		t.Fatalf("expected incremental deltas joining to 'done!', got %q", deltas)
	}
	// The split tool input was reassembled and executed.
	if data, err := os.ReadFile("s.txt"); err != nil || string(data) != "streamed" {
		t.Fatalf("streamed tool_use did not run: %v %q", err, data)
	}
	// Both requests asked for streaming.
	for _, b := range bodies {
		if !strings.Contains(b, `"stream":true`) {
			t.Fatalf("request did not set stream=true: %s", b)
		}
	}
}
