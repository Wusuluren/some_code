package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"claude-code-go/v8/tools"
)

// TestInterpreterPersistentSession covers the v3 addition: Repl keeps
// Python state (variables, functions) across separate tool calls.
func TestInterpreterPersistentSession(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	repl := tools.NewReplTool()
	ctx := context.Background()

	call := func(code string, reset bool) string {
		t.Helper()
		in, _ := json.Marshal(map[string]interface{}{"code": code, "reset": reset})
		out, err := repl.Call(ctx, in)
		if err != nil {
			t.Fatalf("Repl(%q): %v — %s", code, err, out)
		}
		return out
	}

	call("x = 41", false)
	out := call("print(x + 1)", false)
	if !strings.Contains(out, "42") {
		t.Fatalf("state did not persist across calls: %q", out)
	}

	// Errors are reported to the model instead of killing the session.
	in, _ := json.Marshal(map[string]interface{}{"code": "1/0"})
	errOut, err := repl.Call(ctx, in)
	if err != nil {
		t.Fatalf("error should be printed, not fail the call: %v", err)
	}
	if !strings.Contains(errOut, "ZeroDivisionError") {
		t.Fatalf("traceback missing: %q", errOut)
	}

	// reset=true drops state.
	out = call("print(x)", true)
	if !strings.Contains(out, "NameError") {
		t.Fatalf("reset did not clear state: %q", out)
	}
}
