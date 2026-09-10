# HeartEcho 团队协作文档

> 版本：v0.1 ｜ 最后更新：2026-09-10
> 配套文档：[项目概览](../README.md) ｜ [技术文档](TECH_DESIGN.md)
>
> 本文档用于约束三人团队的日常协作方式。**所有成员在第一次提交代码前必须完整阅读一遍。**

---

## 目录

1. [团队与分工](#1-团队与分工)
2. [开发环境与前置准备](#2-开发环境与前置准备)
3. [Git 工作流](#3-git-工作流)
4. [Commit 规范](#4-commit-规范)
5. [PR 与 Code Review](#5-pr-与-code-review)
6. [命名规范](#6-命名规范)
7. [目录结构规范](#7-目录结构规范)
8. [协作节奏与沟通机制](#8-协作节奏与沟通机制)
9. [贡献度说明](#9-贡献度说明)
10. [风险应对与红线](#10-风险应对与红线)

---

## 1. 团队与分工

### 1.1 分工原则

**按功能模块切，不按技术栈切。**

每人负责**一条完整功能链路（前端 + 后端 + AI）**，而不是"一个人只写前端、一个人只写后端"。理由：

- 避免"某人只做前端"导致贡献单薄、成长有限
- 每条链路有明确 Owner，出问题知道找谁
- 三人提交记录自然均衡，避免"前端页面多、后端接口少"造成的贡献度偏差

### 1.2 分工表

| 成员 | 模块 | 具体内容 | 主要产出文件 |
|------|------|----------|--------------|
| **成员 1** | Go 框架 + 鉴权 + 对话核心 + AI 服务 | Go 项目骨架、统一响应中间件、错误码、JWT、注册登录 API、Python AI 服务、SSE 流式接口、情感分析 Agent、记忆管理 Agent、日程抽取与规则时间解析（**P1**） | `backend/cmd`、`backend/pkg/*`、`backend/internal/{middleware,service/chat_*,repository}`、`ai-service/**` |
| **成员 2** | 前端框架 + 聊天界面 + 用户画像 | Vue 骨架、路由配置与守卫、Token 持久化、登录/注册页、聊天界面、SSE 前端解析、对话侧栏、用户画像页 | `frontend/src/{router,stores,api,views/auth,views/chat,views/profile}` |
| **成员 3** | 数据 + 人设 + 朋友圈 + 部署 | 数据库表设计与 AutoMigrate、人设 CRUD（前后端）、主动消息（前后端 + 定时任务 + 手动触发）、AI 朋友圈（前后端 + 定时生成）、日程提醒（表 + 定时触发 + 前端页，**P1**）、Docker Compose、Nginx、云服务器部署 | `backend/internal/{model,handler/persona_handler.go,handler/moment_handler.go,handler/proactive_handler.go,handler/schedule_handler.go,service/proactive_*,service/moment_job.go,service/schedule_service.go,repository/persona_repo.go,repository/moment_repo.go,repository/proactive_repo.go,repository/schedule_repo.go,dto/persona_dto.go,dto/moment_dto.go,dto/proactive_dto.go,dto/schedule_dto.go}`、`frontend/src/views/{persona,moments,schedule}`、`frontend/src/api/{persona,moment,proactive,schedule}.ts`、`deploy/**` |

> 相比原始分工，把**数据库表设计**划给成员 3：成员 1 承担了框架、鉴权、AI 服务三块，表设计前移给成员 3 可让 Week 1 的并行度更高。

> 📄 **每个人的分周任务、自测命令与常见坑**，见各自的开发文档：[总纲](dev/MASTER.md) ｜ [成员 1](dev/MEMBER_1_BACKEND_AI.md) ｜ [成员 2](dev/MEMBER_2_FRONTEND.md) ｜ [成员 3](dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)。

### 1.3 依赖关系

```
准备期 (1-2d)             Week 1                     Week 2 起
┌──────────────┐      ┌──────────────────┐      ┌────────────────────────────┐
│ 三人：        │      │ 成员1：基础框架   │      │ 成员2 / 成员3 并行开发功能  │
│ 环境 + 仓库 + │ ───► │ (Go骨架/JWT/错误码)│ ───► │ (基于已冻结的框架约定)      │
│ 接口契约冻结  │      │ 成员2/3：骨架就绪 │      │                            │
└──────────────┘      └──────────────────┘      └────────────────────────────┘
```

- **成员 1 是第 1 周的瓶颈**：统一响应、错误码、鉴权中间件是另外两人的前置依赖，必须先跑通并**冻结公共约定**（响应结构、错误码、路由命名）。
- 准备期的作用是把**环境配置和契约对齐**从 Week 1 剥离出来，避免第 1 周同时处理框架设计和环境问题。
- 框架冻结后如需变更公共约定，**必须在群里同步并让另两人确认**，不能私自改。

---

## 2. 开发环境与前置准备

### 2.1 工具版本（三人保持一致）

| 工具 | 版本要求 |
|------|----------|
| Node.js | ≥ 18 LTS（建议 20） |
| Go | ≥ 1.21 |
| Python | ≥ 3.10（AI 服务需要；若日后升级 ONNX 小模型，3.10+ 是硬要求） |
| Docker Desktop | 最新稳定版 |
| Git | ≥ 2.30 |
| 数据库 | PostgreSQL 16（通过 Docker 启动，不装本机） |

### 2.2 首次配置（每人执行一次）

```bash
# 1. Git 身份（需与 GitHub 账号关联，便于贡献度统计）
git config --global user.name "Your Name"
git config --global user.email "your@email.com"

# 2. 换行符统一，避免 Windows / Mac 混用导致整文件 diff
git config --global core.autocrlf input

# 3. commit 模板（仓库根目录已有 .gitmessage，用相对路径即可，三人共用同一份）
git config commit.template .gitmessage
```

仓库根目录 `.gitmessage` 的内容：

```
# <type>(<scope>): <subject>   ← 50 字符以内，祈使句，结尾不加句号
# 空行
# <body>                       ← 说明「为什么改」，每行 72 字符以内
# 空行
# <footer>                     ← BREAKING CHANGE / Closes #12
#
# type: feat | fix | docs | style | refactor | test | chore
```

### 2.3 仓库初始化（准备期完成）

**仓库设置**

| 项 | 设置 |
|----|------|
| 仓库名 | `heart-echo`，Private |
| 协作者 | 三人均为 collaborator（另两人需接受邀请，否则无 push 权限） |
| 分支保护 | `main` 与 `develop` 均开启 **Require a pull request before merging** + **Require approvals (1)** |
| 默认分支 | `develop` |

**为什么用单仓库（monorepo）而不是前端/后端分开建仓**：

| 理由 | 说明 |
|------|------|
| 贡献度考核 | 考核依据是 GitHub 提交记录；单仓库一张贡献图看全三人，多仓库会割裂且评委需逐个查看 |
| 契约同步 | 4 周内接口频繁变更，单仓库可一次 commit 同时改前后端与 `docs/API_CONTRACT.md`，不会出现"后端改了字段前端不知道" |
| 部署成本 | `deploy/docker-compose.yml` 的构建上下文是仓库根目录（`../backend`、`../frontend`）；拆分仓库将导致 compose 无法构建，需引入 submodule 等额外机制 |
| 规范统一 | `.gitignore`、`.gitmessage`、Commit 规范、文档目录一次配置，三人共用 |

**初始分支结构**

```bash
git init
git add .
git commit -m "chore: initialize repository structure"
git branch -M main
git remote add origin git@github.com:<org>/heart-echo.git
git push -u origin main

git checkout -b develop        # 日常开发主分支
git push -u origin develop
```

**前提**：三人的 SSH key 已添加到 GitHub 且 `ssh -T git@github.com` 验证通过。若暂时不通，可先用 HTTPS 地址克隆，不阻塞开发。

### 2.4 克隆与启动

```bash
git clone git@github.com:<org>/heart-echo.git
cd heart-echo
cp backend/.env.example backend/.env
cp ai-service/.env.example ai-service/.env
# 找成员1要 DeepSeek API Key，填进 ai-service/.env
docker compose -f deploy/docker-compose.dev.yml up -d
```

详见 [技术文档 · 部署方案](TECH_DESIGN.md#9-部署方案)。

---

## 3. Git 工作流

### 3.1 分支模型

| 分支 | 用途 | 保护规则 |
|------|------|----------|
| `main` | 稳定可发布版本，**只接受来自 `develop` 的合并** | 🔒 禁止直接 push |
| `develop` | 开发主分支，集成本周所有功能 | 🔒 禁止直接 push，只接受 PR 合并 |
| `feature/xxx` | 个人功能分支，从 `develop` 切出，完成后 PR 回 `develop` | 自由 push |
| `fix/xxx` | Bug 修复分支 | 自由 push |
| `hotfix/xxx` | 紧急修复，从 `main` 切出，修完同时合并回 `main` 与 `develop` | 自由 push |

**分支命名规范**：`<类型>/<简短描述>`，全小写，用连字符分隔，**禁止用中文和空格**。

```
feature/chat-sse-stream      ✅
feature/auth-jwt-refresh     ✅
fix/login-token-expired      ✅
feature/朋友圈                ❌
feature/ChatSSE               ❌
my-branch                     ❌（无法看出在做什么）
```

### 3.2 标准开发流程

```bash
# 1. 切到 develop 并拉取最新代码（每天开工前必做）
git checkout develop
git pull origin develop

# 2. 从 develop 切出功能分支
git checkout -b feature/chat-sse-stream

# 3. 开发 —— 小步提交，见第 4 节
git add backend/internal/service/chat_service.go
git commit -m "feat(chat): add SSE streaming endpoint"

# 4. 开发期间定期同步 develop，避免最后大冲突
git fetch origin
git rebase origin/develop     # 或 git merge origin/develop

# 5. 推送功能分支
git push origin feature/chat-sse-stream

# 6. 在 GitHub 上发起 PR → develop，@两位队友 review
# 7. Review 通过后由 PR 作者自己 merge（squash 或 merge commit 均可，全队统一）
# 8. 删除已合并的远程分支
git push origin --delete feature/chat-sse-stream
```

### 3.3 分支纪律

- **每天开工第一件事**：`git checkout develop && git pull`
- **每天收工最后一件事**：确保当天工作已 push（哪怕没写完，推到自己的 feature 分支）
- **不要长期不合并**：功能分支存活不超过 **3 天**，否则冲突会让你怀疑人生
- **绝不 force push 到 `main` / `develop`**
- **绝不提交** `.env`、`node_modules/`、`dist/`、`__pycache__/`、模型权重文件（见 `.gitignore`）
- 冲突不要瞎解决：`git rebase` 冲突时搞不清就找成员 1 一起看，**不要直接把代码删了**

### 3.4 接口契约先行（准备期完成，否则第 2 周必定互相等待）

三人并行开发，最容易出现的死局是：**前端等后端给接口，后端等前端提需求，第 2 周结束还在原地**。

**破解方法**：

1. **准备期冻结契约**：三人一起过一遍 [API 列表](TECH_DESIGN.md#75-关键-api-端点)，逐条确认 URL、请求字段、响应字段、错误码，确认后签署 [API_CONTRACT.md](API_CONTRACT.md)。模板已就位，逐个端点把 `⬜` 改成 `✅` 即可。
2. **前端用 Mock 开发**：真实接口没写好之前，前端用 Mock 数据把页面先跑起来（做法见 [技术文档 7.6](TECH_DESIGN.md#76-接口契约先行准备期必须完成)）。
3. **契约变更必须广播**：任何人改了字段名/结构，**必须在群里说**，并在 [API_CONTRACT.md](API_CONTRACT.md) 的「变更记录」中登记。默默改字段是团队协作中最容易引发返工的行为。

**判断标准**：如果第 1 周结束时，成员 2 因为"后端接口还没好"而无法推进页面 —— 说明契约先行没做到位。

### 3.5 .gitignore 要点

```gitignore
# 环境与密钥（最高优先级）
.env
*.env.local

# Node
node_modules/
dist/
*.local

# Go
backend/bin/
*.exe

# Python
__pycache__/
*.py[cod]
.venv/
venv/
*.onnx          # 模型文件不入库，写文档说明下载方式

# IDE / 系统
.idea/
.vscode/
.DS_Store
Thumbs.db

# 数据卷
pg_data/
chroma_data/
```

---

## 4. Commit 规范

> 依据：团队指定的 [Git 提交信息规范与最佳实践](https://ctbloge.github.io/blog/git/git-commit.html)（遵循 [generate-changelog](https://github.com/lob/generate-changelog#usage) 的书写方式，即社区通行的 Conventional Commits）。

### 4.1 标准结构

```
<type>(<scope>): <subject>

<body>

<footer>
```

| 部分 | 是否必填 | 说明 |
|------|----------|------|
| `<type>` | ✅ 必填 | 提交类型，见 4.2 类型表 |
| `<scope>` | 可选 | 提交范围/模块，如 `auth`、`api`、`chat` |
| `<subject>` | ✅ 必填 | 简短描述，**50 字符以内**，祈使句，简洁明了 |
| `<body>` | 可选 | 详细描述本次提交的背景、原因及实施细节，**每行不超过 72 字符** |
| `<footer>` | 可选 | 标注 `BREAKING CHANGE` 或关闭的 issue 编号 |

### 4.2 提交类型表

| 类型 | 描述 | 适用场景 |
|------|------|----------|
| `feat` | 新特性 | 增加新功能 |
| `fix` | 修复问题 | 修复 bug |
| `docs` | 文档更新 | 修改文档 |
| `style` | 格式调整（无逻辑变更） | 修改代码格式，如缩进、空格 |
| `refactor` | 代码重构 | 重构代码，非修复性改动 |
| `test` | 添加/修改测试 | 修改测试代码 |
| `chore` | 其他杂项任务 | 比如更新依赖、构建工具等 |

> 本团队在实际使用中额外采用两个社区扩展类型（不在原文表中，如不需要可忽略）：
> - `perf`：性能优化
> - `ci`：CI/CD 配置变更（如 GitHub Actions）

### 4.3 格式硬性规则

| 规则 | 要求 |
|------|------|
| 第一行 subject | ❌ **不应换行**，**50 字符以内**（硬性上限；GitHub 列表与 changelog 工具都会在此处截断） |
| 第二行 | ✅ **必须为空行**，用于分隔 subject 和 body |
| 第三行及以后 body | ✅ 每行不超过 72 字符，可换行描述变更内容 |
| footer | ✅ 可另起一段，描述 `BREAKING CHANGE`、关闭 issue 编号等 |
| 语态 | 使用**祈使句**（add / fix / update，而不是 added / fixes / updated） |
| 大小写 | subject 首字母小写，结尾**不加句号** |
| 语言 | **统一使用英文**（避免中文 subject 在 changelog 工具中乱码） |

### 4.4 正例

```
feat(auth): add support for two-factor authentication

This commit introduces two-factor authentication (2FA) to enhance security
during login. It uses a time-based one-time password (TOTP) generator app.

BREAKING CHANGE: This change removes the old login method that didn't require 2FA.
```

```
fix(user): handle edge case where username is null
```

```
feat(chat): stream assistant reply over SSE

The Go backend now forwards LLM chunks to the client as server-sent events
so the frontend can render a typewriter effect. Emotion analysis runs before
generation and shapes the reply strategy, but is not sent to the client.

Closes #12
```

```
docs(readme): add local development quick start
```

```
refactor(errcode): unify business error construction

Replace ad-hoc fmt.Errorf calls with errcode.New / errcode.Wrap so that
every failure path returns a code from the central table.
```

### 4.5 反例

```
update            ❌ 没有 type，看不出改了什么
fix bug           ❌ 没有 scope，没有说清楚修了什么 bug
feat: 增加了登录功能，顺便改了一下样式和数据库   ❌ 中文 + 一次提交干了三件事
feat(auth): Added new login flow.               ❌ 过去式 + 结尾句号
feat(auth): add login flow
fix(auth): fix login flow
refactor(auth): refactor login flow             ⚠️ 三条都能用，但同一件事应合并为一条提交
WIP                                              ❌ 禁止提交 WIP 到共享分支
```

### 4.6 多行提交信息的写法

**方式一：多个 `-m` 参数**

```bash
git commit -m "feat(auth): add login check" \
  -m "This prevents users from staying logged in with expired tokens." \
  -m "BREAKING CHANGE: requires token refresh endpoint."
```

**方式二：写入文件后用 `-F` 提交**（较长信息推荐）

```bash
echo -e "feat: add user pagination\n\nThis allows users to paginate their data." > commit-msg.txt
git commit -F commit-msg.txt
```

### 4.7 提交粒度：产出即提交

> 硬性要求 5：**尽可能有产出/修改后就立即 commit。**

- ✅ 写完一个函数、修完一个 bug、调通一个接口 → **立刻 commit**
- ✅ 一次提交只做一件事（一个功能点 / 一个修复 / 一次文档更新）
- ❌ 攒一天再提交（`git commit -m "今天的活"`）——**这是本项目明确禁止的行为**
- ❌ 一次提交改 20 个文件、跨三个模块——无法 review，也无法回滚

**建议粒度参考**：

| 场景 | 提交次数 |
|------|----------|
| 实现一个 API（含 handler + service + repo） | 1-2 次 |
| 完成一个 Vue 页面 | 1-2 次（骨架 + 联调） |
| 修复一个 bug | 1 次 |
| 调整配置 / 加依赖 | 1 次 |
| 写完一份文档 | 1 次 |

**每天至少 1-2 次提交**，这是贡献度的基本保障。三人应保持大致均衡的提交频率。

### 4.8 可选：用工具强制校验（有时间再做）

```bash
# 安装 commitlint + husky，在 commit 时自动校验格式
npm install --save-dev @commitlint/cli @commitlint/config-conventional husky
npx husky init
echo 'npx --no -- commitlint --edit "$1"' > .husky/commit-msg
```

`.commitlintrc.json`：

```json
{
  "extends": ["@commitlint/config-conventional"],
  "rules": {
    "subject-max-length": [2, "always", 50],
    "body-max-line-length": [2, "always", 72]
  }
}
```

---

## 5. PR 与 Code Review

### 5.1 PR 流程

```
功能分支开发完成
   → push 到 GitHub
   → 发起 PR 到 develop（标题符合 commit 规范）
   → 填写 PR 描述（做了什么 / 怎么验证 / 截图）
   → 指定 2 位 reviewer（剩下两位队友）
   → 至少 1 人 Approve 才能合并
   → 作者自己点击 Merge，删除分支
```

### 5.2 PR 描述模板

> 建议在 `.github/pull_request_template.md` 中固化。

```markdown
## 变更内容
- 一句话说明这个 PR 做了什么

## 关联 Issue
Closes #12

## 变更类型
- [ ] 新功能 feat
- [ ] 修复 fix
- [ ] 重构 refactor
- [ ] 文档 docs
- [ ] 其他

## 如何验证
1. 启动 `docker compose -f deploy/docker-compose.dev.yml up -d`
2. 访问 `http://localhost:5173/login`
3. 用 xxx 账号登录，应看到 xxx

## 截图 / 录屏
（涉及 UI 变更必须附）

## 自查清单
- [ ] 本地已跑通，无报错
- [ ] 未提交 .env / 密钥文件
- [ ] 新增接口已更新 API 文档
- [ ] 错误码使用了 errcode 中的定义，未硬编码数字
```

### 5.3 Code Review 清单

Reviewer 重点看以下几点（**不要求逐行读懂，抓住关键风险**）：

**通用**
- [ ] 有没有把 `.env`、密钥、模型文件提交进来
- [ ] 有没有 `console.log` / `fmt.Println` 调试代码残留
- [ ] 变量命名是否清晰（见第 6 节）

**后端**
- [ ] 错误是否走了 `errcode` + 统一响应，没有手写 `c.JSON` 返回错误
- [ ] 涉及 `persona_id` 的查询**是否带了 `user_id` 条件**（越权风险）
- [ ] 记忆 / 画像查询**是否带了 `persona_id`**（不带会串人设）
- [ ] 改动是否违反了 [开发总纲 §0 功能决策记录](dev/MASTER.md#0-功能决策记录已确认不要私自改)（例如给朋友圈加手动触发按钮、给情绪加展示标签）
- [ ] handler 是否足够薄，业务逻辑是否放在 service
- [ ] 是否有 N+1 查询 / 缺索引的慢查询

**前端**
- [ ] 是否有 `any` 类型（应使用 `unknown` + 类型收窄）
- [ ] API 响应类型是否与后端 `Response<T>` 对应
- [ ] 路由是否需要鉴权，`meta.requiresAuth` 是否正确
- [ ] 是否处理了 loading / empty / error 三种状态

**AI 服务**
- [ ] 记忆 / 画像查询是否**同时**按 `persona_id` 与 `user_id` 过滤（只带 `persona_id` 会串号，只带 `user_id` 会串人设；升级接入 ChromaDB 后两个条件同样都要带）
- [ ] LLM 调用是否有超时与异常处理
- [ ] Prompt 中是否可能注入用户隐私到其他用户上下文

### 5.4 Review 礼仪

- **24 小时内必须响应 PR**，哪怕是"我今天没空，明早看"
- 评论对事不对人：说"这里可能有越权风险，建议加 user_id 条件"，而不是"你这写的什么"
- 有分歧时**拉个 5 分钟语音**，不要在 PR 里吵 20 条评论
- Approve 不代表代码完美，只代表"可以合并，我认可"

---

## 6. 命名规范

> 硬性要求 4：使用易读、标准的变量命名形式。

### 6.1 总原则

1. **见名知意**：`remainingRetryCount` 优于 `cnt`；`userList` 优于 `arr`
2. **禁止拼音命名**：`yonghu`、`huihua` 一律不用
3. **缩写限于团队共识**：`ctx`、`req`、`resp`、`msg` 可用（Go / Web 惯例）；`a1`、`tmp2`、`data2` 这类无意义编号不允许
4. **布尔值用 `is` / `has` / `can` / `should` 开头**：`isLoggedIn`、`hasPermission`
5. **函数名用动词开头**：`getUserByID`、`createPersona`、`handleStreamError`

### 6.2 各语言命名对照

| 场景 | 规范 | 示例 |
|------|------|------|
| **Go 变量/函数** | camelCase（非导出）/ PascalCase（导出） | `userID`、`GetUserByID` |
| **Go 常量** | PascalCase，错误码用 `Err` 前缀 | `ErrTokenExpired`、`MaxRetryCount` |
| **Go 包名** | 全小写，单个单词，不用下划线 | `handler`、`errcode` |
| **Go 文件名** | 小写下划线 | `chat_service.go`、`auth_handler.go` |
| **Go 接口** | 单方法接口用 `-er` 后缀 | `Reader`、`UserRepository` |
| **TS 变量/函数** | camelCase | `userInfo`、`fetchMessages` |
| **TS 常量** | UPPER_SNAKE_CASE | `ACCESS_TOKEN_KEY`、`MAX_RETRY` |
| **TS 类型/接口/组件** | PascalCase | `ChatMessage`、`UserProfile`、`MessageBubble.vue` |
| **TS 文件（非组件）** | camelCase | `request.ts`、`auth.ts` |
| **CSS 类名** | kebab-case | `.message-bubble`、`.typing-indicator` |
| **Python 变量/函数** | snake_case | `user_id`、`extract_memories` |
| **Python 类** | PascalCase | `EmotionAgent`、`MemoryRetriever` |
| **数据库表名** | 全小写 + 下划线，**用复数** | `users`、`chat_messages`、`proactive_settings`（例外：`user_memory`、`user_profile` 语义上不可数，保留单数） |
| **数据库字段** | 全小写 + 下划线 | `password_hash`、`created_at` |
| **URL 路径** | kebab-case + 复数名词；路径参数用 camelCase 且与字段同名 | `/api/v1/chat/personas/:personaId/messages`、`/api/v1/user/profile` |
| **JSON 字段** | camelCase | `accessToken`、`emotionLabel` |
| **Git 分支** | kebab-case + 类型前缀 | `feature/chat-sse-stream` |

### 6.3 Go 命名特别注意

```go
// ✅ 正确：缩写词全大写或全小写，保持一致
userID   := uint(1)
apiURL   := "https://..."
httpClient := &http.Client{}

// ❌ 错误：缩写词大小写混用
userId := uint(1)
apiUrl := "https://..."
```

```go
// ✅ 正确：错误变量统一 err，业务错误用 Err 前缀常量
if err != nil {
    return errcode.Wrap(errcode.ErrDBFailed, err)
}

// ❌ 错误：错误变量命名随意
if e := doSomething(); e != nil { }
```

### 6.4 TypeScript 命名特别注意

```ts
// ✅ 正确：接口用 PascalCase，字段 camelCase，与后端 JSON 对齐
interface ChatMessage {
  id: number
  personaId: number
  emotionLabel: EmotionLabel | null
  createdAt: string
}

// ❌ 错误：字段用下划线（与后端 JSON 不一致，会导致取不到值）
interface ChatMessage {
  persona_id: number
  emotion_label: string
}
```

> ⚠️ **前后端字段对齐**：后端 JSON tag 用 camelCase，前端 TS 接口字段也用 camelCase，两边必须完全一致。这是最容易出 bug 的地方，联调时优先核对。

---

## 7. 目录结构规范

> 完整目录树见 [技术文档 · 目录结构](TECH_DESIGN.md#8-目录结构)。以下是**放置代码的判断规则**。

### 7.1 后端（Go）：新代码放哪里？

| 我要写的东西 | 放哪里 |
|--------------|--------|
| 一个新的 HTTP 接口 | `internal/handler/xxx_handler.go` + 在 `handler/router.go` 注册 |
| 一段业务逻辑（事务、多表操作） | `internal/service/xxx_service.go` |
| 一次数据库查询 | `internal/repository/xxx_repo.go` |
| 一个新的数据表实体 | `internal/model/xxx.go` |
| 请求/响应的数据结构 | `internal/dto/xxx_dto.go` |
| 一个跨模块复用的工具（响应、JWT、错误码） | `pkg/xxx/` |
| 一个中间件 | `internal/middleware/xxx.go` |

**判断口诀**：能脱离本项目被别的项目复用 → `pkg/`；只服务于本项目业务 → `internal/`。

### 7.2 前端（Vue）：新代码放哪里？

| 我要写的东西 | 放哪里 |
|--------------|--------|
| 一个完整页面 | `src/views/<模块>/XxxView.vue` |
| 一个可复用组件 | `src/components/<模块>/Xxx.vue` |
| 一个接口请求函数 | `src/api/<模块>.ts` |
| 一个全局状态 | `src/stores/<模块>.ts` |
| 一个 TS 类型 | `src/types/<模块>.ts` |
| 一个工具函数 | `src/utils/xxx.ts` |

### 7.3 命名与组织纪律

- **一个文件只做一件事**：`chat_service.go` 里不要混入朋友圈逻辑
- **不要新建"杂物间"文件**：`utils.ts`、`common.ts`、`misc.go` 这种无边界文件禁止新增，按用途拆开
- **不要跨层调用**：前端组件不要直接写 `axios`，必须走 `src/api/`；后端 handler 不要直接调 repository，必须走 service
- **新增目录前先在群里说一声**，避免两个人建出两套并行结构

---

## 8. 协作节奏与沟通机制

### 8.1 每日节奏

| 时间 | 事项 | 形式 |
|------|------|------|
| 开工 | 同步进度：昨天做了什么 / 今天做什么 / 有没有卡住 | 群里 3 条消息即可 |
| 开发中 | 产出即 commit + push | Git |
| 遇到阻塞 > 30 分钟 | 立刻在群里说，不要自己死磕 | 群消息 / 语音 |
| 收工 | 确认当天代码已 push | Git |

### 8.2 每周节奏

| 时间 | 事项 |
|------|------|
| 周一 | 对齐本周目标（对应 README 的周计划） |
| 周三 | 中期检查：核心链路是否推进顺利，是否需要砍功能 |
| 周日 | 合并所有 feature 分支到 develop，跑通完整链路，复盘 |

**关键检查点（必须严格执行，不通过就砍功能）**：

| 节点 | 检查内容 | 不通过怎么办 |
|------|----------|-------------|
| 准备期末 | 环境就绪；接口契约已冻结；仓库可用 | **不要带着未就绪的环境进 Week 1** |
| Week 1 末 | 注册 → 登录 → 刷新保持登录态，前后端联调成功 | 第 2 周前 2 天继续补，同时压缩人设功能 |
| **Week 2 末** | **流式对话跑通（生死线）** | **立即砍掉记忆系统与主动消息**，Week 3-4 只打磨 1-4 项 + 部署 |
| Week 3 末 | 核心演示链路全通 | 砍 P1，全力保部署 |
| Week 4 末 | 公网可访问 + 演示预演通过 | 启用备用演示录屏 |

> **Week 2 末的生死线是整份文档里最重要的一条。** 流式链路跨 Go / Python / Nginx / 浏览器四段，是联调成本最集中的地方。若那时仍跑不通，必须立即止损——宁可功能少但完整，不要功能多但都半成品。

### 8.3 沟通纪律

- **阻塞超过 30 分钟主动同步**。跨进程问题（SSE、Docker 网络）单人排查效率低，及时拉人一起看比闷头试更快。
- **公共约定变更必须广播**：改了错误码、改了响应结构、改了数据库字段、加了依赖，都要在群里说。
- **口头结论要落文字**：语音讨论完，把结论发到群里，避免"我以为你记得"。
- **不要直接改别人的文件**：发现队友的 bug，先在群里说 / 提 issue，由 Owner 改；紧急情况改了要立刻告知。

### 8.4 Issue 使用

发现的问题、待办、bug 都提到 GitHub Issues，用标签分类：

| 标签 | 用途 |
|------|------|
| `bug` | 缺陷 |
| `feature` | 新功能 |
| `P0` / `P1` / `P2` | 优先级 |
| `blocked` | 被阻塞，需要协助 |
| `good first issue` | 适合练手的小任务 |

Commit 中通过 `Closes #12` 关联 Issue，合并后自动关闭。

---

## 9. 贡献度说明

> 考核依据：**GitHub 提交记录**。以下行为直接影响你的贡献度评价。

### 9.0 私有仓库的贡献度可见性（先做这一步）

仓库设为 Private 后，**提交默认不会出现在个人 GitHub 主页的贡献图里**。三人各自执行一次：

1. GitHub → 右上角头像 → **Settings** → 左侧 **Public profile**
2. 勾选 **Include private contributions on my profile**

另外两点：

- **提交邮箱必须是 GitHub 账号已验证的邮箱**，否则提交不会被归属到你名下。核对命令：
  ```bash
  git config user.email          # 与 GitHub 账号邮箱一致
  git log --format='%an <%ae>' | sort -u   # 确认没有杂邮箱混入
  ```
- 答辩/考核时若需要评委查看仓库，**提前把评委加入 collaborator**，或在提交前临时改为 Public——不要等到当天才处理。

### 9.1 什么样的提交算有效贡献

| 行为 | 是否计入 |
|------|----------|
| 功能代码提交（feat / fix / refactor） | ✅ 高权重 |
| 文档提交（docs） | ✅ 计入 |
| 配置 / 部署提交（chore / ci） | ✅ 计入 |
| Code Review 评论、Approve | ✅ 计入（GitHub 有记录） |
| Issue 提出与跟进 | ✅ 计入 |
| 只改空格/换行的无意义提交 | ⚠️ 不计入，且观感差 |
| 同一天把一个功能拆成 20 次提交刷量 | ⚠️ 不计入，属于无效刷量 |
| 最后一周集中提交 | ⚠️ 与日常节奏不符，会被识别 |

### 9.2 三人均衡的保障机制

- 按**功能模块**分工，每人都有前端 + 后端产出，天然均衡
- 每周日检查一次 `git shortlog -sn --all`，若差距过大，下周调整任务分配
- **PR 交叉 review**：每条 PR 由另外两人中至少一人 review，保证互相有记录
- 成员 1 在第 1 周工作量最大，第 2 周起应主动把新功能让给成员 2 / 3

### 9.3 常用统计命令

```bash
# 查看各成员提交数
git shortlog -sn --all

# 查看某成员在某时间段的提交
git log --author="Name" --since="2026-09-01" --oneline

# 查看代码行数贡献（仅供参考，不要唯行数论）
git log --author="Name" --pretty=tformat: --numstat | awk '{add+=$1; del+=$2} END {printf "added: %s, deleted: %s\n", add, del}'
```

---

## 10. 风险应对与红线

### 10.1 风险与应对

| 风险 | 应对 |
|------|------|
| **Week 2 生死线** —— 流式对话未按期跑通 | 本周末若未跑通，立即砍掉记忆与主动消息，Week 3-4 全力保核心链路 + 部署。**日程提醒（P1 队尾）同时取消** |
| **前后端互相等待** | 准备期冻结接口契约 + 前端用 Mock 开发（见 [技术文档 7.6](TECH_DESIGN.md#76-接口契约先行准备期必须完成)） |
| **成员 1 第 1 周超载** | 准备期消化环境工作；把数据库表设计划给成员 3，提高并行度 |
| **跨进程排查成本** —— SSE 链路跨 Go / Python / Nginx / 浏览器四段 | 分段验证：先用 `curl -N` 直连 Python 服务确认能流式，再逐层往上加；**每加一层验证一次**，不要一次性全接通再调 |
| **主动消息无法现场演示** | 提供 `POST /api/v1/proactive/trigger` 手动触发，且**与定时任务复用同一段逻辑**；演示前先验证可用 |
| **朋友圈无法现场演示** | 产品决策上**没有手动触发接口**（[开发总纲 §0.3](dev/MASTER.md#03-功能范围)）。靠两条兜底：① 演示前提前灌好历史动态；② `MOMENT_JOB_INTERVAL` 调短到 2 分钟，现场打开时可能刚好有新动态 |
| **日程提醒无法现场演示**（P1） | 与主动消息同理，**有**手动触发接口 `POST /api/v1/schedules/:id/trigger`，与定时任务复用同一逻辑。演示前先验证可用；也可现场说「1 分钟后提醒我」 |
| **DeepSeek API 不稳定** | 超时重试 2 次 + 兜底话术；演示前准备录屏兜底 |
| **部署环节翻车** | Week 4 提前 2 天完成部署与预演，留出修复时间 |
| **阶段二项拖垮主线** | 严格按 [推进顺序](TECH_DESIGN.md#阶段二的推进顺序) 执行；每替换一项必须保持服务可运行，出问题立即切回阶段一实现 |

### 10.2 砍功能的优先级顺序（时间不够时从上往下砍）

| 顺序 | 砍什么 | 说明 |
|------|--------|------|
| ① 先砍 | 语音对话 | P2，本来就不做 |
| ② 再砍 | 情绪趋势可视化 | P2，不做 |
| ③ 再砍 | 人格演化 | P1 |
| ④ 再砍 | 用户画像页 | P1 |
| ⑤ 再砍 | 日程提醒 | **P1 队尾**。实现顺序上排在朋友圈**之后**（所以砍的时候排在朋友圈**之前**），Week 4 主线（朋友圈 + 部署 + 阶段二）没做完就**不开工** |
| ⑥ 再砍 | AI 朋友圈 | P1 |
| ⑦ 最后砍 | 主动消息 / 记忆系统 | P0，仅当 Week 2 生死线未过时才砍 |
| 🚫 绝不砍 | 登录注册 / 人设 / 流式对话 / 情感分析 | 砍掉任一项，硬性要求即不完整 |

### 10.3 必须保证的核心演示链路

```
登录 → 创建人设 → 对话（流式 + 情感感知）→ AI 记住信息 → 主动消息 → 朋友圈互动
（P1 有余量再加：对话里说「明天下午三点提醒我开会」→ 日程页看到它 → 手动触发看到提醒）
```

**这条链路跑通，项目就成立。**

> README 的 [第九节 · 验收演示脚本](../README.md#九验收演示脚本) 给出了逐步操作顺序与翻车预案，Week 4 必须完整预演至少 2 次。

### 10.4 团队红线（不可触碰）

1. ❌ **提交 `.env`、API Key、密码到仓库** —— 一旦提交，必须立刻轮换密钥并清理历史
2. ❌ **直接 push 到 `main` / `develop`** —— 必须走 PR
3. ❌ **force push 共享分支**
4. ❌ **跨用户 / 跨人设数据泄漏**（记忆与画像查询不带 `user_id` 会跨用户，不带 `persona_id` 会跨人设；越权访问他人人设与对话同理）—— 这是功能性问题，不是代码风格问题
5. ❌ **明文/弱哈希存储密码** —— 必须 bcrypt
6. ❌ **连续 3 天无任何提交且不沟通** —— 视为失联，影响贡献度评价
7. ❌ **最后一周突击提交** —— 与"产出即提交"的要求相悖

---

## 附：本项目命名速查

| 场景 | 格式 | 示例 |
|------|------|------|
| 功能分支 | `feature/<模块>-<描述>` | `feature/chat-sse-stream`、`feature/auth-jwt-refresh` |
| 修复分支 | `fix/<描述>` | `fix/login-token-expired` |
| Commit | `<type>(<scope>): <subject>` | `feat(chat): stream assistant reply over SSE` |
| 本项目 scope 取值 | `auth` `user` `model` `persona` `chat` `memory` `emotion` `moment` `proactive` `schedule` `deploy` `errcode` | `fix(errcode): unify business error construction` |
