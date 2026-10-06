import { consumeStream, toChronological } from '@/api/chat'
import type { ChatMessage, StreamChatHandlers, StreamChatPayload } from '@/types/chat'

/**
 * 故意按固定长度切字节，而不是按事件块切。
 * 17 是质数，能保证切点落在「事件块内部」和「汉字的三字节中间」——
 * 这两处正是 consumeStream 最容易错的地方，必须让它们在 Mock 下真的被跑到。
 * 改成 1 可以更暴力地验证（每个字节单独成块）。
 */
const CHUNK_SIZE = 17

/** 分片之间的延迟，模拟 delta 逐条到达；太小看不出打字机效果 */
const CHUNK_DELAY_MS = 40

/** 置 true 会在流中途注入一个 error 事件，用于验证「error 必须落到 UI」 */
const INJECT_ERROR = false

export async function streamChat(
  payload: StreamChatPayload,
  handlers: StreamChatHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const bytes = new TextEncoder().encode(buildSseText(payload.content))

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      for (let offset = 0; offset < bytes.length; offset += CHUNK_SIZE) {
        // 真实实现靠 fetch 的 signal 中断；Mock 没有 fetch，只能自己检查
        if (signal?.aborted === true) {
          controller.error(new DOMException('请求已中断', 'AbortError'))
          return
        }

        controller.enqueue(bytes.slice(offset, offset + CHUNK_SIZE))
        await sleep(CHUNK_DELAY_MS)
      }

      controller.close()
    },
  })

  const response = new Response(stream, {
    headers: { 'Content-Type': 'text/event-stream' },
  })

  await consumeStream(response, handlers)
}

export async function getMessages(
  personaId: number,
  page = 1,
  pageSize = 20,
): Promise<ChatMessage[]> {
  const start = (page - 1) * pageSize
  const list = mockBackendMessages(personaId).slice(start, start + pageSize)

  return toChronological(list)
}

/** 拼出与契约 §6 完全一致的 SSE 文本：三事件、块间空行 */
function buildSseText(userContent: string): string {
  const reply = `收到「${userContent}」啦，这是 Mock 回复，用来验证逐字输出。`
  const blocks: string[] = []

  for (const char of reply) {
    blocks.push(`event: delta\ndata: ${JSON.stringify({ text: char })}\n\n`)
  }

  if (INJECT_ERROR) {
    blocks.push(
      `event: error\ndata: ${JSON.stringify({ code: 5000, message: 'Mock 注入的生成失败' })}\n\n`,
    )
    return blocks.join('')
  }

  blocks.push(`event: done\ndata: ${JSON.stringify({ messageId: Date.now() })}\n\n`)

  return blocks.join('')
}

/**
 * 模拟后端：倒序返回（最新在前），交给 toChronological 翻成界面顺序。
 * personaId 为 1 时返回空数组，用于验证空对话态（契约 §5：200 + []，不是 4043）。
 * 中间那条 isNudge 的行不应进入消息列表（store 层过滤），留在此处当过滤用例。
 */
function mockBackendMessages(personaId: number): ChatMessage[] {
  if (personaId === 1) {
    return []
  }

  return [
    {
      id: personaId * 10 + 3,
      personaId,
      role: 'assistant',
      content: '（Mock）最新的一条，应该显示在最下面。',
      isNudge: false,
      createdAt: '2026-09-27T10:03:00Z',
    },
    {
      id: personaId * 10 + 2,
      personaId,
      role: 'user',
      content: '[nudge] （Mock）系统注入的主动消息：role 是 user，但不是用户打的字。',
      isNudge: true,
      createdAt: '2026-09-27T10:02:00Z',
    },
    {
      id: personaId * 10 + 1,
      personaId,
      role: 'assistant',
      content: '（Mock）最早的一条，应该显示在最上面。',
      isNudge: false,
      createdAt: '2026-09-27T10:01:00Z',
    },
  ]
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
}
