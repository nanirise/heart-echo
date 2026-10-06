<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'

import { toErrorMessage } from '@/api/error'
import { listPersonas, proactiveApi } from '@/api/mock'
import { PERSONA_PAGE_SIZE } from '@/api/persona'
import type { Persona } from '@/types/persona'
import type { SettingsResponse } from '@/types/proactive'

const router = useRouter()

const personas = ref<Persona[]>([])
const selectedPersonaId = ref<number | null>(null)
/**
 * 表单当前装着谁的配置；null 表示还没成功加载过任何配置。
 * 保存只认它（而不是下拉里的值）—— 否则切换加载失败时，会把上一位的配置写进新选的人设。
 */
const loadedPersonaId = ref<number | null>(null)

const loading = ref(true)
const loadingSettings = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const lastNudgeAt = ref<string | null>(null)

const formRef = ref<FormInstance>()

const form = reactive({
  enabled: true,
  intervalMin: 30,
  intervalMax: 120,
  dailyLimit: 3,
})

/** el-input-number 清空会产出 undefined / NaN，所以先收窄类型再判范围 */
function isIntInRange(value: unknown, min: number, max: number): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= min && value <= max
}

/** 规则对齐契约 §9 的约束：间隔 1-1440 且 min < max，日上限 1-10。前端只做提前拦截，后端仍是权威 */
const rules: FormRules<typeof form> = {
  intervalMin: [
    {
      validator: (_rule, value, callback) => {
        if (!isIntInRange(value, 1, 1440)) {
          callback(new Error('请输入 1-1440 之间的整数分钟数'))
          return
        }

        if (value >= form.intervalMax) {
          callback(new Error('最小间隔必须小于最大间隔'))
          return
        }

        callback()
      },
      trigger: 'change',
    },
  ],
  intervalMax: [
    {
      validator: (_rule, value, callback) => {
        if (!isIntInRange(value, 1, 1440)) {
          callback(new Error('请输入 1-1440 之间的整数分钟数'))
          return
        }

        if (value <= form.intervalMin) {
          callback(new Error('最大间隔必须大于最小间隔'))
          return
        }

        callback()
      },
      trigger: 'change',
    },
  ],
  dailyLimit: [
    {
      validator: (_rule, value, callback) => {
        if (!isIntInRange(value, 1, 10)) {
          callback(new Error('请输入 1-10 之间的整数条数'))
          return
        }

        callback()
      },
      trigger: 'change',
    },
  ],
}

/** 契约的时间是 RFC3339，交给 Date 解析 —— 按字符串切片在精度或时区变化时会静默错位 */
function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  const pad = (part: number): string => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`
}

/** 加载与保存共用：保存后回填能显示服务端规范化（夹取）后的值 */
function fillForm(settings: SettingsResponse): void {
  selectedPersonaId.value = settings.personaId
  loadedPersonaId.value = settings.personaId

  form.enabled = settings.enabled
  form.intervalMin = settings.intervalMin
  form.intervalMax = settings.intervalMax
  form.dailyLimit = settings.dailyLimit
  lastNudgeAt.value = settings.lastNudgeAt

  // 清掉上一位人设留下的校验红字，否则会显示在无关的新配置上
  formRef.value?.clearValidate()
}

async function loadSettings(personaId: number): Promise<void> {
  fillForm(await proactiveApi.getSettings(personaId))
}

async function loadPage(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    const result = await listPersonas(1, PERSONA_PAGE_SIZE)
    personas.value = result.list

    // 保留当前选择（重试场景）；列表里没有它了（别处删了 / 换了账号）就落回第一个
    const target =
      personas.value.find((item) => item.id === selectedPersonaId.value) ?? personas.value[0]

    if (target !== undefined) {
      await loadSettings(target.id)
    }
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

async function handlePersonaChange(personaId: number): Promise<void> {
  if (loadingSettings.value) {
    // el-select 的 v-model 已经改成新人，直接 return 会「下拉显示新人、表单还是旧配置」且不退回
    selectedPersonaId.value = loadedPersonaId.value
    return
  }

  errorMessage.value = ''
  successMessage.value = ''
  loadingSettings.value = true

  try {
    await loadSettings(personaId)
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '配置加载失败，请重试')
    // 表单里还是上一位的配置：把下拉退回它，避免「界面显示 B、保存写的是 A 的配置」
    selectedPersonaId.value = loadedPersonaId.value
  } finally {
    loadingSettings.value = false
  }
}

async function handleSave(): Promise<void> {
  const personaId = loadedPersonaId.value

  // loadedPersonaId 为 null 时表单拿的不是真实配置，禁止保存
  if (personaId === null || saving.value) {
    return
  }

  const valid = await formRef.value?.validate().then(() => true).catch(() => false)

  if (!valid) {
    return
  }

  saving.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    const settings = await proactiveApi.updateSettings({
      personaId,
      enabled: form.enabled,
      intervalMin: form.intervalMin,
      intervalMax: form.intervalMax,
      dailyLimit: form.dailyLimit,
    })

    fillForm(settings)
    successMessage.value = '已保存'
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '保存失败，请稍后重试')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void loadPage()
})
</script>

<template>
  <div class="proactive-view">
    <el-card class="settings-card">
      <h2 class="card-title">主动消息设置</h2>

      <el-skeleton v-if="loading" :rows="4" animated />

      <!-- 人设列表本身没拿到：区分「加载失败」与「真的没有人设」两种空 -->
      <div v-else-if="personas.length === 0" class="page-state">
        <template v-if="errorMessage !== ''">
          <el-alert :title="errorMessage" type="error" show-icon :closable="false" />
          <el-button @click="loadPage">重试</el-button>
        </template>
        <el-empty v-else description="还没有人设" :image-size="60">
          <el-button type="primary" @click="router.push({ name: 'Persona' })">
            创建人设
          </el-button>
        </el-empty>
      </div>

      <!-- 人设拿到了但配置没加载出来：表单绝不能拿默认值冒充真实配置 -->
      <div v-else-if="loadedPersonaId === null" class="page-state">
        <el-alert :title="errorMessage" type="error" show-icon :closable="false" />
        <el-button @click="loadPage">重试</el-button>
      </div>

      <template v-else>
        <p class="hint">开启后，伴侣会在你离开一段时间后主动发来消息。</p>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          :disabled="loadingSettings"
        >
          <el-form-item label="伴侣">
            <el-select
              v-model="selectedPersonaId"
              class="persona-select"
              @change="handlePersonaChange"
            >
              <el-option
                v-for="persona in personas"
                :key="persona.id"
                :label="persona.name"
                :value="persona.id"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="启用主动消息">
            <el-switch v-model="form.enabled" active-text="开启" inactive-text="关闭" />
          </el-form-item>

          <el-form-item label="最小间隔（分钟）" prop="intervalMin">
            <el-input-number
              v-model="form.intervalMin"
              class="number-input"
              :min="1"
              :max="1440"
              :step="5"
            />
          </el-form-item>

          <el-form-item label="最大间隔（分钟）" prop="intervalMax">
            <el-input-number
              v-model="form.intervalMax"
              class="number-input"
              :min="1"
              :max="1440"
              :step="5"
            />
          </el-form-item>

          <el-form-item label="每日上限（条）" prop="dailyLimit">
            <el-input-number v-model="form.dailyLimit" class="number-input" :min="1" :max="10" />
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

          <el-button type="primary" :loading="saving" @click="handleSave">
            {{ saving ? '保存中…' : '保存' }}
          </el-button>
        </el-form>

        <p class="nudge-hint">
          最近一次主动消息：{{ lastNudgeAt === null ? '暂无' : formatDateTime(lastNudgeAt) }}
        </p>
      </template>
    </el-card>
  </div>
</template>

<style scoped>
.proactive-view {
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

.page-state {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.persona-select {
  width: 240px;
}

.number-input {
  width: 160px;
}

.form-alert {
  margin-bottom: 12px;
}

.nudge-hint {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
</style>
