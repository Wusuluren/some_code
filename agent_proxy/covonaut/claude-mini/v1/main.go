// Command claude-mini is a trimmed-down coding agent built on covonaut,
// mirroring the v15 "claude-mini" demo: a CLI entry, an LLM provider, a small
// set of tools (Bash/Read/Write/Edit/Grep), the agent loop, human-in-the-loop
// approval, JSONL session persistence, and a live terminal UI.
//
// Unlike v15 — which hand-rolls its own engine, API client, tool registry and
// Bubble Tea front-end — this demo leans on covonaut's packages:
//
//   - agentcore      : the model<->tool loop, events, compaction, retry, store
//   - provider/*     : anthropic / openai(chatcompat) / gemini wire protocols
//   - tools          : ready-made Bash/Read/Write/Edit/Grep tools
//   - session        : append-only JSONL sessions + AgentStore persistence
//   - tui + agentadapter : the live transcript front-end (covonaut's own TUI)
//
// The engine is never modified; the TUI is just another consumer of the agent
// event channel — exactly the relationship v15's ui package had to its engine.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/covoyage/covonaut/agentcore"
	"github.com/covoyage/covonaut/provider/anthropic"
	"github.com/covoyage/covonaut/provider/chatcompat"
	"github.com/covoyage/covonaut/provider/gemini"
	"github.com/covoyage/covonaut/session"
	"github.com/covoyage/covonaut/tools"
	"github.com/covoyage/covonaut/tui"
	"github.com/covoyage/covonaut/tui/agentadapter"
	"github.com/covoyage/covonaut/tui/chat"
	"github.com/covoyage/covonaut/tui/theme"
)

const demoVersion = "0.1.0"

var (
	printMode    bool
	tuiMode      bool
	modelFlag    string
	providerFlag string
	maxTurnsFlag int
	verboseFlag  bool
	apiKeyFlag   string
	baseURLFlag  string
	sandboxFlag  string
	guardFlag    string
	streamFlag   bool
	contextFlag  int64
	resumeFlag   string
	continueFlag bool
	listFlag     bool
	newFlag      bool
	sessionsFlag string
	mcpFlag      string
)

func main() {
	flag.BoolVar(&printMode, "print", false, "Print mode: run one prompt non-interactively and exit")
	flag.BoolVar(&tuiMode, "tui", false, "Run the covonaut terminal UI over the agent event stream")
	flag.StringVar(&modelFlag, "model", "", "Model id (default depends on -provider)")
	flag.StringVar(&providerFlag, "provider", "anthropic", "Wire protocol: anthropic | openai | gemini")
	flag.IntVar(&maxTurnsFlag, "max-turns", 100, "Maximum number of agent turns")
	flag.BoolVar(&verboseFlag, "verbose", false, "Print per-tool execution lines to stderr")
	flag.StringVar(&apiKeyFlag, "api-key", "", "API key (falls back to the provider's env var, then API_KEY)")
	flag.StringVar(&baseURLFlag, "base-url", "", "API base URL (falls back to the provider's env var)")
	flag.StringVar(&sandboxFlag, "sandbox", "", "Confine all file tools and Bash to this directory (created if missing)")
	flag.StringVar(&guardFlag, "guard-tools", "bash,write_file,edit", "Comma-separated tools needing human approval (empty disables prompting)")
	flag.BoolVar(&streamFlag, "stream", false, "Stream the model reply and print tokens as they arrive")
	flag.Int64Var(&contextFlag, "context-size", 0, "Enable auto-compaction against this context window (0 = off)")
	flag.StringVar(&resumeFlag, "resume", "", "Resume a saved session by id or id prefix")
	flag.BoolVar(&continueFlag, "continue", false, "Resume the most recent session for this project")
	flag.BoolVar(&listFlag, "list", false, "List saved sessions and exit")
	flag.BoolVar(&newFlag, "new", false, "Start a fresh session even if -continue/-resume is set")
	flag.StringVar(&sessionsFlag, "sessions-dir", "", "Override the sessions directory (default: .claude-mini/sessions)")
	flag.StringVar(&mcpFlag, "mcp", ".mcp.json", "MCP servers config (JSON, {mcpServers:{...}}); missing file disables MCP, empty disables loading")
	flag.Parse()

	// Signal-cancelled context: Ctrl-C interrupts the running turn (engine
	// emits an interrupt event) rather than killing the process outright.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if sandboxFlag != "" {
		if err := os.MkdirAll(sandboxFlag, 0o755); err != nil {
			fatalf("failed to create sandbox: %v", err)
		}
		root, err := filepath.Abs(sandboxFlag)
		if err != nil {
			fatalf("%v", err)
		}
		sandboxFlag = root
	}

	fs, err := session.NewFileStore(sessionsDir())
	if err != nil {
		fatalf("open session store: %v", err)
	}
	store := session.NewAgentStore(fs, cwdOrDot())

	if listFlag {
		listSessions(ctx, store)
		return
	}

	threadID := resolveThread(ctx, store)
	provider := buildProvider()
	model := resolveModel()

	// v10-equivalent MCP: connect servers from .mcp.json (best-effort) and
	// layer their namespaced tools on top of the built-ins.
	mcpExts := loadMCPExtensions(ctx, mcpFlag)

	agent, err := agentcore.NewWithError(newConfig(provider, model, store, mcpExts))
	if err != nil {
		fatalf("init agent: %v", err)
	}
	defer agent.Close()

	if threadID != "" {
		if ok, _ := store.Has(ctx, threadID); ok {
			if err := agent.LoadState(ctx, threadID); err != nil {
				fatalf("restore session %q: %v", threadID, err)
			}
		}
	}
	// A stable thread id keeps the JSONL session append-only across turns.
	currentThread := threadID
	save := func() {
		if currentThread == "" {
			return
		}
		if err := agent.SaveState(ctx, currentThread); err != nil {
			fmt.Fprintf(os.Stderr, "warn: save session state: %v\n", err)
		}
	}

	switch {
	case tuiMode:
		runTUI(ctx, agent, &currentThread, strings.Join(flag.Args(), " "), save)
	case printMode:
		if len(flag.Args()) == 0 {
			fatalf("no prompt provided in print mode")
		}
		if err := runOnce(ctx, agent, strings.Join(flag.Args(), " ")); err != nil {
			fatalf("%v", err)
		}
		save()
	default:
		fmt.Printf("claude-mini v%s — type a prompt (Ctrl+D to exit)\n", demoVersion)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "/exit" || line == "/quit" {
				break
			}
			if line == "/new" {
				save()
				currentThread = newThreadID()
				agent.State().ReplaceMessages(nil)
				agent.ResetContextEngine()
				fmt.Printf("(started new session %s)\n", currentThread)
				continue
			}
			if err := runOnce(ctx, agent, line); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
			}
			save()
		}
	}
}

// newConfig assembles the covonaut agent config: model/provider, the mini tool
// set, human-in-the-loop guarding, optional compaction and retry, and any MCP
// server extensions.
func newConfig(provider agentcore.Provider, model string, store agentcore.Store, exts []agentcore.Extension) agentcore.Config {
	cfg := agentcore.NewConfig(
		agentcore.WithName("claude-mini"),
		agentcore.WithModel(model),
		agentcore.WithProvider(provider),
		agentcore.WithSystemPrompt(systemPrompt),
		agentcore.WithMaxTurns(int64(maxTurnsFlag)),
		agentcore.WithStreaming(streamFlag),
		agentcore.WithValidateArguments(true),
		agentcore.WithTools(miniTools()...),
		agentcore.WithStore(store),
	)
	if len(exts) > 0 {
		agentcore.WithExtensions(exts...)(&cfg)
	}
	if contextFlag > 0 {
		agentcore.WithContextWindow(contextFlag)(&cfg)
	}
	cfg.RetryConfig = &agentcore.RetryConfig{MaxRetries: 3, BaseDelayMs: 1000, MaxDelayMs: 15000}
	if h := buildGuard(); h != nil {
		cfg.GlobalBefore = append(cfg.GlobalBefore, h)
	}
	return cfg
}

// miniTools builds the v15-equivalent tool set: Bash, Read, Write, Edit, Grep.
// covonaut's tools package provides these with a swappable operations layer,
// so the working directory (sandbox) is all we need to configure here.
func miniTools() []*agentcore.Tool {
	cwd := sandboxFlag
	return []*agentcore.Tool{
		tools.NewBashTool(cwd, nil),
		tools.NewReadTool(cwd, nil),
		tools.NewWriteFileTool(cwd, nil),
		tools.NewEditTool(cwd, nil),
		tools.NewGrepTool(cwd, nil),
	}
}

// runOnce drives one prompt through the agent and renders the event stream to
// stdout/stderr. This is the print/REPL front-end; covonaut's own loop emits
// the same events the TUI consumes, so both front-ends stay thin.
func runOnce(ctx context.Context, agent *agentcore.Agent, prompt string) error {
	var (
		mu        sync.Mutex
		turns     int64
		usage     agentcore.TokenUsage
		streaming = streamFlag
		lineOpen  bool
		start     = time.Now()
	)

	closeLine := func() {
		if lineOpen {
			fmt.Println()
			lineOpen = false
		}
	}

	unsubs := []func(){
		agent.On(agentcore.EventMessageDelta, func(e agentcore.Event) {
			if !streaming {
				return
			}
			d := e.(*agentcore.MessageDeltaEvent)
			if d.Delta == "" {
				return
			}
			fmt.Print(d.Delta)
			lineOpen = true
		}),
		agent.On(agentcore.EventTurnStart, func(agentcore.Event) {
			mu.Lock()
			turns++
			mu.Unlock()
		}),
		agent.On(agentcore.EventTurnEnd, func(e agentcore.Event) {
			te := e.(*agentcore.TurnEndEvent)
			mu.Lock()
			usage.Merge(te.Usage)
			mu.Unlock()
		}),
		agent.On(agentcore.EventToolCallStart, func(e agentcore.Event) {
			closeLine()
			if !verboseFlag {
				return
			}
			tc := e.(*agentcore.ToolCallStartEvent)
			fmt.Fprintf(os.Stderr, "⏺ %s(%s)\n", tc.ToolCall.Name, string(tc.ToolCall.Arguments))
		}),
		agent.On(agentcore.EventToolCallEnd, func(e agentcore.Event) {
			if !verboseFlag {
				return
			}
			te := e.(*agentcore.ToolCallEndEvent)
			if te.Err != nil {
				fmt.Fprintf(os.Stderr, "⏺ %s error: %v\n", te.ToolName, te.Err)
			}
		}),
	}
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	output, err := agent.Run(ctx, prompt)
	closeLine()
	if !streaming && strings.TrimSpace(output) != "" {
		fmt.Println(output)
	}

	mu.Lock()
	tt, uu := turns, usage
	mu.Unlock()
	fmt.Fprintf(os.Stderr, "── %d turns, %s, %d input / %d output tokens ──\n",
		tt, time.Since(start).Round(100*time.Millisecond), uu.PromptTokens, uu.CompletionTokens)
	if err != nil {
		return err
	}
	return nil
}

// runTUI drives covonaut's own terminal front-end over the agent event
// channel. The engine is untouched; the chat app is just another event
// consumer (agentadapter.BindAgent), mirroring v15's ui.RunTUI.
func runTUI(ctx context.Context, agent *agentcore.Agent, thread *string, initial string, save func()) {
	if err := theme.InitThemeFromEnv(); err != nil {
		fatalf("theme: %v", err)
	}

	var busy atomic.Bool
	var app *chat.ChatApp

	submit := func(prompt string) {
		trimmed := strings.TrimSpace(prompt)
		if trimmed == "" {
			return
		}
		switch trimmed {
		case "/quit", "/exit":
			save()
			_ = app.Stop()
			return
		case "/new":
			if busy.Load() {
				app.PrintSystem("(busy — wait for the current reply)")
				return
			}
			save()
			*thread = newThreadID()
			agent.State().ReplaceMessages(nil)
			agent.ResetContextEngine()
			app.History().Clear()
			app.PrintSystem(fmt.Sprintf("started new session %s", *thread))
			return
		}
		if busy.Load() {
			app.PrintSystem("(busy — wait for the current reply)")
			return
		}
		busy.Store(true)
		go func() {
			defer busy.Store(false)
			if _, err := agent.Run(context.Background(), trimmed); err != nil {
				app.PrintError(err)
			}
			save()
		}()
	}

	app = tui.NewChatApp(chat.ChatAppConfig{
		Title:       fmt.Sprintf("claude-mini v%s · %s", demoVersion, agent.Config().Model),
		ShowTimings: true,
		ShowTurns:   true,
		AltScreen:   true,
		OnSubmit:    func(_ context.Context, input string) { submit(input) },
	})
	agentadapter.BindAgent(app, agent)

	app.PrintSystem("claude-mini TUI. Slash: /new (fresh session), /quit (exit). Ctrl-C also quits.")
	if strings.TrimSpace(initial) != "" {
		submit(initial)
	}

	if err := app.Start(); err != nil {
		fatalf("start tui: %v", err)
	}
	<-app.Done()
}

// --- human-in-the-loop guard (v6 equivalent) ---

// buildGuard returns a BeforeHook that prompts on stdin before running any
// guarded tool. Print/TUI modes auto-approve (stdin is not interactive there).
func buildGuard() agentcore.BeforeHook {
	if strings.TrimSpace(guardFlag) == "" {
		return nil
	}
	guarded := map[string]bool{}
	for _, name := range strings.Split(guardFlag, ",") {
		if n := strings.ToLower(strings.TrimSpace(name)); n != "" {
			guarded[n] = true
		}
	}
	var (
		sc     = bufio.NewScanner(os.Stdin)
		always = map[string]bool{}
		mu     sync.Mutex
	)
	return func(_ context.Context, hc *agentcore.HookContext) error {
		key := strings.ToLower(hc.ToolName)
		mu.Lock()
		needApproval := guarded[key] && !always[key]
		mu.Unlock()
		if !needApproval {
			return nil
		}
		if printMode || tuiMode {
			fmt.Fprintf(os.Stderr, "warn: %s mode auto-approves %s (no interactive prompt)\n", modeName(), hc.ToolName)
			return nil
		}
		for {
			fmt.Fprintf(os.Stderr, "\n⚠  %s %s\nallow? [y]es / [N]o / [a]lways: ",
				hc.ToolName, strings.TrimSpace(string(hc.Arguments)))
			if !sc.Scan() {
				return fmt.Errorf("no answer from user (stdin closed), %s skipped", hc.ToolName)
			}
			switch strings.ToLower(strings.TrimSpace(sc.Text())) {
			case "y", "yes":
				return nil
			case "a", "always":
				mu.Lock()
				always[key] = true
				mu.Unlock()
				return nil
			default:
				return fmt.Errorf("user denied %s", hc.ToolName)
			}
		}
	}
}

func modeName() string {
	if tuiMode {
		return "-tui"
	}
	return "-print"
}

// --- provider / model resolution ---

func buildProvider() agentcore.Provider {
	key := firstNonEmpty(apiKeyFlag, os.Getenv(providerEnvKey()), os.Getenv("API_KEY"))
	base := firstNonEmpty(baseURLFlag, os.Getenv(providerBaseURLEnv()))
	switch strings.ToLower(providerFlag) {
	case "anthropic":
		requireKey(key, "ANTHROPIC_API_KEY (or -api-key / API_KEY)")
		return anthropic.New(anthropic.Config{APIKey: key, BaseURL: base})
	case "gemini":
		requireKey(key, "GEMINI_API_KEY (or -api-key / API_KEY)")
		return gemini.New(gemini.Config{APIKey: key, BaseURL: base})
	default: // openai / chat-completions compatible
		requireKey(key, "OPENAI_API_KEY (or -api-key / API_KEY)")
		return chatcompat.New(chatcompat.Config{APIKey: key, BaseURL: base})
	}
}

func providerEnvKey() string {
	switch strings.ToLower(providerFlag) {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

func providerBaseURLEnv() string {
	switch strings.ToLower(providerFlag) {
	case "anthropic":
		return "ANTHROPIC_BASE_URL"
	case "gemini":
		return "GEMINI_BASE_URL"
	default:
		return "OPENAI_BASE_URL"
	}
}

func resolveModel() string {
	if modelFlag != "" {
		return modelFlag
	}
	switch strings.ToLower(providerFlag) {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "gemini":
		return "gemini-2.5-flash"
	default:
		return "gpt-4o-mini"
	}
}

// --- session helpers ---

func sessionsDir() string {
	if sessionsFlag != "" {
		return sessionsFlag
	}
	if e := os.Getenv("AGENT_SESSIONS_DIR"); e != "" {
		return e
	}
	return filepath.Join(cwdOrDot(), ".claude-mini", "sessions")
}

// resolveThread decides which JSONL session this run writes to.
func resolveThread(ctx context.Context, store *session.AgentStore) string {
	if newFlag {
		return newThreadID()
	}
	if resumeFlag != "" {
		threads, _ := store.ListThreads(ctx)
		for _, t := range threads {
			if t.ID == resumeFlag || strings.HasPrefix(t.ID, resumeFlag) {
				return t.ID
			}
		}
		fatalf("no session matching %q", resumeFlag)
	}
	if continueFlag {
		threads, _ := store.ListThreads(ctx)
		if len(threads) > 0 {
			sort.Slice(threads, func(i, j int) bool { return threads[i].UpdatedAt.After(threads[j].UpdatedAt) })
			return threads[0].ID
		}
	}
	return ""
}

func listSessions(ctx context.Context, store *session.AgentStore) {
	threads, err := store.ListThreads(ctx)
	if err != nil {
		fatalf("list sessions: %v", err)
	}
	if len(threads) == 0 {
		fmt.Printf("no sessions in %s\n", sessionsDir())
		return
	}
	sort.Slice(threads, func(i, j int) bool { return threads[i].UpdatedAt.After(threads[j].UpdatedAt) })
	fmt.Printf("Sessions in %s:\n", sessionsDir())
	for _, t := range threads {
		summary := t.Summary
		if summary == "" {
			summary = "(empty)"
		}
		fmt.Printf("  %s  %s  %d msgs  %s\n",
			t.ID, t.UpdatedAt.Format("2006-01-02 15:04"), t.MessageCount, truncate(summary, 60))
	}
}

func newThreadID() string {
	return fmt.Sprintf("mini-%d", time.Now().UnixNano())
}

// --- misc ---

const systemPrompt = `You are claude-mini, a coding agent running in a shell.
You can inspect and edit the workspace with the provided tools:
- bash: run shell commands in the working directory
- read: read a file (offset/limit supported)
- write_file: create or overwrite a file
- edit: replace an exact string in a file
- grep: search file contents with a regex
Prefer gathering context (read/grep/bash) before editing. Keep edits minimal and
precise. When you have completed the task, summarize what you did briefly.`

func cwdOrDot() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func requireKey(key, hint string) {
	if strings.TrimSpace(key) == "" {
		fatalf("missing API key: set %s", hint)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
