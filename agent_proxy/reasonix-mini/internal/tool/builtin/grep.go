package builtin

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

	"reasonix-mini/internal/tool"
)

func init() { tool.RegisterBuiltin(&grep{}) }

type grep struct{}

const grepSchema = `{"type":"object","properties":{` +
	`"pattern":{"type":"string","description":"Regular expression (RE2 syntax) to search for"},` +
	`"path":{"type":"string","description":"File or directory to search (default: workspace root)"},` +
	`"glob":{"type":"string","description":"Only search files matching this glob (e.g. **/*.go)"},` +
	`"case_insensitive":{"type":"boolean","description":"Case-insensitive matching (default false)"}},"required":["pattern"]}`

func (grep) Name() string            { return "grep" }
func (grep) Schema() json.RawMessage { return json.RawMessage(grepSchema) }
func (grep) ReadOnly() bool          { return true }
func (grep) Description() string {
	return "Search file contents with a regular expression; returns path:line:text matches."
}

const (
	grepFileCap  = 2000    // files searched per call
	grepLineCap  = 500     // result lines per call
	grepMaxLine  = 200     // per-line truncation width
	grepMaxBytes = 2 << 20 // skip files larger than this
)

func (grep) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Pattern         string `json:"pattern"`
		Path            string `json:"path"`
		Glob            string `json:"glob"`
		CaseInsensitive bool   `json:"case_insensitive"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("grep: bad args: %w", err)
	}
	if in.Pattern == "" {
		return "", fmt.Errorf("grep: pattern is required")
	}
	expr := in.Pattern
	if in.CaseInsensitive {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("grep: bad regex: %w", err) // fed back so the model can fix it
	}
	if in.Path == "" {
		in.Path = "."
	}

	var sb strings.Builder
	matched := 0
	files := 0
	walkErr := fmt.Errorf("grep: stopped: result cap %d reached", grepLineCap)
	scanErr := filepath.WalkDir(in.Path, func(p string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil // unreadable corners are skipped, not fatal
		}
		if d.IsDir() {
			if noiseDirs[d.Name()] && p != in.Path {
				return filepath.SkipDir
			}
			return nil
		}
		if in.Glob != "" {
			rel, rerr := filepath.Rel(in.Path, p)
			if rerr != nil || !matchGlob(filepath.Clean(in.Glob), filepath.ToSlash(rel)) {
				return nil
			}
		}
		fi, ierr := d.Info()
		if ierr != nil || fi.Size() > grepMaxBytes {
			return nil
		}
		files++
		if files > grepFileCap {
			return nil
		}
		f, oerr := os.Open(p)
		if oerr != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		lineNo := 0
		for sc.Scan() {
			line := sc.Text()
			if isBinarySample(line) {
				break // don't dump binary content
			}
			lineNo++
			if !re.MatchString(line) {
				continue
			}
			matched++
			out := line
			if len(out) > grepMaxLine {
				out = out[:grepMaxLine] + "…"
			}
			fmt.Fprintf(&sb, "%s:%d:%s\n", p, lineNo, strings.TrimRight(out, "\r"))
			if matched >= grepLineCap {
				return walkErr
			}
		}
		return nil
	})
	if scanErr != nil && scanErr != walkErr {
		return "", scanErr
	}
	if matched == 0 {
		return "no matches found", nil
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// isBinarySample is a cheap line-level heuristic: NUL bytes never appear in
// the text files this tool is meant to search.
func isBinarySample(s string) bool { return strings.IndexByte(s, 0) >= 0 }
