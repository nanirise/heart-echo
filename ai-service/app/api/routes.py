import asyncio
import json
from collections.abc import AsyncIterator

from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field

from app.core.security import verify_internal_token

router = APIRouter()


class ChatRequest(BaseModel):
    """Go → Python 的内部契约（chat-stream spec §2.2）。

    刻意用 snake_case：内部调用与前端请求长得不一样，就不容易把两者搞混。
    尤其是 user_id —— 它只能来自 Go 侧的 Token，绝不能从请求体透传（AGENTS §4.3）。
    """

    user_id: int
    persona_id: int
    message: str = Field(min_length=1)


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


# PR 1 的假回复：不调 DeepSeek，先把「能不能流」和「AI 能不能答」拆开验（plan §1.1）。
_FAKE_REPLY = "好呀，我在这儿呢，慢慢说。"

# 每个字的间隔。打太快看不出是流式，太慢联调时干等。
_DELTA_INTERVAL_SECONDS = 0.1


async def _fake_stream() -> AsyncIterator[str]:
    for char in _FAKE_REPLY:
        yield _sse("delta", json.dumps({"text": char}, ensure_ascii=False))
        await asyncio.sleep(_DELTA_INTERVAL_SECONDS)

    # 内部终止事件。Go 收到它 → 落库 assistant 消息 → 自己发 done {"messageId": N}。
    # messageId 是 Go 侧落库后的主键，Python 不知道也不该知道，所以这里不带。
    # end 只存在于 Go ↔ Python 之间，绝不透传给前端 —— 契约 §6 冻结了三种事件。
    yield _sse("end", "{}")


@router.post("/internal/chat/stream", dependencies=[Depends(verify_internal_token)])
async def stream_chat(req: ChatRequest) -> StreamingResponse:
    """流式对话。响应头两项分别对应两层缓冲：

    - Cache-Control 防止中间层缓存事件流
    - X-Accel-Buffering: no 关掉 Nginx 的 proxy_buffering（AGENTS §4.5）

    本阶段 req 不参与生成，只做校验 —— 人格与历史消息 Week 3 才接进来。
    """
    return StreamingResponse(
        _fake_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
