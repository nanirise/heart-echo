"""钉住朋友圈生成的 prompt 组成与"只输出正文"的清洗规则（TECH_DESIGN §5.5）。"""

import asyncio

import pytest

from app.moments import moment_agent
from app.core.llm_client import LLMUnavailableError


def test_prompt_contains_persona_and_context() -> None:
    prompt = moment_agent.build_moment_prompt(
        context="用户：今天加班好累\n小回：辛苦啦",
        name="小回",
        personality="温柔体贴",
        speaking_style="简短",
    )

    assert "你叫小回" in prompt
    assert "你的性格是温柔体贴" in prompt
    assert "说话风格是简短" in prompt
    assert "今天加班好累" in prompt
    assert "直接输出正文" in prompt


def test_prompt_degrades_when_persona_fields_empty() -> None:
    """某一格没填时不能拼出"你的性格是。"这种残句。"""
    prompt = moment_agent.build_moment_prompt("", "", "", "")

    assert "你是一个普通朋友" in prompt
    assert "你的性格是。" not in prompt
    assert "还没有和用户聊过天" in prompt


@pytest.mark.parametrize(
    ("raw", "want"),
    [
        ("今天风很轻。", "今天风很轻。"),
        ("```\n今天风很轻。\n```", "今天风很轻。"),
        ("朋友圈：今天风很轻。", "今天风很轻。"),
        ("“今天风很轻。”", "今天风很轻。"),
        ("好的，以下是为你生成的朋友圈：\n今天风很轻。", "今天风很轻。"),
        ("今天风很轻。\n（这条动态以第一人称写成）", "今天风很轻。"),
    ],
)
def test_clean_keeps_body_only(raw: str, want: str) -> None:
    assert moment_agent._clean(raw) == want


@pytest.mark.parametrize("raw", ["", "   ", "好的，以下是为你生成的朋友圈："])
def test_clean_rejects_non_content(raw: str) -> None:
    assert moment_agent._clean(raw) == ""


def test_generate_moment_returns_cleaned_text(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def fake_complete_text(system_prompt: str, user_prompt: str) -> str:
        return "```\n今天风很轻。\n```"

    monkeypatch.setattr(moment_agent, "complete_text", fake_complete_text)

    content = asyncio.run(
        moment_agent.generate_moment("用户：今天好累", "小回", "温柔", "简短")
    )

    assert content == "今天风很轻。"


def test_generate_moment_raises_when_output_is_empty(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def fake_complete_text(system_prompt: str, user_prompt: str) -> str:
        return "好的，以下是为你生成的朋友圈："

    monkeypatch.setattr(moment_agent, "complete_text", fake_complete_text)

    with pytest.raises(LLMUnavailableError):
        asyncio.run(moment_agent.generate_moment("", "小回", "", ""))
