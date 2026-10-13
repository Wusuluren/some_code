package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderDiff produces a unified-style diff preview for a guarded tool call,
// without ever executing the tool. write_file diffs the current file against
// the content about to be written; edit shows each oldText→newText replacement
// block. Anything else (e.g. bash) returns "" — the raw args are shown in the
// prompt line instead.
func renderDiff(tool string, args json.RawMessage) string {
	switch strings.ToLower(tool) {
	case "write_file":
		return diffWrite(args)
	case "edit":
		return diffEdit(args)
	default:
		return ""
	}
}

// summarizeArgs shortens a tool call for the prompt header: file tools show
// just the path, the rest fall back to a truncated raw argument string.
func summarizeArgs(tool string, args json.RawMessage) string {
	switch strings.ToLower(tool) {
	case "write_file", "edit", "read":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(args, &p) == nil && p.Path != "" {
			return p.Path
		}
	}
	return truncate(strings.TrimSpace(string(args)), 120)
}

func diffWrite(args json.RawMessage) string {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if json.Unmarshal(args, &in) != nil || in.Path == "" {
		return ""
	}
	old, existed := readFileOrNil(in.Path)
	if !existed {
		return diffHeader(in.Path+" (new file)", 1, linesOf(in.Content))
	}
	if old == in.Content {
		return diffHeader(in.Path, 0, nil) + "  (no textual change)\n"
	}
	return diffHeader(in.Path, -1, nil) + lineDiff(old, in.Content)
}

func diffEdit(args json.RawMessage) string {
	var in struct {
		Path  string `json:"path"`
		Edits []struct {
			OldText string `json:"oldText"`
			NewText string `json:"newText"`
		} `json:"edits"`
	}
	if json.Unmarshal(args, &in) != nil || in.Path == "" || len(in.Edits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(diffHeader(in.Path, -1, nil))
	for i, e := range in.Edits {
		if len(in.Edits) > 1 {
			fmt.Fprintf(&b, "  ── edit %d/%d ──\n", i+1, len(in.Edits))
		}
		for _, l := range clampLines(linesOf(e.OldText)) {
			b.WriteString("- " + l + "\n")
		}
		for _, l := range clampLines(linesOf(e.NewText)) {
			b.WriteString("+ " + l + "\n")
		}
	}
	return b.String()
}

// diffHeader prints the leading "⛛ <path>" banner. added>0 labels a new file
// with its line count; added==0 means no change.
func diffHeader(path string, added int, contentLines []string) string {
	if added > 0 {
		return fmt.Sprintf("\n\033[1;35m⛛ %s\033[0m  \033[32m+ new file (%d lines)\033[0m\n", path, addedOf(contentLines))
	}
	return fmt.Sprintf("\n\033[1;35m⛛ %s\033[0m\n", path)
}

func addedOf(lines []string) int { return len(lines) }

const maxDiffLines = 60

// clampLines caps a slice for display, appending an ellipsis row when trimmed.
func clampLines(lines []string) []string {
	if len(lines) <= maxDiffLines {
		return lines
	}
	out := append([]string(nil), lines[:maxDiffLines]...)
	return append(out, fmt.Sprintf("… (%d more lines)", len(lines)-maxDiffLines))
}

// lineDiff is a lightweight line-oriented diff (matches changed/added/removed
// lines heuristically) that emits only the +/- changes, not unchanged context.
func lineDiff(oldStr, newStr string) string {
	oldLines := linesOf(oldStr)
	newLines := linesOf(newStr)
	var out []string
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			i++
			j++
		case j < len(newLines) && (i >= len(oldLines) || !containsFrom(oldLines[i:], newLines[j])):
			out = append(out, "+ "+newLines[j])
			j++
		case i < len(oldLines):
			out = append(out, "- "+oldLines[i])
			i++
		}
	}
	return strings.Join(clampLines(out), "\n") + "\n"
}

func containsFrom(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func linesOf(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// readFileOrNil returns the current contents of a workspace path and whether it
// exists. Relative paths resolve against the sandbox dir (or the process cwd),
// matching how the file tools resolve arguments.
func readFileOrNil(path string) (string, bool) {
	data, err := os.ReadFile(resolveAbs(path))
	if err != nil {
		return "", false
	}
	return string(data), true
}

func resolveAbs(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	base := sandboxFlag
	if base == "" {
		base = cwdOrDot()
	}
	return filepath.Join(base, path)
}
