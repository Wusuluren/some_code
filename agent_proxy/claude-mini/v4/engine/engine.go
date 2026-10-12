package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"claude-code-go/v4/api"
	"claude-code-go/v4/tools"
)

// DefaultModel is used when the user does not pass --model.
const DefaultModel = "claude-sonnet-4-20250514"

// Config configures the query engine.
type Config struct {
	Model    string
	MaxTurns int
	Cwd      string
	Tools    *tools.Registry
	Client   *api.Client
	// Verbose prints per-tool execution lines to stderr.
	Verbose bool
	// StatePath is the file where conversation history persists across
	// sessions (the long-term workspace). Default ".agent_state.json".
	StatePath string
}

// Event is a message emitted by the loop as it progresses.
type Event struct {
	// Type is "assistant_text", "tool_use", "tool_result" or "result".
	Type    string
	Text    string
	Tool    string
	Input   string
	Usage   api.Usage
	Turns   int
	Elapsed time.Duration
}

// Engine owns the conversation state and the agent loop,
// mirroring internal/query.QueryEngine.
type Engine struct {
	config  Config
	history []api.Message
	usage   api.Usage
}

// New creates a query engine.
func New(config Config) *Engine {
	if config.Model == "" {
		config.Model = DefaultModel
	}
	if config.MaxTurns <= 0 {
		config.MaxTurns = 100
	}
	if config.Cwd == "" {
		config.Cwd, _ = os.Getwd()
	}
	if config.StatePath == "" {
		config.StatePath = ".agent_state.json"
	}
	return &Engine{config: config}
}

// Run executes the agent loop for one user prompt, emitting events on the
// returned channel. The channel is closed when the loop finishes.
func (e *Engine) Run(ctx context.Context, prompt string) <-chan Event {
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		e.executeQueryLoop(ctx, prompt, out)
	}()
	return out
}

// executeQueryLoop runs the main agent loop:
//
//	for each turn: call API -> append assistant message -> if tool_use blocks
//	exist, execute tools and append tool_result messages; otherwise stop.
func (e *Engine) executeQueryLoop(ctx context.Context, prompt string, out chan<- Event) {
	start := time.Now()

	e.history = append(e.history, api.Message{
		Role:    "user",
		Content: []api.ContentBlock{{Type: "text", Text: prompt}},
	})

	turnCount := 0
	for turnCount < e.config.MaxTurns {
		if ctx.Err() != nil {
			out <- Event{Type: "result", Text: "interrupted", Turns: turnCount, Elapsed: time.Since(start)}
			return
		}

		req := api.MessageRequest{
			Model:     e.config.Model,
			MaxTokens: 8192,
			System:    e.buildSystemPrompt(),
			Messages:  e.history,
			Tools:     e.toolDefinitions(),
		}

		resp, err := e.config.Client.CreateMessage(ctx, req)
		if err != nil {
			out <- Event{Type: "result", Text: "error: " + err.Error(), Turns: turnCount, Elapsed: time.Since(start)}
			return
		}

		e.usage.InputTokens += resp.Usage.InputTokens
		e.usage.OutputTokens += resp.Usage.OutputTokens

		// Persist the assistant turn into history verbatim.
		e.history = append(e.history, api.Message{Role: "assistant", Content: resp.Content})

		for _, block := range resp.Content {
			if block.Type == "text" && block.Text != "" {
				out <- Event{Type: "assistant_text", Text: block.Text}
			}
		}

		toolUses := extractToolUseBlocks(resp.Content)
		if len(toolUses) == 0 {
			out <- Event{Type: "result", Usage: e.usage, Turns: turnCount + 1, Elapsed: time.Since(start)}
			return
		}

		e.executeTools(ctx, toolUses, out)
		turnCount++
	}

	out <- Event{Type: "result", Text: fmt.Sprintf("max turns (%d) reached", e.config.MaxTurns), Usage: e.usage, Turns: turnCount, Elapsed: time.Since(start)}
}

// executeTools runs every requested tool call and appends one user message
// holding all tool_result blocks, as the Messages API requires.
func (e *Engine) executeTools(ctx context.Context, blocks []api.ContentBlock, out chan<- Event) {
	results := make([]api.ContentBlock, 0, len(blocks))

	for _, block := range blocks {
		out <- Event{Type: "tool_use", Tool: block.Name, Input: string(block.Input)}

		content, isErr := e.runOneTool(ctx, block)
		if e.config.Verbose {
			preview := content
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Fprintf(os.Stderr, "[tool %s] %s\n", block.Name, preview)
		}

		results = append(results, api.ContentBlock{
			Type:      "tool_result",
			ToolUseID: block.ID,
			Content:   content,
			IsError:   isErr,
		})

		out <- Event{Type: "tool_result", Tool: block.Name, Text: content}
	}

	e.history = append(e.history, api.Message{Role: "user", Content: results})
}

func (e *Engine) runOneTool(ctx context.Context, block api.ContentBlock) (string, bool) {
	tool := e.config.Tools.Find(block.Name)
	if tool == nil {
		return fmt.Sprintf("Unknown tool: %s", block.Name), true
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	text, err := tool.Call(callCtx, block.Input)
	if err != nil {
		if text != "" {
			return text + "\nerror: " + err.Error(), true
		}
		return "error: " + err.Error(), true
	}
	if text == "" {
		text = "(no output)"
	}
	return text, false
}

// toolDefinitions converts the registry into API tool definitions.
func (e *Engine) toolDefinitions() []api.ToolDefinition {
	var defs []api.ToolDefinition
	for _, t := range e.config.Tools.List() {
		defs = append(defs, api.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// buildSystemPrompt builds a compact system prompt, mirroring
// constants.BuildSystemPrompt in the main app.
func (e *Engine) buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are Claude Code, a coding assistant that runs in the user's terminal, powered by the Anthropic API.\n")
	b.WriteString("You accomplish tasks by using your tools directly; be concise and factual.\n\n")
	fmt.Fprintf(&b, "# Environment\n- Working directory: %s\n- Platform: %s\n- Shell: %s\n\n",
		e.config.Cwd, runtime.GOOS, shellOrEmpty())

	b.WriteString("# Tool use\n")
	b.WriteString("- Your workspace (cwd) is persistent: intermediate results saved with Write stay available across sessions, use Ls to explore it.\n")
	b.WriteString("- Use Read before Edit; Edit requires the exact text present in the file.\n")
	b.WriteString("- Prefer specialized tools (Read/Write/Edit/Grep) over Bash equivalents (cat/sed/grep).\n")
	b.WriteString("- For multi-step computation or data analysis, use Repl: variables persist across calls, so you can iterate without redoing setup.\n")
	b.WriteString("- Run independent tool calls together in one turn when possible.\n")
	b.WriteString("- When the task is done, reply with a short final answer without further tool calls.\n")

	// v4 Memory + Skills: inject the durable experience layer every turn.
	if mem := tools.ReadMemory(); mem != "" {
		b.WriteString("\n# Long-term memory (saved by you across sessions)\n")
		b.WriteString(mem + "\n")
	}
	if skills := tools.SkillIndex(); skills != "" {
		b.WriteString("\n# Available skills (use the Skill tool with action=use to load a recipe)\n")
		b.WriteString(skills + "\n")
	}
	b.WriteString("\nSave durable facts with Memory and reusable procedures with Skill when a task teaches you something.\n")
	return b.String()
}

func shellOrEmpty() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

func extractToolUseBlocks(content []api.ContentBlock) []api.ContentBlock {
	var blocks []api.ContentBlock
	for _, block := range content {
		if block.Type == "tool_use" {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// SaveState persists the conversation history to Config.StatePath so the
// workspace survives across sessions.
func (e *Engine) SaveState() error {
	data, err := json.Marshal(e.history)
	if err != nil {
		return err
	}
	return os.WriteFile(e.config.StatePath, data, 0o644)
}

// LoadState restores conversation history saved by SaveState.
func (e *Engine) LoadState() error {
	data, err := os.ReadFile(e.config.StatePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &e.history)
}

// PrettyInput formats a tool input JSON for display.
func PrettyInput(raw json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	data, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(data)
}
