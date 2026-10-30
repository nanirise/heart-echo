"""钉住阶段一词典分类器的 8 类标签与兜底行为（TECH_DESIGN §5.1）。"""

import pytest

from app.classifiers.lexicon import EMOTION_KEYWORDS, classify


def test_has_eight_emotion_labels() -> None:
    assert set(EMOTION_KEYWORDS) == {
        "joy",
        "sadness",
        "anger",
        "fear",
        "surprise",
        "disgust",
        "neutral",
        "love",
    }


@pytest.mark.parametrize(
    ("text", "label"),
    [
        ("今天真的太开心了", "joy"),
        ("我最近很难过", "sadness"),
        ("这件事让我很生气", "anger"),
        ("我有点害怕明天", "fear"),
        ("没想到你会来", "surprise"),
        ("这个味道让我恶心", "disgust"),
        ("我很喜欢你", "love"),
    ],
)
def test_classify_returns_expected_label(text: str, label: str) -> None:
    result = classify(text)

    assert result["label"] == label
    assert 0.5 < result["score"] <= 1.0


def test_neutral_fallback_is_fixed() -> None:
    """一个词都没命中时必须是 neutral + 0.5，不能把陈述句判成情绪。"""
    assert classify("今天下午三点开会") == {"label": "neutral", "score": 0.5}


def test_higher_hit_count_raises_score() -> None:
    """命中越多证据越足；三点命中必须高于单点命中。"""
    assert classify("难过 伤心 想哭")["score"] > classify("难过")["score"]


def test_negative_emotion_wins_tie() -> None:
    """“不开心”同时命中 sadness 的“不开心”与 joy 的“开心”，并列时负向优先。"""
    assert classify("我不开心")["label"] == "sadness"
