package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EditTool performs exact string replacement, mirroring internal/tools.FileEditTool.
// v12 adds jingwei's write semantics: refuse stale writes, commit atomically,
// support an all-or-nothing batch of edits, and name line numbers when a match
// is ambiguous.
type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "Edit" }

func (t *EditTool) Description() string {
	return "Performs exact string replacements in a file. old_string must match verbatim (including whitespace) and be unique unless replace_all is true. Pass edits[] to apply several {old_string,new_string} pairs atomically (all succeed or nothing is written). Re-read a file if it changed since your last read."
}

func (t *EditTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"file_path":   strProp("The absolute path to the file to edit"),
		"old_string":  strProp("The exact text to replace (single edit)"),
		"new_string":  strProp("The replacement text (single edit)"),
		"replace_all": map[string]interface{}{"type": "boolean", "description": "Replace all occurrences instead of requiring uniqueness (default false)"},
		"edits": map[string]interface{}{
			"type":        "array",
			"description": "Multiple {old_string,new_string} pairs applied in order, all-or-nothing",
			"items":       map[string]interface{}{"type": "object"},
		},
	}, []string{"file_path"})
}

type editPair struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		FilePath   string     `json:"file_path"`
		OldString  string     `json:"old_string"`
		NewString  string     `json:"new_string"`
		ReplaceAll bool       `json:"replace_all"`
		Edits      []editPair `json:"edits"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.FilePath == "" {
		return "", mustErr("missing required field: file_path")
	}

	pairs := in.Edits
	if len(pairs) == 0 {
		if in.OldString == "" {
			return "", mustErr("provide old_string/new_string or a non-empty edits[] array")
		}
		pairs = []editPair{{OldString: in.OldString, NewString: in.NewString}}
	}

	path, err := confineToCwd(in.FilePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	// v12: refuse to edit a file that changed since we last saw it.
	if err := checkStale(path, data); err != nil {
		return "", err
	}

	content := string(data)
	working := content
	total := 0
	for i, p := range pairs {
		if p.OldString == "" {
			return "", mustErr("edit %d: empty old_string (all-or-nothing, nothing written)", i+1)
		}
		if p.OldString == p.NewString {
			return "", mustErr("edit %d: old_string and new_string must differ", i+1)
		}
		count := strings.Count(working, p.OldString)
		if count == 0 {
			return "", fmt.Errorf("edit %d: old_string %q not found in %s (all-or-nothing, nothing was written)", i+1, clip(p.OldString), path)
		}
		if count > 1 && !in.ReplaceAll {
			return "", fmt.Errorf("edit %d: old_string appears %d times in %s at lines %s; add context or set replace_all", i+1, count, path, lineNumbers(working, p.OldString))
		}
		working = strings.Replace(working, p.OldString, p.NewString, -1)
		total += count
	}

	// v12: commit atomically only after every edit validated.
	if err := atomicWrite(path, []byte(working), 0o644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	commitWrite(path, []byte(working))
	return fmt.Sprintf("Edited %s: %d replacement(s) across %d edit(s)", path, total, len(pairs)), nil
}

// lineNumbers lists (up to 8) 1-based lines where needle starts.
func lineNumbers(content, needle string) string {
	var out []string
	pos := 0
	for {
		idx := strings.Index(content[pos:], needle)
		if idx < 0 {
			break
		}
		abs := pos + idx
		out = append(out, strconv.Itoa(1+strings.Count(content[:abs], "\n")))
		pos = abs + len(needle)
		if len(out) >= 8 {
			out = append(out, "…")
			break
		}
	}
	return strings.Join(out, ", ")
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}
