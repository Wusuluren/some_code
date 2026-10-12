# v9 — 流式输出 SSE（含 v8 双方言）

在 v8 之上补 **流式**。v8 之前每次调用都要等模型整段返回才打印，
长回答体验很差。jingwei 默认 streaming，本版把这条补上：token 边到边出。

## 机制：只在 client 边界做增量解码，回给 engine 的仍是整段中性响应

1. **`api/stream.go` 新增 `CreateMessageStream`**：请求置 `stream:true`、
   `Accept: text/event-stream`，用 `scanSSE` 逐行读 `data:` 事件。
   - Anthropic 方言：`content_block_start/delta/stop` 按 `index` 累积，
     `text_delta` 回调 `onText`、`input_json_delta` 拼 tool 输入；
     `message_delta` 收 `stop_reason` 与 output_tokens。
   - OpenAI 方言：`choices[].delta.content` 回调、`delta.tool_calls` 按 id 聚合
     arguments 碎片，`data: [DONE]` 收尾。
   两种方言最终都**重组成同一个 `MessageResponse`**，所以工具执行逻辑零改动。
2. **engine 侧**（`engine/engine.go`）：`Config.Stream` 为真时改调流式方法，
   把每个文本增量作为 `assistant_text` 事件即时发出；否则维持原阻塞路径。
3. **CLI 侧**（`main.go`）：`--stream` flag；流式下逐 token `fmt.Print`，
   在 tool_use / result 前补一个换行（`closeStreamLine`）。

## 与 v8 的 diff

| 文件 | 变化 |
|---|---|
| `api/stream.go` | 新增：SSE 读取 + 两方言增量解码重组 |
| `api/client.go` | MessageRequest 加 Stream 字段 |
| `api/wire.go` | oaRequest 透传 Stream |
| `engine/engine.go` | Config.Stream；流式分支与增量事件 |
| `main.go` | --stream flag 与逐 token 打印 |
| `stream_test.go` | 新增：假 SSE，文本分片流式、tool 输入分片重组后执行 |

## 运行

```bash
env -u GOROOT go build -o v9-agent ./v9
export ANTHROPIC_API_KEY=sk-ant-...
./v9-agent --stream "讲讲这个仓库的结构"   # 回答逐字出现
```

## 测试

```bash
env -u GOROOT go test ./v9/...
```
