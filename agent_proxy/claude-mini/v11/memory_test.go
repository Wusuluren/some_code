package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-code-go/v11/tools"
)

// TestMemoryAndSkills covers the v4 additions: Memory persists facts,
// Skill stores reusable recipes, and both surface via the helpers the
// system prompt injection uses.
func TestMemoryAndSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_HOME", home)
	ctx := context.Background()

	// Memory save appends a dated fact.
	mem := tools.NewMemoryTool()
	out, err := mem.Call(ctx, json.RawMessage(`{"action":"save","fact":"repo uses go 1.26, build with env -u GOROOT"}`))
	if err != nil || !strings.Contains(out, "saved") {
		t.Fatalf("Memory save: %q %v", out, err)
	}
	out, err = mem.Call(ctx, json.RawMessage(`{"action":"list"}`))
	if err != nil || !strings.Contains(out, "env -u GOROOT") {
		t.Fatalf("Memory list: %q %v", out, err)
	}
	if stored := tools.ReadMemory(); !strings.Contains(stored, "go 1.26") {
		t.Fatalf("ReadMemory injection source empty: %q", stored)
	}

	// Skill save then use round-trips the recipe.
	skill := tools.NewSkillTool()
	out, err = skill.Call(ctx, json.RawMessage(`{"action":"save","name":"loc-count","content":"1. run wc -l\n2. sum by dir"}`))
	if err != nil || !strings.Contains(out, "saved") {
		t.Fatalf("Skill save: %q %v", out, err)
	}
	out, err = skill.Call(ctx, json.RawMessage(`{"action":"use","name":"loc-count"}`))
	if err != nil || !strings.Contains(out, "wc -l") {
		t.Fatalf("Skill use: %q %v", out, err)
	}
	if idx := tools.SkillIndex(); idx != "loc-count" {
		t.Fatalf("SkillIndex: %q", idx)
	}

	// Path traversal in skill names is rejected.
	if _, err := skill.Call(ctx, json.RawMessage(`{"action":"save","name":"..%2fescape","content":"x"}`)); err == nil {
		if _, err := os.Stat(filepath.Join(home, "escape.md")); err == nil {
			t.Fatal("skill escaped its directory")
		}
	}
	if _, err := skill.Call(ctx, json.RawMessage(`{"action":"use","name":"a/b"}`)); err == nil {
		t.Fatal("slash in skill name should be rejected")
	}
}
