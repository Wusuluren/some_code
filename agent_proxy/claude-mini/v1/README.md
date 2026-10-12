# v1 — Filesystem：长期工作空间

基线：`minimal/`（Cobra 入口 + Messages API client + executeQueryLoop + 5 工具）。
本版本叠加 Deep Agents 机制一的 **Filesystem（长期工作空间）**。

## 机制：模型缺的不是工具，是一个不会消失的工位

v0（minimal）里 cwd 只是"碰巧"是当前目录，会话结束上下文即丢失。v1 把工作环境变成一等公民：

1. **工作区可探索**：新增 `Ls` 工具（`tools/workspace.go`，+70 行），模型能看见
   自己/前人留在工作区里的中间结果文件。
2. **跨会话状态**：`engine.SaveState/LoadState`（`engine/engine.go`，+24 行）把
   conversation history 持久化到 `.agent_state.json`；`runOnce` 每轮结束自动落盘。
3. **CLI 开关**：`--state <path>` 指定工作区状态文件、`--resume` 恢复上次会话、
   `--reset` 清空（`main.go`，+22 行）。
4. system prompt 增加一句工作区约定：中间结果用 Write 落盘、用 Ls 探索。

## 与 v0 (minimal) 的 diff

| 文件 | 变化 |
|---|---|
| `tools/workspace.go` | 新增：Ls 工具 |
| `tools/tools.go` | 注册 Ls（5→6 工具） |
| `engine/engine.go` | Config.StatePath + SaveState/LoadState + prompt 一句 |
| `main.go` | --state/--resume/--reset flag，runOnce 结束时 SaveState |
| `filesystem_test.go` | 新增测试：Ls 列目录 + 状态跨两个会话存活 |

## 运行

```bash
env -u GOROOT go build -o v1-agent ./v1
./v1-agent --print "把 internal 各模块行数写到 loc.txt"
./v1-agent --resume --print "读 loc.txt，指出最大的模块"   # 上一轮的中间结果和上下文都还在
```
