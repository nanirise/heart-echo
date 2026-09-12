# AGENT.md · HeartEcho

前后端分离的 AI 情感陪伴 Web 应用：用户创建多个 AI 伴侣，每个伴侣独立记忆、感知情绪、主动问候，并可自行发朋友圈互动。

**当前仓库只有文档，没有代码**——`backend/`、`ai-service/`、`frontend/`、`deploy/` 均待创建。实现前先读文档，不要凭常识补设计。

## 必读文档（按顺序）

1. `README.md` —— 项目定位、功能范围、排期分工、演示脚本
2. `docs/dev/MASTER.md` —— **开发总纲**，其中 §0 功能决策记录是产品决策的唯一来源
3. `docs/TECH_DESIGN.md` —— 技术设计；**先读 §5.0 实现分级与升级路径**
4. `docs/API_CONTRACT.md` —— 接口契约（准备期冻结，改字段须升版本号并广播）
5. `docs/COLLABORATION.md` —— 分工边界、Git 工作流、Commit 规范、命名规范
6. `docs/KICKOFF.md` —— 立项讨论稿（讲"做什么、值不值"）
7. `docs/dev/MEMBER_{1_BACKEND_AI,2_FRONTEND,3_DATA_MOMENTS_DEPLOY}.md` —— 按成员分周任务

## 技术栈

| 层 | 选型 |
|----|------|
| 后端 | Go ≥1.21 + Gin + GORM，JWT 双 Token 鉴权，统一响应与错误码中间件，`robfig/cron` 定时任务 |
| AI 服务 | Python FastAPI 薄层，经内部 HTTP 从 Go 调用（不暴露公网）；对话用 DeepSeek API |
| 情感 / 记忆 | 阶段一：关键词词典 + PostgreSQL 检索；阶段二：ONNX 小模型 + ChromaDB（接口已抽象，只换实现） |
| 前端 | Vue 3 + TypeScript（`strict`）+ Pinia + Vue Router 4 + Element Plus + Axios；流式用 `fetch` + `ReadableStream` 解析 SSE |
| 数据库 | PostgreSQL 16，唯一数据存储；建表用 GORM `AutoMigrate`，不引入迁移工具 |
| 缓存 | 无（明确不引入 Redis） |
| 测试 | Go 单测（`errcode` 包校验错误码集合一致）；前端与 Python 测试框架未指定 |
| 部署 | Docker Compose + Nginx（前端静态文件与 API 转发同镜像）；`http://<公网IP>`，不做域名 / HTTPS / 备案 |

本地端口：前端 `5173`、Go `8080`、AI 服务 `8000`、PostgreSQL `5432`（仅开发映射到宿主机）。

## 常用命令

```bash
# 依赖服务（本地开发，只起 PostgreSQL）
docker compose -f deploy/docker-compose.dev.yml up -d

# 后端 :8080
cd backend && cp .env.example .env && go mod download && go run ./cmd/server
go build ./... && go test ./...

# AI 服务 :8000
cd ai-service && cp .env.example .env && pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000

# 前端 :5173
cd frontend && npm install && npm run dev
npx tsc --noEmit && npm run build

# 生产部署
docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml ps   # 确认全部 healthy
```

约定：Commit 用 `<type>(<scope>): <subject>`，subject ≤50 字符；`.env` 与任何密钥永不入库，仓库只留 `.env.example`。

## TODO

- 以上命令来自文档，代码目录尚未创建；脚本入口（如后端是否是 `./cmd/server`、Python 依赖是否用 `pip` 还是其他工具）待代码落地后核对。
- 前端单测/组件测试框架、Python 测试框架文档中未指定。
- 生产部署脚本、CI 未定义（文档明确不引入 CI/CD 自动化）。
