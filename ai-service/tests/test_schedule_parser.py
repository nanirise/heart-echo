"""钉住规则档日程解析的时间换算与 content 抽取（TECH_DESIGN §5.7）。

固定 now 为 2026-10-19（周一）10:00 +08:00：周末/月份边界都靠它，
不能用 datetime.now()，否则测试在周日或月末会随真实日期漂移。
"""

from datetime import datetime, timedelta, timezone

import pytest

from app.schedules.rule_parser import has_reminder_intent, parse_schedule

CST = timezone(timedelta(hours=8))
NOW = datetime(2026, 10, 19, 10, 0, tzinfo=CST)  # Monday


def parse(text: str):
    return parse_schedule(text, now=NOW)


@pytest.mark.parametrize(
    ("text", "expected", "content"),
    [
        # 今天 / 明天 / 后天 / 大后天 + 时段 + 点
        ("今天上午九点提醒我交周报", datetime(2026, 10, 19, 9, 0, tzinfo=CST), "交周报"),
        ("明天下午三点提醒我开会", datetime(2026, 10, 20, 15, 0, tzinfo=CST), "开会"),
        ("后天晚上八点提醒我看球赛", datetime(2026, 10, 21, 20, 0, tzinfo=CST), "看球赛"),
        ("大后天中午十二点提醒我取快递", datetime(2026, 10, 22, 12, 0, tzinfo=CST), "取快递"),
        # 事件描述里的 "周五" 不能抢走明确的 "明天"（解析器优先级：日词 > 星期）
        (
            "明天下午三点提醒我讨论周五的方案",
            datetime(2026, 10, 20, 15, 0, tzinfo=CST),
            "讨论周五的方案",
        ),
        # 没写上午/下午时按 24 小时制（设计明确钉死，不能猜成 15:00）
        ("明天三点提醒我开会", datetime(2026, 10, 20, 3, 0, tzinfo=CST), "开会"),
        ("明天15点提醒我开会", datetime(2026, 10, 20, 15, 0, tzinfo=CST), "开会"),
        # 半 / 一刻 / 三刻 / 分
        ("明天下午三点半提醒我开会", datetime(2026, 10, 20, 15, 30, tzinfo=CST), "开会"),
        ("明天下午三点一刻提醒我开会", datetime(2026, 10, 20, 15, 15, tzinfo=CST), "开会"),
        ("明天下午三点三刻提醒我开会", datetime(2026, 10, 20, 15, 45, tzinfo=CST), "开会"),
        ("明天下午3点20分提醒我开会", datetime(2026, 10, 20, 15, 20, tzinfo=CST), "开会"),
        ("明天下午三点二十开会提醒我", datetime(2026, 10, 20, 15, 20, tzinfo=CST), "开会"),
        # 24 小时钟点
        ("明天15:30提醒我开会", datetime(2026, 10, 20, 15, 30, tzinfo=CST), "开会"),
        # 相对时间
        ("1分钟后提醒我喝水", NOW + timedelta(minutes=1), "喝水"),
        ("三分钟后提醒我给妈妈打电话", NOW + timedelta(minutes=3), "给妈妈打电话"),
        ("两小时后提醒我关火", NOW + timedelta(hours=2), "关火"),
        ("半小时后提醒我收衣服", NOW + timedelta(minutes=30), "收衣服"),
        # 星期
        ("下周一提醒我开会", datetime(2026, 10, 26, 9, 0, tzinfo=CST), "开会"),
        ("本周五下午两点提醒我汇报", datetime(2026, 10, 23, 14, 0, tzinfo=CST), "汇报"),
        # 月日
        ("10月25号下午2点提醒我体检", datetime(2026, 10, 25, 14, 0, tzinfo=CST), "体检"),
    ],
)
def test_parse_time_and_content(text: str, expected: datetime, content: str) -> None:
    result = parse(text)

    assert result is not None
    assert result.remind_at == expected
    assert result.content == content


@pytest.mark.parametrize(
    "text",
    [
        "我明天要开会",          # 陈述句，不是委托
        "不用提醒我开会",        # 否定委托
        "提醒我开会",            # 有意图没时间
        "明天提醒我开会",        # 有日期没钟点：不能默认 09:00
        "过阵子提醒我",          # 有意图没可解析时间
        "下午三点提醒我开会",    # 没日期：不知道是今天还是明天
        "明天三点提醒我",        # 只有时间没有事件
    ],
)
def test_returns_none_when_not_parseable(text: str) -> None:
    assert parse(text) is None


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("提醒我明天开会", True),
        ("记得提醒我吃药", True),
        ("明天三点叫我一声", True),
        ("别让我忘了交房租", True),
        ("我明天要开会", False),
        ("我记得明天要开会", False),
        ("不用提醒我开会", False),
        ("别叫我起床", False),
    ],
)
def test_has_reminder_intent(text: str, expected: bool) -> None:
    assert has_reminder_intent(text) is expected


def test_content_is_event_not_whole_sentence() -> None:
    """契约 §10：content 是用户原话里的事件描述，不是整句、也不是 AI 写的提醒语。"""
    result = parse("提醒我一下，明天下午三点去超市买东西")

    assert result is not None
    assert result.content == "去超市买东西"


def test_period_after_time_stays_in_content() -> None:
    """离时间很远的 "晚上" 属于事件描述，不能被当成时段删掉、也不能改小时。"""
    result = parse("明天三点提醒我晚上吃药")

    assert result is not None
    assert result.remind_at == datetime(2026, 10, 20, 3, 0, tzinfo=CST)
    assert result.content == "晚上吃药"


def test_naive_now_is_treated_as_cst() -> None:
    result = parse_schedule("明天下午三点提醒我开会", now=datetime(2026, 10, 19, 10, 0))

    assert result is not None
    assert result.remind_at.utcoffset() == timedelta(hours=8)
