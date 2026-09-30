package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&glob{}) }

type glob struct{}

const globSchema = `{"type":"object","properties":{` +
	`"pattern":{"type":"string","description":"Glob pattern, ** matches across directories (e.g. internal/**/*.go)"},` +
	`"root":{"type":"string","description":"Directory to search under (default: workspace root)"}},"required":["pattern"]}`

func (glob) Name() string            { return "glob" }
func (glob) Schema() json.RawMessage { return json.RawMessage(globSchema) }
func (glob) ReadOnly() bool          { return true }
func (glob) Description() string {
	return "Find files by glob pattern. Returns matching paths relative to the root."
}

const globCap = 2000 // same cap as the parent repo's Glob tool

func (glob) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Pattern string `json:"pattern"`
		Root    string `json:"root"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("glob: bad args: %w", err)
	}
	if in.Pattern == "" {
		return "", fmt.Errorf("glob: pattern is required")
	}
	if in.Root == "" {
		in.Root = "."
	}
	pattern := filepath.Clean(in.Pattern)

	var matches []string
	walkErr := fmt.Errorf("glob: stopped: cap %d reached", globCap)
	err := filepath.WalkDir(in.Root, func(p string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if noiseDirs[d.Name()] && p != in.Root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(in.Root, p)
		if rerr != nil {
			return nil
		}
		if matchGlob(pattern, filepath.ToSlash(rel)) {
			matches = append(matches, rel)
			if len(matches) >= globCap {
				return walkErr
			}
		}
		return nil
	})
	if err != nil && err != walkErr {
		return "", err
	}
	if len(matches) == 0 {
		return "no files matched", nil
	}
	return strings.Join(matches, "\n"), nil
}

// matchGlob implements the doublestar subset we advertise: `**/` crosses
// directories, `*` and `?` stay within one path segment. Matching walks
// segment by segment — pure code, no dependency (SPEC §1.3 lean dependencies).
func matchGlob(pattern, name string) bool {
	return matchSegs(split(pattern), split(name))
}

func split(p string) []string {
	if p == "." { // WalkDir/Rel produce "."-prefixed or bare paths; clean keeps segments honest
		p = ""
	}
	return strings.Split(p, "/")
}

func matchSegs(pat, s []string) bool {
	if len(pat) == 0 {
		return len(s) == 0
	}
	if pat[0] == "**" {
		// ** swallows zero or more whole segments
		for i := 0; i <= len(s); i++ {
			if matchSegs(pat[1:], s[i:]) {
				return true
			}
		}
		return false
	}
	if len(s) == 0 {
		return false
	}
	if !matchWord(pat[0], s[0]) {
		return false
	}
	return matchSegs(pat[1:], s[1:])
}

// matchWord matches one path segment against a pattern segment containing
// only * and ? wildcards (never crosses a separator).
func matchWord(pat, s string) bool {
	if pat == "" {
		return s == ""
	}
	switch pat[0] {
	case '*':
		for i := 0; i <= len(s); i++ {
			if matchWord(pat[1:], s[i:]) {
				return true
			}
		}
		return false
	case '?':
		return s != "" && matchWord(pat[1:], s[1:])
	default:
		return s != "" && pat[0] == s[0] && matchWord(pat[1:], s[1:])
	}
}
