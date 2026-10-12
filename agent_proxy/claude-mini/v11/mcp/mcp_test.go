package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// serveMCP is a fake in-memory MCP server used to test the client framing
// without spawning a real process. It answers initialize, tools/list and
// tools/call over newline-delimited JSON-RPC, keyed by the request id.
func serveMCP(in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var req struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(sc.Bytes(), &req) != nil || req.ID == nil {
			continue // notification or unparseable line: no reply
		}
		var result json.RawMessage
		switch req.Method {
		case "initialize":
			result = json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"fake"}}`)
		case "tools/list":
			result = json.RawMessage(`{"tools":[{"name":"echo","description":"echo back","inputSchema":{"type":"object","properties":{"msg":{"type":"string"}}}}]}`)
		case "tools/call":
			result = json.RawMessage(`{"content":[{"type":"text","text":"pong"}]}`)
		default:
			b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": *req.ID, "error": map[string]interface{}{"code": -32601, "message": "no such method"}})
			out.Write(append(b, '\n'))
			continue
		}
		b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": *req.ID, "result": result})
		out.Write(append(b, '\n'))
	}
}

func TestMCPClientOverPipe(t *testing.T) {
	reqR, reqW := io.Pipe() // client -> server
	resR, resW := io.Pipe() // server -> client
	go serveMCP(reqR, resW)

	c := newPipeClient(resR, reqW)
	if err := c.Initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defs, err := c.ListTools()
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", defs)
	}

	// Wrap it as an agent tool and check the namespacing + the call.
	mt := &MCPTool{server: "fake", tool: defs[0].Name, desc: defs[0].Description, schema: defs[0].InputSchema, c: c}
	if mt.Name() != "mcp__fake__echo" {
		t.Fatalf("wrong namespaced name: %s", mt.Name())
	}
	out, err := mt.Call(context.Background(), json.RawMessage(`{"msg":"hi"}`))
	if err != nil || out != "pong" {
		t.Fatalf("tool call: out=%q err=%v", out, err)
	}
}

func TestLoadConfigExpand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")
	content := `{"mcpServers":{"files":{"command":"${MY_BIN:-node}","args":["-y","${PKG}"],"env":{"TOKEN":"secret-${TOK}"}},"remote":{"url":"https://x/y"}}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PKG", "@modelcontextprotocol/server-filesystem")
	t.Setenv("TOK", "abc")
	// MY_BIN unset -> default "node" is used.

	cfgs, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("url-only server should be skipped, got %d: %+v", len(cfgs), cfgs)
	}
	files := cfgs["files"]
	if files.Command != "node" {
		t.Fatalf("command default expand: %q", files.Command)
	}
	if len(files.Args) != 2 || files.Args[1] != "@modelcontextprotocol/server-filesystem" {
		t.Fatalf("args expand: %+v", files.Args)
	}
	if files.Env["TOKEN"] != "secret-abc" {
		t.Fatalf("env expand: %+v", files.Env)
	}
}

func TestConnectSkipsBadServer(t *testing.T) {
	// A server whose command does not exist must be skipped, not fatal.
	cfgs := map[string]ServerConfig{"broken": {Command: "definitely-not-a-real-binary-xyz-123"}}
	mgr := Connect(context.Background(), cfgs)
	defer mgr.Close()
	if len(mgr.Tools()) != 0 {
		t.Fatalf("bad server should contribute no tools, got %d", len(mgr.Tools()))
	}
}
