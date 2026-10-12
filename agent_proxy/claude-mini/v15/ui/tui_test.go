package ui
package ui

import (
	"strings"
	"testing"
	"time"

	"claude-code-go/v15/engine"
)

func TestAppendEventStartsAndStreamsAssistant(t *testing.T) {
	var lines []string
	// First assistant_text starts a new line.
	lines = appendEvent(lines, engine.Event{Type: "assistant_text", Text: "hello "})
	// A second chunk glues onto the same assistant line (streaming).
	lines = appendEvent(lines, engine.Event{Type: "assistant_text", Text: "world"})
	if len(lines) != 1 || lines[0] != pfxAssistant+"hello world" {
		t.Fatalf("assistant stream not concatenated: %#v", lines)
	}
	// Empty text is a no-op.
	before := len(lines)
	lines = appendEvent(lines, engine.Event{Type: "assistant_text", Text: ""})
	if len(lines) != before {
		t.Fatal("empty assistant_text should not add a line")
	}
}

func TestAppendEventToolAndResultStartNewLines(t *testing.T) {
	lines := []string{pfxAssistant + "prior answer"}
	lines = appendEvent(lines, engine.Event{Type: "tool_use", Tool: "Bash", Input: "ls\n-la"})
	if !strings.HasPrefix(lines[len(lines)-1], pfxTool+"Bash") {
		t.Fatalf("tool_use line: %q", lines[len(lines)-1])
	}
	// Newlines in the input are flattened by clip().
	if strings.Contains(lines[len(lines)-1], "\n") {
		t.Fatal("tool input newline not flattened")
	}
	// A tool_use breaks the assistant run, so the next text starts fresh.
	lines = appendEvent(lines, engine.Event{Type: "assistant_text", Text: "after"})
	if lines[len(lines)-1] != pfxAssistant+"after" {
		t.Fatalf("assistant after tool should be new line: %q", lines[len(lines)-1])
	}
}

func TestAppendEventResultErrorAndSummary(t *testing.T) {
	var lines []string
	lines = appendEvent(lines, engine.Event{
		Type: "result", Text: "boom", Turns: 3,
		Elapsed: 120 * time.Millisecond,
	})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "error: boom") {
		t.Fatalf("missing error notice: %q", joined)
	}
	if !strings.Contains(joined, "3 turns") {
		t.Fatalf("missing turn summary: %q", joined)
	}
}

func TestClipCollapsesAndTruncates(t *testing.T) {
	if got := clip("a\nb"); got != "a b" {
		t.Fatalf("newline not collapsed: %q", got)
	}
	long := strings.Repeat("x", 200)
	if got := clip(long); len([]rune(got)) != 81 || !strings.HasSuffix(got, "…") {
		t.Fatalf("clip length: %d", len([]rune(got)))
	}
}
