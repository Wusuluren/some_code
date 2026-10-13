package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/covoyage/covonaut/agentcore"
	"github.com/covoyage/covonaut/tui/component"
)

// --- plan data model ---

type planStep struct {
	Description  string   `json:"description"`
	Files        []string `json:"files,omitempty"`
	Verification string   `json:"verification,omitempty"`
}

type miniPlan struct {
	Goal  string     `json:"goal"`
	Steps []planStep `json:"steps"`
	Notes string     `json:"notes,omitempty"`
}

// planPrompt drives the read-only planning pass. Output shape is enforced by
// DecodeStructured downstream; the prompt keeps the model to a bare JSON
// object so every provider (not just JSON-mode ones) works.
const planPrompt = `You are a planning agent with READ-ONLY tools (read/grep/glob/find/ls).
Investigate the workspace as much as needed, then produce an implementation plan.
Respond with ONLY a JSON object, no prose and no code fences, shaped:
{"goal": "<one sentence>", "steps": [{"description": "<imperative step>", "files": ["<path>"], "verification": "<how to check it works>"}], "notes": "<optional risks or context>"}
Keep steps small and ordered. Every step must be concrete enough to code from.`

// --- planning agent ---

// buildPlan runs a fresh read-only agent over the request and returns the
// decoded plan plus its raw JSON (kept for debugging/verbose output).
func buildPlan(ctx context.Context, provider agentcore.Provider, model, request string) (*miniPlan, string, error) {
	cfg := agentcore.NewConfig(
		agentcore.WithName("planner"),
		agentcore.WithModel(model),
		agentcore.WithProvider(provider),
		agentcore.WithSystemPrompt(planPrompt),
		agentcore.WithMaxTurns(int64(maxTurnsFlag)),
		agentcore.WithValidateArguments(true),
		agentcore.WithTools(workerTools()...),
	)
	planner := agentcore.New(cfg)
	defer planner.Close()

	raw, err := planner.Run(ctx, "Investigate the workspace, then plan for this request:\n"+request)
	if err != nil {
		return nil, "", fmt.Errorf("planning run failed: %w", err)
	}
	p, err := agentcore.DecodeStructured[miniPlan](extractJSONObject(raw))
	if err != nil {
		return nil, raw, fmt.Errorf("planner did not return a valid plan JSON: %w", err)
	}
	if len(p.Steps) == 0 {
		return nil, raw, fmt.Errorf("planner returned an empty plan (no steps)")
	}
	return &p, raw, nil
}

// extractJSONObject tolerates prose/fence wrapping: takes the outermost
// {...} span. Without it, DecodeStructured trips on ```json wrappers.
func extractJSONObject(s string) string {
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return s
	}
	return s[start : end+1]
}

func formatPlan(p *miniPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\033[1;33m◆ Plan:\033[0m %s\n", p.Goal)
	for i, st := range p.Steps {
		fmt.Fprintf(&b, "  \033[1m%d.\033[0m %s\n", i+1, st.Description)
		if len(st.Files) > 0 {
			fmt.Fprintf(&b, "     \033[2mfiles: %s\033[0m\n", strings.Join(st.Files, ", "))
		}
		if st.Verification != "" {
			fmt.Fprintf(&b, "     \033[2mverify: %s\033[0m\n", st.Verification)
		}
	}
	if p.Notes != "" {
		fmt.Fprintf(&b, "  \033[2mnotes: %s\033[0m\n", p.Notes)
	}
	return b.String()
}

// --- approval gate ---

// runPlanGate plans, shows the result, and asks the human for consent.
// Print mode auto-approves (warns, mirroring the tool guard); REPL loops on
// y/N. On approval the plan steps are seeded into the todo list as pending, so
// the execution phase starts with a live checklist the model must keep current.
// Returns the execution prompt, or approved=false when rejected/failed.
func runPlanGate(ctx context.Context, provider agentcore.Provider, model, request string) (string, bool) {
	fmt.Fprintf(os.Stderr, "\n\033[1;35m── plan mode: investigating (read-only) ──\033[0m\n")
	p, raw, err := buildPlan(ctx, provider, model, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan failed: %v\n", err)
		if verboseFlag && raw != "" {
			fmt.Fprintf(os.Stderr, "raw planner output:\n%s\n", truncate(raw, 2000))
		}
		return "", false
	}
	planText := formatPlan(p)
	fmt.Fprint(os.Stderr, planText)

	approved := false
	if printMode || tuiMode {
		fmt.Fprintf(os.Stderr, "warn: %s mode auto-approves the plan (no interactive prompt)\n", modeName())
		approved = true
	} else {
		sc := bufio.NewScanner(os.Stdin)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for {
			fmt.Fprint(os.Stderr, "approve plan and execute? [y]es / [N]o: ")
			if !sc.Scan() {
				fmt.Fprintln(os.Stderr, "no answer (stdin closed), plan not approved")
				return "", false
			}
			switch strings.ToLower(strings.TrimSpace(sc.Text())) {
			case "y", "yes":
				approved = true
			case "", "n", "no":
				fmt.Fprintln(os.Stderr, "plan rejected — start a new attempt or run without -plan.")
				return "", false
			default:
				continue
			}
			break
		}
	}
	if !approved {
		return "", false
	}

	// Seed the checklist: everything pending except step 1, in progress.
	items := make([]component.TodoItem, 0, len(p.Steps))
	for i, st := range p.Steps {
		status := "pending"
		if i == 0 {
			status = "in_progress"
		}
		items = append(items, component.TodoItem{
			ID:      fmt.Sprintf("p%d", i+1),
			Content: st.Description,
			Status:  status,
		})
	}
	todoSetItems(items)

	execPrompt := "The user APPROVED this plan — execute it step by step, in order:\n\n" +
		strings.Join(planLines(p), "\n") +
		"\n\nOriginal request: " + request +
		"\nKeep the todo list current (one in_progress at a time); adapt it if reality differs from the plan."
	return execPrompt, true
}

// planLines renders the plan compactly for embedding into the exec prompt.
func planLines(p *miniPlan) []string {
	out := []string{"Goal: " + p.Goal}
	for i, st := range p.Steps {
		line := fmt.Sprintf("%d. %s", i+1, st.Description)
		if len(st.Files) > 0 {
			line += "  [files: " + strings.Join(st.Files, ", ") + "]"
		}
		if st.Verification != "" {
			line += "  [verify: " + st.Verification + "]"
		}
		out = append(out, line)
	}
	if p.Notes != "" {
		out = append(out, "Notes: "+p.Notes)
	}
	return out
}
