// Command minimal-agent is a trimmed-down Claude Code: Cobra entry +
// Anthropic Messages API client + 5 tools (Bash/Read/Write/Edit/Grep)
// + the executeQueryLoop agent loop. Built from the minimal chain of
// cmd/cli, pkg/api, internal/query and internal/tools of the main app.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"claude-code-go/v15/api"
	"claude-code-go/v15/engine"
	"claude-code-go/v15/mcp"
	"claude-code-go/v15/sessions"
	"claude-code-go/v15/tools"
	"claude-code-go/v15/ui"
)

const agentVersion = "0.1.0"

var (
	printMode       bool
	modelFlag       string
	maxTurnsFlag    int
	verboseFlag     bool
	apiKeyFlag      string
	baseURLFlag     string
	stateFlag       string
	resumeFlag      bool
	resetFlag       bool
	sandboxFlag     string
	guardFlag       string
	protocolFlag    string
	streamFlag      bool
	streamOpen      bool
	mcpFlag         string
	contextFlag     int
	listFlag        bool
	continueFlag    bool
	sessionFlag     string
	sessionsDirFlag string
	tuiFlag         bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "minimal-agent [flags] [prompt]",
	Short: "Minimal Claude Code agent: API loop + 5 tools",
	Long: `minimal-agent is a trimmed-down coding agent built from the minimal
chain of claude-code-go: Cobra entry, net/http Anthropic Messages API
client, Tool interface with Bash/Read/Write/Edit/Grep, and the
executeQueryLoop main loop.

Examples:
  minimal-agent --print "list the go files and summarize the project"
  minimal-agent                      # interactive REPL (one prompt per line)`,
	Args: cobra.ArbitraryArgs,
	RunE: runMain,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("minimal-agent v%s\n", agentVersion)
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&printMode, "print", "p", false, "Print mode: run one prompt non-interactively and exit")
	rootCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Model to use (default "+engine.DefaultModel+")")
	rootCmd.Flags().IntVar(&maxTurnsFlag, "max-turns", 100, "Maximum number of agent turns")
	rootCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Print per-tool execution lines to stderr")
	rootCmd.Flags().StringVar(&apiKeyFlag, "api-key", "", "Anthropic API key (falls back to ANTHROPIC_API_KEY)")
	rootCmd.Flags().StringVar(&baseURLFlag, "base-url", "", "API base URL (falls back to ANTHROPIC_BASE_URL)")
	rootCmd.Flags().StringVar(&stateFlag, "state", ".agent_state.json", "Workspace state file for cross-session persistence")
	rootCmd.Flags().BoolVar(&resumeFlag, "resume", false, "Restore conversation history from the state file")
	rootCmd.Flags().BoolVar(&resetFlag, "reset", false, "Delete the workspace state file before starting")
	rootCmd.Flags().StringVar(&sandboxFlag, "sandbox", "", "Confine all file tools and Bash to this directory (created if missing), with credentials stripped from the shell environment")
	rootCmd.Flags().StringVar(&guardFlag, "guard-tools", "Bash,Write,Edit", "Comma-separated tools that need explicit human approval before running (human-in-the-loop); empty disables prompting")
	rootCmd.Flags().StringVar(&protocolFlag, "protocol", "anthropic", "Wire protocol dialect: anthropic (Messages) or openai (Chat Completions)")
	rootCmd.Flags().BoolVar(&streamFlag, "stream", false, "Stream the model's reply over SSE and print tokens as they arrive")
	rootCmd.Flags().StringVar(&mcpFlag, "mcp", ".mcp.json", "MCP servers config (JSON, {mcpServers:{...}}); missing file disables MCP")
	rootCmd.Flags().IntVar(&contextFlag, "context-size", 0, "Trim the sent history when its token estimate exceeds this (0 = no cap)")
	rootCmd.Flags().BoolVar(&listFlag, "list", false, "List this project's saved sessions and exit")
	rootCmd.Flags().BoolVarP(&continueFlag, "continue", "c", false, "Resume the newest session for this project")
	rootCmd.Flags().StringVar(&sessionFlag, "session", "", "Resume a session by id or id prefix")
	rootCmd.Flags().StringVar(&sessionsDirFlag, "sessions-dir", "", "Override the sessions directory (default: per-project under ~/.v13-agent/sessions)")
	rootCmd.Flags().BoolVar(&tuiFlag, "tui", false, "Run a Bubble Tea terminal UI that renders the event stream as a live transcript")

	rootCmd.AddCommand(versionCmd)
}

func runMain(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if sandboxFlag != "" {
		if err := os.MkdirAll(sandboxFlag, 0o755); err != nil {
			return fmt.Errorf("failed to create sandbox: %w", err)
		}
		root, err := filepath.Abs(sandboxFlag)
		if err != nil {
			return err
		}
		tools.SetSandboxRoot(root)
	}

	// v13 session store: one timestamped session per project, listable and
	// resumable. Persistence routes through it unless the user pinned --state.
	store := sessions.Store{Dir: sessionsDir()}
	if listFlag {
		return listSessions(store)
	}
	switch {
	case sessionFlag != "":
		id, err := store.Resolve(sessionFlag)
		if err != nil {
			return err
		}
		stateFlag, resumeFlag = store.Path(id), true
	case continueFlag:
		if id, ok := store.Newest(); ok {
			stateFlag, resumeFlag = store.Path(id), true
		}
	case !cmd.Flags().Changed("state"):
		stateFlag = store.Path(store.NextID())
	}

	client := api.NewClient(api.Config{
		APIKey:   apiKeyFlag,
		BaseURL:  baseURLFlag,
		Protocol: protocolFlag,
	})
	// The openai dialect needs an OpenAI model name when none was given.
	if protocolFlag == "openai" && modelFlag == "" {
		modelFlag = "gpt-4o-mini"
	}

	// v6 human-in-the-loop: ask before dangerous operations.
	guard := buildGuard()

	// v5 Subagent wiring: each Task runs a fresh Engine with a fresh
	// context and only the worker tools, and reports back its final text.
	tools.SubagentRunner = func(ctx context.Context, task string, maxTurns int) (string, error) {
		worker := engine.New(engine.Config{
			Model:            modelFlag,
			MaxTurns:         maxTurns,
			Cwd:              sandboxFlag,
			Tools:            tools.NewWorkerRegistry(),
			Client:           client,
			Verbose:          verboseFlag,
			MaxContextTokens: contextFlag,
		})
		var final strings.Builder
		var lastErr string
		for ev := range worker.Run(ctx, task) {
			switch ev.Type {
			case "assistant_text":
				final.Reset()
				final.WriteString(ev.Text)
			case "result":
				// A clean finish leaves Text empty; anything else is
				// an error / max-turns / interrupt notice.
				if ev.Text != "" {
					lastErr = ev.Text
				}
			}
		}
		if lastErr != "" && lastErr != "interrupted" {
			return final.String(), fmt.Errorf("subagent stopped: %s", lastErr)
		}
		return final.String(), nil
	}

	eng := engine.New(engine.Config{
		Model:            modelFlag,
		MaxTurns:         maxTurnsFlag,
		Verbose:          verboseFlag,
		StatePath:        stateFlag,
		Cwd:              sandboxFlag,
		Guard:            guard,
		Stream:           streamFlag,
		MaxContextTokens: contextFlag,
		Tools:            mcpRegistry(ctx),
		Client:           client,
	})

	if resetFlag {
		_ = os.Remove(stateFlag)
	}
	if resumeFlag {
		if err := eng.LoadState(); err != nil {
			return fmt.Errorf("failed to restore workspace state: %w", err)
		}
	}

	// TUI mode: a Bubble Tea front-end over the same engine event channel.
	if tuiFlag {
		defer func() {
			if err := eng.SaveState(); err != nil {
				fmt.Fprintf(os.Stderr, "warn: save workspace state: %v\n", err)
			}
		}()
		return ui.RunTUI(ctx, eng, strings.Join(args, " "))
	}

	// Print mode: one prompt, stream events to stdout, exit.
	if printMode {
		if len(args) == 0 {
			return fmt.Errorf("no prompt provided in print mode")
		}
		return runOnce(ctx, eng, strings.Join(args, " "))
	}

	// Interactive mode: minimal REPL over stdin.
	fmt.Printf("minimal-agent v%s — type a prompt (Ctrl+D to exit)\n", agentVersion)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		if err := runOnce(ctx, eng, line); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
	}
}

// mcpRegistry builds the tool registry and layers any MCP server tools on
// top (v10). A missing config disables MCP; a server that fails to start or
// handshake is skipped so it never costs the session. Servers stay alive for
// the process lifetime and are reaped when ctx is cancelled at exit.
func mcpRegistry(ctx context.Context) *tools.Registry {
	reg := tools.NewRegistry()
	if mcpFlag == "" {
		return reg
	}
	if _, err := os.Stat(mcpFlag); err != nil {
		return reg
	}
	cfgs, err := mcp.LoadConfig(mcpFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: mcp config: %v\n", err)
		return reg
	}
	mgr := mcp.Connect(ctx, cfgs)
	for _, t := range mgr.Tools() {
		reg.Register(t)
	}
	if verboseFlag {
		fmt.Fprintf(os.Stderr, "mcp: %d server tools registered from %s\n", len(mgr.Tools()), mcpFlag)
	}
	return reg
}

// sessionsDir resolves the per-project session directory: an explicit flag,
// then an env override, then ~/.v13-agent/sessions/<project key>, falling back
// to a local dir when no home is available.
func sessionsDir() string {
	if sessionsDirFlag != "" {
		return sessionsDirFlag
	}
	if e := os.Getenv("AGENT_SESSIONS_DIR"); e != "" {
		return e
	}
	cwd, _ := os.Getwd()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".v13-agent", "sessions", sessions.ProjectKey(cwd))
	}
	return filepath.Join(cwd, ".agent_sessions")
}

func listSessions(store sessions.Store) error {
	list, _ := store.List()
	if len(list) == 0 {
		fmt.Printf("no sessions for this project (%s)\n", store.Dir)
		return nil
	}
	fmt.Printf("Sessions in %s:\n", store.Dir)
	for _, s := range list {
		preview := s.Preview
		if preview == "" {
			preview = "(empty)"
		}
		fmt.Printf("  %s  %s  %s\n", s.ID, s.Created.Format("2006-01-02 15:04"), preview)
	}
	return nil
}

// buildGuard returns the human-in-the-loop gate for the engine: guarded
// tools prompt on stdin until the user approves, rejects, or chooses
// "always" for the rest of the session.
func buildGuard() func(context.Context, string, json.RawMessage) (bool, string) {
	if guardFlag == "" {
		return nil
	}
	guarded := map[string]bool{}
	for _, name := range strings.Split(guardFlag, ",") {
		if n := strings.TrimSpace(name); n != "" {
			guarded[n] = true
		}
	}
	var (
		guardScanner = bufio.NewScanner(os.Stdin)
		always       = map[string]bool{}
	)
	return func(ctx context.Context, toolName string, input json.RawMessage) (bool, string) {
		if !guarded[toolName] || always[toolName] {
			return true, ""
		}
		if printMode || tuiFlag {
			fmt.Fprintf(os.Stderr, "warn: %s mode auto-approves %s (no interactive prompt available)\n", modeName(), toolName)
			return true, ""
		}
		for {
			fmt.Fprintf(os.Stderr, "\n⚠  %s %s\nallow? [y]es / [N]o / [a]lways: ", toolName, strings.TrimSpace(string(input)))
			if !guardScanner.Scan() {
				return false, "no answer from user (stdin closed), call skipped"
			}
			switch strings.ToLower(strings.TrimSpace(guardScanner.Text())) {
			case "y", "yes":
				return true, ""
			case "a":
				always[toolName] = true
				return true, ""
			default:
				return false, "user denied this operation"
			}
		}
	}
}

// modeName labels the non-interactive front-end for guard warnings.
func modeName() string {
	if tuiFlag {
		return "--tui"
	}
	return "--print"
}

// closeStreamLine ends an inline-streamed assistant line so tool/result notes
// start on their own line (only relevant when --stream is on).
func closeStreamLine() {
	if streamOpen {
		fmt.Println()
		streamOpen = false
	}
}

// runOnce runs one prompt through the engine and renders events.
func runOnce(ctx context.Context, eng *engine.Engine, prompt string) error {
	defer func() {
		if err := eng.SaveState(); err != nil {
			fmt.Fprintf(os.Stderr, "warn: save workspace state: %v\n", err)
		}
	}()
	for ev := range eng.Run(ctx, prompt) {
		switch ev.Type {
		case "assistant_text":
			if streamFlag {
				fmt.Print(ev.Text)
				if ev.Text != "" {
					streamOpen = true
				}
			} else {
				fmt.Println(ev.Text)
			}
		case "tool_use":
			closeStreamLine()
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "⏺ %s(%s)\n", ev.Tool, ev.Input)
			}
		case "result":
			closeStreamLine()
			if ev.Text != "" && ev.Text != "interrupted" {
				// error / max-turns termination notice
				fmt.Fprintln(os.Stderr, ev.Text)
			}
			fmt.Fprintf(os.Stderr, "── %d turns, %s, %d input / %d output tokens ──\n",
				ev.Turns, ev.Elapsed.Round(100*time.Millisecond), ev.Usage.InputTokens, ev.Usage.OutputTokens)
			if ev.Text == "interrupted" || strings.HasPrefix(ev.Text, "error") || strings.HasPrefix(ev.Text, "max turns") {
				return fmt.Errorf("%s", ev.Text)
			}
		}
	}
	return nil
}
