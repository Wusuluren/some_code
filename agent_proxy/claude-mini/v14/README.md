# v14 — 工具输出统一截断 MAX_TOOL_OUTPUT（含 v8..v13）

在 v13 之上补 **工具输出上限**。一次失控的 Bash 或一个巨大的 Read 可能把整段输出
灌回模型，一口撑爆上下文。v14 照搬 jingwei 的 50 KB 工具输出天花板。

## 机制：在所有工具结果经过的唯一收口点裁剪

1. **`engine/truncate.go` 新增**：
   - **`MaxToolOutputBytes = 50_000`**：单个工具结果送回模型前的字节上限。
   - **`truncateToolOutput`**：超限则保留头 ~90% + 尾 ~10%，中间用一条
     `[... N bytes truncated ...]` 通知替代——让模型知道要**收窄调用**，
     而不是以为输出自然结束；裁剪后长度绝不超过上限。
2. **`engine.go` 接线**：`executeTools` 里 `runOneTool` 返回后立刻裁剪，
   覆盖 Read/Write/Edit/Bash/Grep/MCP 等**全部**工具——无需每个工具各自设防。

## 与 v13 的 diff

| 文件 | 变化 |
|---|---|
| `engine/truncate.go` | 新增：`MaxToolOutputBytes` + `truncateToolOutput` |
| `engine/engine.go` | `runOneTool` 结果统一裁剪后再入 history |
| `engine/truncate_test.go` | 新增：小输出不变、大输出封顶且保留头尾与通知 |

## 运行

```bash
env -u GOROOT go build -o v14-agent ./v14
./v14-agent -v "把整个仓库的文件内容 cat 出来"   # 超长结果被裁到 50KB 并附截断通知
```
