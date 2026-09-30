package main_test

import (
	"testing"

	"reasonix-mini/internal/permission"
)

func TestPolicyPrecedence(t *testing.T) {
	p := permission.FromConfig("ask",
		[]string{"Bash(go test:*)", "Bash(git status:*)"},
		[]string{},
		[]string{"Bash(rm -rf*)", `Bash="echo safe"`},
	)
	cases := []struct {
		tool string
		read bool
		args string
		want permission.Decision
		name string
	}{
		{"read_file", true, `{"path":"a.go"}`, permission.Allow, "reader fallback"},
		{"write_file", false, `{"path":"a.go"}`, permission.Ask, "writer fallback ask"},
		{"bash", false, `{"command":"go test ./..."}`, permission.Allow, "prefix rule allows"},
		// A prefix approval must not cover commands that introduce shell operators:
		{"bash", false, `{"command":"go test ./... && rm -rf tmp"}`, permission.Ask, "operators break prefix"},
		{"bash", false, `{"command":"rm -rf /"}`, permission.Deny, "deny wins over all"},
		{"bash", false, `{"command":"echo safe"}`, permission.Deny, "exact literal deny matches identically"},
	}
	for _, c := range cases {
		if got := p.Decide(c.tool, c.read, []byte(c.args)); got != c.want {
			t.Errorf("%s: Decide(%s %s) = %v, want %v", c.name, c.tool, c.args, got, c.want)
		}
	}
}

func TestDenyBeatsAllow(t *testing.T) {
	p := permission.FromConfig("allow", []string{"Bash"}, []string{}, []string{"Bash(rm -rf*)"})
	if d := p.Decide("bash", false, []byte(`{"command":"rm -rf ."}`)); d != permission.Deny {
		t.Errorf("deny must always win, got %v", d)
	}
	if d := p.Decide("bash", false, []byte(`{"command":"ls"}`)); d != permission.Allow {
		t.Errorf("broad allow should pass clean command, got %v", d)
	}
}

func TestFileFamilySharesEditGrants(t *testing.T) {
	p := permission.FromConfig("ask", []string{"Edit(docs/**)"}, []string{}, []string{})
	for _, toolName := range []string{"write_file", "edit_file", "move_file"} {
		args := `{"path":"docs/a.md"}`
		if toolName == "move_file" {
			args = `{"source":"docs/a.md"}`
		}
		if d := p.Decide(toolName, false, []byte(args)); d != permission.Allow {
			t.Errorf("%s: Edit(docs/**) should cover all file mutators, got %v", toolName, d)
		}
	}
	if d := p.Decide("write_file", false, []byte(`{"path":"src/main.go"}`)); d != permission.Ask {
		t.Errorf("outside docs/ must still ask, got %v", d)
	}
}

func TestSubjectlessRuleMatchesOnlyBareFamily(t *testing.T) {
	p := permission.FromConfig("ask", []string{"Bash(npm run build)"}, []string{}, []string{})
	// No glob/prefix form: legacy plain-prefix match on the subject.
	if d := p.Decide("bash", false, []byte(`{"command":"npm run build --verbose"}`)); d != permission.Allow {
		t.Errorf("legacy prefix should match, got %v", d)
	}
	// Unknown subject: a rule with a specifier cannot match.
	if d := p.Decide("bash", false, []byte(`{}`)); d != permission.Ask {
		t.Errorf("bare call without subject must fall back, got %v", d)
	}
}
