# v4 — Memory + Skills：可复用经验层（含 v1..v3）

在 v3 之上叠加 **Memory** 与 **Skills**。前几版每次会话结束，模型学到的东西就归零；
v4 让经验沉淀成文件，下一轮自动回到 system prompt。

## 机制：把"教训"和"套路"从对话里挪到磁盘

统一落在工作区的 `.agent/` 目录（沙箱激活时进箱）：

1. **Memory（事实层）**：`tools/memory.go` 的 `Memory` 工具，`action=save` 追加一行
   带日期的事实到 `.agent/memory.md`，`action=list` 读回。engine 每轮把该文件
   注入 system prompt 的 `# Long-term memory` 段。
2. **Skills（流程层）**：`Skill` 工具，`action=save` 把一段可复用步骤写成
   `.agent/skills/<name>.md`，`action=use` 读回。engine 把技能名索引进 prompt，
   模型看目录→按需加载，避免把整段配方塞进上下文。
3. **安全**：skill 名禁止 `/ \` 和 `.`，防目录穿越；`AGENT_HOME` env 供测试重定向。

## 与 v3 的 diff

| 文件 | 变化 |
|---|---|
| `tools/memory.go` | 新增：Memory + Skill 两个工具，及 ReadMemory/SkillIndex 助手 |
| `tools/tools.go` | 注册 Memory、Skill（9 工具） |
| `engine/engine.go` | buildSystemPrompt 注入 memory 正文 + skill 索引 |
| `memory_test.go` | 新增：save/list 往返、技能往返、目录穿越被拒 |

## 运行

```bash
env -u GOROOT go build -o v4-agent ./v4
./v4-agent --print -v "本项目要用 env -u GOROOT 才能编译，记住这条，并写一个『构建』技能配方"
cat .agent/memory.md .agent/skills/*.md   # 经验已落盘，下个会话自动带上
```
