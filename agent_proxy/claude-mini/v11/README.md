# v11 — 上下文管理：token 估算 + 超限裁剪（含 v8/v9/v10）

在 v10 之上补 **上下文管理**。长会话里历史越滚越大，迟早顶爆 context window。
jingwei 的做法是"估算超过阈值就裁剪历史"，主仓有 `compact.go` + `token_estimation.go`；
v11 取最小版：请求期按估算预算发送一个**近期、且 API 合法**的历史后缀。

## 机制：只裁"发出去的那份"，落盘的完整历史不动

1. **`engine/compact.go` 新增**：
   - `estimateTokens`：对 text / tool_result / tool 输入按 `字符数/4` 粗估（无需真分词器）。
   - `trimForBudget`：预算内则原样返回；否则从前往后丢，直到窗口达标，
     但**保底保留最近 6 条**；再把起点向后滑动，**绝不让首条是 tool_result**
     （否则它的 tool_use 被裁掉，请求会非法）。返回保留的尾部与被丢条数。
2. **engine 主循环**：`Config.MaxContextTokens>0` 时对 `e.history` 求一份裁剪后的
   `req.Messages`，并在 system 末尾追加"已裁剪 N 条"提示。**完整 `e.history`
   不受影响**（状态文件仍存全量），只影响这一次请求。
3. **CLI**：`--context-size`（token，0=关闭），主 Agent 与子 Agent 都接。

## 与 v10 的 diff

| 文件 | 变化 |
|---|---|
| `engine/compact.go` | 新增：token 估算 + 预算裁剪（保底近期、避开 tool_result 起点） |
| `engine/engine.go` | Config.MaxContextTokens；请求前裁剪 + system 提示 |
| `main.go` | `--context-size`，接入主/子 engine |
| `engine/compact_test.go` | 新增：预算/保底/非法起点规避 |

## 运行

```bash
env -u GOROOT go build -o v11-agent ./v11
./v11-agent --context-size 20000 "继续分析这个仓库"   # 历史估算超 2 万 token 就发近期窗口
```

## 测试

```bash
env -u GOROOT go test ./v11/...
```
