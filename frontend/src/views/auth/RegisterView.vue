<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'

import { ApiError } from '@/api/request'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const submitting = ref(false)
const errorMessage = ref('')

const form = reactive({
  username: '',
  email: '',
  password: '',
})

/**
 * 规则对齐契约 §3.1 的校验列，但前端不承担正确性：它只是提前拦下手误，
 * 后端仍会校验并返回 4001，两者冲突时以后端为准。
 * 长度一律用字符集正则判，不用 .length —— 否则中文、全角字符也算数。
 */
const rules: FormRules<typeof form> = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { pattern: /^[A-Za-z0-9]{3,20}$/, message: '用户名需 3-20 位字母或数字', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    // \x21-\x7E 是可打印 ASCII 去掉空格：后端用 bcrypt，它按字节在 72 处截断，
    // 放过中文/全角会让「8-32 位」这个约束在字节层面失效。
    { pattern: /^[\x21-\x7E]{8,32}$/, message: '密码需 8-32 位，且不含空格和中文', trigger: 'blur' },
  ],
}

const canSubmit = computed(
  () => form.username !== '' && form.email !== '' && form.password !== '',
)

async function handleSubmit(): Promise<void> {
  if (submitting.value) return

  // validate() 校验失败会 reject；转成布尔值，避免未捕获的 Promise 异常打到控制台
  const valid = await formRef.value?.validate().then(() => true).catch(() => false)
  if (!valid) return

  submitting.value = true
  errorMessage.value = ''

  try {
    await authStore.register({
      username: form.username,
      email: form.email,
      password: form.password,
    })
    // 契约 §3.1「注册即登录」：令牌已写进 store，直接进应用，不再把人丢回登录页
    await router.replace('/chat')
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : '注册失败，请稍后重试'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <h2 class="auth-title">注册 HeartEcho</h2>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="3-20 位字母或数字"
            autocomplete="username"
          />
        </el-form-item>

        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="请输入邮箱" autocomplete="email" />
        </el-form-item>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="8-32 位，不含空格和中文"
            autocomplete="new-password"
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
          {{ submitting ? '注册中…' : '注册' }}
        </el-button>
      </el-form>

      <p class="auth-switch">
        已有账号？
        <RouterLink to="/login">去登录</RouterLink>
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
