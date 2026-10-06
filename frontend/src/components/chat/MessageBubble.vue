<script setup lang="ts">
import { computed } from 'vue'

import type { ChatMessage } from '@/types/chat'

const props = defineProps<{
  message: ChatMessage
}>()

const isMine = computed(() => props.message.role === 'user')

/**
 * content 为空串的占位消息不渲染气泡 —— 首个 delta 到达前的等待态由 TypingIndicator 负责。
 * 两者接力的分界就是「content 从空变成非空」。
 */
</script>

<template>
  <div v-if="message.content !== ''" class="row" :class="isMine ? 'row-mine' : 'row-theirs'">
    <div class="bubble">
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
</style>
