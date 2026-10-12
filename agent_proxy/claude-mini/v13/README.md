# v13 — 会话管理：按项目多会话 + 时间戳 ID + --list/-c（含 v8..v12）

在 v12 之上补 **会话管理**。v1..v12 只有一个 `--state` 文件，会话要么覆盖要么手动指定路径。
v13 搬进 jingwei 的会话模型：每个项目一个目录、每次运行一个时间戳会话、可列出、可续跑。

## 机制：一个 Store 目录 + 复用引擎已有的 history 格式

1. **`sessions/` 新增包**（不发明新的磁盘格式，直接存引擎序列化的 history）：
   - **`ProjectKey(cwd)`**：`basename + sha256(cwd)[:12]`，文件系统安全、抗碰撞，
     按项目隔离会话。
   - **`NewID(t)`**：时间戳 ID，形如 `20260916-115605`。
   - **`NextID()`**：`MkdirAll` 后避让已存在文件（同名追加 `-1/-2…`）。
   - **`List()`**：Glob `*.json`，最小解码取首条 user 文本做 preview，按 ID 倒序。
   - **`Newest()` / `Resolve(prefix)`**：续跑最新；按 ID 或 ID 前缀定位（精确 > 唯一前缀，
     无匹配 / 歧义报错）。
2. **`main.go` 接线**：`--list` 列出并退出；`-c/--continue` 续最新；`--session` 按 ID/前缀续；
   目录解析优先级 `--sessions-dir` → `AGENT_SESSIONS_DIR` → `~/.v13-agent/sessions/<ProjectKey>` → `cwd/.agent_sessions`。
   除非用户显式 `--state`，持久化自动路由到会话存储的新 ID。

## 与 v12 的 diff

| 文件 | 变化 |
|---|---|
| `sessions/sessions.go` | 新增：Store + ProjectKey + NewID/NextID + List/Newest/Resolve |
| `sessions/sessions_test.go` | 新增：key 确定性、ID 格式、list/newest/resolve、NextID 避让 |
| `main.go` | 新增 `--list`/`-c`/`--session`/`--sessions-dir` + sessionsDir()/listSessions() |

## 运行

```bash
env -u GOROOT go build -o v13-agent ./v13
./v13-agent "帮我看看这个项目结构"      # 自动新建一个时间戳会话
./v13-agent --list                       # 列出本项目所有会话（ID + 预览）
./v13-agent -c "接着刚才的继续"          # 续跑最新会话
./v13-agent --session 20260916 "..."     # 按 ID/前缀续跑
```
