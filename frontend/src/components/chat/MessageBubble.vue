<script setup lang="ts">
import { computed } from 'vue'

import type { ChatMessage } from '@/types/chat'

const props = defineProps<{
  message: ChatMessage
}>()

/**
 * 「我发的」不能只看 role：契约 §5 的主动消息（isNudge）role 也是 'user'，
 * 但那不是用户打的字，必须按 AI 侧渲染。
 * 这条判据在整支里只出现这一次，其他任何地方都不要再写一遍。
 */
const isMine = computed(() => props.message.role === 'user' && props.message.isNudge === false)

/**
 * content 为空串的占位消息不渲染气泡 —— 首个 delta 到达前的等待态由 TypingIndicator 负责。
 * 两者接力的分界就是「content 从空变成非空」。
 */
</script>

<template>
  <div v-if="message.content !== ''" class="row" :class="isMine ? 'row-mine' : 'row-theirs'">
    <div class="bubble" :class="{ 'bubble-nudge': message.isNudge }">
      {{ message.content }}
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
}

.row-mine {
  justify-content: flex-end;
}

.row-theirs {
  justify-content: flex-start;
}

.bubble {
  max-width: 70%;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--el-fill-color-light);
}

.row-mine .bubble {
  background: var(--el-color-primary-light-9);
}

/* 主动消息：role 是 user 却渲染在 AI 侧，再加一道虚边框，免得看的人以为是数据错了 */
.bubble-nudge {
  border: 1px dashed var(--el-border-color);
}
</style>
