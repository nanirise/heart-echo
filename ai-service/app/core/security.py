import secrets

from fastapi import Header, HTTPException, status

from app.core.config import get_settings


async def verify_internal_token(x_internal_token: str | None = Header(default=None)) -> None:
    """校验 Go 侧带过来的 X-Internal-Token。

    内部接口只给 Go 调，不该对外可达，前缀 /internal 本身就是一道防线，这里是第二道。

    缺失与不匹配都返回 401（而不是让 FastAPI 对缺失的 Header 抛 422）——
    对调用方来说两者是同一件事：你的 token 不对。
    """
    expected = get_settings().ai_service_token

    if x_internal_token is None or not secrets.compare_digest(x_internal_token, expected):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid internal token",
        )
