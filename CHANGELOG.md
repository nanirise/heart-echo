# Changelog

本文件记录 HeartEcho 的发版变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 项目全貌见 [README.md](README.md) ｜ 接口字段见 [docs/API_CONTRACT.md](docs/API_CONTRACT.md) ｜ 功能决策见 [docs/dev/MASTER.md](docs/dev/MASTER.md)

---

## [2.0.0] - 2026-10-30 · 已封版

第一次正式发版，也是本项目的结档版本。核心链路全部打通并部署上线，阶段二升级超出原计划范围。

### 新增功能

- **多套主题（五套）**
  - 亮色 `light`、暗黑 `dark`、青春疼痛 `pain`、星夜 `custom-zhang`、薄雾青 `custom-aqua`
  - 侧栏顶部按钮**循环切换**，选择写入 `localStorage`（键 `heart-echo-theme`），刷新保持
  - **暗黑主题复用 Element Plus 官方暗色变量**（`element-plus/theme-chalk/dark/css-vars.css`）
  - 星夜 / 薄雾青带背景图的定制主题，自行补齐 Element Plus 的 fill / border / mask / 阴影派生色
  - `index.html` 预置脚本在 Vue 挂载前先挂类名，**避免首屏闪白**
- **智能朋友圈（LLM 生成）**
  - 动态正文由 `ai-service/app/moments/moment_agent.py` 调 LLM 生成，人设、说话风格、近期事件都进 prompt
  - `MOMENT_GENERATOR=ai` 为默认档，**LLM 调用失败自动回退模板生成**，不让定时任务挂掉
  - 同账号下的其他 AI 角色评论互动；用户只能点赞 / 评论，**不能自己发动态**
  - **刻意没有手动触发接口**（动态只由定时任务产生，见 [API_CONTRACT §8](docs/API_CONTRACT.md)）
- **用户画像（LLM 提取并落库）**
  - LLM 结构化提取 `profile_updates`（职业、兴趣、近期事件）→ Go 合入 `user_profile` → `GET /profile/portrait` 读取
  - **隐式表达也能提取**（"最近老加班到十点"），规则档做不到
  - 一人设一份，按伴侣切换查看；空画像是合法空态（`{}` + `updatedAt: null`）
- **日程提醒（P1，提前完成）**
  - 对话里说「明天下午三点提醒我开会」→ 抽取并落库 → 到点主动提醒
  - 时间解析规则档；**解析不出会回问澄清，不静默丢弃**
  - 演示依赖 `POST /schedules/:id/trigger`（与定时扫描同源复用 `TriggerNow`）
- **主动消息**
  - 开关、间隔区间、每日上限全部可调；消息进对话 + 侧栏红点
  - 手动触发 `POST /proactive/trigger`，与定时任务同源
- **亲密度（人格演化）**
  - `personas.state.familiarity` 拍平为响应字段 `familiarity`，每次回复 +1、封顶 100
  - 按区间映射为关系阶段注入 prompt，说话逐渐更随意亲近

### 技术重构

- **向量检索：接入 ChromaDB**
  - `ai-service/app/memory/vector.py` 通过 `chromadb.HttpClient` 接入，服务端 `chromadb/chroma:0.6.3`（与客户端同版本）
  - **使用 Chroma 默认的 ONNX MiniLM（all-MiniLM-L6-v2）做向量化**，不引入 `sentence-transformers`
  - 检索**必须同时带 `user_id` + `persona_id`** 两个过滤条件（`persona_id` 全局自增，少一个就跨用户/跨人设串号）
  - 向量库不可用时**降级为无记忆，对话主链路照常**；`MEMORY_BACKEND=recent` 仍为默认兜底档
- **记忆提取：规则 → LLM**
  - `MEMORY_EXTRACT_BACKEND=llm` 走 LLM 结构化提取，`POST /memory/extract` 只返回候选不落库，归属由 Go 侧赋值
  - 落库后经 `POST /memory/store` 同步写 ChromaDB，`id` 用 PG 主键作文档 ID
  - LLM 返回非法 `memory_type`（如 `emotion`）**在落库前丢弃并记日志**，不像样地改成 `fact`
- **异步任务架构**
  - 对话链路：用户消息落库后 `go s.asyncExtractMemory(...)` / `go s.asyncExtractSchedule(...)` 异步提取，**不阻塞 SSE 回复**
  - 后台任务：Go 侧 `time.NewTicker` 驱动的 `MomentJob` / `ProactiveJob` / `ScheduleJob`（**不是 Python APScheduler**，也不是 cron 表达式）
  - 所有异步失败**只记 warn、不影响主链路**；主动消息与日程扫描共用 `PROACTIVE_JOB_INTERVAL`
- **部署：国内服务器离线模型搬运**
  - chromadb 首次启动需下载约 243MB 的 ONNX 模型，国内云服务器拉取不稳定
  - 方案：本地打包 → `scp` → 服务器 `docker cp` → 挂 `chroma_cache` 卷持久化，**容器重建不必重新下载**
  - 详见 [TECH_DESIGN §9.6](docs/TECH_DESIGN.md#96-国内服务器离线模型搬运方案2026-10-20新增)
- **工程约束加固**
  - 统一响应 `{code, message, data, timestamp}`；错误码单一数据源，`Fail()` 不接受自定义文案
  - 资源越权一律按「资源不存在」返回 `4043`（废弃 `4031`），`4030` 收窄为**功能越权**专用
  - 前端 `strict: true` 类型零错误；Go 侧 `errcode_test.go` 校验三张 map 的 key 集合一致

### 已知限制

- **⚠️ 情感分析未实现为独立模块**：`ai-service/app/` 下没有 `classifiers/` 目录，也没有 `EMOTION_BACKEND` 环境变量。`chat_messages.emotion_label` / `emotion_score` 与 `ai_moments.emotion_label` 仅作为**预留槽位**（前端不渲染）。当前"情绪感知"由 LLM 理解上下文承担，不是 8 类分类器打分。
  - 验收口径：**不要宣称"情感分析 Agent 在后台识别情绪"**；改说"AI 会结合上下文调整语气"
  - 如需补齐：新增 `classifiers/` + `EMOTION_BACKEND` 开关，约 1-2 天
- **阶段二情感 ONNX 替换未启用**：`EMOTION_BACKEND=onnx` 不存在，情感侧仍是设计稿状态
- **P2 按计划不做**：语音对话、情绪趋势大屏可视化、消息编辑 / 撤回
- **朋友圈无手动触发接口**：这是**刻意设计**（契约 §8），演示靠灌数据或调短 `MOMENT_JOB_INTERVAL`
- **日程没有「新建」入口**：契约 §10 无 `POST /schedules`，日程只能从对话抽取

### 升级 / 部署注意

- **Go 版本要求提升**：`backend/go.mod` 为 `go 1.27.1`，低于此版本 `go build` 直接报错（README 早期写的 ≥1.21 已不适用）
- **`.env` 模板默认值不一致**：`backend/.env.example` 用 `heart_echo` / `change_me`，而 `deploy/.env.example` 是 `your_db_name` 之类的占位符——**两边必须手动对齐**，否则容器建的库和后端要连的库对不上
- **想用向量检索**：`ai-service/.env` 设 `MEMORY_BACKEND=vector`，并确认 `CHROMA_HOST` / `CHROMA_PORT` 指向正确（本地 uvicorn 用 `localhost:8001`，容器内用 `chromadb:8000`）
- **`.env` 与 `*.onnx` 模型文件严禁入库**（仓库只留 `.env.example`）

---

## 版本规划

| 版本 | 状态 | 说明 |
|------|------|------|
| v2.0.0 | ✅ 已封版（2026-10-30） | 结档版本，见上 |
| v1.x | — | 开发期内部迭代，未单独发版；阶段一（词典 / PG 关键词 / 规则提取）保留为可回退兜底档 |
