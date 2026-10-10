# demos：三个 mini demo，各自跑清一件事

对照前两张页面里的结论：**LangChain 让你画出流程，Claude Code 让模型走出流程，RAG 负责把知识递到模型嘴边。**
这里用三个 mini demo 把它们跑出来。

| | `langchain/` | `langgraph/` | `rag/` |
|---|---|---|---|
| 用什么库 | `github.com/tmc/langchaingo` v0.1.15（社区 Go 移植） | 无。LangGraph 没有官方 Go 版，`graph/` 是零依赖手写的最小实现 | 无。`rag/` 零依赖手写 TF-IDF 余弦召回 |
| 抽象 | PromptTemplate + Chain + Memory | State + Node + Edge/条件边 + Reducer + Checkpointer | Chunk + 向量 + top-k 召回 + 检索增强 prompt |
| 回答的问题 | 流程怎么串 | 流程带分支和断点怎么跑 | 模型不知道的东西怎么递给它 |
| 控制流 | 代码写死的单向管道 `A -> B -> C` | 允许回环 `agent -> tools -> agent`，下一步由状态决定 | 一次性：检索 -> 拼装 -> 生成，没有回环 |
| 状态 | 只把上一跳输出传给下一跳 | 一个共享 State，reducer 决定合并方式 | 索引在内存，查询之间不累积 |
| 中断续跑 | 无 | `Interrupt` + `Resume`，断点落盘 | 不适用（重新索引代价低） |

三个 demo 都不联网、不需要 API key，`go test ./...` 即可验证。

## 1. demos/langchain —— 链式管道

```
text --LLMChain(summary)--> summary --LLMChain(keywords)--> keywords --LLMChain(english)--> english
```

- `llm.go`：`scriptedModel` 实现 `llms.Model`，按 prompt 里的 `[task=xxx]` 标记返回固定文本，
  并记录每次收到的 prompt，测试用它断言"上一跳的输出确实进了下一跳的模板"。
- `main.go`：`chains.NewSequentialChain` 串三跳；`chains.NewConversation` + `memory.NewConversationWindowBuffer(2)`
  演示 history 由框架注入 prompt。
- 跑真实模型：`go run . -real`（需要 `ANTHROPIC_API_KEY`，模型名可用 `CLAUDE_MODEL` 覆盖）。

踩到的坑（Go 版和 Python 版行为不一致）：

1. `SequentialChain.Call` 把 `inputs = outputs`，**只传上一跳的输出**。
   第三跳想同时用 `summary` 和 `keywords` 会直接报 `missing key in input values: summary`。
   要汇聚多输入，只能自己拼 map 或用 `TransformChain`。
2. `ContentChoice` 没有 `FinishReason` 字段。
3. `LLMChain.OutputKey` 是公开字段，得构造后赋值，构造函数不接。

## 2. demos/langgraph —— 状态图

```
        ┌────────────┐
        │   agent    │  看 messages 决定：调工具 or 收尾
        └─────┬──────┘
   条件边     │ tool / done
   ┌──────────┴───────────┐
   ▼                      ▼
┌───────┐            ( End )
│ tools │── 固定边 ──► agent     ← 回环
└───────┘
```

`graph/` 里四条机制各对应 LangGraph 的一个概念：

- `AddNode` / `AddEdge` / `AddConditionalEdges` / `SetEntryPoint`：图定义，`Compile()` 做完整性校验。
- `Append` reducer：`messages` 累加，等价 Python 的 `add_messages`。
- `Checkpointer`：每个 super-step 存一次快照，`MemoryCheckpointer` 或 `FileCheckpointer`（JSON 落盘）。
- `Interrupt` / `Resume`：节点用 error 返回中断信号，运行时存断点；`Resume` 让**被中断的节点从头重跑**，
  人工回复放在 `State["human_feedback"]`。所以节点要能容忍重复执行。
- `WithMaxSteps`：回环收敛保护，条件写错时报错而不是死循环。

`main.go` 演两个场景：

1. 问普通问题 → `agent -> tools -> agent -> End`，3 个 super-step。
2. 问"查完资料把 draft.md 删掉" → 走到 `delete_draft` 时中断，打印断点文件；
   然后用**全新的图实例**（`FileCheckpointer` 重新打开目录，内存里没有任何状态）`Resume("yes")` 续跑完成。

## 3. demos/rag —— 检索增强生成

```
语料（内置 7 篇短文）
   │ ① 切块 SplitChunks（句子边界 + 重叠）
   ▼
 chunk  ──② 向量化 Tokens + TF-IDF──▶  归一化向量
                                          │
问题 ──同一套分词+权重──▶ 查询向量 ──③ 余弦 top-k──▶ 命中片段+分数
                                          │
                                          └─④ BuildPrompt：片段带编号塞进 prompt ─▶ 答案标出处
```

`rag/` 里四个文件对应 RAG 的四步：

- `text.go`：`Tokens` 把 ASCII 串当整词、中日韩切成 bigram（绕开分词依赖）；`SplitChunks` 按句子边界切块并保留重叠。
- `index.go`：`Build` 算 TF-IDF 并 L2 归一化；`Search` 用点积算余弦、取 top-k，并返回共现词解释“为什么命中”；`BuildPrompt` 就是 RAG 里的 *Augmented*。
- `main.go`：`answer` 代替真实模型（从命中片段里挑句子拼答案并标编号），真项目里就是把 prompt 发给模型，其余代码不动。
- 分数低于阈值 `scoreCutoff = 0.05` 就走拒答分支，不编造。内置语料里问“明天杭州天气”能直接看到这个分支。

真实语料也能跑：`go run . -docs .. -q "检查点 checkpoint"` 会索引目录里的 `.md`/`.txt`，出处就是文件名。
已知限制：分块不认识 Markdown 结构，代码围栏会被拼进片段（真实项目要按标题/代码块切）。

换成真的 embedding 只需要改一处：把 `Tokens` 换成模型调用、`map[string]float64` 换成 `[]float64`，索引与召回流程不变。

## 4. 与本项目 minimal/ 的关系

`demos/langgraph` 的 agent 节点循环，和 `minimal/engine/engine.go` 的 `executeQueryLoop` 是同一件事：

| | `executeQueryLoop`（Claude Code） | LangGraph 写法 |
|---|---|---|
| 下一步 | 模型返回 `tool_use` 就执行，没有就停 | 条件边读 State 判断 |
| 状态 | `e.history []api.Message` 单向追加 | `State` + reducer |
| 恢复 | 进程内内存态，中断即放弃 | Checkpointer 可跨进程续跑 |
| 人工批准 | 权限系统在执行工具前拦 | `Interrupt` / `Resume` |

一句话总结：**Claude Code = LangGraph 的 `create_react_agent` 那个环，只是把"图"缩成了一圈，
把"人工批准"做成了权限系统，把"检查点"换成了对话历史 + 自动压缩。**

拿"模型怎么知道它本来不知道的东西"对比 `demos/rag` 和 `minimal/tools/grep.go`：

| | `demos/rag`（RAG） | `minimal` 的 Grep（agentic 检索） |
|---|---|---|
| 谁决定检索 | 固定一步：拿 top-k | 模型自己决定搜什么、搜几轮 |
| 什么时候检索 | 提问时 | 推理途中，需要才调 |
| 代价 | 一次 embedding/索引，便宜 | 多轮工具调用，慢且花 token |
| 适合 | 静态知识问答 | 代码这种结构已知、要精确定位的语料 |

## 5. 怎么跑

```bash
# 三个 demo 各是独立 Go module，不动主模块依赖
cd demos/langchain && env -u GOROOT go test ./... && env -u GOROOT go run .
cd demos/langgraph && env -u GOROOT go test ./... && env -u GOROOT go run .
cd demos/rag       && env -u GOROOT go test ./... && env -u GOROOT go run .
```

`demos/rag` 额外两个参数：`-q "你的问题"` 只问一次，`-docs 目录` 用真文档当语料。

`env -u GOROOT` 是本机 Homebrew Go 1.25.7 与 go.mod 要求的 1.26 工具链混用的规避方法。
