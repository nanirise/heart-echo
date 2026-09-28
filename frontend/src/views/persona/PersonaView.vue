<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import {
  PERSONA_PAGE_SIZE,
  deletePersona,
  isPersonaNotFound,
  listPersonas,
  toErrorMessage,
} from '@/api/persona'
import type { Persona } from '@/types/persona'

import PersonaFormDialog from './PersonaFormDialog.vue'

const personas = ref<Persona[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const errorMessage = ref('')
/** 正在删除的那一条；用于按钮 loading，防止连点两次 */
const deletingId = ref<number | null>(null)

const dialogVisible = ref(false)
const editingPersona = ref<Persona | null>(null)

/**
 * 契约的时间是 RFC3339 且带 6 位小数（`2026-09-28T13:53:16.172057+08:00`），
 * 必须交给 Date 解析 —— 按字符串切片在精度或时区变化时会静默错位。
 */
function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  const pad = (part: number): string => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`
}

async function load(targetPage: number = page.value): Promise<void> {
  loading.value = true
  errorMessage.value = ''

  try {
    const result = await listPersonas(targetPage, PERSONA_PAGE_SIZE)
    personas.value = result.list
    total.value = result.total
    // 用后端收敛后的页码：契约规定 page=0 / pageSize=1000 只夹到合法范围、不报错
    page.value = result.page
  } catch (error) {
    personas.value = []
    total.value = 0
    errorMessage.value = toErrorMessage(error, '人设列表加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  editingPersona.value = null
  dialogVisible.value = true
}

function openEdit(persona: Persona): void {
  editingPersona.value = persona
  dialogVisible.value = true
}

/**
 * 保存后一律重拉，不在本地插排：排序权在后端（`lastMessageAt DESC NULLS LAST, id DESC`），
 * 新建的人设 `lastMessageAt` 为 null，该排第几取决于这个账号有多少条"聊过"的人设 ——
 * 本地算出来的位置会和刷新后的不一致。代价是多一次请求，量级可忽略。
 */
async function handleSaved(): Promise<void> {
  await load(editingPersona.value === null ? 1 : page.value)
}

function handlePageChange(nextPage: number): void {
  void load(nextPage)
}

async function handleDelete(persona: Persona): Promise<void> {
  try {
    await ElMessageBox.confirm(
      `删除「${persona.name}」后，它的全部对话与记忆都会一并删除，且无法恢复。`,
      '确认删除这个人设吗？',
      {
        type: 'warning',
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        confirmButtonClass: 'el-button--danger',
      },
    )
  } catch {
    // ElMessageBox 用 reject 表示"用户没确认"（点取消或关闭），不是失败，直接返回
    return
  }

  deletingId.value = persona.id
  try {
    await deletePersona(persona.id)
    ElMessage.success('人设已删除')

    // 本页只剩这一条时，删完会停在超界页：后端返回 200 + 空列表 + 真实 total，
    // 页面就变成"分页器上还有数据、内容区却显示空态"的自相矛盾画面，所以主动回退一页
    const targetPage = personas.value.length === 1 && page.value > 1 ? page.value - 1 : page.value
    await load(targetPage)
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '删除失败，请稍后重试'))
    // 4043：这条已经不在自己名下（别处删了 / 换了账号），界面数据已过期，重拉
    if (isPersonaNotFound(error)) {
      await load(page.value)
    }
  } finally {
    deletingId.value = null
  }
}

onMounted(() => {
  void load(1)
})
</script>

<template>
  <div class="persona-page">
    <header class="page-header">
      <h2 class="page-title">人设管理</h2>
      <el-button type="primary" @click="openCreate">新建人设</el-button>
    </header>

    <el-skeleton v-if="loading" :rows="3" animated />

    <div v-else-if="errorMessage !== ''" class="page-error">
      <el-alert :title="errorMessage" type="error" show-icon :closable="false" />
      <el-button @click="load(page)">重试</el-button>
    </div>

    <el-empty v-else-if="personas.length === 0" description="还没有人设">
      <el-button type="primary" @click="openCreate">创建第一个 AI 伴侣</el-button>
    </el-empty>

    <template v-else>
      <div class="persona-grid">
        <el-card v-for="persona in personas" :key="persona.id" shadow="hover">
          <h3 class="persona-name">{{ persona.name }}</h3>
          <p class="persona-field persona-desc">{{ persona.personalityDesc }}</p>
          <p class="persona-field persona-style">{{ persona.speakingStyle }}</p>

          <div class="persona-meta">
            <span>亲密度 {{ persona.familiarity }}</span>
            <span>
              最近对话
              {{ persona.lastMessageAt === null ? '还没聊过' : formatDateTime(persona.lastMessageAt) }}
            </span>
            <span>创建于 {{ formatDateTime(persona.createdAt) }}</span>
          </div>

          <div class="persona-actions">
            <RouterLink :to="{ name: 'Chat', params: { personaId: persona.id } }">
              <el-button size="small">去对话</el-button>
            </RouterLink>
            <el-button size="small" @click="openEdit(persona)">编辑</el-button>
            <el-button
              size="small"
              type="danger"
              :loading="deletingId === persona.id"
              @click="handleDelete(persona)"
            >
              删除
            </el-button>
          </div>
        </el-card>
      </div>

      <el-pagination
        v-if="total > PERSONA_PAGE_SIZE"
        class="persona-pagination"
        layout="prev, pager, next"
        :total="total"
        :current-page="page"
        :page-size="PERSONA_PAGE_SIZE"
        @current-change="handlePageChange"
      />
    </template>

    <PersonaFormDialog v-model="dialogVisible" :persona="editingPersona" @saved="handleSaved" />
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 500;
}

.page-error {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.persona-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.persona-name {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 500;
}

/* 描述最长可到 2000 字符（后端上限），截断以免一张卡片把栅格撑成整屏 */
.persona-field {
  margin: 0 0 6px;
  overflow: hidden;
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  word-break: break-word;
}

.persona-desc {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.persona-style {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  color: var(--el-text-color-secondary);
}

.persona-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  margin: 12px 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.persona-actions {
  display: flex;
  gap: 8px;
}

.persona-pagination {
  justify-content: center;
  margin-top: 16px;
}
</style>
