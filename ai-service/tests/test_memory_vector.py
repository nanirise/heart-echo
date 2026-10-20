"""钉住向量记忆的归属过滤、双写契约与 Prompt 注入（TECH_DESIGN §5.3）。

ChromaDB 不真起：这些用例验的是"我们发过去的过滤条件长什么样"，
而不是"Chroma 的相似度算得准不准"。真库联调需另起 chromadb 容器。
"""

import logging
import threading
import time
from collections.abc import Iterator
from types import SimpleNamespace
from typing import Any

import pytest
from fastapi.testclient import TestClient

from app.api import routes
from app.core.config import get_settings
from app.memory import RecentMemoryRetriever, get_retriever
from app.memory.vector import VectorMemoryRetriever, ownership_filter

TEST_TOKEN = "test-internal-token"


class FakeCollection:
    """记录 upsert / query 的入参；返回值形状与 chromadb Collection 一致。"""

    def __init__(self, documents: list[list[str]] | None = None) -> None:
        self.upsert_calls: list[dict[str, Any]] = []
        self.query_calls: list[dict[str, Any]] = []
        self._documents = documents or []

    def upsert(
        self, ids: list[str], documents: list[str], metadatas: list[dict[str, Any]]
    ) -> None:
        self.upsert_calls.append(
            {"ids": ids, "documents": documents, "metadatas": metadatas}
        )

    def query(
        self, query_texts: list[str], n_results: int, where: dict[str, Any]
    ) -> dict[str, Any]:
        self.query_calls.append(
            {"query_texts": query_texts, "n_results": n_results, "where": where}
        )
        return {"documents": self._documents}


class FakeRetriever:
    """给路由测试用的双档实现桩，记录调用参数。"""

    def __init__(
        self,
        memories: list[str] | None = None,
        error: Exception | None = None,
        delay: float = 0.0,
    ) -> None:
        self.memories = memories or []
        self.error = error
        self.delay = delay
        self.retrieve_calls: list[tuple[int, int, str, int]] = []
        self.store_calls: list[tuple[int, int, list[dict[str, Any]]]] = []

    def retrieve(
        self, user_id: int, persona_id: int, query: str, top_k: int = 5
    ) -> list[str]:
        self.retrieve_calls.append((user_id, persona_id, query, top_k))
        if self.delay:
            time.sleep(self.delay)
        if self.error is not None:
            raise self.error
        return self.memories

    def store(
        self, user_id: int, persona_id: int, memories: list[dict[str, Any]]
    ) -> int:
        self.store_calls.append((user_id, persona_id, memories))
        return len(memories)


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    # 钉住 Top-K：本机 .env 若改过，断言不能跟着变。
    monkeypatch.setenv("MEMORY_TOP_K", "5")
    # 启动预热不能连真实 Chroma：本机 .env 可能是 vector 档，测试必须可离线跑。
    monkeypatch.setenv("MEMORY_BACKEND", "recent")
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def auth_headers() -> dict[str, str]:
    return {"X-Internal-Token": TEST_TOKEN}


# ---------- VectorMemoryRetriever ----------


def test_ownership_filter_requires_both_ids() -> None:
    """两个条件缺一不可：只带 persona_id 会跨用户串号（AGENTS §4.3）。"""
    assert ownership_filter(7, 3) == {
        "$and": [{"user_id": 7}, {"persona_id": 3}],
    }


def test_store_uses_postgres_id_and_ownership_metadata() -> None:
    collection = FakeCollection()
    retriever = VectorMemoryRetriever(collection=collection)

    stored = retriever.store(
        7,
        3,
        [
            {
                "id": 101,
                "memory_type": "fact",
                "content": "用户养了一只叫豆豆的猫",
                "importance_score": 0.85,
            }
        ],
    )

    assert stored == 1
    call = collection.upsert_calls[0]
    # PG 主键直接当 Chroma 文档 ID：重试同一条记忆是 upsert 而不是再插一份。
    assert call["ids"] == ["101"]
    assert call["documents"] == ["用户养了一只叫豆豆的猫"]
    assert call["metadatas"] == [
        {
            "memory_id": 101,
            "user_id": 7,
            "persona_id": 3,
            "memory_type": "fact",
            "importance_score": 0.85,
        }
    ]


def test_retrieve_filters_by_user_and_persona() -> None:
    collection = FakeCollection(
        documents=[["用户养了一只叫豆豆的猫", "用户是程序员"]]
    )
    retriever = VectorMemoryRetriever(collection=collection)

    got = retriever.retrieve(7, 3, "我的猫叫什么", top_k=2)

    assert got == ["用户养了一只叫豆豆的猫", "用户是程序员"]
    assert collection.query_calls == [
        {
            "query_texts": ["我的猫叫什么"],
            "n_results": 2,
            "where": {"$and": [{"user_id": 7}, {"persona_id": 3}]},
        }
    ]


def test_retrieve_skips_chroma_for_empty_query() -> None:
    collection = FakeCollection()
    retriever = VectorMemoryRetriever(collection=collection)

    assert retriever.retrieve(7, 3, "   ", top_k=5) == []
    assert collection.query_calls == []


def test_warmup_queries_with_nonmatching_owner() -> None:
    """预热必须真的走一次 query_texts，否则 embedding function 不会被调用。"""
    collection = FakeCollection()
    retriever = VectorMemoryRetriever(collection=collection)

    retriever.warmup()

    assert collection.query_calls == [
        {
            "query_texts": ["warmup"],
            "n_results": 1,
            "where": {"$and": [{"user_id": 0}, {"persona_id": 0}]},
        }
    ]


def test_recent_backend_keeps_existing_behavior() -> None:
    retriever = RecentMemoryRetriever()

    assert retriever.retrieve(7, 3, "我的猫叫什么") == []
    assert retriever.store(7, 3, []) == 0


def test_vector_retriever_is_process_singleton(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """模块级单例：第二次调用不能再构造实例（否则模型会被重复加载）。"""
    import app.memory.vector as vector_module

    created: list[object] = []

    class CountingRetriever:
        def __init__(self) -> None:
            created.append(self)

    monkeypatch.setattr(vector_module, "_retriever", None)
    monkeypatch.setattr(vector_module, "VectorMemoryRetriever", CountingRetriever)

    first = vector_module.get_vector_retriever()
    second = vector_module.get_vector_retriever()

    assert first is second
    assert len(created) == 1


def test_factory_switches_on_memory_backend(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """MEMORY_BACKEND 是唯一开关；vector 档才导入 chromadb（延迟导入）。"""
    import app.memory.vector as vector_module

    sentinel = object()
    # 单例若已被别的用例填过，这里的 monkeypatch 就不会生效。
    monkeypatch.setattr(vector_module, "_retriever", None)
    monkeypatch.setattr(vector_module, "VectorMemoryRetriever", lambda: sentinel)
    monkeypatch.setattr(
        "app.memory.get_settings",
        lambda: SimpleNamespace(memory_backend="vector"),
    )
    get_retriever.cache_clear()
    try:
        assert get_retriever() is sentinel

        monkeypatch.setattr(
            "app.memory.get_settings",
            lambda: SimpleNamespace(memory_backend="recent"),
        )
        get_retriever.cache_clear()
        assert isinstance(get_retriever(), RecentMemoryRetriever)
    finally:
        get_retriever.cache_clear()


# ---------- POST /memory/store ----------


STORE_PAYLOAD = {
    "user_id": 7,
    "persona_id": 3,
    "memories": [
        {
            "id": 101,
            "memory_type": "fact",
            "content": "用户养了一只叫豆豆的猫",
            "importance_score": 0.85,
        }
    ],
}


def test_store_rejects_missing_token(client: TestClient) -> None:
    assert client.post("/memory/store", json=STORE_PAYLOAD).status_code == 401


def test_store_passes_owner_ids_and_memory_ids(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    fake = FakeRetriever()
    monkeypatch.setattr(routes, "get_retriever", lambda: fake)

    response = client.post(
        "/memory/store", json=STORE_PAYLOAD, headers=auth_headers()
    )

    assert response.status_code == 200
    assert response.json() == {"stored": 1}
    assert fake.store_calls == [
        (
            7,
            3,
            [
                {
                    "id": 101,
                    "memory_type": "fact",
                    "content": "用户养了一只叫豆豆的猫",
                    "importance_score": 0.85,
                }
            ],
        )
    ]


def test_store_rejects_empty_memories(client: TestClient) -> None:
    response = client.post(
        "/memory/store",
        json={**STORE_PAYLOAD, "memories": []},
        headers=auth_headers(),
    )

    assert response.status_code == 422


def test_store_rejects_emotion_as_memory_type(client: TestClient) -> None:
    """情绪是内部信号，不建向量索引（AGENTS §4.4）。"""
    payload = {
        **STORE_PAYLOAD,
        "memories": [{**STORE_PAYLOAD["memories"][0], "memory_type": "emotion"}],
    }

    response = client.post("/memory/store", json=payload, headers=auth_headers())

    assert response.status_code == 422


# ---------- 启动预热 ----------


def test_startup_warms_up_vector_retriever(
    monkeypatch: pytest.MonkeyPatch, caplog: pytest.LogCaptureFixture
) -> None:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    monkeypatch.setenv("MEMORY_BACKEND", "vector")
    get_settings.cache_clear()

    class RecordingRetriever:
        def __init__(self) -> None:
            self.warmed_up = False

        def warmup(self) -> None:
            self.warmed_up = True

    fake = RecordingRetriever()
    import app.main as main_module

    monkeypatch.setattr(main_module, "get_retriever", lambda: fake)
    try:
        with caplog.at_level(logging.INFO, logger="app.main"):
            from app.main import create_app

            with TestClient(create_app()) as client:
                assert client.get("/health").status_code == 200

        assert fake.warmed_up
        assert "ChromaDB 预热中..." in caplog.text
        assert "ChromaDB 预热完成" in caplog.text
    finally:
        get_settings.cache_clear()


def test_startup_warmup_failure_does_not_block(
    monkeypatch: pytest.MonkeyPatch, caplog: pytest.LogCaptureFixture
) -> None:
    """Chroma 没起时必须继续启动，只留一条 warning。"""
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    monkeypatch.setenv("MEMORY_BACKEND", "vector")
    get_settings.cache_clear()

    class BrokenRetriever:
        def warmup(self) -> None:
            raise RuntimeError("chroma down")

    import app.main as main_module

    monkeypatch.setattr(main_module, "get_retriever", lambda: BrokenRetriever())
    try:
        with caplog.at_level(logging.WARNING, logger="app.main"):
            from app.main import create_app

            with TestClient(create_app()) as client:
                assert client.get("/health").status_code == 200

        assert "ChromaDB 预热失败" in caplog.text
    finally:
        get_settings.cache_clear()


# ---------- 检索接入流式对话 ----------


async def fake_reply(message: str, **_kwargs: object):
    yield "好"


def test_chat_passes_retrieved_memories_to_prompt(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    captured: dict[str, object] = {}
    fake = FakeRetriever(memories=["用户养了一只叫豆豆的猫"])

    async def reply(message: str, **kwargs: object):
        captured.update(kwargs)
        yield "好"

    monkeypatch.setattr(routes, "get_retriever", lambda: fake)
    monkeypatch.setattr(routes, "stream_reply", reply)

    response = client.post(
        "/internal/chat/stream",
        json={"user_id": 7, "persona_id": 3, "message": "我的猫叫什么"},
        headers=auth_headers(),
    )

    assert response.status_code == 200
    # 两个归属 ID 必须原样传给检索器，不能只传 persona。
    assert fake.retrieve_calls == [(7, 3, "我的猫叫什么", 5)]
    assert captured["memories"] == ["用户养了一只叫豆豆的猫"]


def test_chat_skips_retrieval_for_nudge(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    captured: dict[str, object] = {}
    fake = FakeRetriever(memories=["不该被召回"])

    async def reply(message: str, **kwargs: object):
        captured.update(kwargs)
        yield "好"

    monkeypatch.setattr(routes, "get_retriever", lambda: fake)
    monkeypatch.setattr(routes, "stream_reply", reply)

    response = client.post(
        "/internal/chat/stream",
        json={"user_id": 7, "persona_id": 3, "message": "[nudge] 主动打个招呼"},
        headers=auth_headers(),
    )

    assert response.status_code == 200
    assert fake.retrieve_calls == []
    assert captured["memories"] == []


def test_chat_survives_retrieval_failure(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    """向量库挂掉不能影响主对话：降级为无记忆，照常出 delta + end。"""
    fake = FakeRetriever(error=RuntimeError("chroma down"))

    monkeypatch.setattr(routes, "get_retriever", lambda: fake)
    monkeypatch.setattr(routes, "stream_reply", fake_reply)

    response = client.post(
        "/internal/chat/stream",
        json={"user_id": 7, "persona_id": 3, "message": "你好"},
        headers=auth_headers(),
    )

    assert response.status_code == 200
    assert "event: delta" in response.text
    assert "event: end" in response.text
    assert "event: error" not in response.text


def test_chat_survives_retrieval_timeout(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Chroma 卡住时只等 2 秒：超时降级为无记忆，回复照常开始。

    用 Event 而不是 sleep：请求若没走超时分支，会一直等到 5 秒上限，
    时长断言就能把「没实现超时」和「实现了」区分开。
    """

    class BlockingRetriever:
        def __init__(self) -> None:
            self.release = threading.Event()

        def retrieve(
            self, user_id: int, persona_id: int, query: str, top_k: int = 5
        ) -> list[str]:
            self.release.wait(5.0)
            return ["迟到的记忆"]

    fake = BlockingRetriever()
    monkeypatch.setattr(routes, "get_retriever", lambda: fake)
    monkeypatch.setattr(routes, "stream_reply", fake_reply)
    monkeypatch.setattr(routes, "_RETRIEVE_TIMEOUT_SECONDS", 0.05)

    started = time.monotonic()
    response = client.post(
        "/internal/chat/stream",
        json={"user_id": 7, "persona_id": 3, "message": "你好"},
        headers=auth_headers(),
    )
    elapsed = time.monotonic() - started
    fake.release.set()

    assert elapsed < 2.0
    assert response.status_code == 200
    assert "event: delta" in response.text
    assert "event: end" in response.text
    assert "event: error" not in response.text
