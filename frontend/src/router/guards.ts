import { ElMessage } from 'element-plus'
import type { Router } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

export function setupRouterGuards(router: Router): void {
  router.beforeEach((to) => {
    const authStore = useAuthStore()

    // 缺省视为需要登录：新增页面漏写标记时宁可多拦一次，也不能少一道防线
    if (to.meta.requiresAuth !== false && !authStore.isLogin) {
      ElMessage.warning('请先登录')
      return { name: 'Login', query: { redirect: to.fullPath } }
    }

    if (authStore.isLogin && (to.name === 'Login' || to.name === 'Register')) {
      return { name: 'Chat' }
    }

    return true
  })

  router.afterEach((to) => {
    document.title = to.meta.title ? `${to.meta.title} · HeartEcho` : 'HeartEcho'
  })
}
