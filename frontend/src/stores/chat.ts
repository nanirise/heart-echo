import { defineStore } from 'pinia'

import { chatApi, listPersonas } from '@/api/mock'
import { ApiError, NETWORK_ERROR_CODE } from '@/api/request'
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
 * 把 catch 到的 unknown 收窄成一句能展示的文案。
 * 只有「请求根本没到后端」时才用兜底 —— 那时 message 是 axios 的英文原文
 * （Network Error / timeout of 15000ms exceeded），直接给用户看太难看。
 */
function toMessage(error: unknown): string {
  if (error instanceof ApiError && error.code !== NETWORK_ERROR_CODE) {
    return error.message
  }

  return '网络异常，请检查网络后重试'
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
    /** 未读标记（主动消息到达时点亮侧栏红点） */
    unread: {} as Record<number, boolean>,
  }),

  getters: {
    /** 当前人设对象；列表还没加载或 id 已失效时为 null */
    currentPersona(state): Persona | null {
      return state.personas.find((item) => item.id === state.currentPersonaId) ?? null
    },
  },

  actions: {
    // ── 纯数据操作 ──

    clearError(): void {
      this.errorMessage = ''
    },

    markUnread(personaId: number): void {
      this.unread[personaId] = true
    },

    // ── 数据加载 ──

    /**
     * 拉侧栏的人设列表。
     * 这里**不**顺手选中第一个：「当前在看谁」由路由参数唯一决定，
     * store 不该知道路由的存在，落点交给 ChatView 处理。
     */
    async loadPersonas(): Promise<void> {
      try {
        const result = await listPersonas()
        this.personas = result.list
      } catch (error) {
        this.errorMessage = toMessage(error)
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

        this.messages = list
      } catch (error) {
        if (personaId !== this.currentPersonaId) {
          return
        }

        this.errorMessage = toMessage(error)
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
        return
      }

      this.stopStreaming()

      this.currentPersonaId = personaId
      this.messages = []
      this.errorMessage = ''
      this.unread[personaId] = false
      lastSentContent = ''

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
          this.errorMessage = toMessage(error)
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
})
