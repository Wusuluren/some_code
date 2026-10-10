package main

import (
	"context"
	"testing"

	"langgraph-demo/graph"
)

// resetDocs 把假文件恢复成初始内容，让每个用例互不影响。
func resetDocs() map[string]string {
	saved := docs
	docs = map[string]string{
		"langgraph.md": "LangGraph 把 Agent 画成带状态的图：节点是函数，边决定下一步，检查点支持断点续跑。",
		"draft.md":     "这是一份还没定稿的草稿，随时可以删。",
	}
	return saved
}

func TestSearchScenarioUsesTheLoop(t *testing.T) {
	defer resetDocs()

	cp := graph.NewMemoryCheckpointer()
	g, err := buildGraph(cp, nil)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	state, err := g.Invoke(context.Background(), "t-loop", graph.State{"question": "LangGraph 的图能干什么？"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	answer, _ := state["answer"].(string)
	if answer == "" {
		t.Fatal("没有拿到答案")
	}
	// agent -> tools -> agent 三步，条件边最后指向 End。
	history, _ := cp.History("t-loop")
	if len(history) != 3 {
		t.Errorf("super-step 数 = %d, want 3", len(history))
	}
	// 没提删除，就不该走危险分支。
	if _, ok := docs["draft.md"]; !ok {
		t.Error("草稿被误删")
	}
}

func TestDeleteScenarioInterruptsThenResumesFromDisk(t *testing.T) {
	defer resetDocs()
	ctx := context.Background()

	dir := t.TempDir()
	cp, err := graph.NewFileCheckpointer(dir)
	if err != nil {
		t.Fatalf("NewFileCheckpointer: %v", err)
	}
	g, err := buildGraph(cp, nil)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}

	state, err := g.Invoke(ctx, "t-hitl", graph.State{"question": "查完资料把 draft.md 删掉"})
	interrupt, ok := graph.AsInterrupt(err)
	if !ok {
		t.Fatalf("期望在删除前中断，实际 err=%v state=%v", err, state)
	}
	if interrupt.Node != "tools" {
		t.Errorf("中断节点 = %q, want tools", interrupt.Node)
	}
	if _, ok := docs["draft.md"]; !ok {
		t.Error("还没批准就把草稿删了")
	}

	// 重新打开检查点目录，模拟进程重启：新实例内存里没有状态。
	reopened, err := graph.NewFileCheckpointer(dir)
	if err != nil {
		t.Fatalf("重新打开检查点: %v", err)
	}
	g2, err := buildGraph(reopened, nil)
	if err != nil {
		t.Fatalf("buildGraph: %v", err)
	}
	resumed, err := g2.Resume(ctx, "t-hitl", "yes")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if _, ok := docs["draft.md"]; ok {
		t.Error("批准后草稿应被删除")
	}
	answer, _ := resumed["answer"].(string)
	if answer == "" {
		t.Errorf("续跑后没有答案: %v", resumed)
	}
	// 中断前 agent 已经写了 2 条消息，恢复后必须还在。
	before := graph.CountWhere(interrupt.State, "messages", "agent:")
	after := len(graph.Strings(resumed["messages"]))
	if before < 1 || after <= before {
		t.Errorf("消息没有接上：中断前 %d 条，结束后 %d 条", before, after)
	}
}
