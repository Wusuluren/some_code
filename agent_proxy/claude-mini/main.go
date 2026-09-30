// Command minimal-agent is a trimmed-down Claude Code: Cobra entry +
// Anthropic Messages API client + 5 tools (Bash/Read/Write/Edit/Grep)
// + the executeQueryLoop agent loop. Built from the minimal chain of
// cmd/cli, pkg/api, internal/query and internal/tools of the main app.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"claude-code-go/minimal/api"
	"claude-code-go/minimal/engine"
	"claude-code-go/minimal/tools"
)

const agentVersion = "0.1.0"

var (
	printMode    bool
	modelFlag    string
	maxTurnsFlag int
	verboseFlag  bool
	apiKeyFlag   string
	baseURLFlag  string
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

	rootCmd.AddCommand(versionCmd)
}

func runMain(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	eng := engine.New(engine.Config{
		Model:    modelFlag,
		MaxTurns: maxTurnsFlag,
		Verbose:  verboseFlag,
		Tools:    tools.NewRegistry(),
		Client: api.NewClient(api.Config{
			APIKey:  apiKeyFlag,
			BaseURL: baseURLFlag,
		}),
	})

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

// runOnce runs one prompt through the engine and renders events.
func runOnce(ctx context.Context, eng *engine.Engine, prompt string) error {
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
