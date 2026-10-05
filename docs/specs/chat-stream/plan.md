# plan · 流式对话（chat-stream）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/chat-stream-ai-service` → `feature/chat-stream-llm` → `feature/chat-stream-chat-chain`
> 负责人：成员 1 ｜ 最后更新：2026-10-05

---

## 1. PR 划分

一个 PR = 一件**能独立验证、能单独 review** 的事。切分原则见 [spec §0](../../../AGENTS.md)。

| # | 分支 | 覆盖 spec | 内容 | 状态 |
|---|---|---|---|---|
| **0** | （`chat-message` 的分支） | — | `message_repo.Create` | ⛔ **硬前置，不属本 spec** |
| **1** | `feature/chat-stream-ai-service` | §1.7、§2.2 | `ai-service/` 骨架 + `X-Internal-Token` 校验 + **假流式**（不接 DeepSeek） | ✅ **PR #51 已合并（2026-10-01）** |
| **2** | `feature/chat-stream-llm` | §1.3、§2.2 | 接 DeepSeek 真流式（`core/llm_client.py`） | ✅ **已并入 `develop`** |
| **3** | `feature/chat-stream-chat-chain` | §1.1–§1.5、§2.1、§2.3 | Go 侧 `ai_client.go` + `chat_dto.go` + `chat_service.go` + `chat_handler.go` + 挂载 | ✅ **PR #62 已合并（2026-10-05，合并点 `074bde5`）** |

### 1.1 为什么 PR 1 要「假流式」

**先把"能不能流"和"AI 能不能答"拆开验。**

假流式 = 不调 DeepSeek，自己按固定文本每 100ms 吐一个 `delta`。好处：

- **不烧 API 余额**，不怕 DeepSeek 抽风，可以随便重跑
- 排错时能**确定问题不在 LLM 侧** —— 生死线最大的不确定项是 SSE 管道本身，不是模型
- `/health` 和鉴权头也一并验证掉

PR 1 通过判据（[spec §4 A 第 4 步](spec.md)）：
```bash
curl -N -X POST http://localhost:8000/internal/chat/stream \
  -H "X-Internal-Token: $AI_SERVICE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user_id":1,"persona_id":1,"message":"你好"}'
```
**看到 `event:` / `data:` 逐条输出**（不是最后一次性吐出）。这一步不过，PR 2/3 都不用开工。

> ⚠️ **Windows 上这条命令里的中文要换成 ASCII** —— `curl.exe` 的 argv 走 ANSI 代码页（中文 Windows 是 GBK），内联中文会变成非法 UTF-8、服务端返回 `400`。把 `"你好"` 换成 `"hello"`，或写成文件后 `--data-binary @body.json`。详见 [spec §4](spec.md) 与 [PROGRESS 遗留 12](../../../PROGRESS.md)。

---

## 2. 步骤

| # | 步骤 | 涉及文件 | 状态 |
|---|---|---|---|
| 0 | 与成员 3 敲定 `chat_service.go` / `chat_handler.go` 的文件归属（[spec §3.2](spec.md)） | — | ✅ 归成员 1（`chat-message` 的 `message_repo.go` / `chat_dto.go` 由成员 1 一并落下） |
| 1 | `ai-service/` 骨架：`main.py` / `api/routes.py` / `core/config.py` / `core/security.py` + `.env.example` | 新增 5 个 | ✅ PR #51 |
| 2 | `/internal/chat/stream` 假流式 + `/health` | `api/routes.py` | ✅ PR #51 |
| 3 | `core/llm_client.py` 接 DeepSeek 流式 | 新增 | ✅ 已并入 `develop` |
| 4 | Go 侧 `AIClient` 接口 + HTTP 实现 | `internal/service/ai_client.go` | ✅ PR #62 |
| 5 | `StreamChatRequest` DTO | `internal/dto/chat_dto.go` | ✅ PR #62 |
| 6 | `chat_service.go`：判归属 → 落 user → 调 AI → 转发 → 落 assistant → 补发 `done` | `internal/service/chat_service.go` | ✅ PR #62 |
| 7 | `chat_handler.go`：SSE 响应头 + 逐事件 Flush + `RegisterChatRoutes` | `internal/handler/chat_handler.go` | ✅ PR #62 |
| 8 | `router.go` 挂到 `protected` 组（一行） | `internal/handler/router.go` | ✅ PR #62 |
| 9 | 四层分段联调 + 浏览器打字机 | — | 🟡 **第 4、5 步已过（2026-10-05 实测）；第 6 步（Nginx）无部署、第 7 步（浏览器渲染）未做** |

> ⚠️ 步骤 8 按 [MASTER §4.5](../../dev/MASTER.md) 约定：`router.go` **不要两人同时改**，挂载行由我在群里说一声后加。

---

## 3. 关键实现片段

### 3.1 顺序不能反（[spec §1.1](spec.md)）

```go
// ❌ 错的顺序：先写了 200，4043 就发不出去了
c.Writer.WriteHeader(http.StatusOK)
persona, err := s.personaRepo.FindOwned(ctx, userID, personaID)
if err != nil {
    _ = c.Error(err) // BizErrorHandler 想回 404，但响应头已经写出去了
}

// ✅ 对的顺序：校验全过再进 SSE
persona, err := s.personaRepo.FindOwned(ctx, userID, personaID)
if err != nil {
    _ = c.Error(err) // 这里还能正常回 4043
    return
}
// ... 落 user 消息 ...
// 到这里才写 SSE 头
```

### 3.2 Go 补发 `done`（[spec §1.3](spec.md)）

```
for each event from Python:
    delta  → 转发给前端 + Flush
    end    → 吞掉，不转发
落库 assistant → 拿到 messageId → 发 event: done
```

### 3.3 已定 / 待补

**已定（PR #62 实际落地）**：SSE 用**裸 `c.Writer` + `WriteHeader` + 每事件 `Flush()`**，不用 `c.Stream()`。两者都能写出流式响应；选前者是因为数据源是一个 channel，`for ev := range ch` 与它直接对应，套进 `c.Stream` 的 `step` 回调只会把转发逻辑塞进闭包。

**🚧 待补 —— Go 侧自动化测试目前一个都没有**：

- `chat_handler.go` 的测试怎么写（现有 `auth_handler_test.go` / `router_test.go` 的分工可参照）
- 越权用例怎么钉（`4043` 必须走普通 JSON 而非 SSE，参照 `auth_handler_test.go` 的反向注入手法）
- 事件流的断言：只有 `delta` / `done` / `error` 三型、`end` 不透传、分隔符是 `\n\n`

> 2026-10-05 的联调是**手工 curl** 做的（结论见 §5），**没有留下可回归的测试**。这是本功能目前最大的欠账。

---

## 4. 风险

| 风险 | 应对 |
|---|---|
| **SSE 联调排错占三分之一工时**（[PROGRESS](../../../PROGRESS.md) 评估） | 严格四层分段，每层验证一次。「字到了不刷新 / 一次性吐完 / 被中间件缓冲住」只能真跑起来才看得见 |
| **`message_repo` 未落地**（spec §3.2） | PR 1 / PR 2 不依赖它，**可以先做**；PR 3 的落库那一步必须等 |
| DeepSeek 不稳定 | 超时重试 2 次 + 兜底话术（[COLLABORATION §10.1](../../COLLABORATION.md)） |
| `gin` 的响应缓冲 | `Flush()` 与 `X-Accel-Buffering` 两个都要，见 spec §1.4 |

---

## 5. 进度

| 日期 | 事件 |
|---|---|
| 2026-09-30 | spec / plan 骨架创建。四条决策已拍板：功能名 `chat-stream`、内部路径加 `/internal` 前缀、内部请求体 snake_case、Python 出 SSE 由 Go 逐事件转发（结尾 `done` 由 Go 补发） |
| 2026-10-01 | **PR 1 合并（PR #51）**：`ai-service/` 骨架 + 假流式，12 文件 / 274 行。实测 `pytest` 6 passed、`curl -N` 14×`delta` + 1×`end`、行尾纯 LF（`cat -A` 无 `^M`）。合并前把 `core/config.py` 的 `extra="ignore"` 改回默认 `forbid` |
| 2026-10-04 | 交接核对：PR 1 的四个 commit 均在 `origin/develop` 上（合并点 `09a0e7b`），`ai-service/` 12 文件在库。本文件的状态表当时漏勾，本次补上 |
| 2026-10-05 | **PR 3 合并（PR #62，合并点 `074bde5`）——本 spec 的代码部分全部落地**。分区 2 / 3 状态表与步骤 0、3–9 一并补勾 |
| 2026-10-05 | **四层联调第 4、5 步实测通过**。7 段 `delta` 到达时刻 0.00 / 0.25 / … / 1.50 秒（分散到达 = 真流式）；`end` 未透传；`done {"messageId":3}` 与库里 `chat_messages.id=3`（`role=assistant`）对得上；越权 → `4043` + HTTP 404 + `application/json`；`last_message_at` 与最后一条消息 `created_at` SQL 比对为 `true`。**用的是假上游**（本机 `.env` 的 `DEEPSEEK_API_KEY` 仍是占位符），所以验的是链路不是 DeepSeek 集成。第 6 步（Nginx）无部署、第 7 步（浏览器渲染）未做 |
