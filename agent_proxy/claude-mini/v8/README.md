# v8 — Provider 线协议抽象：Messages + Chat Completions 双方言

从 v8 起进入**生产落地能力**阶梯（对照 jingwei 与主仓蓝图，补齐 v7 缺的 8 项，v8..v15）。
v1..v7 解决的是"长任务工作环境"（harness），v8 起解决"能不能真的拿去对接各家模型干活"。

v7 的 client 只会说 Anthropic Messages 一种线协议（`POST /v1/messages`）。
jingwei 的卖点是"providers live behind a port"：换方言就能接 MiniMax/智谱/DeepSeek/OpenAI。
v8 把这条端口补上——**同一套 engine 与中性 ContentBlock 模型**，底层可切 Messages 或 Chat Completions。

## 机制：翻译只发生在 client 边界，上层不动

1. **中性模型不变**：engine 仍按 Anthropic 形状组装 `Message`/`ContentBlock`
   （text / tool_use / tool_result），`CreateMessage` 的入参出参签名不变。
2. **api/wire.go 新增方言翻译**：
   - `marshalOpenAIRequest`：system→首条 system 消息；assistant 的 tool_use→`tool_calls`；
     user 里的 tool_result→每条一个 `role:"tool"` 消息（用 `tool_call_id` 关联）；
     工具定义→`{type:"function", function:{name,description,parameters}}`。
   - `parseOpenAIResponse`：`choices[0].message`（content + tool_calls）→ 中性 text/tool_use 块，
     `finish_reason` 映射为 `stop_reason`，`prompt/completion_tokens` 映射为中性 Usage。
3. **client.go 按协议分支**：`Config.Protocol`（`anthropic` 默认 / `openai`）决定
   端点（`/messages` vs `/chat/completions`）、鉴权头（`x-api-key` vs `Bearer`）、
   默认 base URL 与 key 的环境变量回退。`--protocol openai` 未给 `-m` 时默认 `gpt-4o-mini`。

## 与 v7 的 diff

| 文件 | 变化 |
|---|---|
| `api/wire.go` | 新增：Chat Completions 双向翻译（中性↔wire JSON） |
| `api/client.go` | Config/Client 加 Protocol；CreateMessage 按协议选端点/头/编解码 |
| `main.go` | `--protocol` flag；openai 方言的默认 model |
| `protocol_test.go` | 新增：假 OpenAI-compat server 跑通 tool_call 往返，断言 wire 形状 |

## 运行

```bash
env -u GOROOT go build -o v8-agent ./v8
# 接任意 Chat Completions 兼容端点：
./v8-agent --protocol openai --base-url https://api.openai.com/v1 -m gpt-4o-mini \
  --api-key sk-xxx "统计当前目录有几个 .go 文件"
# 默认仍是 Anthropic Messages：
./v8-agent "hello"
```

## 测试

```bash
env -u GOROOT go test ./v8/...
```
