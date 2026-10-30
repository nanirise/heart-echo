"""钉住 POST /emotion/analyze 的内部鉴权与字段契约（TECH_DESIGN §5.1）。"""

from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings

TEST_TOKEN = "test-internal-token"
ANALYZE_PATH = "/emotion/analyze"


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    # deepseek_api_key 没有默认值，缺了 Settings() 直接抛；本端点不调 LLM，
    # 但 create_app 仍会构造 Settings。
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    monkeypatch.setenv("EMOTION_BACKEND", "lexicon")
    # 本机 .env 可能是 vector 档；启动预热不能连真实 Chroma。
    monkeypatch.setenv("MEMORY_BACKEND", "recent")
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def post(client: TestClient, text: str):
    return client.post(
        ANALYZE_PATH,
        json={"text": text},
        headers={"X-Internal-Token": TEST_TOKEN},
    )


def test_rejects_missing_token(client: TestClient) -> None:
    response = client.post(ANALYZE_PATH, json={"text": "我很难过"})

    assert response.status_code == 401


def test_rejects_unknown_field(client: TestClient) -> None:
    """extra="forbid"：Go 侧字段名拼错必须 422，不能静默当成没传。"""
    response = client.post(
        ANALYZE_PATH,
        json={"text": "我很难过", "personaId": 1},
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 422


def test_rejects_empty_text(client: TestClient) -> None:
    assert post(client, "").status_code == 422


def test_returns_emotion_label_and_score(client: TestClient) -> None:
    response = post(client, "我最近真的很难过")

    assert response.status_code == 200
    body = response.json()
    assert body["label"] == "sadness"
    assert 0.5 < body["score"] <= 1.0
    assert set(body) == {"label", "score"}


def test_neutral_fallback(client: TestClient) -> None:
    response = post(client, "今天下午三点开会")

    assert response.status_code == 200
    assert response.json() == {"label": "neutral", "score": 0.5}
