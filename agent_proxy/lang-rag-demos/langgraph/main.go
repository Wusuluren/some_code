// LangGraph mini demo（Go 零依赖实现，LangGraph 官方没有 Go 版本）
//
// 图结构：
//
//	agent --条件边--> tools --> agent   （回环）
//	   └──条件边--> End
//
// 演示 LangGraph 相对链式管道的四个能力：
//  1. 回环：工具结果回到 agent，由模型决定继续调工具还是收尾
//  2. 条件边：下一步走哪里由状态决定
//  3. reducer：messages 累加而不是覆盖
//  4. Checkpointer + Interrupt：危险动作停下来等人工批准，换一个新图实例从磁盘续跑
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"langgraph-demo/graph"
)

// 内置的假文件系统，让 demo 不碰真实磁盘也能演出「工具调用」。
var docs = map[string]string{
	"langgraph.md": "LangGraph 把 Agent 画成带状态的图：节点是函数，边决定下一步，检查点支持断点续跑。",
	"draft.md":     "这是一份还没定稿的草稿，随时可以删。",
}

func main() {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "langgraph-demo")
	if err != nil {
		exit(err)
	}
	defer os.RemoveAll(dir)

	cp, err := graph.NewFileCheckpointer(dir)
	if err != nil {
		exit(err)
	}

	// 场景 1：正常跑完，图里有回环。
	fmt.Println("== 场景 1：回环 + 条件边 ==")
	runOnce(ctx, cp, "thread-search", "LangGraph 的图能干什么？")

	// 场景 2：撞到危险工具就中断，用另一个图实例从磁盘续跑。
	fmt.Println("\n== 场景 2：人工批准 + 断点续跑 ==")
	runWithInterrupt(ctx, cp, "thread-delete", "查完资料把 draft.md 删掉")
}

// buildGraph 每次返回全新的执行实例：内存里没有残留状态，
// 能续跑只靠 Checkpointer，这正是断点续跑的意义。
func buildGraph(cp graph.Checkpointer, trace func(graph.StepInfo)) (*graph.Graph, error) {
	g := graph.New()
	g.AddReducer("messages", graph.Append)

	g.AddNode("agent", agentNode)
	g.AddNode("tools", toolsNode)
	g.SetEntryPoint("agent")

	g.AddConditionalEdges("agent", agentDecides, map[string]string{
		"tool": "tools",
		"done": graph.End,
	})
	g.AddEdge("tools", "agent") // 回环：工具结果交回模型

	return g.Compile(
		graph.WithCheckpointer(cp),
		graph.WithMaxSteps(12),
		graph.WithObserver(trace),
	)
}

// agentNode 扮演模型：看已有消息决定调哪个工具，或者给出最终答案。
func agentNode(_ context.Context, s graph.State) (graph.State, error) {
	question, _ := s["question"].(string)
	toolCalls := graph.CountWhere(s, "messages", "agent: 调用工具")

	if s["answer"] != nil {
		return nil, nil
	}

	switch {
	case toolCalls == 0:
		return graph.State{
			"tool_call": "search",
			"messages":  []any{"agent: 调用工具 search，先查资料"},
		}, nil
	case toolCalls == 1 && needsDelete(question):
		return graph.State{
			"tool_call": "delete_draft",
			"messages":  []any{"agent: 调用工具 delete_draft"},
		}, nil
	default:
		result, _ := s["tool_result"].(string)
		answer := fmt.Sprintf("根据查到的资料：%s", result)
		return graph.State{
			"answer":   answer,
			"messages": []any{"agent: " + answer},
		}, nil
	}
}

// agentDecides 是条件边：有工具要调就去 tools，拿到答案就走 End。
func agentDecides(s graph.State) string {
	if s["answer"] != nil {
		return "done"
	}
	if s["tool_call"] != nil {
		return "tool"
	}
	return "done"
}

// toolsNode 执行工具。删除动作没人批准时抛 Interrupt，让图停下来。
func toolsNode(_ context.Context, s graph.State) (graph.State, error) {
	tool, _ := s["tool_call"].(string)

	switch tool {
	case "search":
		question, _ := s["question"].(string)
		content := docs["langgraph.md"]
		if strings.Contains(question, "草稿") {
			content = docs["draft.md"]
		}
		return graph.State{
			"tool_result": content,
			"messages":    []any{"tool: search 返回 " + fmt.Sprint(len([]rune(content))) + " 字"},
		}, nil

	case "delete_draft":
		if !approved(s) {
			// 返回中断：运行时存下断点，把状态交还调用方等人工输入。
			return nil, &graph.Interrupt{
				Reason: "delete_draft 要删除 draft.md，需要人批准",
				Ask:    tool,
			}
		}
		deleted := false
		if _, ok := docs["draft.md"]; ok {
			delete(docs, "draft.md")
			deleted = true
		}
		result := "draft.md 不存在"
		if deleted {
			result = "已删除 draft.md"
		}
		return graph.State{
			"tool_result": result,
			"messages":    []any{"tool: delete_draft -> " + result, "human: 批准删除"},
		}, nil

	default:
		return graph.State{
			"tool_result": "未知工具 " + tool,
			"messages":    []any{"tool: 未知工具 " + tool},
		}, nil
	}
}

// approved 读人工回复。Resume 会把回复写进 human_feedback。
func approved(s graph.State) bool {
	switch feedback := s["human_feedback"].(type) {
	case bool:
		return feedback
	case string:
		switch strings.ToLower(feedback) {
		case "yes", "y", "true", "approve", "批准":
			return true
		}
	}
	return false
}

func needsDelete(question string) bool {
	return strings.Contains(question, "删") || strings.Contains(question, "清理")
}

func runOnce(ctx context.Context, cp *graph.FileCheckpointer, thread, question string) {
	g, err := buildGraph(cp, printStep)
	if err != nil {
		exit(err)
	}

	state, err := g.Invoke(ctx, thread, graph.State{"question": question})
	if err != nil {
		exit(err)
	}

	history, _ := cp.History(thread)
	fmt.Printf("  答案：%v\n", state["answer"])
	fmt.Printf("  super-step 数=%d，消息条数=%d，快照已写入 %s\n",
		len(history), len(graph.Strings(state["messages"])), cp.Path(thread))
}

func runWithInterrupt(ctx context.Context, cp *graph.FileCheckpointer, thread, question string) {
	g, err := buildGraph(cp, printStep)
	if err != nil {
		exit(err)
	}

	state, err := g.Invoke(ctx, thread, graph.State{"question": question})
	interrupt, ok := graph.AsInterrupt(err)
	if !ok {
		if err != nil {
			exit(err)
		}
		fmt.Printf("  没有触发中断，答案：%v\n", state["answer"])
		return
	}

	fmt.Printf("  在第 %d 步停下：%s\n", interrupt.Step+1, interrupt.Reason)
	fmt.Printf("  停在节点 %q，断点已落盘：%s\n", interrupt.Node, cp.Path(thread))
	fmt.Printf("  停下时状态里的消息条数=%d\n", len(graph.Strings(state["messages"])))

	// 换一个全新的图实例：进程内没留任何状态，只靠磁盘上的断点恢复。
	resumed, err := buildGraph(cp, printStep)
	if err != nil {
		exit(err)
	}
	state, err = resumed.Resume(ctx, thread, "yes")
	if err != nil {
		exit(err)
	}

	fmt.Printf("  批准后续跑完成：%v\n", state["answer"])
	fmt.Printf("  最终消息：%s\n", strings.Join(graph.Strings(state["messages"]), " | "))
	if _, stillThere := docs["draft.md"]; stillThere {
		fmt.Println("  注意：draft.md 还在，说明恢复后没走到删除分支")
	} else {
		fmt.Println("  draft.md 已被删除，恢复的是中断那一刻的进度")
	}
}

// printStep 打印每个 super-step 走了哪个节点、下一步去哪。
func printStep(step graph.StepInfo) {
	fmt.Printf("  step %d: %-5s -> %-5s\n", step.Step, step.Node, step.Next)
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "运行失败:", err)
	os.Exit(1)
}
