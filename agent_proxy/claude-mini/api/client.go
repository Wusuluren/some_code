package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is a minimal Anthropic Messages API client (non-streaming),
// mirroring pkg/api.Client from the main app.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config contains client configuration.
type Config struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// NewClient creates a new API client, falling back to env vars.
func NewClient(config Config) *Client {
	if config.APIKey == "" {
		config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if config.BaseURL == "" {
		config.BaseURL = os.Getenv("ANTHROPIC_BASE_URL")
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com/v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	return &Client{
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		httpClient: &http.Client{Timeout: config.Timeout},
	}
}

// MessageRequest represents a Messages API request.
type MessageRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system,omitempty"`
	Messages  []Message        `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
}

// Message represents a single conversation message.
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock is a piece of message content: text, tool_use or tool_result.
type ContentBlock struct {
	Type string `json:"type"`

	// text / thinking
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ToolDefinition defines a tool for the API.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// Usage contains token usage information.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// MessageResponse represents the Messages API response.
type MessageResponse struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// CreateMessage sends a message request to the Anthropic API.
func (c *Client) CreateMessage(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("no API key: set ANTHROPIC_API_KEY or pass --api-key")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("API error (%s): %s - %s", resp.Status, apiErr.Error.Type, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("API error (%s): %s", resp.Status, string(respBody))
	}

	var result MessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &result, nil
}
