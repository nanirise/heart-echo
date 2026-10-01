from fastapi import FastAPI

from app.api.routes import router


def create_app() -> FastAPI:
    """装配 FastAPI 应用。

    用工厂函数而不是模块级 app 对象：测试可以拿到干净实例，将来加中间件也不会污染已导入的模块。
    """
    app = FastAPI(title="HeartEcho AI Service")
    app.include_router(router)
    return app


app = create_app()
