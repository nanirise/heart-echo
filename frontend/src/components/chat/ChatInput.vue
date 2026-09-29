<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
  /** 有流正在进行时为 true：输入框禁用，发送按钮换成「停止生成」 */
  streaming: boolean
}>()

const emit = defineEmits<{
  send: [content: string]
  stop: []
}>()

const draft = ref('')

const canSend = computed(() => draft.value.trim() !== '')

function send(): void {
  if (props.streaming || !canSend.value) {
    return
  }

  emit('send', draft.value.trim())
  draft.value = ''
}

/**
 * 自己判 Enter，而不是用 @keydown.enter.prevent 那套修饰符 —— 图的是能看见 isComposing：
 * 用中文输入法打「你好」时，按 Enter 是在**选字**。不看这个标记就会把半句话当消息发出去，
 * 中文用户几乎必然踩到，而且看起来像「偶发」。
 */
function handleKeydown(event: KeyboardEvent): void {
  if (event.isComposing) {
    return
  }

  if (event.key === 'Enter' && event.shiftKey === false) {
    event.preventDefault()
    send()
  }
}
</script>

<template>
  <div class="chat-input">
    <div class="chat-input-field">
      <el-input
        v-model="draft"
        type="textarea"
        :rows="2"
        resize="none"
        :disabled="streaming"
        placeholder="说点什么……（Enter 发送，Shift + Enter 换行）"
        @keydown="handleKeydown"
      />
    </div>

    <el-button v-if="streaming" @click="emit('stop')">停止生成</el-button>
    <el-button v-else type="primary" :disabled="!canSend" @click="send">发送</el-button>
  </div>
</template>

<style scoped>
.chat-input {
  display: flex;
  gap: 8px;
  align-items: flex-end;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color);
}

.chat-input-field {
  flex: 1;
}
</style>
