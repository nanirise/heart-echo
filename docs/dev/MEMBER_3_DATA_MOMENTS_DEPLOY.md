# 成员 3 开发文档 · 数据 + 人设 + 朋友圈 + 部署

> 项目全貌：[项目概览](../../README.md)。
> 你是唯一横跨「数据库 / 后端 / 前端 / 运维」的人。你的 model 交付是成员 1 的前置，你的部署决定项目能不能上云。
> 协调规则与集成顺序见 [开发总纲](MASTER.md)，设计细节见 [技术文档](../TECH_DESIGN.md)。

## 0. 快速定位

| 项 | 内容 |
|----|------|
| 你负责 | 数据库表设计与 AutoMigrate、人设 CRUD（前后端）、主动消息（前后端 + 定时任务 + 手动触发）、AI 朋友圈（前后端）、**日程提醒（表 + 定时触发 + 前端列表，P1）**、Docker Compose、Nginx、云服务器部署 |
| 你的主要文件 | `backend/internal/model/**`、`handler/{persona_handler.go,moment_handler.go,proactive_handler.go,schedule_handler.go}`、`service/{proactive_*.go,moment_job.go,schedule_service.go}`、`repository/{persona_repo,moment_repo,proactive_repo,schedule_repo}.go`、`dto/{persona_dto,moment_dto,proactive_dto,schedule_dto}.go`；`frontend/src/views/{persona,moments,schedule}/`、`api/{persona,moment,proactive,schedule}.ts`、`stores/persona.ts`；`deploy/**` |
| 你不负责 | 统一响应/错误码/JWT/中间件（成员 1）、聊天与 AI 服务（成员 1）、登录注册页/聊天页/画像页（成员 2） |
| 你的 commit scope | `model` `persona` `moment` `proactive` `schedule` `deploy` |
| 你依赖 | 成员 1 的 `pkg/response`、`pkg/errcode`、中间件、路由注册方式；成员 2 的 `request.ts`、`types/api.ts`、路由挂载点 |
| 谁依赖你 | **成员 1**——`internal/model/*.go` 是他写 repository 的前置；**成员 2**——人设接口是他聊天页与画像页的消费方 |
| 你的硬性要求 | ④ 目录结构与命名规范；⑥ 部署上云、公网访问 |

**第一原则**：准备期先把 10 个 GORM struct 的字段定死并交付，这是全队的起跑发令枪。（第 10 个 `Schedule` 是 **P1**，字段照技术文档 §6.2 先定死，实现留到 Week 4 余量。）

---

## 1. 准备期（1-2 天）

- [ ] 本地环境：Docker Desktop、Go ≥1.21、Node ≥18
- [ ] 按 [技术文档 §6.2 DDL](../TECH_DESIGN.md#62-核心表-ddl) 翻译成 `internal/model/*.go`，共 10 个结构体——**交接物 #1**
- [ ] 写 `internal/model/migrate.go` 的 `AutoMigrate(db)`（技术文档 §6.4）
- [ ] `deploy/docker-compose.dev.yml`：只起 PostgreSQL 16——**交接物 #6**（记得 `ports: "5432:5432"`，否则本机 Go 连不上）
- [ ] `docker compose up -d` 验证 `psql` 能连，字段与 DDL 一致
- [ ] 建好 `deploy/.env.example`
- [ ] 建好 `deploy/Dockerfile`、`backend/Dockerfile`、`ai-service/Dockerfile`、根目录 `.dockerignore`（见 [技术文档 §9.3.1](../TECH_DESIGN.md#931-dockerfile三个镜像)）——**不要留到 Week 4**，部署问题越晚暴露越致命
- [ ] 逐个构建验证，确认三个镜像都能成功构建（不需要真的部署）

**自己写 Dockerfile 时的验证方法**——一次只构建一个，出错才知道是谁的问题：

```bash
# ① 单独构建后端镜像（第一次会拉取 golang 基础镜像，慢是正常的）
docker build -t heart-echo-backend ./backend

# ② 单独构建 AI 服务镜像
docker build -t heart-echo-ai ./ai-service

# ③ 构建 nginx 镜像（它同时构建前端，上下文必须是仓库根目录）
docker compose -f deploy/docker-compose.yml build nginx
```

**报错怎么定位**——看输出里最后一行 `=> ERROR [stage N/M]` 的 stage 编号：

| 失败位置 | 常见原因 |
|----------|----------|
| 拉基础镜像阶段 | 网络问题，配置国内 Docker 镜像加速器 |
| `go mod download` | `go.sum` 没提交，或 `go.mod` 里 module 名与 import 路径不一致 |
| `npm ci` | `package-lock.json` 没提交 |
| `npm run build` | TS 编译报错 —— 先在本地跑 `npm run build` 确认能过 |
| `pip install` | `requirements.txt` 里有装不上的包 |
| `COPY` 找不到文件 | 路径写错，或 `.dockerignore` 把它排除了 |

> 一个实用判断：**构建失败时，先在本地（不用 Docker）把同样的命令跑一遍**。本地能过、容器里不过，才是 Docker 的问题；本地就不过，那是代码问题。
- [ ] 参与契约评审，确认人设/朋友圈/主动消息三组端点的字段
- [ ] 与成员 1 确认路由注册方式（[总纲 §4.5](MASTER.md#45-路由注册方式成员-1-冻结全员遵守)）
- [ ] 与成员 2 确认 `request.ts` 与 `types/api.ts` 的用法

**准备期末判据**：`go build ./...` 通过；`AutoMigrate` 跑完 10 张表全建出来；成员 1 确认 model 字段可用。

**表清单**（10 张，字段以 [技术文档 §6.2 DDL](../TECH_DESIGN.md#62-核心表-ddl) 为准）：`users` `personas` `chat_messages` `user_memory` `user_profile` `ai_moments` `moment_comments` `moment_likes` `proactive_settings` `schedules`。

> **没有会话表**：一个人设只有一个对话，`chat_messages` 直接挂 `persona_id`，`personas.last_message_at` 供列表排序与主动消息空闲判定。
> **记忆与画像也是「一人设一份」**：`user_memory` 与 `user_profile` 都要带 `persona_id`。`user_id` 保留，但它的作用是越权防线，不是分区键。
> `memory_type` 只有 `fact` / `preference` / `event` 三种，**没有 `emotion`**——情绪是内部信号，不作为长期记忆存储。

> `user_memory` 的 `embedding_id` / `embedding_status` 阶段一保留为空，不要删——阶段二接 ChromaDB 时无需改表。

---

## 2. Week 1：人设 CRUD 后端

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 人设列表 | `handler/persona_handler.go` + `service/persona_service.go` + `repository/persona_repo.go` | 只返回当前用户的 |
| 创建人设 | 同上 | 名字/性格/说话风格必填，缺失返回 4001 |
| 编辑人设 | 同上 | `PUT /personas/:id` |
| 删除人设 | 同上 | 级联删除其全部消息 |
| 越权校验 | service 层 | 改别人的返回 4031 `ErrPersonaNotOwned` |
| 路由注册 | `RegisterPersonaRoutes` | 通知成员 1 挂到 `router.go` |
| DTO | `dto/persona_dto.go` | JSON tag 用 camelCase |

**关键要求**

- **所有涉及 `persona_id` 的查询必须带 `user_id` 条件**——不能只按 id 查。越权是红线（协作 §10.4 红线 4）。
- handler 只做绑定与上抛，错误一律 `_ = c.Error(errcode.New(...))`，不写 `response.Fail`。
- 人设删除用外键 `ON DELETE CASCADE` 自动清理，不要手写多表删除逻辑。

**Week 1 末里程碑**：注册 → 登录 → 刷新保持登录态，前后端联调成功（你提供的人设接口参与其中）。

---

## 3. Week 2：人设管理页面

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 人设列表页 | `views/persona/PersonaView.vue` | 卡片展示，可进入对话 |
| 创建/编辑表单 | 同上 + `components/persona/` | 名字、性格描述、说话风格 |
| 删除确认 | 同上 | 二次确认后删除 |
| API 封装 | `api/persona.ts` + `stores/persona.ts` | 复用成员 2 的 `request.ts` |
| 类型定义 | `types/persona.ts` | 与后端字段对齐 |

**注意**：页面挂在成员 2 预留的 `/personas` 路由下，不要改路由结构；组件放 `views/persona/`，不要新建平行目录。

**Week 2 末**：若流式对话未跑通（生死线），立即在群里接受砍功能决策——主动消息与朋友圈可能被砍，你要准备转向支援部署。

---

## 4. Week 3：主动消息

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 配置表与 CRUD | `proactive_settings` 的 repo/service/handler | `GET/PUT /proactive/settings`（`personaId` + 可改字段） |
| 定时扫描 | `service/proactive_job.go` | 每 5 分钟扫描一次，按 `personas.last_message_at` 判空闲 |
| 触发逻辑 | `ProactiveService.TriggerNow(personaID)` | 检查沉默时长、日限额、最近回复 |
| 手动触发接口 | `POST /proactive/trigger` 🚨 | **与定时任务复用同一段逻辑** |
| 配置页 UI | 设置页 / 人设页内的表单 | **三个控件都要给**：开关、间隔区间（min/max）、每日上限 |
| 侧栏红点 | 前端 store + 对话侧栏 | 有未读主动消息时对应人设旁显示红点，打开后清除 |
| 防骚扰 | service 层 | 每日上限（可配）、可关闭、1 小时内回复过不触发 |

**实现要点**

- 主动消息只负责**制造一次对话机会**：向该人设的对话注入 `role='user'`、`is_nudge=true` 的 `[nudge]` 消息，然后走原本的聊天链路生成回复。**不要为主动消息单独写一套生成逻辑**。
- prompt 里要告知模型 `[nudge]` 不是用户刚打的字（技术文档 §5.2 prompt 第 4 条）。
- 手动触发接口**不是作弊**，是标准可测试性设计。答辩现场不可能等 30 分钟。
- 定时任务与手动接口必须调**同一个** `TriggerNow`，否则演示通过、线上逻辑却是另一套。

**Week 3 末里程碑**：核心演示链路全通。

---

## 5. Week 4：朋友圈 + 部署

### 5.1 AI 朋友圈

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 动态表与 CRUD | `moment_repo.go` + `service/moment_service.go` | `GET /moments` |
| 生成动态 | `service/moment_job.go` | **定时任务生成，没有手动触发接口** |
| 评论 | 同上 | AI 评论与用户评论二选一（数据库 CHECK 约束） |
| 点赞 | 同上 | `POST /moments/:id/like`，**幂等**：靠 `moment_likes` 的 `UNIQUE (user_id, moment_id)`，重复点不累加 |
| 前端页面 | `views/moments/MomentsView.vue` | 列表 + 点赞 + 评论（**没有发动态按钮，用户不能发**） |
| 评论策略 | 阶段一模板，阶段二 Agent 自主 | 由服务内策略切换 |

- 动态生成要注入该 AI 的人设 + 最近对话提取的「生活事件」，让动态与陪伴关系呼应（技术文档 §5.5）。
- **`ai_moments.emotion_label` 不返回给前端**：它只用于决定生成语气，`Moment` 响应体里没有这个字段，别顺手加回去。
- **点赞表是 `moment_likes`**：写入用 `INSERT ... ON CONFLICT DO NOTHING` 再自增 `like_count`，`UNIQUE (user_id, moment_id)` 保证重复点击不累加。
- **可见范围是账号内**：`GET /moments` 只返回当前用户自己的人设的动态；评论里出现的其他 AI 也必须是**同账号**的人设。不要写跨用户的查询。
- **没有手动触发按钮**（这是明确定下来的产品决策，不是遗漏）。现场演示靠两件事兜底：
  1. 定时任务间隔用环境变量 `MOMENT_JOB_INTERVAL` 控制，演示环境调到 2 分钟；
  2. **演示前提前灌好一批历史动态**，保证页面点开就有内容，不依赖定时任务刚好触发。

### 5.2 部署

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 生产 compose | `deploy/docker-compose.yml` | 4 个容器（nginx / backend / ai-service / postgres），`ps` 全部 healthy |
| Nginx（兼前端托管） | `deploy/Dockerfile` + `deploy/nginx.conf` | 镜像内已含前端 dist；SSE 关缓冲 + history 模式 + `/api` 转发 |
| 服务器初始化 | 云服务器（国内） | Ubuntu 22.04，安全组只开 22/80（**不开 443**） |
| 生产 `.env` | 服务器上 | 权限 `600`，强密码，**永不入库** |
| 首次部署 | — | 按技术文档 §9.5 检查清单逐项过 |
| 公网验证 | — | 手机 4G 打开 **`http://<公网IP>`** 走完整演示脚本 |

> **不买域名、不做 HTTPS、不备案**：硬性要求只要求"公网可访问"，IP 直访完全满足。国内服务器绑域名必须 ICP 备案（2-3 周），时间成本与收益不匹配。演示地址就是 `http://123.45.67.89` 这种形式。

**Nginx 两条关键配置**（最容易翻车）：

```nginx
location / {
    try_files $uri $uri/ /index.html;      # Vue Router history 模式
}
location /api/ {
    proxy_buffering off;                    # SSE 不流式的元凶
    proxy_read_timeout 300s;
}
```

**安全约束（不可妥协）**：数据库、ChromaDB、AI 服务端口**一律不对外开放**，只在 Docker 内网互通；生产禁止默认密码；`.env` 权限 600 且不入库。

### 5.3 日程提醒（P1，有余量才做）

> **先看这条**：日程提醒排在 P1 队尾，**朋友圈做完、部署验证通过之后**才开工。进度紧就直接不做——它**不受 [总纲 §0.3](MASTER.md#03-功能范围) 保护**（砍功能顺序里它排第 ⑤，在朋友圈之前就砍掉了）。**不要因为它拖慢部署。**

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 表与 model | `internal/model/schedule.go` | `AutoMigrate` 建出 `schedules`（你在准备期就该把 struct 定好） |
| 列表 / 取消 / 触发 | `service/schedule_service.go` + `repository/schedule_repo.go` + `handler/schedule_handler.go` | 三个端点按契约 §10 工作（**没有新建端点**，日程只能从对话抽取） |
| 定时扫描 | `proactive_job.go` 里加一段 | 扫 `status='pending' AND remind_at <= NOW()`，走与 `TriggerNow` 同款的注入逻辑 |
| 手动触发接口 | 同上 | `POST /schedules/:id/trigger` 🚨，**与定时任务复用同一段逻辑** |
| 前端列表页 | `views/schedule/ScheduleView.vue` + `api/schedule.ts` + `types/schedule.ts` | 按人设筛选、显示待提醒/已提醒、可取消 |

**实现要点**

- **它不是第 4 个 Agent，也不是新管线**：到点后注入一条 `[nudge]` 消息，然后走成员 1 原有的聊天链路。**不要为提醒单独写一套生成逻辑**——那会和主动消息维护两份。
- **`GET /schedules` 必须带 `personaId`**，且查询同时过滤 `persona_id` + `user_id`（和记忆一样的越权防线）。
- **取消是软删**：`DELETE` 置 `status='cancelled'`，不物理删。
- **规则时间解析归成员 1**（他负责抽取），你只负责**存绝对时间**（`remind_at`）与**到期触发**。解析结果如果拿不到，就是成员 1 那边回问用户，你这边不建行。
- **演示前实测 `/schedules/:id/trigger` 可用**，并准备一条 1 分钟后到期的日程做现场演示（时间太近会来不及切页面，1 分钟刚好）。

---

## 6. 你的接口契约（对外发布）

| 端点 | 请求要点 | 响应要点 |
|------|----------|----------|
| `GET /personas` | `page`/`pageSize` | `PageResult<Persona>`（**是分页对象，不是数组**，见契约 §4） |
| `POST /personas` | `{name, personalityDesc, speakingStyle}` | 新建的 Persona |
| `PUT /personas/:id` | 同上 | 更新后的 Persona |
| `DELETE /personas/:id` | — | `data: null` |
| `GET /moments` | `page`/`pageSize` | 动态 + 评论数 + 点赞数 + 我是否点过（**只返回本账号的**） |
| `POST /moments/:id/like` | — | `{likeCount, liked}`，重复点幂等 |
| `GET/POST /moments/:id/comments` | `{content}` | 评论列表 / 新评论 |
| `GET/PUT /proactive/settings` | `{personaId, enabled, intervalMin, intervalMax, dailyLimit}` | 配置 |
| `POST /proactive/trigger` | `{personaId}` | 触发生成的消息 |
| `GET /schedules`（**P1**） | `personaId`（**必带**）+ `page`/`pageSize` | `PageResult<Schedule>` |
| `DELETE /schedules/:id`（**P1**） | — | `data: null`（软删，置 `cancelled`） |
| `POST /schedules/:id/trigger` 🚨（**P1**） | — | `{messageId, content, createdAt}`，与定时任务同逻辑 |

**错误码**：越权 `4031`、人设不存在 `4043`、资源不存在（含日程不存在）`4040`、参数错误 `4001`。**日程提醒不新增错误码**——同一个语义不开两个码。完整表见 [技术文档 §7.4](../TECH_DESIGN.md#74-错误码总表)。

**契约规则**：字段名改一个字母都要在群里广播，并在 [API_CONTRACT.md](../API_CONTRACT.md)「变更记录」中登记。你负责的三组端点（人设 / 朋友圈 / 主动消息）在准备期要逐条确认。

---

## 7. 常见坑

| 坑 | 现象 | 处理 |
|----|------|------|
| 越权查询 | 只按 `persona_id` 查，能改到别人的 | 所有查询加 `user_id` 条件 |
| 路由冲突 | 三人同时改 `router.go` | 用 `RegisterXxxRoutes`，改前群里说 |
| AutoMigrate 误用 | 想删列结果没删 | 它不删列，是安全默认；不要用 `DropTable` |
| 外键顺序 | AutoMigrate 报依赖错误 | 按 `User → Persona → Message → ...` 顺序传 |
| 记忆串号 | A 人设记起 B 人设的事 | `user_memory` / `user_profile` 带 `persona_id`，查询同时过滤 `persona_id` + `user_id` |
| 朋友圈想加"手动生成"按钮 | 产品决策里明确没有 | **不要加**（[总纲 §0.3](MASTER.md#03-功能范围)）。演示靠提前灌数据 + 调短 `MOMENT_JOB_INTERVAL` |
| Nginx 缓冲 | 生产不流式 | `proxy_buffering off` |
| history 404 | 刷新子路由白屏 | `try_files` |
| 端口暴露 | 数据库公网可连 | 只映射 80，其余走内网 |
| `.env` 入库 | 密钥泄漏 | 提交前 `git status` 自查；误提交立刻轮换密钥 |
| 容器内连不上 DB | 用了 `localhost` | 容器内主机名是服务名 `postgres`，不是 localhost |
| CHECK 约束报错 | 评论作者为空 | `persona_id` 与 `user_id` 必须恰好有一个非空 |
| compose 路径解析错 | `Cannot locate Dockerfile` | 相对路径以 **compose 文件所在目录**为基准，文件在 `deploy/` 就要写 `../backend` |
| `npm ci` 失败 | 部署构建中断 | `package-lock.json` 必须提交进仓库 |
| 部署后前端请求 `localhost:8080` | 页面能开但接口全挂 | Vite 环境变量是**构建时**注入的，`VITE_API_BASE_URL` 要在 Dockerfile 里用 `ARG` 传 `/api/v1`；运行期改 `.env` 无用 |
| 容器内服务连不上 | connection refused | 服务必须监听 `0.0.0.0`，不能是 `127.0.0.1` |
| 构建上下文过大 | build 极慢 / 可能泄漏密钥 | 根目录必须有 `.dockerignore`，排除 `node_modules`、`pg_data`、`.env`、`.git` |

---

## 8. 每周自检

- [ ] `go build ./...` 通过；`docker compose config` 无报错
- [ ] 我的提交都 push 了，commit 符合 `<type>(<scope>): <subject>`
- [ ] 所有查询带 `user_id` 过滤；记忆/画像查询**还带了 `persona_id`**
- [ ] 朋友圈查询没有跨账号泄漏——`GET /moments` 只返回本用户人设的动态
- [ ] 点赞是幂等的：连点两次 `likeCount` 不变（`moment_likes` 的唯一约束生效）
- [ ] `backend` 服务读得到 `MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL`（定时任务在 Go 侧），`backend/.env.example` 与 `deploy/.env.example` 里都有这两项
- [ ] 没有把 `.env`、`pg_data/`、模型权重提交进仓库
- [ ] 生产 `.env` 权限 `600`，安全组只开 22/80
- [ ] `/proactive/trigger` 演示前实测可用，且与定时任务复用同一逻辑
- [ ] 朋友圈**演示前已灌好历史动态**（没有手动触发接口，不能指望现场生成）
- [ ] （若做了 P1）`/schedules/:id/trigger` 演示前实测可用；`GET /schedules` 带了 `personaId` 且查询过滤了 `user_id`
