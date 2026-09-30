// Command reasonix-mini is the minimal compiling skeleton of the Reasonix
// harness: config-driven providers (Step 0), the Provider interface with an
// OpenAI-compatible implementation (Step 1), the Tool interface with eight
// built-in tools (Step 2), and the agent loop behind a permission gate
// (Step 3). Only cli/main decides exit codes (SPEC §6).
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"reasonix-mini/internal/agent"
	"reasonix-mini/internal/config"
	"reasonix-mini/internal/permission"
	"reasonix-mini/internal/provider"
	"reasonix-mini/internal/tool"

	// Blank imports self-register the concrete kinds at init (SPEC §2:
	// entry imports the leaves; the registry core stays implementation-free).
	_ "reasonix-mini/internal/provider/openai"
	_ "reasonix-mini/internal/tool/builtin"
)

func main() {
	modelFlag := flag.String("model", "", "model reference: provider name, bare model, or provider/model")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `usage: reasonix-mini [flags] [command]

Commands:
  (none)            start an interactive session
  run "<prompt>"    one-shot headless run (Ask resolves to allow)
  setup             write a starter config to ~/.reasonix-mini/config.toml

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(modelFlag); err != nil {
		fmt.Fprintf(os.Stderr, "reasonix-mini: %v\n", err)
		os.Exit(1)
	}
}

func run(modelFlag *string) error {
	if flag.NArg() > 0 && flag.Arg(0) == "setup" {
		return setup()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ref := *modelFlag
	if ref == "" {
		ref = cfg.DefaultModel
	}
	p, model, err := cfg.ResolveModel(ref)
	if err != nil {
		return err
	}
	if p.APIKeyEnv != "" && p.APIKey() == "" {
		return fmt.Errorf("provider %q needs $%s to be set", p.Name, p.APIKeyEnv)
	}

	// Step 1 assembly: config instance → registered kind → live provider.
	prov, err := provider.New(p.Kind, provider.Config{
		Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL,
		Model: model, APIKey: p.APIKey(), ContextWindow: p.ContextWindow,
	})
	if err != nil {
		return err
	}

	// Step 2 assembly: per-run registry = builtins filtered by config.
	enabled := map[string]bool{}
	for _, name := range cfg.Tools.Enabled {
		enabled[name] = true
	}
	var tools []tool.Tool
	for _, t := range tool.Builtins() {
		if len(enabled) == 0 || enabled[t.Name()] {
			tools = append(tools, t)
		}
	}
	reg, err := tool.NewRegistry(tools)
	if err != nil {
		return err
	}

	a := &agent.Agent{
		Provider:    prov,
		Tools:       reg,
		Policy:      permission.FromConfig(cfg.Permissions.Mode, cfg.Permissions.Allow, cfg.Permissions.Ask, cfg.Permissions.Deny),
		Grants:      permission.NewSessionGrants(),
		System:      cfg.SystemPrompt(), // built once; byte-stable across turns (cache-first)
		Temperature: cfg.Agent.Temperature,
		MaxSteps:    cfg.Agent.MaxSteps,
	}

	ctx, stop := signalContext()
	defer stop()

	if flag.NArg() > 0 && flag.Arg(0) == "run" {
		if flag.NArg() < 2 {
			return fmt.Errorf("run needs a prompt: reasonix-mini run \"...\"")
		}
		a.ApproveLine = nil // headless contract: no prompting, ordinary Ask→allow
		return a.Run(ctx, flag.Arg(1), os.Stdout)
	}
	return repl(ctx, a, os.Stdin, os.Stdout)
}

// repl is the minimal interactive loop. The same stdin reader serves chat
// input and permission approvals, so the approval prompt is handed to a
// shared line reader (the parent repo does this inside its Bubble Tea TUI).
func repl(ctx context.Context, a *agent.Agent, stdin io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	readLine := func(prompt string) (string, error) {
		fmt.Fprint(out, prompt)
		if !sc.Scan() {
			return "", io.EOF
		}
		return sc.Text(), nil
	}
	a.ApproveLine = func(prompt string) (string, error) { return readLine(prompt) }

	fmt.Fprintf(out, "reasonix-mini — interactive session (Ctrl-D or /exit to quit)\n")
	fmt.Fprintf(out, "commands: /help /new /exit   (the real TUI, @refs and skills are the parent repo's)\n")
	for {
		input, err := readLine("\n> ")
		if err == io.EOF {
			return nil // Ctrl-D: clean exit
		}
		if err != nil {
			return err
		}
		input = strings.TrimSpace(input)
		switch {
		case input == "":
			continue
		case input == "/exit" || input == "/quit":
			return nil
		case input == "/help":
			fmt.Fprintln(out, "/new starts a fresh session; anything else is sent to the model.")
			continue
		case input == "/new":
			a.Reset()
			fmt.Fprintln(out, "(new session)")
			continue
		case strings.HasPrefix(input, "/"):
			fmt.Fprintf(out, "unknown command %s (try /help)\n", input)
			continue
		}
		if err := a.Run(ctx, input, out); err != nil {
			if ctx.Err() != nil {
				return nil // Ctrl-C during a turn: abort back to the prompt
			}
			fmt.Fprintf(out, "\n[error] %v\n", err) // one failed turn does not kill the session
		}
	}
}

// setup writes the starter config to the user config path (SPEC §5: reasonix
// setup writes the default config so the CLI is usable out of the box).
func setup() error {
	path := config.UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists at %s — edit it directly", path)
	}
	if err := os.WriteFile(path, []byte(config.Sample()), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\nset the provider's api_key_env (e.g. export DEEPSEEK_API_KEY=...), then run reasonix-mini\n", path)
	return nil
}
