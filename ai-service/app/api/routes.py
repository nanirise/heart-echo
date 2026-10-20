import json
from collections.abc import AsyncIterator
from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, ConfigDict, Field

from app.agents.memory_agent import build_memory_extractor
from app.core.llm_client import LLMUnavailableError, stream_reply
from app.core.security import verify_internal_token
from app.schedules.rule_parser import has_reminder_intent, parse_schedule

router = APIRouter()


class PersonaBrief(BaseModel):
    """参与生成的那三行人格设定（TECH_DESIGN §5.2 的占位符来源）。

    三个字段都给默认空串而不是必填：某一格没填时应该退化成"没有这条设定"，
    而不是让整次对话 422 —— 一次聊天不该因为人设少填一格就整个失败。
    """

    name: str = ""
    personality_desc: str = ""
    speaking_style: str = ""

    # 严格解码，与 PUT /user/profile 同一个理由（契约 §12 2026-09-30 条）：
    # 三个字段都有默认值，宽松解码下「Go 把字段名拼成 personalityDesc」的结果
    # 与「用户没填性格」完全一样 —— 都是空串，system prompt 变成"你是 。"，
    # 现象只是"说话不太像"，没有任何报错。宁可 422 报出来。
    model_config = ConfigDict(extra="forbid")


class HistoryTurn(BaseModel):
    """一条历史消息。role 的取值与 chat_messages 表的 CHECK 约束一致（user / assistant）。"""

    role: str
    content: str


class ChatRequest(BaseModel):
    """Go → Python 的内部契约（chat-stream spec §2.2）。

    刻意用 snake_case：内部调用与前端请求长得不一样，就不容易把两者搞混。
    尤其是 user_id —— 它只能来自 Go 侧的 Token，绝不能从请求体透传（AGENTS §4.3）。
    """

    user_id: int
    persona_id: int
    message: str = Field(min_length=1)

    # 两个**可选**字段（spec §2.2，2026-10-05 新增）。不传时的行为与 Week 2 完全一致：
    # 没有 persona 就不发 system message，没有 history 就只有当前这一句。
    #
    # 做成可选而不是必填，是为了让 spec §4 那条只发三个字段的验收 curl 继续有效 ——
    # 那条命令是排障入口，让它失效的代价比"漏传时静默退化"更大。
    persona: PersonaBrief | None = None
    history: list[HistoryTurn] = Field(default_factory=list)

    # 同 PersonaBrief：字段名拼错必须 422，不能退化成"这个字段没传"。
    model_config = ConfigDict(extra="forbid")


class ScheduleParseRequest(BaseModel):
    """Go → Python 的日程解析请求（TECH_DESIGN §5.7）。

    只收用户原话：解析不需要 user_id / persona_id，归属校验与落库都在 Go 侧。
    """

    message: str = Field(min_length=1)
    model_config = ConfigDict(extra="forbid")


class ScheduleParseResponse(BaseModel):
    """解析结果，三个互斥出口（配合 response_model_exclude_none 只发有值的字段）。

    - has_intent=false：这句话不是提醒委托（如"我明天要开会"），不建日程也不回问
    - has_intent=true + need_clarify=true：有提醒意图但时间/事件解析不出，Go 侧应让 AI 回问
    - has_intent=true + need_clarify=false + content + remind_at：解析成功，Go 侧据此建日程

    has_intent 必须显式下发：Go 侧不能把"缺省字段"与"明确没有意图"混为一谈，
    否则陈述句也会被当成需要澄清的委托。
    """

    has_intent: bool
    content: str | None = None
    remind_at: datetime | None = None
    need_clarify: bool | None = None


class MemoryExtractRequest(BaseModel):
    """Go → Python 的记忆提取请求（TECH_DESIGN §5.3）。

    只收用户原话：归属字段（user_id / persona_id）由 Go 侧从本轮对话补上，
    本服务不连数据库，也不该知道这条记忆属于谁。
    """

    message: str = Field(min_length=1)
    model_config = ConfigDict(extra="forbid")


class MemoryItem(BaseModel):
    """候选记忆。恰好三个字段，与 Phase 2 的输出契约逐字一致。"""

    memory_type: str
    content: str
    importance_score: float


class MemoryExtractResponse(BaseModel):
    """提取结果。memories 为空列表而不是 null；profile_updates 没有时给 {}。"""

    memories: list[MemoryItem]
    profile_updates: dict[str, Any]


@router.get("/health")
async def health() -> dict[str, str]:
    """Go 侧探活用，**故意不校验 token** —— 契约 §11 明写健康检查不带 X-Internal-Token。

    给它加鉴权会让 Go 的 /health 永远报 aiService: down。
    """
    return {"status": "ok"}


def _sse(event: str, data: str) -> str:
    """拼一个 SSE 事件块。

    块尾固定两个换行符（契约 §6 已冻死这一条）。写成 \r\n\r\n 在 Windows 本地照样跑通，
    只在 Linux 容器里表现成「页面一个字都不出、控制台不报错」。
    """
    return f"event: {event}\ndata: {data}\n\n"


# 生成失败时下发的错误码，与 backend/pkg/errcode 的 ErrLLMFailed 同码同文案（契约 §2：一 code 一 msg）。
# Python 与 Go 没有共享常量的地方，改一处必须改两处 —— 这是这条链路唯一的跨语言常量。
_ERR_LLM_FAILED = 5001
_ERR_LLM_FAILED_MESSAGE = "AI 回复生成失败，请稍后重试"


async def _chat_stream(
    message: str,
    persona: PersonaBrief | None,
    history: list[HistoryTurn],
) -> AsyncIterator[str]:
    """把 LLM 的文本段转成 delta 事件；生成失败转成 error 事件。

    失败时**不发 end**：end 的语义是"生成完了，你可以落库了"，两者同时出现会让 Go
    对着一句半截话去落库。
    """
    try:
        async for text in stream_reply(message, persona=persona, history=history):
            yield _sse("delta", json.dumps({"text": text}, ensure_ascii=False))
    except LLMUnavailableError:
        # 已经吐出去的 delta 收不回来 —— 前端会先留半句话再叠一条错误提示。
        # 这就是"重试只能在流开始前做"的代价（spec §2.3 的已知取舍）。
        yield _sse(
            "error",
            json.dumps(
                {"code": _ERR_LLM_FAILED, "message": _ERR_LLM_FAILED_MESSAGE},
                ensure_ascii=False,
            ),
        )
        return

    # 内部终止事件。Go 收到它 → 落库 assistant 消息 → 自己发 done {"messageId": N}。
    # messageId 是 Go 侧落库后的主键，Python 不知道也不该知道，所以这里不带。
    # end 只存在于 Go ↔ Python 之间，绝不透传给前端 —— 契约 §6 冻结了三种事件。
    yield _sse("end", "{}")


@router.post("/internal/chat/stream", dependencies=[Depends(verify_internal_token)])
async def stream_chat(req: ChatRequest) -> StreamingResponse:
    """流式对话。响应头两项分别对应两层缓冲：

    - Cache-Control 防止中间层缓存事件流
    - X-Accel-Buffering: no 关掉 Nginx 的 proxy_buffering（AGENTS §4.5）

    req.user_id / persona_id 仍不参与生成，只做校验与日志。真正决定"谁在说话、记不记得
    上一句"的是 req.persona 与 req.history —— 它们由 Go 侧从数据库取好送进来，
    因为本服务不连数据库（spec §2.2）。
    """
    return StreamingResponse(
        _chat_stream(req.message, req.persona, req.history),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


@router.post(
    "/schedule/parse",
    dependencies=[Depends(verify_internal_token)],
    response_model_exclude_none=True,
)
async def parse_schedule_endpoint(req: ScheduleParseRequest) -> ScheduleParseResponse:
    """把用户原话解析成 {has_intent, content, remind_at}，解析不出时给 need_clarify。

    路径按 TECH_DESIGN §5.7 固定为 /schedule/parse（不是 /internal/...）；
    鉴权仍走 X-Internal-Token，与 /internal/chat/stream 同一道防线。

    三种出口都不能省略：把"没有提醒意图"也回成 need_clarify=true，会让 AI 对
    每一句陈述都追问一次，比漏建日程更打扰用户。
    """
    text = req.message.strip()
    if not has_reminder_intent(text):
        # 陈述句不是委托：不建日程，也不回问。
        return ScheduleParseResponse(has_intent=False)

    parsed = parse_schedule(text)
    if parsed is None:
        # 有意图但解析不出时间/事件：Go 侧据此让 AI 回问，不能静默丢弃。
        return ScheduleParseResponse(has_intent=True, need_clarify=True)

    return ScheduleParseResponse(
        has_intent=True,
        need_clarify=False,
        content=parsed.content,
        remind_at=parsed.remind_at,
    )


@router.post("/memory/extract", dependencies=[Depends(verify_internal_token)])
async def extract_memory(req: MemoryExtractRequest) -> MemoryExtractResponse:
    """从一轮用户消息里提取候选记忆与画像更新（TECH_DESIGN §5.3）。

    只返回候选，不落库：user_id / persona_id 由 Go 侧赋值（user-memory spec §3.3）。
    MEMORY_EXTRACT_BACKEND 决定本轮用规则还是 LLM；切换不改变本端点的契约。
    """
    result = await build_memory_extractor().extract(req.message.strip())
    return MemoryExtractResponse(
        memories=[MemoryItem(**candidate.as_contract()) for candidate in result.memories],
        profile_updates=result.profile_updates,
    )
