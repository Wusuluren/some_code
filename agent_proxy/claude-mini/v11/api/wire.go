package api

import (
	"encoding/json"
)

// This file adds the second wire protocol (v8): OpenAI-style Chat
// Completions. The engine keeps talking in the neutral Anthropic-shaped
// ContentBlock model; the client translates neutral <-> wire JSON here,
// so a Messages endpoint and a Chat-Completions endpoint both work
// behind one Client (jingwei's "providers live behind a port").

func (c *Client) isOpenAI() bool { return c.protocol == "openai" }

// oaMessage is a Chat Completions message.
type oaMessage struct {
	Role       string       `json:"role"`
	Content    *string      `json:"content,omitempty"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaRequest struct {
	Model      string      `json:"model"`
	MaxTokens  int         `json:"max_tokens"`
	Messages   []oaMessage `json:"messages"`
	Tools      []oaToolDef `json:"tools,omitempty"`
	ToolChoice string      `json:"tool_choice,omitempty"`
	Stream     bool        `json:"stream,omitempty"`
}

type oaToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description,omitempty"`
		Parameters  map[string]interface{} `json:"parameters,omitempty"`
	} `json:"function"`
}

func strPtr(s string) *string { return &s }

// marshalOpenAIRequest converts the neutral request to a Chat Completions body.
func marshalOpenAIRequest(req MessageRequest) ([]byte, error) {
	var msgs []oaMessage
	if req.System != "" {
		msgs = append(msgs, oaMessage{Role: "system", Content: strPtr(req.System)})
	}
	for _, m := range req.Messages {
		var text []string
		var toolResults []ContentBlock
		var toolUses []ContentBlock
		for _, b := range m.Content {
			switch b.Type {
			case "tool_result":
				toolResults = append(toolResults, b)
			case "tool_use":
				toolUses = append(toolUses, b)
			case "text":
				if b.Text != "" {
					text = append(text, b.Text)
				}
			}
		}
		joined := joinText(text)
		if m.Role == "assistant" {
			om := oaMessage{Role: "assistant"}
			if joined != "" {
				om.Content = strPtr(joined)
			}
			for _, tu := range toolUses {
				var tc oaToolCall
				tc.ID = tu.ID
				tc.Type = "function"
				tc.Function.Name = tu.Name
				tc.Function.Arguments = string(tu.Input)
				if tc.Function.Arguments == "" {
					tc.Function.Arguments = "{}"
				}
				om.ToolCalls = append(om.ToolCalls, tc)
			}
			msgs = append(msgs, om)
			continue
		}
		// user turn: any plain text first, then each tool result as its own
		// role:"tool" message (the Chat Completions way to feed results back).
		if joined != "" {
			msgs = append(msgs, oaMessage{Role: "user", Content: strPtr(joined)})
		}
		for _, tr := range toolResults {
			content := tr.Content
			if tr.IsError {
				content = "error: " + content
			}
			msgs = append(msgs, oaMessage{Role: "tool", ToolCallID: tr.ToolUseID, Content: strPtr(content)})
		}
	}

	var tools []oaToolDef
	for _, td := range req.Tools {
		var t oaToolDef
		t.Type = "function"
		t.Function.Name = td.Name
		t.Function.Description = td.Description
		t.Function.Parameters = td.InputSchema
		tools = append(tools, t)
	}

	or := oaRequest{Model: req.Model, MaxTokens: req.MaxTokens, Messages: msgs, Tools: tools, Stream: req.Stream}
	if len(tools) > 0 {
		or.ToolChoice = "auto"
	}
	return json.Marshal(or)
}

type oaResponse struct {
	Choices []struct {
		Message struct {
			Content   *string      `json:"content"`
			ToolCalls []oaToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// parseOpenAIResponse turns a Chat Completions body back into the neutral model.
func parseOpenAIResponse(body []byte) (*MessageResponse, error) {
	var or oaResponse
	if err := json.Unmarshal(body, &or); err != nil {
		return nil, err
	}
	result := &MessageResponse{}
	if len(or.Choices) == 0 {
		return result, nil
	}
	ch := or.Choices[0]
	if ch.Message.Content != nil && *ch.Message.Content != "" {
		result.Content = append(result.Content, ContentBlock{Type: "text", Text: *ch.Message.Content})
	}
	for _, tc := range ch.Message.ToolCalls {
		result.Content = append(result.Content, ContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	switch ch.FinishReason {
	case "tool_calls":
		result.StopReason = "tool_use"
	case "stop":
		result.StopReason = "end_turn"
	default:
		result.StopReason = ch.FinishReason
	}
	result.Usage = Usage{InputTokens: or.Usage.PromptTokens, OutputTokens: or.Usage.CompletionTokens}
	return result, nil
}

func joinText(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n"
		}
		out += p
	}
	return out
}
