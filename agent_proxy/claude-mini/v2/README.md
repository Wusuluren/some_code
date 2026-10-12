# v2 — Sandbox：隔离执行环境（含 v1 Filesystem）

在 v1（长期工作空间）之上叠加 **Sandbox**。v1 的 `confineToCwd` 只保护 Write/Edit，
且边界"碰巧"是进程 cwd；v2 把沙箱变成显式的一等配置。

## 机制：执行环境要与用户真实环境隔离

1. **可配置沙箱根**：`--sandbox <dir>`（`main.go`）创建目录并 `tools.SetSandboxRoot`，
   不再依赖进程 cwd。
2. **全部文件工具收口**：`confineToCwd` 的边界从 `os.Getwd()` 改为 `sandboxBase()`
   （`tools/sandbox.go` + `write.go` 3 行改动）；**Read 也补上收口**（v1 可读任意路径），
   Ls/Write/Edit/Grep 原本就走 `confineToCwd`。
3. **Bash 隔离**：工作目录固定在沙箱根，环境变量经 `ScrubEnv` 脱敏——
   名称含 KEY/TOKEN/SECRET/PASSWORD/CRED/AUTH 的变量（如 `ANTHROPIC_API_KEY`）不进 shell，
   模型执行 `env` 也拿不到凭证。
4. `sandboxBase` 做 `EvalSymlinks`，兼容 macOS `/var → /private/var`。

## 与 v1 的 diff

| 文件 | 变化 |
|---|---|
| `tools/sandbox.go` | 新增：SetSandboxRoot / sandboxBase / ScrubEnv（~55 行） |
| `tools/write.go` | confineToCwd 边界 cwd → sandboxBase（仅换 2 处取值） |
| `tools/read.go` | +5 行：读也过 confineToCwd |
| `tools/bash.go` | +5 行：cmd.Dir 固定沙箱 + Env 脱敏 |
| `main.go` | +14 行：--sandbox flag |
| `sandbox_test.go` | 新增：箱内可写/箱外读写被拒/凭证不漏进 shell/ScrubEnv 单测 |

## 运行

```bash
env -u GOROOT go build -o v2-agent ./v2
./v2-agent --sandbox ./box --print "用脚本统计 box 内文件数并把结果写到 count.txt"
# 模型在 box 里干活：读不到 ~/、拿不到 API key，产物全部落在 ./box
```
