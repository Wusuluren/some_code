package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SubagentRunner executes one isolated agent session for a subtask and
// returns only its final text. main.go wires this to a fresh
// engine.Engine; keeping it a function variable avoids an import cycle
// (engine -> tools -> engine). Mirrors the Task tool of the main app.
var SubagentRunner func(ctx context.Context, task string, maxTurns int) (string, error)

// TaskTool spawns a subagent with its own fresh context to do a
// self-contained subtask (code analysis, tests, docs) and report back.
type TaskTool struct{}

func NewTaskTool() *TaskTool { return &TaskTool{} }

func (t *TaskTool) Name() string { return "Task" }

func (t *TaskTool) Description() string {
	return "Delegates a self-contained subtask to a fresh subagent with its own context window (code analysis, running tests, documentation). The subagent works independently and returns only its final answer, keeping your context clean. Give it a complete task description: it cannot see this conversation."
}

func (t *TaskTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"task":        strProp("Full instructions for the subagent, self-contained"),
		"max_turns":   map[string]interface{}{"type": "integer", "description": "Optional turn budget for the subagent (default 15)"},
		"description": strProp("Short label for the subtask"),
	}, []string{"task"})
}

func (t *TaskTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	if SubagentRunner == nil {
		return "", mustErr("subagents are not wired up")
	}
	var in struct {
		Task     string `json:"task"`
		MaxTurns int    `json:"max_turns"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if strings.TrimSpace(in.Task) == "" {
		return "", mustErr("missing required field: task")
	}
	if in.MaxTurns <= 0 {
		in.MaxTurns = 15
	}
	out, err := SubagentRunner(ctx, in.Task, in.MaxTurns)
	if err != nil {
		return out, err
	}
	if out == "" {
		out = "(subagent returned no text)"
	}
	return "Subagent result:\n" + out, nil
}
