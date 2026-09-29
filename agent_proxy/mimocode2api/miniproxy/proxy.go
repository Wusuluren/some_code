// minimax/proxy 教学版：把 OpenAI /v1/chat/completions 请求转发给 MiMoCode(opencode) headless server。
//
// 原理一句话：MiMoCode 后端不是 OpenAI API，它是一套「会话式」REST + SSE：
//
//	POST   /session                     新建会话          -> {id}
//	POST   /session/{id}/message        提交 prompt parts （阻塞直到生成结束）
//	GET    /event                       全局 SSE 事件流    （增量内容从这里流出）
//	GET    /session/{id}/message        轮询兜底：拉完整消息
//	DELETE /session/{id}                用完即删
//	鉴权：Basic base64("mimocode:" + MIMOCODE_SERVER_PASSWORD)
//
// 所以代理要做的事 = 协议翻译：
//
//	OpenAI messages  ->  opencode parts（纯文本 + role 前缀）+ system
//	opencode SSE     ->  OpenAI chat.completion.chunk（content / reasoning_content）
//
// 只保留主干，删掉了原项目里的：工具调用桥接、图片多模态、模型名模糊匹配、
// 后端进程托管、请求队列（这里用一把互斥锁代替，因为后端同一时刻只跑一个会话）。
//
// 运行：BACKEND_URL=http://127.0.0.1:10001 API_KEY=sk-test go run proxy.go
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ───────────────────────── 配置 ─────────────────────────

type Config struct {
	Addr            string        // 代理监听地址
	BackendURL      string        // MiMoCode 后端地址
	BackendPassword string        // 后端 Basic auth 密码
	APIKey          string        // 对外 Bearer key，留空 = 不鉴权
	Timeout         time.Duration // 单次请求上限
}

func loadConfig() Config {
	d := func(k, v string) string {
		if x := os.Getenv(k); x != "" {
			return x
		}
		return v
	}
	timeout := 180 * time.Second
	if v := os.Getenv("TIMEOUT_MS"); v != "" {
		var ms int
		if _, err := fmt.Sscanf(v, "%d", &ms); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return Config{
		Addr:            d("ADDR", ":10000"),
		BackendURL:      strings.TrimSuffix(d("BACKEND_URL", "http://127.0.0.1:10001"), "/"),
		BackendPassword: os.Getenv("BACKEND_PASSWORD"),
		APIKey:          os.Getenv("API_KEY"),
		Timeout:         timeout,
	}
}

// ───────────────────────── OpenAI 侧数据结构 ─────────────────────────

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Content 可能是 string，也可能是 [{type:"text",...},{type:"image_url",...}]
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// opencode 的 part：教学版只发文本
type part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// 把 OpenAI 的 content 压成纯文本
func textOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &arr) == nil {
		var sb strings.Builder
		for _, p := range arr {
			if p.Type == "text" {
				sb.WriteString(p.Text)
			}
		}
		return sb.String()
	}
	return ""
}

// OpenAI 的多轮对话 -> opencode 的单段 prompt：把 role 写进文本里，让模型自己分辨
func buildPrompt(msgs []Message) (parts []part, system, promptText string) {
	var sys []string
	for _, m := range msgs {
		text := textOf(m.Content)
		if text == "" {
			continue
		}
		switch strings.ToLower(m.Role) {
		case "system":
			sys = append(sys, text)
		case "user", "assistant":
			parts = append(parts, part{Type: "text", Text: strings.ToUpper(m.Role) + ": " + text})
		}
	}
	return parts, strings.Join(sys, "\n\n"), joinText(parts)
}

func joinText(parts []part) string {
	var out []string
	for _, p := range parts {
		out = append(out, p.Text)
	}
	return strings.Join(out, "\n\n")
}

// ───────────────────────── opencode 事件流数据结构 ─────────────────────────

type partRef struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
}

type eventProps struct {
	SessionID string   `json:"sessionID"`
	PartID    string   `json:"partID"`
	Field     string   `json:"field"`
	Delta     any      `json:"delta"` // 目前都是 string，留 any 防止后端换形状
	Part      *partRef `json:"part"`
}

type event struct {
	Type       string     `json:"type"`
	Properties eventProps `json:"properties"`
}

const (
	evPartUpdated = "message.part.updated"
	evPartDelta   = "message.part.delta"
	evSessionIdle = "session.idle"
)

// ───────────────────────── 后端客户端 ─────────────────────────

type Backend struct {
	cfg Config
	hc  *http.Client
}

func (b *Backend) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.cfg.BackendURL+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.cfg.BackendPassword != "" {
		token := base64.StdEncoding.EncodeToString([]byte("mimocode:" + b.cfg.BackendPassword))
		req.Header.Set("Authorization", "Basic "+token)
	}
	return req, nil
}

// do：一次性请求，读完 body 就关连接
func (b *Backend) do(ctx context.Context, method, path string, body any, out any) error {
	req, err := b.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	res, err := b.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> HTTP %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// readSSE：把 SSE 长连接逐条解析后塞进 channel
func readSSE(ctx context.Context, res *http.Response, out chan<- event) {
	defer close(out)
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var data []string
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case line == "": // 空行表示一个事件结束
			if len(data) == 0 {
				continue
			}
			var ev event
			if err := json.Unmarshal([]byte(strings.Join(data, "")), &ev); err == nil {
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
			data = data[:0]
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
}

// ───────────────────────── 核心转发 ─────────────────────────

// subscribe 建立 /event SSE 长连接，返回事件 channel。
// 注意：连接必须在发 prompt 之前建立；另外有些后端不会立刻 flush 响应头，
// 所以这里加了 5s 建连超时，超时就算订阅失败，交给调用方回退轮询。
func (b *Backend) subscribe(ctx context.Context) (<-chan event, error) {
	req, err := b.newRequest(ctx, "GET", "/event", nil)
	if err != nil {
		return nil, err
	}
	type result struct {
		res *http.Response
		err error
	}
	connected := make(chan result, 1)
	go func() {
		res, err := b.hc.Do(req)
		connected <- result{res, err}
	}()

	select {
	case r := <-connected:
		if r.err != nil {
			return nil, r.err
		}
		if r.res.StatusCode >= 300 {
			r.res.Body.Close()
			return nil, fmt.Errorf("GET /event -> HTTP %d", r.res.StatusCode)
		}
		out := make(chan event, 256)
		go readSSE(ctx, r.res, out)
		return out, nil
	case <-time.After(5 * time.Second):
		// ctx 是 streamCtx，外层 defer cancel 会把这条挂着的连接关掉
		return nil, fmt.Errorf("connect /event timeout (响应头未返回)")
	}
}

// promptBody 是 POST /session/{id}/message 的请求体
type promptBody struct {
	Model struct {
		ProviderID string `json:"providerID"`
		ModelID    string `json:"modelID"`
	} `json:"model"`
	System string `json:"system,omitempty"`
	Parts  []part `json:"parts"`
}

// chat 把一次对话跑完：建会话 -> 订阅事件流 -> 发 prompt -> 增量喂给 onDelta。
// 返回累计的 content 和 reasoning；若事件流一个增量都没送到，用轮询兜底。
func (b *Backend) chat(ctx context.Context, req *ChatRequest, parts []part, system string, onDelta func(kind, text string)) (content, reasoning string, err error) {
	providerID, modelID := splitModel(req.Model)

	// 1) 新建会话
	var sess struct {
		ID string `json:"id"`
	}
	if err = b.do(ctx, "POST", "/session", map[string]any{}, &sess); err != nil {
		return "", "", fmt.Errorf("create session: %w", err)
	}
	defer func() { // 4) 用完即删，别把后端会话堆爆
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		b.do(cleanupCtx, "DELETE", "/session/"+sess.ID, nil, nil)
	}()

	// 2) 先订阅事件流，再发 prompt —— 顺序反了会丢掉开头的增量。
	//    订阅失败不致命：记下来，稍后走轮询兜底。
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	events, sseErr := b.subscribe(streamCtx)
	if sseErr != nil {
		log.Printf("[proxy] /event 订阅失败，回退轮询: %v", sseErr)
	}

	// 3) 发 prompt。这个调用在后端是阻塞的，放独立 goroutine 里跑；
	//    ctx 不能跟着事件流一起取消，所以用 WithoutCancel 派生。
	promptCtx, cancelPrompt := context.WithTimeout(context.WithoutCancel(ctx), b.cfg.Timeout)
	defer cancelPrompt()
	body := promptBody{System: system, Parts: parts}
	body.Model.ProviderID, body.Model.ModelID = providerID, modelID
	promptDone := make(chan error, 1)
	go func() { promptDone <- b.do(promptCtx, "POST", "/session/"+sess.ID+"/message", &body, nil) }()

	// partID -> part 类型。message.part.delta 不带类型，得靠先前 updated 事件建立的映射。
	partType := map[string]string{}
	deadline := time.After(b.cfg.Timeout)

	flush := func(kind string, text string) {
		if text == "" {
			return
		}
		switch kind {
		case "reasoning":
			reasoning += text
		default:
			content += text
		}
		onDelta(kind, text)
	}

	if sseErr == nil {
	consume:
		for {
			select {
			case <-ctx.Done():
				return content, reasoning, ctx.Err()
			case <-deadline:
				err = fmt.Errorf("timeout after %s", b.cfg.Timeout)
				break consume
			case ev, ok := <-events:
				if !ok {
					break consume // 后端断流
				}
				p := ev.Properties
				switch ev.Type {
				case evPartUpdated:
					if p.Part == nil || p.Part.SessionID != sess.ID {
						continue
					}
					if p.Part.ID != "" && p.Part.Type != "" {
						partType[p.Part.ID] = p.Part.Type
					}
					if d, _ := p.Delta.(string); d != "" {
						flush(partType[p.Part.ID], d)
					}
				case evPartDelta:
					if p.SessionID != sess.ID {
						continue
					}
					kind := partType[p.PartID]
					if kind == "" {
						kind = p.Field // 兜底：映射没建起来时信 field
					}
					if d, _ := p.Delta.(string); d != "" {
						flush(kind, d)
					}
				case evSessionIdle:
					if p.SessionID == sess.ID {
						break consume // 真正的结束信号
					}
				}
			}
		}
	}

	// 兜底：SSE 啥也没给（老后端/代理吞流），就轮询消息列表拿全文
	if content == "" && reasoning == "" {
		c, r, perr := b.pollFinal(ctx, sess.ID)
		if perr != nil {
			if err == nil {
				err = perr
			}
			return content, reasoning, err
		}
		flush("reasoning", r)
		flush("text", c)
		return content, reasoning, nil
	}

	select { // 等一下 prompt 请求，把后端错误带出来
	case perr := <-promptDone:
		if perr != nil && content == "" && reasoning == "" {
			err = perr
		}
	default:
	}
	return content, reasoning, err
}

// pollFinal：从消息尾巴往前找最后一条 assistant，读到内容非空且已结束为止
func (b *Backend) pollFinal(ctx context.Context, sessionID string) (content, reasoning string, err error) {
	type msgPart struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type msg struct {
		Info struct {
			Role   string          `json:"role"`
			Finish json.RawMessage `json:"finish"`
			Error  json.RawMessage `json:"error"`
		} `json:"info"`
		Parts []msgPart `json:"parts"`
	}
	// ctx 已带整体超时，轮询只要贴着它跑就行
	for ctx.Err() == nil {
		var msgs []msg
		if e := b.do(ctx, "GET", "/session/"+sessionID+"/message", nil, &msgs); e == nil {
			for i := len(msgs) - 1; i >= 0; i-- {
				m := msgs[i]
				if m.Info.Role != "assistant" {
					continue
				}
				var c, r []string
				for _, p := range m.Parts {
					switch p.Type {
					case "reasoning":
						r = append(r, p.Text)
					case "text":
						c = append(c, p.Text)
					}
				}
				finished := len(m.Info.Finish) > 0 && string(m.Info.Finish) != "null"
				if finished && (len(strings.Join(c, "")) > 0 || len(m.Info.Error) > 0) {
					return strings.Join(c, ""), strings.Join(r, ""), nil
				}
				break // 只看最后一条 assistant
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", "", fmt.Errorf("poll timeout after %s", b.cfg.Timeout)
}

func splitModel(m string) (providerID, modelID string) {
	if m == "" {
		return "mimo", "mimo-auto"
	}
	if i := strings.Index(m, "/"); i > 0 {
		return m[:i], m[i+1:]
	}
	return "mimo", m
}

// ───────────────────────── HTTP 层 ─────────────────────────

type server struct {
	cfg Config
	b   *Backend
	mu  sync.Mutex // 后端同一时刻只跑一个会话，请求排队即可
}

func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIKey != "" && r.Header.Get("Authorization") != "Bearer "+s.cfg.APIKey {
			writeJSON(w, 401, map[string]any{"error": map[string]string{"message": "Unauthorized"}})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// chunk 是一条 OpenAI SSE 帧
func chunk(id, model string, delta map[string]any, finish string) string {
	return fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":%q,\"choices\":[{\"index\":0,\"delta\":%s,\"finish_reason\":%s}]}\n\n",
		id, time.Now().Unix(), model, mustJSON(delta), nullIfEmpty(finish))
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func nullIfEmpty(s string) string {
	if s == "" {
		return "null"
	}
	return `"` + s + `"`
}

// tokens 估算：原项目也是 len/4 的粗估，OpenAI 客户端只用来展示
func tokens(s string) int { return (len(s) + 3) / 4 }

func (s *server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 50<<20)).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"message": "invalid json: " + err.Error()}})
		return
	}
	parts, system, promptText := buildPrompt(req.Messages)
	if len(parts) == 0 {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"message": "no text content in messages"}})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeout)
	defer cancel()

	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixMilli())
	modelStr := req.Model
	if modelStr == "" {
		modelStr = "mimo/mimo-auto"
	}

	if !req.Stream {
		content, reasoning, err := s.b.chat(ctx, &req, parts, system, func(string, string) {})
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": map[string]string{"message": err.Error()}})
			return
		}
		writeJSON(w, 200, map[string]any{
			"id":      id,
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   modelStr,
			"choices": []any{map[string]any{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": content, "reasoning_content": reasoning},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{
				"prompt_tokens":     tokens(promptText),
				"completion_tokens": tokens(content) + tokens(reasoning),
				"total_tokens":      tokens(promptText) + tokens(content) + tokens(reasoning),
			},
		})
		return
	}

	// 流式：Content-Type 必须是 text/event-stream，且每帧都要 Flush，否则客户端收不到增量
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": map[string]string{"message": "streaming unsupported"}})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	flusher.Flush()

	// 首帧给出 role，让客户端知道这是一条 assistant 消息
	fmt.Fprint(w, chunk(id, modelStr, map[string]any{"role": "assistant"}, ""))
	flusher.Flush()

	content, reasoning, err := s.b.chat(ctx, &req, parts, system, func(kind, text string) {
		delta := map[string]any{}
		if kind == "reasoning" {
			delta["reasoning_content"] = text // 思维链走扩展字段，不混进 content
		} else {
			delta["content"] = text
		}
		fmt.Fprint(w, chunk(id, modelStr, delta, ""))
		flusher.Flush()
	})
	if err != nil {
		fmt.Fprint(w, chunk(id, modelStr, map[string]any{"content": "[proxy error] " + err.Error()}, ""))
	}
	// 末帧带 finish_reason + usage（token 数只是 len/4 的粗估，后端不返回真实值）
	usage := map[string]any{
		"prompt_tokens":     tokens(promptText),
		"completion_tokens": tokens(content) + tokens(reasoning),
		"total_tokens":      tokens(promptText) + tokens(content) + tokens(reasoning),
	}
	fmt.Fprintf(w, "data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":%s}\n\n",
		id, time.Now().Unix(), modelStr, mustJSON(usage))
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// /v1/models：后端 /config/providers 的形状是 {providers:[{id, models:{...}}]}，展平成 OpenAI 列表
func (s *server) handleModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var res struct {
		Providers []struct {
			ID     string `json:"id"`
			Models map[string]struct {
				Name string `json:"name"`
			} `json:"models"`
		} `json:"providers"`
	}
	out := []any{}
	if err := s.b.do(ctx, "GET", "/config/providers", nil, &res); err != nil {
		log.Printf("models: %v", err)
		out = append(out, map[string]any{"id": "mimo/mimo-auto", "object": "model", "owned_by": "mimo"})
		writeJSON(w, 200, map[string]any{"object": "list", "data": out})
		return
	}
	for _, p := range res.Providers {
		for id, m := range p.Models {
			name := m.Name
			if name == "" {
				name = id
			}
			out = append(out, map[string]any{
				"id": p.ID + "/" + id, "name": name, "object": "model", "owned_by": p.ID,
			})
		}
	}
	writeJSON(w, 200, map[string]any{"object": "list", "data": out})
}

func main() {
	cfg := loadConfig()
	s := &server{
		cfg: cfg,
		b:   &Backend{cfg: cfg, hc: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/v1/models", s.auth(s.handleModels))
	mux.HandleFunc("/v1/chat/completions", s.auth(s.handleCompletions))

	log.Printf("[miniproxy] listening %s -> backend %s (auth=%v)", cfg.Addr, cfg.BackendURL, cfg.APIKey != "")
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		log.Fatal(err)
	}
}
