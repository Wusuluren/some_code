// Package provider defines the Provider interface, the wire data types, and
// the kind→factory registry (SPEC §3.1 / §4). The core knows only interfaces;
// concrete vendors self-register from their own subpackages via init(), and
// parents never import children.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

// Role identifies the author of a Message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one chat message in OpenAI wire shape.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a complete tool invocation emitted by a model. Arguments is raw JSON.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSchema advertises one tool to the model in chat-completions function shape.
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Request is a single chat-completion call.
type Request struct {
	Messages    []Message
	Tools       []ToolSchema
	Temperature float64
	MaxTokens   int
}

// ChunkType discriminates streamed chunks.
type ChunkType int

const (
	ChunkText ChunkType = iota
	ChunkToolCall
	ChunkDone
	ChunkError
)

// Chunk is one streamed event from a provider. Only complete ToolCalls are
// emitted; delta accumulation lives inside the provider implementation.
type Chunk struct {
	Type     ChunkType
	Text     string
	ToolCall *ToolCall
	Err      error
}

// Provider is a vendor endpoint able to stream a chat completion.
type Provider interface {
	Name() string
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
}

// Config is the resolved provider instance passed from config to factories.
type Config struct {
	Name          string
	Kind          string
	BaseURL       string
	Model         string
	APIKey        string
	ContextWindow int
}

// Factory builds a Provider from a resolved config instance.
type Factory func(cfg Config) (Provider, error)

var (
	kinds     = map[string]Factory{}
	providers = map[string]Provider{}
)

// Register adds a factory under a kind (e.g. "openai"). Called from init().
func Register(kind string, f Factory) {
	if _, dup := kinds[kind]; dup {
		panic(fmt.Sprintf("provider: kind %q registered twice", kind))
	}
	kinds[kind] = f
}

// New instantiates the provider of the given kind and memoizes it by instance
// name, so the same config entry yields the same live connection.
func New(kind string, cfg Config) (Provider, error) {
	if p, ok := providers[cfg.Name]; ok {
		return p, nil
	}
	f, ok := kinds[kind]
	if !ok {
		return nil, fmt.Errorf("provider: kind %q is not registered (known: %v)", kind, registeredKinds())
	}
	p, err := f(cfg)
	if err != nil {
		return nil, err
	}
	providers[cfg.Name] = p
	return p, nil
}

func registeredKinds() []string {
	var out []string
	for k := range kinds {
		out = append(out, k)
	}
	return out
}
