package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/memory"
)

// TestPipelineFeedsPreviousOutputIntoNextPrompt 验证 LangChain 的核心承诺：
// 上一跳的输出被框架自动填进下一跳的模板，业务代码不用自己搬运。
func TestPipelineFeedsPreviousOutputIntoNextPrompt(t *testing.T) {
	model := &scriptedModel{}
	pipeline, err := buildPipeline(model)
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}

	values, err := chains.Call(context.Background(), pipeline, map[string]any{"text": sourceText})
	if err != nil {
		t.Fatalf("chains.Call: %v", err)
	}

	if got := values["english"]; got != englishText {
		t.Errorf("english = %v, want %q", got, englishText)
	}

	prompts := model.SeenPrompts()
	if len(prompts) != 3 {
		t.Fatalf("模型调用次数 = %d, want 3", len(prompts))
	}
	// 每一跳的 prompt 里必须带上一跳的输出，否则管道就断了。
	if !strings.Contains(prompts[1], summarizedText) {
		t.Errorf("第 2 跳 prompt 未包含第 1 跳输出:\n%s", prompts[1])
	}
	if !strings.Contains(prompts[2], keywordsText) {
		t.Errorf("第 3 跳 prompt 未包含第 2 跳输出:\n%s", prompts[2])
	}
}

// TestConversationMemoryInjectsHistory 验证 Memory：第二轮的 prompt 带上第一轮问答。
func TestConversationMemoryInjectsHistory(t *testing.T) {
	model := &scriptedModel{}
	conv := chains.NewConversation(model, memory.NewConversationWindowBuffer(2))
	ctx := context.Background()

	first := "LangChain 的核心抽象是什么？"
	if _, err := chains.Call(ctx, conv, map[string]any{"input": first}); err != nil {
		t.Fatalf("第一轮: %v", err)
	}
	values, err := chains.Call(ctx, conv, map[string]any{"input": "我上一个问题问的是什么？"})
	if err != nil {
		t.Fatalf("第二轮: %v", err)
	}

	seen := model.SeenPrompts()
	if len(seen) != 2 {
		t.Fatalf("模型调用次数 = %d, want 2", len(seen))
	}
	answer, _ := values["text"].(string)
	if !strings.Contains(seen[1], first) {
		t.Errorf("第二轮 prompt 未包含历史问题:\n%s", seen[1])
	}
	if !strings.Contains(answer, first) {
		t.Errorf("第二轮回复未体现读到 history: %q", answer)
	}
}

func TestRunWritesOutput(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	model := &scriptedModel{}

	if err := runPipeline(ctx, model, &buf); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	if err := runConversation(ctx, model, &buf); err != nil {
		t.Fatalf("runConversation: %v", err)
	}
	for _, want := range []string{
		"SequentialChain",
		englishText,
		"第2跳带上了第1跳输出=true",
		"第3跳带上了第2跳输出=true",
		"我从 history 里读到：",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("输出缺少 %q:\n%s", want, buf.String())
		}
	}
}
