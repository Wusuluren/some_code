# v6 — Human-in-the-loop：关键操作卡口（含 v1..v5）

在 v5 之上叠加 **Human-in-the-loop**。Agent 能删文件、跑任意 shell、委派子任务，
全自动就意味着高风险操作没人拦；v6 在执行前插一道人工确认。

## 机制：工具执行前问一句，拒绝原样回给模型

1. **engine 侧钩子**（`engine/engine.go`）：`Config.Guard` 是个可选回调，
   `executeTools` 每跑一个工具前先问 Guard；返回 deny 就**跳过执行**，把
   `user rejected: <原因>` 作为 `is_error` 的 tool_result 回给模型，循环照常继续
   （模型可改道或放弃）。
2. **CLI 侧确认**（`main.go` 的 `buildGuard`）：`--guard-tools`（默认 `Bash,Write,Edit`）
   列出需要审批的工具，命中时打印调用详情并在 stdin 等 `y`/`n`/`a`
   （`a`=本会话对该工具后续都放行）。`--print` 非交互模式自动放行并告警。

## 与 v5 的 diff

| 文件 | 变化 |
|---|---|
| `engine/engine.go` | Config.Guard 字段 + executeTools 里的门禁分支 |
| `main.go` | --guard-tools flag、buildGuard 交互确认；Subagent 循环判空更严谨 |
| `hitl_test.go` | 新增：deny 不落盘且回传模型、allow 正常执行、循环继续 |

## 运行

```bash
env -u GOROOT go build -o v6-agent ./v6
./v6-agent -v "删掉 out.txt 再重写"
# ⚠ Write {"file_path":"out.txt",...}  allow? [y]es / [N]o / [a]lways:
# 按 n，模型收到 "user rejected"，会换一种做法或停下解释
```
