package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// agentDir is the durable experience folder inside the workspace.
// It lives under the sandbox root when one is configured.
func agentDir() string {
	if env := os.Getenv("AGENT_HOME"); env != "" { // test override
		return env
	}
	base := "."
	if root := SandboxRoot(); root != "" {
		base = root
	}
	return filepath.Join(base, ".agent")
}

// memoryFile is the append-only experience log.
func memoryFile() string { return filepath.Join(agentDir(), "memory.md") }

// skillsDir holds one markdown file per reusable skill.
func skillsDir() string { return filepath.Join(agentDir(), "skills") }

// ReadMemory returns the persisted memory file ("" when none yet).
// The engine injects this into the system prompt every turn.
func ReadMemory() string {
	data, err := os.ReadFile(memoryFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SkillIndex lists available skills for the system prompt.
func SkillIndex() string {
	entries, err := os.ReadDir(skillsDir())
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// MemoryTool saves and reads the agent's long-term memory.
type MemoryTool struct{}

func NewMemoryTool() *MemoryTool { return &MemoryTool{} }

func (t *MemoryTool) Name() string { return "Memory" }

func (t *MemoryTool) Description() string {
	return "Persists reusable experience. action=save appends a dated note to long-term memory (project facts, user preferences, lessons learned) that is re-injected into every future session; action=list returns the stored memory."
}

func (t *MemoryTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"action": map[string]interface{}{"type": "string", "enum": []string{"save", "list"}, "description": "save appends a fact; list returns current memory"},
		"fact":   strProp("The fact or lesson to remember (short, one sentence)"),
	}, []string{"action"})
}

func (t *MemoryTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Action string `json:"action"`
		Fact   string `json:"fact"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	switch in.Action {
	case "list":
		mem := ReadMemory()
		if mem == "" {
			return "(memory is empty)", nil
		}
		return mem, nil
	case "save":
		if in.Fact == "" {
			return "", mustErr("missing required field: fact")
		}
		if err := os.MkdirAll(agentDir(), 0o755); err != nil {
			return "", err
		}
		f, err := os.OpenFile(memoryFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		defer f.Close()
		line := fmt.Sprintf("- [%s] %s\n", time.Now().Format("2006-01-02"), in.Fact)
		if _, err := f.WriteString(line); err != nil {
			return "", err
		}
		return "saved to long-term memory", nil
	default:
		return "", mustErr("action must be save or list")
	}
}

// SkillTool writes and reads reusable procedures learned by the agent.
type SkillTool struct{}

func NewSkillTool() *SkillTool { return &SkillTool{} }

func (t *SkillTool) Name() string { return "Skill" }

func (t *SkillTool) Description() string {
	return "Reusable procedures. action=save writes a step-by-step recipe (learned from finishing a task) to .agent/skills/<name>.md; action=use returns that recipe so you can follow it. Available skills are listed in the system prompt."
}

func (t *SkillTool) InputSchema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"action":  map[string]interface{}{"type": "string", "enum": []string{"save", "use"}, "description": "save a new skill or use an existing one"},
		"name":    strProp("Skill name (kebab-case, becomes the file name)"),
		"content": strProp("Markdown steps of the recipe (required for save)"),
	}, []string{"action", "name"})
}

func (t *SkillTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if in.Name == "" || strings.ContainsAny(in.Name, "/\\.") {
		return "", mustErr("name must be kebab-case without dots or slashes")
	}
	path := filepath.Join(skillsDir(), in.Name+".md")
	switch in.Action {
	case "use":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("no skill %q (see the skill index in the system prompt)", in.Name)
		}
		return string(data), nil
	case "save":
		if in.Content == "" {
			return "", mustErr("missing required field: content")
		}
		if err := os.MkdirAll(skillsDir(), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("skill %q saved to %s", in.Name, path), nil
	default:
		return "", mustErr("action must be save or use")
	}
}
