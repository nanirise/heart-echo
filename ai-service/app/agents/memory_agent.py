"""记忆提取（TECH_DESIGN §5.3）。

规则档与 LLM 档输出同一份契约，Go 侧不需要知道当前用的是哪一种：

    {"memories": [{"memory_type", "content", "importance_score"}],
     "profile_updates": {...}}

记忆类型只有 fact / preference / event。情绪是内部信号，不进长期记忆（AGENTS §4.4）。
"""

from __future__ import annotations

import json
import logging
import re
from dataclasses import dataclass, field
from typing import Any, Protocol

from app.core.config import get_settings
from app.core.llm_client import complete_json

logger = logging.getLogger(__name__)

ALLOWED_MEMORY_TYPES = frozenset({"fact", "preference", "event"})

# 句式 → (抽取槽位, 记忆类型)。与 TECH_DESIGN §5.3 的阶段一实现一致。
_FACT_PATTERNS: list[tuple[str, str, str]] = [
    (r"我(?:的名字)?叫([^\s，。！？]{1,10})", "user_name", "fact"),
    (r"我(?:养了|有一只)([^\s，。！？]{1,15})", "pet", "fact"),
    (r"我(?:喜欢|爱|最喜欢)([^\s，。！？]{1,20})", "preference", "preference"),
    (r"我(?:是|在)([^\s，。！？]{1,15})(?:工作|上班)", "occupation", "fact"),
    (r"我(?:住在|家在)([^\s，。！？]{1,15})", "location", "fact"),
]

_SYSTEM_PROMPT = """你是一个记忆提取器，负责从用户的一句话里提取值得长期记住的信息。
只输出一个 JSON 对象，不要输出解释、Markdown 或代码围栏。

输出格式：
{"memories":[{"memory_type":"fact","content":"用户养了一只叫豆豆的猫","importance_score":0.85}],"profile_updates":{"occupation":"程序员","interests":["跑步","猫"]}}

规则：
1. memory_type 只能是 fact、preference、event 三者之一。情绪不是记忆，绝不提取。
2. content 用第三人称"用户……"陈述，一句话，不要编造用户没说的信息。
3. importance_score 是 0 到 1 的数字，越长期、越具体越高。
4. profile_updates 只放可稳定复用的画像键（如 occupation、interests、location、pet）；没有就给 {}。
5. 没有值得记住的信息时，memories 给 []，profile_updates 给 {}。"""


@dataclass(frozen=True)
class MemoryCandidate:
    """一条候选记忆。归属字段不在这里：user_id / persona_id 只能由 Go 侧补。"""

    memory_type: str
    content: str
    importance_score: float
    # 规则档用来标注命中的句式；它不属于跨服务契约，不会进入 HTTP 响应。
    source: str = ""

    def as_contract(self) -> dict[str, object]:
        """只输出契约允许的三个字段，source 等内部信息一律不下发。"""
        return {
            "memory_type": self.memory_type,
            "content": self.content,
            "importance_score": self.importance_score,
        }


@dataclass(frozen=True)
class ExtractionResult:
    """一次提取的全部产物；memories 与 profile_updates 一起交给 Go 侧落库。"""

    memories: list[MemoryCandidate]
    profile_updates: dict[str, Any] = field(default_factory=dict)


class MemoryExtractor(Protocol):
    """规则档与 LLM 档的共同接口，由 MEMORY_EXTRACT_BACKEND 选择。"""

    async def extract(self, user_message: str) -> ExtractionResult: ...


class RuleMemoryExtractor:
    """基于句式的规则提取：零成本、零延迟，覆盖演示所需的显式事实。"""

    async def extract(self, user_message: str) -> ExtractionResult:
        text = user_message.strip()
        memories: list[MemoryCandidate] = []
        seen: set[str] = set()

        for pattern, kind, memory_type in _FACT_PATTERNS:
            match = re.search(pattern, text)
            if match is None:
                continue
            content = match.group(0)
            if content in seen:
                continue
            seen.add(content)
            memories.append(
                MemoryCandidate(
                    memory_type=memory_type,
                    content=content,
                    importance_score=0.8,
                    source=kind,
                )
            )

        # 规则档不做画像推断：只有 LLM 能从"我最近在准备考研"里归纳出画像键。
        return ExtractionResult(memories=memories)


class LLMMemoryExtractor:
    """LLM 结构化提取：能覆盖"最近老加班到十点"这类隐式表达（TECH_DESIGN §5.3）。"""

    async def extract(self, user_message: str) -> ExtractionResult:
        raw = await complete_json(_SYSTEM_PROMPT, user_message.strip())
        try:
            payload = _decode_json(raw)
        except (json.JSONDecodeError, ValueError) as exc:
            # 整次提取作废，由 Go 侧记 warn；半截 JSON 不能猜成记忆落库。
            logger.warning("memory extraction returned invalid JSON: %s", exc)
            raise
        return _normalize(payload)


def build_memory_extractor() -> MemoryExtractor:
    """按 MEMORY_EXTRACT_BACKEND 选实现；默认 rule，演示时切 llm。"""
    if get_settings().memory_extract_backend == "llm":
        return LLMMemoryExtractor()
    return RuleMemoryExtractor()


def _decode_json(raw: str) -> dict[str, Any]:
    """解析模型输出。

    即使要求了 json_object，部分兼容端点仍会带 ```json 围栏，所以先剥围栏再解析。
    解析失败直接抛给调用方：宁可整次提取作废，也不要把半截 JSON 猜成记忆落库。
    """
    text = raw.strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text)
        text = re.sub(r"\s*```$", "", text)

    payload = json.loads(text)
    if not isinstance(payload, dict):
        raise ValueError("memory extraction: top-level JSON must be an object")
    return payload


def _normalize(payload: dict[str, Any]) -> ExtractionResult:
    """把模型输出收敛成契约结构；非法记忆逐条丢弃并记日志。"""
    memories: list[MemoryCandidate] = []
    raw_memories = payload.get("memories")
    if isinstance(raw_memories, list):
        for item in raw_memories:
            candidate = _normalize_memory(item)
            if candidate is not None:
                memories.append(candidate)

    profile_updates = payload.get("profile_updates")
    if not isinstance(profile_updates, dict):
        profile_updates = {}

    return ExtractionResult(memories=memories, profile_updates=profile_updates)


def _normalize_memory(item: Any) -> MemoryCandidate | None:
    if not isinstance(item, dict):
        logger.warning("memory extraction: drop non-object memory item")
        return None

    memory_type = item.get("memory_type")
    if not isinstance(memory_type, str) or memory_type not in ALLOWED_MEMORY_TYPES:
        # 非法类型必须在落库前丢掉，不能原样写进 user_memory（user-memory spec §1.2）。
        logger.warning("memory extraction: drop invalid memory_type %r", memory_type)
        return None

    content = item.get("content")
    if not isinstance(content, str) or not content.strip():
        logger.warning("memory extraction: drop empty content")
        return None

    score = item.get("importance_score")
    if isinstance(score, bool) or not isinstance(score, (int, float)):
        logger.warning("memory extraction: drop invalid importance_score %r", score)
        return None
    score = max(0.0, min(1.0, float(score)))

    return MemoryCandidate(
        memory_type=memory_type,
        content=content.strip(),
        importance_score=score,
    )
