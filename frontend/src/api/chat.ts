import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/api/request'
import { ErrorCode } from '@/types/errcode'
import type {
  StreamChatPayload,
  StreamChatHandlers,
  DeltaPayload,
  DonePayload,
  ErrorPayload,
} from '@/types/chat'

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

  if (response.ok === false) {
    throw new ApiError(ErrorCode.ErrInternal, `对话请求失败（HTTP ${response.status}）`)
  }

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
