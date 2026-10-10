package main

import (
	"context"
	"strings"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

// scriptedModel 是一个离线、可复现的 llms.Model 实现。
// 它根据 prompt 里的任务标记返回固定文本，并把每次收到的 prompt 记录下来，
// 这样测试能断言链路确实把上一跳的输出喂给了下一跳。
// 换成真实模型（anthropic / openai）时，业务代码不用改。
type scriptedModel struct {
	mu      sync.Mutex
	prompts []string
}

func (m *scriptedModel) record(prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prompts = append(m.prompts, prompt)
}

// SeenPrompts 返回模型实际收到的 prompt 列表，按调用顺序排列。
func (m *scriptedModel) SeenPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.prompts...)
}

func (m *scriptedModel) GenerateContent(
	ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption,
) (*llms.ContentResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var sb strings.Builder
	for _, mc := range messages {
		for _, part := range mc.Parts {
			if text, ok := part.(llms.TextContent); ok {
				sb.WriteString(text.Text)
				sb.WriteString("\n")
			}
		}
	}

	prompt := sb.String()
	m.record(prompt)
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: replyFor(prompt)}},
	}, nil
}

func (m *scriptedModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	resp, err := m.GenerateContent(ctx, []llms.MessageContent{
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: prompt}}},
	}, options...)
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Content, nil
}

const (
	summarizedText = "LangChain 用统一接口把模型、模板和组件串成固定管道。"
	keywordsText   = "统一接口, 模板, 管道"
	englishText    = "LangChain wires models, templates and components into a fixed pipeline."
)

// replyFor 按标记分派回复，保证整个 demo 不联网也能跑通。
func replyFor(prompt string) string {
	switch {
	case strings.Contains(prompt, "[task=summarize]"):
		return summarizedText
	case strings.Contains(prompt, "[task=keywords]"):
		return keywordsText
	case strings.Contains(prompt, "[task=translate]"):
		return englishText
	case strings.Contains(prompt, "Current conversation:"):
		return replyForChat(prompt)
	default:
		return "（脚本模型没匹配到任务标记）"
	}
}

// replyForChat 从会话模板里取出 history 段，证明 memory 已经注入 prompt。
func replyForChat(prompt string) string {
	history := sectionOf(prompt, "Current conversation:\n", "\nHuman:")
	if strings.TrimSpace(history) == "" {
		return "这是第一轮，history 是空的。"
	}
	return "我从 history 里读到：" + firstLine(history)
}

// sectionOf 取两个标记之间的文本；标记缺失时返回空串。
func sectionOf(text, start, end string) string {
	i := strings.Index(text, start)
	if i < 0 {
		return ""
	}
	rest := text[i+len(start):]
	if j := strings.Index(rest, end); j >= 0 {
		return rest[:j]
	}
	return rest
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}
