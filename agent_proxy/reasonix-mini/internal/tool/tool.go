// Package tool defines the Tool interface and the process-global builtin set
// (SPEC §3.2). Built-in subpackages self-register via init(); a runtime
// *Registry is assembled per run from the enabled subset, and the agent only
// ever sees the *Registry.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Tool is one capability the model can call.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema for parameters
	Execute(ctx context.Context, args json.RawMessage) (string, error)
	ReadOnly() bool // readers are auto-allowed; writers hit the permission gate
}

// Registry is the per-run set of callable tools.
type Registry struct {
	mu     sync.RWMutex
	byName map[string]Tool
}

// NewRegistry builds a runtime registry from tools, rejecting name clashes.
func NewRegistry(tools []Tool) (*Registry, error) {
	r := &Registry{byName: map[string]Tool{}}
	for _, t := range tools {
		if err := r.Add(t); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Add inserts one tool, canonicalizing nothing but guarding duplicates.
func (r *Registry) Add(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.byName[t.Name()]; dup {
		return fmt.Errorf("tool: %q registered twice", t.Name())
	}
	r.byName[t.Name()] = t
	return nil
}

// Get returns the named tool or nil.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[name]
}

// Tools returns all registered tools sorted by name, so the tool schemas
// serialized into the system prefix keep a stable order across turns
// (cache-first: the prefix must stay byte-stable).
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.byName))
	for _, t := range r.byName {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

var (
	builtinMu sync.Mutex
	builtins  = map[string]Tool{}
)

// RegisterBuiltin adds a compile-time tool to the global builtin set. Called
// from init() in tool/builtin files.
func RegisterBuiltin(t Tool) {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	if _, dup := builtins[t.Name()]; dup {
		panic(fmt.Sprintf("tool: builtin %q registered twice", t.Name()))
	}
	builtins[t.Name()] = t
}

// Builtins lists the global builtin set sorted by name.
func Builtins() []Tool {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	out := make([]Tool, 0, len(builtins))
	for _, t := range builtins {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
