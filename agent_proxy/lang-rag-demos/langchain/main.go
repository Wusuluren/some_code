// LangChain mini demo（Go 版 LangChain：github.com/tmc/langchaingo）
//
// 演示 LangChain 的三件事：
//  1. PromptTemplate —— 把变量填进模板
//  2. Chain 组合     —— SequentialChain 把上一跳输出喂给下一跳
//  3. Memory         —— ConversationChain 把历史注入 prompt
//
// 控制流是写死的管道：A -> B -> C，没有回环，也没有分支。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/prompts"
)

const sourceText = `LangChain 把模型调用、Prompt 模板、向量库和工具都收敛到同一个 Runnable 接口上，
组件之间用竖线串联，前一个的输出就是后一个的输入。它让原型开发很快，
但链路一旦变长，调试就要穿过好几层抽象。`

func main() {
	flag.Parse()
	ctx := context.Background()

	model, err := pickModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化模型失败:", err)
		os.Exit(1)
	}

	out := os.Stdout
	if err := runPipeline(ctx, model, out); err != nil {
		fmt.Fprintln(os.Stderr, "链路执行失败:", err)
		os.Exit(1)
	}
	if err := runConversation(ctx, model, out); err != nil {
		fmt.Fprintln(os.Stderr, "会话执行失败:", err)
		os.Exit(1)
	}
}

// pickModel 默认用离线脚本模型；带 -real 且设置了 ANTHROPIC_API_KEY 时走真实 API。
func pickModel() (llms.Model, error) {
	if !*realModel {
		return &scriptedModel{}, nil
	}
	token := os.Getenv("ANTHROPIC_API_KEY")
	if token == "" {
		return nil, fmt.Errorf("-real 需要 ANTHROPIC_API_KEY")
	}
	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return anthropic.New(anthropic.WithToken(token), anthropic.WithModel(model))
}

var realModel = flag.Bool("real", false, "调用真实 Anthropic API，而不是离线脚本模型")

// newChain 构造一跳：输入变量由模板里的 {{.x}} 声明，输出键用 OutputKey 指定。
func newChain(model llms.Model, template, outputKey string) *chains.LLMChain {
	c := chains.NewLLMChain(model, prompts.NewPromptTemplate(template, inputVars(template)))
	c.OutputKey = outputKey
	return c
}

// inputVars 按出现顺序取出模板里所有 {{.x}} 变量名，去重。
func inputVars(template string) []string {
	var names []string
	seen := map[string]bool{}
	for rest := template; ; {
		i := strings.Index(rest, "{{.")
		if i < 0 {
			break
		}
		rest = rest[i+3:]
		j := strings.Index(rest, "}}")
		if j < 0 {
			break
		}
		name := rest[:j]
		rest = rest[j:]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// buildPipeline 用 SequentialChain 串起三跳：摘要 -> 关键词 -> 英文。
// 注意 Go 版的 SequentialChain 只把「上一跳的输出」传给下一跳，
// 所以每一跳只能声明一个输入变量；要多输入得自己拼装 map。
func buildPipeline(model llms.Model) (chains.Chain, error) {
	summarize := newChain(model,
		"任务：把下面的文本压缩成一句话，只输出结果。\n[task=summarize]\n文本：{{.text}}",
		"summary")
	keywords := newChain(model,
		"任务：从下面的摘要里提取 3 个关键词，用逗号分隔。\n[task=keywords]\n摘要：{{.summary}}",
		"keywords")
	translate := newChain(model,
		"任务：把下面的关键词翻成英文。\n[task=translate]\n关键词：{{.keywords}}",
		"english")

	return chains.NewSequentialChain(
		[]chains.Chain{summarize, keywords, translate},
		[]string{"text"},
		[]string{"english"},
	)
}

func runPipeline(ctx context.Context, model llms.Model, out io.Writer) error {
	pipeline, err := buildPipeline(model)
	if err != nil {
		return fmt.Errorf("组装链路: %w", err)
	}

	values, err := chains.Call(ctx, pipeline, map[string]any{"text": sourceText})
	if err != nil {
		return fmt.Errorf("执行链路: %w", err)
	}

	fmt.Fprintln(out, "== 1. SequentialChain：固定管道，方向由代码写死 ==")
	fmt.Fprintf(out, "  [text]    输入 %d 字\n", len([]rune(sourceText)))
	fmt.Fprintf(out, "  [english] %v\n", values["english"])
	if m, ok := model.(*scriptedModel); ok {
		seen := m.SeenPrompts()
		if len(seen) < 3 {
			return fmt.Errorf("链路应调用模型 3 次，实际 %d 次", len(seen))
		}
		fmt.Fprintf(out, "  透传证据：第2跳带上了第1跳输出=%v，第3跳带上了第2跳输出=%v\n",
			strings.Contains(seen[1], summarizedText), strings.Contains(seen[2], keywordsText))
	}
	return nil
}

// runConversation 展示 Memory：第二轮的 prompt 里带上了第一轮问答。
func runConversation(ctx context.Context, model llms.Model, out io.Writer) error {
	conv := chains.NewConversation(model, memory.NewConversationWindowBuffer(2))

	fmt.Fprintln(out, "\n== 2. Memory：history 由框架注入 prompt ==")
	for _, question := range []string{"LangChain 的核心抽象是什么？", "我上一个问题问的是什么？"} {
		values, err := chains.Call(ctx, conv, map[string]any{"input": question})
		if err != nil {
			return fmt.Errorf("执行会话: %w", err)
		}
		fmt.Fprintf(out, "  Human: %s\n", question)
		fmt.Fprintf(out, "  AI:    %v\n", values["text"])
	}

	if m, ok := model.(*scriptedModel); ok {
		fmt.Fprintf(out, "  （模型共被调用 %d 次，其中会话 %d 次）\n",
			len(m.SeenPrompts()), 2)
	}
	return nil
}
