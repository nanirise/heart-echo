<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import { toErrorMessage } from '@/api/error'
import { PERSONA_PAGE_SIZE, listPersonas } from '@/api/persona'
import {
  SCHEDULE_PAGE_SIZE,
  cancelSchedule,
  isScheduleNotFound,
  listSchedules,
} from '@/api/schedule'
import type { Persona } from '@/types/persona'
import type { ScheduleItem, ScheduleStatus } from '@/types/schedule'
import { formatDateTime } from '@/utils/datetime'

const router = useRouter()

const personas = ref<Persona[]>([])
const selectedPersonaId = ref<number | null>(null)

const schedules = ref<ScheduleItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const loadingMore = ref(false)
const errorMessage = ref('')

/** 取消请求在飞的日程 id：防止连点重复发 DELETE（后端幂等，但按钮应即时禁用） */
const cancellingIds = reactive(new Set<number>())

const hasMore = computed(() => schedules.value.length < total.value)

const STATUS_TEXT: Record<ScheduleStatus, string> = {
  pending: '待提醒',
  sent: '已提醒',
  cancelled: '已取消',
}

const STATUS_TYPE: Record<ScheduleStatus, 'warning' | 'success' | 'info'> = {
  pending: 'warning',
  sent: 'success',
  cancelled: 'info',
}

async function loadSchedules(personaId: number): Promise<void> {
  loading.value = true
  errorMessage.value = ''

  try {
    const result = await listSchedules(personaId, 1, SCHEDULE_PAGE_SIZE)
    schedules.value = result.list
    total.value = result.total
    // 用后端收敛后的页码：契约规定非法 page / pageSize 只夹到合法范围、不报错
    page.value = result.page
  } catch (error) {
    // 换人设失败时不能留着上一位的日程，否则会「标题是 B、列表是 A」
    schedules.value = []
    total.value = 0
    errorMessage.value = toErrorMessage(error, '日程加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

async function loadPage(): Promise<void> {
  loading.value = true
  errorMessage.value = ''

  try {
    const result = await listPersonas(1, PERSONA_PAGE_SIZE)
    personas.value = result.list

    // 保留当前选择（重试场景）；列表里没有它了（别处删了 / 换了账号）就落回第一个
    const target =
      personas.value.find((item) => item.id === selectedPersonaId.value) ?? personas.value[0]

    if (target === undefined) {
      schedules.value = []
      total.value = 0
      return
    }

    selectedPersonaId.value = target.id
    await loadSchedules(target.id)
  } catch (error) {
    errorMessage.value = toErrorMessage(error, '加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

async function handlePersonaChange(personaId: number): Promise<void> {
  selectedPersonaId.value = personaId
  await loadSchedules(personaId)
}

async function loadMore(): Promise<void> {
  const personaId = selectedPersonaId.value
  if (personaId === null || loadingMore.value || !hasMore.value) return

  loadingMore.value = true
  try {
    const result = await listSchedules(personaId, page.value + 1, SCHEDULE_PAGE_SIZE)
    schedules.value.push(...result.list)
    total.value = result.total
    page.value = result.page
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '加载更多失败，请稍后重试'))
  } finally {
    loadingMore.value = false
  }
}

async function handleCancel(item: ScheduleItem): Promise<void> {
  if (cancellingIds.has(item.id)) return

  try {
    await ElMessageBox.confirm(`取消后不会再收到「${item.content}」的提醒。`, '取消这条日程', {
      confirmButtonText: '取消日程',
      cancelButtonText: '再想想',
      type: 'warning',
    })
  } catch {
    // 用户点了「再想想」：ElMessageBox 用 reject 表达取消，这里不是错误
    return
  }

  cancellingIds.add(item.id)
  try {
    await cancelSchedule(item.id)
    // 软删除：服务端只把 status 置为 cancelled，行还在列表里，所以原地改状态而不是移除
    item.status = 'cancelled'
    ElMessage.success('已取消')
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '取消失败，请稍后重试'))
    // 4040：这条日程已不存在或不属于当前账号，界面数据已过期
    if (isScheduleNotFound(error) && selectedPersonaId.value !== null) {
      await loadSchedules(selectedPersonaId.value)
    }
  } finally {
    cancellingIds.delete(item.id)
  }
}

onMounted(() => {
  void loadPage()
})
</script>

<template>
  <div class="schedule-page">
    <header class="page-header">
      <div>
        <h2 class="page-title">日程提醒</h2>
        <p class="page-desc">
          日程只能从对话里产生 —— 说一句「明天下午三点提醒我开会」，AI 会自动记下来。
        </p>
      </div>

      <!-- 一人设一份：切到谁就看谁的日程 -->
      <el-select
        v-if="personas.length > 0"
        v-model="selectedPersonaId"
        class="persona-select"
        :disabled="loading"
        @change="handlePersonaChange"
      >
        <el-option
          v-for="persona in personas"
          :key="persona.id"
          :label="persona.name"
          :value="persona.id"
        />
      </el-select>
    </header>

    <el-card v-if="loading" shadow="never">
      <el-skeleton :rows="4" animated />
    </el-card>

    <div v-else-if="errorMessage !== ''" class="page-error">
      <el-alert :title="errorMessage" type="error" show-icon :closable="false" />
      <el-button type="primary" @click="loadPage">重试</el-button>
    </div>

    <el-empty v-else-if="personas.length === 0" description="还没有人设" :image-size="80">
      <p class="state-hint">日程是按人设存的，先创建一个伴侣再来这里看。</p>
      <el-button type="primary" @click="router.push({ name: 'Persona' })">去人设管理</el-button>
    </el-empty>

    <el-empty v-else-if="schedules.length === 0" description="还没有日程" :image-size="80">
      <p class="state-hint">
        在对话里告诉 TA 时间和事情，比如「明天下午三点提醒我开会」，TA 会帮你记下来。
      </p>
      <el-button type="primary" @click="router.push({ name: 'Chat' })">去对话</el-button>
    </el-empty>

    <template v-else>
      <ul class="schedule-list">
        <li v-for="item in schedules" :key="item.id">
          <el-card shadow="never" class="schedule-card">
            <div class="schedule-head">
              <span class="schedule-time">{{ formatDateTime(item.remindAt) }}</span>
              <el-tag :type="STATUS_TYPE[item.status]" size="small" disable-transitions>
                {{ STATUS_TEXT[item.status] }}
              </el-tag>
            </div>

            <p class="schedule-content" :class="{ 'schedule-content-done': item.status !== 'pending' }">
              {{ item.content }}
            </p>

            <div class="schedule-actions">
              <span class="schedule-created">创建于 {{ formatDateTime(item.createdAt) }}</span>
              <el-button
                v-if="item.status === 'pending'"
                size="small"
                text
                type="danger"
                :loading="cancellingIds.has(item.id)"
                @click="handleCancel(item)"
              >
                取消
              </el-button>
            </div>
          </el-card>
        </li>
      </ul>

      <div class="load-more">
        <el-button v-if="hasMore" :loading="loadingMore" @click="loadMore">加载更多</el-button>
        <span v-else class="list-end">没有更多了</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.schedule-page {
  max-width: var(--app-width-prose);
  margin: 0 auto;
}

.page-header {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: var(--app-gap);
}

.page-desc {
  margin: var(--spacing-unit) 0 0;
  font-size: var(--el-font-size-small);
  color: var(--color-text-secondary);
}

.persona-select {
  width: 200px;
}

.page-error {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.state-hint {
  margin: 0 0 12px;
  font-size: var(--el-font-size-small);
  color: var(--color-text-secondary);
}

.schedule-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.schedule-head {
  display: flex;
  gap: var(--spacing-unit);
  align-items: center;
  justify-content: space-between;
}

.schedule-time {
  font-size: var(--el-font-size-base);
  font-weight: 500;
  color: var(--color-text-main);
}

.schedule-content {
  margin: 12px 0;
  font-size: var(--el-font-size-base);
  line-height: 1.6;
  color: var(--color-text-main);
  white-space: pre-wrap;
  word-break: break-word;
}

/* 已提醒 / 已取消用次级色，让待办项在列表里更跳 */
.schedule-content-done {
  color: var(--color-text-secondary);
}

.schedule-actions {
  display: flex;
  gap: var(--spacing-unit);
  align-items: center;
  justify-content: space-between;
  padding-top: var(--spacing-unit);
  border-top: 1px solid var(--el-border-color-lighter);
}

.schedule-created {
  font-size: var(--el-font-size-extra-small);
  color: var(--color-text-placeholder);
}

.load-more {
  display: flex;
  justify-content: center;
  margin-top: var(--app-gap);
}

.list-end {
  font-size: var(--el-font-size-extra-small);
  color: var(--color-text-placeholder);
}
</style>
