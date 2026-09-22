import os
import asyncio
import logging
from typing import Literal
from mcp.server.fastmcp import FastMCP
from openai import AsyncOpenAI
import httpx

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("moa-server")

# 初始化 OpenAI 兼容客户端 
# 强烈建议使用 OpenRouter API，这样只需一个 Key 就能调用文章提到的所有模型 (DeepSeek/Gemini/Claude/GPT)
API_KEY = os.environ.get("OPENROUTER_API_KEY") or os.environ.get("OPENAI_API_KEY")
BASE_URL = os.environ.get("OPENAI_BASE_URL", "https://openrouter.ai/api/v1")

if not API_KEY:
    logger.warning("Warning: OPENROUTER_API_KEY or OPENAI_API_KEY is not set in env.")

client = AsyncOpenAI(
    api_key=API_KEY,
    base_url=BASE_URL,
    http_client=httpx.AsyncClient(
        headers={
            "HTTP-Referer": "https://your-website.com", # OpenRouter 建议的 referer
            "X-Title": "MoA MCP Server",               # OpenRouter 建议的 title
        },
        timeout=180.0 # MoA 需要较长的超时时间
    )
)

mcp = FastMCP("moa-server")

# 预设配置 (可根据你拥有的 API 权限自行修改模型 ID)
PRESETS = {
    "default": {
        "reference_models": ["deepseek-v4-flash", "glm-5.3-flash"],
        "aggregator_model": "glm-5.3-flash"
    }
}

async def call_model(model: str, system_prompt: str, user_prompt: str) -> str:
    """底层 LLM 调用函数，自带容错处理"""
    try:
        response = await client.chat.completions.create(
            model=model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            temperature=0.7,
        )
        return response.choices[0].message.content.strip()
    except Exception as e:
        logger.error(f"Error calling model {model}: {e}")
        return f"[Error calling {model}: {str(e)}]"

@mcp.tool()
async def moa_consult(
    question: str, 
    context: str, 
    preset: Literal["default", "quality", "cheap"] = "default"
) -> str:
    """
    调用 Mixture of Agents (MoA) 进行关键决策、架构设计或疑难 bug 分析。
    并行咨询多个参考模型，并由聚合模型综合出最终方案。
    """
    if preset not in PRESETS:
        preset = "default"
    
    cfg = PRESETS[preset]
    ref_models = cfg["reference_models"]
    agg_model = cfg["aggregator_model"]

    # 1. 参考阶段 (Reference Phase)
    ref_system_prompt = (
        "[MoA-Reference] 你是独立的参考模型，禁止调用任何工具。仅基于下方提供的上下文进行深度推理。\n"
        "请输出：方案草案、关键决策、潜在风险、验证方式。\n"
        "格式要求：行动优先、编号步骤、无客套话、直接输出结论。"
    )
    ref_user_prompt = f"[上下文]\n{context}\n\n[核心问题]\n{question}"

    # 并发调用所有参考模型 (asyncio.gather 是保证墙钟时间的关键)
    ref_tasks = [call_model(model, ref_system_prompt, ref_user_prompt) for model in ref_models]
    ref_results = await asyncio.gather(*ref_tasks)

    # 2. 聚合阶段 (Aggregation Phase)
    drafts_text = "\n\n".join([
        f"--- 草案 {i+1} (模型: {model}) ---\n{result}" 
        for i, (model, result) in enumerate(zip(ref_models, ref_results))
    ])
    
    agg_system_prompt = (
        "[MoA-Aggregator] 你是聚合器。下面是多份独立的参考模型草案。\n"
        "你的任务是综合这些草案，得出一个最终的高质量方案：\n"
        "1. 共识点直接采纳。\n"
        "2. 冲突点逐条裁决，并给出明确的理由。\n"
        "3. 补充所有草案可能遗漏的盲点。\n"
        "4. 输出唯一终稿，禁止提及“模型A说”或“草案1”。\n"
        "5. 结尾单独留一行，总结“残余风险”。\n"
        "格式要求：行动优先、结构化输出、无客套话。"
    )
    agg_user_prompt = f"[参考草案]\n{drafts_text}\n\n[核心问题]\n{question}"

    final_result = await call_model(agg_model, agg_system_prompt, agg_user_prompt)

    # 3. 红队挑刺阶段 (Red Team Adversarial Phase) - 新增！
    red_team_prompt = (
        "[Red Team] 你是一个无情的架构审计员。你的唯一任务是挑出上述终稿的致命缺陷。\n"
        "规则：\n"
        "1. 只挑毛病，不给替代方案，禁止说好话。\n"
        "2. 每个缺陷必须提供【证据】：代码类给 repro(复现步骤)，设计类给 argument(逻辑链)。\n"
        "3. 最多挑 3 个最致命的缺陷。"
    )
    red_team_result = await call_model("deepseek-v4-flash", red_team_prompt, final_result)

    # 构建输出 (包含终稿和审计附录)
    output = f"=== MoA 聚合终稿 (Preset: {preset}) ===\n\n{final_result}\n\n"
    output += f"=== 🚨 红队审计意见 (Red Team Review) ===\n{red_team_result}\n\n"
    output += f"=== 附录：参考模型原始草案 (仅供审计) ===\n{drafts_text}"

    return output

if __name__ == "__main__":
    # 以 stdio 模式启动 MCP Server，供 Trae/Cursor 调用
    mcp.run(transport="stdio")
