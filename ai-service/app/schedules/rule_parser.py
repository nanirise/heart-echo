"""日程时间解析的规则档实现（TECH_DESIGN §5.7）。

只做解析，不落库、不判断归属：Go 侧拿到 {content, remind_at} 后自己写 schedules 表。
失败返回 None，由调用方决定是否让 AI 回问；这里不做任何静默兜底。

时间统一按 +08:00 计算。不依赖进程本地时区：部署容器的 TZ 通常是 UTC，
用 datetime.now() 会把"明天下午三点"算错 8 小时。
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

CST = timezone(timedelta(hours=8))

# 只有明确的委托才算提醒意图；"我明天要开会"是陈述句，不能建日程。
# 长词必须排在短词前面，否则 "提醒我一下" 会先被 "提醒我" 命中，残留 "一下"。
_INTENT_RE = re.compile(
    r"记得提醒我|提醒我一下|提醒一下我|提醒下我|提醒我|提醒一下|"
    r"叫我一下|叫我一声|叫我|别让我忘(?:了|记)|别忘了"
)

# "不用提醒我 / 别叫我" 是否定句，不能算委托。
_NEGATED_INTENT_RE = re.compile(
    r"(?:不用|不要|无需|不必|别)\s*(?:再)?\s*(?:提醒我|叫我)"
)

_RELATIVE_RE = re.compile(
    r"(?P<num>\d+|[零一二两三四五六七八九十]+)\s*"
    r"(?P<unit>分钟|分|个?小时|钟头)\s*(?:之后|以后|后)"
)
_HALF_HOUR_RE = re.compile(r"(?:半|半个)\s*个?\s*小时\s*(?:之后|以后|后)")

# 大后天必须排在 后天 前面，否则正则会从 "大" 之后开始匹配到 "后天"。
_DAY_RE = re.compile(r"(?P<day>大后天|后天|明天|今天)")
_DAY_OFFSETS = {"今天": 0, "明天": 1, "后天": 2, "大后天": 3}

_PERIOD_RE = re.compile(
    r"(?P<period>凌晨|早上|早晨|上午|中午|下午|傍晚|晚上|夜里|夜晚)"
)

_CLOCK_RE = re.compile(r"(?P<hour>\d{1,2})\s*[:：]\s*(?P<minute>\d{1,2})")
_HOUR_RE = re.compile(
    r"(?P<hour>\d{1,2}|[零一二两三四五六七八九十]{1,3})\s*点"
    r"(?:\s*(?:(?P<half>半)|(?P<quarter>一刻|三刻)|"
    r"(?P<minute>\d{1,2}|[零一二两三四五六七八九十]{1,3})\s*分?))?"
)

_WEEKDAY_RE = re.compile(
    r"(?P<prefix>下下|下|本|这)?\s*(?:周|星期|礼拜)\s*(?P<weekday>[一二三四五六日天])"
)
_WEEKDAY_MAP = {"一": 0, "二": 1, "三": 2, "四": 3, "五": 4, "六": 5, "日": 6, "天": 6}

_MONTH_DAY_RE = re.compile(
    r"(?P<month>\d{1,2}|[一二三四五六七八九十]{1,2})\s*月\s*"
    r"(?P<day>\d{1,2}|[一二三四五六七八九十]{1,3})\s*[日号]"
)

_CN_DIGITS = {
    "零": 0, "一": 1, "二": 2, "两": 2, "三": 3, "四": 4,
    "五": 5, "六": 6, "七": 7, "八": 8, "九": 9,
}


@dataclass(frozen=True)
class ParsedSchedule:
    """一次成功解析的结果；content 是用户原话里的事件描述。"""

    content: str
    remind_at: datetime


def has_reminder_intent(text: str) -> bool:
    """这句话是不是在委托 AI 提醒（而不是仅仅提到了一个时间）。"""
    if _NEGATED_INTENT_RE.search(text):
        return False
    return _INTENT_RE.search(text) is not None


def parse_schedule(text: str, now: datetime | None = None) -> ParsedSchedule | None:
    """把用户原话解析成事件描述 + 绝对时间；解析不出返回 None。

    now 仅用于测试注入；默认取 +08:00 的当前时间。调用方应先判断
    has_reminder_intent：本函数只负责"有没有时间"，不负责"是不是委托"。
    """
    text = text.strip()
    if not text:
        return None

    if now is None:
        now = datetime.now(CST)
    elif now.tzinfo is None:
        now = now.replace(tzinfo=CST)
    else:
        now = now.astimezone(CST)

    # 优先级：相对时间最具体；其次今天/明天/后天（最常见的委托）；星期与月日放最后——
    # 它们很容易出现在事件描述里（"讨论周五的方案"），不能让它们抢走明确的日词。
    for resolver in (_resolve_relative, _resolve_day, _resolve_weekday, _resolve_month_day):
        result = resolver(text, now)
        if result is None:
            continue
        remind_at, time_spans = result
        content = _extract_content(text, time_spans)
        if not content:
            # 只有时间没有事件（"明天三点提醒我"）：交给调用方回问"提醒你什么"。
            return None
        return ParsedSchedule(content=content, remind_at=remind_at)
    return None


def _to_number(token: str | None) -> int | None:
    """把阿拉伯数字或 0-99 的中文数字转成 int；无法识别返回 None。"""
    if token is None:
        return None
    if token.isdigit():
        return int(token)
    if token.startswith("零") and len(token) == 2:
        return _CN_DIGITS.get(token[1])
    if "十" in token:
        left, _, right = token.partition("十")
        if left and left not in _CN_DIGITS:
            return None
        if right and right not in _CN_DIGITS:
            return None
        tens = _CN_DIGITS[left] if left else 1
        ones = _CN_DIGITS[right] if right else 0
        return tens * 10 + ones
    if len(token) == 1:
        return _CN_DIGITS.get(token)
    return None


def _period_before(text: str, position: int) -> re.Match[str] | None:
    """取紧贴小时前的时段词。

    只认与小时之间只隔空白的那个：如果 "晚上" 离时间很远（"明天三点提醒我晚上吃药"），
    它属于事件描述，不能拿来改小时，也不能从 content 里删掉。
    """
    candidate = None
    for match in _PERIOD_RE.finditer(text):
        if match.end() > position:
            break
        if not text[match.end():position].strip():
            candidate = match
    return candidate


def _apply_period(hour: int, period: str) -> int:
    """按时段把 12 小时制的小时改成 24 小时制。"""
    if period == "凌晨":
        return 0 if hour == 12 else hour
    if period in ("中午", "下午", "傍晚", "晚上", "夜里", "夜晚"):
        return hour + 12 if hour < 12 else hour
    # 早上 / 早晨 / 上午：不变；上午12点按 12:00 处理。
    return hour


def _clock_parts(text: str) -> tuple[int, int, list[tuple[int, int]]] | None:
    """解析显式钟点，返回 (hour, minute, 需要从正文里删掉的 span)。

    没有显式钟点时返回 None——今天/明天/后天不能默认一个时间，
    否则 "明天提醒我开会" 会静默变成 "明天 09:00"，属于替用户做决定。
    """
    match = _CLOCK_RE.search(text)
    if match is not None:
        hour = int(match.group("hour"))
        minute = int(match.group("minute"))
        if hour > 23 or minute > 59:
            return None
        spans = [match.span()]
        period = _period_before(text, match.start())
        if period is not None:
            hour = _apply_period(hour, period.group("period"))
            spans.append(period.span())
        return hour, minute, spans

    match = _HOUR_RE.search(text)
    if match is None:
        return None

    hour = _to_number(match.group("hour"))
    if hour is None or hour > 23:
        return None

    if match.group("half") is not None:
        minute = 30
    elif match.group("quarter") is not None:
        minute = 15 if match.group("quarter") == "一刻" else 45
    elif match.group("minute") is not None:
        minute = _to_number(match.group("minute"))
        if minute is None or minute > 59:
            return None
    else:
        minute = 0

    spans = [match.span()]
    period = _period_before(text, match.start())
    if period is not None:
        hour = _apply_period(hour, period.group("period"))
        spans.append(period.span())
    return hour, minute, spans


def _resolve_relative(
    text: str, now: datetime,
) -> tuple[datetime, list[tuple[int, int]]] | None:
    """X 分钟后 / X 小时后 / 半小时后。"""
    half = _HALF_HOUR_RE.search(text)
    if half is not None:
        return now + timedelta(minutes=30), [half.span()]

    match = _RELATIVE_RE.search(text)
    if match is None:
        return None
    amount = _to_number(match.group("num"))
    if amount is None:
        return None

    unit = match.group("unit")
    if unit.endswith("小时") or unit == "钟头":
        return now + timedelta(hours=amount), [match.span()]
    return now + timedelta(minutes=amount), [match.span()]


def _resolve_weekday(
    text: str, now: datetime,
) -> tuple[datetime, list[tuple[int, int]]] | None:
    """本周五 / 下周一 / 周五；没写钟点时按设计默认 09:00。"""
    match = _WEEKDAY_RE.search(text)
    if match is None:
        return None

    target = _WEEKDAY_MAP[match.group("weekday")]
    monday = now.date() - timedelta(days=now.weekday())
    prefix = match.group("prefix")

    if prefix == "下下":
        target_date = monday + timedelta(days=14 + target)
    elif prefix == "下":
        target_date = monday + timedelta(days=7 + target)
    else:
        # 本周 / 这周 / 不带前缀：本周还没到就用本周的，已经过了就顺延到下周同一天。
        target_date = monday + timedelta(days=target)
        if target_date < now.date():
            target_date += timedelta(days=7)

    clock = _clock_parts(text)
    if clock is None:
        hour, minute, spans = 9, 0, []
    else:
        hour, minute, spans = clock

    remind_at = datetime(
        target_date.year, target_date.month, target_date.day,
        hour, minute, tzinfo=CST,
    )
    return remind_at, [match.span(), *spans]


def _resolve_month_day(
    text: str, now: datetime,
) -> tuple[datetime, list[tuple[int, int]]] | None:
    """10月25号 + 钟点；按设计取当年该日期（必须显式给时间）。"""
    match = _MONTH_DAY_RE.search(text)
    if match is None:
        return None

    month = _to_number(match.group("month"))
    day = _to_number(match.group("day"))
    if month is None or day is None:
        return None

    clock = _clock_parts(text)
    if clock is None:
        return None
    hour, minute, spans = clock

    try:
        remind_at = datetime(now.year, month, day, hour, minute, tzinfo=CST)
    except ValueError:
        return None
    return remind_at, [match.span(), *spans]


def _resolve_day(
    text: str, now: datetime,
) -> tuple[datetime, list[tuple[int, int]]] | None:
    """今天 / 明天 / 后天 / 大后天 + 钟点。没有钟点返回 None，交给调用方回问。"""
    match = _DAY_RE.search(text)
    if match is None:
        return None

    clock = _clock_parts(text)
    if clock is None:
        return None
    hour, minute, spans = clock

    target = now.date() + timedelta(days=_DAY_OFFSETS[match.group("day")])
    remind_at = datetime(
        target.year, target.month, target.day, hour, minute, tzinfo=CST,
    )
    return remind_at, [match.span(), *spans]


def _extract_content(text: str, time_spans: list[tuple[int, int]]) -> str:
    """从原话里删掉时间与委托词，剩下的就是事件描述（如 "开会"）。

    这是启发式：规则档不追求覆盖所有中文表达，只保证常见句式不把时间或
    "提醒我" 混进 content。删完为空说明用户只给了时间没给事，应回问。
    """
    merged: list[tuple[int, int]] = []
    for start, end in sorted(time_spans):
        if merged and start <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], end))
        else:
            merged.append((start, end))

    parts: list[str] = []
    cursor = 0
    for start, end in merged:
        parts.append(text[cursor:start])
        cursor = end
    parts.append(text[cursor:])

    content = _INTENT_RE.sub("", "".join(parts))
    content = re.sub(r"\s+", " ", content).strip()
    content = content.strip("，,。.!！?？、；;：:~～")
    content = re.sub(r"^(?:记得|要|一下)+", "", content)
    content = re.sub(r"(?:吧|哦|呀|啊|呢|哈|啦|嘛|哟)+$", "", content)
    return content.strip()
