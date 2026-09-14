# spec · 前端数据契约层

> 功能名：frontend-api-types ｜ 分支：`feature/frontend-api-types`
> 负责人：成员 2（前端） ｜ 状态：进行中
> 创建：2026-09-14 ｜ 最后更新：2026-09-14
> 关联：[协作文档](../../COLLABORATION.md) ｜ [接口契约](../../API_CONTRACT.md) ｜ [技术文档](../../TECH_DESIGN.md)

---

## 1. 背景与目标

分支 1 的 spec 把 `src/types/api.ts`、`types/errcode.ts`、`api/request.ts` 一并列进了范围，
但该分支实际只交付了工程骨架，类型与请求层未落地。

本功能只交付类型契约层：把后端已有的 `Response[T]`、错误码表翻译成前端可 import 的 TS 类型。
请求层（`request.ts`）、路由表、`MainLayout.vue` 移交后续分支。

**目标（一句话）**：让成员 3 能 `import type { ApiResponse, PageResult } from '@/types/api'` 写他的 `api/persona.ts`，不必再等前端。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 响应体类型 | `src/types/api.ts`（`ApiResponse<T>`、`PageResult<T>`） |
| 错误码类型 | `src/types/errcode.ts`（`ErrorCode`、`ErrorCodeMessages`） |
| 环境变量示例 | `frontend/.env.example` |
| 进度同步 | `PROGRESS.md` 成员 2 段（搭车提交） |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 |
|---|---|
| `src/api/request.ts`（axios 实例 + 拦截器 + 4012 刷新重放） | 后续分支 |
| `src/router/index.ts`、`guards.ts` | 后续分支 |
| `src/layouts/MainLayout.vue` | 后续分支 |
| `src/stores/auth.ts` | 后续分支 |
| 各业务实体类型（`user.ts` / `persona.ts` / `chat.ts` / `moment.ts` 等） | 按各自功能分支追加 |

> 本功能不写任何运行时代码，只产出类型声明与环境变量示例。

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 提供方 | 状态 |
|---|---|---|
| `backend/pkg/response/response.go` 的 `Response[T]` | 成员 1 | 已合并 `29f1733` |
| `backend/pkg/errcode/errcode.go` 错误码表 | 成员 1 | 已合并 `29f1733` |
| 契约字段冻结（§13 三人签署） | 三人共同 | 未完成 |

### 3.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `src/types/api.ts` | 成员 3 | 写 `api/persona.ts` 时包裹响应体（准备期交接物 #8） |
| `src/types/errcode.ts` | 成员 3、成员 2 后续分支 | 错误分支判断 |
| `frontend/.env.example` | 全体 | 本地起前端的配置样例（准备期交接物 #7） |

## 4. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| TypeScript `strict`，禁止 `any`（用 `unknown` + 类型收窄） | 项目硬性要求 |
| `npx tsc --noEmit` 零错误、`npm run build` 通过 | 成员 2 自查清单 |
| 字段名全 camelCase，与后端 JSON tag 完全一致 | 协作文档 §6.4 |
| 错误码常量值必须与 `backend/pkg/errcode/errcode.go` 逐条一致 | 一 code 一 msg 约定 |
| 不得出现 `4031` / `4042`（均已废弃） | `errcode.go` 4030–4043 段注释 |
| `frontend/.env.example` 只放示例值，不含真实密钥 | 团队红线 |
| `.env` 永不入库 | 团队红线 |

## 5. 验收标准

- [ ] `src/types/api.ts` 导出 `ApiResponse<T>`：`code: number`、`message: string`、`data: T`、`timestamp: number`
- [ ] `src/types/api.ts` 导出 `PageResult<T>`：`list: T[]`、`total: number`、`page: number`、`pageSize: number`
- [ ] `src/types/errcode.ts` 含 19 个业务码常量，值与 `errcode.go` 逐条一致
- [ ] `src/types/errcode.ts` 不含 `4031`、`4042`
- [ ] `src/types/errcode.ts` 提供 code 到中文文案映射，文案与 `errcode.go` 的 `codeMessages` 逐字一致
- [ ] `frontend/.env.example` 含 `VITE_API_BASE_URL=http://localhost:8080/api/v1`
- [ ] `npx tsc --noEmit` 零错误，全项目无 `any`
- [ ] `npm run build` 通过
- [ ] `PROGRESS.md` 成员 2 段已填写

## 6. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-09-14 | 创建 | 从 `frontend-skeleton` 的 §2.1 拆出「数据契约」部分单独成支 |
