"""钉住 chat-stream spec §1.2 / §1.3 / §2.2 的三条硬约定。

注意本文件**不验「是否真流式」**：TestClient 会把整个响应体缓冲完再交给断言，
所以它证明不了「字是一个个到的」。那一件事只能靠 `curl -N` 人工验（spec §4 A 第 4 步），
而且它恰恰是生死线上最容易坏、也最该人工看一眼的部分。
"""

import json
from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient

from app.api.routes import _FAKE_REPLY
from app.core.config import get_settings

TEST_TOKEN = "test-internal-token"

STREAM_PATH = "/internal/chat/stream"
VALID_BODY = {"user_id": 1, "persona_id": 1, "message": "你好"}


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    # get_settings 是 lru_cache 单例，不 clear 会沿用上一次测试的配置
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def parse_blocks(body: str) -> list[tuple[str, dict]]:
    """按契约 §6 的切块方式还原事件 —— 与前端 consumeStream 同一种切法。"""
    events: list[tuple[str, dict]] = []

    for block in body.split("\n\n"):
        if not block:
            continue
        lines = block.split("\n")
        events.append(
            (lines[0].removeprefix("event: "), json.loads(lines[1].removeprefix("data: ")))
        )

    return events


def test_health_does_not_require_token(client: TestClient) -> None:
    """契约 §11：Go 的健康检查不带 X-Internal-Token，加了鉴权会让它恒报 down。"""
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_stream_rejects_missing_token(client: TestClient) -> None:
    assert client.post(STREAM_PATH, json=VALID_BODY).status_code == 401


def test_stream_rejects_wrong_token(client: TestClient) -> None:
    response = client.post(
        STREAM_PATH, json=VALID_BODY, headers={"X-Internal-Token": "not-the-token"}
    )

    assert response.status_code == 401


def test_stream_rejects_empty_message(client: TestClient) -> None:
    """message 的 min_length=1 生效 —— 空消息不该流一路空 delta 再 end。"""
    response = client.post(
        STREAM_PATH,
        json={**VALID_BODY, "message": ""},
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 422


def test_stream_emits_delta_chunks_then_end(client: TestClient) -> None:
    response = client.post(
        STREAM_PATH, json=VALID_BODY, headers={"X-Internal-Token": TEST_TOKEN}
    )

    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/event-stream")
    assert response.headers["x-accel-buffering"] == "no"

    events = parse_blocks(response.text)
    names = [name for name, _ in events]

    assert names[:-1] == ["delta"] * (len(names) - 1)
    # 终止事件必须是 end：done 由 Go 在落库后补发（spec §1.3），Python 发不得
    assert names[-1] == "end"
    assert "done" not in names
    assert "emotion" not in names

    assert "".join(data["text"] for _, data in events[:-1]) == _FAKE_REPLY


def test_stream_never_uses_crlf(client: TestClient) -> None:
    """spec §1.2：块尾必须是 \\n\\n。用 \\r\\n\\r\\n 在 Windows 本地照样跑通，
    只在 Linux 容器里表现成「页面一个字都不出」。这条断言是唯一能在本地拦住它的地方。
    """
    response = client.post(
        STREAM_PATH, json=VALID_BODY, headers={"X-Internal-Token": TEST_TOKEN}
    )

    assert b"\r" not in response.content
