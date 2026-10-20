"""记忆召回（TECH_DESIGN §5.3）。

MEMORY_BACKEND 决定档位：
- recent：阶段一，保持既有行为（Python 不连 PostgreSQL，不注入记忆）
- vector：阶段二，从 ChromaDB 按 user_id + persona_id 做语义检索

工厂用延迟导入：recent 档不需要加载 chromadb / onnxruntime。
"""

from __future__ import annotations

from functools import lru_cache
from typing import Protocol, TypedDict

from app.core.config import get_settings


class MemoryRecord(TypedDict):
    """一条已落 PostgreSQL 的记忆，主键由 Go 侧回填。"""

    id: int
    memory_type: str
    content: str
    importance_score: float


class MemoryRetriever(Protocol):
    """两种档位的共同接口；实现必须是同步的，调用方用线程池避免阻塞事件循环。"""

    def retrieve(
        self, user_id: int, persona_id: int, query: str, top_k: int = 5
    ) -> list[str]: ...

    def store(
        self, user_id: int, persona_id: int, memories: list[MemoryRecord]
    ) -> int: ...

    def warmup(self) -> None: ...


class RecentMemoryRetriever:
    """阶段一档位。

    Python 不连 PostgreSQL，"最近 N 条"只能在 Go 侧查；当前 ChatRequest 没有记忆字段，
    因此这里返回空，保持 MEMORY_BACKEND=recent 时的既有行为（不向 Prompt 注入记忆）。
    真正接 recent 档时需要 Go 把最近记忆随 ChatRequest 发过来。
    """

    def retrieve(
        self, user_id: int, persona_id: int, query: str, top_k: int = 5
    ) -> list[str]:
        return []

    def store(
        self, user_id: int, persona_id: int, memories: list[MemoryRecord]
    ) -> int:
        return 0

    def warmup(self) -> None:
        """recent 档没有需要预热的模型。"""
        return None


@lru_cache
def get_retriever() -> MemoryRetriever:
    """按 MEMORY_BACKEND 选实现；进程内复用，避免每个请求新建 Chroma 客户端。

    lru_cache 保证返回值是单例；vector 档内部再走 get_vector_retriever() 的模块级单例，
    启动预热与请求处理因此拿到的是同一个实例（同一个已加载的 ONNX session）。
    """
    if get_settings().memory_backend == "vector":
        from app.memory.vector import get_vector_retriever

        return get_vector_retriever()
    return RecentMemoryRetriever()
