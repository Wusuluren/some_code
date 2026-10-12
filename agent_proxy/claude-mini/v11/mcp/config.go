package mcp

import (
	"encoding/json"
	"os"
	"strings"
)

// ServerConfig is one entry of an mcp.json "mcpServers" map. The minimal
// v10 client speaks the stdio transport only; url-based HTTP servers are
// recognised but skipped.
type ServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}

type fileConfig struct {
	MCPServers map[string]struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	} `json:"mcpServers"`
}

// LoadConfig reads Claude-Cour/Cursor-shaped mcp.json ({mcpServers: {...}})
// and expands ${VAR} / ${VAR:-default} in every string field.
func LoadConfig(path string) (map[string]ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	out := make(map[string]ServerConfig)
	for name, s := range fc.MCPServers {
		if s.Command == "" {
			continue // HTTP/url servers not supported by the minimal client
		}
		cfg := ServerConfig{Command: expand(s.Command), Args: expandAll(s.Args), Env: map[string]string{}}
		for k, v := range s.Env {
			cfg.Env[expand(k)] = expand(v)
		}
		out[name] = cfg
	}
	return out, nil
}

func expandAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = expand(s)
	}
	return out
}

// expand substitutes ${NAME} and ${NAME:-default} from the environment.
func expand(s string) string {
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
