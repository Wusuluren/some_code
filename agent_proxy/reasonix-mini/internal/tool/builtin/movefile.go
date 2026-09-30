package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&moveFile{}) }

type moveFile struct{}

const moveFileSchema = `{"type":"object","properties":{` +
	`"source":{"type":"string","description":"Existing path to move"},` +
	`"destination":{"type":"string","description":"Target path (must not exist)"}},"required":["source","destination"]}`

func (moveFile) Name() string            { return "move_file" }
func (moveFile) Schema() json.RawMessage { return json.RawMessage(moveFileSchema) }
func (moveFile) ReadOnly() bool          { return false }
func (moveFile) Description() string {
	return "Rename or move a file/directory. The destination must not already exist."
}

func (moveFile) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("move_file: bad args: %w", err)
	}
	src, err := confine(in.Source)
	if err != nil {
		return "", err
	}
	dst, err := confine(in.Destination)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("move_file: destination %s already exists", in.Destination)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	return fmt.Sprintf("moved %s -> %s", in.Source, in.Destination), nil
}
