from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """AI 服务的全部配置。字段名小写，pydantic-settings 会大小写不敏感地匹配同名环境变量。"""

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    # 无默认值 = 缺失时服务起不来。这是故意的：token 缺失时若给个空串默认值，
    # 服务能起来但 Go 侧调用恒 401，排查方向会完全跑偏（AGENTS §4.3）。
    ai_service_token: str

    # 容器内必须是 0.0.0.0。写 127.0.0.1 时本机 curl 正常、容器间连接被拒（AGENTS §4.10）。
    host: str = "0.0.0.0"
    port: int = 8000

    # 同 ai_service_token：不给默认值，缺了就在启动时报出来。
    # 给空串的话服务能起、每次对话才 401，与"Key 忘了填"这个真实原因隔了好几层。
    deepseek_api_key: str
    # 这几个是"有唯一合理值"的配置，给默认值让本地少配几行。
    deepseek_base_url: str = "https://api.deepseek.com"
    deepseek_model: str = "deepseek-flash"
    llm_timeout_seconds: float = 60.0
    llm_max_tokens: int = 4096


@lru_cache
def get_settings() -> Settings:
    """进程内单例 —— 避免每个请求都重读一次 .env。"""
    return Settings()
