"""钉住 POST /moment/generate 的鉴权、请求字段与失败出口。"""

from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings
from app.core.llm_client import LLMUnavailableError

TEST_TOKEN = "test-internal-token"
GENERATE_PATH = "/moment/generate"
VALID_BODY = {
    "name": "小回",
    "personality_desc": "温柔体贴",
    "speaking_style": "简短",
    "context": "用户：今天加班好累",
}


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    # 本机 .env 可能是 vector 档；启动预热不能连真实 Chroma。
    monkeypatch.setenv("MEMORY_BACKEND", "recent")
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def test_rejects_missing_token(client: TestClient) -> None:
    response = client.post(GENERATE_PATH, json=VALID_BODY)

    assert response.status_code == 401


def test_rejects_unknown_field(client: TestClient) -> None:
    """extra="forbid"：Go 侧字段名拼错必须 422，不能静默当成没传。"""
    response = client.post(
        GENERATE_PATH,
        json={**VALID_BODY, "personaId": 1},
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 422


def test_returns_content_only(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    async def fake_generate_moment(**kwargs: str) -> str:
        assert kwargs == {
            "context": "用户：今天加班好累",
            "name": "小回",
            "personality": "温柔体贴",
            "speaking_style": "简短",
        }
        return "今天风很轻。"

    monkeypatch.setattr("app.api.routes.generate_moment", fake_generate_moment)

    response = client.post(
        GENERATE_PATH,
        json=VALID_BODY,
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 200
    assert response.json() == {"content": "今天风很轻。"}


def test_llm_failure_returns_503(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    """非 200 是 Go 侧回退模板的触发条件，不能被吞成 200 + 空正文。"""

    async def failing_generate_moment(**kwargs: str) -> str:
        raise LLMUnavailableError("timeout")

    monkeypatch.setattr("app.api.routes.generate_moment", failing_generate_moment)

    response = client.post(
        GENERATE_PATH,
        json=VALID_BODY,
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 503
