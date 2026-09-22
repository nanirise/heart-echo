/**
 * 对话（Chat）模块的数据契约 —— 与 API_CONTRACT §5 / §6 一一对应。
 *
 * 本文件只定义"东西长什么样"，不含任何逻辑。
 */

/** 一条对话消息（契约 §5 ChatMessage） */
export interface ChatMessage{
    id: number
    personaId: number
    role: 'user' | 'assistant'
    content: string
    isNudge: boolean
    createdAt: string
}

/** SSE 事件名（契约 §6，冻结不可增删） */
export type ChatStreamEventName = 'delta' | 'done' | 'error'

/** delta：一段增量文本 */
export interface DeltaPayload {
  text: string
}

/** done：全量回复落库完成 */
export interface DonePayload {
  messageId: number
}

/** error：生成失败。注意此时 HTTP 状态仍是 200 */
export interface ErrorPayload {
  code: number
  message: string
}

/** POST /chat/stream 请求体（契约 §6） */
export interface StreamChatPayload {
  personaId: number
  content: string
}

/** 流式回调集合，签名对齐 MEMBER_2_FRONTEND §3 */
export interface StreamChatHandlers {
  onDelta: (text: string) => void
  onDone: (payload: DonePayload) => void
  onError: (payload: ErrorPayload) => void
}
