# 成员 1 开发文档 · Go 框架 + 鉴权 + 对话核心 + AI 服务

> 项目全貌：[项目概览](../../README.md)。
> 你是 Week 1 的瓶颈，也是整个项目的技术地基。你的产出被另外两人直接依赖。
> 协调规则与集成顺序见 [开发总纲](MASTER.md)，设计细节见 [技术文档](../TECH_DESIGN.md)。

## 0. 快速定位

| 项 | 内容 |
|----|------|
| 你负责 | Go 项目骨架、统一响应中间件、错误码、JWT 双令牌、注册登录 API、消息仓储、SSE 流式对话后端、Python AI 服务（三 Agent）、**日程抽取 + 规则时间解析（P1）** |
| 你的主要文件 | `backend/cmd/`、`backend/internal/{config,middleware,dto,handler/auth_handler.go,handler/chat_handler.go,handler/memory_handler.go,handler/router.go,service/auth_service.go,service/chat_service.go,service/ai_client.go,repository/memory_repo.go}`、`backend/pkg/**`、`ai-service/**`（含 `app/schedules/`） |
| 你不负责 | `internal/model/`（成员 3）、人设/朋友圈/主动消息（成员 3）、**日程的表、定时触发与前端页（成员 3）**、前端全部（成员 2）、部署（成员 3） |
| 你的 commit scope | `auth` `user` `chat` `memory` `emotion` `schedule` `errcode` |
| 你依赖 | 成员 3 的 `internal/model/*.go`（准备期交付）；成员 2 的前端页面用于联调 |
| 谁依赖你 | **全员**——`pkg/response`、`pkg/errcode`、`pkg/jwt`、中间件链、路由注册方式 |
| 你的硬性要求 | ② Go + 统一响应中间件 + 统一错误码（一 code 一 msg）；③ JWT 鉴权；有 Agent |

**第一原则**：Week 1 前半段必须把 `pkg/*` 和中间件链**冻结**。冻结后改公共约定要群里广播。

---

## 1. 准备期（1-2 天）

- [ ] 本地环境：Go ≥1.21、Docker Desktop、Python ≥3.10
- [ ] `go mod init github.com/<org>/heart-echo/backend`，建立目录骨架（技术文档 [§8](../TECH_DESIGN.md#8-目录结构)）
- [ ] `cp backend/.env.example backend/.env`，本地起 PostgreSQL 后确认能连
- [ ] 写 `pkg/errcode/errcode.go` + `errcode_test.go`（**交接物 #2**）
- [ ] 写 `pkg/response/response.go`（`Success[T]` / `Fail`，`Fail` 不接受 message 参数）
- [ ] 写 `internal/config/config.go`（用 `caarlos0/env` 加载）
- [ ] 与成员 3 对齐 `model/*.go` 字段（他交付，你消费）
- [ ] 参与契约评审：过一遍 [总纲 §4 端点表](MASTER.md#42-端点归属表)，确认字段与错误码
- [ ] 确认路由注册方式（[总纲 §4.5](MASTER.md#45-路由注册方式成员-1-冻结全员遵守)）并告诉另两人

**准备期末判据**：`go build ./...` 通过；`go test ./pkg/errcode/` 通过；`curl localhost:8080/api/v1/health` 返回 `{"code":200,...}`。

---

## 2. Week 1：认证链路

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 中间件链 | `internal/middleware/{recovery,biz_error,jwt,cors,logger}.go` | 顺序 Recovery → Logger → CORS → BizError → JWTAuth → Handler |
| JWT 双令牌 | `pkg/jwt/jwt.go` | Access 2h / Refresh 7d；refresh 不能访问业务接口 |
| 注册 API | `internal/handler/auth_handler.go` + `service/auth_service.go` | 用户名/邮箱重复返回 4003/4004 |
| 登录 API | 同上 | 密码错误返回 4013；bcrypt 校验 |
| 刷新 Token | `POST /auth/refresh` | 返回新令牌对 |
| 用户信息 | `GET/PUT /user/profile` | 带 Token 可读，不带返回 4010；`PUT` 支持改 `avatarUrl` / `username`（用户名冲突返回 4004） |
| 修改密码 | `handler/auth_handler.go` + `service/auth_service.go` | `PUT /user/password`；bcrypt 比对旧密码，错返回 4015 |
| 仓储层 | `internal/repository/{user_repo,message_repo}.go` | 所有查询带 `user_id` 条件 |
| 路由汇总 | `internal/handler/router.go` | 全部端点可达；`/health` 可用 |
| 路由挂载点 | 预留 `RegisterXxxRoutes` 调用行 | 成员 3 通知后加一行 |

**关键代码要求**

- `Fail()` 签名固定为 `Fail(c *gin.Context, code errcode.ErrorCode)`——**不接受 message 参数**，这是一 code 一 msg 的结构性保证。
- handler 里**不写** `response.Fail`，错误一律 `_ = c.Error(err)` 上抛，交给 `BizErrorHandler` 中间件出口（技术文档 §4.4）。
- 登录失败统一返回 `4013`，不区分「用户不存在」和「密码错误」，防止用户名枚举。
- 密码 bcrypt cost ≥ 10，**严禁明文/MD5**。

**Week 1 末里程碑**：注册 → 登录 → 刷新页面保持登录态 → 与成员 2 前后端联调成功。

---

## 3. Week 2：流式对话（生死线）

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| Python 服务骨架 | `ai-service/app/main.py`、`api/routes.py`、`core/config.py` | `uvicorn app.main:app` 可起，`/health` 通 |
| DeepSeek 客户端 | `ai-service/app/core/llm_client.py` | 能拿到流式 chunk |
| 对话生成 | `ai-service/app/agents/dialogue_agent.py` | 拼装人格 + 历史，流式返回 |
| Go 侧 AI 客户端 | `internal/service/ai_client.go` | 接口抽象见技术文档 §5.0 |
| SSE 接口 | `internal/handler/chat_handler.go` + `chat_service.go` | 3 种事件（`delta`/`done`/`error`）按契约下发 |
| 消息查询 | 同上 | 按 `personaId` 拉历史消息（**一个人设一个对话，没有会话 CRUD**） |

**SSE 实现要点**

- 响应头：`Content-Type: text/event-stream`、`Cache-Control: no-cache`、`Connection: keep-alive`、`X-Accel-Buffering: no`。
- 事件严格按契约：`delta`* → `done`，异常走 `error`（[总纲 §4.3](MASTER.md#43-sse-事件契约成员-1-提供成员-2-解析)）。
- **情感分析结果不通过 SSE 下发**：它写入消息表，并作为对话生成 Agent 的输入（决定共情策略）。前端不展示情绪标签。
- 完整回复落库后才发 `done`，`done` 里带 `messageId`。
- 生成失败不要让 HTTP 500，要以 `event: error` 形式下发，前端才能优雅提示。

**Week 2 末生死线自测**（按 [总纲 §6](MASTER.md#6-集成顺序按这个顺序联调不要跳) 第 4-6 步，逐层验证）：

```bash
# 第 4 步：直连 Python，确认它本身能流式
curl -N -X POST http://localhost:8000/chat/stream \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: $AI_SERVICE_TOKEN" \
  -d '{"user_id":1,"persona_id":1,"message":"今天上班好累啊"}'

# 第 5 步：经过 Go
curl -N -X POST http://localhost:8080/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"personaId":1,"content":"今天上班好累啊"}'
```

**判据**：两条命令都能看到 `event:` / `data:` 逐条输出，不是最后一次性吐出。第 4 步不过就不要往上查 Go。

> 🚨 本周末若前端还不能流式显示回复，立即在群里提议砍掉记忆与主动消息（总纲 §10）。

---

## 4. Week 3：Agent 与记忆

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 情感分析 Agent | `ai-service/app/classifiers/{base,lexicon,__init__}.py` | 8 类情绪，返回 `{label, score}` |
| 记忆提取 Agent | `ai-service/app/agents/memory_agent.py` | 规则提取「我叫X」「我养了X」等 |
| 记忆检索 | `ai-service/app/memory/{base,recent,__init__}.py` | **该人设**最近 50 条 → 关键词加权 → Top-5 |
| 记忆注入 | `dialogue_agent.py` prompt 组装 | 技术文档 §5.2 的 Prompt 结构 |
| 记忆查询 API | `GET /api/v1/memory`（必带 `personaId`） | **必须带 `persona_id` + `user_id` 过滤** |
| 用户画像 API | `GET /profile/portrait`（必带 `personaId`） | 返回该人设的 `profile_data` |
| 人格演化 | `personas.state.familiarity` 累加 + 注入 prompt | 技术文档 §5.6 的亲密度区间表 |

> `GET /emotion/trend` 是 **P2 预留端点，本周不实现**（README P2 已明确不做情绪趋势）。不要把时间花在这里。
> **没有 `/emotion/diary`**：情绪标签是内部信号、不向用户展示，"情绪日记"与这条全局约定冲突，已从契约中删除——**不要顺手实现它**。

**必须遵守的接口抽象**（阶段二替换实现不改调用方）：

```python
def get_classifier() -> EmotionClassifier:
    if settings.EMOTION_BACKEND == "onnx":
        from .onnx import OnnxEmotionClassifier
        return OnnxEmotionClassifier(settings.EMOTION_MODEL_PATH)
    from .lexicon import LexiconEmotionClassifier
    return LexiconEmotionClassifier()
```

**安全红线**：记忆检索、画像查询一律 `WHERE persona_id = ? AND user_id = ?`。
`persona_id` 保证「一人设一份记忆」，`user_id` 保证不跨用户——**两个都要带**，只带 `persona_id` 会串号（它是全局自增，别的用户的同号人设会命中）。跨用户召回是功能性问题，不是代码风格问题（协作 §10.4 红线 4）。

**Week 3 末里程碑**：核心演示链路（登录 → 人设 → 流式对话 → 记忆 → 主动消息）全通。

---

## 5. Week 4：阶段二升级 + 联调

按技术文档 [§5.0 推进顺序](../TECH_DESIGN.md#阶段二的推进顺序)，**每完成一项立即 commit，保持可回退**：

- [ ] ① LLM 结构化记忆提取（`MEMORY_EXTRACT_BACKEND=llm`）
- [ ] ② ChromaDB 向量检索（`MEMORY_BACKEND=vector`，**metadata 存 `user_id` + `persona_id`，检索两个都过滤**）
- [ ] ③ ONNX 情感模型（`EMOTION_BACKEND=onnx`，**先单独跑通一次推理再集成**）
- [ ] （**P1，有余量才做**）④ 日程抽取 + 规则时间解析（`app/schedules/` + `SCHEDULE_PARSE_BACKEND`）
- [ ] 前后端联调，配合成员 3 排查部署环境问题

**日程抽取要点（P1，见 [技术文档 §5.7](../TECH_DESIGN.md#57-日程提醒p1)）**

- **只抽有明确提醒意图的句子**（"提醒我"、"叫我"、"别让我忘了"）。「我明天要开会」是陈述句，**不建日程**——抽错比不抽更糟。
- **时间解析的阶段一只做规则档**：今天/明天/后天 + 上午下午晚上 + N 点；X 分钟后/小时后；下周一/本周五。够答辩演示用。
- **解析不出必须回问**：「好的，但我没听准是哪天——你是说下周一吗？」**绝不静默丢弃**。这个回问本身就是演示点。
- **你不碰 `schedules` 表**：抽出来的 `{content, remind_at}` 交给成员 3 落库；到期触发也是他做。你只负责"从话里听出来 + 把时间算准"。

**纪律**：任何一项升级到一半卡住，立刻改回阶段一的环境变量值，保证服务可演示，再继续下一项。

---

## 6. 你的接口契约（对外发布）

你发布的契约，成员 2 / 成员 3 依赖：

| 契约 | 内容 | 冻结时间 |
|------|------|----------|
| `Response[T]` | `{code, message, data, timestamp}` | 准备期 |
| 错误码表 | [技术文档 §7.4](../TECH_DESIGN.md#74-错误码总表) | 准备期 |
| JWT Header | `Authorization: Bearer <access_token>` | 准备期 |
| SSE 事件 | `delta` / `done` / `error`（情绪为内部信号，不下发） | Week 2 前 |
| 路由注册函数签名 | `RegisterXxxRoutes(rg *gin.RouterGroup, h *XxxHandler)` | 准备期 |
| AI 服务内部 API | 见下 | Week 2 前 |

**AI 服务内部接口**（Go ↔ Python，不对外）：

```
POST /chat/stream        流式对话，SSE
POST /emotion/analyze    单条文本情感分析
POST /memory/extract     从对话提取记忆
POST /schedule/parse     从对话抽取日程意图并解析时间（P1，解不出返回 need_clarify）
```

请求头带 `X-Internal-Token: <AI_SERVICE_TOKEN>`，Python 侧校验，不一致返回 401。

---

## 7. 常见坑

| 坑 | 现象 | 处理 |
|----|------|------|
| Nginx 缓冲 | 本地流式正常，生产不流式 | `proxy_buffering off` + `X-Accel-Buffering: no` |
| Go 未 flush | 前端等到最后一次性出现 | 每次 write 后 `c.Writer.Flush()` |
| CORS 预检 | 浏览器报跨域，curl 正常 | 本地走 CORS 中间件；生产走 Nginx 同源，不需要 CORS |
| refresh 当 access 用 | 业务接口鉴权异常 | JWTAuth 里校验 `claims.TokenType == "access"` |
| 跨用户查询 | 越权拿到他人数据 | 所有涉及 `persona_id` 的查询加 `user_id` 条件 |
| 记忆串号 | A 人设记起 B 人设的事 | 记忆/画像查询**必须同时**带 `persona_id` 与 `user_id`；阶段二 ChromaDB 的 metadata 两个都要存、检索两个都要过滤 |
| SSE 超时 | 长回复中途断开 | 客户端与 Nginx 的 read timeout 都设 300s |
| AI 服务 token 不一致 | 后端调 AI 恒 401 | 两个 `.env` 的 `AI_SERVICE_TOKEN` 必须相同 |
| 日程静默丢弃（P1） | 用户说"过阵子提醒我"，AI 应了声但什么都没发生 | 解析不出**必须回问**，不能既不入库也不吭声——这是该功能最差的表现 |
| 日程误抽（P1） | "我明天要开会"被建成日程 | 必须有显式提醒意图才抽；陈述句不建 |

---

## 8. 每周自检

- [ ] 我的提交都 push 了，commit 符合 `<type>(<scope>): <subject>`
- [ ] `go build ./...`、`go test ./...` 通过
- [ ] 新增错误码都登记进了 `codeMessages` 与 `codeHTTPStatus`，单测通过
- [ ] 没有把 `.env` / API Key 提交进仓库
- [ ] 改动了公共约定（响应结构 / 错误码 / SSE 事件）→ 已在群里广播
- [ ] 记忆/画像查询都带了 `persona_id` **和** `user_id` 两个条件
