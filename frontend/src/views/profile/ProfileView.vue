<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'

import { toErrorMessage } from '@/api/error'
import { useAuthStore } from '@/stores/auth'
import type { ProfilePatch } from '@/stores/auth'

import PasswordDialog from './PasswordDialog.vue'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const passwordVisible = ref(false)

const form = reactive({
  username: '',
  avatarUrl: '',
})

/**
 * 规则对齐契约 §3.5 的约束列，前端只做提前拦截，后端仍是权威。
 * avatarUrl 不加校验：留空是合法的（表示清空头像），长度也交给后端管。
 */
const rules: FormRules<typeof form> = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { pattern: /^[A-Za-z0-9]{3,20}$/, message: '用户名需 3-20 位字母或数字', trigger: 'blur' },
  ],
}

/**
 * 空串会被 el-avatar 当成一个真实地址去请求，所以换成 undefined 表示「没有头像」，
 * 让它直接走兜底插槽。只判 null/undefined 是不够的 —— 输入框清空留下的是 ''。
 */
const avatarSrc = computed<string | undefined>(() => {
  const url = authStore.user?.avatarUrl

  return url === null || url === undefined || url === '' ? undefined : url
})

/** 契约 §3.5：avatarUrl 传 null 清空时，前端用用户名首字母兜底 */
const avatarFallback = computed(() => (authStore.user?.username ?? '?').charAt(0).toUpperCase())

function syncForm(): void {
  form.username = authStore.user?.username ?? ''
  form.avatarUrl = authStore.user?.avatarUrl ?? ''
}

// 进页面重新拉一次（契约 §3.4）：store 里的 user 是登录那一刻的快照并被持久化，
// 在别处改过资料的话，这个页面上显示的就是旧值
onMounted(async () => {
  try {
    await authStore.fetchProfile()
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '资料加载失败，当前显示的是本地缓存')
  } finally {
    // 无论成败都要填表单：失败时用 store 的缓存兜底，页面不至于空白打不开
    syncForm()
    loading.value = false
  }
})

/**
 * 收集「真的改过」的字段。
 *
 * 为什么不能直接把整个 user 对象 PUT 上去：契约 §3.5 是严格解码端点，
 * 多一个键就返回 4001，而 id / email / createdAt 在那里都是「不认识的键」。
 *
 * 而「改没改」有三种情况，不是两种：键不出现 = 不要动；传 null = 清空；传值 = 改成它。
 * 输入框删空得到的是空字符串而不是 null，必须在这里转一次，否则会被判成非法 URL。
 */
function buildPatch(): ProfilePatch {
  const patch: ProfilePatch = {}
  const current = authStore.user

  if (current === null) {
    return patch
  }

  const username = form.username.trim()

  if (username !== current.username) {
    patch.username = username
  }

  const avatarUrl = form.avatarUrl.trim()

  // ?? '' 把「store 里是 null」与「输入框是空串」归一到同一种空；
  // 不归一的话，什么都没改也会被判成「从 null 变成了空串」
  if (avatarUrl !== (current.avatarUrl ?? '')) {
    patch.avatarUrl = avatarUrl === '' ? null : avatarUrl
  }

  return patch
}

async function handleSubmit(): Promise<void> {
  if (submitting.value) return

  const valid = await formRef.value?.validate().then(() => true).catch(() => false)

  if (!valid) return

  const patch = buildPatch()

  // 空 patch 是合法请求（契约 §3.5 明写），但发它是白跑一次往返；
  // 用户也该看到「没有改动」这个反馈，而不是一个声称保存成功的 200
  if (Object.keys(patch).length === 0) {
    errorMessage.value = ''
    successMessage.value = '没有需要保存的改动'
    return
  }

  submitting.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    await authStore.updateProfile(patch)
    // store 已被响应更新，回填一次以显示后端规范化后的值
    syncForm()
    successMessage.value = '已保存'
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '保存失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

// changePassword 成功后 store 已经清空登录态（契约 §3.6：服务端不失效旧 token，本地自己作废）。
// 跳转放在视图这一层，与 logout() 只清状态、跳转归调用方的分工一致
function handlePasswordChanged(): void {
  void router.replace('/login')
}
</script>

<template>
  <div class="profile-view">
    <el-card class="profile-card">
      <h2 class="card-title">账号资料</h2>

      <p v-if="loading" class="hint">加载中…</p>

      <el-form v-else ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="头像">
          <div class="avatar-row">
            <el-avatar :size="64" :src="avatarSrc">{{ avatarFallback }}</el-avatar>
            <el-input v-model="form.avatarUrl" placeholder="粘贴图片链接，留空则清除头像" />
          </div>
        </el-form-item>

        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="3-20 位字母或数字" />
        </el-form-item>

        <el-alert
          v-if="errorMessage !== ''"
          :title="errorMessage"
          type="error"
          show-icon
          :closable="false"
          class="form-alert"
        />
        <el-alert
          v-if="successMessage !== ''"
          :title="successMessage"
          type="success"
          show-icon
          :closable="false"
          class="form-alert"
        />

        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ submitting ? '保存中…' : '保存' }}
        </el-button>
      </el-form>
    </el-card>

    <el-card class="profile-card">
      <h2 class="card-title">安全</h2>
      <p class="hint">修改密码后需要重新登录。</p>
      <el-button @click="passwordVisible = true">修改密码</el-button>
    </el-card>

    <PasswordDialog v-model="passwordVisible" @changed="handlePasswordChanged" />
  </div>
</template>

<style scoped>
.profile-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 520px;
}

.card-title {
  margin: 0 0 16px;
  font-size: 16px;
  font-weight: 500;
}

.hint {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.avatar-row {
  display: flex;
  gap: 12px;
  align-items: center;
  width: 100%;
}

.form-alert {
  margin-bottom: 12px;
}
</style>
