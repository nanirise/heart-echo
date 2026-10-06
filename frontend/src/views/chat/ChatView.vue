<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import ChatInput from '@/components/chat/ChatInput.vue'
import MessageBubble from '@/components/chat/MessageBubble.vue'
import TypingIndicator from '@/components/chat/TypingIndicator.vue'
import { useChatStore } from '@/stores/chat'

const route = useRoute()
const router = useRouter()
const chatStore = useChatStore()

const loadingPersonas = ref(true)
const scrollContainer = ref<HTMLElement | null>(null)

/**
 * 路由参数是字符串、还可能缺失；store 里的 currentPersonaId 是 number | null。
 * 这层翻译只在这里做一次，模板和子组件一律只认 store。
 * Number('') 是 0、Number('abc') 是 NaN —— 不显式挡掉就会拿 0 去查人设。
 */
function parsePersonaId(raw: unknown): number | null {
  if (typeof raw !== 'string') {
    return null
  }

  const id = Number(raw)

  return Number.isInteger(id) && id > 0 ? id : null
}

const routePersonaId = computed(() => parsePersonaId(route.params.personaId))

const lastMessage = computed(() => {
  const list = chatStore.messages

  return list.length > 0 ? list[list.length - 1] : null
})

/**
 * 等待态与正文的接力判据：流在跑、且最后一条还是空占位，说明第一个 delta 没到。
 * 首个 delta 一到，占位的 content 变成非空，MessageBubble 立刻接手渲染，这条判据自然失效。
 * 所以不需要额外的「已开始输出」标志位，一个 content 字段就够两态。
 */
const showTyping = computed(() => {
  return chatStore.isStreaming && lastMessage.value !== null && lastMessage.value.content === ''
})

/** 列表为空时不给「重试」按钮 —— 历史加载失败这种情况按了也没东西可重发 */
const canRetry = computed(() => chatStore.errorMessage !== '' && chatStore.messages.length > 0)

// ① URL → store：地址栏变，store 跟着变。
// 切人设时要中断旧流，这一步在 selectPersona 内部已经做了，这里不用重复
watch(
  routePersonaId,
  (personaId) => {
    if (personaId !== null) {
      chatStore.selectPersona(personaId)
    }
  },
  { immediate: true },
)

// ② store → URL：URL 没带 id 时落到第一个人设。
// 必须等列表回来才知道「第一个人」是谁，所以这一步只能在加载完之后做
onMounted(async () => {
  await chatStore.loadPersonas()
  loadingPersonas.value = false

  if (routePersonaId.value !== null) {
    return
  }

  const first = chatStore.personas[0]

  if (first !== undefined) {
    await router.replace({ name: 'Chat', params: { personaId: String(first.id) } })
  }
})

// ③ 离开页面就断流（spec §2 硬性约束第 6 条），兼做 ④ 的解绑
// 切人设的场景已被 ① 覆盖，这里补的是「整页切走」（点侧栏去人设 / 画像）——
// 那种情况组件会被卸载，这是必经之路
onBeforeUnmount(() => {
  chatStore.stopStreaming()
  document.removeEventListener('visibilitychange', handleForeground)
  window.removeEventListener('focus', handleForeground)
})

// ④ 回前台刷新（spec §5.3）：
// 主动消息由后端定时落库，停在对话页不动看不到；只做「离开又回来」，不做常驻订阅
function handleForeground(): void {
  // visibilitychange 切到后台时也会触发，那一次不需要刷新
  if (document.visibilityState !== 'visible') {
    return
  }

  void chatStore.refreshActiveConversation()
}

onMounted(() => {
  document.addEventListener('visibilitychange', handleForeground)
  window.addEventListener('focus', handleForeground)
})

/**
 * 贴底。deep 是必需的：流式追加走的是 content += delta，数组长度不变，浅比较看不到变化。
 * watch 默认 flush 是 'pre'（回调跑在 DOM 更新之前），所以要先 nextTick 再量 scrollHeight。
 */
watch(
  () => chatStore.messages,
  async () => {
    await nextTick()

    const container = scrollContainer.value

    if (container !== null) {
      container.scrollTop = container.scrollHeight
    }
  },
  { deep: true },
)

function handleSend(content: string): void {
  void chatStore.sendMessage(content)
}

function handleRetry(): void {
  void chatStore.retry()
}
</script>

<template>
  <div class="chat-view">
    <aside class="persona-list">
      <p v-if="loadingPersonas" class="list-hint">加载中…</p>

      <el-empty
        v-else-if="chatStore.personas.length === 0"
        description="还没有人设"
        :image-size="60"
      >
        <el-button size="small" @click="router.push({ name: 'Persona' })">创建人设</el-button>
      </el-empty>

      <ul v-else class="list-items">
        <li v-for="persona in chatStore.personas" :key="persona.id">
          <RouterLink
            class="list-item"
            :class="{ 'list-item-active': persona.id === chatStore.currentPersonaId }"
            :to="{ name: 'Chat', params: { personaId: String(persona.id) } }"
          >
            <span class="item-name">
              {{ persona.name }}
              <span v-if="chatStore.unreadPersonaIds.has(persona.id)" class="item-dot" />
            </span>
            <span class="item-meta">亲密度 {{ persona.familiarity }}</span>
          </RouterLink>
        </li>
      </ul>
    </aside>

    <section class="conversation">
      <template v-if="chatStore.currentPersona !== null">
        <header class="conversation-head">
          <h1 class="conversation-name">{{ chatStore.currentPersona.name }}</h1>
          <p class="conversation-desc">{{ chatStore.currentPersona.personalityDesc }}</p>
        </header>

        <div ref="scrollContainer" class="message-list">
          <MessageBubble
            v-for="message in chatStore.messages"
            :key="message.id"
            :message="message"
          />

          <TypingIndicator v-if="showTyping" />
        </div>

        <div v-if="chatStore.errorMessage !== ''" class="chat-error">
          <span class="error-text">{{ chatStore.errorMessage }}</span>
          <el-button v-if="canRetry" size="small" @click="handleRetry">重试</el-button>
          <el-button size="small" @click="chatStore.clearError()">关闭</el-button>
        </div>

        <ChatInput
          :streaming="chatStore.isStreaming"
          @send="handleSend"
          @stop="chatStore.stopStreaming"
        />
      </template>

      <p v-else class="conversation-empty">从左侧选一位开始聊天</p>
    </section>
  </div>
</template>

<style scoped>
/* .content（MainLayout）没有确定高度，`height: 100%` 会塌成 0、内部滚动条失效；
   layout 的 min-height 是 100vh、content 上下 padding 各 24px，所以减去 48px */
.chat-view {
  display: flex;
  gap: 16px;
  height: calc(100vh - 48px);
}

.persona-list {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 180px;
  overflow-y: auto;
  padding-right: 12px;
  border-right: 1px solid var(--el-border-color);
}

.list-hint {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.list-items {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.list-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border-radius: 6px;
  color: inherit;
  text-decoration: none;
}

.list-item:hover {
  background: var(--el-fill-color-light);
}

.list-item-active {
  background: var(--el-fill-color);
}

.item-name {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 14px;
  font-weight: 500;
}

.item-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--el-color-danger);
}

.item-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.conversation {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.conversation-head {
  padding-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color);
}

.conversation-name {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
}

.conversation-desc {
  margin: 4px 0 0;
  overflow: hidden;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
  overflow-y: auto;
  padding: 16px 4px;
}

.conversation-empty {
  margin: auto;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.chat-error {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--el-color-danger-light-9);
}

.error-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--el-color-danger);
}
</style>
