<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import { Cloudy, Moon, Star, Sunny } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'

import AvatarBadge from '@/components/common/AvatarBadge.vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import type { Theme } from '@/stores/theme'

const router = useRouter()
const authStore = useAuthStore()
const themeStore = useThemeStore()

/** 图标对应当前主题本身，点击后按 THEMES 顺序进入下一套 */
const THEME_ICONS: Record<Theme, Component> = {
  light: Sunny,
  dark: Moon,
  pain: Cloudy,
  'custom-zhang': Star,
}

const THEME_LABELS: Record<Theme, string> = {
  light: '亮色',
  dark: '暗黑',
  pain: '青春疼痛',
  'custom-zhang': '私货',
}

const themeIcon = computed(() => THEME_ICONS[themeStore.theme])
const themeToggleLabel = computed(
  () => `当前主题：${THEME_LABELS[themeStore.theme]}，点击切换`,
)

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
        <div class="user-identity">
          <AvatarBadge
            :src="authStore.user?.avatarUrl"
            :name="authStore.user?.username"
            :size="28"
          />
          <span class="username">{{ authStore.user?.username ?? '未登录' }}</span>
        </div>
        <div class="user-actions">
          <el-button size="small" @click="handleLogout">退出登录</el-button>
          <el-button
            size="small"
            text
            circle
            :title="themeToggleLabel"
            :aria-label="themeToggleLabel"
            @click="themeStore.cycleTheme()"
          >
            <el-icon>
              <component :is="themeIcon" />
            </el-icon>
          </el-button>
        </div>
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

/* 退出登录留在左、主题切换推到右侧，两者共用一行 */
.user-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/*
 * 头像与用户名同排，用户名再用省略号收尾。
 * min-width: 0 是让这个 flex 子项能窄于内容宽度 —— 缺了它，长用户名会
 * 顶宽侧栏，ellipsis 永远不触发。
 */
.user-identity {
  display: flex;
  align-items: center;
  gap: var(--spacing-unit);
  min-width: 0;
}

.username {
  flex: 1;
  min-width: 0;
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

  /*
   * 窄屏空间要留给导航：用户名是纯噪音，收起来。
   * 头像留着 —— 它取代了原来那行用户名，宽度只多 28px，但一眼能看出是谁。
   */
  .username {
    display: none;
  }
}

</style>
