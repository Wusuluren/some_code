# reasonix-mini

Reasonix 编码 Agent 的**最小可编译骨架**，按父仓库 `docs/SPEC.md` 的 Step 0→3 实现：

| Step | 包 | 对应 SPEC |
| --- | --- | --- |
| 0 配置加载 | `internal/config` | §5：flag 不参与合并链，项目 `./reasonix-mini.toml` > 用户 `~/.reasonix-mini/config.toml` > 内置默认；密钥只经 `api_key_env` 从环境读取；`REASONIX.md`/`AGENTS.md` 折入缓存稳定前缀 |
| 1 Provider | `internal/provider` + `internal/provider/openai` | §3.1/§4：接口 + kind→factory 注册表；OpenAI 兼容 SSE 流式，tool_call delta **按 index 累积**只吐完整调用 |
| 2 Tool | `internal/tool` + `internal/tool/builtin` | §3.2：接口 + 编译期 `init()` 自注册；8 个内置工具 `read_file write_file edit_file move_file bash ls glob grep`；`Execute` 错误返回给模型而非崩溃（§6） |
| 3 Agent | `internal/agent` + `internal/permission` | §3.4 循环 + §3.7 权限门：`deny > ask > allow > fallback`，只读默认放行；headless 时 Ask→allow；`Bash(go test:*)` 前缀规则遇到 shell 操作符失效；`Edit(docs/**)` 文件族共享；写工具受 cwd confine（§5 sandbox 的骨架版） |

设计约束与父仓库一致：**cache-first** —— system 前缀与工具 schema（按名排序）每次运行只构建一次、字节稳定，会话历史 prepend-only 增长。

## 使用

```sh
go build -o bin/reasonix-mini .

export DEEPSEEK_API_KEY=sk-...
./bin/reasonix-mini setup        # 写 ~/.reasonix-mini/config.toml（已存在则拒绝覆盖）
./bin/reasonix-mini              # 交互式会话：/help /new /exit，Ctrl-D 退出
./bin/reasonix-mini run "..."    # 一次性 headless（Ask→allow，deny 仍硬拦）
./bin/reasonix-mini -model provider/model ...
```

配置示例见 `reasonix-mini.example.toml`。

## 与完整版差距（后续 Step 6+）

无 TUI（纯 stdin/stdout REPL）、无 MCP 插件、无上下文压缩/记忆检索、无 Coordinator 双模型、无 OS 沙箱（仅进程内 confine）、无审批规则持久化到 config（`a` 回答只降级为会话级 grant 并打印应添加的规则）。依赖同样保持精简：仅 `BurntSushi/toml`。

验证：`gofmt -l . && go vet ./... && go test ./...` 全绿。
