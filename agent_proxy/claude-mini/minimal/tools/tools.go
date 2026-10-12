package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Tool is the minimal interface every agent tool must implement,
// mirroring internal/types.Tool from the main app.
type Tool interface {
	// Name returns the tool name exposed to the API.
	Name() string
	// Description returns the tool description exposed to the API.
	Description() string
	// InputSchema returns the JSON Schema for the tool input.
	InputSchema() map[string]interface{}
	// Call executes the tool with the given JSON input and returns text output.
	// A non-nil error is surfaced to the model as an errored tool_result.
	Call(ctx context.Context, input json.RawMessage) (string, error)
}

// Registry holds the available tools keyed by name.
type Registry struct {
	tools  []Tool
	byName map[string]Tool
}

// NewRegistry creates a registry with the five minimal tools registered:
// Bash, Read, Write, Edit, Grep.
func NewRegistry() *Registry {
	r := &Registry{}
	for _, t := range []Tool{NewBashTool(), NewReadTool(), NewWriteTool(), NewEditTool(), NewGrepTool()} {
		r.Register(t)
	}
	return r
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	if r.byName == nil {
		r.byName = map[string]Tool{}
	}
	if _, dup := r.byName[t.Name()]; !dup {
		r.tools = append(r.tools, t)
	}
	r.byName[t.Name()] = t
}

// Find looks up a tool by name.
func (r *Registry) Find(name string) Tool {
	return r.byName[name]
}

// List returns all tools sorted by name (stable request payload).
func (r *Registry) List() []Tool {
	out := make([]Tool, len(r.tools))
	copy(out, r.tools)
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// helper: build an object JSON schema.
func objectSchema(props map[string]interface{}, required []string) map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// helper: shorthand for a string property with description.
func strProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func mustErr(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
