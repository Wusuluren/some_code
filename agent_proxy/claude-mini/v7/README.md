# v7 — 全机制协作（Deep Agent Harness 完整版）

代码与 v6 相同（六个机制 v1..v6 已全部就位）。v7 的价值是一个**集成测试**：
证明这些机制不是各玩各的，而是能在同一条 Agent 回路上串起来干活。

## 这版回答的问题

前面每版只加一个机制、只测那个机制。但真实工程场景里它们同时生效：
用户一个"分析仓库并总结经验"的请求，会**同时**触发工作空间、子 Agent、人工卡口、
经验沉淀。v7 用 `harness_test.go` 把这条链路一次性跑通。

## 一次请求里六个机制怎么串

```
用户 prompt
  └─ Engine.executeTools 遍历模型要的 tool_use：
       Task     →  Guard 先问人(v6) → 放行后 SubagentRunner 起独立子 Engine(v5) → 只回结论
       Memory   →  落到 .agent/memory.md(v4)，下一轮 buildSystemPrompt 注入(v4)
       (Repl/Ls/Write…) 都限定在 Sandbox 里(v2)，产物留在工作空间供跨会话复用(v1)
```

`TestFullHarness` 用一个假模型脚本走 Task→Memory→收尾，断言：
子 Agent 真的跑了、Guard 只对受管工具计数、Memory 落盘可读、
且**下一轮请求里带着子 Agent 的结论**（说明委派结果回流进主上下文）。

## 与 v6 的 diff

| 文件 | 变化 |
|---|---|
| `harness_test.go` | 新增：六机制端到端协作测试（其余代码与 v6 一致） |

## 运行

```bash
env -u GOROOT go test ./v7/          # 集成测试（假模型，不需 API key）
env -u GOROOT go build -o v7-agent ./v7
export ANTHROPIC_API_KEY=sk-ant-...
./v7-agent --sandbox ./box -v "分析 demos 目录，分模块委派子任务，把结论和踩坑记进 memory/skill"
```
