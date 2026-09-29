# Proxy 共性总结与最小实现步骤

> 来源：本仓库 7 个教学版实现的横向对比。所有 `file:line` 均可跳转验证。

## 0. 七个实现定位表

| 目录 | 角色 | 上游 | 翻译量 |
|---|---|---|---|
| `aicli/miniproxy` | 纯转发 | OpenAI 兼容 | 0（字节透传 + 换 key） |
| `opencode2api/simpleproxy` | 转发 + 伪装 | OpenCode Zen | 同协议，改 body/头 |
| `opencode2api/simpleproxy-stateful` | 带服务端账本 | 同上 | 同上 + 服务端拼 messages |
| `CLIProxyAPI/simple-proxy` | 跨格式 | Gemini generateContent | 结构体级字段映射 |
| `mimocode2api/miniproxy` | 跨协议 | opencode 会话式 REST + SSE | 会话生命周期桥接 |
| `glm-web-code/mini-proxy` | 跨传输 | chatglm.cn 网页（CDP） | 非 HTTP，轮询 + 回灌 |
| `aicli/scli` | 客户端（反向） | OpenAI 兼容 | 调 LLM 而非服务 LLM |

---

## 1. 共性

### 1.1 都是"教学版最小复刻"，不是独立产品
每个文件头部注释都写明原理对应原项目的哪个函数/行号，例如
`opencode2api/simpleproxy/main.go:6-14`（`gateway.handleInference → handleChatCompletions`）、
`CLIProxyAPI/simple-proxy/main.go:117`（`internal/translator/gemini/...`）。
清一色**单文件、仅标准库**（`glm-web-code` 多 goja + websocket 两个依赖），
并主动删掉原项目的配置系统 / 多 Provider / 图片多模态 / 重试 / 请求队列。

### 1.2 都把 OpenAI Chat Completions 当通用接口
5/7 的对外契约就是 `POST /v1/chat/completions`，多余路径一律 404。
这是"最小公分母"选择：任何现成客户端都能接。

### 1.3 凭证边界一致：假 key 对内、真 key 对外
- `envOr(k, def)` 这个 5 行小工具在 3 个文件里逐字重复。
- 鉴权都是开关式：`localKey == "" → 不鉴权`
  （`simpleproxy/main.go:129`、`simpleproxy-stateful/main.go:93`、`mimocode2api/miniproxy/proxy.go:490`）。
- 上游侧重新注入：`req.Header.Set("Authorization", "Bearer "+upKey)`。
- 默认监听 `127.0.0.1`（本地开发工具，不是生产网关）。

### 1.4 流式是真正的技术难点，且各家写法收敛到同一套
全都：`bufio.Scanner` + `sc.Buffer(64KB, 上限 1~10MB)` 逐行读 SSE、认 `data:` 前缀、
认 `[DONE]`、每帧后 `flusher.Flush()`。
且都面对同一个二选一：**relay（透传流）** vs **collapse（折回 JSON）** ——
`simpleproxy/main.go:191-199` 的三条分支最典型（免费档强制 `stream=true`，而客户端可能想要 JSON）。

### 1.5 都实现了"无状态客户端 → 有状态上游"的身份映射
形式完全不同，目标一样：**同一会话用同一 session 种子，才能吃到上游 prompt 缓存**。
- `canonicalSessionID`：sha256 派生 `ses_` + 12hex + 14base62（`simpleproxy/main.go:61-86`、`simpleproxy-stateful/main.go:81-91` 是同一函数的两份拷贝）。
- `mimocode2api/miniproxy/proxy.go:306-424`：建会话 → 订阅 → 用完即删。
- `glm-web-code/mini-proxy/main.go:325-330`：sha256 去重 `seen` 集合。

### 1.6 错误与资源的约定统一
- `io.LimitReader` 限 body（1 / 10 / 32 / 50 MB）。
- 上游不可达 → 502；非法 JSON → 400；key 不对 → 401。
- **上游错误体原样透传，不翻译**（`CLIProxyAPI/simple-proxy/main.go:240-244`）。
- `defer resp.Body.Close()`；context 一路从 `r.Context()` 传下去。
- 收尾清理用 `context.WithoutCancel` 派生，避免跟着请求 context 一起死掉
  （`mimocode2api/miniproxy/proxy.go:317`）。

### 1.7 都在处理"不可信输入"，但分层不同
| 实现 | 手段 |
|---|---|
| `aicli/scli` | 危险命令正则 + 人工确认（`main.go:110-120`） |
| `glm-web-code/mini-proxy` | goja 封 `eval`/`Function`（`main.go:174-177`）+ 30s `vm.Interrupt` + 输出截断 8000 |
| `simpleproxy-stateful` | 四条防线：服务端签发会话、只读 `content`、system 由环境变量决定、历史只存服务端账本（`main.go:1-23,138-183`） |
| 其余 | 直接透传 |

### 1.8 可观测性与运维接近于零
只有 `log.Printf` + 一行启动日志（listen / upstream / auth 模式）。
没有 metrics、trace、优雅退出（全是 `log.Fatal(http.ListenAndServe(...))`）、
限流与并发控制（唯一例外：`mimocode2api` 的一把 `sync.Mutex`，因为后端同时只跑一个会话）。
token 用量在 `mimocode2api/miniproxy/proxy.go:523` 是 `len/4` 粗估。

### 1.9 共同的测试钩子
`UPSTREAM` / `GEMINI_BASE` / `BACKEND_URL` 环境变量可以把上游指到本地 mock server，
不需要真实 key 就能跑完整链路。

**一句话概括**：这批代码是同一个模板的 7 种变体——单文件、环境变量配 key、OpenAI 格式对外、
逐行 SSE + Flush、把上游的差异（伪装头 / 另一套 JSON / 会话式 REST / 浏览器 DOM）消化在代理内部。

---

## 2. 实现一个 proxy 的最小步骤

> 地板是 `aicli/miniproxy`（116 行）；其余实现都是在它之上加码。

### 2.1 最小可行骨架（9 步，约 40 行）

```
1. 契约     mux.HandleFunc("POST /v1/chat/completions", handle)
2. 限体积   body := io.ReadAll(io.LimitReader(r.Body, N<<20))
3. 鉴权     if localKey != "" && !auth(r) → 401        // 开关式：不设 key 就不鉴权
4. 换凭证   upReq.Header.Set("Authorization", "Bearer "+realKey)
5. 建请求   http.NewRequestWithContext(r.Context(), POST, upstream+"/chat/completions", bytes.NewReader(body))
6. 发出去   resp, err := client.Do(upReq); defer resp.Body.Close()   // err → 502
7. 抬头     w.Header().Set("Content-Type", resp.Header.Get("Content-Type")); w.WriteHeader(resp.StatusCode)
8. 回传     按 Content-Type 分三条路走（见 2.2）
9. main     log.Fatal(http.ListenAndServe(addr, mux))
```

### 2.2 第 8 步的分岔 = 整个 proxy 的唯一实质逻辑

```go
if !strings.HasPrefix(ct, "text/event-stream") {  io.Copy(w, resp.Body)   } // 非流：字节透传
else if clientWantsStream                        {  relaySSE(w, resp.Body)  } // 流：边读边 Flush
else                                             {  collapseSSE(w, resp.Body)} // 流 → JSON 折叠
```

### 2.3 五个坑（每个都在代码里留有对策）

1. **不 Flush 就没有流式**。`http.ResponseWriter` 有缓冲，必须拿 `w.(http.Flusher)`；
   老写法用 `http.NewResponseController(w)`（`aicli/miniproxy/main.go:99`）。
2. **`http.Client` 不能设 `Timeout`**。它是整个请求的总时限，流式跑几分钟会被掐死；
   超时交给 per-request context（`aicli/miniproxy/main.go:36-38` 特意写了这条）。
3. **`bufio.Scanner` 默认 64KB 上限**，一个 tool_call chunk 超了就静默丢帧；
   必须 `sc.Buffer(make([]byte, 64<<10), 1<<20)`（`opencode2api/simpleproxy/main.go:207`）。
4. **`sc.Bytes()` 不含换行**，逐行读但要逐行写回 `\n`，否则 SSE 事件边界消失：
   `w.Write(append(line, '\n'))`（`opencode2api/simpleproxy/main.go:210`）。
5. **上游错误体原样透传，别自己造**。真实状态码 + 原始 JSON 一起回给客户端
   （`CLIProxyAPI/simple-proxy/main.go:240-244`）。

### 2.4 按需加码的三级台阶

| 触发条件 | 加什么 | 参考实现 |
|---|---|---|
| 上游不是 OpenAI 格式 | 定义 `xxxRequest/xxxResponse` 结构体做字段映射：`system` 单独拎出、`assistant → model`、`finishReason` 枚举转换、`stop → stopSequences` | `CLIProxyAPI/simple-proxy/main.go:118-163` |
| 上游是会话式 REST 而非单请求 | 建会话 → **先订阅 SSE 再发 prompt**（顺序反了丢首包）→ 靠 `session.idle` 判结束 → `DELETE` 清理；订阅失败要能回退轮询兜底 | `mimocode2api/miniproxy/proxy.go:306-424,427-468` |
| 客户端不可信 / 上游要求"像 agent 会话" | 服务端账本：只读 body 的 `model`+`content`，`messages` 由服务端拼、role 由服务端盖章、**上游报错不入账**保持历史干净、`maxTurns` 截断防无界增长 | `opencode2api/simpleproxy-stateful/main.go:138-183` |

跨进程 / 跨传输的额外一层：`glm-web-code/mini-proxy` 的"上游"是浏览器 DOM，
于是第 5 步变成 CDP `Runtime.evaluate` 抓代码块、第 8 步变成 `Input.insertText` + 模拟回车回灌
——但**轮询、去重、超时中断、输出截断**这四个骨架元素一个没少（`main.go:286-345`）。

### 2.5 两件几乎所有人都做的事

- **session 亲和**：把会话种子（客户端 session 头，或首条 user 消息）哈希成固定 ID 发给上游，
  多轮请求命中同一份 prompt 缓存。这是免费的性能收益。
- **`envOr(k, def)` 配一切**：`UPSTREAM / *_API_KEY / LISTEN / TIMEOUT_MS`，默认 `127.0.0.1`。

---

## 3. 收束

**proxy 的本质就是「第 4 步换 key + 第 8 步按 Content-Type 三分支」**，
其余全部代码都是在回答"上游到底有多不正常"。
