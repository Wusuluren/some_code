package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CreateMessageStream sends a streaming request and reassembles the reply
// into the same neutral MessageResponse the blocking CreateMessage returns,
// while pushing text deltas to onText as they arrive (v9). Tool_use blocks
// are still returned whole at the end, so the engine's tool extraction is
// unchanged whether or not streaming is on.

func (c *Client) CreateMessageStream(ctx context.Context, req MessageRequest, onText func(string)) (*MessageResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("no API key: set ANTHROPIC_API_KEY or pass --api-key")
	}
	req.Stream = true

	var body []byte
	var err error
	url := c.baseURL + "/messages"
	if c.isOpenAI() {
		body, err = marshalOpenAIRequest(req)
		url = c.baseURL + "/chat/completions"
	} else {
		body, err = json.Marshal(req)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.isOpenAI() {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else {
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := readAll(resp)
		return nil, fmt.Errorf("API error (%s): %s", resp.Status, string(respBody))
	}

	if c.isOpenAI() {
		return readOpenAIStream(resp, onText)
	}
	return readAnthropicStream(resp, onText)
}

// readAll drains an error body (small).
func readAll(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

// scanSSE walks "data:" lines of an SSE stream, calling fn with each payload.
// It stops on "data: [DONE]" and tolerates comment/keep-alive lines.
func scanSSE(resp *http.Response, fn func(data string) error) error {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		if err := fn(data); err != nil {
			return err
		}
	}
	return sc.Err()
}

// sBlock accumulates one streamed content block by index.
type sBlock struct {
	typ   string
	id    string
	name  string
	text  strings.Builder
	input strings.Builder
}

func readAnthropicStream(resp *http.Response, onText func(string)) (*MessageResponse, error) {
	blocks := map[int]*sBlock{}
	order := []int{}
	result := &MessageResponse{}
	err := scanSSE(resp, func(data string) error {
		var ev struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return nil // skip lines we can't parse rather than abort the stream
		}
		switch ev.Type {
		case "message_start":
			result.Usage.InputTokens = ev.Message.Usage.InputTokens
		case "content_block_start":
			b := &sBlock{typ: ev.ContentBlock.Type, id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
			blocks[ev.Index] = b
			order = append(order, ev.Index)
		case "content_block_delta":
			b := blocks[ev.Index]
			if b == nil {
				return nil
			}
			switch ev.Delta.Type {
			case "text_delta":
				b.text.WriteString(ev.Delta.Text)
				if onText != nil {
					onText(ev.Delta.Text)
				}
			case "input_json_delta":
				b.input.WriteString(ev.Delta.PartialJSON)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				result.StopReason = ev.Delta.StopReason
			}
			result.Usage.OutputTokens = ev.Usage.OutputTokens
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, i := range order {
		b := blocks[i]
		switch b.typ {
		case "text":
			result.Content = append(result.Content, ContentBlock{Type: "text", Text: b.text.String()})
		case "tool_use":
			result.Content = append(result.Content, ContentBlock{Type: "tool_use", ID: b.id, Name: b.name, Input: json.RawMessage(b.input.String())})
		}
	}
	return result, nil
}

func readOpenAIStream(resp *http.Response, onText func(string)) (*MessageResponse, error) {
	result := &MessageResponse{}
	var text strings.Builder
	toolByID := map[string]*ContentBlock{}
	toolOrder := []string{}
	err := scanSSE(resp, func(data string) error {
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
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
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			return nil
		}
		ch := chunk.Choices[0]
		if ch.Delta.Content != "" {
			text.WriteString(ch.Delta.Content)
			if onText != nil {
				onText(ch.Delta.Content)
			}
		}
		for _, tc := range ch.Delta.ToolCalls {
			key := tc.ID
			if key == "" && len(toolOrder) > 0 {
				key = toolOrder[len(toolOrder)-1] // argument fragment for the last tool
			}
			blk, ok := toolByID[key]
			if !ok {
				blk = &ContentBlock{Type: "tool_use", ID: tc.ID, Name: tc.Function.Name}
				toolByID[key] = blk
				toolOrder = append(toolOrder, key)
			}
			if tc.Function.Name != "" && blk.Name == "" {
				blk.Name = tc.Function.Name
			}
			blk.Input = append(blk.Input, []byte(tc.Function.Arguments)...)
		}
		if ch.FinishReason != "" {
			switch ch.FinishReason {
			case "tool_calls":
				result.StopReason = "tool_use"
			case "stop":
				result.StopReason = "end_turn"
			default:
				result.StopReason = ch.FinishReason
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if text.Len() > 0 {
		result.Content = append(result.Content, ContentBlock{Type: "text", Text: text.String()})
	}
	for _, k := range toolOrder {
		blk := toolByID[k]
		if len(blk.Input) == 0 {
			blk.Input = json.RawMessage("{}")
		}
		result.Content = append(result.Content, *blk)
	}
	return result, nil
}
