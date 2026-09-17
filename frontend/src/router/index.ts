import { defineComponent, h } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { setupRouterGuards } from './guards'

// 占位组件：路由引用的页面文件不存在时 vue-tsc 报 TS2307、构建直接失败。
// 各功能分支把 component 换成 () => import('@/views/...') 即可，路径与 meta 不动。
const Placeholder = defineComponent({
  name: 'RoutePlaceholder',
  render: () => h('div', '待实现'),
})

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: Placeholder,
    meta: { requiresAuth: false, title: '登录' },
  },
  {
    path: '/register',
    name: 'Register',
    component: Placeholder,
    meta: { requiresAuth: false, title: '注册' },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/chat',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'chat/:personaId?',
        name: 'Chat',
        component: Placeholder,
        props: true,
        meta: { title: '对话' },
      },
      { path: 'personas', name: 'Persona', component: Placeholder, meta: { title: '人设管理' } },
      { path: 'profile', name: 'Profile', component: Placeholder, meta: { title: '用户画像' } },
      { path: 'moments', name: 'Moments', component: Placeholder, meta: { title: '朋友圈' } },
      { path: 'schedules', name: 'Schedule', component: Placeholder, meta: { title: '日程提醒' } },
    ],
  },
  // 404 必须显式 false：否则未登录访问不存在的路径会被守卫拦到登录页，永远看不到 404
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: Placeholder,
    meta: { requiresAuth: false, title: '页面不存在' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

setupRouterGuards(router)

export default router