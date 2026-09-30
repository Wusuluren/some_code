// Package builtin holds the compile-time built-in tools that self-register
// into the process-global builtin set (SPEC §3.2). Errors from Execute are
// returned, never fatal — the agent feeds them back so the model can
// self-correct.
package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"reasonix-mini/internal/tool"
)

const (
	maxReadBytes  = 512 << 10 // a single read_file result is capped to keep tool outputs bounded
	maxOutputText = 100 << 10 // soft text budget for listing/grep-style tools
)

func init() { tool.RegisterBuiltin(&readFile{}) }

type readFile struct{}

const readFileSchema = `{"type":"object","properties":{` +
	`"path":{"type":"string","description":"File path to read"},` +
	`"offset":{"type":"integer","description":"1-based first line to read (default 1)"},` +
	`"limit":{"type":"integer","description":"Max lines to return (default all)"}},"required":["path"]}`

func (readFile) Name() string            { return "read_file" }
func (readFile) Schema() json.RawMessage { return json.RawMessage(readFileSchema) }
func (readFile) ReadOnly() bool          { return true }
func (readFile) Description() string {
	return "Read a text file and return its contents, optionally a 1-based line window."
}

func (readFile) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("read_file: bad args: %w", err)
	}
	if in.Path == "" {
		return "", fmt.Errorf("read_file: path is required")
	}
	f, err := os.Open(in.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		if in.Offset > 0 && line < in.Offset {
			continue
		}
		if in.Limit > 0 && line >= in.Offset+in.Limit {
			break
		}
		if sb.Len() > maxReadBytes {
			sb.WriteString("... [truncated: output cap reached]\n")
			break
		}
		fmt.Fprintf(&sb, "%6d | %s\n", line, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	return sb.String(), nil
}
