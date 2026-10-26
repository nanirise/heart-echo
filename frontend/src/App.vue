<script setup lang="ts">
import { watch } from 'vue'

import { applyTheme, useThemeStore } from '@/stores/theme'

const themeStore = useThemeStore()

/*
 * immediate 是必需的：store 从 localStorage 恢复了上次的主题，
 * 但刷新后 DOM 上还没有对应 class，这一轮 watch 负责补上。
 * 运行期点切换按钮时，状态变化会再走同一条路径。
 */
watch(
  () => themeStore.theme,
  (theme) => {
    applyTheme(theme)
  },
  { immediate: true },
)
</script>

<template>
  <RouterView />
</template>
