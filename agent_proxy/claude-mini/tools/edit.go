package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EditTool performs exact string replacement, mirroring internal/tools.FileEditTool.
type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string { return "Edit" }

func (t *EditTool) Description() string {
	return "Performs exact string replacements in a file. old_string must match the file content verbatim (including whitespace) and be unique unless replace_all is true. Read the file before editing."
}

func (t *EditTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"file_path":   strProp("The absolute path to the file to edit"),
		"old_string":  strProp("The exact text to replace"),
		"new_string":  strProp("The replacement text"),
		"replace_all": map[string]interface{}{"type": "boolean", "description": "Replace all occurrences instead of requiring uniqueness (default false)"},
	}, []string{"file_path", "old_string", "new_string"})
}

func (t *EditTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.FilePath == "" {
		return "", mustErr("missing required field: file_path")
	}
	if in.OldString == in.NewString {
		return "", mustErr("old_string and new_string must differ")
	}

	path, err := confineToCwd(in.FilePath)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(data)

	count := strings.Count(content, in.OldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s; read the file and match the exact text", path)
	}
	if count > 1 && !in.ReplaceAll {
		return "", fmt.Errorf("old_string appears %d times in %s; provide more context to make it unique or set replace_all", count, path)
	}

	updated := strings.Replace(content, in.OldString, in.NewString, -1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return fmt.Sprintf("Edited %s: %d occurrence(s) replaced", path, count), nil
}
