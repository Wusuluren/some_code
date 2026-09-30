package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&bash{}) }

type bash struct{}

const bashSchema = `{"type":"object","properties":{` +
	`"command":{"type":"string","description":"Shell command to run"},` +
	`"timeout_seconds":{"type":"integer","description":"Foreground safety cap, default 120"}},"required":["command"]}`

func (bash) Name() string            { return "bash" }
func (bash) Schema() json.RawMessage { return json.RawMessage(bashSchema) }
func (bash) ReadOnly() bool          { return false }
func (bash) Description() string {
	return "Run a shell command in the workspace and return combined stdout/stderr."
}

func (bash) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Command        string `json:"command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("bash: bad args: %w", err)
	}
	if strings.TrimSpace(in.Command) == "" {
		return "", fmt.Errorf("bash: command is required")
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 120 // [tools] bash_timeout_seconds default (SPEC §5)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(in.TimeoutSeconds)*time.Second)
	defer cancel()

	// [tools.shell] prefer = "auto": pick the platform-native interpreter.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", in.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", in.Command)
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if len(text) > maxOutputText {
		text = text[:maxOutputText] + "\n... [output truncated]"
	}
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("bash: timed out after %d seconds", in.TimeoutSeconds)
	}
	if err != nil {
		// Non-zero exit is fed back to the model, not fatal (SPEC §6).
		return text, fmt.Errorf("bash: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return "(command succeeded with no output)", nil
	}
	return text, nil
}
