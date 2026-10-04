FROM python:3.11-slim
WORKDIR /app
# 不生成 .pyc；日志实时输出（容器里没有 tty，输出被缓冲会让日志滞后）
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
# --host 必须 0.0.0.0：绑 127.0.0.1 时容器间连接被拒（AGENTS §4.10）
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
