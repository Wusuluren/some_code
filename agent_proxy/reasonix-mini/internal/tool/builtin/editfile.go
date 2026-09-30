package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&editFile{}) }

type editFile struct{}

const editFileSchema = `{"type":"object","properties":{` +
	`"path":{"type":"string","description":"File to edit"},` +
	`"old_string":{"type":"string","description":"Exact text to replace"},` +
	`"new_string":{"type":"string","description":"Replacement text"},` +
	`"replace_all":{"type":"boolean","description":"Replace every occurrence (default: false, requires uniqueness)"}},"required":["path","old_string","new_string"]}`

func (editFile) Name() string            { return "edit_file" }
func (editFile) Schema() json.RawMessage { return json.RawMessage(editFileSchema) }
func (editFile) ReadOnly() bool          { return false }
func (editFile) Description() string {
	return "Replace an exact string in a file. Without replace_all, old_string must match exactly once."
}

func (editFile) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("edit_file: bad args: %w", err)
	}
	if in.OldString == "" {
		return "", fmt.Errorf("edit_file: old_string must not be empty (use write_file to rewrite wholesale)")
	}
	abs, err := confine(in.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(data)
	count := strings.Count(text, in.OldString)
	if count == 0 {
		return "", fmt.Errorf("edit_file: old_string not found in %s — re-read the file and match it exactly", in.Path)
	}
	if count > 1 && !in.ReplaceAll {
		return "", fmt.Errorf("edit_file: old_string matches %d times in %s; extend it until unique or set replace_all", count, in.Path)
	}
	updated := strings.ReplaceAll(text, in.OldString, in.NewString)
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s (%d occurrence(s) replaced)", in.Path, count), nil
}
