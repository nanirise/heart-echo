"""ChromaDB 向量记忆检索（TECH_DESIGN §5.3 阶段二）。

不引入 sentence-transformers：Chroma 默认的 ONNX MiniLM（all-MiniLM-L6-v2）
比 torch 版轻得多，镜像体积和启动开销都更可控。
"""

from __future__ import annotations

import logging
from typing import Any

from app.core.config import get_settings
from app.memory import MemoryRecord

logger = logging.getLogger(__name__)


def ownership_filter(user_id: int, persona_id: int) -> dict[str, Any]:
    """检索与写入共用的归属条件。

    两个条件缺一不可：persona_id 是全局自增，别的用户同号人设会串号（AGENTS §4.3）。
    """
    return {
        "$and": [
            {"user_id": int(user_id)},
            {"persona_id": int(persona_id)},
        ]
    }


class VectorMemoryRetriever:
    """把已落 PG 的记忆写入 ChromaDB，并按语义相似度召回 Top-K。"""

    def __init__(self, collection: Any | None = None) -> None:
        # collection 参数只用于测试注入；生产路径自己建 HttpClient。
        if collection is not None:
            self._collection = collection
            return

        # 延迟导入：注入假 collection 的单测不需要装 chromadb / onnxruntime，
        # recent 档也永远不会走到这里。
        import chromadb
        from chromadb.config import Settings as ChromaSettings

        settings = get_settings()
        client = chromadb.HttpClient(
            host=settings.chroma_host,
            port=settings.chroma_port,
            settings=ChromaSettings(anonymized_telemetry=False),
        )
        # 不传 embedding_function：使用 Chroma 默认的 ONNX MiniLM。
        # upsert 与 query 必须用同一个函数，否则向量空间不一致。
        self._collection = client.get_or_create_collection(
            name=settings.chroma_collection,
            metadata={"hnsw:space": "cosine"},
        )

    def warmup(self) -> None:
        """强制加载 ONNX 模型，避免首轮对话卡在 2 秒检索超时上。

        用不存在的归属条件（user_id=0 / persona_id=0）查询：结果一定为空，
        但 query_texts 在被发出前会先经过 embedding function，模型加载就发生在那一步。
        """
        print("开始预热", flush=True)
        self._collection.query(
            query_texts=["warmup"],
            n_results=1,
            where=ownership_filter(0, 0),
        )

    def store(
        self, user_id: int, persona_id: int, memories: list[MemoryRecord]
    ) -> int:
        """按 PostgreSQL 主键 upsert，重复通知不会产生重复向量。"""
        if not memories:
            return 0

        ids = [str(memory["id"]) for memory in memories]
        documents = [memory["content"] for memory in memories]
        metadatas = [
            {
                "memory_id": int(memory["id"]),
                # 归属条件必须进 metadata，检索时才能用 where 过滤（AGENTS §4.3）。
                "user_id": int(user_id),
                "persona_id": int(persona_id),
                "memory_type": memory["memory_type"],
                "importance_score": float(memory["importance_score"]),
            }
            for memory in memories
        ]

        self._collection.upsert(ids=ids, documents=documents, metadatas=metadatas)
        logger.info("memory vector store: upserted %d memories", len(ids))
        return len(ids)

    def retrieve(
        self, user_id: int, persona_id: int, query: str, top_k: int = 5
    ) -> list[str]:
        text = query.strip()
        if not text or top_k <= 0:
            return []

        result = self._collection.query(
            query_texts=[text],
            n_results=top_k,
            where=ownership_filter(user_id, persona_id),
        )
        documents = result.get("documents") or []
        if not documents:
            return []
        # query_texts 只有一个元素，documents 是二维列表：外层是 query，内层是命中项。
        return [document for document in documents[0] if document]


# 进程级单例。Chroma 客户端与 ONNX embedding function 都只建一次：
# 每次 new 都重新加载模型，既慢又占内存，还会让启动预热失效。
_retriever: VectorMemoryRetriever | None = None


def get_vector_retriever() -> VectorMemoryRetriever:
    """返回进程内唯一的向量检索器；首次调用时才建立连接。"""
    global _retriever
    if _retriever is None:
        _retriever = VectorMemoryRetriever()
    return _retriever
