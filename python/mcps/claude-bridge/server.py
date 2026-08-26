import os
import subprocess
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("claude-code-bridge")

# 防 macOS GUI 应用 PATH 问题：可在 MCP 配置 env 里用 which claude 的结果设绝对路径
CLAUDE_BIN = os.environ.get("CLAUDE_BIN", "claude")

@mcp.tool()
def ask_claude_code_expert(prompt: str, target_dir: str = "", max_budget_usd: float = 0.5) -> str:
    """
    召唤 Claude Code 作为只读外部专家（可探索代码，不可修改）。
    target_dir 传绝对路径（如 /Users/wav/GOPATH/src/mp_occ）。
    需要专家跑测试时，自己先跑，把输出贴进 prompt。
    """
    workdir = os.path.abspath(os.path.expanduser(target_dir)) if target_dir else os.getcwd()
    if not os.path.isdir(workdir):
        return f"target_dir 不存在: {workdir}"

    cmd = [
        CLAUDE_BIN,
        "-p", prompt,
        "--output-format", "text",
        "--tools", "Read,Grep,Glob",        # 只读专家：能探索、不能改，无权限弹窗
        "--max-budget-usd", str(max_budget_usd),  # 成本护栏（仅 -p 模式有效）
        "--no-session-persistence",          # 专家会话不污染你的 /resume 列表
    ]

    try:
        result = subprocess.run(
            cmd,
            stdin=subprocess.DEVNULL,   # 关键修复：切断 stdin，claude 不再等 EOF
            capture_output=True,
            text=True,
            timeout=300,
            cwd=workdir,                    # 关键：进程工作目录 = 项目根
        )
        if result.returncode != 0:
            return f"Claude Code 执行出错 (exit {result.returncode}): {result.stderr[-2000:]}"
        return result.stdout or "(Claude Code 无输出)"
    except subprocess.TimeoutExpired:
        return "Claude Code 超时（300s），请缩小问题范围。"
    except FileNotFoundError:
        return "找不到 claude 命令。在 MCP 配置 env 设置 CLAUDE_BIN 为绝对路径（which claude 获取）。"

if __name__ == "__main__":
    mcp.run(transport="stdio")
