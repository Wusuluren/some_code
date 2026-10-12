# Deep Agent Harness — 逐版本最小实现（v1..v15）

两阶段阶梯：**v1..v7** 长出 Deep Agents 的 harness 机制；**v8..v15** 对照 jingwei
（Rust 写的"最小生产级编码 Agent"）补齐把它真正落地运行的工程能力。

起点是 `minimal/`（约 880 行的精简 Claude Code：Cobra 入口 + net/http Messages API
client + `executeQueryLoop` 主循环 + Bash/Read/Write/Edit/Grep 五工具）。它已经能跑
`用户输入 → LLM 推理 → 调工具 → 返回结果`，但一进真实工程就不够用了：代码分析 Agent
要持续读文件、跑命令、存中间结果、反复调试、拆子任务——**模型缺的从来不是"再多一个
工具"，而是一个稳定的工作环境（Agent Harness）。**

下面每个目录都是**自包含、可独立编译运行**的一版，在上一版之上只叠加 Deep Agents 的
一个机制，改动尽量小。按顺序读，就是 Harness 的生长史。

| 版本 | 机制 | 一句话 | 关键改动 |
|---|---|---|---|
| `minimal/` | （基线） | 最小可用 Agent 回路 | 5 工具 + executeQueryLoop |
| `v1/` | **Filesystem** | 长期工作空间：中间结果落盘、会话可恢复 | `Ls` 工具 + `SaveState/LoadState` + `--resume` |
| `v2/` | **Sandbox** | 隔离执行：文件工具收口、shell 环境脱敏 | `SetSandboxRoot/ScrubEnv` + `--sandbox` |
| `v3/` | **Interpreter** | 常驻 python 会话，变量跨调用保持 | `Repl` 工具（base64 + 哨兵协议） |
| `v4/` | **Memory + Skills** | 可复用经验沉淀到 `.agent/`，回注 prompt | `Memory`/`Skill` 工具 + prompt 注入 |
| `v5/` | **Subagent** | 子任务进独立上下文，只回传结论 | `Task` 工具递归起新 `Engine` + 受限工具集 |
| `v6/` | **Human-in-the-loop** | 关键操作执行前人工确认 | `Config.Guard` 钩子 + stdin `y/n/a` |
| `v7/` | （全机制协作） | 代码同 v6，加一条端到端集成测试 | `harness_test.go` 串起六个机制 |

## 设计取向：为什么这样切

- **每版只动一处**：新机制尽量落在新文件（`workspace.go`/`sandbox.go`/`interpreter.go`
  /`memory.go`/`subagent.go`），对既有代码只做接线级改动，方便 diff 出"这个机制到底改了什么"。
- **机制之间是串联的**：Sandbox 的根目录被 Interpreter/Subagent 复用；Memory 的落盘位置
  跟随 Sandbox；Guard 卡在 executeTools 入口，对所有工具（含 Task）统一生效。
- **可测**：每版都有对应 `_test.go`，用 `httptest` 假 API（无需真实 key）覆盖该机制；
  v7 的 `harness_test.go` 验证六机制能同场工作。

## 构建与测试

本机 Go 环境变量 `GOROOT` 指向旧版工具链，与 go.mod 要求冲突，统一用 `env -u GOROOT`：

```bash
# 全部版本一次性验证
env -u GOROOT go build ./v1/... ./v2/... ./v3/... ./v4/... ./v5/... ./v6/... ./v7/...
env -u GOROOT go test  ./v1/... ./v2/... ./v3/... ./v4/... ./v5/... ./v6/... ./v7/...

# 单版本示例（真实 API 端到端，需先 export ANTHROPIC_API_KEY）
env -u GOROOT go build -o v7-agent ./v7
./v7-agent --sandbox ./box -v "分模块分析 demos/，委派子任务，结论记进 memory"   # REPL 交互模式，命中 guard 会弹确认
```

各版本的细节见对应目录下的 `README.md`。

## 第二阶段：v8..v15 —— 生产落地能力（对照 jingwei）

v7 的 harness 齐了，但相比 jingwei 还缺 8 项"能稳定跑在生产里"的能力。每版仍只叠加一个
机制、改动收口在边界（协议翻译在 client、MCP 在独立包、裁剪在 engine），沿用 `--flag` 开关。

| 版本 | 能力 | 一句话 | 关键改动 |
|---|---|---|---|
| `v8/` | **多厂商线协议** | 同一中性模型，可选 OpenAI Chat Completions 方言 | `api/wire.go` 双向翻译 + `--protocol` |
| `v9/` | **流式 SSE** | 逐 token 边到边显示，增量块重组为中性响应 | `api/stream.go`（anthropic+openai）+ `--stream` |
| `v10/` | **MCP 客户端** | stdio JSON-RPC 2.0 接外部工具，命名 `mcp__server__tool` | `mcp/` 包 + `--mcp` |
| `v11/` | **上下文管理** | 估算 token，超限只裁发送副本、保底近期 | `engine/compact.go` + `--context-size` |
| `v12/` | **健壮文件写** | 原子 temp+rename、陈旧写拒绝、`edits[]` 原子批 | `tools/robust.go` + Write/Edit 重写 |
| `v13/` | **会话管理** | 按项目多会话、时间戳 ID、列出/续跑 | `sessions/` 包 + `--list`/`-c`/`--session` |
| `v14/` | **工具输出截断** | 所有工具结果在唯一收口点封顶 50KB | `engine/truncate.go`（`MAX_TOOL_OUTPUT`） |
| `v15/` | **TUI 前端** | 最小 Bubble Tea，消费同一 Event channel | `ui/tui.go` + `--tui` |

v8..v15 反向也保留了 jingwei 没有的 harness 机制（Sandbox/Interpreter/Memory/Subagent/
Human-in-the-loop），且协议、MCP、流式都收口在边界，上层引擎与中性模型基本不动。

```bash
# 第二阶段一次性验证
env -u GOROOT go build ./v8/... ./v9/... ./v10/... ./v11/... ./v12/... ./v13/... ./v14/... ./v15/...
env -u GOROOT go test  ./v8/... ./v9/... ./v10/... ./v11/... ./v12/... ./v13/... ./v14/... ./v15/...

# 组合示例：OpenAI 方言 + 流式 + MCP + 上下文裁剪 + TUI
env -u GOROOT go build -o v15-agent ./v15
./v15-agent --protocol openai --stream --context-size 8000 --tui "分析 demos/ 并总结"
```
