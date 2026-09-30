# minimal-agent — 精简版 Claude Code Agent 实现总结

参照主工程最小链路四源文件（`cmd/cli/main.go`、`pkg/api/client.go`、`internal/query/engine.go`、`internal/tools/tools.go`），
在 `minimal/` 下实现的可编译运行的精简 Agent。主链路约 780 行 + e2e 测试 100 行。

## 一、项目背景（原版功能摘要）

主工程 claude-code-go 是 Claude Code v2.1.88（Source Map 还原 TS 源码）的 Go 翻译版，约 4.2 万行，
包含：Agent 主循环、20+ 工具、OAuth/多云认证、重试、MCP、权限系统、Hooks、Bash 安全解析、
对话压缩、LSP、任务系统、Vim 模式、语音、插件、Bubble Tea UI 等模块。
本目录只保留其中能独立跑通的最小闭环。

## 二、目录结构

```
minimal/
├── main.go              # Cobra 入口（参照 cmd/cli/main.go）
├── main_test.go         # httptest 假 API server 端到端测试
├── api/
│   └── client.go        # net/http Anthropic Messages client（参照 pkg/api/client.go）
├── engine/
│   └── engine.go        # executeQueryLoop 主循环（参照 internal/query/engine.go）
└── tools/
    ├── tools.go         # Tool 接口 + Registry（参照 internal/tools/tools.go）
    ├── bash.go          # Bash：shell 执行 + 超时（默认 120s，上限 10min）
    ├── read.go          # Read：带行号，支持 offset/limit，默认 2000 行截断
    ├── write.go         # Write：自动建父目录 + confineToCwd 路径约束
    ├── edit.go          # Edit：精确替换、唯一性检查、replace_all
    └── grep.go          # Grep：纯 Go WalkDir+regexp，跳过 .git/node_modules，封顶 200 条
```

## 三、核心设计

### Agent 主循环（engine.go 的 executeQueryLoop）

```
for turn < maxTurns:
    resp = API(systemPrompt + history + toolDefs)
    history.append(assistant resp.content)          # 原样保留 tool_use 块
    blocks = 提取 resp 中的 tool_use
    if blocks 为空: 发出 result 事件，结束
    results = 逐个执行工具
    history.append(user 消息，含全部 tool_result 块)  # API 要求合并为一条
```

- 状态：`history []api.Message` 跨轮累积，REPL 模式下多轮共享同一 Engine 实例
- 事件流：`Run()` 返回 channel，发出 `assistant_text / tool_use / tool_result / result`，
  result 事件带累计 usage、轮数、耗时
- system prompt：环境段（cwd / GOOS / $SHELL）+ 工具使用约定（Read before Edit、优先专用工具、
  独立调用同轮并发、完成后简短收尾），对标 `constants.BuildSystemPrompt` 的紧凑版

### API 客户端（api/client.go）

- 纯 `net/http`，非流式 `POST {baseURL}/messages`，header：`x-api-key` + `anthropic-version: 2023-06-01`
- key/baseURL 三级回退：`--api-key` flag → `ANTHROPIC_API_KEY` env → 报错提示
- `ContentBlock` 单结构覆盖 text / tool_use / tool_result 三态（`omitempty` 区分）
- 错误解析：API 错误体 `{error:{type,message}}` 转为 Go error

### 工具层（tools/）

- `Tool` 接口四方法：`Name / Description / InputSchema / Call(ctx, json) (string, error)`
- 工具 error 不中断循环：engine 把 `is_error: true` 的 tool_result 回给模型自纠错
  （对标原版 ToolResult.Error 语义）
- Edit 对齐 Claude Code 行为：old_string 不存在报 not found、多处匹配且未设 replace_all 报歧义

### 安全垫

- Write/Edit 路径经 `confineToCwd`：相对路径挂到 cwd；`filepath.EvalSymlinks` 解析真实路径
  （处理 macOS `/var → /private/var` 符号链接）后校验不逃出 cwd，越界报错回给模型
- Bash 走 `exec.CommandContext` 超时强杀；Grep 只读、结果封顶

## 四、构建与运行

```bash
# 构建（注意：本机 GOROOT 环境变量指向 Homebrew go1.25.7，
# 与 go.mod 要求的 go1.26 工具链混用会报版本不匹配，需 env -u GOROOT）
env -u GOROOT go build -o minimal-agent ./minimal

export ANTHROPIC_API_KEY=sk-ant-...

# print 模式端到端（-v 打印每次工具调用）
./minimal-agent --print -v "统计本项目 internal 下各模块的代码行数"

# 交互 REPL（每行一个 prompt，/exit 或 Ctrl+D 退出）
./minimal-agent

# 其他 flag：-m/--model（默认 claude-sonnet-4-20250514）、--max-turns（默认 100）、
# --base-url、--api-key
```

## 五、验证结果

| 检查项 | 方式 | 结果 |
|---|---|---|
| 编译 | `go build ./minimal/...`、`go vet ./minimal/...` | ✅ |
| 端到端回路 | `go test ./minimal/`（httptest 假 API：第一轮返回 Write 的 tool_use，第二轮返回 end_turn；断言 2 次 API 调用、文件真实写出、事件流完整、usage 累加 30/8） | ✅ PASS |
| 五工具功能 | 单测覆盖 Write→Read→Edit→Bash→Grep 链路 + 错误路径（old_string 缺失、路径越界） | ✅ PASS |
| CLI 冒烟 | `version`；`--print` 缺 prompt 报错退出码 1；无 API key 友好报错 | ✅ |
| 主工程回归 | `go build ./...` 全仓编译 | ✅ 不受影响 |

**未验证项**：真实 Anthropic API 端到端（执行环境无 `ANTHROPIC_API_KEY`），
假 server 测试已覆盖协议格式与循环逻辑；设置真实 key 后建议先用
`--print "列一下当前目录的文件"` 做最小实测。

## 六、与完整版的差距（增量扩展路线）

按投入产出排序：重试/指数退避（429/529/401 分类）→ 流式输出（SSE）→
权限确认（CanUseTool 交互式弹窗）→ Bash 静态安全检查 → MCP 客户端 →
对话压缩（history 超限时 summarize）→ 子代理（Agent tool 递归调用 Engine）→
Bubble Tea TUI。原版对应模块在 `internal/services/api/retry.go`、
`internal/permissions/`、`internal/services/mcp/`、`internal/services/compact.go` 可直接参考移植。
