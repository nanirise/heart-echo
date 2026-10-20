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

/*
 * 窄屏：竖排侧栏固定 200px 会吃掉半屏宽（375px 屏下主内容只剩 127px 可用），
 * 改成一条横向可滚动的顶栏。断点 768px 与 ChatView 保持一致 ——
 * 自定义属性不能用在 @media 条件里，只能两边手写同一个值。
 */
@media (max-width: 768px) {
  .layout {
    flex-direction: column;
  }

  .sidebar {
    flex-direction: row;
    flex-shrink: 0;
    align-items: center;
    gap: var(--spacing-unit);
    width: auto;
    height: var(--app-nav-height);
    padding: 0 var(--app-gap);
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }

  .brand {
    flex-shrink: 0;
    padding: 0;
    font-size: var(--el-font-size-base);
  }

  /* min-width: 0 是让 flex 子项能小于内容宽度，否则 overflow-x 永远不触发 */
  .nav {
    flex-direction: row;
    flex: 1;
    min-width: 0;
    gap: 4px;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .nav::-webkit-scrollbar {
    display: none;
  }

  .nav-item {
    padding: 6px 10px;
    white-space: nowrap;
  }

  .user {
    flex-direction: row;
    flex-shrink: 0;
    align-items: center;
    padding: 0;
  }

  /* 窄屏空间要留给导航，用户名是纯噪音；退出按钮保留 */
  .username {
    display: none;
  }
}

</style>
