"""钉住 chat-stream spec §1.2 / §1.3 / §2.2 / §2.3 的约定。

DeepSeek 被 monkeypatch 挡在门外：这层要验的是「LLM 给什么，我们就转成什么事件」，
不是「DeepSeek 答得好不好」。附带好处是跑测试不烧 API 余额、不怕网络抽风。

注意本文件**不验「是否真流式」**：TestClient 会把整个响应体缓冲完再交给断言，
所以它证明不了「字是一个个到的」。那一件事只能靠 `curl -N` 人工验（spec §4 A 第 4 步），
而且它恰恰是生死线上最容易坏、也最该人工看一眼的部分。
"""

import json
from collections.abc import AsyncIterator, Iterator

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings
from app.core.llm_client import LLMUnavailableError

TEST_TOKEN = "test-internal-token"

STREAM_PATH = "/internal/chat/stream"
VALID_BODY = {"user_id": 1, "persona_id": 1, "message": "你好"}

# 假回复切成三段 —— 段与段的边界才是要验的东西：每段必须各成一个 delta 事件，
# 不能被攒成一条（攒批会把打字机效果变成一段一段地蹦，spec §1.3）。
FAKE_PIECES = ["辛苦", "了，今天发生", "什么了吗？"]


async def fake_reply(message: str, **_kwargs: object) -> AsyncIterator[str]:
    for piece in FAKE_PIECES:
        yield piece


async def failing_reply(message: str, **_kwargs: object) -> AsyncIterator[str]:
    """先吐一段再炸 —— 对应「DeepSeek 中途断了」。已发出的 delta 收不回来。"""
    yield FAKE_PIECES[0]
    raise LLMUnavailableError("connection reset")


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    # deepseek_api_key 没有默认值，缺了 Settings() 直接抛 —— 真实服务靠这条在启动时报错，
    # 测试里补一个假值把这一层让过去。
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    monkeypatch.setattr("app.api.routes.stream_reply", fake_reply)
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

    # 一段 LLM 输出 = 一个 delta，不许攒批（spec §1.3）
    assert names == ["delta"] * len(FAKE_PIECES) + ["end"]
    # 终止事件必须是 end：done 由 Go 在落库后补发（spec §1.3），Python 发不得
    assert "done" not in names
    assert "emotion" not in names

    assert "".join(data["text"] for _, data in events[:-1]) == "".join(FAKE_PIECES)


def test_stream_emits_error_when_llm_fails(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    """spec §2.3：生成失败 → event: error + 5001，且**不发 end**。

    发 end 会让 Go 以为生成完了、对着一句半截话去落库。
    """
    monkeypatch.setattr("app.api.routes.stream_reply", failing_reply)

    response = client.post(
        STREAM_PATH, json=VALID_BODY, headers={"X-Internal-Token": TEST_TOKEN}
    )

    # 生成阶段的失败不改 HTTP 状态：流已经以 200 开头了（契约 §6）
    assert response.status_code == 200

    events = parse_blocks(response.text)
    names = [name for name, _ in events]

    assert names == ["delta", "error"]
    assert events[-1][1] == {"code": 5001, "message": "AI 回复生成失败，请稍后重试"}


def test_stream_never_uses_crlf(client: TestClient) -> None:
    """spec §1.2：块尾必须是 \\n\\n。用 \\r\\n\\r\\n 在 Windows 本地照样跑通，
    只在 Linux 容器里表现成「页面一个字都不出」。这条断言是唯一能在本地拦住它的地方。
    """
    response = client.post(
        STREAM_PATH, json=VALID_BODY, headers={"X-Internal-Token": TEST_TOKEN}
    )

    assert b"\r" not in response.content
