// Package graph 是一个零依赖的最小状态图运行时，用来还原 LangGraph 的核心机制：
// 节点、固定边、条件边、回环、reducer、Checkpointer 和 human-in-the-loop 中断。
//
// 与链式管道的区别：图允许环，下一步走哪里由状态决定，每一步都能存盘续跑。
package graph

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// End 是终止哨兵，等价于 LangGraph 的 END。
const End = "__end__"

// KeyHumanFeedback 是 Resume 写入状态的人工回复键。
const KeyHumanFeedback = "human_feedback"

// State 是节点共享的数据袋。值只放字符串、布尔和切片，便于整体快照与还原。
type State map[string]any

// Node 读当前状态，返回要合并进状态的增量。返回 nil 表示状态不变。
type Node func(ctx context.Context, s State) (State, error)

// Reducer 决定同一个键的旧值与增量如何合并，等价于 LangGraph 的 state reducer。
type Reducer func(old, delta any) any

// Condition 根据当前状态返回分支名，配合 routes 把分支名映射到下一节点。
type Condition func(s State) string

// deleteMarker 出现在增量里表示把该键从状态中删掉。
type deleteMarker struct{}

// Delete 用作增量值，把某个键从状态里移除。
var Delete = deleteMarker{}

// StateGraph 是图的构造器，Compile 之前不做完整性校验。
type StateGraph struct {
	nodes    map[string]Node
	edges    map[string]string
	conds    map[string]Condition
	routes   map[string]map[string]string
	reducers map[string]Reducer
	entry    string
}

// New 返回一个空图。
func New() *StateGraph {
	return &StateGraph{
		nodes:    map[string]Node{},
		edges:    map[string]string{},
		conds:    map[string]Condition{},
		routes:   map[string]map[string]string{},
		reducers: map[string]Reducer{},
	}
}

// AddNode 注册一个节点。同名节点后者覆盖前者。
func (g *StateGraph) AddNode(name string, n Node) {
	g.nodes[name] = n
}

// AddEdge 注册固定边：from 执行完无条件走 to。to 可以是 End。
func (g *StateGraph) AddEdge(from, to string) {
	g.edges[from] = to
	delete(g.conds, from)
}

// AddConditionalEdges 注册条件边：decide 返回分支名，routes 决定实际去向。
func (g *StateGraph) AddConditionalEdges(from string, decide Condition, routes map[string]string) {
	g.conds[from] = decide
	g.routes[from] = routes
	delete(g.edges, from)
}

// SetEntryPoint 设置起始节点。
func (g *StateGraph) SetEntryPoint(name string) { g.entry = name }

// AddReducer 为某个键设置合并规则，没有规则时增量直接覆盖旧值。
func (g *StateGraph) AddReducer(key string, r Reducer) { g.reducers[key] = r }

// StepInfo 描述一次 super-step 的结果，供轨迹打印使用。
type StepInfo struct {
	Step  int
	Node  string
	Next  string
	State State
}

// Graph 是编译后可执行的图。
type Graph struct {
	nodes    map[string]Node
	edges    map[string]string
	conds    map[string]Condition
	routes   map[string]map[string]string
	reducers map[string]Reducer
	entry    string

	maxSteps int
	cp       Checkpointer
	observer func(StepInfo)
}

// Option 配置编译后的图。
type Option func(*Graph)

// WithCheckpointer 打开快照与断点续跑能力。
func WithCheckpointer(cp Checkpointer) Option {
	return func(c *Graph) { c.cp = cp }
}

// WithMaxSteps 设置回环上限，避免条件写错时死循环。
func WithMaxSteps(n int) Option {
	return func(c *Graph) { c.maxSteps = n }
}

// WithObserver 注入每步回调，用于打印轨迹。
func WithObserver(f func(StepInfo)) Option {
	return func(c *Graph) { c.observer = f }
}

// Compile 校验图的完整性：入口存在、每条边的目标都已注册。
func (g *StateGraph) Compile(opts ...Option) (*Graph, error) {
	if len(g.nodes) == 0 {
		return nil, errors.New("graph: 没有任何节点")
	}
	if g.entry == "" {
		return nil, errors.New("graph: 未设置入口节点")
	}
	if _, ok := g.nodes[g.entry]; !ok {
		return nil, fmt.Errorf("graph: 入口节点 %q 未注册", g.entry)
	}

	for from, to := range g.edges {
		if _, ok := g.nodes[from]; !ok {
			return nil, fmt.Errorf("graph: 边 %q 的起点未注册", from)
		}
		if err := g.checkTarget(to); err != nil {
			return nil, err
		}
	}
	for from, decide := range g.conds {
		if _, ok := g.nodes[from]; !ok {
			return nil, fmt.Errorf("graph: 条件边 %q 的起点未注册", from)
		}
		if decide == nil {
			return nil, fmt.Errorf("graph: 条件边 %q 缺少判定函数", from)
		}
		if len(g.routes[from]) == 0 {
			return nil, fmt.Errorf("graph: 条件边 %q 没有可选分支", from)
		}
		for branch, to := range g.routes[from] {
			if err := g.checkTarget(to); err != nil {
				return nil, fmt.Errorf("graph: 节点 %q 的分支 %q: %w", from, branch, err)
			}
		}
	}

	c := &Graph{
		nodes: g.nodes, edges: g.edges, conds: g.conds,
		routes: g.routes, reducers: g.reducers, entry: g.entry,
		maxSteps: 25,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.maxSteps <= 0 {
		return nil, errors.New("graph: maxSteps 必须大于 0")
	}
	return c, nil
}

func (g *StateGraph) checkTarget(to string) error {
	if to == End {
		return nil
	}
	if _, ok := g.nodes[to]; !ok {
		return fmt.Errorf("目标节点 %q 未注册", to)
	}
	return nil
}

// Invoke 从入口开始跑到结束，或跑到中断点。
func (c *Graph) Invoke(ctx context.Context, thread string, in State) (State, error) {
	return c.run(ctx, thread, clone(in), c.entry, 0)
}

// Resume 从检查点续跑。feedback 写进状态的 human_feedback 键，
// 被中断的节点会从头重跑一次，因此它必须能容忍重复执行。
func (c *Graph) Resume(ctx context.Context, thread string, feedback any) (State, error) {
	if c.cp == nil {
		return nil, errors.New("graph: Resume 需要 Checkpointer")
	}
	pending, ok, err := c.cp.LoadPending(thread)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("graph: 会话 %q 没有待恢复的断点", thread)
	}
	s := clone(pending.State)
	s[KeyHumanFeedback] = feedback
	return c.run(ctx, thread, s, pending.Node, pending.Step)
}

// run 按 super-step 推进：执行节点 -> 合并增量 -> 找出边 -> 存快照。
func (c *Graph) run(ctx context.Context, thread string, s State, next string, step int) (State, error) {
	for {
		if next == End {
			return s, nil
		}
		if step >= c.maxSteps {
			return s, fmt.Errorf("graph: 超过最大步数 %d，检查回环是否收敛", c.maxSteps)
		}
		if err := ctx.Err(); err != nil {
			return s, err
		}

		node, ok := c.nodes[next]
		if !ok {
			return s, fmt.Errorf("graph: 节点 %q 未注册", next)
		}

		delta, err := node(ctx, s)
		var interrupt *Interrupt
		if errors.As(err, &interrupt) {
			interrupt.Node = next
			interrupt.Step = step
			interrupt.State = clone(s)
			if c.cp != nil {
				if serr := c.cp.SavePending(Checkpoint{Thread: thread, Node: next, Step: step, State: s}); serr != nil {
					return s, serr
				}
			}
			return s, err
		}
		if err != nil {
			return s, fmt.Errorf("graph: 节点 %q 失败: %w", next, err)
		}

		c.merge(s, delta)
		step++

		target, err := c.route(next, s)
		if err != nil {
			return s, err
		}
		if c.cp != nil {
			if serr := c.cp.Save(Checkpoint{Thread: thread, Node: next, Step: step, State: s}); serr != nil {
				return s, serr
			}
		}
		if c.observer != nil {
			c.observer(StepInfo{Step: step, Node: next, Next: target, State: s})
		}
		next = target
	}
}

// route 解析下一步：条件边优先，其次固定边。
func (c *Graph) route(from string, s State) (string, error) {
	if decide, ok := c.conds[from]; ok {
		branch := decide(s)
		to, ok := c.routes[from][branch]
		if !ok {
			return "", fmt.Errorf("graph: 节点 %q 返回了未注册的分支 %q", from, branch)
		}
		return to, nil
	}
	if to, ok := c.edges[from]; ok {
		return to, nil
	}
	return "", fmt.Errorf("graph: 节点 %q 没有出边，流程无法收敛", from)
}

// merge 把增量并进状态：注册过 reducer 的键做合并，Delete 键做删除。
func (c *Graph) merge(s, delta State) {
	for key, value := range delta {
		if _, isDelete := value.(deleteMarker); isDelete {
			delete(s, key)
			continue
		}
		if value == nil {
			continue
		}
		if r, ok := c.reducers[key]; ok {
			s[key] = r(s[key], value)
			continue
		}
		s[key] = value
	}
}

// Append 是切片累加 reducer：把增量追加到旧值后面，等价于 add_messages。
func Append(old, delta any) any {
	if old == nil {
		return delta
	}
	oldSlice, newSlice := reflect.ValueOf(old), reflect.ValueOf(delta)
	if oldSlice.Kind() != reflect.Slice {
		return delta
	}
	if newSlice.Kind() == reflect.Slice && newSlice.Type() == oldSlice.Type() {
		return reflect.AppendSlice(oldSlice, newSlice).Interface()
	}
	if newSlice.Kind() == reflect.Slice {
		out := oldSlice
		for i := 0; i < newSlice.Len(); i++ {
			out = reflect.Append(out, newSlice.Index(i))
		}
		return out.Interface()
	}
	return reflect.Append(oldSlice, newSlice).Interface()
}

// Strings 把状态里的值读成字符串切片，兼容 JSON 还原后的 []any。
func Strings(v any) []string {
	switch typed := v.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	case nil:
		return nil
	default:
		return []string{fmt.Sprint(v)}
	}
}

// CountWhere 统计消息里带某个前缀的条数，节点用它判断当前处于第几轮。
func CountWhere(s State, key, prefix string) int {
	n := 0
	for _, msg := range Strings(s[key]) {
		if len(msg) >= len(prefix) && msg[:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

func clone(s State) State {
	out := make(State, len(s))
	for key, value := range s {
		out[key] = value
	}
	return out
}
