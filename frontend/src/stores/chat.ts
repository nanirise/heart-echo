import { toErrorMessage } from '@/api/error'
import { defineStore } from 'pinia'
import { chatApi, listPersonas } from '@/api/mock'
import { useAuthStore } from '@/stores/auth'
import type { ChatMessage } from '@/types/chat'
import type { Persona } from '@/types/persona'

/**
 * 进行中那条流的中断器 —— 放模块级，不进 state。
 * AbortController 是宿主对象，被 pinia 的 reactive 包一层之后调 abort() 可能抛
 * `Illegal invocation`，而且不是每次都抛。口诀：模板要渲染的进 state，
 * 生命周期内幕（controller / timer / promise）放模块级。
 */
let activeController: AbortController | null = null

/** 上一次真正发出去的内容，供「重试」原样重发；切人设时清空，避免把话发给别人 */
let lastSentContent = ''

/** 本地临时消息的 id 计数器，只减不增 —— 生成的 id 全是负数，不会和真实 id 撞 */
let tempSeq = 0

/**
 * 时间串统一解析成毫秒再比较。
 * 列表与消息两端的时间格式 / 时区未必逐字一致，字符串比较会错位；
 * 解析失败得到 NaN，任何比较都为 false（红点不亮）。
 */
function toTime(timestamp: string): number {
  return new Date(timestamp).getTime()
}

/**
 * 中断与真失败必须分开处理：前者是用户自己的选择（切人设、主动停止），不该弹错误。
 * 用 `instanceof Error` 而不是 `instanceof DOMException`：真实通道和 Mock 通道抛的都是
 * 带 name='AbortError' 的 Error 子类，这条判据两边都接得住。
 */
function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

/**
 * 造一条本地临时消息。
 * id 用负数：真实 id 由数据库自增、必定为正，于是「id 为负 = 还没落库」这条判据
 * 不需要额外字段，也不用改契约里的 ChatMessage。
 */
function buildLocalMessage(
  personaId: number,
  role: ChatMessage['role'],
  content: string,
): ChatMessage {
  tempSeq -= 1

  return {
    id: tempSeq,
    personaId,
    role,
    content,
    isNudge: false,
    createdAt: new Date().toISOString(),
  }
}

export const useChatStore = defineStore('chat', {
  state: () => ({
    /** 侧栏的人设列表；顺序由后端定（按最后发言时间倒序），前端不再排一遍 */
    personas: [] as Persona[],
    /** 正在看的对话对象；null 表示还没选中任何人 */
    currentPersonaId: null as number | null,
    /** 当前对话的消息，永远按「旧 → 新」排列 */
    messages: [] as ChatMessage[],
    /** 是否有流正在进行；为 true 时输入框应禁用 */
    isStreaming: false,
    /** 一句可展示的错误文案，空串表示当前没有错误 */
    errorMessage: '',
    /**
     * 已读位：personaId → 该对话「最后一条已读消息」的服务端时间戳。
     * 红点判据 = 列表里的 lastMessageAt 晚于它（服务端没有未读状态，plan 步 6 口径）。
     * 只记录服务端给过的时间（lastMessageAt / createdAt），不掺本地时钟 ——
     * 本机与服务端时钟有偏差时，本地时间会让红点永远亮着或永远不亮。
     */
    readAt: {} as Record<number, string>,
    /** 本地 readAt 归属的账号 id；换账号后 readAt 清空，避免跨账号串已读位 */
    uid: null as number | null,
  }),

  getters: {
    /** 当前人设对象；列表还没加载或 id 已失效时为 null */
    currentPersona(state): Persona | null {
      return state.personas.find((item) => item.id === state.currentPersonaId) ?? null
    },

    /** 有未读的人设 id 集合，供侧栏红点。判据 = lastMessageAt 晚于本地已读位 */
    unreadPersonaIds(state): Set<number> {
      const ids = new Set<number>()

      for (const persona of state.personas) {
        const last = persona.lastMessageAt

        // 从未聊过（null）没有未读可言
        if (last === null) {
          continue
        }

        const read = state.readAt[persona.id]

        if (read === undefined || toTime(last) > toTime(read)) {
          ids.add(persona.id)
        }
      }

      return ids
    },
  },

  actions: {
    // ── 纯数据操作 ──

    clearError(): void {
      this.errorMessage = ''
    },

    /**
     * 记录「看到这里为止」。只接受服务端给过的时间戳（lastMessageAt / createdAt）——
     * 掺本地时钟会在两端时钟有偏差时把红点点亮或永远点不亮。
     * 已读位只前移：滞后到达的旧时间戳不能把已读退回未读，否则红点会闪回。
     */
    markRead(personaId: number, seenAt: string): void {
      const current = this.readAt[personaId]

      if (current !== undefined && toTime(seenAt) <= toTime(current)) {
        return
      }

      this.readAt[personaId] = seenAt
    },

    // ── 数据加载 ──

    /**
     * 拉侧栏的人设列表。
     * 这里**不**顺手选中第一个：「当前在看谁」由路由参数唯一决定，
     * store 不该知道路由的存在，落点交给 ChatView 处理。
     */
    async loadPersonas(): Promise<void> {
      const auth = useAuthStore()

      // 换账号后清空已读位。不做在登出处是为了避免 auth ↔ chat 循环依赖
      if (this.uid !== (auth.user?.id ?? null)) {
        this.readAt = {}
        this.uid = auth.user?.id ?? null
      }

      try {
        const result = await listPersonas()
        this.personas = result.list
      } catch (error) {
        this.errorMessage = toErrorMessage(error, '网络异常，请检查网络后重试')
      }
    },

    /**
     * 拉某个人设的历史消息。
     * await 期间用户可能已经切到别人，回来必须比对「发起时的 id」与「现在的 id」，
     * 不一致就整份丢掉 —— 否则会出现「B 的标题配 A 的消息」。
     * 契约 §5 后端是倒序返回，反转已经在 api/chat.ts 里做完，这里拿到的就是正序。
     */
    async loadMessages(personaId: number): Promise<void> {
      try {
        const list = await chatApi.getMessages(personaId)

        if (personaId !== this.currentPersonaId) {
          return
        }

        // 注入行不进消息列表；readAt 仍需按原始 list 校准
        this.messages = list.filter((item) => item.isNudge === false)

        // 消息已渲染到屏幕上 = 读到最新一条；用服务端 createdAt 校准已读位
        const newest = list[list.length - 1]

        if (newest !== undefined) {
          this.markRead(personaId, newest.createdAt)
        }
      } catch (error) {
        if (personaId !== this.currentPersonaId) {
          return
        }

        this.errorMessage = toErrorMessage(error, '网络异常，请检查网络后重试')
      }
    },

    // ── 切换与中断 ──

    /**
     * 切换对话对象。
     * 第一句必须是 stopStreaming()：不中断旧流的话，旧流的回调返回时已经找不到
     * 自己那条占位消息，isStreaming 会永远停在 true，界面看起来像卡死。
     */
    selectPersona(personaId: number): void {
      if (personaId === this.currentPersonaId) {
        // 重新进入对话页：重载消息，让 readAt 跟上 lastMessageAt（红点消除）
        void this.loadMessages(personaId)
        return
      }

      this.stopStreaming()

      this.currentPersonaId = personaId
      this.messages = []
      this.errorMessage = ''
      lastSentContent = ''

      // 打开即清红点（列表值可能滞后，loadMessages 拿到真实消息后会再校准一次）
      const persona = this.personas.find((item) => item.id === personaId)

      if (persona !== undefined && persona.lastMessageAt !== null) {
        this.markRead(personaId, persona.lastMessageAt)
      }

      void this.loadMessages(personaId)
    },

    /** 中断进行中的流；没有流在跑时是空操作 */
    stopStreaming(): void {
      if (activeController !== null) {
        activeController.abort()
        activeController = null
      }

      this.isStreaming = false
    },

    // ── 发送与重试 ──

    /**
     * 发送一条消息并消费流式回复。
     * 先把两条「本地临时消息」（用户消息 + 空的 AI 占位）推进列表把界面点亮，
     * 之后每个 delta 都追加进占位里 —— 「逐字出现」的全部实现就是那一句 +=。
     */
    async sendMessage(content: string): Promise<void> {
      const personaId = this.currentPersonaId
      const text = content.trim()

      // 三种都不该发的理由：没选人 / 空消息 / 已经有一条流在跑。
      // 第三项同时保证了「发送中再按 Enter 不会重复发送」。
      if (personaId === null || text === '' || this.isStreaming) {
        return
      }

      this.errorMessage = ''
      this.isStreaming = true
      lastSentContent = text

      const placeholder = buildLocalMessage(personaId, 'assistant', '')
      // onDone 会把占位换成真实 id，所以留一个可变的引用给 finally 用
      let placeholderId = placeholder.id

      this.messages.push(buildLocalMessage(personaId, 'user', text), placeholder)

      // 用局部变量记住这次的 controller：finally 里靠它判断「我还是不是当前那条流」
      const controller = new AbortController()
      activeController = controller

      try {
        await chatApi.streamChat(
          { personaId, content: text },
          {
            onDelta: (delta) => {
              const target = this.messages.find((item) => item.id === placeholderId)

              if (target !== undefined) {
                target.content += delta
              }
            },

            onDone: (payload) => {
              const target = this.messages.find((item) => item.id === placeholderId)

              if (target !== undefined) {
                target.id = payload.messageId
              }

              placeholderId = payload.messageId
            },

            onError: (payload) => {
              // 契约 §6：error 事件时 HTTP 仍是 200，所以失败只能从这里拿到
              if (personaId === this.currentPersonaId) {
                this.errorMessage = payload.message
              }
            },
          },
          controller.signal,
        )
      } catch (error) {
        // 中断是用户自己的选择，不是失败，不该弹错误
        if (isAbortError(error) === false && personaId === this.currentPersonaId) {
          this.errorMessage = toErrorMessage(error, '网络异常，请检查网络后重试')
        }
      } finally {
        // 只有「我还是当前那条流」才复位。否则用户已经切走并开了新流，
        // 这一复位会把新流的 isStreaming 抹掉，输入框提前解禁。
        if (activeController === controller) {
          activeController = null
          this.isStreaming = false
        }

        // 一个字都没流出来的占位没有保留价值（一开始就失败、或被直接中断），删掉；
        // 已经流出一部分的保留 —— 用户已经看到的字不该凭空消失。
        const index = this.messages.findIndex((item) => item.id === placeholderId)

        if (index !== -1 && this.messages[index].content === '') {
          this.messages.splice(index, 1)
        }
      }
    },

    /**
     * 重发上一条。
     * 先把上一次失败留下的尾巴（AI 占位 + 用户消息）摘掉再发，
     * 否则每重试一次，列表里就多出一份重复的提问。
     */
    async retry(): Promise<void> {
      const personaId = this.currentPersonaId
      const text = lastSentContent

      // 先判再摘：条件不满足时提前返回，免得白白摘掉消息却不重发
      if (personaId === null || this.isStreaming || text === '') {
        return
      }

      if (this.messages.length > 0 && this.messages[this.messages.length - 1].role === 'assistant') {
        this.messages.pop()
      }

      if (this.messages.length > 0 && this.messages[this.messages.length - 1].role === 'user') {
        this.messages.pop()
      }

      await this.sendMessage(text)
    },
  },

  // 只持久化已读位与它归属的账号：uid 必须一起落盘，否则刷新后 uid 为 null，
  // 首次 loadPersonas 会把刚恢复的 readAt 当成「换了账号」清空
  persist: {
    key: 'heart-echo-chat',
    paths: ['readAt', 'uid'],
  },
})
