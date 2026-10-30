"""情感分类器工厂：按 EMOTION_BACKEND 选择实现（TECH_DESIGN §5.1）。"""

from typing import Protocol

from app.classifiers.lexicon import LexiconEmotionClassifier
from app.core.config import get_settings


class EmotionClassifier(Protocol):
    """分类器的统一契约；阶段二的 ONNX 实现只需满足这一个方法。"""

    def classify(self, text: str) -> dict[str, str | float]: ...


def build_emotion_classifier() -> EmotionClassifier:
    """返回当前后端对应的分类器。

    阶段二接 onnx 时在这里加分支即可，调用方与 HTTP 契约都不变。
    onnx 档尚未落地前显式报错，而不是静默退回词典 —— 切了开关却还在用
    词典，演示时完全看不出来，验收时才暴露。
    """
    backend = get_settings().emotion_backend
    if backend == "lexicon":
        return LexiconEmotionClassifier()
    raise NotImplementedError(f"EMOTION_BACKEND={backend} 尚未实现")


def classify(text: str) -> dict[str, str | float]:
    """按当前后端分类一条文本。"""
    return build_emotion_classifier().classify(text)
