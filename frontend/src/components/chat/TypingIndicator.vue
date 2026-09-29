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
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
}
</style>
