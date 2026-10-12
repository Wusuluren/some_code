package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-code-go/v5/tools"
)

// TestSandbox covers the v2 additions: every file tool and Bash is
// confined to the sandbox root and the shell env is scrubbed.
func TestSandbox(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("do not read\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools.SetSandboxRoot(root)
	defer tools.SetSandboxRoot("")

	// Write inside the box works.
	res, err := tools.NewWriteTool().Call(context.Background(),
		json.RawMessage(`{"file_path":"work/a.txt","content":"hi"}`))
	if err != nil || !strings.Contains(res, "created") {
		t.Fatalf("write inside sandbox failed: %q, %v", res, err)
	}

	// Write outside the box is rejected.
	if _, err := tools.NewWriteTool().Call(context.Background(),
		json.RawMessage(`{"file_path":"`+outside+`","content":"x"}`)); err == nil {
		t.Fatal("write outside sandbox should fail")
	}

	// Read outside the box is rejected (this was unguarded in v1).
	if _, err := tools.NewReadTool().Call(context.Background(),
		json.RawMessage(`{"file_path":"`+outside+`"}`)); err == nil {
		t.Fatal("read outside sandbox should fail")
	}

	// Bash runs with a scrubbed environment: no credentials leak.
	os.Setenv("ANTHROPIC_API_KEY", "sk-leak-me")
	out, err := tools.NewBashTool().Call(context.Background(),
		json.RawMessage(`{"command":"env | grep -c ANTHROPIC_API_KEY || true; pwd"}`))
	if err != nil {
		t.Fatalf("bash in sandbox: %v", err)
	}
	if strings.Contains(out, "sk-leak-me") {
		t.Fatal("credential leaked into sandboxed shell")
	}
	if !strings.Contains(out, filepath.Base(root)) {
		t.Fatalf("bash did not run inside sandbox dir: %q", out)
	}

	// ScrubEnv keeps normal vars and drops sensitive ones.
	kept := tools.ScrubEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=k", "HOME=/h"})
	if len(kept) != 2 || kept[0] != "PATH=/bin" || kept[1] != "HOME=/h" {
		t.Fatalf("ScrubEnv wrong: %v", kept)
	}
}
