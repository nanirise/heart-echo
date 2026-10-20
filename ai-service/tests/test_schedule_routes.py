"""钉住 POST /schedule/parse 的三个出口与内部鉴权（TECH_DESIGN §5.7）。"""

from collections.abc import Iterator
from datetime import datetime, timedelta

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings

TEST_TOKEN = "test-internal-token"
PARSE_PATH = "/schedule/parse"


@pytest.fixture
def client(monkeypatch: pytest.MonkeyPatch) -> Iterator[TestClient]:
    monkeypatch.setenv("AI_SERVICE_TOKEN", TEST_TOKEN)
    # deepseek_api_key 没有默认值，缺了 Settings() 直接抛；解析接口不调 LLM，
    # 但 create_app 仍会构造 Settings。
    monkeypatch.setenv("DEEPSEEK_API_KEY", "test-key")
    get_settings.cache_clear()

    from app.main import create_app

    with TestClient(create_app()) as test_client:
        yield test_client

    get_settings.cache_clear()


def post(client: TestClient, message: str):
    return client.post(
        PARSE_PATH,
        json={"message": message},
        headers={"X-Internal-Token": TEST_TOKEN},
    )


def test_rejects_missing_token(client: TestClient) -> None:
    response = client.post(PARSE_PATH, json={"message": "明天下午三点提醒我开会"})

    assert response.status_code == 401


def test_rejects_unknown_field(client: TestClient) -> None:
    """extra="forbid"：Go 侧字段名拼错必须 422，不能静默当成没传。"""
    response = client.post(
        PARSE_PATH,
        json={"message": "明天下午三点提醒我开会", "personaId": 1},
        headers={"X-Internal-Token": TEST_TOKEN},
    )

    assert response.status_code == 422


def test_statement_is_not_a_reminder(client: TestClient) -> None:
    """陈述句不是委托：不建日程，也不回问。"""
    response = post(client, "我明天要开会")

    assert response.status_code == 200
    assert response.json() == {"has_intent": False}


def test_unparseable_request_asks_clarify(client: TestClient) -> None:
    """有提醒意图但解析不出时间：Go 侧据此让 AI 回问，不能静默丢弃。"""
    response = post(client, "过阵子提醒我")

    assert response.status_code == 200
    assert response.json() == {"has_intent": True, "need_clarify": True}


def test_success_returns_content_and_remind_at(client: TestClient) -> None:
    response = post(client, "明天下午三点提醒我开会")

    assert response.status_code == 200
    body = response.json()

    assert body["has_intent"] is True
    assert body["need_clarify"] is False
    assert body["content"] == "开会"

    remind_at = datetime.fromisoformat(body["remind_at"])
    # 绝对时间必须带时区（契约 §10 的 remindAt 要求 RFC3339 带时区），且按 +08:00 计算
    assert remind_at.utcoffset() == timedelta(hours=8)
    assert remind_at.hour == 15
