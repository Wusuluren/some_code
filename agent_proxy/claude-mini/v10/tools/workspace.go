package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LsTool lists a directory, giving the agent a view of its long-term
// workspace. Mirrors the listing half of internal/tools Glob/LS.
type LsTool struct{}

func NewLsTool() *LsTool { return &LsTool{} }

func (t *LsTool) Name() string { return "Ls" }

func (t *LsTool) Description() string {
	return "Lists the contents of a directory (default: the working directory). Use it to explore the persistent workspace and find intermediate results saved by earlier runs."
}

func (t *LsTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path": strProp("Directory to list (relative to the working directory, default '.')"),
	}, nil)
}

func (t *LsTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return "", fmt.Errorf("failed to parse input: %w", err)
		}
	}
	path := in.Path
	if path == "" {
		path = "."
	}
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(cwd, path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			names = append(names, name+"/")
			continue
		}
		if info, err := e.Info(); err == nil {
			name = fmt.Sprintf("%s (%dB)", name, info.Size())
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprintf("%s\n%s", path, strings.Join(names, "\n")), nil
}
