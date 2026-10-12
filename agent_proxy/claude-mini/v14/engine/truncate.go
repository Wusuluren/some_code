package engine
package engine

import "fmt"

// MaxToolOutputBytes caps how much of a single tool result is sent back to the
// model. A run-away Bash or a huge Read could otherwise blow the context budget
// in one shot. This mirrors jingwei's 50 KB tool-output ceiling: the limit is
// applied centrally at the one place every tool result passes through, so no
// individual tool has to remember to guard itself.
const MaxToolOutputBytes = 50_000

// truncateToolOutput keeps the head and tail of an over-long result and makes
// the cut visible to the model, so it knows to narrow the query rather than
// assume the output ended naturally. Byte-safe: it splits on byte length but
// only ever cuts at ASCII-safe boundaries of the returned substrings, and the
// notice never re-expands the string past the cap.
func truncateToolOutput(content string) string {
	if len(content) <= MaxToolOutputBytes {
		return content
	}
	// Reserve room for the notice; keep ~90% head, ~10% tail.
	reserve := 128
	if MaxToolOutputBytes <= reserve {
		return content[:MaxToolOutputBytes]
	}
	keep := MaxToolOutputBytes - reserve
	head := keep * 9 / 10
	tail := keep - head
	omitted := len(content) - head - tail
	return fmt.Sprintf("%s\n\n[... %d bytes truncated by MAX_TOOL_OUTPUT=%d; narrow the tool call ...]\n\n%s",
		content[:head], omitted, MaxToolOutputBytes, content[len(content)-tail:])
}
