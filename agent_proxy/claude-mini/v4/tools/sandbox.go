package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// sandboxRoot is the base directory every tool is confined to.
// Empty means "no sandbox": fall back to the process cwd (v1 behavior).
var sandboxRoot string

// SetSandboxRoot confines all file tools and Bash to root.
func SetSandboxRoot(root string) {
	sandboxRoot = root
}

// SandboxRoot returns the configured sandbox root ("" when disabled).
func SandboxRoot() string { return sandboxRoot }

// sandboxBase is the boundary confineToCwd checks against. Symlinks are
// resolved (macOS /var -> /private/var) so real paths compare correctly.
func sandboxBase() (string, error) {
	base := sandboxRoot
	if base == "" {
		var err error
		if base, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	return filepath.EvalSymlinks(base)
}

// sensitiveEnv reports whether an env var must not leak into sandboxed
// commands (API keys, cloud credentials, OAuth tokens).
func sensitiveEnv(name string) bool {
	for _, w := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "CRED", "AUTH"} {
		if strings.Contains(strings.ToUpper(name), w) {
			return true
		}
	}
	return false
}

// ScrubEnv filters credentials out of the environment handed to Bash.
func ScrubEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 && sensitiveEnv(kv[:i]) {
			continue
		}
		out = append(out, kv)
	}
	return out
}
