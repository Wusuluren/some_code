// Package openai implements the OpenAI-compatible chat-completions provider
// and self-registers it under kind "openai" at init (SPEC §3.1). Any
// OpenAI-compatible vendor (DeepSeek included) is a config instance of this
// kind, differing only in base_url / model / api_key_env.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"reasonix-mini/internal/provider"
)

func init() {
	provider.Register("openai", newProvider)
}

type openaiProvider struct {
	cfg    provider.Config
	client *http.Client
}

func newProvider(cfg provider.Config) (provider.Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("openai: provider %q has empty base_url", cfg.Name)
	}
	return &openaiProvider{
		cfg:    cfg,
		client: &http.Client{}, // bounded retries are the parent repo's Phase; mini keeps it simple
	}, nil
}

func (p *openaiProvider) Name() string { return p.cfg.Name }

// ---- request wire shape ----

type chatRequest struct {
	Model       string             `json:"model"`
	Messages    []provider.Message `json:"messages"`
	Tools       []toolWrapper      `json:"tools,omitempty"`
	Temperature float64            `json:"temperature"`
	Stream      bool               `json:"stream"`
}

type toolWrapper struct {
	Type     string              `json:"type"`
	Function provider.ToolSchema `json:"function"`
}

// ---- response wire shape ----

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    *int   `json:"index"`
				ID       any    `json:"id"` // some vendors send null on continuation deltas
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Stream posts to base_url + "/chat/completions" and consumes the SSE
// response. Streaming tool-call deltas are accumulated by index here; only
// complete ToolCalls are emitted on the channel (SPEC §3.1). ctx cancellation
// aborts the in-flight request.
func (p *openaiProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	tools := make([]toolWrapper, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, toolWrapper{Type: "function", Function: t})
	}
	body, err := json.Marshal(chatRequest{
		Model:       p.cfg.Model,
		Messages:    req.Messages,
		Tools:       tools,
		Temperature: req.Temperature,
		Stream:      true,
	})
	if err != nil {
		return nil, fmt.Errorf("openai: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	out := make(chan provider.Chunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		if err := p.consumeStream(resp.Body, out); err != nil {
			select {
			case out <- provider.Chunk{Type: provider.ChunkError, Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

// frag accumulates one streaming tool call, keyed by the vendor's delta index.
type frag struct {
	id, name string
	args     strings.Builder
}

func (p *openaiProvider) consumeStream(r io.Reader, out chan<- provider.Chunk) error {
	// Accumulated tool-call fragments keyed by stream index.
	frags := map[int]*frag{}

	emit := func(c provider.Chunk) error {
		out <- c // the consumer always drains until Done/Error, so no ctx select is needed here
		return nil
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // vendor lines can be long
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue // ignore event/id/comment lines: the minimal SSE contract
		}
		payload := bytes.TrimSpace(line[len("data: "):])
		if len(payload) == 0 {
			continue
		}
		if bytes.Equal(payload, []byte("[DONE]")) {
			for _, i := range sortedIndexes(frags) {
				f := frags[i]
				if err := emit(provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
					ID: f.id, Name: f.name, Arguments: f.args.String(),
				}}); err != nil {
					return err
				}
			}
			return emit(provider.Chunk{Type: provider.ChunkDone})
		}
		var sc streamChunk
		if err := json.Unmarshal(payload, &sc); err != nil {
			return fmt.Errorf("openai: bad SSE json: %w", err)
		}
		for _, ch := range sc.Choices {
			if ch.Delta.Content != "" {
				if err := emit(provider.Chunk{Type: provider.ChunkText, Text: ch.Delta.Content}); err != nil {
					return err
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				f := frags[idx]
				if f == nil {
					f = &frag{id: stringify(tc.ID)}
					frags[idx] = f
				} else if f.id == "" {
					f.id = stringify(tc.ID)
				}
				if tc.Function.Name != "" {
					f.name += tc.Function.Name
				}
				f.args.WriteString(tc.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openai: stream read: %w", err)
	}
	return fmt.Errorf("openai: stream ended without [DONE]")
}

// stringify normalizes the tool-call id, which some vendors send as JSON null.
func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func sortedIndexes(m map[int]*frag) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
