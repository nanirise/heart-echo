# PROGRESS

最后更新：2026-09-13

## 成员1
- 在做：`feature/backend-skeleton`（spec: `docs/specs/backend-skeleton/`）。plan 的 10 步完成 2 步——`pkg/errcode`、`pkg/response` 已随 PR #12 合入 develop
- 下一步：Step 4 `pkg/logger`（读 `GIN_MODE` 区分 JSON / 人类可读）→ Step 5 `pkg/jwt` → Step 6 `internal/config` → Step 7 中间件 ×5 → Step 8 `router.go` + `cmd/server/main.go` → Step 9 `.env.example` → Step 10 冒烟验收
- 卡住：无。等成员 2、成员 3 确认错误码规则的同步（见下方「待确认」）
- 改了哪些文件：
  - 已合入 develop：`backend/pkg/errcode/{errcode.go,errcode_test.go}`、`backend/pkg/response/response.go`
  - 全局契约：`AGENTS.md` §4.3、`docs/API_CONTRACT.md` §2/§4/§5/§7/§9/§10、`docs/TECH_DESIGN.md` §4.3/§7.4
  - 本功能文档：`docs/specs/backend-skeleton/{spec.md,plan.md}`

## 成员2
- 在做：
- 下一步：
- 卡住：
- 改了哪些文件：

## 成员3
- 在做：
- 下一步：
- 卡住：
- 改了哪些文件：

## 接口/数据结构变更
- 有。
  - 改了什么：错误码 `4031`（原「该人设不属于当前用户」）**废弃**。资源越权（访问他人 persona / 日程）统一返回 `4043`「人设不存在」+ HTTP 404；`4030` 收窄为**功能越权**专用，文案由「无权限访问该资源」改为「无权限使用该功能」
  - 涉及文件：`backend/pkg/errcode/errcode.go`、`docs/API_CONTRACT.md`、`AGENTS.md`、`docs/TECH_DESIGN.md`
  - 其他人要做什么：pull 最新 develop；归属校验**不要再返回 `4031`**；前端错误分支若按 `4031` 写过要改

## 待确认
- 错误码规则已于 2026-09-13 定为：**资源越权 → `4043`**、**功能越权 → `4030`**、**`4031` 废弃**。以下两处仍是旧规则，等 Owner 自行更新：
  - 成员 2：`docs/specs/persona-model/{spec.md,plan.md}` 按 `4040`/`4031` 写（含 `plan.md:277` 的 grep）
  - 成员 3：`docs/dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md` 第 83、223 行仍写 `4031`