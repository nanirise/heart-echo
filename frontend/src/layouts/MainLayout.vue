<script setup lang="ts">
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const navItems = [
  { path: '/chat', label: '对话' },
  { path: '/personas', label: '人设' },
  { path: '/proactive', label: '主动消息' },
  { path: '/profile', label: '我的画像' },
  { path: '/moments', label: '朋友圈' },
  { path: '/schedules', label: '日程提醒' },
]

function handleLogout(): void {
  authStore.logout()
  router.replace('/login')
}
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand">HeartEcho</div>

      <nav class="nav">
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path" class="nav-item">
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="user">
        <span class="username">{{ authStore.user?.username ?? '未登录' }}</span>
        <el-button size="small" @click="handleLogout">退出登录</el-button>
      </div>
    </aside>

    <main class="content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  min-height: 100vh;
}

.sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--app-gap);
  flex-shrink: 0;
  width: 200px;
  padding: var(--app-gap) 12px;
  border-right: 1px solid var(--color-border);
}

.brand {
  padding: 0 var(--spacing-unit);
  font-size: var(--el-font-size-large);
  font-weight: 500;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: calc(var(--spacing-unit) / 2);
  flex: 1;
}

.nav-item {
  padding: var(--spacing-unit) 12px;
  border-radius: var(--border-radius-base);
  font-size: var(--el-font-size-base);
  color: inherit;
  text-decoration: none;
}

.nav-item.router-link-active {
  background: var(--el-fill-color);
}

.user {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-unit);
  padding: 0 var(--spacing-unit);
}

.username {
  overflow: hidden;
  font-size: var(--el-font-size-small);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.content {
  flex: 1;
  min-width: 0;
  padding: var(--app-gutter);
}
</style>
