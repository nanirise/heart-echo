/**
 * 人设（Persona）—— 与契约 §4 的响应结构逐字对齐，字段名 camelCase。
 *
 * 本文件是 `GET /personas` 响应类型的唯一一份定义，其他页面（聊天页侧栏等）
 * 一律从这里 import，不要各自再声明一遍。
 */

/** 人设实体，对应后端 `dto.PersonaResponse` */
export interface Persona {
  id: number
  name: string
  personalityDesc: string
  speakingStyle: string
  /** 人格状态原始 JSONB。契约要求前端不解析、不回传，这里只占位，代码里不读它 */
  state: Record<string, unknown>
  /** 亲密度。只读 —— 累加在对话链路，本模块不写 */
  familiarity: number
  /** 该对话最后一条消息时间；从未聊过为 null。列表排序由它决定 */
  lastMessageAt: string | null
  createdAt: string
}

/** 新建 / 编辑的请求体。三个字段全必填；PUT 是整体替换，不是部分更新 */
export interface PersonaPayload {
  name: string
  personalityDesc: string
  speakingStyle: string
}
