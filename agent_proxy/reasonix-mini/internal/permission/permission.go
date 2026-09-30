// Package permission implements per-tool-call gating (SPEC §3.7): a pure
// static Policy plus an optional interactive Approver. It is independent of
// the model and of the CLI. Precedence: deny > ask > allow > fallback.
package permission

import (
	"encoding/json"
	"strings"
)

// Decision is the gate's answer for one tool call.
type Decision int

const (
	Allow Decision = iota
	Ask
	Deny
)

// Rule is `Tool` or `Tool(specifier)`. Mini implements the shipped subset:
// bash prefix rules `Bash(go test:*)`, exact rules `Bash=<literal>`, and
// glob specifiers for file families (Edit(docs/**)).
type Rule struct {
	Family    string
	Specifier string
	Exact     bool // Bash="literal" form: only the identical complete command matches
}

// ParseRule splits one configured rule string.
func ParseRule(s string) Rule {
	if i := strings.Index(s, `="`); i >= 0 && strings.HasSuffix(s, "\"") && i+2 < len(s) {
		// Bash="literal" exact form
		return Rule{Family: strings.ToLower(s[:i]), Specifier: s[i+2 : len(s)-1], Exact: true}
	}
	if open := strings.Index(s, "("); open >= 0 && strings.HasSuffix(s, ")") {
		fam := s[:open]
		spec := s[open+1 : len(s)-1]
		return Rule{Family: strings.ToLower(fam), Specifier: spec}
	}
	return Rule{Family: strings.ToLower(s)}
}

// Policy is the pure, no-I/O static rule set.
type Policy struct {
	Mode  Decision // writer fallback when no rule matches
	Allow []Rule
	Ask   []Rule
	Deny  []Rule
}

// FromConfig builds a Policy from mode string and rule strings.
func FromConfig(mode string, allow, ask, deny []string) Policy {
	return Policy{
		Mode:  parseMode(mode),
		Allow: parseRules(allow),
		Ask:   parseRules(ask),
		Deny:  parseRules(deny),
	}
}

func parseMode(s string) Decision {
	switch strings.ToLower(s) {
	case "allow":
		return Allow
	case "deny":
		return Deny
	default:
		return Ask
	}
}

func parseRules(in []string) []Rule {
	out := make([]Rule, 0, len(in))
	for _, s := range in {
		out = append(out, ParseRule(s))
	}
	return out
}

// Decide evaluates the static rules against one tool call.
func (p Policy) Decide(toolName string, readOnly bool, args json.RawMessage) Decision {
	subject := ExtractSubject(args)
	if matchAny(p.Deny, toolName, subject) {
		return Deny
	}
	if matchAny(p.Ask, toolName, subject) {
		return Ask
	}
	if matchAny(p.Allow, toolName, subject) {
		return Allow
	}
	if readOnly {
		return Allow // fallback: read-only tools are always eligible readers
	}
	return p.Mode // fallback: writer posture from config
}

func matchAny(rules []Rule, toolName, subject string) bool {
	fam := family(toolName)
	for _, r := range rules {
		if r.Family != fam {
			continue
		}
		if r.Specifier == "" {
			return true // bare Tool matches any call in the family
		}
		if subject == "" {
			continue // a rule with a specifier cannot match an unknown subject
		}
		if r.Specifier == subject {
			return true // legacy form also matches the complete string
		}
		if r.Exact {
			continue // exact form: only the identical complete command matches
		}
		if strings.HasSuffix(r.Specifier, ":*") {
			prefix := strings.TrimSuffix(r.Specifier, ":*")
			// A remembered prefix approval must not cover commands that
			// introduce shell operators (SPEC §3.7).
			if strings.HasPrefix(subject, prefix) && !hasShellOperator(subject[len(prefix):]) {
				return true
			}
			continue
		}
		if strings.ContainsRune(r.Specifier, '*') || strings.ContainsRune(r.Specifier, '?') {
			if globMatch(strings.ToLower(r.Specifier), strings.ToLower(subject)) {
				return true
			}
			continue
		}
		if strings.HasPrefix(subject, r.Specifier) {
			return true // legacy plain-prefix form, e.g. Bash(npm run test)
		}
	}
	return false
}

func hasShellOperator(s string) bool {
	return strings.ContainsAny(s, "&|;`$<>\n")
}

// family normalizes tool names into Claude Code-style approval families:
// every mutating file tool shares "edit", bash is "bash", everything else is
// its own lowercase name.
func family(toolName string) string {
	switch toolName {
	case "bash":
		return "bash"
	case "write_file", "edit_file", "move_file":
		return "edit"
	default:
		return strings.ToLower(toolName)
	}
}

// ExtractSubject reads the call's subject generically from known JSON argument
// keys — command (bash), path/file_path/source (file tools), pattern
// (grep/glob) — so tools need not change (SPEC §3.7).
func ExtractSubject(args json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return ""
	}
	for _, key := range []string{"command", "path", "file_path", "source", "pattern"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// globMatch is a separator-crossing match used by Edit(docs/**)-style rules.
// Unlike the tool-side glob it treats ** as "any characters".
func globMatch(pattern, s string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	parts := strings.Split(pattern, "*")
	rest := s
	for i, p := range parts {
		if p == "" {
			continue
		}
		idx := strings.Index(rest, p)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false // leading literal must anchor at the start
		}
		rest = rest[idx+len(p):]
	}
	return true
}
