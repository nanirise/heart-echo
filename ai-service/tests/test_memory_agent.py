"""钉住记忆提取的类型约束与 LLM 输出归一化（TECH_DESIGN §5.3）。"""

import asyncio

from app.agents.memory_agent import (
    ALLOWED_MEMORY_TYPES,
    LLMMemoryExtractor,
    RuleMemoryExtractor,
    _normalize,
)


def test_rule_extractor_only_produces_contract_types() -> None:
    result = asyncio.run(
        RuleMemoryExtractor().extract("我养了一只叫豆豆的猫，我喜欢跑步")
    )

    assert result.memories
    assert {memory.memory_type for memory in result.memories} <= ALLOWED_MEMORY_TYPES
    assert any("豆豆" in memory.content for memory in result.memories)
    # 规则档不做画像推断
    assert result.profile_updates == {}


def test_contract_never_leaks_rule_source() -> None:
    result = asyncio.run(RuleMemoryExtractor().extract("我养了一只叫豆豆的猫"))

    assert set(result.memories[0].as_contract()) == {
        "memory_type",
        "content",
        "importance_score",
    }


def test_normalize_drops_invalid_types_and_empty_content() -> None:
    result = _normalize(
        {
            "memories": [
                {"memory_type": "emotion", "content": "用户很难过", "importance_score": 0.9},
                {"memory_type": "fact", "content": "用户养了一只猫", "importance_score": 0.85},
                {"memory_type": "preference", "content": "  ", "importance_score": 0.8},
            ],
            "profile_updates": {"occupation": "程序员", "interests": ["跑步"]},
        }
    )

    assert [memory.memory_type for memory in result.memories] == ["fact"]
    assert result.profile_updates == {"occupation": "程序员", "interests": ["跑步"]}


def test_normalize_clamps_score_and_defaults_profile_updates() -> None:
    result = _normalize(
        {
            "memories": [
                {"memory_type": "event", "content": "用户最近在忙项目上线", "importance_score": 3}
            ],
            "profile_updates": "not-an-object",
        }
    )

    assert result.memories[0].importance_score == 1.0
    assert result.profile_updates == {}


def test_llm_extractor_accepts_code_fenced_json(monkeypatch) -> None:
    async def fake_complete_json(system_prompt: str, user_prompt: str) -> str:
        assert "JSON" in system_prompt
        assert user_prompt == "我最近在准备考研"
        return (
            "```json\n"
            '{"memories":[{"memory_type":"event","content":"用户最近在准备考研",'
            '"importance_score":0.8}],"profile_updates":{"goal":"考研"}}\n'
            "```"
        )

    monkeypatch.setattr("app.agents.memory_agent.complete_json", fake_complete_json)

    result = asyncio.run(LLMMemoryExtractor().extract("我最近在准备考研"))

    assert result.memories[0].content == "用户最近在准备考研"
    assert result.profile_updates == {"goal": "考研"}
