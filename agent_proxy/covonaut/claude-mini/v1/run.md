export ANTHROPIC_API_KEY=sk-...

# 单轮 print 模式（流式 + verbose 工具行）
go run ./example/claude-mini -print -stream -verbose "列出当前目录并总结项目"

# 交互式 REPL
go run ./example/claude-mini

# TUI 模式（对应 v15 的 --tui）
go run ./example/claude-mini -tui "列出当前目录并总结项目"

# 会话管理
go run ./example/claude-mini -list
go run ./example/claude-mini -continue
go run ./example/claude-mini -provider openai -model gpt-4o-mini -print "hello"

# 自定义 API Key / Base URL（-api-key / -base-url 优先于环境变量）
#   兼容 OpenAI Chat Completions 的自建/代理网关：
go run ./example/claude-mini -provider openai -model gpt-4o-mini \
  -api-key "sk-your-key" -base-url "https://your-gateway.example.com/v1" \
  -print "hello"
#   Anthropic 兼容网关（如自建反代 / 三方服务）：
go run ./example/claude-mini -provider anthropic -model claude-sonnet-4-20250514 \
  -api-key "sk-ant-your-key" -base-url "https://your-anthropic-proxy.com" \
  -print "hello"
#   也可只用环境变量：OPENAI_API_KEY/OPENAI_BASE_URL、ANTHROPIC_API_KEY/ANTHROPIC_BASE_URL、
#   GEMINI_API_KEY/GEMINI_BASE_URL，或通用的 API_KEY。

# MCP（默认读取 ./.mcp.json，缺失则自动禁用；-mcp 指定路径，-mcp "" 关闭）
#   ./.mcp.json 采用 Claude Code / Cursor 形状，支持 ${VAR} / ${VAR:-default} 展开：
#   {
#     "mcpServers": {
#       "filesystem": {
#         "command": "npx",
#         "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]
#       },
#       "remote": { "url": "https://example.com/mcp" }
#     }
#   }
# 工具以 mcp__<server>__<tool> 命名，避免与内置工具冲突；连不上的服务会被跳过、不影响会话。
go run ./example/claude-mini -verbose -print "用 filesystem 工具列出当前目录"
go run ./example/claude-mini -mcp ./custom.mcp.json -tui