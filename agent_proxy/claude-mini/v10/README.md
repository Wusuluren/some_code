# v10 — MCP 客户端（Model Context Protocol，含 v8 协议 / v9 流式）

在 v9 之上补 **MCP**。不改一行代码就能给 Agent 加工具，是 jingwei 和主仓
`internal/services/mcp/` 都在做的扩展方式：外部进程用 JSON-RPC over stdio
暴露自己的工具，Agent 启动时问一遍，模型看到的就是和内置同形状的工具。

## 机制：新增独立 `mcp` 包，工具挂进现有 Registry

1. **`mcp/config.go`**：读 Claude/Cursor 通用的 `{"mcpServers":{name:{command,args,env}}}`，
   对 command/args/env 每个字符串做 `${VAR}` 与 `${VAR:-默认}` 展开。
   只有 `url` 没有 `command` 的（HTTP 传输）本最小版跳过。
2. **`mcp/client.go`**：`Start` 用 `os/exec` 拉起 server，走 stdin/stdout 的
   **换行分隔 JSON-RPC 2.0**。`call` 发请求、按 id 匹配响应（跳过通知与别的 id）；
   提供 `Initialize`（握手 + `notifications/initialized`）、`ListTools`、`CallTool`。
   `newPipeClient` 让测试用 `io.Pipe` 驱动同一套协议，不必真起进程。
3. **`mcp/manager.go`**：`MCPTool` 把某个 server 工具包成 `tools.Tool`，
   命名 `mcp__<server>__<tool>` 避免与内置撞名；`Connect` **尽力而为**——
   起不来或握手失败的 server 直接跳过，绝不拖垮整个会话。
4. **`main.go`**：`--mcp`（默认 `.mcp.json`），文件不存在即关闭 MCP；
   存在则把发现的工具 `Register` 进主 registry。server 随进程 ctx 取消而回收。

## 与 v9 的 diff

| 文件 | 变化 |
|---|---|
| `mcp/config.go` | 新增：mcp.json 解析 + `${VAR}` 展开 |
| `mcp/client.go` | 新增：stdio JSON-RPC 客户端（initialize/list/call） |
| `mcp/manager.go` | 新增：MCPTool 命名空间包装 + 尽力而为的 Connect |
| `main.go` | `--mcp` flag，连接并把工具注册进 registry |
| `mcp/mcp_test.go` | 新增：假 server 走 io.Pipe 跑握手/列表/调用、配置展开、坏 server 跳过 |

## 运行

```bash
env -u GOROOT go build -o v10-agent ./v10
cat > .mcp.json <<'JSON'
{"mcpServers":{"files":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp"]}}}
JSON
./v10-agent -v "用 files 服务列一下 /tmp"   # 模型会看到 mcp__files__list_files 等工具
```

## 测试

```bash
env -u GOROOT go test ./v10/...
```
