package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Client is a minimal MCP JSON-RPC 2.0 client over a newline-delimited
// stdio pipe: it sends one request per line and reads responses until the
// one matching the request id lands (notifications and other ids are
// skipped). Start wires it to a real server process; newPipeClient lets a
// test drive it over an in-memory pipe.
type Client struct {
	mu     sync.Mutex
	w      io.Writer
	sc     *bufio.Scanner
	id     int
	closer io.Closer
	cmd    *exec.Cmd
}

func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return sc
}

// Start launches the server process and connects over its stdin/stdout.
func Start(ctx context.Context, cfg ServerConfig) (*Client, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Client{w: stdin, sc: newScanner(stdout), closer: stdin, cmd: cmd}, nil
}

func newPipeClient(in io.Reader, out io.Writer) *Client {
	return &Client{w: out, sc: newScanner(in)}
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

func (c *Client) write(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.w.Write(append(b, '\n'))
	return err
}

// call sends a request and waits for the matching response id.
func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id++
	id := c.id
	if err := c.write(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	for c.sc.Scan() {
		line := strings.TrimSpace(c.sc.Text())
		if line == "" {
			continue
		}
		var rr rpcResponse
		if json.Unmarshal([]byte(line), &rr) != nil {
			continue
		}
		if rr.ID == nil || *rr.ID != id {
			continue // a server notification or a different response
		}
		if rr.Error != nil {
			return nil, fmt.Errorf("mcp error %d: %s", rr.Error.Code, rr.Error.Message)
		}
		return rr.Result, nil
	}
	if err := c.sc.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("mcp: stream ended before response")
}

func (c *Client) notify(method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.write(map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params})
}

// Initialize performs the MCP handshake.
func (c *Client) Initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "v10-agent", "version": "0.1.0"},
	}
	if _, err := c.call("initialize", params); err != nil {
		return err
	}
	return c.notify("notifications/initialized", map[string]interface{}{})
}

// ToolDef is one tool advertised by a server's tools/list.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ListTools returns the tools this server exposes.
func (c *Client) ListTools() ([]ToolDef, error) {
	res, err := c.call("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []ToolDef `json:"tools"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

// CallTool runs a tool and flattens its text content into one string.
func (c *Client) CallTool(name string, args json.RawMessage) (string, error) {
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	res, err := c.call("tools/call", map[string]interface{}{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, part := range out.Content {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	text := b.String()
	if out.IsError {
		return "", fmt.Errorf("mcp tool error: %s", text)
	}
	return text, nil
}

// Close shuts the server process down.
func (c *Client) Close() error {
	if c.closer != nil {
		_ = c.closer.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Wait()
	}
	return nil
}
