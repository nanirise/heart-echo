<script setup lang="ts">
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const navItems = [
  { path: '/chat', label: '对话' },
  { path: '/personas', label: '人设' },
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
  gap: 16px;
  flex-shrink: 0;
  width: 200px;
  padding: 16px 12px;
  border-right: 1px solid var(--el-border-color);
}

.brand {
  padding: 0 8px;
  font-size: 18px;
  font-weight: 500;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.nav-item {
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 14px;
  color: inherit;
  text-decoration: none;
}

.nav-item.router-link-active {
  background: var(--el-fill-color);
}

.user {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 8px;
}

.username {
  overflow: hidden;
  font-size: 13px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.content {
  flex: 1;
  min-width: 0;
  padding: 24px;
}
</style>
