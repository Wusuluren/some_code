package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// BashTool executes shell commands, mirroring internal/tools.BashTool.
type BashTool struct{}

func NewBashTool() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string { return "Bash" }

func (t *BashTool) Description() string {
	return "Executes a bash command and returns its output. The command runs in the current working directory. Use this for running tests, git commands, package managers, and other shell operations."
}

func (t *BashTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"command":     strProp("The bash command to execute"),
		"timeout_ms":  map[string]interface{}{"type": "integer", "description": "Optional timeout in milliseconds (default 120000, max 600000)"},
		"description": strProp("Short description of what this command does"),
	}, []string{"command"})
}

func (t *BashTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Command   string `json:"command"`
		TimeoutMs int    `json:"timeout_ms"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.Command == "" {
		return "", mustErr("missing required field: command")
	}

	timeout := time.Duration(in.TimeoutMs) * time.Millisecond
	if in.TimeoutMs <= 0 {
		timeout = 120 * time.Second
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell := "/bin/sh"
	if s := os.Getenv("SHELL"); s != "" {
		shell = s
	}

	cmd := exec.CommandContext(execCtx, shell, "-c", in.Command)
	if cwd, err := os.Getwd(); err == nil {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	text := string(output)
	if execCtx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("command timed out after %s", timeout)
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("command failed: %w", err)
	}
	return text, nil
}
