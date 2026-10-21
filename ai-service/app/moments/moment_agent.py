"""朋友圈动态生成（TECH_DESIGN §5.5）。

输入是 Go 侧拼好的近期互动摘要，输出只有正文：标题、引号、代码围栏和
"以下是……"这类解释性引导语都在 _clean 里剥掉。动态会原样落进
ai_moments.content 并展示给用户，模型多说一句就会变成用户看到的内容。
"""

from __future__ import annotations

import logging
import re

from app.core.llm_client import LLMUnavailableError, complete_text

logger = logging.getLogger(__name__)

_SYSTEM_PROMPT = """你是一个朋友圈文案生成器，输出必须像真人随手发的动态。
只输出动态正文本身：不要解释、不要标题、不要引号、不要列表、不要 Markdown，
也不要写"朋友圈""文案"之类的说明词。"""

# 模型偶尔仍会带 ``` 围栏或"朋友圈："前缀；不剥掉就会原样进库。
_CODE_FENCE_START = re.compile(r"^```[a-zA-Z0-9_-]*\s*")
_CODE_FENCE_END = re.compile(r"\s*```$")
_LABEL_PREFIX = re.compile(r"^(?:朋友圈|动态文案|动态|文案|正文)\s*[：:]\s*")
# 只丢弃"以下是……："这类整行引导语；正文里以"好的，"开头的句子不动。
_INTRO_LINE = re.compile(
    r"^(?:好的|当然|以下是|这是|下面|朋友圈|动态|文案|正文).{0,30}[：:]\s*$"
)
_NOTE_LINE = re.compile(
    r"^(?:注[:：]|说明[:：]|（注|\(注|（这条|\(这条|这条动态|以上是)"
)

# 20-50 字是目标；超过这个长度基本是模型在解释，宁可不落库让 Go 侧回退模板。
_MAX_MOMENT_LENGTH = 200

_QUOTE_PAIRS = (('"', '"'), ("'", "'"), ("“", "”"), ("「", "」"), ("『", "』"))


def build_moment_prompt(
    context: str, name: str, personality: str, speaking_style: str
) -> str:
    """拼生成 prompt。单独成函数是为了让测试能逐字检查人设与上下文都进去了。"""
    identity = _identity_clause(name, personality, speaking_style)
    context_line = context.strip() or "（最近还没有和用户聊过天）"

    return "\n".join(
        [
            identity,
            f"你最近和用户的互动摘要如下：{context_line}",
            "请以第一人称发一条朋友圈动态（20-50字），不要像 AI，"
            "要像真人随手发的，不要用列表或解释，直接输出正文。",
        ]
    )


async def generate_moment(
    context: str, name: str, personality: str, speaking_style: str
) -> str:
    """调用 DeepSeek 生成一条朋友圈正文。

    失败（网络 / 空输出 / 清洗后为空）一律抛 LLMUnavailableError，由路由转成
    503，Go 侧据此回退模板 —— 降级兜底只保留在 Go 一处，这里不重复实现。
    """
    raw = await complete_text(
        _SYSTEM_PROMPT,
        build_moment_prompt(context, name, personality, speaking_style),
    )
    content = _clean(raw)
    if not content:
        logger.warning("moment output rejected after cleaning: %r", raw)
        raise LLMUnavailableError("moment generation returned empty content")
    return content


def _identity_clause(name: str, personality: str, speaking_style: str) -> str:
    """按非空字段拼人设句；某一格没填时不该变成"你的性格是。"。"""
    parts: list[str] = []
    if name.strip():
        parts.append(f"你叫{name.strip()}")
    if personality.strip():
        parts.append(f"你的性格是{personality.strip()}")
    if speaking_style.strip():
        parts.append(f"说话风格是{speaking_style.strip()}")
    if not parts:
        parts.append("你是一个普通朋友")
    return "，".join(parts) + "。"


def _clean(raw: str) -> str:
    """把模型输出收敛成"只有正文"；无法收敛时返回空串。"""
    text = raw.strip()
    text = _CODE_FENCE_START.sub("", text)
    text = _CODE_FENCE_END.sub("", text).strip()

    lines = [line.strip() for line in text.splitlines() if line.strip()]
    lines = [
        line
        for line in lines
        if not _INTRO_LINE.match(line) and not _NOTE_LINE.match(line)
    ]
    if not lines:
        return ""

    text = "\n".join(lines)
    text = _LABEL_PREFIX.sub("", text).strip()
    text = _strip_wrapping_quotes(text)
    if len(text) > _MAX_MOMENT_LENGTH:
        logger.warning("moment output too long: %d chars", len(text))
        return ""
    return text.strip()


def _strip_wrapping_quotes(text: str) -> str:
    if len(text) < 2:
        return text
    for left, right in _QUOTE_PAIRS:
        if text.startswith(left) and text.endswith(right):
            return text[len(left) : -len(right)].strip()
    return text
