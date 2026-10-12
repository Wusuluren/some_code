# v3 — Interpreter：持久代码执行会话（含 v1 Filesystem + v2 Sandbox）

在 v2 之上叠加 **Interpreter**。v2 的 Bash 每次都是全新 shell，多步分析要重跑全部
准备代码；v3 给 Agent 一个"活的"解释器。

## 机制：反复调试依赖内存中的状态，不依赖每次从头执行

`Repl` 工具（`tools/interpreter.go`，~180 行）维护**一个常驻 python3 会话**：

1. **状态保持**：向子进程 stdin 发送 base64 编码的代码片段，子进程
   `exec(compile(...), g)` 在共享 globals 里执行——上一次定义的变量、函数、导入直接可用。
2. **输出切分**：每段执行完打印哨兵 `<<POTETO-DONE>>`，Go 侧读到哨兵即认为本段结束；
   stderr 合并进同一管道（io.Pipe），traceback 原样回给模型自纠错，且不会撑爆管道死锁。
3. **生命周期**：60s 超时杀进程并自动重开；`reset=true` 主动重开；python 缺失时
   报错回给模型而不是崩溃。
4. **与 v2 沙箱联动**：沙箱激活时解释器同样 `cmd.Dir` 进箱 + `ScrubEnv` 脱敏。

## 与 v2 的 diff

| 文件 | 变化 |
|---|---|
| `tools/interpreter.go` | 新增：Repl 持久会话工具 |
| `tools/tools.go` | 注册 Repl（7 工具） |
| `engine/engine.go` | prompt 一句：多步计算用 Repl |
| `interpreter_test.go` | 新增：跨调用状态保持 / traceback / reset |

## 运行

```bash
env -u GOROOT go build -o v3-agent ./v3
./v3-agent --print -v "用 python 分步分析 demos/ 下的数据：先加载，再统计，最后给结论"
# 观察 -v 输出：多次 Repl 调用之间变量没有丢
```
