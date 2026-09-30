// Package agent is the harness loop (SPEC §3.4). A Session holds []Message;
// Run builds a Request with tool schemas, streams the completion, prints text
// deltas live, executes tool calls through the permission gate, appends the
// results, and repeats until no tools are requested or the step budget runs
// out. ctx threads throughout so Ctrl-C aborts in-flight requests.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"reasonix-mini/internal/permission"
	"reasonix-mini/internal/provider"
	"reasonix-mini/internal/tool"
)

// Agent wires one model endpoint to one tool registry and one gate.
type Agent struct {
	Provider    provider.Provider
	Tools       *tool.Registry
	Policy      permission.Policy
	ApproveLine func(prompt string) (string, error) // nil = headless: Ask→allow
	Grants      *permission.SessionGrants
	Temperature float64
	MaxSteps    int    // bounded loop budget
	System      string // cache-stable prefix, built once at startup

	messages []provider.Message // prepend-only session history, grows across turns
}

// Reset starts a fresh session; the previous transcript is the caller's to
// archive if it wants one (mini keeps none).
func (a *Agent) Reset() { a.messages = nil }

// Run executes one user turn through the loop.
func (a *Agent) Run(ctx context.Context, input string, out io.Writer) error {
	a.messages = append(a.messages, provider.Message{Role: provider.RoleUser, Content: input})
	return a.loop(ctx, out)
}

// loop is the whole magic of a coding agent, per SPEC §3.4.
func (a *Agent) loop(ctx context.Context, out io.Writer) error {
	for step := 0; step < a.MaxSteps; step++ {
		req := a.composeRequest()
		stream, err := a.Provider.Stream(ctx, req)
		if err != nil {
			return err
		}

		var (
			text      strings.Builder
			toolCalls []provider.ToolCall
		)
		for c := range stream {
			switch c.Type {
			case provider.ChunkText:
				text.WriteString(c.Text)
				if _, err := io.WriteString(out, c.Text); err != nil {
					return err
				}
			case provider.ChunkToolCall:
				toolCalls = append(toolCalls, *c.ToolCall)
			case provider.ChunkError:
				return c.Err
			case provider.ChunkDone:
			}
		}
		if text.Len() > 0 {
			fmt.Fprintln(out) // newline after the live-streamed deltas
		}

		if len(toolCalls) == 0 {
			// No tools: the turn is done. Record the answer and stop.
			a.messages = append(a.messages, provider.Message{Role: provider.RoleAssistant, Content: text.String()})
			return nil
		}

		// Record the assistant turn with its calls, then answer every call —
		// assistant tool_calls and tool results must stay paired on the wire.
		a.messages = append(a.messages, provider.Message{
			Role: provider.RoleAssistant, Content: text.String(), ToolCalls: toolCalls,
		})
		for _, tc := range toolCalls {
			result := a.executeTool(ctx, out, tc)
			a.messages = append(a.messages, provider.Message{
				Role: provider.RoleTool, ToolCallID: tc.ID, Name: tc.Name, Content: result,
			})
		}
	}
	return fmt.Errorf("agent: reached the max_steps budget (%d) without finishing", a.MaxSteps)
}

// composeRequest rebuilds the request from the stable prefix + growing
// prepend-only history. The system message and tool schemas are byte-stable
// across turns; only the tail changes (cache-first, SPEC §1).
func (a *Agent) composeRequest() provider.Request {
	tools := a.Tools.Tools()
	schemas := make([]provider.ToolSchema, 0, len(tools))
	for _, t := range tools {
		schemas = append(schemas, provider.ToolSchema{
			Name: t.Name(), Description: t.Description(), Parameters: t.Schema(),
		})
	}
	msgs := make([]provider.Message, 0, len(a.messages)+1)
	msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Content: a.System})
	msgs = append(msgs, a.messages...)
	return provider.Request{
		Messages:    msgs,
		Tools:       schemas,
		Temperature: a.Temperature,
	}
}

// executeTool gates and runs one call; every outcome comes back as a tool
// result string — errors, denials, and blocks are fed to the model so it can
// adapt rather than crash (SPEC §6).
func (a *Agent) executeTool(ctx context.Context, out io.Writer, tc provider.ToolCall) string {
	fmt.Fprintf(out, "\n⚙ %s %s\n", tc.Name, abbrev(tc.Arguments, 160))

	t := a.Tools.Get(tc.Name)
	if t == nil {
		return fmt.Sprintf("[blocked] unknown tool %q", tc.Name)
	}

	subject := permission.ExtractSubject(json.RawMessage(tc.Arguments))
	decision := a.Policy.Decide(tc.Name, t.ReadOnly(), json.RawMessage(tc.Arguments))
	if decision == permission.Ask && a.Grants != nil && a.Grants.Covers(tc.Name, subject) {
		decision = permission.Allow
	}
	if decision == permission.Ask {
		if a.ApproveLine == nil {
			// Headless: ordinary Ask resolves to Allow to stay autonomous.
			decision = permission.Allow
		} else {
			d, err := a.askUser(tc.Name, subject)
			if err != nil {
				return fmt.Sprintf("[blocked] approval prompt failed: %v", err)
			}
			decision = d
		}
	}
	switch decision {
	case permission.Deny:
		return fmt.Sprintf("[blocked] %s(%s) is denied by permission policy; pick another approach", tc.Name, subject)
	case permission.Ask:
		return "[blocked] the user did not answer the approval prompt"
	case permission.Allow:
	}

	result, err := t.Execute(ctx, json.RawMessage(tc.Arguments))
	if err != nil {
		if result != "" {
			return fmt.Sprintf("%s\n[tool error] %v", result, err)
		}
		return fmt.Sprintf("[tool error] %v", err)
	}
	return result
}

// askUser prompts allow once / session / always-equivalent / deny, mirroring
// the chat TUI's four answers (SPEC §3.7). "always" persists rules to config
// in the parent repo; mini upgrades it to a session grant and prints the
// rule the user should add to [permissions].
func (a *Agent) askUser(toolName, subject string) (permission.Decision, error) {
	prompt := fmt.Sprintf("Allow %s(%s)? [y]once / [s]session / [a]always / [n]deny: ", toolName, abbrev(subject, 80))
	answer, err := a.ApproveLine(prompt)
	if err != nil {
		return permission.Ask, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return permission.Allow, nil
	case "s":
		a.Grants.Remember(toolName, subject)
		return permission.Allow, nil
	case "a":
		a.Grants.Remember(toolName, subject)
		fmt.Printf("(mini does not write config; add %q to [permissions].allow to persist)\n", toolName+"("+subject+")")
		return permission.Allow, nil
	case "n", "no", "deny":
		return permission.Deny, nil
	default:
		return permission.Deny, nil // anything else is not consent
	}
}

func abbrev(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
