import { useAuthStore } from '@/stores/auth'
import { ApiError, request } from '@/api/request'
import { ErrorCode } from '@/types/errcode'
import type {
  ChatMessage,
  StreamChatPayload,
  StreamChatHandlers,
  DeltaPayload,
  DonePayload,
  ErrorPayload,
} from '@/types/chat'

import type { PageResult } from '@/types/api'

const baseURL = import.meta.env.VITE_API_BASE_URL

export async function streamChat(
  payload: StreamChatPayload,
  handlers: StreamChatHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const auth = useAuthStore()

  if (auth.accessToken === null) {
    throw new ApiError(ErrorCode.ErrUnauthorized, '未登录，无法发送消息')
  }

  const response = await fetch(`${baseURL}/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${auth.accessToken}`,
    },
    body: JSON.stringify(payload),
    signal,
  })

  // 契约 §6：error 事件时 HTTP 仍为 200，这里只能发现「请求没建立成功」这一类问题
  if (response.ok === false) {
    throw new ApiError(ErrorCode.ErrInternal, `对话请求失败（HTTP ${response.status}）`)
  }

  await consumeStream(response, handlers)
}

/**
 * 消费一个 SSE 响应体，按契约 §6 解析事件并派发。
 *
 * 抽成独立函数是为了让 Mock 复用同一段解析：Mock 造一个真的 ReadableStream 交给它，
 * 半行切分与 { stream: true } 就会被真正跑到。若 Mock 直接回调 onDelta，
 * 解析代码一行都不会执行，等接上真后端才暴露问题 —— 那 Mock 就白做了。
 */
export async function consumeStream(
  response: Response,
  handlers: StreamChatHandlers,
): Promise<void> {
  if (response.body === null) {
    throw new ApiError(ErrorCode.ErrInternal, '对话流为空，浏览器不支持流式读取')
  }

  const reader = response.body.getReader()
  // TextDecoder 有状态，必须跨 chunk 复用，否则 { stream: true } 失效
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()

      if (done) {
        break
      }

      buffer += decoder.decode(value, { stream: true })

      const chunks = buffer.split('\n\n')
      // 最后一块可能是被切断的半行，留回 buffer 等下一个 chunk
      buffer = chunks.pop() ?? ''

      for (const chunk of chunks) {
        const eventName = pickField(chunk, 'event')
        const dataText = pickField(chunk, 'data')

        if (eventName === null || dataText === null) {
          continue
        }

        emitEvent(eventName, dataText, handlers)
      }
    }
  } finally {
    reader.releaseLock()
  }
}

/**
 * 后端倒序返回（最新在前），界面需要「旧 → 新」。
 * 抽出来同样是为了让 Mock 走到这一步 —— spec §5.1 有一条验收专验它。
 */
export function toChronological(list: ChatMessage[]): ChatMessage[] {
  return list.slice().reverse()
}

export async function getMessages(
  personaId: number,
  page = 1,
  pageSize = 20,
): Promise<ChatMessage[]> {
  const result = await request.get<PageResult<ChatMessage>>(
    `/chat/personas/${personaId}/messages`,
    { params: { page, pageSize } },
  )

  return toChronological(result.list)
}

function pickField(chunk: string, field: string): string | null {
  const prefix = `${field}:`

  for (const line of chunk.split('\n')) {
    if (line.startsWith(prefix)) {
      return line.slice(prefix.length).trim()
    }
  }

  return null
}

function emitEvent(
  eventName: string,
  dataText: string,
  handlers: StreamChatHandlers,
): void {
  if (eventName === 'delta') {
    const payload = JSON.parse(dataText) as DeltaPayload
    handlers.onDelta(payload.text)
    return
  }

  if (eventName === 'done') {
    const payload = JSON.parse(dataText) as DonePayload
    handlers.onDone(payload)
    return
  }

  if (eventName === 'error') {
    const payload = JSON.parse(dataText) as ErrorPayload
    handlers.onError(payload)
  }
}
