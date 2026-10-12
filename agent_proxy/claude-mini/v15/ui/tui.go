package ui

// Package ui is v15's front-end: a minimal Bubble Tea TUI that renders the
// engine's Event channel as a live transcript with an input line. It adds no
// new engine capability — it is a different consumer of the same events the
// CLI print loop already reads, mirroring jingwei's ratatui front-end.

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"claude-code-go/v15/engine"
)

// Transcript line prefixes. They also drive the streaming heuristic in
// appendEvent: consecutive assistant_text events concatenate onto one line.
const (
	pfxUser      = "❯ "
	pfxAssistant = "● "
	pfxTool      = "⏺ "
	pfxResult    = "  ⎿ "
)

// Messages pumped from the engine goroutine back into the Bubble Tea loop.
type eventMsg struct{ ev engine.Event }
type doneMsg struct{}

type model struct {
	eng  *engine.Engine
	ctx  context.Context
	send func(tea.Msg) // set by RunTUI to the program's Send

	lines []string
	input string
	busy  bool
	width int

	pending string // first prompt to auto-run from Init
}

// appendEvent folds one engine event into the transcript lines. Kept pure so
// it can be unit-tested without a TTY or an API key.
func appendEvent(lines []string, ev engine.Event) []string {
	switch ev.Type {
	case "assistant_text":
		if ev.Text == "" {
			return lines
		}
		// Stream chunks glue onto the current assistant line; a fresh turn
		// (last line not assistant) starts a new one.
		if n := len(lines); n > 0 && strings.HasPrefix(lines[n-1], pfxAssistant) {
			lines[n-1] += ev.Text
		} else {
			lines = append(lines, pfxAssistant+ev.Text)
		}
	case "tool_use":
		lines = append(lines, pfxTool+ev.Tool+" "+clip(ev.Input))
	case "tool_result":
		lines = append(lines, pfxResult+clip(ev.Text))
	case "result":
		if ev.Text != "" && ev.Text != "interrupted" {
			lines = append(lines, pfxResult+"error: "+ev.Text)
		}
		lines = append(lines, fmt.Sprintf("── %d turns, %s, %d in / %d out tokens ──",
			ev.Turns, ev.Elapsed.Round(100*time.Millisecond), ev.Usage.InputTokens, ev.Usage.OutputTokens))
	}
	return lines
}

func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

func (m model) Init() tea.Cmd {
	if m.pending != "" {
		p := m.pending
		m.pending = ""
		return m.startRun(p)
	}
	return nil
}

// startRun returns a tea.Cmd that drains the engine event channel and forwards
// each event to the program; it is launched by Bubble Tea after the program has
// started, so p.Send is safe to call from inside.
func (m model) startRun(prompt string) tea.Cmd {
	return func() tea.Msg {
		for ev := range m.eng.Run(m.ctx, prompt) {
			m.send(eventMsg{ev})
		}
		return doneMsg{}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case eventMsg:
		m.lines = appendEvent(m.lines, msg.ev)
	case doneMsg:
		m.busy = false
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.busy || strings.TrimSpace(m.input) == "" {
				return m, nil
			}
			prompt := m.input
			m.input = ""
			m.lines = append(m.lines, pfxUser+prompt)
			m.busy = true
			return m, m.startRun(prompt)
		case tea.KeyBackspace:
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		default:
			if !m.busy && len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
	}
	return m, nil
}

var (
	styleUser   = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	styleTool   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	stylePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m model) View() string {
	var b strings.Builder
	for _, ln := range m.lines {
		switch {
		case strings.HasPrefix(ln, pfxUser):
			b.WriteString(styleUser.Render(ln))
		case strings.HasPrefix(ln, pfxTool), strings.HasPrefix(ln, pfxResult):
			b.WriteString(styleTool.Render(ln))
		case strings.HasPrefix(ln, "──"):
			b.WriteString(styleDim.Render(ln))
		default:
			b.WriteString(ln)
		}
		b.WriteString("\n")
	}
	if m.busy {
		b.WriteString(styleDim.Render("⋯ working… (Ctrl-C to quit)") + "\n")
	} else {
		b.WriteString(stylePrompt.Render(pfxUser+m.input) + "\n")
	}
	return b.String()
}

// RunTUI drives the engine through an interactive Bubble Tea transcript. A
// non-empty firstPrompt is submitted automatically as the opening turn.
func RunTUI(ctx context.Context, eng *engine.Engine, firstPrompt string) error {
	m := &model{eng: eng, ctx: ctx, pending: firstPrompt}
	p := tea.NewProgram(m)
	m.send = p.Send
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
