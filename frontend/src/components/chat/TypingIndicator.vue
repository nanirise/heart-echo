<script setup lang="ts">
// 首个 delta 到达前的等待态。
// 故意不做跳动圆点那类动效：spec §2.2 把动效留给 Week 4 的视觉分支，
// 而且动画会干扰「流式到底通没通」这个判据 —— 静态文案有没有出现，一眼就能看清。
</script>

<template>
  <div class="typing">正在输入…</div>
</template>

<style scoped>
.typing {
  align-self: flex-start;
  padding: var(--spacing-unit) 12px;
  border-radius: var(--el-card-border-radius);
  font-size: var(--el-font-size-small);
  color: var(--color-text-secondary);
  background: var(--el-fill-color-light);
}
</style>
