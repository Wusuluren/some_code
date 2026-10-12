package engine

import (
	"strings"
	"testing"
)

func TestTruncateToolOutputSmallUnchanged(t *testing.T) {
	in := "short output"
	if got := truncateToolOutput(in); got != in {
		t.Fatalf("small output altered: %q", got)
	}
}

func TestTruncateToolOutputCapsLarge(t *testing.T) {
	in := strings.Repeat("x", MaxToolOutputBytes*2)
	got := truncateToolOutput(in)
	if len(got) > MaxToolOutputBytes {
		t.Fatalf("truncated len=%d exceeds cap=%d", len(got), MaxToolOutputBytes)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatal("missing truncation notice")
	}
	// Head and tail of the original survive the cut.
	if !strings.HasPrefix(got, "xxx") || !strings.HasSuffix(got, "xxx") {
		t.Fatal("head/tail not preserved")
	}
}
