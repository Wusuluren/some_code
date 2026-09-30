package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// confine is the skeleton-grade cousin of the parent repo's [sandbox]
// workspace_root (SPEC §5): a mutation target resolved to an absolute,
// symlink-free path must stay inside the launch cwd, so ".." or a symlinked
// dir cannot tunnel out. Read tools are not confined in mini.
func confine(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	// Eval the nearest existing ancestor so a not-yet-created file can still
	// be checked without a full symlink walk of nonexistent children.
	dir := filepath.Dir(abs)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		abs = filepath.Join(real, filepath.Base(abs))
	}
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(root, abs); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q is outside the workspace root %q (refused by confine)", path, root)
	}
	return abs, nil
}
