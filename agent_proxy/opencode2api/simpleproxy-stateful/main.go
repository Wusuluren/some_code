// simpleproxy-stateful：带"服务端记忆"的会话代理（安全版路子 B）。
//
// 与 simpleproxy（无状态透传）对照，这个文件演示如何把"上下文可信权"
// 从客户端收回到服务端，对应上一轮讨论的四条防线：
//
//  1. 会话凭证由服务端签发（POST /v1/sessions，高熵随机），
//     客户端不能用自带的 session 头认领/污染别人的会话；
//  2. 客户端只能提交"最新一句话"（content 纯文本），
//     messages 数组由服务端根据账本拼出来，role 全部服务端盖章；
//  3. system 提示词由服务端环境变量决定，客户端传的 system/messages 字段被无视；
//  4. 历史只存在服务端账本（内存 map），伪造"从未发生的对话"在此不再可行。
//
// 运行（默认仍走 Zen 匿名免费档，协议细节同 simpleproxy）：
//
//	go run ./examples/simpleproxy-stateful
//
// 用法：
//
//	POST /v1/sessions                      → {"id":"conv_..."}  （申请会话）
//	POST /v1/sessions/{id}/chat {"model":..,"content":".."}  → {"reply":".."} （只发增量）
//	GET  /v1/sessions/{id}                 → 查看服务端账本
//	DELETE /v1/sessions/{id}               → 清空账本
package main

import (
	"bufio"
	"bytes"
	"context"
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
	"sync"
)

var (
	upstream     = envOr("UPSTREAM", "https://opencode.ai/zen")
	upKey        = envOr("UPSTREAM_KEY", "public") // "public" = 匿名免费档
	localKey     = os.Getenv("API_KEY")            // 设置后所有端点要求 Bearer 鉴权
	systemPrompt = os.Getenv("SYSTEM_PROMPT")      // 防线3：system 只能服务端配
	maxTurns     = 40                              // 账本超长截断，防无界增长
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// —— 防线 1+4：服务端账本 ——
// key = 服务端签发的 conv_ 高熵 ID；value = 真实发生过的对话账本。
// 客户端永远不能直接写这个结构，只能追加 content 文本，role 由服务端盖章。

type turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var (
	mu      sync.Mutex
	ledger  = map[string][]turn{}
	base62a = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// canonicalSessionID：与 simpleproxy 相同，把 conv ID 哈希成
// ses_+12hex+14base62 的 Zen canonical 形态，维持上游 prompt 缓存亲和。
func canonicalSessionID(signal string) string {
	sum := sha256.Sum256([]byte("ses\x00" + signal))
	n := new(big.Int).SetBytes(sum[6:16])
	base, rem := big.NewInt(62), new(big.Int)
	out := make([]byte, 14)
	for i := 13; i >= 0; i-- {
		n.DivMod(n, base, rem) // 商写回 n，余数进 rem，无需交换
		out[i] = base62a[rem.Int64()]
	}
	return "ses_" + hex.EncodeToString(sum[:6]) + string(out)
}

func authOK(r *http.Request) bool {
	return localKey == "" || strings.Contains(r.Header.Get("Authorization"), localKey)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// —— 端点 ——

func createSession(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeJSON(w, 401, map[string]any{"error": "invalid api key"})
		return
	}
	id := "conv_" + newToken() // 高熵随机：猜不到别人的会话
	mu.Lock()
	ledger[id] = nil
	mu.Unlock()
	writeJSON(w, 200, map[string]any{"id": id})
}

func getSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mu.Lock()
	hist, ok := ledger[id]
	mu.Unlock()
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "no such session"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "history": hist})
}

func deleteSession(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	delete(ledger, r.PathValue("id"))
	mu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true})
}

// chat 是防线 2 的落点：客户端 body 里我们只读 model 和 content 两个字段，
// 它传来的任何 messages / role / system 都会被丢弃。
func chat(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeJSON(w, 401, map[string]any{"error": "invalid api key"})
		return
	}
	id := r.PathValue("id")
	var in struct {
		Model   string `json:"model"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil || in.Content == "" || in.Model == "" {
		writeJSON(w, 400, map[string]any{"error": `need {"model":..., "content":...}`})
		return
	}

	// 从账本取真实历史 + 本次 user 增量
	mu.Lock()
	hist, ok := ledger[id]
	if !ok {
		mu.Unlock()
		writeJSON(w, 404, map[string]any{"error": "no such session"})
		return
	}
	turns := append(append([]turn{}, hist...), turn{Role: "user", Content: in.Content})
	mu.Unlock()

	reply, upstreamErr, err := askUpstream(r.Context(), id, in.Model, turns)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if upstreamErr != "" { // 上游报错：不写入账本，保持历史干净
		writeJSON(w, 502, map[string]any{"error": upstreamErr})
		return
	}

	// assistant 回复由服务端盖章入账；超长截断保留最近的对话
	mu.Lock()
	if len(turns) > maxTurns {
		turns = turns[len(turns)-maxTurns:]
	}
	ledger[id] = append(turns, turn{Role: "assistant", Content: reply})
	mu.Unlock()

	writeJSON(w, 200, map[string]any{"reply": reply})
}

// —— 上游调用（协议细节同 simpleproxy）——

var coreTools = []string{"bash", "edit", "glob", "grep", "read"}

func askUpstream(ctx context.Context, convID, model string, turns []turn) (reply, upstreamErr string, err error) {
	messages := make([]any, 0, len(turns)+1)
	if systemPrompt != "" { // 防线3：唯一的 system 来源
		messages = append(messages, map[string]any{"role": "system", "content": systemPrompt})
	}
	for _, t := range turns { // 防线2：role 全部来自账本
		messages = append(messages, map[string]any{"role": t.Role, "content": t.Content})
	}

	payload := map[string]any{
		"model":          model,
		"messages":       messages,
		"stream":         true, // 免费档强制流式
		"stream_options": map[string]any{"include_usage": true},
		"tools":          anonymousTools(),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		upstream+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	session := canonicalSessionID(convID) // 同一会话恒定 session → 命中上游 prompt 缓存
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "opencode/1.18.31 ("+runtime.GOOS+" "+runtime.GOARCH+")")
	req.Header.Set("x-opencode-client", "cli")
	req.Header.Set("x-opencode-session", session)
	req.Header.Set("x-session-affinity", session)
	req.Header.Set("X-Session-Id", session)
	req.Header.Set("Authorization", "Bearer "+upKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode/100 == 2 {
			return "", string(b), nil // 极少数非流错误但 2xx：原样回传
		}
		return "", string(b), nil // 错误 JSON：入账失败，由调用方拒写账本
	}
	content, errJSON := collapseToJSON(resp.Body)
	if errJSON != "" {
		return "", errJSON, nil
	}
	return content, "", nil
}

func anonymousTools() []any {
	tools := make([]any, 0, len(coreTools))
	for _, name := range coreTools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": "Agent tool " + name,
				"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
			},
		})
	}
	return tools
}

// collapseToJSON：教学简化版折叠，只取 delta.content（不含 tool_calls/reasoning）。
func collapseToJSON(src io.Reader) (content, errText string) {
	sc := bufio.NewScanner(src)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var b strings.Builder
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
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if chunk.Error != nil {
			return "", chunk.Error.Message
		}
		for _, ch := range chunk.Choices {
			b.WriteString(ch.Delta.Content)
		}
	}
	if b.Len() == 0 {
		return "", "empty stream"
	}
	return b.String(), ""
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/sessions", createSession)
	mux.HandleFunc("GET /v1/sessions/{id}", getSession)
	mux.HandleFunc("DELETE /v1/sessions/{id}", deleteSession)
	mux.HandleFunc("POST /v1/sessions/{id}/chat", chat)
	addr := envOr("LISTEN", "127.0.0.1:8082")
	log.Printf("simpleproxy-stateful listening on http://%s → %s (key=%s)", addr, upstream, upKey)
	log.Fatal(http.ListenAndServe(addr, mux))
}
