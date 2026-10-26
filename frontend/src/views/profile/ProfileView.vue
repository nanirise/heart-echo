<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'

import { toErrorMessage } from '@/api/error'
import { PERSONA_PAGE_SIZE, listPersonas } from '@/api/persona'
import { getPortrait } from '@/api/profile'
import AvatarBadge from '@/components/common/AvatarBadge.vue'
import { useAuthStore } from '@/stores/auth'
import type { ProfilePatch } from '@/stores/auth'
import type { Persona } from '@/types/persona'
import type { PortraitResponse } from '@/types/profile'
import { USER_AVATAR_PRESETS } from '@/utils/avatar'
import { formatDateTime } from '@/utils/datetime'

import PasswordDialog from './PasswordDialog.vue'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const passwordVisible = ref(false)
/** 外链输入默认收起：预设点选是主路径，贴链接只是少数情况的后门 */
const linkInputVisible = ref(false)

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

/* ---------- 画像区块：与人设绑定，一人设一份 ---------- */

const personas = ref<Persona[]>([])
const selectedPersonaId = ref<number | null>(null)
const portrait = ref<PortraitResponse | null>(null)
const portraitLoading = ref(true)

/** profileData 的键由提取链路决定，值是 unknown，统一转成可读字符串 */
function formatProfileValue(value: unknown): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) return value.map((item) => formatProfileValue(item)).join('、')

  return JSON.stringify(value)
}

/**
 * 空对象 / 全为空值的对象都算「还没画像」。
 * 后端现在恒返回 {}，但提取链路一旦产出，这里不用改就能显示。
 */
/**
 * 空对象 / 全为空值的对象都算「还没画像」。
 * 键集合由提取链路决定，这里按 entries 泛化渲染 —— 出现新键不用改代码。
 */
const portraitEntries = computed<Array<[string, string]>>(() => {
  const data = portrait.value?.profileData
  if (data === undefined) return []

  return Object.entries(data)
    .filter(([, value]) => value !== null && value !== undefined && value !== '')
    .map(([key, value]): [string, string] => [key, formatProfileValue(value)])
})

/**
 * 「更新于」的展示文本。契约 §7：尚未生成画像时 `updatedAt` 为 `null`。
 *
 * null 不能直接交给 formatDateTime —— `new Date(null)` 被当成 epoch，
 * 会渲染成 1970-01-01 而不是兜底符。这里收敛成空串，渲染层只判空。
 *
 * 先判 `portrait` 再判 `updatedAt`（而不是用可选链）：可选链会往类型里掺进
 * `undefined`，把「updatedAt 到底可不可空」这条信息糊掉，类型写错也不会报。
 */
const portraitUpdatedText = computed<string>(() => {
  const current = portrait.value
  if (current === null) return ''

  return current.updatedAt === null ? '' : formatDateTime(current.updatedAt)
})

async function loadPortrait(personaId: number): Promise<void> {
  portraitLoading.value = true
  try {
    portrait.value = await getPortrait(personaId)
  } catch {
    // 画像只是本页的次要信息源，请求失败时按空态呈现，
    // 不把一个可选区块表现成整页报错
    portrait.value = null
  } finally {
    portraitLoading.value = false
  }
}

async function loadPersonas(): Promise<void> {
  portraitLoading.value = true
  try {
    const result = await listPersonas(1, PERSONA_PAGE_SIZE)
    personas.value = result.list

    // 保留当前选择（重试场景）；列表里没有它了就落回第一个
    const target =
      personas.value.find((item) => item.id === selectedPersonaId.value) ?? personas.value[0]

    if (target === undefined) {
      portrait.value = null
      portraitLoading.value = false
      return
    }

    selectedPersonaId.value = target.id
    await loadPortrait(target.id)
  } catch {
    personas.value = []
    portrait.value = null
    portraitLoading.value = false
  }
}

function handlePortraitPersonaChange(personaId: number): void {
  void loadPortrait(personaId)
}

// 独立于账号资料的加载：画像接口没通时不该把「账号资料」一起拖成空白
onMounted(() => {
  void loadPersonas()
})

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
      <div class="card-head">
        <h2 class="section-title">我的画像</h2>

        <!-- 画像跟着人设走：切到谁就看谁 -->
        <el-select
          v-if="personas.length > 0"
          v-model="selectedPersonaId"
          class="persona-select"
          :disabled="portraitLoading"
          @change="handlePortraitPersonaChange"
        >
          <el-option
            v-for="persona in personas"
            :key="persona.id"
            :label="persona.name"
            :value="persona.id"
          />
        </el-select>
      </div>

      <el-skeleton v-if="portraitLoading" :rows="3" animated />

      <el-empty v-else-if="personas.length === 0" description="还没有人设" :image-size="60">
        <el-button type="primary" @click="router.push({ name: 'Persona' })">去人设管理</el-button>
      </el-empty>

      <div v-else-if="portraitEntries.length === 0" class="portrait-empty">
        <p class="portrait-empty-title">暂无画像数据</p>
        <p class="portrait-empty-hint">
          请在对话中多提及自己（如喜好、工作等），AI 会为你生成画像。
        </p>
      </div>

      <template v-else>
        <dl class="portrait-list">
          <div v-for="([label, value]) in portraitEntries" :key="label" class="portrait-item">
            <dt class="portrait-label">{{ label }}</dt>
            <dd class="portrait-value">{{ value }}</dd>
          </div>
        </dl>
        <p v-if="portraitUpdatedText !== ''" class="portrait-updated">
          更新于 {{ portraitUpdatedText }}
        </p>
      </template>
    </el-card>

    <el-card class="profile-card">
      <h2 class="section-title">账号资料</h2>

      <p v-if="loading" class="hint">加载中…</p>

      <el-form v-else ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="头像">
          <div class="avatar-picker">
            <!--
              预览跟着 form 走而不是 store：点完预设要立刻看到效果，
              不必先保存再刷新。store 里是上一次保存成功的值。
            -->
            <AvatarBadge :src="form.avatarUrl" :name="form.username" :size="72" />

            <div class="avatar-presets">
              <button
                v-for="(preset, index) in USER_AVATAR_PRESETS"
                :key="preset"
                type="button"
                class="avatar-preset"
                :class="{ 'is-active': preset === form.avatarUrl }"
                :aria-label="`使用预设头像 ${index + 1}`"
                @click="form.avatarUrl = preset"
              >
                <img :src="preset" alt="" />
              </button>
            </div>

            <div class="avatar-actions">
              <el-button link type="primary" @click="linkInputVisible = !linkInputVisible">
                {{ linkInputVisible ? '收起链接输入' : '或粘贴图片链接' }}
              </el-button>
              <!-- 取消头像也走同一条路：清空 → buildPatch 转成 null 清库 -->
              <el-button v-if="form.avatarUrl !== ''" link @click="form.avatarUrl = ''">
                清除头像
              </el-button>
            </div>

            <el-input
              v-if="linkInputVisible"
              v-model="form.avatarUrl"
              placeholder="粘贴图片链接，留空则清除头像"
              clearable
            />
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
      <h2 class="section-title">安全</h2>
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
  gap: var(--app-gap);
  max-width: var(--app-width-form);
}

.card-head {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--app-gap);
}

/* 全局 .section-title 自带 16px 下边距，放进标题行里会把下拉顶歪 */
.card-head .section-title {
  margin-bottom: 0;
}

.persona-select {
  width: 180px;
}

.portrait-empty {
  padding: 12px 0 4px;
}

.portrait-empty-title {
  margin: 0 0 var(--spacing-unit);
  font-size: var(--el-font-size-base);
  font-weight: 500;
  color: var(--color-text-main);
}

.portrait-empty-hint {
  margin: 0;
  font-size: var(--el-font-size-small);
  line-height: 1.6;
  color: var(--color-text-secondary);
}

.portrait-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 0;
}

.portrait-item {
  display: flex;
  gap: 12px;
  align-items: baseline;
}

.portrait-label {
  flex: 0 0 96px;
  font-size: var(--el-font-size-small);
  color: var(--color-text-secondary);
}

.portrait-value {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: var(--el-font-size-base);
  line-height: 1.6;
  color: var(--color-text-main);
  word-break: break-word;
}

.portrait-updated {
  margin: 12px 0 0;
  font-size: var(--el-font-size-extra-small);
  color: var(--color-text-placeholder);
}

.hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-small);
  color: var(--el-text-color-secondary);
}

.avatar-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

/*
 * 固定 5 列而不是 flex-wrap：预设是 10 个，flex 在 480px 内容宽里只会排成
 * 8 + 2 —— 第二行孤零零两个，比整齐的两行难看。5 列 × 44px + 4 × 12px = 268px，
 * 窄屏（375px）也放得下，不需要额外断点。
 */
.avatar-presets {
  display: grid;
  grid-template-columns: repeat(5, 44px);
  gap: 12px;
}

.avatar-preset {
  width: 44px;
  height: 44px;
  padding: 0;
  /*
   * 全站没有全局 box-sizing: border-box，默认 content-box 下 44px + 2px 边框
   * 会撑成 48px，网格列宽就对不上了。这里显式声明，让 44px 是含边框的总尺寸。
   */
  box-sizing: border-box;
  /*
   * 2px 边框一开始就占位，选中只改颜色。若改成「选中时才加 border」，
   * 圆心会偏移、后面几项也会被挤动一下。
   */
  border: 2px solid transparent;
  border-radius: 50%;
  background: none;
  cursor: pointer;
  overflow: hidden;
  transition: border-color 0.15s;
}

.avatar-preset img {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

.avatar-preset:hover {
  border-color: var(--color-border);
}

.avatar-preset.is-active {
  border-color: var(--color-primary);
}

.avatar-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.form-alert {
  margin-bottom: 12px;
}
</style>
