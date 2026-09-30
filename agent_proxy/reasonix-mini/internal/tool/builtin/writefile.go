package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&writeFile{}) }

type writeFile struct{}

const writeFileSchema = `{"type":"object","properties":{` +
	`"path":{"type":"string","description":"Target file path"},` +
	`"content":{"type":"string","description":"Full file content to write"}},"required":["path","content"]}`

func (writeFile) Name() string            { return "write_file" }
func (writeFile) Schema() json.RawMessage { return json.RawMessage(writeFileSchema) }
func (writeFile) ReadOnly() bool          { return false }
func (writeFile) Description() string {
	return "Create or overwrite a text file with the given content (parent dirs are created)."
}

func (writeFile) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("write_file: bad args: %w", err)
	}
	if in.Path == "" {
		return "", fmt.Errorf("write_file: path is required")
	}
	abs, err := confine(in.Path) // sandbox-like confinement
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path), nil
}
