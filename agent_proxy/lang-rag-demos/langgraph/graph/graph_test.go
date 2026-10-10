package graph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// recorder 记录节点执行顺序，用来证明条件边与回环真的走了。
type recorder struct{ visited []string }

func (r *recorder) node(name string, delta State) Node {
	return func(_ context.Context, _ State) (State, error) {
		r.visited = append(r.visited, name)
		return delta, nil
	}
}

// TestConditionalEdgeAndLoop 走一条带回环的图：a -> b -> a -> End，共两轮。
func TestConditionalEdgeAndLoop(t *testing.T) {
	rec := &recorder{}
	g := New()
	g.AddReducer("messages", Append)
	g.AddNode("a", func(_ context.Context, s State) (State, error) {
		rec.visited = append(rec.visited, "a")
		round := CountWhere(s, "messages", "round")
		if round >= 2 {
			return State{"answer": "done"}, nil
		}
		return State{"messages": []any{"round"}}, nil
	})
	g.AddNode("b", rec.node("b", State{"messages": []any{"step-b"}}))
	g.SetEntryPoint("a")
	g.AddConditionalEdges("a", func(s State) string {
		if s["answer"] != nil {
			return "done"
		}
		return "more"
	}, map[string]string{"more": "b", "done": End})
	g.AddEdge("b", "a")

	compiled, err := g.Compile(WithMaxSteps(10))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	state, err := compiled.Invoke(context.Background(), "t1", State{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if state["answer"] != "done" {
		t.Errorf("answer = %v", state["answer"])
	}
	if got := strings.Join(rec.visited, ","); got != "a,b,a,b,a" {
		t.Errorf("节点顺序 = %s, want a,b,a,b,a", got)
	}
	// reducer 让 messages 累加而不是被覆盖。
	if n := len(Strings(state["messages"])); n != 4 {
		t.Errorf("messages 条数 = %d, want 4", n)
	}
}

// TestInterruptAndResume 验证中断把进度存盘、Resume 从断点继续。
func TestInterruptAndResume(t *testing.T) {
	cp := NewMemoryCheckpointer()
	g := New()
	g.AddNode("ask", func(_ context.Context, s State) (State, error) {
		if s[KeyHumanFeedback] != nil {
			return State{"answer": "已收到回复"}, nil
		}
		return nil, &Interrupt{Reason: "需要你确认"}
	})
	g.AddNode("finish", func(_ context.Context, _ State) (State, error) {
		return State{"reached": true}, nil
	})
	g.SetEntryPoint("ask")
	g.AddEdge("ask", "finish")
	g.AddEdge("finish", End)

	compiled, err := g.Compile(WithCheckpointer(cp))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	_, err = compiled.Invoke(context.Background(), "t2", State{"seen": "before"})
	interrupt, ok := AsInterrupt(err)
	if !ok {
		t.Fatalf("期望返回 Interrupt，实际 %v", err)
	}
	if interrupt.Node != "ask" || interrupt.Step != 0 {
		t.Errorf("中断位置 = %s/%d, want ask/0", interrupt.Node, interrupt.Step)
	}

	history, _ := cp.History("t2")
	if len(history) != 0 {
		t.Errorf("中断那一步不该写普通快照，得到 %d 条", len(history))
	}

	// 换一个图实例，模拟进程重启后只靠断点恢复。
	other, err := g.Compile(WithCheckpointer(cp))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	state, err := other.Resume(context.Background(), "t2", "yes")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if state["answer"] != "已收到回复" || state["reached"] != true {
		t.Errorf("续跑后的状态不完整: %v", state)
	}
	if state["seen"] != "before" {
		t.Errorf("断点前的状态丢失了: %v", state)
	}
	if _, err := other.Resume(context.Background(), "t2", "yes"); err == nil {
		t.Error("断点已被消费，第二次 Resume 应报错")
	}
}

// TestCompileRejectsBrokenGraph 校验图写错时 Compile 会拦住。
func TestCompileRejectsBrokenGraph(t *testing.T) {
	cases := []struct {
		name string
		g    *StateGraph
		want string
	}{
		{
			name: "没有入口",
			g: func() *StateGraph {
				g := New()
				g.AddNode("a", func(_ context.Context, _ State) (State, error) { return nil, nil })
				return g
			}(),
			want: "入口",
		},
		{
			name: "边指向不存在的节点",
			g: func() *StateGraph {
				g := New()
				noop := func(_ context.Context, _ State) (State, error) { return nil, nil }
				g.AddNode("a", noop)
				g.SetEntryPoint("a")
				g.AddEdge("a", "ghost")
				return g
			}(),
			want: "ghost",
		},
		{
			name: "条件边没有分支",
			g: func() *StateGraph {
				g := New()
				noop := func(_ context.Context, _ State) (State, error) { return nil, nil }
				g.AddNode("a", noop)
				g.SetEntryPoint("a")
				g.AddConditionalEdges("a", func(State) string { return "x" }, nil)
				return g
			}(),
			want: "分支",
		},
	}

	for _, tc := range cases {
		_, err := tc.g.Compile()
		if err == nil {
			t.Errorf("%s: 期望 Compile 报错", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: 错误 %q 未提到 %q", tc.name, err, tc.want)
		}
	}
}

// TestMaxStepsStopsRunawayLoop 防止条件边写错时无限打转。
func TestMaxStepsStopsRunawayLoop(t *testing.T) {
	g := New()
	g.AddNode("spin", func(_ context.Context, _ State) (State, error) { return nil, nil })
	g.SetEntryPoint("spin")
	g.AddEdge("spin", "spin")

	compiled, err := g.Compile(WithMaxSteps(3))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	_, err = compiled.Invoke(context.Background(), "t3", State{})
	if err == nil || !strings.Contains(err.Error(), "最大步数") {
		t.Fatalf("期望触发步数上限，实际 %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Error("错误类型不应是取消")
	}
}

// TestDeleteRemovesKeyFromState 验证增量里的 Delete 哨兵能清掉键。
func TestDeleteRemovesKeyFromState(t *testing.T) {
	g := New()
	g.AddNode("unset", func(_ context.Context, _ State) (State, error) {
		return State{"tmp": Delete}, nil
	})
	g.AddNode("check", func(_ context.Context, s State) (State, error) {
		if _, ok := s["tmp"]; ok {
			return nil, errors.New("tmp 没被删掉")
		}
		return State{"ok": true}, nil
	})
	g.SetEntryPoint("unset")
	g.AddEdge("unset", "check")
	g.AddEdge("check", End)

	compiled, err := g.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, err := compiled.Invoke(context.Background(), "t4", State{"tmp": "value"}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
}
