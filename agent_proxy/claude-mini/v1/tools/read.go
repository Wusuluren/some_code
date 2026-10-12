package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTool reads file contents with line numbers (cat -n style),
// mirroring internal/tools.FileReadTool.
type ReadTool struct{}

func NewReadTool() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string { return "Read" }

func (t *ReadTool) Description() string {
	return "Reads a file from the local filesystem and returns its text content with 1-based line numbers. Supports reading a range of lines via offset/limit for large files."
}

func (t *ReadTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"file_path": strProp("The absolute path to the file to read"),
		"offset":    map[string]interface{}{"type": "integer", "description": "The line number to start reading from (1-based)"},
		"limit":     map[string]interface{}{"type": "integer", "description": "The maximum number of lines to read (default 2000)"},
	}, []string{"file_path"})
}

const defaultReadLimit = 2000

func (t *ReadTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		FilePath string `json:"file_path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.FilePath == "" {
		return "", mustErr("missing required field: file_path")
	}
	if in.Offset < 0 {
		in.Offset = 0
	}
	limit := in.Limit
	if limit <= 0 || limit > defaultReadLimit {
		limit = defaultReadLimit
	}

	f, err := os.Open(in.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	lineNo := 0
	shown := 0
	for scanner.Scan() {
		lineNo++
		if lineNo <= in.Offset {
			continue
		}
		if shown >= limit {
			fmt.Fprintf(&b, "... (truncated, more lines after line %d)\n", lineNo-1)
			break
		}
		fmt.Fprintf(&b, "%6d\t%s\n", lineNo, scanner.Text())
		shown++
	}
	if err := scanner.Err(); err != nil {
		return b.String(), fmt.Errorf("failed to read file: %w", err)
	}
	if shown == 0 {
		return "", fmt.Errorf("file %s is empty or offset %d is past end of file (%d lines)", in.FilePath, in.Offset, lineNo)
	}
	return b.String(), nil
}
