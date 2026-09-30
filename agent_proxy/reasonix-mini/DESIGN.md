# Reasonix 功能解析与最小实现骨架

> 本文档整理自对 [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)
> 源码、`README.md`、`docs/SPEC.md`、`REASONIX.md` 与依赖清单的分析，分三部分：
> 一、项目详细功能；二、实现它的最小化步骤；三、已生成的可编译骨架 `reasonix-mini/`。

---

## 一、项目是什么

**Reasonix** 是一个 **DeepSeek 原生的终端 AI 编码 Agent**——类似 Claude Code 的定位，但围绕
DeepSeek 的**前缀缓存（prefix cache）**做了深度调优，用一个单一静态 Go 二进制
（`CGO_ENABLED=0`，约 25 万行非测试 Go 代码、557 个包文件）实现。

核心理念：**内核只认识接口，一切能力都来自配置和插件**（模型、工具、Agent 行为全部在
`reasonix.toml` 里声明，无硬编码模型）。

## 二、详细功能清单

### 1. 多前端、同一内核
- **CLI / TUI**：Bubble Tea 交互式聊天终端
- **Desktop 桌面 App**：Wails 原生窗口（macOS/Windows/Linux，含自动更新、主题、Remote-SSH 远程工作区）
- **HTTP/SSE serve 模式**：`reasonix serve` 浏览器前端（带 token/password 鉴权）
- **ACP 协议后端**：`reasonix acp` 供 VS Code 扩展接入
- 所有前端共用一个传输无关的 `control.Controller`——行为加在控制器上，三个前端自动继承

### 2. 模型与 Provider 体系
- `Provider` 接口 + kind→factory 注册表；任何 OpenAI 兼容端点只是配置项，不是新代码
- 一个 vendor 端点可暴露多个 `models`，支持 `provider/model` 解析、按模型覆盖 `context_window`、价格配置
- **双模型协作（Coordinator）**：`planner_model` + executor 分别在**独立会话**中运行
  （planner 低频只读研究、executor 全工具执行），互不污染各自的缓存前缀；路由策略是
  确定性主机策略而非分类模型

### 3. 缓存优先的上下文引擎（最大特色）
- 系统提示前缀（base prompt + tools + memory）跨轮次**字节级稳定**，会话只 prepend 增长，
  保持 DeepSeek 自动前缀缓存命中率
- **分层上下文维护**：0.6 软提示 → snip（截断陈旧工具输出）→ prune（0.8 占位符化）→
  摘要压缩（fold），0.9 强制折叠；原始历史归档为 JSONL 可回溯
- `history` 工具：BM25 检索已归档会话；`memory`/`remember`/`forget`：分层记忆
  （项目/全局、`REASONIX.md`/`AGENTS.md`/`CLAUDE.md` 层级加载、`@path` 导入、`#<note>` 快记），
  每轮用户消息前做有界 BM25 记忆召回

### 4. 工具体系（19 个编译期自注册工具）
- 文件类：`read_file` / `write_file` / `edit_file` / `multi_edit` / `move_file` /
  `notebook_edit` / `delete_range` / `delete_symbol` / `ls` / `glob` / `grep`
- 执行类：`bash`（后台任务 `bash_output` / `wait` / `kill_shell`）、`web_fetch`、
  `code_index`（tree-sitter 代码索引）
- 流程类：`todo_write` / `complete_step`（任务契约 + 进度租约：8 轮无进展提示重估、16 轮暂停）、
  `task` / `fleet`（2–64 个子代理并发调度）

### 5. 插件 = MCP 客户端
- JSON-RPC 2.0 over **stdio / streamable-http / 旧版 SSE** 三种传输；工具名空间 `mcp__<server>__<tool>`
- 支持 `roots`、progress token、`readOnlyHint`/`destructiveHint` 映射调度与安全策略；
  兼容 Claude Code 的 `.mcp.json` 项目声明；MCP prompts → `/mcp__server__prompt` 斜杠命令、
  resources → `@server:uri` 引用

### 6. 权限与沙箱（双重防线）
- **Permissions（策略层）**：逐次工具调用 allow/ask/deny；Claude Code 风格规则
  `Bash(npm run test:*)`、`Edit(docs/**)`、精确命令字面量授权；`deny > ask > allow > fallback`；
  动态/嵌套命令（`eval`、`$()`、`-c`）强制人工确认，YOLO 也不能绕过 deny
- **Sandbox（执行层）**：文件写工具限制在 `workspace_root`（符号链接解析防逃逸）、
  `forbid_read` 禁读路径；bash 用 OS 级沙箱（macOS Seatbelt / Linux bubblewrap）jail，
  Windows 明确不发货沙箱并降级为 off

### 7. 协作模式与自主执行
- **Plan Mode**（计划先于执行、审批后开短暂的自动执行窗口）、**Goal 模式**（`/goal` 长时自主
  目标：持续续跑直到完成/连续 3 次相同阻塞/安全上限，含 AutoResearch 研究策略）
- **Checkpoints & rewind**（快照回退）、**Recovery / Safe Mode**（崩溃恢复、卡死看门狗）
- **Subagent profiles**：Skill `runAs: subagent` 复用为子代理档案，可按档案指定模型和推理强度
- **Skills**、**Hooks**（外部命令钩子）、**自定义斜杠命令**（`.reasonix/commands/*.md` + `$ARGUMENTS` 模板）
- **Bot 集成**（飞书/微信/QQ 机器人远程驱动会话）、**Remote-SSH**（SSH+SFTP+端口转发+远程 serve 引导）
- **i18n**（中英双语 UI）、telemetry、billing（区域定价）、LSP 客户端、能力诊断（capdiag）

---

## 三、实现它的最小化步骤

项目自身设计哲学就是答案（*"Evolve, don't over-engineer"*，早期依赖只有一个 TOML 解析器）。
最小内核 = **配置 + Provider 注册表 + Tool 注册表 + Agent 循环 + 权限门 + TUI**，约 6 步：

### Step 0：骨架与配置（`internal/config`）
`go mod init`（`CGO_ENABLED=0`，唯一依赖 BurntSushi/toml）。定义 `reasonix.toml` 加载：
解析顺序 **flag > 项目 ./reasonix.toml > 用户 ~/.reasonix/config.toml > 默认值**；
`[[providers]]`、`[agent]`、`[tools].enabled`、`[[plugins]]`。API key 用 `api_key_env`
指向环境变量，不落盘。

### Step 1：Provider 抽象 + OpenAI 兼容实现（`internal/provider`）
定义 `Provider` 接口、`Request/Message/ToolCall/Chunk` 数据类型和 kind→factory 全局注册表；
实现 `provider/openai`（SSE 流式 `/chat/completions`，**按 index 累积 tool_call delta，只吐完整
ToolCall**），`init()` 自注册 `"openai"`。DeepSeek 只是 `base_url=api.deepseek.com` 的一个配置实例。

### Step 2：Tool 接口 + 内置工具（`internal/tool/builtin`）
`Tool{Name, Description, Schema, Execute}`；先实现 8 个最小集
`read_file/write_file/edit_file/move_file/bash/ls/glob/grep`，`init()` 调 `RegisterBuiltin`；
每次运行按配置过滤组装成 per-run `*Registry`。**工具错误返回给模型而不是崩溃**，让模型自我纠错。

### Step 3：Agent 循环（`internal/agent`）
核心不到 100 行：

```
loop (bounded):
  req = {messages + tool schemas} → provider.Stream(ctx, req)
  边收 ChunkText 边打印；收齐 ChunkToolCall
  无工具调用 → 结束；否则逐个执行(内置/插件) → 结果作为 tool 消息 append → 继续
```

`ctx` 贯穿（Ctrl-C 中断在途请求）；`Session` 持 `[]Message`。

### Step 4：权限门（`internal/permission`）
`Policy{Mode, Allow/Ask/Deny rules}.Decide()` 纯函数 + 交互式 `Approver`。规则语法
`Tool` / `Tool(specifier)` / `Bash(x:*)`，优先级 `deny > ask > allow > fallback`
（只读工具默认 allow、写工具默认 ask）；无 TTY 的 headless 运行 Ask→allow 保持自主。
**这一步之前必须先把系统提示前缀做成字节稳定**（base prompt + 工具 schema 排序固定）。

### Step 5：CLI + 聊天 TUI（`internal/cli`）
子命令 `setup` / `run "..."` / 交互式；Bubble Tea 单输入框 TUI，`/` 和 `@` 自动补全。

### Step 6+：按 SPEC 顺序叠加
MCP 插件客户端 → 上下文维护（snip/prune/fold）→ 记忆系统 → 自定义命令/Plan Mode/Subagents/
Sandbox → 双模型 Coordinator、serve/ACP/桌面/Bot 等前端。

**一句话总结**：最小实现 = "配置驱动的 Provider×Tool 双注册表 + 一个流式 agent loop + 逐调用权限门"，
其余一切都是在五个骨架件之上按 SPEC 契约逐层叠加的插件化能力。

---

## 四、已生成骨架：`reasonix-mini/`

已按 Step 0→3 生成本目录下的可编译骨架，编译与测试全绿（`gofmt -l .` 无输出、
`go vet ./...` 通过、`go test ./...` 通过），并用本地假 OpenAI SSE 服务端跑通了完整
agent loop（流式文本 → 分片 tool_call 拼接 → 权限门放行执行 → 结果回填 → 模型收敛）。

### 4.1 目录与对应 SPEC

```
reasonix-mini/
├── go.mod                          # 唯一第三方依赖：BurntSushi/toml
├── main.go / signal.go             # CLI 装配 + REPL + setup + Ctrl-C ctx
├── reasonix-mini.example.toml / README.md
├── permission_test.go              # 权限规则回归测试
└── internal/
    ├── config/      # Step 0：项目>用户>默认三层合并、api_key_env、ResolveModel、REASONIX.md 折入前缀
    ├── provider/    # Step 1：Provider 接口 + kind→factory 注册表 + 数据类型
    │   └── openai/  #   SSE 流式，tool_call delta 按 index 累积只吐完整调用
    ├── tool/        # Step 2：Tool 接口 + 全局 builtin 集 + 按名排序的 per-run Registry
    │   └── builtin/ #   8 工具 + confine()(cwd防逃逸) + 纯 Go doublestar 子集 glob
    ├── permission/  # Step 3：deny>ask>allow>fallback、前缀规则遇 shell 操作符失效、Edit(docs/**)族共享
    └── agent/       # Step 3：stream→print→gate→execute→append→repeat，max_steps 有界
```

### 4.2 与 SPEC 一致的关键约束
- **cache-first**：system 前缀与工具 schema（按名排序）每次运行只构建一次、字节稳定，
  会话历史 prepend-only 增长。
- **错误不致命**：`Execute` 的错误/阻塞以字符串回填给模型（SPEC §6）。
- **headless 契约**：`reasonix-mini run` 无 TTY 时普通 Ask→allow，deny 仍硬拦。

### 4.3 使用方式

```sh
cd reasonix-mini && go build -o bin/reasonix-mini .

export DEEPSEEK_API_KEY=sk-...
./bin/reasonix-mini setup          # 写 ~/.reasonix-mini/config.toml（已存在则拒绝覆盖）
./bin/reasonix-mini                # 交互式：/help /new /exit，写工具逐次询问 y/s/a/n
./bin/reasonix-mini run "..."      # headless：Ask→allow，deny 硬拦
./bin/reasonix-mini -model provider/model ...
```

### 4.4 骨架刻意未做（留作 Step 6+）
无 TUI（纯 stdin/stdout REPL）、无 MCP 插件、无上下文压缩/记忆检索、无 Coordinator 双模型、
无 OS 沙箱（仅进程内 confine）、无审批规则持久化到 config（`a` 回答只降级为会话级 grant 并
打印应添加的规则）。依赖保持精简：仅 `BurntSushi/toml`。
