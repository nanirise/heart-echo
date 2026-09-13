<script setup lang="ts">
import { ref } from 'vue'

const appName = 'HeartEcho'
const clickCount = ref<number>(0)
</script>

<template>
  <main class="shell">
    <h1 class="title">{{ appName }}</h1>
    <p class="subtitle">前端脚手架自检页</p>

    <el-button type="primary" @click="clickCount += 1">
      点我 +1（当前 {{ clickCount }}）
    </el-button>

    <p v-if="clickCount > 0" class="hint">
      Vue 响应式 + Element Plus 均已就绪
    </p>
  </main>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  margin: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  font-family: system-ui, -apple-system, 'Segoe UI', 'Microsoft YaHei', sans-serif;
}

.title {
  margin: 0;
  font-size: 40px;
  letter-spacing: 1px;
}

.subtitle {
  margin: 0;
  color: #888;
}

.hint {
  margin: 0;
  color: #2f855a;
  font-size: 14px;
}
</style>
