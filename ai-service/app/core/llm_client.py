"""DeepSeek 流式对话客户端。

DeepSeek 的 /chat/completions 与 OpenAI 协议兼容（TECH_DESIGN §3.2），所以直接用 openai SDK
指到它的 base_url，不自己拼 SSE 解析 —— 多行 data、[DONE]、超时与重试这些边界 SDK 都处理过。
"""

import logging
from collections.abc import AsyncIterator, Sequence
from functools import lru_cache
from typing import Protocol

from openai import AsyncOpenAI

from app.core.config import get_settings

logger = logging.getLogger(__name__)


class LLMUnavailableError(Exception):
    """对话生成失败。调用方按 chat-stream spec §2.3 转成 event: error + 5001。"""


class PersonaLike(Protocol):
    """人格三字段。

    用 Protocol 而不是 import api 层的模型：core 反过来依赖 api 是层次倒挂，
    而这里真的只需要这三个属性，duck typing 刚好。
    """

    name: str
    personality_desc: str
    speaking_style: str


class HistoryTurnLike(Protocol):
    """一条历史消息。"""

    role: str
    content: str


def build_system_prompt(persona: PersonaLike) -> str:
    """按 TECH_DESIGN §5.2 的模板拼 system prompt。

    只实现当前拿得到的要求。§5.2 的框架里还有"当前人格状态 / 长期记忆 / 用户情绪"三段与
    一条情绪应对要求 —— 它们分别依赖人格演化、记忆系统、情感分析，三者都还没做
    （AnalyzeEmotion 与记忆提取仍是待做）。这里**不写占位符**：写了，后续接入时就看不出
    哪段是真数据；不写，缺什么一目了然。

    ⚠️ 第 2 条（[nudge]）不能省：主动消息注入的 "[nudge] ..." 在历史里与用户手打的字长得
    一模一样，没有这句说明，模型会当成用户真的这么打字，回复里跟着出现 "[nudge]"
    （TECH_DESIGN §5.4 流程 5）。
    """
    lines = [f"你是{persona.name}。"]
    if persona.personality_desc:
        lines.append(f"性格：{persona.personality_desc}")
    if persona.speaking_style:
        lines.append(f"说话风格：{persona.speaking_style}")

    lines.append("")
    lines.append("要求：")
    lines.append("1. 以第一人称自然对话，不要暴露你在读记忆或设定，要像真人自然想起。")
    lines.append(
        "2. 若本条用户消息以 [nudge] 开头，说明这是系统在你主动找人聊天，不是用户刚发的，"
        "请结合最近对话自然地问候或开启一个话题，不要提及 [nudge] 本身。"
    )
    return "\n".join(lines)


def build_messages(
    message: str,
    persona: PersonaLike | None,
    history: Sequence[HistoryTurnLike],
) -> list[dict[str, str]]:
    """把人格、历史、当前消息拼成 messages 数组 —— 这就是模型的**全部记忆**。

    LLM 接口是无状态的，服务端不存会话：模型每一轮能看到什么，完全由这个数组决定。
    少拼一段，模型就"不知道"那一段，而且不会报错，只会答得不对。

    顺序不能变：system 必须最前（OpenAI 协议要求），历史按**时间正序**居中，
    当前消息最后。persona 为 None 时干脆不发 system message，而不是发一条空的。
    """
    messages: list[dict[str, str]] = []
    if persona is not None:
        messages.append({"role": "system", "content": build_system_prompt(persona)})
    for turn in history:
        messages.append({"role": turn.role, "content": turn.content})
    messages.append({"role": "user", "content": message})
    return messages


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


async def stream_reply(
    message: str,
    persona: PersonaLike | None = None,
    history: Sequence[HistoryTurnLike] = (),
) -> AsyncIterator[str]:
    """逐段产出回复文本，每段几个字。

    空段直接跳过：delta.content 在首包（只有 role）和末包（只有 finish_reason）都是 None，
    原样吐出去会变成一堆 {"text": null}，前端拿到 undefined。
    """
    settings = get_settings()
    client = build_client()

    try:
        stream = await client.chat.completions.create(
            model=settings.deepseek_model,
            # 上下文整个由 build_messages 组装后送上来。
            #
            # ⚠️ 这里**不能再硬编一段 system prompt**：它会把 persona 那一份盖掉，
            # 于是用户在界面上填的说话风格完全不生效，而现象只是"说话不太像"，
            # 没有任何报错 —— 2026-10-05 就是这么踩的（当时写死了"温柔陪伴、不超过三句"）。
            messages=build_messages(message, persona, history),
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
