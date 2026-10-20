import asyncio
import logging
import time
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.routes import router
from app.core.config import get_settings
from app.memory import get_retriever

logger = logging.getLogger(__name__)


async def _warmup_memory_retriever() -> None:
    """启动时预加载 ONNX 模型，避免首轮检索撞上 2 秒超时。"""
    if get_settings().memory_backend != "vector":
        return

    # 应用 logger 默认没有 handler，uvicorn 也不会配置它：logger.info 会被
    # root logger 的 WARNING 级别过滤掉，终端看起来就像 lifespan 没执行。
    # 关键节点用 print(flush=True) 保证一定可见。
    print("ChromaDB 预热中...", flush=True)
    logger.info("ChromaDB 预热中...")
    started = time.perf_counter()
    try:
        # get_retriever() 首次调用会连 Chroma，连不上也必须在这里捕获：
        # 记忆检索是增强功能，不能因为容器没起就让整个服务启动失败。
        retriever = get_retriever()
        # 模型加载是阻塞调用（首次还会下载模型），放线程里不占事件循环。
        await asyncio.to_thread(retriever.warmup)
    except Exception:
        logger.warning(
            "ChromaDB 预热失败，服务继续启动；首次检索时会重试",
            exc_info=True,
        )
        print("ChromaDB 预热失败，服务继续启动（详见日志）", flush=True)
        return

    elapsed = time.perf_counter() - started
    logger.info("ChromaDB 预热完成，耗时 %.2f 秒", elapsed)
    print(f"ChromaDB 预热完成，耗时 {elapsed:.2f} 秒", flush=True)


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    await _warmup_memory_retriever()
    yield


def create_app() -> FastAPI:
    """装配 FastAPI 应用。

    用工厂函数而不是模块级 app 对象：测试可以拿到干净实例，将来加中间件也不会污染已导入的模块。
    """
    app = FastAPI(title="HeartEcho AI Service", lifespan=lifespan)
    app.include_router(router)
    return app


app = create_app()
