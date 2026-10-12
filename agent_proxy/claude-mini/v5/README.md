# v5 — Subagent：任务拆分（含 v1..v4）

在 v4 之上叠加 **Subagent**。代码分析要把"读仓库 / 跑测试 / 写文档"这类子任务的
中间产物塞进同一条上下文，很快就把主对话挤爆；v5 让主 Agent 把子任务外包出去。

## 机制：子任务进独立上下文，只把结论带回来

`Task` 工具（`tools/subagent.go`，~60 行）：

1. **递归 Engine**：`tools.SubagentRunner` 是一个函数变量，`main.go` 把它接到一个
   **全新的 `engine.Engine`**（独立 history、独立 max turns）。用函数变量是为了打破
   `engine → tools → engine` 的 import 环。
2. **只回传最终文本**：子 Agent 跑完，只有它最后一段 assistant 文本进主对话的
   tool_result，中间几十次工具调用的噪声留在子上下文里。
3. **受限工具集**：`NewWorkerRegistry()` 给子 Agent 只暴露文件/计算工具，
   **拿掉 Task（禁止再套娃）、Memory、Skill**（经验归主 Agent 所有）。

## 与 v4 的 diff

| 文件 | 变化 |
|---|---|
| `tools/subagent.go` | 新增：Task 工具 + SubagentRunner 钩子 |
| `tools/tools.go` | 注册 Task；新增 NewWorkerRegistry（受限工具集） |
| `main.go` | 接线 SubagentRunner→新 Engine（每个 Task 一个干净上下文） |
| `engine/engine.go` | prompt 一句：独立子任务用 Task 外包 |
| `subagent_test.go` | 新增：worker 无 Task/Memory/Skill；Task 端到端回传结论 |

## 运行

```bash
env -u GOROOT go build -o v5-agent ./v5
./v5-agent --print -v "分别分析 internal/query 和 internal/tools 两个模块，再汇总"
# -v 里能看到 Task 委派：子分析在独立上下文完成，主线只收到结论
```
