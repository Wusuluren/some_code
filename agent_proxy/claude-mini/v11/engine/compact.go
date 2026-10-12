package engine

import (
	"claude-code-go/v11/api"
)

// estimateTokens is a cheap chars/4 heuristic over every message's textual
// payload (text, tool results, tool inputs). Good enough to decide when to
// trim without a real tokenizer, mirroring the spirit of the main repo's
// services/token_estimation.
func estimateTokens(msgs []api.Message) int {
	n := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			n += len(b.Text) + len(b.Content) + len(b.Input)
		}
	}
	return n / 4
}

// keepRecentMessages is the floor of recent messages always retained, so
// trimming never eats the current prompt and the last few exchanges.
const keepRecentMessages = 6

// trimForBudget returns the suffix of history that fits maxTokens and how many
// leading messages were dropped. It keeps the most recent keepRecentMessages
// as a hard floor, then slides the start forward until the kept window fits and
// begins on a clean user turn (never a tool_result, which would orphan its
// tool_use and make the request invalid). maxTokens <= 0 disables trimming.
func trimForBudget(msgs []api.Message, maxTokens int) ([]api.Message, int) {
	if maxTokens <= 0 || estimateTokens(msgs) <= maxTokens {
		return msgs, 0
	}
	start := 0
	limit := len(msgs) - keepRecentMessages
	for start < limit && estimateTokens(msgs[start:]) > maxTokens {
		start++
	}
	// Never begin on a message that carries a tool_result block.
	for start < len(msgs)-1 && hasToolResult(msgs[start]) {
		start++
	}
	return msgs[start:], start
}

func hasToolResult(m api.Message) bool {
	for _, b := range m.Content {
		if b.Type == "tool_result" {
			return true
		}
	}
	return false
}
