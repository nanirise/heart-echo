# HeartEcho 技术文档

> 版本：v0.1 ｜ 最后更新：2026-09-15
> 配套文档：[项目概览](../README.md) ｜ [团队协作文档](COLLABORATION.md)

---

## 目录

1. [总体架构](#1-总体架构)
2. [技术选型](#2-技术选型)
3. [前端设计](#3-前端设计)
4. [后端设计](#4-后端设计)
5. [AI 服务设计](#5-ai-服务设计)
   - ⭐ [5.0 实现分级与升级路径](#50-实现分级与升级路径核心设计) —— **先读这一节**
6. [数据库设计](#6-数据库设计)
7. [API 规范](#7-api-规范)
8. [目录结构](#8-目录结构)
9. [部署方案](#9-部署方案)
10. [安全、降级与边界处理](#10-安全降级与边界处理)

---

## 1. 总体架构

### 1.1 分层架构

```
┌──────────────────────────────────────────────────────────┐
│  前端层  Vue3 + TypeScript + Pinia + Vue Router           │
│  登录注册 / 流式对话 / 朋友圈 / 人设管理 / 用户画像         │
└────────────────────────┬─────────────────────────────────┘
                         │ RESTful API + SSE
                         ▼
┌──────────────────────────────────────────────────────────┐
│  业务后端层  Go + Gin + GORM                              │
│  JWT 鉴权 / 人设与消息持久化                               │
│  统一响应中间件 / 统一错误码 / 路由分组                    │
└────────────────────────┬─────────────────────────────────┘
                         │ 内部 HTTP 调用
                         ▼
┌──────────────────────────────────────────────────────────┐
│  AI 服务层  Python FastAPI（薄层，约 300 行）              │
│  情感分析 Agent / 对话生成 Agent / 记忆管理 Agent          │
│  DeepSeek API                                              │
│  ── 升级路径：ONNX 情感模型 / ChromaDB 向量检索 ──          │
└────────────────────────┬─────────────────────────────────┘
                         ▼
┌──────────────────────────────────────────────────────────┐
│  数据层  PostgreSQL（关系数据 + 记忆 + 画像）              │
│          —— 一个库搞定，不引入向量库与缓存 ——              │
└──────────────────────────────────────────────────────────┘
```

### 1.2 为什么分成 Go 后端和 Python AI 服务两个服务？

| 考量 | 说明 |
|------|------|
| 生态匹配 | Go 擅长高并发 Web 服务与稳定的业务 API；Python 拥有 LLM SDK、ONNX Runtime、ChromaDB 等 AI 生态 |
| 隔离故障 | LLM 调用慢且不稳定，独立服务可单独超时、降级、重启，不拖垮主业务 |
| 独立部署 | AI 服务资源占用模型不同（内存大、CPU 密集），可单独扩容 |
| 学习价值 | 练习服务间 HTTP 通信与契约设计 |

**通信方式**：Go 后端通过内部 HTTP 调用 Python 服务（`http://ai-service:8000`），使用内部 API Key 做简单校验，**该服务不暴露公网**。

---

## 2. 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| 前端框架 | Vue3 + TypeScript | 硬性要求，且生态成熟 |
| 状态管理 | Pinia | Vue3 官方推荐，TS 支持好 |
| 路由 | Vue Router 4 | 硬性要求，配路由守卫 |
| UI 库 | Element Plus | 中文文档完善，组件覆盖全 |
| HTTP 客户端 | Axios | 拦截器机制成熟，便于统一处理 Token 与错误码 |
| Markdown 渲染 | markdown-it + highlight.js | 轻量、可控，避免 v-html 注入风险（配合 DOMPurify） |
| 后端框架 | Go + Gin | 最易上手的 Go Web 框架 |
| ORM | GORM | Go 生态最成熟的 ORM |
| 数据库 | PostgreSQL 16 | 一个库搞定全部数据，JSONB 适合存画像与人格状态这类字段不固定的结构 |
| AI 服务 | Python FastAPI | 薄层（约 300 行），只做 Agent 编排，与 Go 通过内部 HTTP 通信 |
| LLM | DeepSeek API | 国内可访问、便宜、兼容 OpenAI 格式 |
| 情感分析（阶段一） | 8 类情绪关键词词典 | 离线、零依赖，保证 Week 2 链路先通 |
| 情感分析（阶段二） | chinese-chat-sentiment-8class + ONNX | 接口已预留，替换实现不改调用方 |
| 记忆检索（阶段一） | PostgreSQL：最近 N 条 + 关键词加权 | 零额外组件 |
| 记忆检索（阶段二） | ChromaDB 向量检索 | 接口已预留，替换实现不改调用方 |
| 部署 | Docker Compose + 云服务器 | 学生机约 15-20 元/月 |
| 反向代理 | Nginx | 前端静态文件 + API 转发 |

**明确不做的技术**：

| 不做 | 理由 |
|------|------|
| Redis | 本项目 QPS 极低，无真实缓存需求，引入只增加一个需要运维的容器 |
| 消息队列 | 主动消息用定时任务扫描即可，不需要 MQ |
| Kubernetes | Docker Compose 足够，K8s 的运维复杂度与项目规模不匹配 |
| CI/CD 自动化 | 手动 `git pull && docker compose up -d --build` 已满足 4 周节奏，投入产出比低 |
| 数据库迁移工具（golang-migrate 等） | 用 GORM `AutoMigrate`，结构变更直接改 struct，省去迁移版本管理 |
| HTTPS / 域名 / 备案 | 硬性要求只要求"公网可访问"，用 `http://公网IP` 即可。国内服务器绑域名必须 ICP 备案，要 2-3 周，收益与时间成本不匹配 |
| 微服务拆分 | 两个服务（Go + Python）已是本项目的合理上限，不再拆 |

> **阶段划分的完整设计**见 [5.0 实现分级与升级路径](#50-实现分级与升级路径核心设计)。

---

## 3. 前端设计

> 对应硬性要求 1：**Vue 全家桶 + Route 路由管理 + TypeScript 类型规范 + 路由守卫**。

### 3.1 技术栈组成

| 层次 | 技术 | 职责 |
|------|------|------|
| 视图层 | Vue3 `<script setup>` + Element Plus | 页面渲染与交互 |
| 类型层 | TypeScript（`strict: true`） | 全量类型约束 |
| 状态层 | Pinia + `pinia-plugin-persistedstate` | 全局状态与持久化 |
| 路由层 | Vue Router 4 | 路由管理 + 路由守卫 |
| 通信层 | Axios + fetch ReadableStream | REST 请求 + SSE 流式解析 |

### 3.2 TypeScript 类型规范

**`tsconfig.json` 关键配置**：

```jsonc
{
  "compilerOptions": {
    "strict": true,              // 开启全部严格检查
    "noImplicitAny": true,       // 禁止隐式 any
    "strictNullChecks": true,    // 严格的 null 检查
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "paths": { "@/*": ["./src/*"] }
  }
}
```

**规范要求**：

1. **禁止 `any`**。确实未知用 `unknown`，使用前做类型收窄。
2. **接口优先用 `interface`，联合类型用 `type`**。
3. **API 响应必须泛型化**，与后端统一响应结构严格对应：

```ts
// src/types/api.ts —— 与后端 Response[T] 一一对应
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
  timestamp: number
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  pageSize: number
}
```

4. **业务实体类型集中在 `src/types/`**，禁止在组件里散落匿名对象类型：

```ts
// src/types/chat.ts
export type MessageRole = 'user' | 'assistant'

/** 8 类情绪标签，与 AI 服务约定一致 */
export type EmotionLabel =
  | 'joy' | 'sadness' | 'anger' | 'fear'
  | 'surprise' | 'disgust' | 'neutral' | 'love'

export interface ChatMessage {
  id: number
  personaId: number
  role: MessageRole
  content: string
  emotionLabel: EmotionLabel | null   // 内部信号，界面不展示
  emotionScore: number | null         // 同上，不展示
  isNudge: boolean                    // 是否为主动消息注入；未读红点靠它区分
  createdAt: string
}
```

> **没有 `ChatSession` 类型**：一个人设只有一个对话，会话列表就是人设列表。

5. **组件 props / emits 显式声明类型**：

```ts
const props = defineProps<{
  message: ChatMessage
  streaming?: boolean
}>()

const emit = defineEmits<{
  (e: 'retry', messageId: number): void
}>()
```

### 3.3 路由管理与路由守卫

> 硬性要求：使用 Route 进行路由管理，配置路由守卫。

**路由表**（`src/router/index.ts`）：

```ts
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { requiresAuth: false, title: '登录' },
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: { requiresAuth: false, title: '注册' },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/chat',
    meta: { requiresAuth: true },
    children: [
      // 一个人设 = 一个对话，personaId 决定打开哪个对话；缺省时自动切到最近对话的人设
      { path: 'chat/:personaId?', name: 'Chat', component: () => import('@/views/chat/ChatView.vue'), props: true, meta: { title: '对话' } },
      { path: 'personas', name: 'Persona', component: () => import('@/views/persona/PersonaView.vue'), meta: { title: '人设管理' } },
      { path: 'profile', name: 'Profile', component: () => import('@/views/profile/ProfileView.vue'), meta: { title: '用户画像' } },
      { path: 'moments', name: 'Moments', component: () => import('@/views/moments/MomentsView.vue'), meta: { title: '朋友圈' } },
      // P1：日程提醒。路由先留着，时间不够就不实现这个页面（砍功能顺序见总纲 §0.3）
      { path: 'schedules', name: 'Schedule', component: () => import('@/views/schedule/ScheduleView.vue'), meta: { title: '日程提醒' } },
    ],
  },
  // meta.requiresAuth: false —— 否则未登录用户访问不存在的路径会被守卫拦到登录页，看不到 404
  { path: '/:pathMatch(.*)*', name: 'NotFound', component: () => import('@/views/error/NotFoundView.vue'), meta: { requiresAuth: false, title: '页面不存在' } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
```

**全局前置守卫**（`src/router/guards.ts`）：

```ts
import type { Router } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

export function setupRouterGuards(router: Router) {
  // 全局前置守卫：登录态校验
  router.beforeEach(async (to) => {
    const authStore = useAuthStore()

    // 1. 恢复持久化的登录态（首次进入时从 localStorage 恢复）
    if (!authStore.initialized) {
      await authStore.restore()
    }

    // 2. 需要登录但未登录 → 跳转登录页，并记录来源路径
    if (to.meta.requiresAuth !== false && !authStore.isLoggedIn) {
      ElMessage.warning('请先登录')
      return { name: 'Login', query: { redirect: to.fullPath } }
    }

    // 3. 已登录时访问登录/注册页 → 重定向回首页
    if (authStore.isLoggedIn && (to.name === 'Login' || to.name === 'Register')) {
      return { name: 'Chat' }
    }

    // 4. 主动刷新：Access Token 即将过期时静默续期
    if (authStore.isLoggedIn && authStore.isAccessTokenExpiring) {
      try {
        await authStore.refreshToken()
      } catch {
        authStore.logout()
        return { name: 'Login', query: { redirect: to.fullPath } }
      }
    }

    return true
  })

  // 全局后置守卫：设置页面标题
  router.afterEach((to) => {
    document.title = to.meta.title ? `${to.meta.title} · HeartEcho` : 'HeartEcho'
  })
}
```

**路由 meta 类型扩展**（`src/types/router.d.ts`）：

```ts
import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** 是否需要登录，false 表示白名单页面 */
    requiresAuth?: boolean
    /** 页面标题 */
    title?: string
  }
}
```

### 3.4 登录状态持久化

> 硬性要求 3：前端实现登录状态持久化。

**持久化方案对比与选择**：

| 方案 | 说明 | 采用 |
|------|------|------|
| localStorage 存 Token | 简单直观，XSS 可读取；配合后端短有效期 Access Token 风险可控 | ✅ 采用 |
| HttpOnly Cookie 存 Refresh Token | 更安全，但跨域与 CSRF 配置复杂，不适合 4 周项目 | ❌ |
| 内存 + 刷新即失效 | 安全性最好，但体验差，刷新页面即掉登录 | ❌ |

**决策**：Access Token 存 `localStorage`，Refresh Token 也存 `localStorage` 但**仅在刷新接口使用**；Access Token 有效期 2 小时，Refresh Token 7 天。生产环境若有时间，可升级为 Refresh Token 走 HttpOnly Cookie。

**Pinia Store**（`src/stores/auth.ts`）：

```ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import type { UserInfo, LoginPayload } from '@/types/user'

const ACCESS_TOKEN_KEY = 'heart_echo_access_token'
const REFRESH_TOKEN_KEY = 'heart_echo_refresh_token'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref<string>('')
  const refreshToken = ref<string>('')
  const userInfo = ref<UserInfo | null>(null)
  const initialized = ref(false)

  const isLoggedIn = computed(() => !!accessToken.value)

  /** 解析 JWT exp，判断 Access Token 是否将在 5 分钟内过期 */
  const isAccessTokenExpiring = computed(() => {
    if (!accessToken.value) return false
    const payload = JSON.parse(atob(accessToken.value.split('.')[1]))
    return payload.exp * 1000 - Date.now() < 5 * 60 * 1000
  })

  /** 应用启动时从 localStorage 恢复登录态 */
  async function restore() {
    accessToken.value = localStorage.getItem(ACCESS_TOKEN_KEY) ?? ''
    refreshToken.value = localStorage.getItem(REFRESH_TOKEN_KEY) ?? ''
    if (accessToken.value) {
      try {
        userInfo.value = await authApi.getCurrentUser()
      } catch {
        await logout()
      }
    }
    initialized.value = true
  }

  async function login(payload: LoginPayload) {
    const res = await authApi.login(payload)
    setTokens(res.accessToken, res.refreshToken)
    userInfo.value = res.user
  }

  async function refreshTokenSilently() {
    const res = await authApi.refresh(refreshToken.value)
    setTokens(res.accessToken, res.refreshToken)
  }

  function setTokens(access: string, refresh: string) {
    accessToken.value = access
    refreshToken.value = refresh
    localStorage.setItem(ACCESS_TOKEN_KEY, access)
    localStorage.setItem(REFRESH_TOKEN_KEY, refresh)
  }

  async function logout() {
    accessToken.value = ''
    refreshToken.value = ''
    userInfo.value = null
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  return {
    accessToken, refreshToken, userInfo, initialized,
    isLoggedIn, isAccessTokenExpiring,
    restore, login, refreshToken: refreshTokenSilently, logout,
  }
})
```

> 注意：上例手写 `localStorage` 读写，也可使用 `pinia-plugin-persistedstate` 简化。**不允许把密码、明文敏感信息写入 localStorage。**

### 3.5 Axios 拦截器：统一注入 Token、统一处理错误码

```ts
// src/api/request.ts
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'
import { ErrorCode } from '@/types/errcode'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL, // 生产是 http://<公网IP>/api/v1（不买域名、不做 HTTPS）
  timeout: 15000,
})

// 是否正在刷新 Token，避免并发请求重复刷新
let isRefreshing = false
let pendingQueue: Array<(token: string) => void> = []

request.interceptors.request.use((config) => {
  const authStore = useAuthStore()
  if (authStore.accessToken) {
    config.headers.Authorization = `Bearer ${authStore.accessToken}`
  }
  return config
})

request.interceptors.response.use(
  // 后端 HTTP 状态码为 200，业务错误通过 code 表达
  (response) => {
    const body = response.data
    if (body.code === ErrorCode.Success) return body.data
    ElMessage.error(body.message)
    return Promise.reject(new Error(body.message))
  },
  async (error) => {
    const { response, config } = error
    if (!response) {
      ElMessage.error('网络异常，请检查网络连接')
      return Promise.reject(error)
    }

    // 401 → 尝试用 Refresh Token 换新 Access Token，并重放原请求
    if (response.status === 401 && !config._retried) {
      const authStore = useAuthStore()
      if (isRefreshing) {
        // 排队等待刷新完成
        return new Promise((resolve) => {
          pendingQueue.push((token: string) => {
            config.headers.Authorization = `Bearer ${token}`
            config._retried = true
            resolve(request(config))
          })
        })
      }
      isRefreshing = true
      try {
        await authStore.refreshToken()
        const newToken = authStore.accessToken
        pendingQueue.forEach((cb) => cb(newToken))
        pendingQueue = []
        config.headers.Authorization = `Bearer ${newToken}`
        config._retried = true
        return request(config)
      } catch {
        await authStore.logout()
        router.push({ name: 'Login' })
        ElMessage.error('登录已过期，请重新登录')
        return Promise.reject(error)
      } finally {
        isRefreshing = false
      }
    }

    ElMessage.error(response.data?.message ?? '请求失败')
    return Promise.reject(error)
  },
)

export default request
```

### 3.6 SSE 流式对话的前端实现

**关键决策**：原生 `EventSource` **只支持 GET 且无法自定义 Header**，无法携带 `Authorization`。因此采用 **`fetch` + `ReadableStream` 手动解析 SSE**，以 POST 发送消息并在 Header 中带 Token。

```ts
// src/api/chat.ts
import type { ChatMessage } from '@/types/chat'

/** 请求体：与契约 §6 的 SSE 请求一致 */
export interface StreamChatPayload {
  personaId: number
  content: string
}

/** done 事件载荷：与契约 §6 一致 */
export interface DonePayload {
  messageId: number
}

export interface StreamHandlers {
  onDelta: (text: string) => void
  onDone: (r: DonePayload) => void
  onError: (e: { code: number; message: string }) => void
}

export async function streamChat(
  payload: StreamChatPayload,
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const token = localStorage.getItem('heart_echo_access_token')
  const res = await fetch(`${import.meta.env.VITE_API_BASE_URL}/chat/stream`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(payload),
    signal,
  })

  if (!res.ok || !res.body) {
    handlers.onError({ code: 5002, message: `SSE 连接失败: ${res.status}` })
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    // SSE 事件以空行分隔
    const events = buffer.split('\n\n')
    buffer = events.pop() ?? ''

    for (const raw of events) {
      const eventMatch = raw.match(/^event:\s*(.+)$/m)
      const dataMatch = raw.match(/^data:\s*(.+)$/m)
      if (!dataMatch) continue
      const eventName = eventMatch?.[1]?.trim() ?? 'message'
      const data = JSON.parse(dataMatch[1])

      if (eventName === 'delta') handlers.onDelta(data.text)
      else if (eventName === 'done') handlers.onDone(data)          // { messageId }
      else if (eventName === 'error') handlers.onError(data)        // { code, message }，不要把 code 丢掉
    }
  }
}
```

**边界处理**：

| 场景 | 处理 |
|------|------|
| 用户中途停止生成 | `AbortController.abort()`，前端保留已生成的部分内容 |
| 网络中断 | 捕获异常，提示"回复中断"，展示重试按钮 |
| 服务端生成失败 | 后端发送 `event: error`，前端移除"正在输入"气泡并提示 |
| 消息乱序/粘包 | 用缓冲区按 `\n\n` 切分，不完整片段留在 buffer |

**打字机效果**：`onDelta` 直接追加文本即可自然形成打字机效果；若模型返回的 chunk 过大，可再用 `requestAnimationFrame` 做逐字渲染队列，避免一次刷出大段文字。

---

## 4. 后端设计

> 对应硬性要求 2：**Go 技术栈 + 业务异常统一响应中间件 + 统一错误码（一 code 一 msg）**。

### 4.1 分层结构

```
handler（HTTP 层：参数绑定、校验、调用 service、返回响应）
    ↓
service（业务逻辑：事务编排、调用 repository / AI 服务）
    ↓
repository（数据访问：只做 CRUD，不含业务判断）
    ↓
model（数据模型：GORM 实体）
```

**分层纪律**：

- handler 不直接操作数据库，只通过 service。
- service 不感知 `*gin.Context`（除了传 `context.Context`），保证可测试。
- repository 只返回数据与 `error`，业务错误由 service 转换成 `errcode.BizError`。

### 4.2 统一响应结构

```go
// pkg/response/response.go
package response

type Response[T any] struct {
    Code      int    `json:"code"`
    Message   string `json:"message"`
    Data      T      `json:"data"`
    Timestamp int64  `json:"timestamp"`
}

func Success[T any](c *gin.Context, data T) {
    c.JSON(http.StatusOK, Response[T]{
        Code:      int(errcode.Success),
        Message:   errcode.Success.Message(),
        Data:      data,
        Timestamp: time.Now().UnixMilli(),
    })
}

// Fail 只接受错误码，message 一律从错误码表查，禁止调用方自定义文案
func Fail(c *gin.Context, code errcode.ErrorCode) {
    c.JSON(code.HTTPStatus(), Response[any]{
        Code:      int(code),
        Message:   code.Message(),
        Data:      nil,
        Timestamp: time.Now().UnixMilli(),
    })
}
```

**设计要点**：

- **HTTP 状态码与业务 code 分离**：HTTP 层用 `code.HTTPStatus()` 决定状态码（便于网关/浏览器识别），响应体里的 `code` 表达具体业务语义。
- **`Fail` 不接收 message 参数**，从根上保证"一个 code 对应一个 msg"，杜绝同一个错误码在不同地方文案不一致。

### 4.3 统一错误码（一 code 一 msg）

```go
// pkg/errcode/errcode.go
package errcode

type ErrorCode int

const (
    // 成功
    Success ErrorCode = 200

    // 4xxx 客户端错误
    ErrInvalidParams ErrorCode = 4001 // 参数校验失败
    ErrParamMissing  ErrorCode = 4002 // 必填参数缺失
    ErrEmailExists   ErrorCode = 4003 // 该邮箱已被注册
    ErrUsernameTaken ErrorCode = 4004 // 该用户名已被占用

    // 行尾注释必须与下方 codeMessages 逐字一致
    ErrUnauthorized     ErrorCode = 4010 // 未登录或登录已过期
    ErrTokenInvalid     ErrorCode = 4011 // Token 无效
    ErrTokenExpired     ErrorCode = 4012 // Token 已过期
    ErrPasswordWrong    ErrorCode = 4013 // 用户名或密码错误
    ErrRefreshInvalid   ErrorCode = 4014 // 刷新令牌无效，请重新登录
    ErrOldPasswordWrong ErrorCode = 4015 // 原密码不正确

    // 4030 只用于功能层面的越权（封禁用户、无权限的功能）。
    // 资源越权（访问他人 persona）按「人设不存在」返回 4043，不走这里。
    ErrForbidden ErrorCode = 4030 // 无权限使用该功能

    ErrNotFound         ErrorCode = 4040 // 资源不存在
    ErrUserNotFound     ErrorCode = 4041 // 用户不存在
    // 4042 原「会话不存在」已废弃：一个人设只有一个对话，不存在会话实体
    ErrPersonaNotFound  ErrorCode = 4043 // 人设不存在

    // 5xxx 服务端错误
    ErrInternal      ErrorCode = 5000 // 服务端内部错误
    ErrLLMFailed     ErrorCode = 5001 // AI 回复生成失败，请稍后重试
    ErrAIUnavailable ErrorCode = 5002 // AI 服务暂时不可用
    ErrDBFailed      ErrorCode = 5003 // 数据库操作失败
)

// 唯一数据源：一个 code 严格对应一个 msg
var codeMessages = map[ErrorCode]string{
    Success: "success",

    ErrInvalidParams: "参数校验失败",
    ErrParamMissing:  "必填参数缺失",
    ErrEmailExists:   "该邮箱已被注册",
    ErrUsernameTaken: "该用户名已被占用",

    ErrUnauthorized:    "未登录或登录已过期",
    ErrTokenInvalid:    "Token 无效",
    ErrTokenExpired:    "Token 已过期",
    ErrPasswordWrong:   "用户名或密码错误",
    ErrRefreshInvalid:  "刷新令牌无效，请重新登录",
    ErrOldPasswordWrong: "原密码不正确",

    ErrForbidden: "无权限使用该功能",

    ErrNotFound:        "资源不存在",
    ErrUserNotFound:    "用户不存在",
    ErrPersonaNotFound: "人设不存在",

    ErrInternal:      "服务端内部错误",
    ErrLLMFailed:     "AI 回复生成失败，请稍后重试",
    ErrAIUnavailable: "AI 服务暂时不可用",
    ErrDBFailed:      "数据库操作失败",
}

// 一个 code 对应一个 HTTP 状态码
var codeHTTPStatus = map[ErrorCode]int{
    Success: http.StatusOK,

    ErrInvalidParams: http.StatusBadRequest,
    ErrParamMissing:  http.StatusBadRequest,
    ErrEmailExists:   http.StatusBadRequest,
    ErrUsernameTaken: http.StatusBadRequest,

    ErrUnauthorized:   http.StatusUnauthorized,
    ErrTokenInvalid:   http.StatusUnauthorized,
    ErrTokenExpired:   http.StatusUnauthorized,
    ErrPasswordWrong:  http.StatusUnauthorized,
    ErrRefreshInvalid: http.StatusUnauthorized,
    // 4015 必须登记：漏登记会走 HTTPStatus() 的兜底分支，改密码失败变成 HTTP 500
    ErrOldPasswordWrong: http.StatusUnauthorized,

    ErrForbidden: http.StatusForbidden,

    ErrNotFound:        http.StatusNotFound,
    ErrUserNotFound:    http.StatusNotFound,
    ErrPersonaNotFound: http.StatusNotFound,

    ErrInternal:      http.StatusInternalServerError,
    ErrLLMFailed:     http.StatusInternalServerError,
    ErrAIUnavailable: http.StatusInternalServerError,
    ErrDBFailed:      http.StatusInternalServerError,
}

// Message 返回该错误码唯一对应的提示文案
func (e ErrorCode) Message() string {
    if msg, ok := codeMessages[e]; ok {
        return msg
    }
    return "未知错误"
}

// HTTPStatus 返回该错误码对应的 HTTP 状态码
func (e ErrorCode) HTTPStatus() int {
    if status, ok := codeHTTPStatus[e]; ok {
        return status
    }
    return http.StatusInternalServerError
}

// BizError 业务异常，service 层统一抛出
type BizError struct {
    Code ErrorCode
    Err  error // 原始错误，仅用于日志，不返回给前端
}

func (e *BizError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("[%d] %s: %v", e.Code, e.Code.Message(), e.Err)
    }
    return fmt.Sprintf("[%d] %s", e.Code, e.Code.Message())
}

func New(code ErrorCode) *BizError            { return &BizError{Code: code} }
func Wrap(code ErrorCode, err error) *BizError { return &BizError{Code: code, Err: err} }
```

**错误码分段规则**（新增错误码时必须遵守）：

| 段位 | 含义 | 范围 |
|------|------|------|
| 200 | 成功 | 200 |
| 4000-4009 | 参数/注册类错误 | 参数校验、字段冲突 |
| 4010-4019 | 认证类错误 | Token、密码 |
| 4030-4039 | 授权类错误 | **功能**越权（资源越权归入 4040-4049「不存在」） |
| 4040-4049 | 资源不存在 | 各类 Not Found |
| 5000-5009 | 服务端错误 | 内部、DB、LLM、AI 服务 |

**新增错误码流程**：① 在 `errcode.go` 常量区按段位分配；② 同时在 `codeMessages` 与 `codeHTTPStatus` 中登记；③ 跑 `errcode` 包的单测（校验两个 map 的 key 集合与常量集合一致），防止漏登记。

### 4.4 统一响应中间件 & 全局异常捕获

```go
// internal/middleware/recovery.go
package middleware

// Recovery 全局 panic 捕获，统一转为 5000 响应，避免服务崩溃
func Recovery(logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                logger.Error("panic recovered",
                    zap.String("traceId", c.GetString(ContextKeyTraceID)),
                    zap.Any("error", err),
                    zap.String("path", c.Request.URL.Path),
                    zap.String("method", c.Request.Method),
                    zap.Stack("stack"),
                )
                // 响应头已发出（如 SSE 流到一半崩溃）时不能再写 JSON，会搅乱响应体
                if !c.Writer.Written() {
                    response.Fail(c, errcode.ErrInternal) // 不向前端泄漏堆栈
                }
                c.Abort()
            }
        }()
        c.Next()
    }
}
```

```go
// internal/middleware/biz_error.go
// BizError 业务异常出口：放在所有 handler 之后，统一把 *errcode.BizError 转成响应
func BizErrorHandler(logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next() // 先执行后续 handler

        if len(c.Errors) == 0 {
            return
        }
        err := c.Errors.Last().Err
        traceID := c.GetString(ContextKeyTraceID)

        // 用 errors.As 而非类型断言：错误可能被 fmt.Errorf("%w") 包过一层
        var bizErr *errcode.BizError
        if errors.As(err, &bizErr) {
            // 5xxx 记 error 级日志并带原始错误；4xxx 记 warn 级即可
            if bizErr.Code >= 5000 {
                logger.Error("business error",
                    zap.String("traceId", traceID),
                    zap.String("codeMsg", bizErr.Code.Message()),
                    zap.Error(bizErr.Err))
            } else {
                logger.Warn("business error",
                    zap.String("traceId", traceID),
                    zap.String("codeMsg", bizErr.Code.Message()))
            }
            response.Fail(c, bizErr.Code)
            return
        }

        logger.Error("unknown error", zap.String("traceId", traceID), zap.Error(err))
        response.Fail(c, errcode.ErrInternal)
    }
}
```

**Service 层抛错示例**：

```go
func (s *userService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
    user, err := s.userRepo.FindByUsername(ctx, req.Username)
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, errcode.New(errcode.ErrPasswordWrong) // 不暴露"用户不存在"，防止用户名枚举
    }
    if err != nil {
        return nil, errcode.Wrap(errcode.ErrDBFailed, err)
    }
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        return nil, errcode.New(errcode.ErrPasswordWrong)
    }
    // 注意：bcrypt 输入上限为 72 字节，超长密码会被静默截断。
    // 因此 RegisterRequest 的 password 用 max=32 限制的是字符数，
    // 若允许中文等多字节字符需另行按字节数校验，详见 §4.8。
    // ...
}
```

**Handler 层写法**（薄，只做绑定与错误上抛）：

```go
func (h *UserHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        _ = c.Error(errcode.New(errcode.ErrInvalidParams))
        return
    }
    res, err := h.userService.Login(c.Request.Context(), &req)
    if err != nil {
        _ = c.Error(err)
        return
    }
    response.Success(c, res)
}
```

> 注意：handler 中**不写** `response.Fail`，统一交给 `BizErrorHandler` 中间件出口，保证所有错误路径一致。

### 4.5 中间件链顺序

```
Request
  → Recovery        (panic 捕获，必须最外层)
  → RequestLogger   (访问日志、耗时、traceId)
  → CORS            (跨域，预检请求直接放行)
  → BizErrorHandler (业务错误出口，注册在路由组上)
  → JWTAuth         (白名单路由除外，解析并校验 Token)
  → Handler
Response
```

```go
// internal/middleware/cors.go
func CORS(allowedOrigins []string) gin.HandlerFunc {
    return cors.New(cors.Config{
        AllowOrigins:     allowedOrigins,
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    })
}
```

**⚠️ CORS 与 SSE**：生产环境用 Nginx 同源代理（前端与 API 同域），可完全绕开跨域问题；本地开发才依赖 CORS。SSE 响应需要 `X-Accel-Buffering: no`，否则 Nginx 会缓冲导致"不流式"。

### 4.6 环境变量清单

> 三个服务各有自己的 `.env.example`，**入仓库的只有 `.env.example`，`.env` 必须在 `.gitignore` 中**。

**`backend/.env.example`**

```bash
# 服务
SERVER_PORT=8080
GIN_MODE=debug                 # debug | release

# 数据库
DB_HOST=localhost
DB_PORT=5432
DB_USER=heart_echo
DB_PASSWORD=change_me
DB_NAME=heart_echo
DB_SSLMODE=disable

# JWT（密钥必须 ≥32 字符，各环境不同，禁止用示例值上线）
JWT_SECRET=please_change_this_to_a_long_random_string
JWT_ACCESS_EXPIRE=2h
JWT_REFRESH_EXPIRE=168h

# AI 服务
AI_SERVICE_URL=http://localhost:8000
AI_SERVICE_TOKEN=internal_token_change_me    # 必须与 ai-service 一致

# CORS（本地开发用；生产走 Nginx 同源代理，不需要）
CORS_ALLOW_ORIGINS=http://localhost:5173

# ===== 定时任务间隔（定时任务跑在 Go 侧，不在 Python 侧）=====
# 朋友圈无手动触发接口，这个值是答辩现场唯一的兜底旋钮：演示环境调到 2m
MOMENT_JOB_INTERVAL=30m
# 主动消息与日程提醒共用（两者都是"注入 nudge 走正常链路"）
PROACTIVE_JOB_INTERVAL=5m
```

**`ai-service/.env.example`**

```bash
AI_SERVICE_PORT=8000
AI_SERVICE_TOKEN=internal_token_change_me    # 必须与 backend 一致

DEEPSEEK_API_KEY=sk-xxxxxxxxxxxxxxxx
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-chat
LLM_TIMEOUT_SECONDS=60
LLM_MAX_TOKENS=512             # 开发期调小，控制成本

# ===== 阶段一/阶段二开关（阶段二只改这里）=====
EMOTION_BACKEND=lexicon        # lexicon | onnx
MEMORY_BACKEND=recent          # recent | vector
MEMORY_EXTRACT_BACKEND=rule    # rule | llm
SCHEDULE_PARSE_BACKEND=rule    # rule | llm（P1：时间解析，做不动就保持 rule）

# ===== 以下仅在阶段二需要 =====
# EMOTION_MODEL_PATH=./models/sentiment-8class.onnx
# CHROMA_HOST=localhost          # 容器内用服务名 chroma，不是 localhost
# CHROMA_PORT=8001               # 宿主机映射端口；容器内仍是 8000，避开 AI 服务的 8000
```

**`frontend/.env.example`**

```bash
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

**生产环境 `deploy/.env`**（docker compose 读取）：包含 `POSTGRES_*`、`AI_SERVICE_TOKEN`、`DEEPSEEK_API_KEY`、`JWT_SECRET`、`MOMENT_JOB_INTERVAL`、`PROACTIVE_JOB_INTERVAL`、`SCHEDULE_PARSE_BACKEND` 等。**服务器上此文件权限设为 `600`，永不入库。**

### 4.7 JWT 双令牌鉴权

> 对应硬性要求 3：后端要求使用常见的鉴权方式 → **JWT Bearer Token**。

**令牌设计**：

| 令牌 | 有效期 | 存储位置（前端） | 用途 |
|------|--------|------------------|------|
| Access Token | 2 小时 | localStorage | 每次业务请求携带 |
| Refresh Token | 7 天 | localStorage | 仅用于 `/auth/refresh` 换新令牌 |

**Claims 结构**：

```go
// pkg/jwt/jwt.go
type Claims struct {
    UserID    uint64 `json:"uid"` // 与 internal/model.User.ID 保持同一类型
    Username  string `json:"uname"`
    TokenType string `json:"typ"` // "access" | "refresh"
    jwt.RegisteredClaims
}

// 有效期一律由调用方从配置传入，禁止在代码里写死——否则会与 .env 的
// JWT_ACCESS_EXPIRE / JWT_REFRESH_EXPIRE 形成两个事实来源。
// 传 time.Duration 而不是 config.JWTConfig，是为了让 pkg/ 不反向依赖 internal/。
func GenerateTokenPair(
    userID uint64,
    username, secret string,
    accessTTL, refreshTTL time.Duration,
) (*TokenPair, error) {
    now := time.Now()
    accessClaims := Claims{
        UserID: userID, Username: username, TokenType: "access",
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    "heart-echo",
            Subject:   strconv.FormatUint(userID, 10),
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
        },
    }
    // refresh token 同理，typ = "refresh"，有效期取 refreshTTL
    // ...
}
```

**鉴权中间件**：

```go
// internal/middleware/jwt.go
func JWTAuth(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            _ = c.Error(errcode.New(errcode.ErrUnauthorized))
            c.Abort()
            return
        }
        tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := jwt.ParseToken(tokenStr, secret) // pkg/jwt，不直接引 golang-jwt
        if err != nil {
            // 判 pkg/jwt 暴露的哨兵错误，调用方不需要 import 底层库
            if errors.Is(err, jwt.ErrTokenExpired) {
                _ = c.Error(errcode.New(errcode.ErrTokenExpired))
            } else {
                _ = c.Error(errcode.New(errcode.ErrTokenInvalid))
            }
            c.Abort()
            return
        }
        // refresh token 不能用于访问业务接口
        if claims.TokenType != "access" {
            _ = c.Error(errcode.New(errcode.ErrTokenInvalid))
            c.Abort()
            return
        }
        c.Set(ContextKeyUserID, claims.UserID)
        c.Set(ContextKeyUsername, claims.Username)
        c.Next()
    }
}
```

**密码存储**：使用 `bcrypt`（cost ≥ 10），**严禁明文或 MD5 存储**。

**已知取舍**：JWT 无状态、无法主动吊销。本项目接受该取舍；如后续需要"登出即失效"，可引入 Redis 黑名单（存 `jti`，TTL 设为 Token 剩余有效期）。

### 4.8 参数校验

使用 Gin 内置的 `binding` tag，配合 `validator`：

```go
type RegisterRequest struct {
    Username string `json:"username" binding:"required,min=3,max=20,alphanum"`
    Email    string `json:"email"    binding:"required,email"`
    Password string `json:"password" binding:"required,min=8,max=32"`
}
```

校验未通过统一返回 `4001 ErrInvalidParams`；如需更精细的字段级提示，可在 `ShouldBindJSON` 失败时解析 `validator.ValidationErrors` 生成 detail 字段（但 `message` 仍为 `4001` 的唯一文案）。

> ⚠️ **bcrypt 的 72 字节上限**：`max=32` 限制的是**字符数**，而 bcrypt 只取输入的前 72 **字节**，超出部分被静默丢弃。纯中文密码 24 个字符就可能超过 72 字节，导致「两个不同密码哈希相同」。若要支持中文密码，需额外校验 `len([]byte(password)) <= 72`；本项目若限制密码为 ASCII 可见字符，则 32 字符 < 72 字节，无此问题——**在注册接口明确限定字符集即可**。

### 4.9 配置管理

```go
// internal/config/config.go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    JWT      JWTConfig
    AI       AIConfig
}

type JWTConfig struct {
    Secret             string        `env:"JWT_SECRET,required"`
    AccessTokenExpire  time.Duration `env:"JWT_ACCESS_EXPIRE" envDefault:"2h"`
    RefreshTokenExpire time.Duration `env:"JWT_REFRESH_EXPIRE" envDefault:"168h"`
}
```

使用 `github.com/caarlos0/env` + `.env` 文件加载，`config.yaml` 仅存非敏感默认值。**所有密钥通过环境变量注入**，`.env` 加入 `.gitignore`，仓库中只保留 `.env.example`。

---

## 5. AI 服务设计

> 核心创新点：**三 Agent 协作架构**。

### 5.0 实现分级与升级路径（核心设计）

> **交付策略**：核心链路先用最直接的实现打通，再在**不改变接口**的前提下替换为完整实现。
> 这样前端和后端不会因为 AI 侧的复杂度而阻塞，阶段二替换实现时**调用方一行不用改**。

#### 阶段划分总表

| 能力 | 阶段一（Week 2-3） | 阶段二（Week 3-4） | 切换方式 |
|------|-------------------|-------------------|----------|
| 情感分析 | 关键词词典，8 类情绪 | ONNX 小模型推理 | 改环境变量 `EMOTION_BACKEND` |
| 记忆检索 | PostgreSQL：最近 N 条 + 关键词匹配 | ChromaDB 向量语义检索 | 改环境变量 `MEMORY_BACKEND` |
| 记忆提取 | 规则匹配（正则 + 关键词模板） | LLM 结构化提取 | 改环境变量 `MEMORY_EXTRACT_BACKEND` |
| 人格演化 | 对话轮数驱动的亲密度（§5.6） | 亲密度 + AI 自述补充 | `personas.state.familiarity` |
| 朋友圈评论 | 固定风格模板生成 | Agent 自主生成 | 服务内策略切换 |
| 日程时间解析（P1，§5.7） | 规则档（正则 + 句式模板） | LLM 结构化解析 | 改环境变量 `SCHEDULE_PARSE_BACKEND` |

**推进纪律**：阶段二每替换一项，当天收工前必须保证服务可运行、可演示。**始终在可工作的版本上迭代**，不允许出现"升级到一半、周末不能用"的状态。若某项替换遇到阻塞，先切回阶段一实现（改回环境变量即可），继续推进其他项。

#### 关键：接口抽象（Python 侧）

**情感分类器**——阶段一用词典，阶段二换 ONNX，调用方完全无感：

```python
# app/classifiers/base.py
from typing import Protocol
from dataclasses import dataclass

EMOTION_LABELS = ("joy", "sadness", "anger", "fear",
                  "surprise", "disgust", "neutral", "love")

@dataclass
class EmotionResult:
    label: str
    score: float   # 0.0 ~ 1.0

class EmotionClassifier(Protocol):
    """情感分类器接口。所有实现必须满足此契约。"""
    def classify(self, text: str) -> EmotionResult: ...
```

```python
# app/classifiers/lexicon.py —— 阶段一实现，零依赖
import re
from .base import EmotionClassifier, EmotionResult, EMOTION_LABELS

# 8 类情绪的关键词表（可随时补充）
LEXICON: dict[str, list[str]] = {
    "joy":      ["开心", "高兴", "太好了", "哈哈", "喜欢", "棒", "幸福", "满足", "爽"],
    "sadness":  ["难过", "伤心", "累", "哭", "失落", "沮丧", "委屈", "孤单", "emo"],
    "anger":    ["生气", "烦", "讨厌", "气死", "愤怒", "滚", "无语", "受不了"],
    "fear":     ["害怕", "担心", "焦虑", "慌", "紧张", "恐惧", "不安", "压力"],
    "surprise": ["居然", "竟然", "没想到", "天呐", "震惊", "哇"],
    "disgust":  ["恶心", "反感", "嫌弃", "受不了", "烦人"],
    "love":     ["爱你", "想你", "宝贝", "亲亲", "抱抱", "离不开"],
}

NEGATION_WORDS = ("不", "没", "别", "无")

class LexiconEmotionClassifier:
    """基于关键词词典的情感分类，无需任何模型文件。"""

    def classify(self, text: str) -> EmotionResult:
        hits: dict[str, int] = {}
        for label, words in LEXICON.items():
            for word in words:
                idx = text.find(word)
                if idx == -1:
                    continue
                # 简单否定处理："不开心" 不计入 joy
                prefix = text[max(0, idx - 2):idx]
                if any(neg in prefix for neg in NEGATION_WORDS):
                    continue
                hits[label] = hits.get(label, 0) + 1

        if not hits:
            return EmotionResult(label="neutral", score=0.5)

        label = max(hits, key=hits.get)
        # 命中词数越多，置信度越高，封顶 0.95
        score = min(0.5 + 0.15 * hits[label], 0.95)
        return EmotionResult(label=label, score=round(score, 3))
```

```python
# app/classifiers/__init__.py —— 工厂：靠环境变量切换实现
from app.core.config import settings
from .base import EmotionClassifier, EmotionResult, EMOTION_LABELS

def get_classifier() -> EmotionClassifier:
    if settings.EMOTION_BACKEND == "onnx":
        from .onnx import OnnxEmotionClassifier   # 阶段二新增的文件
        return OnnxEmotionClassifier(settings.EMOTION_MODEL_PATH)
    from .lexicon import LexiconEmotionClassifier
    return LexiconEmotionClassifier()
```

> **阶段二动作**：新增 `app/classifiers/onnx.py`（约 50 行），把 `.env` 里 `EMOTION_BACKEND` 改成 `onnx`，重启服务。**其他代码一行不改。**

**记忆检索器**——同理，阶段一走 PostgreSQL，阶段二走向量库：

```python
# app/memory/base.py
from typing import Protocol
from dataclasses import dataclass

@dataclass
class Memory:
    id: int
    content: str
    memory_type: str      # fact | preference | event
    importance_score: float
    similarity: float = 0.0

class MemoryRetriever(Protocol):
    # persona_id 决定「是谁的记忆」，user_id 是越权防线，两个都要带
    async def retrieve(self, user_id: int, persona_id: int, query: str, top_k: int) -> list[Memory]: ...
```

```python
# app/memory/recent.py —— 阶段一实现：PostgreSQL 最近 N 条 + 关键词加权
class RecentMemoryRetriever:
    """取该人设最近记忆，按关键词重合度重排。

    覆盖显式事实的召回（"我养了只猫叫豆豆"）。隐式语义记忆
    （"最近工作压力大"）由阶段二的向量检索覆盖。
    """

    async def retrieve(self, user_id: int, persona_id: int, query: str, top_k: int = 5) -> list[Memory]:
        # 1. 取该人设最近 50 条记忆（WHERE persona_id = ? AND user_id = ?，按 importance_score + created_at 排序）
        candidates = await self._fetch_recent(user_id, persona_id, limit=50)

        # 2. 关键词重合度加权：query 中的词出现在记忆里则加分
        query_chars = set(query)
        for mem in candidates:
            overlap = len(query_chars & set(mem.content))
            mem.similarity = overlap / max(len(query_chars), 1) + mem.importance_score * 0.5

        # 3. 按加权分排序取 Top-K
        candidates.sort(key=lambda m: m.similarity, reverse=True)
        return candidates[:top_k]
```

```python
# app/memory/__init__.py —— 工厂
def get_retriever() -> MemoryRetriever:
    if settings.MEMORY_BACKEND == "vector":
        from .vector import VectorMemoryRetriever   # 阶段二新增的文件
        return VectorMemoryRetriever()
    from .recent import RecentMemoryRetriever
    return RecentMemoryRetriever()
```

> **关于中文分词**：阶段一用 `set(query)` 按**单字**重合，对中文短句召回足够，且零依赖。引入 jieba 分词会略微提升精度，但相比阶段二的向量检索收益有限，不属于优先事项。

#### Go 侧对应的接口抽象

Go 后端只通过 HTTP 调用 Python 服务，**不需要知道 Python 用的是词典还是 ONNX、是 PG 还是 ChromaDB**。响应结构与字段保持稳定即可：

```go
// internal/service/ai_client.go
type EmotionResult struct {
    Label string  `json:"label"`
    Score float64 `json:"score"`
}

type AIClient interface {
    AnalyzeEmotion(ctx context.Context, text string) (*EmotionResult, error)
    StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
    ExtractMemories(ctx context.Context, req *ExtractRequest) ([]Memory, error)
}
```

**这就是阶段二替换不痛的关键**：契约稳定，实现可换。

#### 阶段二的推进顺序

| 顺序 | 项 | 前置条件 |
|------|-----|----------|
| ① | LLM 结构化记忆提取 | 规则提取已能跑通，且 API 余额充足 |
| ② | ChromaDB 向量检索 | 记忆表已有真实数据，阶段一检索稳定 |
| ③ | ONNX 小模型 | Python 环境已验证可加载模型（**先单独跑通一次推理再集成**） |
| ④ | 朋友圈 Agent 自主评论 | 朋友圈基础功能已上线 |

**纪律**：动手前确认当前版本已**提交、合并、可演示**；每完成一项立即 commit，保持随时可回退。

---

### 5.1 三 Agent 协作架构

| Agent | 职责 | 触发时机 | 阶段一实现 | 阶段二实现 |
|-------|------|----------|-----------|-----------|
| **情感分析 Agent** | 识别用户消息和 AI 回复的情绪类型、强度 | 每条消息到达时 | 关键词词典（`LexiconEmotionClassifier`） | ONNX 小模型 |
| **对话生成 Agent** | 基于情绪 + 记忆 + 人格状态生成共情回应 | 情感分析完成后 | 调用 DeepSeek API，prompt 注入记忆 | 同左 |
| **记忆管理 Agent** | 提取关键信息、判断是否写入长期记忆 | 对话结束后异步执行 | 规则提取（正则 + 句式模板） | LLM 结构化提取 |

> 三个 Agent 在代码中是 `ai-service/app/agents/` 下的三个独立模块，各自有明确的输入输出契约。**它们是"职责分离的三个模块"，不是"三个独立部署的服务"**——后者对三人团队是过度设计。

> **没有第 4 个 Agent**：日程提醒（§5.7）不是新 Agent，它是主动消息管线的一次复用，日程抽取挂在现有对话链路的异步流程里。**不要为它单开服务或单开 Agent。**

**一次对话的完整时序**：

```
用户发送消息
   │
   ▼
[Go 后端] 落库 user message → 调用 AI 服务 /chat/stream
   │
   ▼
[AI 服务] ──① 情感分析 Agent：用户消息 → 情绪标签 + 强度（内部信号，不下发前端）
   │            结果三个用途：写入消息表 / 驱动回复策略 / 供记忆提取打分
   ▼
         ──② 记忆检索：从 PostgreSQL 取**该人设**的记忆，按「最近 + 关键词重合度」排序
   │            取 Top-K（阶段二：改为 ChromaDB 向量相似度检索，调用方无感）
   ▼
         ──③ 对话生成 Agent：拼装 Prompt（人格 + 记忆 + 情绪）→ DeepSeek 流式生成
   │            （逐 chunk 下发 event: delta，前端打字机渲染）
   ▼
         ──④ 完整回复落库 → 下发 event: done
   │
   ▼
[异步]   ──⑤ 记忆管理 Agent：规则提取本轮对话中的事实 → 打分 → 超阈值写入
                user_memory 表（阶段二：同时写 ChromaDB 索引）
                同一步顺带做日程抽取（P1）：有明确提醒意图 + 时间可解析 → 返回 {content, remind_at}，
                由 Go 侧 schedule_repo 落库；解析不出 → 下一轮回复里回问澄清（见 §5.7）
```

### 5.2 Prompt 组装结构（对话生成 Agent）

```
[System]
你是 {persona.name}。
性格：{persona.personality_desc}
说话风格：{persona.speaking_style}
当前人格状态：{persona_state}   ← 人格演化后的微调参数

关于你正在陪伴的用户（**你与他之间**的长期记忆）：
- 事实记忆：{显式记忆 Top-N，如"用户叫小明""养了一只叫豆豆的猫"}
- 相关回忆：{向量检索召回 Top-K}
- 用户画像：{profile_data 摘要}

用户当前情绪：{emotion_label}（强度 {score}）
情绪应对策略：{根据情绪映射的共情策略}

要求：
1. 以第一人称自然对话，不要暴露你在读"记忆列表"，要像真人自然想起。
2. 回复长度控制在 1-3 句，避免说教。
3. 若用户情绪为 sadness/anger/fear，优先共情，不急着给建议。
4. 若本条用户消息以 [nudge] 开头，说明这是系统在你主动找人聊天，不是用户刚发的，
   请结合最近对话与记忆，自然地问候或开启一个话题，不要提及 [nudge] 本身。
[History]
{最近 N 轮对话}
[User]
{当前消息}
```

### 5.3 记忆系统设计

**阶段一（Week 3）：单库方案 —— PostgreSQL 一张表**

```
每轮对话结束
  → 记忆管理 Agent 分析对话（阶段一：规则提取）
  → 判断是否包含值得记住的信息（打分 0-1）
  → 候选记忆
  → 重要性阈值判断（≥0.6 写入）
  → 写入 PostgreSQL user_memory 表
  → 下次对话时按「最近 + 关键词重合度」召回 Top-5，注入 Prompt
  → 阶段二：额外写入 ChromaDB，召回升级为向量相似度检索
```

**阶段一记忆提取：规则实现**（约 100 行，零 LLM 成本）

```python
# app/agents/memory_agent.py
import re

# 句式 → (抽取槽位, 记忆类型)。记忆类型只能是 fact | preference | event。
FACT_PATTERNS: list[tuple[str, str, str]] = [
    (r"我(?:的名字)?叫([^\s，。！？]{1,10})",        "user_name",  "fact"),
    (r"我(?:养了|有一只)([^\s，。！？]{1,15})",       "pet",        "fact"),
    (r"我(?:喜欢|爱|最喜欢)([^\s，。！？]{1,20})",    "preference", "preference"),
    (r"我(?:是|在)([^\s，。！？]{1,15})(?:工作|上班)", "occupation", "fact"),
    (r"我(?:住在|家在)([^\s，。！？]{1,15})",         "location",   "fact"),
]

class RuleMemoryExtractor:
    """基于句式的记忆提取。覆盖演示所需的常见事实，零成本、零延迟。"""

    def extract(self, user_message: str) -> list[dict]:
        memories: list[dict] = []
        for pattern, kind, memory_type in FACT_PATTERNS:
            match = re.search(pattern, user_message)
            if match:
                memories.append({
                    # 不能一律写 "fact"：偏好类句式要落成 preference，否则画像页分类是错的
                    "memory_type": memory_type,
                    "content": match.group(0),          # 例："我养了一只叫豆豆的猫"
                    "importance_score": 0.8,
                    "source": kind,
                })
        return memories
```

> 正则句式（"我叫X"、"我喜欢X"）覆盖显式事实的提取。隐式表达（"最近老加班到十点"）需要 LLM 结构化提取，排入阶段二。

**记忆的生命周期**

| 阶段 | 阶段一实现 | 阶段二实现 |
|------|-----------|-----------|
| 提取 | 正则匹配触发句式 | LLM 结构化提取 |
| 打分 | 句式命中即固定分（如 0.8） | LLM 返回 importance_score |
| 存储 | `user_memory` 表（带 `persona_id`） | 同左 + ChromaDB 索引 |
| 召回 | **该人设**最近 50 条 → 关键词重合度排序 → Top-5 | 向量相似度检索 Top-5（按 `persona_id` + `user_id` 过滤） |
| 注入 | 拼进 system prompt 的"关于你正在陪伴的用户"段落 | 同左 |

**阶段二：双轨设计**

| 轨道 | 存储 | 存什么 | 检索方式 |
|------|------|--------|----------|
| 显式记忆 | PostgreSQL | 事实性信息："用户叫小明"、"养了一只叫豆豆的猫" | 精确 / 关键词 |
| 隐式记忆 | ChromaDB | 语义记忆："用户最近工作压力大" | Embedding 向量相似度 |

**⚠️ 双写一致性风险（仅在阶段二遇到）**

> 阶段一不涉及双写，无需关心这一节。

接入时以 PostgreSQL 为**主**，ChromaDB 为**索引**：

1. 先写 PostgreSQL（拿到 `memory_id`）；
2. 再写 ChromaDB，`embedding_id = memory_id`；
3. ChromaDB 写失败**不回滚 PostgreSQL**，只记日志并标记 `embedding_status = 'pending'`；
4. 由一个补偿任务定期重试 `pending` 记录。

**演示话术**：PostgreSQL 是唯一事实来源，向量库是可重建的索引，因此不需要分布式事务。

**LLM 提取的输出契约**（阶段二用）：

```json
{
  "memories": [
    { "memory_type": "fact",       "content": "用户养了一只叫豆豆的猫",   "importance_score": 0.85 },
    { "memory_type": "event",      "content": "用户最近在忙项目上线",     "importance_score": 0.72 },
    { "memory_type": "preference", "content": "用户喜欢跑步",             "importance_score": 0.60 }
  ],
  "profile_updates": { "occupation": "程序员", "interests": ["跑步", "猫"] }
}
```

### 5.4 主动消息实现

**设计原则**：主动触发只负责**制造一次对话机会**，真正回复仍走原本聊天链路。这样避免为主动消息单独写一套生成逻辑，也能天然复用记忆与人格。

**流程**：

1. 后台定时任务（Go `robfig/cron`，每 5 分钟一次；**定时任务一律跑在 Go 侧，不用 Python APScheduler**）扫描 `proactive_settings`，联表拿到该人设的 `personas.last_message_at`
2. 检查该人设最后一条消息时间，超过随机阈值（`interval_min` ~ `interval_max`，默认 30-120 分钟）则触发
3. 向该人设的对话注入一条特殊的 `[nudge]` 用户消息（`role = 'user'`，`is_nudge = true`）
4. 模型正常读取上下文、记忆，生成回复
5. system prompt 告知模型：`[nudge]` 是自动注入的，不是用户本人刚打字

**效果**：主动消息内容基于当前上下文和记忆生成，而非写死模板，更像真人。

**🚨 必须有：手动触发接口（否则演示会翻车）**

定时任务最短也要等 30 分钟，答辩现场不可能等。**主动消息功能从第一天设计起就必须带一个手动触发入口**：

```
POST /api/v1/proactive/trigger     # 请求体：{ "personaId": 1 }
```

- 复用定时任务的同一段逻辑（抽成 `ProactiveService.TriggerNow(personaID)`），定时任务和手动接口都调它
- 前端在对话页放一个隐藏按钮（如连点标题 5 次，或仅开发环境显示）
- **演示前先验证这个按钮可用**

> 这不是"作弊"，而是标准的可测试性设计——任何依赖时间的逻辑都必须能被手动触发。

**防骚扰约束**：
- 每日主动消息上限（默认 3 条），超过则跳过
- 用户可全局关闭或在单人设上关闭
- 用户 1 小时内已回复过则不触发

### 5.5 AI 朋友圈（P1）

**功能设计**：

- 动态集中在**一张** `ai_moments` 表，按 `persona_id` 区分是哪个 AI 发的（没有分表）
- 定时任务触发 Agent 生成一条动态
- **同一个账号下的其他 AI 伴侣**随机挑动态评论
- 用户可以点赞和评论

**可见范围**：朋友圈是**账号内**的，不是社交平台。一个用户只能看到自己创建的 AI；账号与账号之间完全隔离，不存在跨用户的动态、评论或互相可见。跨用户社区是远期进阶选项，本期不做。

**核心**：让 Agent"自主产生内容"，而非预设模板。

**动态生成 Prompt 要点**：注入该 AI 的人设 + 最近对话中提取的"生活事件"（如"用户提到今天下雨"→ AI 可发"今天下雨了，突然想喝热可可"），使动态与陪伴关系产生呼应。

**分阶段实现**：

| 阶段 | 发动态 | 评论 |
|------|--------|------|
| 阶段一（Week 4） | 定时任务生成 | 固定风格模板随机挑选 |
| 阶段二 | 保持 | Agent 自主生成 |

**没有手动触发按钮**：动态只由定时任务产生。为了让答辩现场能看到，定时任务的执行间隔用环境变量 `MOMENT_JOB_INTERVAL` 控制，演示环境调到 2 分钟；同时演示前灌好一批历史动态作为兜底，保证就算定时任务没赶上，页面也不是空的。

### 5.6 人格演化（P1）

**做哪一档**：只做**亲密度**这一档，不做"AI 主动改写自己人设"。

**机制**：

- `personas.state` 里存一个 `familiarity` 数值（0-100），随对话轮数增长
- 每次对话结束后累加（如每轮 +1，上限 100）
- 按区间映射成"关系阶段"，拼进对话生成 Agent 的 system prompt，让说话方式随之变化

| familiarity | 关系阶段 | prompt 里的语气指令（示例） |
|:-----------:|----------|------------------------------|
| 0-19 | 初识 | 礼貌、克制，不要过分亲昵，不使用昵称 |
| 20-49 | 熟悉 | 可以开玩笑，语气自然放松 |
| 50-79 | 亲近 | 可以主动关心对方的生活，使用昵称 |
| 80-100 | 默契 | 语气随意，可以提起以前聊过的事，偶尔撒娇或吐槽 |

**为什么不做第三档（AI 改写自己人设）**：让 LLM 直接改 `personalityDesc` / `speakingStyle` 不可控且不可逆，答辩现场一旦跑偏无法解释、无法回退。亲密度是单向累加、可预期、可回退（改 `state` 即可）。

**时间够的话**：可以在亲密度基础上再加一条——亲密度到 50 以上时，允许 AI 自己追加一句"自我描述"存进 `state.self_note`，作为人设的次要补充。但这是加分项，**不进主线、不进验收**，也**不受 [总纲 §0.3](dev/MASTER.md#03-功能范围) 保护**——时间不够时第一个砍的就是它。

### 5.7 日程提醒（P1）

**先说清楚它不是第 4 个 Agent**：日程提醒**没有独立的人格、没有独立的服务、没有独立的生成链路**。它是 §5.4 主动消息管线的一次复用——把「空闲太久 → 注入 `[nudge]`」换成「到点了 → 注入 `[nudge]`」，后面的链路一模一样。如果有人提议给它单开一个 Agent 或一个 Python 服务，那是过度设计。

**用户看到的**：在对话里说「明天下午三点提醒我开会」，AI 应一声记下；到点后 AI 主动发来一条提醒消息，侧栏红点亮起。日程列表页能看到待提醒和历史提醒。

**流程**：

1. 对话结束后，在**异步步骤**（与记忆提取同一步，见 §5.1 时序图 ⑤）从这轮用户消息里抽取日程意图：有没有**显式提醒意图**、事件描述、时间表达
2. 时间表达交给**规则解析器**转成绝对时间（见下）；解析不出则返回 `need_clarify`
3. **落库由 Go 侧完成，AI 服务不碰数据库**：Python 只返回 `{content, remind_at}` 或 `{need_clarify: true}`，Go 侧调成员 3 的 `schedule_repo` 写 `schedules` 行（`status = 'pending'`）。解析失败 → 把"需要澄清"的信号带回对话，由 AI 在**下一次回复里回问用户**，不要静默丢弃
4. 定时任务（生产 5 分钟一次，`PROACTIVE_JOB_INTERVAL` 复用）扫描 `status = 'pending' AND remind_at <= NOW()`
5. 到点的行复用 `TriggerNow` 的同款逻辑：注入 `[nudge]` 消息 → 走正常聊天链路 → 生成提醒语 → `status = 'sent'`
6. 侧栏红点逻辑与主动消息完全共用

> **`POST /schedule/parse` 的调用时机**：与 `/memory/extract` 完全相同的模式——**SSE 的 `done` 发出之后**，Go 侧异步调一次，不阻塞用户看到回复。不要把它内联进流式生成过程：那会让"AI 说话的速度"被一个可以晚点再做的小功能拖慢。

**时间解析：阶段一只做规则档**

不做 LLM 解析。支持这几种句式，够覆盖答辩演示：

| 表达式 | 解析为 |
|--------|--------|
| 今天/明天/后天 + 上午/下午/晚上 + N 点（半/刻） | 对应日期时间；**没有"上午下午"时按 24 小时制** |
| X 分钟后 / X 小时后 | `NOW() + interval` |
| 下周一 / 本周五 | 下一个该星期几的日期，默认 09:00 |
| M 月 D 号 + 时间 | 当年该日期 |

解析不出（如"过阵子"、"天气好的时候"）→ **回问澄清**：「好的，但我没听准具体是哪天——你是说下周一吗？」。**这个回问本身就是演示点**，说明系统不是假装听懂了丢个假数据，而是知道自己不知道。**绝不静默丢弃**：解析失败却既不建行也不回问，是这一功能最差的表现。

**抽错比不抽更糟**：规则档宁可漏抽（用户再说一遍），也不要把"我明天要开会"这种陈述句误建成日程。抽取必须显式出现提醒意图（"提醒我"、"叫我"、"别让我忘了"），**只有时间没有提醒意图的句子不建日程**。

**阶段二（可选）**：`SCHEDULE_PARSE_BACKEND=llm`，用 `ai-service/app/schedules/llm_parser.py` 处理「下周三」这类相对表达。**做不动就保持规则档**——这是 P1 队尾的加分项，不影响验收。

> **文件名里没有 `_agent`**：日程解析器**不放在 `app/agents/` 下**，因为它不是一个 Agent（没有独立人格、不参与对话生成）。放进 `agents/` 会让人以为有第 4 个 Agent，与 §5.1 的"三 Agent"约定冲突。它是 `app/schedules/` 下的一个**解析器**，与 `classifiers/`、`memory/` 同类——可替换实现，不是 Agent。

**演示问题与解法**：提醒是时间触发的，答辩现场不可能等到明天下午三点。因此**必须有 `POST /schedules/:id/trigger` 手动触发接口**，与定时任务复用同一段逻辑——和 §5.4 的 `/proactive/trigger` 是同一个模式、同一个理由。

---

## 6. 数据库设计

### 6.1 ER 关系概览

```
users 1───N personas 1───N chat_messages
  │                          │
  │                          ├──N ai_moments 1───N moment_comments
  │                          │        └───N moment_likes（用户点赞，每人每条约一次）
  │                          ├──1 proactive_settings
  │                          ├──N schedules（日程提醒，P1；走主动消息同一条触发链路）
  │                          ├──N user_memory（记忆，一人设一份；阶段二加 ChromaDB 索引）
  │                          └──1 user_profile（JSON 画像，一人设一份）
  │
  └───N （user_id 只作为越权防线冗余在子表上）
```

> **记忆与画像都挂在 `persona_id` 上**：每个 AI 伴侣各记各的、各有一份画像。
> `user_id` 仍然保留在两张表上，但它的作用是**越权防线**（防跨用户召回），不是分区键——分区键是 `persona_id`。

> **一个人设 = 一个对话**，所以没有独立的会话表：消息直接挂在人设下。
> 会话列表就是人设列表，排序看 `personas.last_message_at`；删除人设时其全部消息级联删除。

### 6.2 核心表 DDL

```sql
-- 用户表
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(20)  NOT NULL UNIQUE,
    email         VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(100) NOT NULL,          -- bcrypt
    avatar_url    VARCHAR(255),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- AI 人设表（同时代表该人设的唯一对话）
CREATE TABLE personas (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              VARCHAR(50)  NOT NULL,
    personality_desc  TEXT         NOT NULL,       -- 性格描述
    speaking_style    VARCHAR(255) NOT NULL,       -- 说话风格
    state             JSONB        NOT NULL DEFAULT '{}',  -- 人格状态，如 {"familiarity": 0}，见 §5.6
    last_message_at   TIMESTAMPTZ,                 -- 会话列表排序 + 主动消息空闲判定
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_personas_user_last_msg ON personas(user_id, last_message_at DESC NULLS LAST);

-- 消息表（直接挂人设，无中间会话表）
CREATE TABLE chat_messages (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id     BIGINT      NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    role           VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant')),
    content        TEXT        NOT NULL,
    emotion_label  VARCHAR(20),                    -- 8 类情绪之一
    emotion_score  NUMERIC(4,3),                   -- 0.000 ~ 1.000
    is_nudge       BOOLEAN     NOT NULL DEFAULT FALSE,  -- 是否为主动消息注入的 [nudge]
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_persona_time ON chat_messages(persona_id, created_at);
CREATE INDEX idx_messages_user_id ON chat_messages(user_id);

-- 记忆表（一人设一份）
CREATE TABLE user_memory (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id        BIGINT       NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    memory_type       VARCHAR(20)  NOT NULL,       -- fact | preference | event
    content           TEXT         NOT NULL,
    embedding_id      VARCHAR(64),                 -- 对应 ChromaDB 中的向量 id
    embedding_status  VARCHAR(10)  NOT NULL DEFAULT 'pending', -- pending | synced | failed
    importance_score  NUMERIC(4,3) NOT NULL,       -- 0.000 ~ 1.000
    source_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memory_persona_type ON user_memory(persona_id, memory_type);
CREATE INDEX idx_memory_user_id ON user_memory(user_id);
CREATE INDEX idx_memory_embedding_status ON user_memory(embedding_status); -- 补偿任务扫描

-- 画像表（一人设一份，与记忆同粒度）
CREATE TABLE user_profile (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id   BIGINT      NOT NULL UNIQUE REFERENCES personas(id) ON DELETE CASCADE,
    profile_data JSONB       NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- AI 朋友圈动态表
CREATE TABLE ai_moments (
    id         BIGSERIAL PRIMARY KEY,
    persona_id BIGINT      NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    emotion_label VARCHAR(20),                 -- 内部信号：只用于决定生成语气，不返回给前端
    like_count INT         NOT NULL DEFAULT 0, -- 冗余计数，真值以 moment_likes 为准
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_moments_persona_time ON ai_moments(persona_id, created_at DESC);

-- 动态评论表（persona_id 与 user_id 二选一，表示是 AI 评论还是用户评论）
CREATE TABLE moment_comments (
    id         BIGSERIAL PRIMARY KEY,
    moment_id  BIGINT      NOT NULL REFERENCES ai_moments(id) ON DELETE CASCADE,
    persona_id BIGINT      REFERENCES personas(id) ON DELETE CASCADE,
    user_id    BIGINT      REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_comment_author CHECK (
        (persona_id IS NOT NULL AND user_id IS NULL) OR
        (persona_id IS NULL AND user_id IS NOT NULL)
    )
);
CREATE INDEX idx_comments_moment ON moment_comments(moment_id, created_at);

-- 点赞表：每个用户对每条动态只能点一次（UNIQUE 约束挡住重复点赞）
CREATE TABLE moment_likes (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    moment_id  BIGINT      NOT NULL REFERENCES ai_moments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_moment_like UNIQUE (user_id, moment_id)
);
CREATE INDEX idx_likes_moment ON moment_likes(moment_id);   -- 统计某条动态的点赞数
CREATE INDEX idx_likes_user ON moment_likes(user_id);       -- 查「我点过哪些」

-- 主动消息配置表
CREATE TABLE proactive_settings (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id   BIGINT  NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    interval_min INT     NOT NULL DEFAULT 30,      -- 分钟
    interval_max INT     NOT NULL DEFAULT 120,
    daily_limit  INT     NOT NULL DEFAULT 3,
    last_nudge_at TIMESTAMPTZ,
    UNIQUE (user_id, persona_id)
);

-- 日程提醒表（P1）
CREATE TABLE schedules (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id        BIGINT      NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    content           TEXT        NOT NULL,                    -- "提醒我开会"
    remind_at         TIMESTAMPTZ NOT NULL,
    status            VARCHAR(10) NOT NULL DEFAULT 'pending',  -- pending | sent | cancelled
    source_message_id BIGINT      REFERENCES chat_messages(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_schedules_due ON schedules(status, remind_at);   -- 定时任务扫描到期项
CREATE INDEX idx_schedules_persona ON schedules(persona_id);      -- 日程列表页按人设查
```

**`schedules` 几个字段为什么这么定**：

- `content` 存**用户原话里的事件描述**（"开会"），不存 AI 生成的话。提醒语由触发时现场生成，避免用户改了人设语气后旧提醒还用老腔调。
- `source_message_id` 用 `ON DELETE SET NULL`：删消息不该连带删日程；它只是"这条提醒从哪儿来的"的溯源，丢了不影响提醒本身。
- `status` 只三态，**不设 `failed`**：发送失败重试即可，反复失败超过阈值就转 `cancelled`，不必加第四态增加前端分支。
- 索引 `(status, remind_at)` 是**给定时任务用的**：扫描永远是 `WHERE status='pending' AND remind_at <= NOW()`，这个复合索引让扫描不必全表。

### 6.3 设计说明

- **`emotion_label` / `emotion_score` 冗余在消息表**：情绪是内部信号，供画像聚合与记忆打分使用；冗余存储可避免每次回查 AI 服务。**该字段随历史消息一并返回（供回读与聚合），但界面一律不得渲染**——不要把它理解成「不下发」。
- **`ai_moments.emotion_label` 同样是内部信号**：只用于决定动态的生成语气，供后续评论策略参考。**该列不进 `Moment` 响应体**——前端物理上拿不到，从结构上杜绝误渲染。
- **`ai_moments.like_count` 是冗余计数，真值在 `moment_likes`**：点赞走 `INSERT ... ON CONFLICT DO NOTHING` + `like_count` 自增，靠 `UNIQUE (user_id, moment_id)` 保证幂等。之所以保留冗余列，是因为朋友圈列表要按条显示点赞数，不该为每条动态做一次 `COUNT(*)`。
- **`schedules` 与记忆不是一回事，所以能删**：记忆（`user_memory`）是**只读的长期事实**，由抽取器写入、不予删除；日程是**用户自己的待办**，`DELETE /schedules/:id` 是正常操作（置 `status='cancelled'`，保留行以便回溯）。不要把两者合并成一张表——它们的生命周期与权限完全不同。
- **`users` 与 `personas` 均用 `BIGSERIAL`**：避免后期数据量大时的迁移成本，成本几乎为零。
- **外键统一 `ON DELETE CASCADE`**：删用户或删人设即级联清理其数据；`source_message_id` 用 `SET NULL`，保留记忆本身。
- **JSONB 存画像与人格状态**：字段不固定、演进频繁，用 JSONB 免去频繁改表；PostgreSQL 支持 `->>` 直接查询与 GIN 索引。
- **`embedding_id` / `embedding_status` 阶段一为空**：保留这两个字段是为了阶段二接入 ChromaDB 时**无需改表**。阶段一全部为 `NULL` / `pending`，不影响任何逻辑。
- **ChromaDB 集合设计（阶段二）**：`persona_memories` 集合，`id = user_memory.id`，`metadata = {user_id, persona_id, memory_type, importance_score}`，检索时**必须同时按 `persona_id` 与 `user_id` 过滤**再做向量相似度排序。只按 `persona_id` 过滤不够——它是全局自增，别的用户的同号人设会串号。

### 6.4 建表方式：GORM AutoMigrate

**不引入 golang-migrate / Flyway 等迁移工具**。本项目表结构在 4 周内演进频繁但部署环境单一，迁移版本管理带来的收益不足以抵消其成本。

```go
// internal/model/migrate.go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &User{},
        &Persona{},
        &ChatMessage{},
        &UserMemory{},
        &UserProfile{},
        &AIMoment{},
        &MomentComment{},
        &MomentLike{},
        &ProactiveSetting{},
        &Schedule{},
    )
}
```

**规则**：

1. 服务启动时自动执行一次 `AutoMigrate`（幂等，已存在的表不会被破坏）。
2. **改表结构 = 改 GORM struct**，重启服务即生效，不要手写 `ALTER TABLE`。
3. `AutoMigrate` **不会删除**已废弃的列，这正好是安全的默认行为。
4. `docs/` 中的 DDL 是**设计与对照用的**，实际以 GORM struct 为准。两者要保持同步更新。
5. **严禁在生产环境开 `db.AutoMigrate` 之外的破坏性操作**，如 `db.Migrator().DropTable`。

---

## 7. API 规范

### 7.1 通用约定

| 项 | 约定 |
|----|------|
| Base URL | `/api/v1` |
| 认证 | `Authorization: Bearer <access_token>` |
| 请求体 | `application/json` |
| 响应体 | 统一 `Response<T>` 结构 |
| 时间格式 | ISO 8601 / RFC3339（如 `2026-09-10T14:30:00+08:00`） |
| 分页参数 | `page`（从 1 开始）、`pageSize`（默认 20，上限 100） |
| 命名 | URL 用 kebab-case 复数名词，JSON 字段用 camelCase |

### 7.2 成功响应示例

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "accessToken": "eyJhbGciOi...",
    "refreshToken": "eyJhbGciOi...",
    "user": { "id": 1, "username": "xiaoming", "email": "xm@example.com" }
  },
  "timestamp": 1789000000000
}
```

### 7.3 失败响应示例

```json
{
  "code": 4013,
  "message": "用户名或密码错误",
  "data": null,
  "timestamp": 1789000000000
}
```

> `HTTP 401` + 业务 `code 4013`。前端以 `code` 判断具体语义，`message` 可直接展示。

### 7.4 错误码总表

| 错误码 | HTTP 状态 | 含义 |
|--------|-----------|------|
| 200 | 200 | 成功 |
| 4001 | 400 | 参数校验失败 |
| 4002 | 400 | 必填参数缺失 |
| 4003 | 400 | 该邮箱已被注册 |
| 4004 | 400 | 该用户名已被占用 |
| 4010 | 401 | 未登录或登录已过期 |
| 4011 | 401 | Token 无效 |
| 4012 | 401 | Token 已过期 |
| 4013 | 401 | 用户名或密码错误 |
| 4014 | 401 | 刷新令牌无效，请重新登录 |
| 4015 | 401 | 原密码不正确 |
| 4030 | 403 | 无权限使用该功能（功能越权） |
| ~~4031~~ | — | ~~该人设不属于当前用户~~（已废弃：资源越权按「人设不存在」返回 4043） |
| 4040 | 404 | 资源不存在 |
| 4041 | 404 | 用户不存在 |
| 4043 | 404 | 人设不存在 |
| ~~4042~~ | — | ~~会话不存在~~（已废弃：无会话实体） |
| 5000 | 500 | 服务端内部错误 |
| 5001 | 500 | AI 回复生成失败，请稍后重试 |
| 5002 | 500 | AI 服务暂时不可用 |
| 5003 | 500 | 数据库操作失败 |

> **4002 / 4030 / 4041 / 5003 是预留码**：当前端点表里没有直接引用它们（分别由 Gin 绑定校验、通用权限判断、内部查询、DB 层错误使用）。保留是为了让 `pkg/errcode` 的分段完整，不是为了凑数——评审时不要以为漏了实现。
> **4011 / 4012 / 5000 也不逐端点登记**：它们由中间件全局抛出——`4011` / `4012` 来自 `JWTAuth`，`5000` 来自 `Recovery` 与 `BizErrorHandler` 兜底。所有需要鉴权的端点都可能返回 4011/4012，所有端点都可能返回 5000，逐条抄进端点表只会制造噪音。
> **日程提醒（P1）不新增错误码**：日程不存在复用 `4040`，人设越权按「人设不存在」复用 `4043`，参数缺失/非法复用 `4001`。**不要为它单开 `4044`**——同一个语义（"资源不存在"）开两个码，正是"一 code 一 msg"要杜绝的事。
> **4015 必须同时出现在 `codeHTTPStatus` 里**：漏登记会走 `HTTPStatus()` 的兜底分支，导致改密码失败返回 HTTP 500。`errcode_test.go` 应当校验「常量集合 == `codeMessages` 键集合 == `codeHTTPStatus` 键集合」。

### 7.5 关键 API 端点

```
POST   /api/v1/auth/register        注册
POST   /api/v1/auth/login           登录
POST   /api/v1/auth/refresh         刷新 Token
GET    /api/v1/user/profile         获取当前用户信息
PUT    /api/v1/user/profile         更新用户信息（avatarUrl / username）
PUT    /api/v1/user/password        修改密码（需原密码）

GET    /api/v1/personas             人设列表（= 会话列表，按 lastMessageAt 倒序）
POST   /api/v1/personas             创建人设
PUT    /api/v1/personas/:id         编辑人设
DELETE /api/v1/personas/:id         删除人设（级联删除其全部消息）

GET    /api/v1/chat/personas/:personaId/messages   历史消息
POST   /api/v1/chat/stream                         流式对话（SSE）

GET    /api/v1/emotion/trend        情绪趋势数据（P2 · 不做，仅预留路径）

GET    /api/v1/memory                       某个人设的记忆列表（?personaId=）
GET    /api/v1/profile/portrait             某个人设的画像（?personaId=）

GET    /api/v1/moments                     朋友圈动态列表
POST   /api/v1/moments/:id/like            点赞
POST   /api/v1/moments/:id/comments        评论动态
GET    /api/v1/moments/:id/comments        评论列表

GET    /api/v1/proactive/settings          主动消息配置
PUT    /api/v1/proactive/settings          更新配置
POST   /api/v1/proactive/trigger           🚨 立即触发一次主动消息（演示/调试必备）

GET    /api/v1/schedules                   日程列表（?personaId=，**必带**）      P1
DELETE /api/v1/schedules/:id               取消/删除日程（置 cancelled）         P1
POST   /api/v1/schedules/:id/trigger       🚨 立即触发一次提醒（演示/调试必备）  P1

GET    /api/v1/health                      健康检查（部署核对用）
```

> **`/proactive/trigger` 是硬性要求**：没有它，答辩现场无法演示主动消息（定时任务最短要等 30 分钟）。它调用与定时任务**完全相同**的业务逻辑，不是"演示专用代码"。

> **朋友圈没有手动触发接口**：动态只由定时任务产生。为了让答辩现场能演示到，定时任务的执行间隔用环境变量 `MOMENT_JOB_INTERVAL` 控制，演示环境调短（如 2 分钟），并提前灌好一批历史动态作为兜底。

> **`/schedules/:id/trigger` 与 `/proactive/trigger` 同源**：两者都调 `TriggerNow` 同款逻辑，只是触发原因不同（一个是"到点了"，一个是"空闲太久"）。日程提醒**不是第 4 个 Agent**，是主动消息管线的扩展（见 §5.7）。**日程提醒是 P1**，排在 Week 2 生死线之后；时间不够时按总纲 §0.3 的砍功能顺序处理。

### 7.6 接口契约先行（准备期必须完成）

> 三人并行开发最大的死法是"前端等后端接口、后端等前端联调"。**准备期就必须把契约冻结，并用 Mock 让前端不被阻塞。**

**做法**：

1. 准备期三人一起过一遍上面的 API 列表，确认：URL、请求字段、响应字段、错误码。
2. 冻结后写进 [API_CONTRACT.md](API_CONTRACT.md)（模板已生成，可从本文第 7 节派生，逐条确认后签署）。
3. **前端在真实接口就绪前，用 Mock 开发**——三种方式任选：
   - 最简单：本地写一个 `src/api/mock/*.ts`，导出与真实 API 同签名的函数，靠 `VITE_USE_MOCK=true` 切换
   - 用 Vite 插件 `vite-plugin-mock`
   - 用 Apifox / Postman 的 Mock Server
4. **契约变更必须群里广播**，改了字段要通知另外两人，不能默默改。

**Mock 示例**：

```ts
// src/api/auth.ts — 真实实现
export const authApi = {
  login: (payload: LoginPayload) =>
    request.post<LoginResult>('/auth/login', payload),
}

// src/api/mock/auth.ts — Mock 实现，返回值结构与真实接口完全一致
export const authApiMock = {
  login: async (payload: LoginPayload): Promise<LoginResult> => ({
    accessToken: 'mock-access-token',
    refreshToken: 'mock-refresh-token',
    user: { id: 1, username: payload.username, email: 'mock@example.com' },
  }),
}
```

> 统一响应结构 `{code, message, data, timestamp}` 决定了：Mock 要包一层。建议在 `request.ts` 里做响应解包，Mock 层直接返回 `data` 部分即可，两边保持一致。

### 7.7 SSE 接口契约

**`POST /api/v1/chat/stream`**

请求：

```json
{ "personaId": 1, "content": "今天上班好累啊" }
```

响应头：

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

事件流：

```
event: delta
data: {"text":"辛苦"}

event: delta
data: {"text":"了，今天发生"}

event: delta
data: {"text":"什么了吗？"}

event: done
data: {"messageId":348}
```

错误事件：

```
event: error
data: {"code":5001,"message":"AI 回复生成失败，请稍后重试"}
```

**事件类型约定**：

| event | 时机 | data |
|-------|------|------|
| `delta` | LLM 每生成一段文本 | `{text}` |
| `done` | 全量回复落库完成 | `{messageId}` |
| `error` | 生成失败 | `{code, message}` |

> **为什么没有 `emotion` 事件**：情绪标签是**内部信号**，不向用户展示——它写入消息表、驱动回复策略、供记忆提取打分。把它推给前端属于无用数据，只会增加契约维护成本。情感分析的效果通过 **AI 回复策略的差异**体现（见 §5.2 的情绪应对策略）。

---

## 8. 目录结构

```
heart-echo/
├── backend/                            # Go 业务后端
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                 # 入口：装配依赖、启动 HTTP
│   ├── internal/
│   │   ├── handler/                    # HTTP 处理器（参数绑定 → service → 响应）
│   │   │   ├── auth_handler.go
│   │   │   ├── chat_handler.go
│   │   │   ├── memory_handler.go       # /memory 与 /profile/portrait
│   │   │   ├── persona_handler.go      # 成员 3
│   │   │   ├── moment_handler.go       # 成员 3
│   │   │   ├── proactive_handler.go    # 成员 3
│   │   │   ├── schedule_handler.go     # 成员 3（P1）
│   │   │   └── router.go               # 路由注册与分组
│   │   ├── service/                    # 业务逻辑
│   │   │   ├── auth_service.go
│   │   │   ├── chat_service.go
│   │   │   ├── memory_service.go
│   │   │   ├── persona_service.go      # 成员 3
│   │   │   ├── moment_service.go       # 成员 3
│   │   │   ├── moment_job.go           # 成员 3（朋友圈定时生成，无手动触发）
│   │   │   ├── proactive_service.go    # 成员 3（含 TriggerNow，定时任务与手动触发共用）
│   │   │   ├── proactive_job.go        # 成员 3（主动消息 + 日程到期的扫描任务）
│   │   │   ├── schedule_service.go     # 成员 3（P1：日程列表 / 取消 / 到期触发；时间解析在 Python 侧）
│   │   │   └── ai_client.go            # 调用 Python AI 服务的客户端
│   │   ├── repository/                 # 数据库访问
│   │   │   ├── user_repo.go
│   │   │   ├── message_repo.go
│   │   │   ├── memory_repo.go          # 记忆与画像查询（必带 persona_id + user_id）
│   │   │   ├── persona_repo.go         # 成员 3
│   │   │   ├── moment_repo.go          # 成员 3
│   │   │   ├── proactive_repo.go       # 成员 3
│   │   │   └── schedule_repo.go        # 成员 3（P1）
│   │   ├── model/                      # GORM 实体（成员 3）
│   │   │   ├── migrate.go              # AutoMigrate 入口
│   │   │   ├── user.go
│   │   │   ├── persona.go
│   │   │   ├── message.go
│   │   │   ├── memory.go
│   │   │   ├── moment.go
│   │   │   ├── proactive.go
│   │   │   └── schedule.go
│   │   ├── middleware/                 # 中间件
│   │   │   ├── recovery.go
│   │   │   ├── biz_error.go
│   │   │   ├── jwt.go
│   │   │   ├── cors.go
│   │   │   └── logger.go
│   │   ├── dto/                        # 请求/响应数据传输对象
│   │   │   ├── auth_dto.go
│   │   │   ├── chat_dto.go
│   │   │   ├── persona_dto.go          # 成员 3
│   │   │   ├── moment_dto.go           # 成员 3
│   │   │   ├── proactive_dto.go        # 成员 3
│   │   │   └── schedule_dto.go         # 成员 3（P1）
│   │   └── config/                     # 配置加载
│   │       └── config.go
│   ├── pkg/                            # 可被外部复用的通用包
│   │   ├── response/                   # 统一响应封装
│   │   │   └── response.go
│   │   ├── errcode/                    # 错误码定义（一 code 一 msg）
│   │   │   ├── errcode.go
│   │   │   └── errcode_test.go
│   │   ├── jwt/                        # JWT 生成与解析
│   │   │   └── jwt.go
│   │   └── logger/                     # zap 日志封装
│   │       └── logger.go
│   ├── .env.example
│   ├── Dockerfile                      # 多阶段：golang 构建 → alpine 运行
│   ├── go.mod
│   └── go.sum
│
├── ai-service/                         # Python AI 服务（薄层）
│   ├── app/
│   │   ├── agents/                     # 三个 Agent
│   │   │   ├── emotion_agent.py        # 情感分析 Agent（薄封装，实际分类在 classifiers/）
│   │   │   ├── dialogue_agent.py       # 对话生成 Agent
│   │   │   └── memory_agent.py         # 记忆管理 Agent（含规则提取）
│   │   ├── schedules/                  # 日程时间解析（P1）——注意：不是 Agent，是解析器
│   │   │   ├── rule_parser.py          # 阶段一：正则 + 句式模板
│   │   │   ├── llm_parser.py           # 阶段二：LLM 结构化解析
│   │   │   └── __init__.py             # 工厂：靠 SCHEDULE_PARSE_BACKEND 切换
│   │   ├── classifiers/                # 情感分类器（可替换实现）
│   │   │   ├── base.py                 # EmotionClassifier 接口
│   │   │   ├── lexicon.py              # 阶段一：关键词词典
│   │   │   ├── onnx.py                 # 阶段二：ONNX 小模型
│   │   │   └── __init__.py             # 工厂：靠 EMOTION_BACKEND 切换
│   │   ├── memory/                     # 记忆检索器（可替换实现）
│   │   │   ├── base.py                 # MemoryRetriever 接口
│   │   │   ├── recent.py               # 阶段一：PostgreSQL 最近 N 条 + 关键词
│   │   │   ├── vector.py               # 阶段二：ChromaDB
│   │   │   └── __init__.py             # 工厂：靠 MEMORY_BACKEND 切换
│   │   ├── core/
│   │   │   ├── config.py               # 含实现分级开关
│   │   │   └── llm_client.py           # DeepSeek 客户端
│   │   ├── schemas/                    # Pydantic 模型
│   │   │   └── chat.py
│   │   ├── api/                        # 路由
│   │   │   └── routes.py
│   │   └── main.py
│   ├── tests/
│   ├── .env.example
│   ├── Dockerfile
│   └── requirements.txt
│
├── frontend/                           # Vue 前端
│   ├── src/
│   │   ├── api/                        # API 封装
│   │   │   ├── request.ts              # Axios 实例 + 拦截器
│   │   │   ├── auth.ts
│   │   │   ├── chat.ts                 # 含 SSE 流式封装
│   │   │   ├── memory.ts               # 成员 2（画像页）
│   │   │   ├── profile.ts              # 成员 2（画像页 / 资料编辑）
│   │   │   ├── persona.ts              # 成员 3
│   │   │   ├── moment.ts               # 成员 3
│   │   │   ├── proactive.ts            # 成员 3
│   │   │   ├── schedule.ts             # 成员 3（P1）
│   │   │   └── mock/                   # 后端未就绪时的 Mock（VITE_USE_MOCK 切换）
│   │   ├── components/                 # 通用组件
│   │   │   ├── chat/
│   │   │   │   ├── MessageBubble.vue
│   │   │   │   ├── TypingIndicator.vue
│   │   │   │   └── ChatInput.vue
│   │   │   ├── persona/                # 成员 3（人设表单等）
│   │   │   └── common/
│   │   │       └── EmptyState.vue
│   │   ├── layouts/
│   │   │   └── MainLayout.vue
│   │   ├── router/                     # 路由 + 守卫
│   │   │   ├── index.ts
│   │   │   └── guards.ts
│   │   ├── stores/                     # Pinia 状态
│   │   │   ├── auth.ts                 # 登录态 + 持久化
│   │   │   ├── chat.ts
│   │   │   └── persona.ts              # 成员 3
│   │   ├── types/                      # TS 类型定义
│   │   │   ├── api.ts                  # 成员 2（交接物 #8）
│   │   │   ├── user.ts
│   │   │   ├── chat.ts
│   │   │   ├── persona.ts              # 成员 3
│   │   │   ├── moment.ts               # 成员 3（含 Comment 类型）
│   │   │   ├── memory.ts               # 成员 2
│   │   │   ├── profile.ts              # 成员 2（Portrait）
│   │   │   ├── proactive.ts            # 成员 3（Settings）
│   │   │   ├── schedule.ts             # 成员 3（P1）
│   │   │   ├── errcode.ts
│   │   │   └── router.d.ts
│   │   ├── utils/
│   │   │   ├── markdown.ts
│   │   │   └── time.ts
│   │   ├── views/                      # 页面
│   │   │   ├── auth/{LoginView.vue,RegisterView.vue}
│   │   │   ├── chat/ChatView.vue
│   │   │   ├── persona/PersonaView.vue     # 成员 3
│   │   │   ├── profile/{ProfileView.vue,PasswordView.vue}   # 成员 2（含修改密码）
│   │   │   ├── moments/MomentsView.vue     # 成员 3
│   │   │   ├── schedule/ScheduleView.vue   # 成员 3（P1：日程列表）
│   │   │   └── error/NotFoundView.vue
│   │   ├── App.vue
│   │   └── main.ts
│   ├── .env.example
│   ├── index.html
│   ├── package.json
│   ├── package-lock.json               # 必须提交：deploy/Dockerfile 用 npm ci
│   ├── tsconfig.json
│   └── vite.config.ts
│
├── deploy/
│   ├── Dockerfile                      # 构建前端 dist + 打包 Nginx（构建上下文 = 仓库根）
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yml
│   ├── nginx.conf
│   └── .env.example                    # 生产环境变量模板（.env 权限 600，永不入库）
├── docs/
│   ├── TECH_DESIGN.md                  # 本文档
│   ├── COLLABORATION.md
│   ├── API_CONTRACT.md                 # 准备期冻结的接口契约（见 §7.6）
│   ├── KICKOFF.md                      # 立项讨论稿（一页讲清做什么/难不难）
│   └── dev/                            # 个人开发文档
│       ├── MASTER.md
│       ├── MEMBER_1_BACKEND_AI.md
│       ├── MEMBER_2_FRONTEND.md
│       └── MEMBER_3_DATA_MOMENTS_DEPLOY.md
├── .dockerignore
├── .gitignore
├── .gitmessage                         # commit 模板（git config commit.template .gitmessage）
└── README.md
```

---

## 9. 部署方案

### 9.1 云服务器配置

| 项 | 配置 |
|----|------|
| 厂商 | 腾讯云 / 阿里云学生机（国内） |
| 规格 | 2 核 4G，约 15-20 元/月 |
| 系统 | Ubuntu 22.04 LTS |
| 访问方式 | **`http://公网IP` 直访，不买域名、不做 HTTPS、不备案** |
| 安全组开放端口 | 22（SSH）、80（HTTP）——**不开放 443** |

> 演示地址形如 `http://123.45.67.89`。硬性要求只要求"公网可访问"，IP 直访完全满足。
> 国内服务器绑域名必须走 ICP 备案（2-3 周），时间成本与收益不匹配。

> ⚠️ **安全提醒**：数据库、ChromaDB、AI 服务的端口**一律不对外开放**，只在 Docker 内网互通。生产环境禁止使用默认密码，禁止把 `.env` 提交到仓库。

### 9.2 部署架构

```
云服务器（公网 IP）
└── Docker Compose
    ├── nginx 容器          :80 → 宿主机 80（唯一对外暴露的端口）
    │                       └── 镜像内已含前端构建产物（多阶段构建）
    ├── backend 容器        :8080（仅内网）
    ├── ai-service 容器     :8000（仅内网）
    └── postgres 容器       :5432（仅内网）

Nginx 同时承担两件事：托管前端静态文件 + 把 /api 反向代理到 backend。
前端不用独立容器——由 deploy/Dockerfile 的 node 构建阶段产出 dist，
直接 COPY 进 nginx 镜像，**避免跨容器共享文件**。

（chromadb 容器在阶段二接入向量检索时启用）
```

### 9.3 docker-compose.yml

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks: [heart_echo_net]

  # ===== 阶段二：接入向量检索时启用 =====
  # chromadb:
  #   image: chromadb/chroma:latest
  #   restart: always
  #   volumes:
  #     - chroma_data:/chroma/chroma
  #   networks: [heart_echo_net]

  # ===== 明确不做：Redis =====
  # 本项目 QPS 极低，没有缓存需求。加它只会多一个要运维的容器。

  ai-service:
    build: ../ai-service
    restart: always
    env_file: .env
    networks: [heart_echo_net]

  backend:
    build: ../backend
    restart: always
    env_file: .env
    environment:
      DB_HOST: postgres
      AI_SERVICE_URL: http://ai-service:8000
    depends_on:
      postgres:
        condition: service_healthy
      ai-service:
        condition: service_started
    networks: [heart_echo_net]

  # 前端没有独立容器：由 deploy/Dockerfile 的 node 阶段构建后 COPY 进 nginx 镜像
  nginx:
    build:
      context: ..                 # 构建上下文是仓库根目录（需访问 frontend/ 与 deploy/）
      dockerfile: deploy/Dockerfile
    restart: always
    ports:
      - "80:80"
      # - "443:443"
    depends_on:
      - backend
    networks: [heart_echo_net]

volumes:
  pg_data:
  # chroma_data:   # 阶段二接入 ChromaDB 时启用

networks:
  heart_echo_net:
    driver: bridge
```

### 9.3.1 Dockerfile（三个镜像）

compose 的 `build:` 依赖这三个文件，**准备期就要建好，不要留到 Week 4**。

**`deploy/Dockerfile`**（构建上下文为仓库根目录，同一镜像产出前端静态文件 + Nginx 配置）

```dockerfile
# ===== stage 1: 构建前端 =====
FROM node:20-alpine AS build
WORKDIR /app
# npm ci 需要 package-lock.json —— 必须提交进仓库，否则部署时构建失败
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
# ⚠️ Vite 的环境变量在「构建时」注入，运行期改 .env 无效。
# 生产前后端同源，API 走 Nginx 的 /api 转发，所以是相对路径 /api/v1。
ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

# ===== stage 2: Nginx 托管静态资源 + 反向代理 =====
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

**`backend/Dockerfile`**

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 静态编译，产出不依赖 libc 的单一二进制
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server .
EXPOSE 8080
CMD ["./server"]
```

**`ai-service/Dockerfile`**

```dockerfile
FROM python:3.11-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

> **后端必须监听 `0.0.0.0` 而不是 `127.0.0.1`**，否则容器外（包括同网络的 nginx）连不上。这是容器化最常见的坑：本地跑得好好的，进容器就 connection refused。

**根目录另需 `.dockerignore`**，避免把 `node_modules/`、`pg_data/`、`.env` 打进构建上下文（既慢又可能泄漏密钥）：

```
**/node_modules
**/dist
**/__pycache__
**/.venv
**/.env
pg_data
chroma_data
.git
```

### 9.4 nginx.conf

```nginx
upstream backend_upstream {
    server backend:8080;
}

server {
    listen 80;
    server_name _;

    # 前端静态资源
    location / {
        root /usr/share/nginx/html;
        index index.html;
        try_files $uri $uri/ /index.html;   # 支持 Vue Router history 模式
    }

    # API 反向代理
    location /api/ {
        proxy_pass http://backend_upstream;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 关键配置：关闭缓冲，延长超时，否则不是流式
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
        chunked_transfer_encoding on;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        root /usr/share/nginx/html;
        expires 7d;
        add_header Cache-Control "public, immutable";
    }

    gzip on;
    gzip_types text/plain text/css application/json application/javascript application/xml;
    gzip_min_length 1k;
}
```

### 9.5 部署流程（GitHub Actions 可选自动化）

**手动部署（推荐前期使用）**：

```bash
# 本地提交并推送
git push origin develop

# SSH 登录服务器
ssh ubuntu@<公网IP>
cd /opt/heart-echo
git pull origin develop
docker compose -f deploy/docker-compose.yml up -d --build
docker compose -f deploy/docker-compose.yml ps     # 确认全部 healthy
```

**访问地址**：`http://<公网IP>`——本期不做域名与 HTTPS（见 [§9.1](#91-云服务器配置)）。

**首次部署检查清单**：

- [ ] `.env` 已在服务器创建且未提交仓库
- [ ] `deploy/Dockerfile`、`backend/Dockerfile`、`ai-service/Dockerfile` 三个文件已提交
- [ ] `frontend/package-lock.json` 已提交（`npm ci` 依赖它，缺了构建直接失败）
- [ ] 根目录 `.dockerignore` 存在，且确认 `.env` 不会被打进镜像
- [ ] 数据库迁移已执行（GORM AutoMigrate）
- [ ] 后端 `/api/v1/health` 健康检查可访问
- [ ] AI 服务可连通 DeepSeek（容器内 curl 测试）
- [ ] 前端 `VITE_API_BASE_URL` 在生产构建时注入为 `/api/v1`（同源，走 Nginx 转发；**不是** `localhost:8080`）
- [ ] 安全组仅开放 22/80（本期不开 443）
- [ ] 用手机 4G 打开 `http://<公网IP>` 走通核心演示链路

---

## 10. 安全、降级与边界处理

### 10.1 安全要点

| 风险 | 措施 |
|------|------|
| 密码泄漏 | bcrypt 加盐哈希（cost ≥ 10），严禁明文/MD5 |
| 越权访问 | 所有涉及 `persona_id` 的查询必须带 `user_id` 条件，禁止只按 id 查询 |
| 用户数据串号 | 记忆/画像查询必须同时带 `persona_id`（隔离人设）与 `user_id`（越权防线）；阶段二接入 ChromaDB 后，向量检索同样两个条件都要带 |
| JWT 密钥泄漏 | 密钥从环境变量注入，长度 ≥ 32 字符，各环境不同 |
| XSS | Markdown 渲染前用 DOMPurify 清洗；避免 `v-html` 直接渲染后端内容 |
| SQL 注入 | 统一使用 GORM 参数化查询，禁止字符串拼接 SQL |
| 敏感信息泄漏 | panic 堆栈、原始 error 只进日志，不返回前端 |
| 用户名枚举 | 登录失败统一返回 `4013 用户名或密码错误`，不区分"用户不存在" |
| 接口刷量 | 登录/注册接口做 IP 限流（如 `golang.org/x/time/rate` 或 Nginx `limit_req`） |
| **LLM 成本被刷爆** | 公网暴露后任何人可调用 `/chat/stream` 消耗 DeepSeek 余额。两类措施：① 登录用户级限流（如每人每分钟 ≤5 条、每日上限）；② Nginx 对 `/api/v1/chat/stream` 加 `limit_req`。**同时给 DeepSeek 账号设置消费上限/告警**——这是最后一道防线 |
| 密钥入库 | `.env` 加入 `.gitignore`，仓库只留 `.env.example`；`.dockerignore` 同样排除 `.env` |

### 10.2 降级与容错

| 故障 | 降级策略 |
|------|----------|
| DeepSeek API 超时/失败 | 重试 2 次（指数退避），仍失败则返回 `5001`，前端展示重试按钮 |
| DeepSeek 完全不可用 | 返回兜底话术（如"我现在有点卡壳，稍后再聊好吗？"），保证页面不白屏 |
| 情感分析失败 | 该条消息 `emotion_label` 置 `null`，对话主链路继续，不阻塞 |
| 记忆提取失败 | 仅记日志，不影响本轮对话结果（异步任务失败可静默重试） |
| SSE 中断 | 前端保留已生成内容并显示"回复中断"，提供重试 |
| AI 服务整体宕机 | Go 后端健康检查失败，接口返回 `5002`，前端提示"AI 服务暂时不可用" |
| **阶段二** ChromaDB 不可用 | 记忆召回自动降级为 PostgreSQL「最近 N 条」，对话继续 |

> **注意**：阶段一架构里没有 ChromaDB、没有 Redis，因此**少了两类故障源**——这是分阶段交付的附带收益，故障面更小，排查路径更短。

### 10.3 性能与体验边界

- **上下文窗口控制**：注入 Prompt 的历史消息默认取最近 20 条，记忆取 Top-5，防止 Token 超限与成本失控。
- **LLM 调用超时**：单次请求超时 60s，SSE 首字节超时 15s。
- **消息分页**：历史消息按 `created_at` 倒序分页加载，前端滚动到底部再拉取上一页。
- **成本控制**：DeepSeek 计费按 Token，开发期用较小 max_tokens（如 512）；记忆提取仅在对话轮次 ≥ 3 且消息长度 ≥ 20 字时触发。

### 10.4 可观测性

- **结构化日志**：使用 `zap`，每条请求日志包含 `traceId`（中间件生成 UUID）、`path`、`method`、`status`、`latency`、`userId`。
- **健康检查**：`GET /api/v1/health`（后端）、`GET /health`（AI 服务），返回各依赖连通状态，供 Docker healthcheck 与部署核对使用。
- **错误日志分级**：4xxx 记 `warn`，5xxx 记 `error`（带原始错误），便于快速定位。
