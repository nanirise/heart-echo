"""DeepSeek 流式对话客户端。

DeepSeek 的 /chat/completions 与 OpenAI 协议兼容（TECH_DESIGN §3.2），所以直接用 openai SDK
指到它的 base_url，不自己拼 SSE 解析 —— 多行 data、[DONE]、超时与重试这些边界 SDK 都处理过。
"""

import logging
from collections.abc import AsyncIterator
from functools import lru_cache

from openai import AsyncOpenAI

from app.core.config import get_settings

logger = logging.getLogger(__name__)

# Week 2 的 prompt 是一段硬编字符串：人格描述与历史消息 Week 3 才接进来（chat-stream spec §2.2）。
# 必须告诉模型 [nudge] 的含义，否则它会把系统注入的 "[nudge] ..." 当成用户真的这么打字，
# 回复里跟着出现 "[nudge]" 这种不该给用户看见的标记（TECH_DESIGN §5.4 流程 5）。
_SYSTEM_PROMPT = (
    "你是一个温柔、耐心的情感陪伴伙伴，用简短、口语化的中文回应，每次不超过三句话。"
    "如果用户消息以 [nudge] 开头，那说明它不是用户本人刚打的字，而是系统提示你主动关心一下他，"
    "请自然地把话头接起来，绝不要提到 [nudge] 这几个字。"
)


class LLMUnavailableError(Exception):
    """对话生成失败。调用方按 chat-stream spec §2.3 转成 event: error + 5001。"""


@lru_cache
def build_client() -> AsyncOpenAI:
    """进程内复用同一个客户端 —— 它内部持有连接池，每请求新建一个等于每次重开 TCP。

    max_retries=2 只覆盖「流还没开始」的失败：连接失败、超时、4xx/5xx。
    一旦开始收 chunk 就不能再重试了 —— 那几段 delta 早就发给前端了，
    重来一遍用户会看到两遍开头（spec §2.3 的已知取舍）。
    """
    settings = get_settings()
    return AsyncOpenAI(
        api_key=settings.deepseek_api_key,
        base_url=settings.deepseek_base_url,
        timeout=settings.llm_timeout_seconds,
        max_retries=2,
    )


async def stream_reply(message: str) -> AsyncIterator[str]:
    """逐段产出回复文本，每段几个字。

    空段直接跳过：delta.content 在首包（只有 role）和末包（只有 finish_reason）都是 None，
    原样吐出去会变成一堆 {"text": null}，前端拿到 undefined。
    """
    settings = get_settings()
    client = build_client()

    try:
        stream = await client.chat.completions.create(
            model=settings.deepseek_model,
            messages=[
                {"role": "system", "content": _SYSTEM_PROMPT},
                {"role": "user", "content": message},
            ],
            stream=True,
            max_tokens=settings.llm_max_tokens,
        )
        async for chunk in stream:
            if not chunk.choices:
                continue
            text = chunk.choices[0].delta.content
            if text:
                yield text
    except Exception as exc:
        # 真实原因只进日志：给前端的 5001 文案是固定的（契约 §2 一 code 一 msg），
        # 但没有这条日志，排障就只能从"AI 回复生成失败"这七个字往回猜 ——
        # 超时、Key 失效、网络不通在这句话上长得一模一样。
        # 文案用英文：中文 Windows 上 Python 写日志走 GBK、终端按 UTF-8 读，中文会是乱码。
        logger.warning("DeepSeek call failed: %s", exc)
        # SDK 的异常分了好几层（连接 / 状态码 / 超时），调用方只关心"生成失败"这一件事，
        # 在这一层统一收口，免得 routes.py 去 import openai 的异常类型。
        raise LLMUnavailableError(str(exc)) from exc
