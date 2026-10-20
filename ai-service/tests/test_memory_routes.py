"""钉住 POST /memory/extract 的鉴权、契约字段与阶段切换（TECH_DESIGN §5.3）。"""

from collections.abc import Iterator

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings

TEST_TOKEN = "test-internal-token"
EXTRACT_PATH = "/memory/extract"


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    monkeypatch.setenv("MEMORY_EXTRACT_BACKEND", "rule")
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def post(client: TestClient, message: str):
    return client.post(
        EXTRACT_PATH,
        json={"message": message},
        headers={"X-Internal-Token": TEST_TOKEN},
    )


def test_rejects_missing_token(client: TestClient) -> None:
    response = client.post(EXTRACT_PATH, json={"message": "我叫小明"})

    assert response.status_code == 401


def test_rejects_unknown_field(client: TestClient) -> None:
    """extra="forbid"：Go 侧字段名拼错必须 422。"""
    response = client.post(
        EXTRACT_PATH,
        json={"message": "我叫小明", "user_id": 1},
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 422


def test_rule_backend_returns_contract_fields_only(client: TestClient) -> None:
    response = post(client, "我养了一只叫豆豆的猫")

    assert response.status_code == 200
    payload = response.json()
    assert payload["profile_updates"] == {}
    assert payload["memories"]
    assert set(payload["memories"][0]) == {
        "memory_type",
        "content",
        "importance_score",
    }
    assert payload["memories"][0]["memory_type"] == "fact"


def test_llm_backend_uses_llm_extractor(
    client: TestClient, monkeypatch: pytest.MonkeyPatch
) -> None:
    async def fake_complete_json(system_prompt: str, user_prompt: str) -> str:
        return (
            '{"memories":[{"memory_type":"fact","content":"用户是程序员",'
            '"importance_score":0.9}],"profile_updates":{"occupation":"程序员"}}'
        )

    monkeypatch.setenv("MEMORY_EXTRACT_BACKEND", "llm")
    monkeypatch.setattr("app.agents.memory_agent.complete_json", fake_complete_json)
    get_settings.cache_clear()

    response = post(client, "我是程序员")

    assert response.status_code == 200
    payload = response.json()
    assert payload["memories"] == [
        {"memory_type": "fact", "content": "用户是程序员", "importance_score": 0.9}
    ]
    assert payload["profile_updates"] == {"occupation": "程序员"}
