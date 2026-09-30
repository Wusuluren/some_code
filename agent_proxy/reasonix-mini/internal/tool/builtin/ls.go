package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&ls{}) }

type ls struct{}

const lsSchema = `{"type":"object","properties":{` +
	`"path":{"type":"string","description":"Directory to list (default: workspace root)"},` +
	`"depth":{"type":"integer","description":"Recursion depth, default 1"}},"required":[]}`

func (ls) Name() string            { return "ls" }
func (ls) Schema() json.RawMessage { return json.RawMessage(lsSchema) }
func (ls) ReadOnly() bool          { return true }
func (ls) Description() string {
	return "List a directory tree, depth-first, skipping common noise dirs (.git, node_modules, ...)."
}

// noiseDirs mirrors the parent repo's directory-listing skips (SPEC §3.9).
var noiseDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".reasonix": true,
}

func (ls) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("ls: bad args: %w", err)
	}
	if in.Path == "" {
		in.Path = "."
	}
	if in.Depth <= 0 {
		in.Depth = 1
	}
	info, err := os.Stat(in.Path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return fmt.Sprintf("%s (file, %d bytes)", in.Path, info.Size()), nil
	}

	var sb strings.Builder
	err = filepath.WalkDir(in.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable corners are skipped, not fatal
		}
		rel, rerr := filepath.Rel(in.Path, p)
		if rerr != nil {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if rel == "." {
			return nil // the listing root itself is not printed
		}
		if d.IsDir() && depth >= in.Depth {
			return filepath.SkipDir
		}
		if d.IsDir() && noiseDirs[d.Name()] {
			return filepath.SkipDir
		}
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			fmt.Fprintf(&sb, "%s%s/\n", indent, d.Name())
		} else {
			fi, _ := d.Info()
			fmt.Fprintf(&sb, "%s%s (%d bytes)\n", indent, d.Name(), fi.Size())
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if sb.Len() == 0 {
		return "(empty directory)", nil
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
