<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'

import { ApiError } from '@/api/request'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const submitting = ref(false)
const errorMessage = ref('')

const form = reactive({
  username: '',
  password: '',
})

/**
 * 只校验非空。格式规则（用户名 3-20 位、密码 8-32 位）是「创建密码」时的约束，
 * 放到登录页会让规则收紧后的老用户连登录框都提交不出去，也等于把密码策略印在登录页上。
 */
const rules: FormRules<typeof form> = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

const canSubmit = computed(() => form.username !== '' && form.password !== '')

/**
 * 只接受站内路径。直接跳 query 里的值就是开放重定向：
 * /login?redirect=https://evil.com 会把用户送去站外，而地址栏那条链接确实来自本域名。
 * 交给 URL 解析后比 origin，可一并挡掉 //evil.com 与 /\evil.com 两种变体。
 */
function resolveRedirect(raw: unknown): string {
  if (typeof raw !== 'string') return '/chat'

  const target = new URL(raw, window.location.origin)
  if (target.origin !== window.location.origin) return '/chat'

  return target.pathname + target.search
}

async function handleSubmit(): Promise<void> {
  if (submitting.value) return

  // validate() 校验失败会 reject；转成布尔值，避免未捕获的 Promise 异常打到控制台
  const valid = await formRef.value?.validate().then(() => true).catch(() => false)
  if (!valid) return

  submitting.value = true
  errorMessage.value = ''

  try {
    await authStore.login({ username: form.username, password: form.password })
    // replace 而不是 push：登录页不该留在历史记录里，否则按「后退」会退回登录框
    await router.replace(resolveRedirect(route.query.redirect))
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : '登录失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <h2 class="auth-title">登录 HeartEcho</h2>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="请输入用户名" autocomplete="username" />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="请输入密码"
            autocomplete="current-password"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-alert
          v-if="errorMessage !== ''"
          :title="errorMessage"
          type="error"
          show-icon
          :closable="false"
          class="auth-error"
        />

        <el-button
          type="primary"
          class="auth-submit"
          :loading="submitting"
          :disabled="!canSubmit"
          @click="handleSubmit"
        >
          {{ submitting ? '登录中…' : '登录' }}
        </el-button>
      </el-form>

      <p class="auth-switch">
        还没有账号？
        <RouterLink to="/register">去注册</RouterLink>
      </p>
    </el-card>
  </div>
</template>

<style scoped>
.auth-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--el-fill-color-light);
}

.auth-card {
  width: 360px;
}

.auth-title {
  margin: 0 0 20px;
  font-size: 20px;
  font-weight: 500;
  text-align: center;
}

.auth-error {
  margin-bottom: 12px;
}

.auth-submit {
  width: 100%;
}

.auth-switch {
  margin: 16px 0 0;
  font-size: 13px;
  text-align: center;
}

.auth-switch a {
  color: var(--el-color-primary);
  text-decoration: none;
}
</style>
