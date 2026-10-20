"""钉住 system prompt 的记忆注入与消息顺序（TECH_DESIGN §5.2 / §5.3）。"""

from app.core.llm_client import build_messages, build_system_prompt


class FakePersona:
    name = "小回"
    personality_desc = "温柔"
    speaking_style = "简短"


class FakeTurn:
    def __init__(self, role: str, content: str) -> None:
        self.role = role
        self.content = content


def test_system_prompt_injects_memories() -> None:
    prompt = build_system_prompt(
        FakePersona(), ["用户养了一只叫豆豆的猫", "用户是程序员"]
    )

    assert "关于你正在陪伴的用户（长期记忆）：" in prompt
    assert "- 用户养了一只叫豆豆的猫" in prompt
    assert "- 用户是程序员" in prompt


def test_system_prompt_omits_memory_section_when_empty() -> None:
    """没有记忆时整段不写：空标题会让模型以为"这个人没有过去"。"""
    prompt = build_system_prompt(FakePersona())

    assert "长期记忆" not in prompt


def test_build_messages_keeps_system_first_and_history_in_order() -> None:
    messages = build_messages(
        "现在这句",
        FakePersona(),
        [FakeTurn("user", "上一句"), FakeTurn("assistant", "上一答")],
        memories=["用户养了一只叫豆豆的猫"],
    )

    assert messages[0]["role"] == "system"
    assert "用户养了一只叫豆豆的猫" in messages[0]["content"]
    assert [message["content"] for message in messages[1:]] == [
        "上一句",
        "上一答",
        "现在这句",
    ]
