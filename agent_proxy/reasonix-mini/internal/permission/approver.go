package permission

// Package-level gating contract: the agent consults a Policy plus an optional
// interactive line-reader at execute time. mini threads the prompt through a
// func(prompt) (answer, error) callback so the REPL and the approval share
// one stdin reader; a nil callback means a non-interactive run, where
// ordinary Ask resolves to allow to preserve autonomous behaviour, exactly
// like the parent repo's headless contract (SPEC §3.7).

// SessionGrants remembers "allow for this session" answers, keyed by the same
// family/specifier shape config rules use, so similar calls stop prompting.
type SessionGrants struct {
	grants map[string]bool
}

// NewSessionGrants returns an empty grant set.
func NewSessionGrants() *SessionGrants {
	return &SessionGrants{grants: map[string]bool{}}
}

// Remember stores a session-scope approval for `Family(specifier)`.
func (g *SessionGrants) Remember(toolName, subject string) {
	g.grants[sessionKey(toolName, subject)] = true
}

// Covers reports whether a previous session approval covers this call.
func (g *SessionGrants) Covers(toolName, subject string) bool {
	return g.grants[sessionKey(toolName, subject)]
}

func sessionKey(toolName, subject string) string {
	fam := family(toolName)
	if subject == "" {
		return fam
	}
	return fam + "(" + subject + ")"
}
