"""阶段一情感分类器：关键词词典打分（TECH_DESIGN §5.1）。

零依赖、零延迟、结果可解释。阶段二换成 ONNX 小模型时保持同一个
``classify(text) -> {"label", "score"}`` 契约，调用方不需要改。
"""

# 8 类情绪关键词。dict 的插入顺序就是并列时的优先级：
# 负向情绪排在前面 —— 一句话里同时出现"难过"和"开心"时，按更值得
# 优先共情的那一类回复（TECH_DESIGN §5.2 要求 2）。
EMOTION_KEYWORDS: dict[str, tuple[str, ...]] = {
    "sadness": (
        "难过", "伤心", "悲伤", "痛苦", "低落", "沮丧", "失落", "委屈",
        "心碎", "想哭", "哭泣", "眼泪", "哭了", "郁闷", "压抑", "绝望",
        "孤独", "孤单", "不开心", "不高兴", "心情不好", "难受", "心累",
        "崩溃", "唉",
    ),
    "anger": (
        "生气", "愤怒", "火大", "气死", "恼火", "抓狂", "暴怒", "发火",
        "烦死", "烦躁", "讨厌", "恨", "气人", "可恶", "受不了", "无语",
        "暴躁", "不爽",
    ),
    "fear": (
        "害怕", "恐惧", "担心", "焦虑", "紧张", "不安", "恐慌", "吓死",
        "吓人", "恐怖", "畏惧", "心慌", "忐忑", "后怕", "发慌", "不敢",
    ),
    "disgust": (
        "恶心", "厌恶", "反感", "嫌弃", "恶臭", "想吐", "吐了", "下头",
        "油腻", "作呕", "鄙视", "轻蔑", "膈应", "恶寒",
    ),
    "joy": (
        "开心", "高兴", "快乐", "幸福", "喜悦", "兴奋", "激动", "满足",
        "欣慰", "愉快", "欢乐", "哈哈", "笑死", "太棒了", "太好了",
        "好耶", "好爽", "真爽", "美滋滋", "乐死",
    ),
    "love": (
        "爱你", "喜欢", "爱意", "心动", "想你", "想念", "眷恋", "依恋",
        "深爱", "亲爱的", "宝贝", "么么哒", "抱抱", "恋爱", "爱了",
    ),
    "surprise": (
        "惊讶", "震惊", "意外", "吃惊", "没想到", "出乎意料", "惊喜",
        "天哪", "卧槽", "竟然", "居然", "不可思议", "神奇", "哇塞",
        "不会吧", "真的假的",
    ),
    # neutral 没有关键词：它是"一个词都没命中"的兜底。
    # 硬塞"还好 / 一般"之类会把陈述句误判成需要回应的情绪。
    "neutral": (),
}

NEUTRAL_SCORE = 0.5
_BASE_SCORE = 0.6
_STEP_SCORE = 0.1
_MAX_SCORE = 0.95


class LexiconEmotionClassifier:
    """关键词词典分类器：命中数最多的那一类胜出，命中越多置信度越高。"""

    def classify(self, text: str) -> dict[str, str | float]:
        normalized = text.strip()
        hits = {
            label: sum(normalized.count(word) for word in words)
            for label, words in EMOTION_KEYWORDS.items()
        }

        # max 并列时返回第一个遇到的键；EMOTION_KEYWORDS 的插入顺序
        # 已经按"负向优先"排好，这里不再另写一套 tie-break。
        label = max(hits, key=lambda emotion: hits[emotion])
        if hits[label] == 0:
            return {"label": "neutral", "score": NEUTRAL_SCORE}

        # 置信度只表达"命中了多少证据"，不是模型概率。
        # 封顶 0.95：词典永远拿不准言外之意，不伪装成确定性。
        score = min(_BASE_SCORE + _STEP_SCORE * hits[label], _MAX_SCORE)
        return {"label": label, "score": round(score, 3)}


_DEFAULT_CLASSIFIER = LexiconEmotionClassifier()


def classify(text: str) -> dict[str, str | float]:
    """模块级便捷入口，与 LexiconEmotionClassifier.classify 同一契约。"""
    return _DEFAULT_CLASSIFIER.classify(text)
