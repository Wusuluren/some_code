package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool searches file contents by regex, mirroring internal/tools.GrepTool.
// Implemented in pure Go (WalkDir + regexp) so it needs no rg/grep binary.
type GrepTool struct{}

func NewGrepTool() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string { return "Grep" }

func (t *GrepTool) Description() string {
	return "Searches file contents for a regular expression pattern and returns matching lines as path:line:text. Skips .git, node_modules and other heavy directories. Caps results at 200 matches."
}

func (t *GrepTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"pattern": strProp("The regular expression pattern to search for (Go regexp syntax)"),
		"path":    strProp("The directory to search in (defaults to the working directory)"),
		"glob":    strProp("Optional glob to filter file names, e.g. \"*.go\""),
		"-i":      map[string]interface{}{"type": "boolean", "description": "Case-insensitive search"},
	}, []string{"pattern"})
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"target": true, "vendor": true, ".idea": true, "__pycache__": true,
}

const maxGrepResults = 200

func (t *GrepTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
		Ignore  bool   `json:"-i"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.Pattern == "" {
		return "", mustErr("missing required field: pattern")
	}

	pattern := in.Pattern
	if in.Ignore {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex %q: %w", in.Pattern, err)
	}

	root := in.Path
	if root == "" {
		root, _ = os.Getwd()
	}

	var b strings.Builder
	matches := 0
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if in.Glob != "" {
			ok, _ := filepath.Match(in.Glob, d.Name())
			if !ok {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if !re.MatchString(line) {
				continue
			}
			matches++
			if matches > maxGrepResults {
				return filepath.SkipAll
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				rel = path
			}
			if len(line) > 400 {
				line = line[:400] + "..."
			}
			fmt.Fprintf(&b, "%s:%d:%s\n", rel, lineNo, line)
		}
		return nil
	})
	if walkErr != nil && walkErr != ctx.Err() {
		return "", fmt.Errorf("search failed: %w", walkErr)
	}
	if matches == 0 {
		return "No matches found.", nil
	}
	if matches > maxGrepResults {
		fmt.Fprintf(&b, "... (results capped at %d matches)\n", maxGrepResults)
	}
	return b.String(), nil
}
