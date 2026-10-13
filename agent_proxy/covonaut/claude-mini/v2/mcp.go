package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/covoyage/covonaut/agentcore"
	"github.com/covoyage/covonaut/mcp"
)

// mcpServerConfig is one entry of an mcp.json "mcpServers" map. Servers using
// the stdio transport declare "command"/"args"/"env"; HTTP/SSE servers declare
// "url"/"headers". An entry with neither is skipped.
type mcpServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type mcpFile struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
}

// loadMCPExtensions reads a Claude Code / Cursor-shaped .mcp.json
// ({mcpServers:{...}}), expands ${VAR} / ${VAR:-default} in every string, and
// connects each server as a covonaut MCP extension. Tools are namespaced as
// mcp__<server>__<tool> so they never collide with the built-ins — the same
// convention v15 used. Connection is best-effort: a server that fails to start
// or handshake is skipped so it never costs the session.
//
// The returned extensions are attached to the agent's Config.Extensions; the
// agent disposes the underlying clients on Close. Servers live for the process
// lifetime and are reaped when ctx is cancelled at exit.
func loadMCPExtensions(ctx context.Context, path string) []agentcore.Extension {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warn: mcp config: %v\n", err)
		}
		return nil
	}
	var f mcpFile
	if err := json.Unmarshal(data, &f); err != nil {
		fmt.Fprintf(os.Stderr, "warn: mcp config parse: %v\n", err)
		return nil
	}
	if len(f.MCPServers) == 0 {
		return nil
	}

	names := make([]string, 0, len(f.MCPServers))
	for n := range f.MCPServers {
		names = append(names, n)
	}
	sort.Strings(names)

	var exts []agentcore.Extension
	totalTools := 0
	for _, name := range names {
		s := f.MCPServers[name]
		prefix := "mcp__" + name + "__"
		switch {
		case strings.TrimSpace(s.Command) != "":
			ext, err := mcp.NewStdioExtension(ctx, mcp.StdioConfig{
				Name:       name,
				Command:    expandEnv(s.Command),
				Args:       expandAllEnv(s.Args),
				Env:        expandEnvPairs(s.Env),
				ToolPrefix: prefix,
				Dir:        sandboxFlag,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: mcp server %q skipped: %v\n", name, err)
				continue
			}
			totalTools += len(ext.Tools())
			exts = append(exts, ext)
		case strings.TrimSpace(s.URL) != "":
			ext, err := mcp.NewHTTPExtension(ctx, mcp.HTTPConfig{
				Name:       name,
				Endpoint:   expandEnv(s.URL),
				ToolPrefix: prefix,
				Headers:    expandEnvMap(s.Headers),
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: mcp http server %q skipped: %v\n", name, err)
				continue
			}
			totalTools += len(ext.Tools())
			exts = append(exts, ext)
		default:
			fmt.Fprintf(os.Stderr, "warn: mcp server %q has neither command nor url, skipped\n", name)
		}
	}
	if verboseFlag {
		fmt.Fprintf(os.Stderr, "mcp: %d server tools registered from %s (%d servers)\n",
			totalTools, path, len(exts))
	}
	return exts
}

// expandAllEnv applies ${VAR} / ${VAR:-default} expansion to a string slice.
func expandAllEnv(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = expandEnv(s)
	}
	return out
}

// expandEnvPairs turns a {KEY:VALUE} map into the KEY=VALUE slice the stdio
// client expects, expanding ${...} references in both halves.
func expandEnvPairs(in map[string]string) []string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, expandEnv(k)+"="+expandEnv(in[k]))
	}
	return out
}

// expandEnvMap expands values only, keeping keys literal (HTTP headers).
func expandEnvMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = expandEnv(v)
	}
	return out
}

// expandEnv substitutes ${NAME} and ${NAME:-default} from the environment.
func expandEnv(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			expr := s[i+2 : i+end]
			name, def, hasDef := expr, "", false
			if p := strings.Index(expr, ":-"); p >= 0 {
				name, def, hasDef = expr[:p], expr[p+2:], true
			}
			val := os.Getenv(name)
			if val == "" && hasDef {
				val = def
			}
			b.WriteString(val)
			i += end + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
