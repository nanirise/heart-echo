# plan · 前端数据契约层

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/frontend-api-types`
> 负责人：成员 2 ｜ 最后更新：2026-09-14

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | 响应体类型 | `src/types/api.ts` | 与 `Response[T]` 四字段逐字段对齐 | 未开始 |
| 2 | 分页类型 | `src/types/api.ts` | 与契约 §1 分页结构对齐 | 未开始 |
| 3 | 错误码常量 | `src/types/errcode.ts` | 19 个业务码与 `errcode.go` 逐条一致，无 4031 / 4042 | 未开始 |
| 4 | 错误码文案映射 | `src/types/errcode.ts` | 文案与 `codeMessages` 逐字一致 | 未开始 |
| 5 | 环境变量示例 | `frontend/.env.example` | 含 `VITE_API_BASE_URL` | 未开始 |
| 6 | 自查 | — | `tsc --noEmit` 与 `build` 双通过 | 未开始 |
| 7 | 进度同步 | `PROGRESS.md` 成员 2 段 | 四栏填写完整 | 未开始 |

**优先级说明**：Step 1-2 优先于其余全部。`types/api.ts` 是准备期交接物 #8，成员 3 的 `api/persona.ts` 在等它，越早交付并行度越高。

## 2. 文件清单

| 路径 | 作用 | 谁会用到 |
|---|---|---|
| `frontend/src/types/api.ts` | 响应体与分页泛型 | 成员 3（写 `api/persona.ts`）、成员 2 后续分支 |
| `frontend/src/types/errcode.ts` | 业务错误码常量与文案映射 | 成员 3、成员 2 后续分支 |
| `frontend/.env.example` | 环境变量样例 | 全员 |
| `docs/specs/frontend-api-types/spec.md` | 本功能规格 | 审计用 |
| `docs/specs/frontend-api-types/plan.md` | 本功能计划 | 审计用 |
| `PROGRESS.md` | 全员进度看板 | 全员 |

## 3. 关键实现要点

### 3.1 响应体类型（Step 1-2）

`ApiResponse<T>` 与后端 `backend/pkg/response/response.go` 的 `Response[T]` 一一对应：

| 后端字段 | Go 类型 | 前端类型 | 说明 |
|---|---|---|---|
| `code` | `int` | `number` | 业务码，非 HTTP 状态码 |
| `message` | `string` | `string` | 后端已给中文文案 |
| `data` | `T` | `T` | 泛型参数 |
| `timestamp` | `int64` | `number` | 毫秒时间戳 |

`timestamp` 用 `number` 是安全的：Go 的 `int64` 范围远超 JS 安全整数（2^53），但它是毫秒时间戳（约 1.7e12），远小于 2^53，不会精度丢失。

`T` 默认 `unknown` 而非 `any`：项目禁 `any`。用 `unknown` 强制调用方显式收窄，`any` 会让类型检查直接失效。

### 3.2 错误码（Step 3-4）

权威来源是 `backend/pkg/errcode/errcode.go`，不是 `API_CONTRACT.md` 的文字表。

三处必须对齐：

| 来源 | 内容 |
|---|---|
| 常量块 | 19 个业务码（含 `Success = 200`） |
| `codeMessages` | code 到中文文案 |
| `codeHTTPStatus` | code 到 HTTP 状态码 |

不得出现 `4031` / `4042`，两者均已废弃，`errcode.go` 里连常量都不存在。

`4043` 一条码承担两种语义（人设不存在 + 资源越权），前端不区分，统一按「人设不存在」处理。

文案映射的定位是兜底：后端 `Fail()` 已经返回中文 message，前端优先用后端的；映射只用于「后端没给」或「本地预判」的场景。

### 3.3 环境变量（Step 5）

| 变量 | 开发值 | 生产值 |
|---|---|---|
| `VITE_API_BASE_URL` | `http://localhost:8080/api/v1` | `/api/v1` |

注意带 `/api/v1` 后缀。分支 1 的 plan §3.2 写的是 `http://localhost:8080`（不带后缀），以本文件为准。契约 §1 规定 Base URL 含 `/api/v1`。

> 这是本 plan 与分支 1 plan 的一处已知冲突，需在群里说明；或由分支 3 写 `request.ts` 时决定在哪一层拼。

### 3.4 环境变量的编译期特性

`VITE_` 前缀的变量在 `vite build` 时被静态替换，不是运行时读取。所以：

- `.env.example` 只放样例值，真实 `.env` 不进库
- 变量值改变后必须重新 build，dev 模式下要重启 `npm run dev`

## 4. 风险与对策

| 风险 | 对策 |
|---|---|
| 契约 §13 三人签署未完成，字段可能再变 | 类型集中在 `types/`，变更时只改一处；§13 签署后本分支类型视为冻结 |
| 后端新增错误码，前端表滞后 | errcode 表以 `errcode.go` 为准，后端改动需广播；建议在群里约定「新增码必须同步通知前端」 |
| 错误码文案抄错（逐字比对易出错） | 写完用脚本比对两份文件的文案，不靠肉眼 |
| 手抄 Markdown 混入零宽字符 | 提交前跑 `` 检查（PowerShell 字节级） |
| 成员 3 等不及先自己写了类型 | 先把 `types/api.ts` 推上去（Step 1-2 优先），并在群里通知 |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-14 | 创建 spec 与 plan | 无 |
