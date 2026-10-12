package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"claude-code-go/v13/api"
)

func txt(role, s string) api.Message {
	return api.Message{Role: role, Content: []api.ContentBlock{{Type: "text", Text: s}}}
}

func TestTrimForBudgetBasics(t *testing.T) {
	big := strings.Repeat("x", 400) // ~100 estimated tokens per message
	var msgs []api.Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, txt("user", fmt.Sprintf("q%d %s", i, big)))
		msgs = append(msgs, txt("assistant", big))
	}

	// Cap disabled.
	if got, d := trimForBudget(msgs, 0); d != 0 || len(got) != len(msgs) {
		t.Fatal("budget 0 must not trim")
	}
	// Budget larger than the whole history.
	if _, d := trimForBudget(msgs, estimateTokens(msgs)+1); d != 0 {
		t.Fatal("should not trim when under budget")
	}

	kept, dropped := trimForBudget(msgs, 300)
	if dropped == 0 {
		t.Fatal("expected some messages dropped")
	}
	if len(kept) < keepRecentMessages {
		t.Fatalf("kept floor violated: %d", len(kept))
	}
	if hasToolResult(kept[0]) {
		t.Fatal("kept window starts on a tool_result")
	}
}

func TestTrimNeverStartsOnToolResult(t *testing.T) {
	// A history with tool_use/tool_result pairs so a naive token-window cut
	// could land on a tool_result; the guard must slide off it.
	toolRes := func(s string) api.Message {
		return api.Message{Role: "user", Content: []api.ContentBlock{{Type: "tool_result", ToolUseID: "id", Content: s}}}
	}
	toolUse := func() api.Message {
		return api.Message{Role: "assistant", Content: []api.ContentBlock{{Type: "tool_use", ID: "id", Name: "Bash", Input: json.RawMessage("{}")}}}
	}
	msgs := []api.Message{
		txt("user", strings.Repeat("a", 800)), toolUse(), toolRes(strings.Repeat("r", 800)),
		txt("assistant", strings.Repeat("b", 800)), txt("user", strings.Repeat("c", 800)),
		toolUse(), toolRes(strings.Repeat("r", 800)), txt("assistant", strings.Repeat("d", 800)),
		txt("user", strings.Repeat("e", 800)), txt("assistant", strings.Repeat("f", 800)),
		txt("user", strings.Repeat("g", 800)), txt("user", "what now?"),
	}
	for budget := 10; budget <= 600; budget += 10 {
		kept, dropped := trimForBudget(msgs, budget)
		if dropped > 0 && hasToolResult(kept[0]) {
			t.Fatalf("budget %d: kept window starts on a tool_result", budget)
		}
	}
}
