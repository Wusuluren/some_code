package mcp

import (
	"context"
	"encoding/json"
	"sort"
)

// MCPTool wraps one server tool behind the agent Tool interface, namespaced
// as mcp__<server>__<tool> so it can never collide with a built-in — the
// same convention Claude Code, Cursor and jingwei use.
type MCPTool struct {
	server string
	tool   string
	desc   string
	schema map[string]interface{}
	c      *Client
}

// Server is the configured server name this tool came from.
func (t *MCPTool) Server() string { return t.server }

// Name is the namespaced tool name exposed to the model.
func (t *MCPTool) Name() string { return "mcp__" + t.server + "__" + t.tool }

// Description is the server-provided tool description.
func (t *MCPTool) Description() string { return t.desc }

// InputSchema is the server-provided JSON Schema (default empty object).
func (t *MCPTool) InputSchema() map[string]interface{} {
	if t.schema == nil {
		return map[string]interface{}{"type": "object"}
	}
	return t.schema
}

// Call forwards to the server's tools/call.
func (t *MCPTool) Call(ctx context.Context, input json.RawMessage) (string, error) {
	return t.c.CallTool(t.tool, input)
}

// Manager owns the connected servers and the tools they contributed.
// Connection is best-effort: a server that fails to start or handshake is
// skipped so it never costs the session, exactly like jingwei's MCP.
type Manager struct {
	clients []*Client
	tools   []*MCPTool
}

// Connect starts every configured stdio server and gathers its tools.
func Connect(ctx context.Context, cfgs map[string]ServerConfig) *Manager {
	names := make([]string, 0, len(cfgs))
	for n := range cfgs {
		names = append(names, n)
	}
	sort.Strings(names)

	m := &Manager{}
	for _, n := range names {
		c, err := Start(ctx, cfgs[n])
		if err != nil {
			continue
		}
		if err := c.Initialize(); err != nil {
			_ = c.Close()
			continue
		}
		defs, err := c.ListTools()
		if err != nil {
			_ = c.Close()
			continue
		}
		m.clients = append(m.clients, c)
		for _, d := range defs {
			m.tools = append(m.tools, &MCPTool{server: n, tool: d.Name, desc: d.Description, schema: d.InputSchema, c: c})
		}
	}
	return m
}

// Tools returns every namespaced tool across connected servers.
func (m *Manager) Tools() []*MCPTool { return m.tools }

// Close shuts down all server processes.
func (m *Manager) Close() {
	for _, c := range m.clients {
		_ = c.Close()
	}
}
