import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import router from '@/router'


// 故意用 @ 别名导入：一次验证 vite.config.ts 和 tsconfig.json 两处别名都配对了
import App from '@/App.vue'

const app = createApp(App)

// Pinia 是全局状态管理；插件负责把指定 store 自动持久化到 localStorage
const pinia = createPinia()
pinia.use(piniaPluginPersistedstate)

app.use(pinia)
app.use(ElementPlus)
app.use(router)

app.mount('#app')