<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'

import { toErrorMessage } from '@/api/error'
import { useAuthStore } from '@/stores/auth'

/**
 * `PersonaFormDialog.vue` 里那套 `computed` + `defineEmits('update:modelValue')`
 * 是同样效果的手工写法；这是 Vue 3.4 起把它收成一行的写法，本项目 `vue` 为 `^3.5.12`。
 * 用默认的 `v-model` 而不是 `v-model:visible`，是为了在调用侧与既有弹窗保持一致。
 */
const visible = defineModel<boolean>({ required: true })

const emit = defineEmits<{
  /** 密码已改成功；跳回登录页由父组件负责，与 `logout()` 只清状态、不管跳转的分工一致 */
  changed: []
}>()

const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const submitting = ref(false)
const errorMessage = ref('')

/**
 * `confirmPassword` 只在本组件里有意义 —— 它唯一的作用是拦住手误。
 * 请求体里放它现在不会报错（契约 §3.6 不是严格解码端点），
 * 但等哪天加严就会全线 4001，而且从现象上完全看不出与它有关。
 */
const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const rules: FormRules<typeof form> = {
  // 原密码只判必填：它可能是按更早的规则设下的，长度与字符集交给服务端比对定夺
  oldPassword: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  // 与注册页同一套正则，契约 §3.6 写的是「与注册同规则（限 ASCII 可见字符）」
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { pattern: /^[\x21-\x7E]{8,32}$/, message: '密码需 8-32 位，且不含空格和中文', trigger: 'blur' },
  ],
  // 只出现一次的校验器不做成具名函数：这里多一层跳转，读的人还得回头找比对的是谁
  confirmPassword: [
    {
      validator: (_rule, value: string, callback) => {
        if (value === '') {
          callback(new Error('请再次输入新密码'))
          return
        }
        if (value !== form.newPassword) {
          callback(new Error('两次输入的密码不一致'))
          return
        }
        callback()
      },
      trigger: 'blur',
    },
  ],
}

// 每次打开都从空白开始：上一次失败留在框里的密码不该等着被再次提交，原密码更不该
watch(visible, (open: boolean) => {
  if (!open) return

  form.oldPassword = ''
  form.newPassword = ''
  form.confirmPassword = ''
  errorMessage.value = ''
  // 弹窗内容是懒渲染的，首次打开时 formRef 还不存在，等一帧再清校验状态
  nextTick(() => formRef.value?.clearValidate())
})

// 新密码一改，上一次「两次不一致」的结论就作废了。
// 不重校验的话，用户已经改对了、红字却还挂着，只会以为没生效
watch(
  () => form.newPassword,
  () => {
    if (form.confirmPassword !== '') {
      void formRef.value?.validateField('confirmPassword')
    }
  },
)

async function handleSubmit(): Promise<void> {
  if (submitting.value) return

  const valid = await formRef.value?.validate().then(() => true).catch(() => false)
  if (!valid) return

  submitting.value = true
  errorMessage.value = ''

  try {
    // 只发这两个键，确认密码不进请求体
    await authStore.changePassword({
      oldPassword: form.oldPassword,
      newPassword: form.newPassword,
    })
    // changePassword 成功时已清空登录态（契约 §3.6：服务端不失效旧 token，本地自己作废）
    visible.value = false
    emit('changed')
  } catch (error) {
    // 4015「原密码不正确」的文案由后端 message 给出，前端不做二次映射
    errorMessage.value = toErrorMessage(error, '修改失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" title="修改密码" width="420px" :close-on-click-modal="false">
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
      <el-form-item label="原密码" prop="oldPassword">
        <el-input
          v-model="form.oldPassword"
          type="password"
          show-password
          autocomplete="current-password"
        />
      </el-form-item>

      <el-form-item label="新密码" prop="newPassword">
        <el-input
          v-model="form.newPassword"
          type="password"
          show-password
          autocomplete="new-password"
          placeholder="8-32 位，不含空格和中文"
        />
      </el-form-item>

      <el-form-item label="确认新密码" prop="confirmPassword">
        <el-input
          v-model="form.confirmPassword"
          type="password"
          show-password
          autocomplete="new-password"
        />
      </el-form-item>

      <el-alert
        v-if="errorMessage !== ''"
        :title="errorMessage"
        type="error"
        show-icon
        :closable="false"
      />
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ submitting ? '提交中…' : '确认修改' }}
      </el-button>
    </template>
  </el-dialog>
</template>
