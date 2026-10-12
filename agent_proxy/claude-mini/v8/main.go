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

	"claude-code-go/v8/api"
	"claude-code-go/v8/engine"
	"claude-code-go/v8/tools"
)

const agentVersion = "0.1.0"

var (
	printMode    bool
	modelFlag    string
	maxTurnsFlag int
	verboseFlag  bool
	apiKeyFlag   string
	baseURLFlag  string
	stateFlag    string
	resumeFlag   bool
	resetFlag    bool
	sandboxFlag  string
	guardFlag    string
	protocolFlag string
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
			Model:    modelFlag,
			MaxTurns: maxTurns,
			Cwd:      sandboxFlag,
			Tools:    tools.NewWorkerRegistry(),
			Client:   client,
			Verbose:  verboseFlag,
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
		Model:     modelFlag,
		MaxTurns:  maxTurnsFlag,
		Verbose:   verboseFlag,
		StatePath: stateFlag,
		Cwd:       sandboxFlag,
		Guard:     guard,
		Tools:     tools.NewRegistry(),
		Client:    client,
	})

	if resetFlag {
		_ = os.Remove(stateFlag)
	}
	if resumeFlag {
		if err := eng.LoadState(); err != nil {
			return fmt.Errorf("failed to restore workspace state: %w", err)
		}
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
		if printMode {
			fmt.Fprintf(os.Stderr, "warn: --print mode auto-approves %s (no interactive prompt available)\n", toolName)
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
			fmt.Println(ev.Text)
		case "tool_use":
			if verboseFlag {
				fmt.Fprintf(os.Stderr, "⏺ %s(%s)\n", ev.Tool, ev.Input)
			}
		case "result":
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
