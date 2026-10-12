package tools

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ReplTool is the Interpreter: one long-lived python3 session whose
// variables survive across tool calls, so the agent can iterate on
// analysis without re-running setup every turn. Mirrors the sandbox
// interpreter of Deep Agents.
type ReplTool struct {
	mu       sync.Mutex
	proc     *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Scanner
	lastCall time.Time
}

func NewReplTool() *ReplTool { return &ReplTool{} }

func (t *ReplTool) Name() string { return "Repl" }

func (t *ReplTool) Description() string {
	return "Runs Python code in a persistent interpreter session: variables, imports and functions defined in earlier calls stay available. Use it for multi-step data analysis and iterative debugging instead of one-off Bash heredocs. Pass reset=true to restart the session; it also reports errors instead of failing the run."
}

func (t *ReplTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"code":  strProp("Python source to execute in the persistent session"),
		"reset": map[string]interface{}{"type": "boolean", "description": "Restart the interpreter before running (drops all state)"},
	}, []string{"code"})
}

// doneSentinel must never appear in python output unless it is the marker
// itself; base64 payloads make accidental collisions practically impossible.
const doneSentinel = "<<POTETO-DONE>>"

// bootstrap reads base64-encoded snippets from stdin, execs them keeping
// globals between snippets, prints tracebacks to the shared pipe, and
// emits the sentinel when a snippet finishes.
const bootstrap = `import sys, base64, traceback
g = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        code = base64.b64decode(line).decode("utf-8")
        exec(compile(code, "<repl>", "exec"), g)
    except Exception:
        traceback.print_exc()
    except SystemExit:
        g.clear()
    print("` + doneSentinel + `", flush=True)
`

// Call executes one snippet in the persistent session and returns
// everything printed before the done sentinel.
func (t *ReplTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Code  string `json:"code"`
		Reset bool   `json:"reset"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.Code == "" {
		return "", mustErr("missing required field: code")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if in.Reset {
		t.killLocked()
	}
	if err := t.ensureLocked(); err != nil {
		return "", err
	}

	deadline := time.Now().Add(60 * time.Second)
	if dl, ok := ctx.Deadline(); ok {
		deadline = dl
		if time.Until(deadline) > 60*time.Second {
			deadline = time.Now().Add(60 * time.Second)
		}
	}
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	proc := t.proc
	go func() {
		<-callCtx.Done()
		// Kills the session on timeout only; the next Call restarts it.
		if callCtx.Err() == context.DeadlineExceeded && proc != nil && proc.Process != nil {
			_ = proc.Process.Kill()
		}
	}()

	payload := base64.StdEncoding.EncodeToString([]byte(in.Code)) + "\n"
	if _, err := t.stdin.Write([]byte(payload)); err != nil {
		t.killLocked()
		return "", fmt.Errorf("interpreter session lost: %w (will restart on next call)", err)
	}

	var out []string
	for t.stdout.Scan() {
		line := t.stdout.Text()
		if line == doneSentinel {
			t.lastCall = time.Now()
			if len(out) == 0 {
				return "(no output)", nil
			}
			return strings.Join(out, "\n") + "\n[session state kept]", nil
		}
		out = append(out, line)
	}
	killed := callCtx.Err() == context.DeadlineExceeded
	t.killLocked()
	if killed {
		return strings.Join(out, "\n"), fmt.Errorf("interpreter timed out after 60s, session reset")
	}
	if err := t.stdout.Err(); err != nil {
		return strings.Join(out, "\n"), fmt.Errorf("interpreter read error: %w", err)
	}
	return strings.Join(out, "\n"), fmt.Errorf("interpreter session ended unexpectedly, reset on next call")
}

// ensureLocked starts python3 if there is no live session. Sessions die
// only via killLocked (which nils the fields) or a broken pipe caught in
// Call, so a non-nil proc means the process is still ours to talk to.
func (t *ReplTool) ensureLocked() error {
	if t.proc != nil {
		return nil
	}
	if _, err := exec.LookPath("python3"); err != nil {
		return mustErr("python3 not found in PATH; install python3 to use Repl")
	}
	cmd := exec.Command("python3", "-u", "-c", bootstrap)
	if root := SandboxRoot(); root != "" {
		cmd.Dir = root
		cmd.Env = ScrubEnv(os.Environ())
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	// Merge stderr into the same pipe so tracebacks and prints arrive in
	// one stream and a full stderr buffer can never deadlock the child.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return err
	}
	t.proc = cmd
	t.stdin = stdin
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	t.stdout = sc
	return nil
}

// killLocked terminates the session; safe to call when there is none.
func (t *ReplTool) killLocked() {
	if t.proc != nil && t.proc.Process != nil {
		_ = t.proc.Process.Kill()
		_, _ = t.proc.Process.Wait()
	}
	t.proc = nil
	t.stdin = nil
	t.stdout = nil
}
