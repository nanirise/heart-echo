<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormItemRule, FormRules } from 'element-plus'

import { createPersona, toErrorMessage, updatePersona } from '@/api/persona'
import type { Persona, PersonaPayload } from '@/types/persona'

const props = defineProps<{
  modelValue: boolean
  /** null = 新建；非 null = 编辑这一条 */
  persona: Persona | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = computed({
  get: (): boolean => props.modelValue,
  set: (value: boolean): void => emit('update:modelValue', value),
})

const isEdit = computed((): boolean => props.persona !== null)

const formRef = ref<FormInstance>()
const submitting = ref(false)
const errorMessage = ref('')

/** 请求体只由这三个字段构成，契约 §4 之外的一律不进 */
const form = reactive<PersonaPayload>({
  name: '',
  personalityDesc: '',
  speakingStyle: '',
})

/**
 * 必填的判据是 trim 后的空串，不是「非空字符串」：后端 `binding:"required"` 对字符串
 * 只判零值，一串空格能建出名字看不见的卡片。
 */
function requiredText(label: string): FormItemRule {
  return {
    required: true,
    validator: (_rule, value: string, callback) => {
      if (value.trim() === '') {
        callback(new Error(`请输入${label}`))
        return
      }
      callback()
    },
    trigger: 'blur',
  }
}

/** 三个长度上限与后端 `persona_dto.go` 的 binding tag 对齐，只是提前拦截 */
const rules: FormRules<typeof form> = {
  name: [requiredText('人设名'), { max: 50, message: '人设名不超过 50 个字符', trigger: 'blur' }],
  personalityDesc: [
    requiredText('性格描述'),
    { max: 2000, message: '性格描述不超过 2000 个字符', trigger: 'blur' },
  ],
  speakingStyle: [
    requiredText('说话风格'),
    { max: 255, message: '说话风格不超过 255 个字符', trigger: 'blur' },
  ],
}

watch(
  () => props.modelValue,
  (open: boolean) => {
    if (!open) return

    // 逐字段复制，不能把 form 指向列表里那条对象：否则用户在弹窗里打字时卡片会跟着变，
    // 请求还没发界面就已经"改好了"；点取消更糟——列表上留着没保存过的值
    form.name = props.persona?.name ?? ''
    form.personalityDesc = props.persona?.personalityDesc ?? ''
    form.speakingStyle = props.persona?.speakingStyle ?? ''
    errorMessage.value = ''
    nextTick(() => formRef.value?.clearValidate())
  },
)

async function handleSubmit(): Promise<void> {
  if (submitting.value) return

  // validate() 校验失败会 reject；转成布尔值，避免未捕获的 Promise 异常打到控制台
  const valid = await formRef.value?.validate().then(() => true).catch(() => false)
  if (!valid) return

  submitting.value = true
  errorMessage.value = ''

  const payload: PersonaPayload = {
    name: form.name.trim(),
    personalityDesc: form.personalityDesc.trim(),
    speakingStyle: form.speakingStyle.trim(),
  }

  try {
    if (props.persona === null) {
      await createPersona(payload)
      ElMessage.success('人设已创建')
    } else {
      await updatePersona(props.persona.id, payload)
      ElMessage.success('人设已更新')
    }
    emit('saved')
    visible.value = false
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '保存失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="isEdit ? '编辑人设' : '新建人设'"
    width="480px"
    :close-on-click-modal="false"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
      <el-form-item label="人设名" prop="name">
        <el-input
          v-model="form.name"
          :maxlength="50"
          show-word-limit
          placeholder="例如：小暖"
        />
      </el-form-item>

      <el-form-item label="性格描述" prop="personalityDesc">
        <el-input
          v-model="form.personalityDesc"
          type="textarea"
          :rows="4"
          :maxlength="2000"
          show-word-limit
          placeholder="例如：温柔、耐心，喜欢倾听"
        />
      </el-form-item>

      <el-form-item label="说话风格" prop="speakingStyle">
        <el-input
          v-model="form.speakingStyle"
          :maxlength="255"
          show-word-limit
          placeholder="例如：语气轻柔，偶尔用颜文字"
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
        {{ submitting ? '保存中…' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>
