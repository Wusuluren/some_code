package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"claude-code-go/v3/api"
	"claude-code-go/v3/engine"
	"claude-code-go/v3/tools"
)

// fakeServer answers every request with a plain end_turn text message.
func fakeServer(t *testing.T, text string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "msg", "role": "assistant", "model": "test",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
			"content":     []map[string]interface{}{{"type": "text", "text": text}},
		})
	})
	return mux
}

// TestFilesystemWorkspace covers the v1 additions: the Ls tool and
// cross-session history persistence via SaveState/LoadState.
func TestFilesystemWorkspace(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	if err := os.WriteFile("notes.md", []byte("intermediate result\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ls lists the workspace and shows the file.
	ls := tools.NewLsTool()
	out, err := ls.Call(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Ls failed: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Fatalf("Ls output missing file: %q", out)
	}

	// Run one prompt against a fake API, save state, restore into a new engine.
	server := httptest.NewServer(fakeServer(t, "answered"))
	defer server.Close()
	client := api.NewClient(api.Config{APIKey: "test-key", BaseURL: server.URL + "/v1"})
	eng := engine.New(engine.Config{Model: "test-model", MaxTurns: 2, Tools: tools.NewRegistry(), Client: client})
	for range eng.Run(context.Background(), "first prompt") {
	}
	if err := eng.SaveState(); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if _, err := os.Stat(".agent_state.json"); err != nil {
		t.Fatalf("state file not written: %v", err)
	}

	eng3 := engine.New(engine.Config{Model: "test-model", MaxTurns: 2, Tools: tools.NewRegistry(), Client: client})
	if err := eng3.LoadState(); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for range eng3.Run(context.Background(), "second prompt") {
	}
	if err := eng3.SaveState(); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// The persisted history (2 messages) plus the new prompt means the
	// second session's request must have carried the old context.
	data, _ := os.ReadFile(".agent_state.json")
	if !strings.Contains(string(data), "first prompt") || !strings.Contains(string(data), "second prompt") {
		t.Fatalf("state does not span sessions: %s", data)
	}
}
