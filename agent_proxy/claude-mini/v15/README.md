# v15 — TUI 前端：最小 Bubble Tea，消费 Event channel（含 v8..v14）

在 v14 之上补 **TUI 前端**。v1..v14 只有 stdout 打印循环；v15 加一个交互式终端 UI，
**不改引擎**——它只是引擎 Event channel 的另一个消费者，对应 jingwei 的 ratatui 前端。

## 机制：Bubble Tea 模型 + 纯事件渲染函数

1. **`ui/tui.go` 新增**（依赖仓库里已有的 bubbletea + lipgloss，未引新模块）：
   - **`appendEvent(lines, ev)` 纯函数**：把 `assistant_text`/`tool_use`/`tool_result`/`result`
     折进 transcript 行。连续的 `assistant_text` 片段拼到同一行（流式效果），
     非助手事件另起一行。纯函数、无 TTY，可直接单测。
   - **事件泵**：`startRun` 返回一个 `tea.Cmd`，在 goroutine 里 `for ev := range eng.Run(...)`
     逐事件 `p.Send(eventMsg{ev})`，结束发 `doneMsg`。Bubble Tea 启动后才执行，`p.Send` 安全。
   - **手工输入行**：不引 `bubbles`，用 `tea.KeyMsg` 逐字符处理 Enter/Backspace/可见键；
     `busy` 时锁输入；Ctrl-C/Ctrl-D 退出。
2. **`main.go` 接线**：`--tui` 分支在 print/REPL 之前，调用 `ui.RunTUI(ctx, eng, prompt)`，
   退出时 `SaveState`。TUI 接管 stdin，故 `buildGuard` 在 tui 模式像 print 一样自动放行并告警
   （避免两处争抢 stdin）。

## 与 v14 的 diff

| 文件 | 变化 |
|---|---|
| `ui/tui.go` | 新增：Bubble Tea 模型、`appendEvent`、事件泵、`RunTUI` |
| `ui/tui_test.go` | 新增：流式拼接、工具/结果换行、result 汇总、clip |
| `main.go` | 新增 `--tui` 分支 + `modeName()`；guard 在 tui 模式自动放行 |

## 运行

```bash
env -u GOROOT go build -o v15-agent ./v15
./v15-agent --tui "列出当前目录并总结项目"   # 实时 transcript，底部输入下一轮
# 也可先 --tui 不带 prompt，进去后逐轮输入；Ctrl-C 退出
```
