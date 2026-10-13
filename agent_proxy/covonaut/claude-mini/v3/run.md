export ANTHROPIC_API_KEY=sk-...

# 单轮 print 模式（流式 + verbose 工具行）
go run ./example/claude-mini/v3 -print -stream -verbose "列出当前目录并总结项目"

# 交互式 REPL（可带初始 prompt 参数，作为第一轮）
go run ./example/claude-mini/v3
go run ./example/claude-mini/v3 "把 README 里的安装步骤核对一遍"

# TUI 模式（底部 footer 即 TODO 面板，见下）
go run ./example/claude-mini/v3 -tui "列出当前目录并总结项目"

# 会话 / provider / MCP：与 v1/v2 完全一致（路径换成 v3 即可）
go run ./example/claude-mini/v3 -list
go run ./example/claude-mini/v3 -continue
go run ./example/claude-mini/v3 -provider openai -model gpt-4o-mini -api-key sk-x -base-url https://gw.example.com/v1 -print "hello"
go run ./example/claude-mini/v3 -mcp ./.mcp.json -verbose -print "用 filesystem 工具列出当前目录"

# ─────────────────────────────────────────────────────────────
# v3 新增：最小版 plan 交互（全部走公共 API，不改 covonaut 源码）
# ─────────────────────────────────────────────────────────────

# ① todo 工具（模型侧任务清单，REPL 实时回显）
#    工具语义：全量替换。模型每次更新都提交完整列表
#    （items:[{id?,content,status:pending|in_progress|completed|cancelled}]，items:[] 清空）。
#    system prompt 已引导："超过 3 步的任务先挂清单、每完成一步更新"。
#    每次模型调 todo 工具，REPL/print 模式在 stderr 打印：
#      📋 todo (4)
#        ● 读取配置  [completed]
#        ◐ 新增 flag  [in_progress]
#        ○ ...
#        ── 1/4 completed ──
go run ./example/claude-mini/v3 "给 mini.go 加 -json 输出和 -dry-run 两个 flag，先列任务清单再动手"

# ② TUI 的 TODO 面板（component.TodoPanel 的第一个消费者，经 ChatApp.SetFooter 挂载）
#    空闲时 footer 显示原默认内容；一旦模型挂出任务清单，footer 切换为面板
#    （状态图标 ○◐●✗ + Pending/Completed 统计），清单清空后自动还原。
go run ./example/claude-mini/v3 -tui

# ③ -plan：规划 → 人工批准 → 带清单执行
#    流程：只读 planner（复用 v2 worker 工具集）调研工作区 → 产出 JSON plan
#    （goal/steps[{description,files,verification}]/notes）→ 终端打印并 y/N 批准
#    → 批准后将 steps 种入 todo（第 1 步置 in_progress）→ 主 agent 按计划执行。
#    拒绝（n）则退出；print 模式沿用自动放行 + warn（与工具审批同规则）；
#    -tui 无交互 stdin，-plan 会被忽略并 warn。
go run ./example/claude-mini/v3 -plan "把会话存储从 JSONL 换成 sqlite"
go run ./example/claude-mini/v3 -print -plan "..."   # 自动批准（无 stdin 交互）

# 已知边界（最小版刻意不做）：
#   - 批准后无"改 plan"循环（拒绝即退出，改需重跑 -plan）
#   - plan 不持久化，-resume 恢复的是执行中的对话而非 plan 状态机
#   - TUI 面板只展示进度，Space/Enter 不反向改模型清单（toggle 回调未接）
