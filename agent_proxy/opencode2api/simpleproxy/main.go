// simpleproxy：opencode2api 的教学版最小实现（单文件、仅标准库、无 UI）。
//
// 它只做一件事：把本地 OpenAI Chat Completions 请求转发到 OpenCode Zen 上游，
// 再把上游响应（JSON 或 SSE 流）回传给客户端。
//
// 与完整项目的对应关系：
//   - gateway.handleInference  → handleChatCompletions
//   - gateway.newUpstreamRequest → 请求头构造（session 亲和 + Bearer key）
//   - protocol.ForwardStream   → relaySSE（同协议透传，因为 Zen chat 原生就是 OpenAI 格式）
//   - protocol.CollapseStream  → collapseSSE（匿名免费档强制 stream，需要折叠回 JSON）
//
// 运行：
//   UPSTREAM_KEY=你的key go run ./examples/simpleproxy   # 有 key：原样透传
//   go run ./examples/simpleproxy                          # 无 key：用 "public" 匿名免费档
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"runtime"
	"strings"
)

var (
	upstream = envOr("UPSTREAM", "https://opencode.ai/zen") // 对应 config.go 的 Zen base URL
	upKey    = envOr("UPSTREAM_KEY", "public")              // "public" = 匿名免费档（gateway.go 的 anonymousZenKey）
	// 本地 API key；设置后客户端必须带 Authorization: Bearer <localKey>
	localKey = os.Getenv("API_KEY")
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// —— session id：Zen 免费档校验 ses_+12位hex时间戳+14位base62，格式不对会 403 ——
// 对应 internal/identity/request.go 的 CanonicalSessionID。

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// canonicalSessionID 对应 CanonicalSessionID：对"会话种子"做 sha256，
// 前 6 字节当 12 位 hex 时间段、后面派生 14 位 base62 随机段。
// 同一个对话（首条 user 消息相同）→ 同一个 session → 上游 prompt 缓存可命中。
func canonicalSessionID(signal string) string {
	if signal == "" {
		b := make([]byte, 26)
		_, _ = rand.Read(b)
		return "ses_" + hex.EncodeToString(b[:6]) + base62From(b[6:])
	}
	sum := sha256.Sum256([]byte("ses\x00" + signal))
	return "ses_" + hex.EncodeToString(sum[:6]) + base62From(sum[6:16])
}

func conversationSeed(r *http.Request, payload map[string]any) string {
	for _, h := range []string{"x-opencode-session", "x-session-affinity", "X-Session-Id"} {
		if v := r.Header.Get(h); v != "" {
			return v
		}
	}
	if msgs, ok := payload["messages"].([]any); ok {
		for _, m := range msgs {
			if msg, ok := m.(map[string]any); ok && msg["role"] == "user" {
				b, _ := json.Marshal(msg["content"])
				return string(b)
			}
		}
	}
	return ""
}

func base62From(b []byte) string {
	n := new(big.Int).SetBytes(b)
	base := big.NewInt(62)
	rem := new(big.Int)
	out := make([]byte, 14)
	for i := 13; i >= 0; i-- {
		n.DivMod(n, base, rem) // n /= 62，rem = n % 62
		out[i] = base62Alphabet[rem.Int64()]
	}
	return string(out)
}

var coreTools = []string{"bash", "edit", "glob", "grep", "read"}

// ensureCoreTools 补齐缺失的核心工具声明，让上游把请求识别为 agent 会话。
func ensureCoreTools(payload map[string]any) {
	present := map[string]bool{}
	tools, _ := payload["tools"].([]any)
	for _, t := range tools {
		if fn, ok := t.(map[string]any)["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				present[name] = true
			}
		}
	}
	for _, name := range coreTools {
		if !present[name] {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        name,
					"description": "Agent tool " + name,
					"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
				},
			})
		}
	}
	payload["tools"] = tools
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if localKey != "" && !strings.Contains(r.Header.Get("Authorization"), localKey) {
		http.Error(w, `{"error":{"message":"invalid api key"}}`, http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, `{"error":{"message":"invalid json"}}`, http.StatusBadRequest)
		return
	}
	clientWantsStream, _ := payload["stream"].(bool)

	// 匿名免费档只接受"看起来像 agent 会话"的请求（否则 403 FreeTierError）：
	// 1) 强制 stream=true；2) 注入核心工具 bash/edit/glob/grep/read。
	// 对应 upstream.go 的 prepareAnonymousBody。stream:false 的客户端
	// 我们会把 SSE 再折叠回 JSON。
	if upKey == "public" {
		payload["stream"] = true
		// 让上游在最后一个 chunk 里带上 usage（ensureAnonymousChatUsage）
		payload["stream_options"] = map[string]any{"include_usage": true}
		ensureCoreTools(payload) // ensureAnonymousTools
		body, _ = json.Marshal(payload)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		upstream+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "build request failed", http.StatusInternalServerError)
		return
	}
	// 请求头集合与 upstream.go:785 newUpstreamRequest 一一对应
	// 会话种子（对应 identity.DeriveRequestIDs）：优先客户端自带的 session 头，
	// 否则用首条 user 消息——同一对话多轮请求 → 同一 session → 命中 prompt 缓存。
	session := canonicalSessionID(conversationSeed(r, payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("x-opencode-client", "cli")
	req.Header.Set("x-opencode-session", session)
	req.Header.Set("x-session-affinity", session) // 同一 session 落在同一上游，命中 prompt 缓存
	req.Header.Set("X-Session-Id", session)
	req.Header.Set("x-opencode-request", newID("req_"))
	req.Header.Set("x-opencode-project", newID("prj_"))
	// 免费档会校验客户端身份（httpx.UserAgent：opencode/x.y.z ...）
	req.Header.Set("User-Agent", "opencode/1.18.31 ("+runtime.GOOS+" "+runtime.GOARCH+")")
	req.Header.Set("Authorization", "Bearer "+upKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":{"message":"upstream request failed: `+err.Error()+`"}}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		io.Copy(w, resp.Body) // 非流式：字节级透传（同协议无需翻译）
		return
	}
	if clientWantsStream {
		relaySSE(w, resp.Body) // 客户端要流：透传 + 逐块 Flush
		return
	}
	collapseSSE(w, resp.Body) // 客户端要 JSON：把 SSE 折叠回 chat.completion
}

// relaySSE 对应 protocol.ForwardStream：SSE 行原样转发，空行（事件边界）时 flush，
// 保证客户端能逐 token 收到。
func relaySSE(w http.ResponseWriter, src io.Reader) {
	flusher, _ := w.(http.Flusher)
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if _, err := w.Write(append(line, '\n')); err != nil {
			return
		}
		if len(line) == 0 && flusher != nil {
			flusher.Flush()
		}
	}
}

// collapseSSE 对应 protocol.CollapseStream：逐条解析 data: {...} chunk，
// 累加 delta.content / tool_calls / usage，最后合成一条非流式响应。
func collapseSSE(w http.ResponseWriter, src io.Reader) {
	var (
		content, reasoning strings.Builder
		toolCalls          []map[string]any
		usage              map[string]any
		finish             any
		id, model          string
		created            int64
		sc                 = bufio.NewScanner(src)
	)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Created int64  `json:"created"`
			Usage   *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		id, model, created = chunk.ID, chunk.Model, chunk.Created
		if chunk.Usage != nil {
			usage = map[string]any{
				"prompt_tokens": chunk.Usage.PromptTokens, "completion_tokens": chunk.Usage.CompletionTokens,
				"total_tokens": chunk.Usage.TotalTokens,
			}
		}
		for _, ch := range chunk.Choices {
			content.WriteString(ch.Delta.Content)
			reasoning.WriteString(ch.Delta.ReasoningContent)
			if ch.FinishReason != nil {
				finish = *ch.FinishReason
			}
			for _, tc := range ch.Delta.ToolCalls {
				if tc.ID != "" { // 新 tool call：占位并记名字
					toolCalls = append(toolCalls, map[string]any{
						"id": tc.ID, "type": "function",
						"function": map[string]any{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
					})
				} else if len(toolCalls) > 0 { // 后续分片：把 arguments 拼到最后一条
					fn := toolCalls[len(toolCalls)-1]["function"].(map[string]any)
					fn["arguments"] = fn["arguments"].(string) + tc.Function.Arguments
				}
			}
		}
	}

	msg := map[string]any{"role": "assistant", "content": content.String()}
	if reasoning.Len() > 0 {
		msg["reasoning_content"] = reasoning.String()
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	out := map[string]any{
		"id": id, "object": "chat.completion", "created": created, "model": model,
		"choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": orStr(finish, "stop")}},
	}
	if usage != nil {
		out["usage"] = usage
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func orStr(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", handleChatCompletions)
	addr := envOr("LISTEN", "127.0.0.1:8081")
	log.Printf("simpleproxy listening on http://%s → %s (key=%s)", addr, upstream, upKey)
	log.Fatal(http.ListenAndServe(addr, mux))
}
