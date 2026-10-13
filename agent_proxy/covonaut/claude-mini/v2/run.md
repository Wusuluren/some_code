export ANTHROPIC_API_KEY=sk-...

# 单轮 print 模式（流式 + verbose 工具行）
go run ./example/claude-mini/v2 -print -stream -verbose "列出当前目录并总结项目"

# 交互式 REPL
go run ./example/claude-mini/v2

# TUI 模式（对应 v15 的 --tui）
go run ./example/claude-mini/v2 -tui "列出当前目录并总结项目"

# 会话管理
go run ./example/claude-mini/v2 -list
go run ./example/claude-mini/v2 -continue
go run ./example/claude-mini/v2 -provider openai -model gpt-4o-mini -print "hello"

# 自定义 API Key / Base URL（-api-key / -base-url 优先于环境变量）
#   兼容 OpenAI Chat Completions 的自建/代理网关：
go run ./example/claude-mini/v2 -provider openai -model gpt-4o-mini \
  -api-key "sk-your-key" -base-url "https://your-gateway.example.com/v1" \
  -print "hello"
#   Anthropic 兼容网关（如自建反代 / 三方服务）：
go run ./example/claude-mini/v2 -provider anthropic -model claude-sonnet-4-20250514 \
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
go run ./example/claude-mini/v2 -verbose -print "用 filesystem 工具列出当前目录"
go run ./example/claude-mini/v2 -mcp ./custom.mcp.json -tui

# ─────────────────────────────────────────────────────────────
# v2 相对 v1 的两个新特性
# ─────────────────────────────────────────────────────────────

# 1) Task 子代理工具（agentcore.TaskTool + AgentAsTool）
#    新增内置 task 工具，参数 {agent:"worker", task:"..."}。worker 是一个只读
#    （read/grep/glob/find/ls）的全新子 Agent，跑完后把结论作为工具结果回传主 Agent，
#    主上下文保持干净。仅在 REPL / TUI 交互式模式下审批 worker 内的工具；print 模式自动放行。
go run ./example/claude-mini/v2 -verbose -print \
  "用 task 工具派 worker 找出项目里所有 Temporal workflow 定义，然后汇总"

# 2) 带 diff 预览的权限审批（BeforeHook）
#    交互式 REPL 下，命中 write_file / edit 时先在 stderr 打印统一 diff（不执行工具、
#    只读取当前文件做对比），再询问 allow? [y]/[N]/[a]。bash 仍显示命令原文。
#    新建 v2 的 workspace 目录试跑，观察 diff：
go run ./example/claude-mini/v2 -sandbox /tmp/mini-v2-ws
#    让模型写文件时，审批提示前会出现形如：
#      ⛛ hello.txt  + new file (3 lines)
#      + package main
#      ...
#    edit 则按 oldText/newText 逐块显示 -/+ 行；超过 60 行自动截断为 "… (N more lines)"。
#    注意：print / tui 模式无交互 stdin，会沿用 v1 行为自动放行并打印 warn（TUI 抢占屏幕，
#    故不在其中渲染 diff）。要看 diff 请用 REPL（不带 -print/-tui）。
