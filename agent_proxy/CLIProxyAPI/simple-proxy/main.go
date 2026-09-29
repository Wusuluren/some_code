// simple-proxy: a minimal, single-file protocol-translating LLM proxy.
//
// It demonstrates the core idea of CLIProxyAPI:
//
//	client (OpenAI Chat Completions format)
//	  -> translate request  -> upstream (Gemini generateContent format)
//	  <- translate response <- upstream SSE stream
//
// Only the stdlib is used so every line stays readable.
//
// Usage:
//
//	export GEMINI_API_KEY=...
//	go run .
//
// Then point any OpenAI-compatible client at http://127.0.0.1:8080/v1
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// geminiBase is the upstream base URL; override GEMINI_BASE to point at a mock server.
var geminiBase = envOr("GEMINI_BASE", "https://generativelanguage.googleapis.com/v1beta/models")

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------- OpenAI request model (only fields we care about) ----------

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`

	Temperature    *float64 `json:"temperature,omitempty"`
	MaxTokens      *int     `json:"max_tokens,omitempty"`
	TopP           *float64 `json:"top_p,omitempty"`
	Stop           any      `json:"stop,omitempty"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string, or [{"type":"text","text":...}] parts
}

// contentToString flattens string or text-part-array content.
func contentToString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if s, ok := m["text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		return b.String()
	}
	return ""
}

// ---------- Gemini request model ----------

type geminiRequest struct {
	Contents          []geminiContent       `json:"contents"`
	SystemInstruction *geminiSystem         `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig      `json:"generationConfig,omitempty"`
	SafetySettings    []geminiSafetySetting `json:"safetySettings,omitempty"`
}

type geminiSystem struct {
	Parts []geminiPart `json:"parts"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxOutputTokens  *int     `json:"maxOutputTokens,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
	ResponseMIMEType string   `json:"responseMimeType,omitempty"`
}

// translateRequest converts an OpenAI chat request body into a Gemini request body.
// This mirrors internal/translator/gemini/openai/chat-completions in the real project.
func translateRequest(req *openAIRequest) (*geminiRequest, string, error) {
	g := &geminiRequest{}
	for _, m := range req.Messages {
		text := contentToString(m.Content)
		if text == "" {
			continue
		}
		switch m.Role {
		case "system":
			// Gemini carries the system prompt separately.
			g.SystemInstruction = &geminiSystem{Parts: []geminiPart{{Text: text}}}
		case "assistant":
			g.Contents = append(g.Contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: text}}})
		default:
			g.Contents = append(g.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: text}}})
		}
	}
	if len(g.Contents) == 0 {
		return nil, "", fmt.Errorf("no usable messages")
	}

	cfg := &geminiGenConfig{}
	cfg.Temperature = req.Temperature
	cfg.TopP = req.TopP
	cfg.MaxOutputTokens = req.MaxTokens
	if stops, ok := req.Stop.([]any); ok {
		for _, s := range stops {
			if str, ok := s.(string); ok {
				cfg.StopSequences = append(cfg.StopSequences, str)
			}
		}
	} else if str, ok := req.Stop.(string); ok && str != "" {
		cfg.StopSequences = []string{str}
	}
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		cfg.ResponseMIMEType = "application/json"
	}
	if cfg.Temperature != nil || cfg.TopP != nil || cfg.MaxOutputTokens != nil || len(cfg.StopSequences) > 0 || cfg.ResponseMIMEType != "" {
		g.GenerationConfig = cfg
	}

	// OpenAI model names like "gemini-2.5-flash" map 1:1 to Gemini model ids here.
	model := strings.TrimPrefix(req.Model, "gemini-")
	model = "gemini-" + model
	return g, model, nil
}

// ---------- Gemini response model ----------

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Usage struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func finishReason(g string) string {
	switch g {
	case "MAX_TOKENS":
		return "length"
	default:
		return "stop"
	}
}

// ---------- handlers ----------

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		http.Error(w, `{"error":{"message":"GEMINI_API_KEY not set"}}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		http.Error(w, `{"error":{"message":"only POST /v1/chat/completions is supported"}}`, http.StatusNotFound)
		return
	}

	var req openAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"invalid JSON: `+err.Error()+`"}}`, http.StatusBadRequest)
		return
	}
	g, model, err := translateRequest(&req)
	if err != nil {
		http.Error(w, `{"error":{"message":"`+err.Error()+`"}}`, http.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(g)
	// The only difference between streaming and non-streaming upstream calls.
	path := fmt.Sprintf("%s/%s:%s", geminiBase, model, "generateContent")
	if req.Stream {
		path = fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse", geminiBase, model)
	}

	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		http.Error(w, `{"error":{"message":"`+err.Error()+`"}}`, http.StatusInternalServerError)
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("x-goog-api-key", apiKey) // same header the real GeminiExecutor sets

	upResp, err := http.DefaultClient.Do(upReq)
	if err != nil {
		http.Error(w, `{"error":{"message":"upstream request failed: `+err.Error()+`"}}`, http.StatusBadGateway)
		return
	}
	defer func() { _ = upResp.Body.Close() }()

	respBody, _ := io.ReadAll(upResp.Body)
	if upResp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(upResp.StatusCode)
		_, _ = w.Write(respBody) // pass upstream errors through as-is
		return
	}

	if req.Stream {
		relayStream(w, respBody, model)
		return
	}
	relayJSON(w, respBody, model)
}

// relayJSON translates a complete Gemini JSON response back to OpenAI format.
func relayJSON(w http.ResponseWriter, respBody []byte, model string) {
	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil || len(gr.Candidates) == 0 {
		http.Error(w, `{"error":{"message":"cannot parse upstream response"}}`, http.StatusBadGateway)
		return
	}
	var sb strings.Builder
	for _, p := range gr.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	out := map[string]any{
		"id":     "chatcmpl-simple-proxy",
		"object": "chat.completion",
		"model":  model,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": sb.String()},
			"finish_reason": finishReason(gr.Candidates[0].FinishReason),
		}},
		"usage": map[string]any{
			"prompt_tokens":     gr.Usage.PromptTokenCount,
			"completion_tokens": gr.Usage.CandidatesTokenCount,
			"total_tokens":      gr.Usage.TotalTokenCount,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// relayStream parses upstream Gemini SSE lines ("data: {...}") and re-emits them
// as OpenAI chat.completion.chunk SSE events. This is the SSE-relay core of the project.
func relayStream(w http.ResponseWriter, respBody []byte, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	// SSE requires the "data: " prefix and a blank line between events.
	write := func(v any) {
		var buf bytes.Buffer
		_ = json.NewEncoder(&buf).Encode(v)
		_, _ = w.Write(append([]byte("data: "), append(buf.Bytes(), '\n')...))
	}

	sc := bufio.NewScanner(bytes.NewReader(respBody))
	sc.Buffer(make([]byte, 64*1024), 10*1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue // ignore empty separators and comments
		}
		payload := bytes.TrimSpace(line[len("data: "):])
		if bytes.Equal(payload, []byte("[DONE]")) {
			break
		}

		var gr geminiResponse
		if err := json.Unmarshal(payload, &gr); err != nil || len(gr.Candidates) == 0 {
			continue
		}
		c := gr.Candidates[0]
		var sb strings.Builder
		for _, p := range c.Content.Parts {
			sb.WriteString(p.Text)
		}
		chunk := map[string]any{
			"id":     "chatcmpl-simple-proxy",
			"object": "chat.completion.chunk",
			"model":  model,
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": sb.String(),
				},
				"finish_reason": nil,
			}},
		}
		if c.FinishReason != "" {
			chunk["choices"].([]any)[0].(map[string]any)["finish_reason"] = finishReason(c.FinishReason)
		}
		write(chunk)
		if flusher != nil {
			flusher.Flush() // incremental delivery = streaming UX
		}
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func main() {
	http.HandleFunc("/v1/chat/completions", proxyHandler)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("simple-proxy listening on http://%s/v1 (OpenAI-compatible)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
