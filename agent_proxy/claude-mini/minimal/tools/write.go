package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool writes (creates/overwrites) a file, mirroring internal/tools.FileWriteTool.
type WriteTool struct{}

func NewWriteTool() *WriteTool { return &WriteTool{} }

func (t *WriteTool) Name() string { return "Write" }

func (t *WriteTool) Description() string {
	return "Writes a file to the local filesystem, creating it (with parent directories) or overwriting it if it already exists. Prefer Edit for modifying existing files."
}

func (t *WriteTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"file_path": strProp("The absolute path to the file to write"),
		"content":   strProp("The content to write to the file"),
	}, []string{"file_path", "content"})
}

// confineToCwd resolves a write target and rejects paths outside the
// working tree, so model-proposed paths cannot escape the project.
// Symlinks are resolved (macOS /var -> /private/var) before comparing.
func confineToCwd(path string) (string, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(cwd, path)
	}
	real, err := resolveReal(path)
	if err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	base, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", err
	}
	base = filepath.Clean(base) + string(os.PathSeparator)
	if real != filepath.Clean(base[:len(base)-1]) && !startsWith(real, base) {
		return "", fmt.Errorf("path %q is outside the working directory %s", path, cwd)
	}
	return real, nil
}

// resolveReal is EvalSymlinks on a possibly not-yet-existing path,
// falling back to the parent dir.
func resolveReal(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return real, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		// Parent may not exist yet (MkdirAll creates it later); just clean it.
		return filepath.Clean(path), nil
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func (t *WriteTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.FilePath == "" {
		return "", mustErr("missing required field: file_path")
	}

	path, err := confineToCwd(in.FilePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	existed := false
	if _, err := os.Stat(path); err == nil {
		existed = true
	}

	if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	verb := "created"
	if existed {
		verb = "updated"
	}
	return fmt.Sprintf("File %s: %s (%d bytes)", verb, path, len(in.Content)), nil
}
