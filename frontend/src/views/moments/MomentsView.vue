<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

import { toErrorMessage } from '@/api/error'
import {
  COMMENT_PAGE_SIZE,
  MOMENT_PAGE_SIZE,
  createComment,
  getComments,
  getMoments,
  isMomentNotFound,
  likeMoment,
} from '@/api/moment'
import type { CommentItem, MomentItem } from '@/types/moment'

/** 单条动态的评论区本地状态；首次展开时才创建并拉取 */
interface CommentState {
  open: boolean
  loaded: boolean
  loading: boolean
  loadingMore: boolean
  errorMessage: string
  list: CommentItem[]
  total: number
  page: number
  draft: string
  sending: boolean
}

const moments = ref<MomentItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(true)
const loadingMore = ref(false)
const errorMessage = ref('')

/** 点赞请求在飞的动态 id：防止连点重复发请求（后端虽幂等，按钮也应即时禁用） */
const likingIds = reactive(new Set<number>())

/** 评论状态按动态 id 隔离：展开一条不影响另一条 */
const commentStates = reactive<Record<number, CommentState | undefined>>({})

const hasMore = computed(() => moments.value.length < total.value)

/**
 * 模板遍历用的视图模型：把动态与它的评论状态配成一对，
 * 避免在模板里反复索引 commentStates 并处理 undefined。
 */
const momentViews = computed(() =>
  moments.value.map((moment) => ({
    moment,
    comments: commentStates[moment.id] ?? null,
  })),
)

/** 契约时间是 RFC3339（可能带 6 位小数），交给 Date 解析，不按字符串切片 */
function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  const pad = (part: number): string => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`
}

/** 头像色块：按名字哈希到固定色相，同一个人设每次进页面颜色一致 */
function avatarColor(name: string): string {
  let hash = 0
  for (const char of name) {
    hash = (hash * 31 + (char.codePointAt(0) ?? 0)) % 360
  }
  return `hsl(${hash}, 58%, 52%)`
}

/** 取首个码点而不是 name[0]，保证 emoji 名字不会被切成半个字符 */
function avatarText(name: string): string {
  return Array.from(name)[0] ?? '?'
}

async function loadFirstPage(): Promise<void> {
  loading.value = true
  errorMessage.value = ''

  try {
    const result = await getMoments(1, MOMENT_PAGE_SIZE)
    moments.value = result.list
    total.value = result.total
    // 用后端收敛后的页码：契约规定非法 page / pageSize 只夹到合法范围、不报错
    page.value = result.page
  } catch (error) {
    moments.value = []
    total.value = 0
    errorMessage.value = toErrorMessage(error, '朋友圈加载失败，请稍后重试')
  } finally {
    loading.value = false
  }
}

async function loadMore(): Promise<void> {
  if (loadingMore.value || !hasMore.value) return

  loadingMore.value = true
  try {
    const result = await getMoments(page.value + 1, MOMENT_PAGE_SIZE)
    moments.value.push(...result.list)
    total.value = result.total
    page.value = result.page
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '加载更多失败，请稍后重试'))
  } finally {
    loadingMore.value = false
  }
}

/** 点赞成功后原地更新，不重拉整个列表（契约返回的就是最新 likeCount / liked） */
async function handleLike(moment: MomentItem): Promise<void> {
  if (likingIds.has(moment.id)) return
  likingIds.add(moment.id)

  try {
    const result = await likeMoment(moment.id)
    moment.likeCount = result.likeCount
    moment.liked = result.liked
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '点赞失败，请稍后重试'))
    // 4040：动态已不存在或不再属于当前账号，界面数据已过期
    if (isMomentNotFound(error)) {
      await loadFirstPage()
    }
  } finally {
    likingIds.delete(moment.id)
  }
}

function ensureCommentState(momentID: number): CommentState {
  const existing = commentStates[momentID]
  if (existing !== undefined) return existing

  const state: CommentState = {
    open: false,
    loaded: false,
    loading: false,
    loadingMore: false,
    errorMessage: '',
    list: [],
    total: 0,
    page: 1,
    draft: '',
    sending: false,
  }
  commentStates[momentID] = state
  // 必须把 reactive 的代理返回给调用方：直接改原始对象会绕过 set 陷阱，界面不刷新
  return commentStates[momentID] ?? state
}

function toggleComments(moment: MomentItem): void {
  const state = ensureCommentState(moment.id)
  state.open = !state.open

  // 懒加载：只有第一次展开才请求；收起再展开直接复用已加载的列表
  if (state.open && !state.loaded && !state.loading) {
    void loadComments(moment)
  }
}

async function loadComments(moment: MomentItem): Promise<void> {
  const state = ensureCommentState(moment.id)
  state.loading = true
  state.errorMessage = ''

  try {
    const result = await getComments(moment.id, 1, COMMENT_PAGE_SIZE)
    state.list = result.list
    state.total = result.total
    state.page = result.page
    state.loaded = true
    // 动态流里的 commentCount 是聚合派生值，以评论接口的 total 为准
    moment.commentCount = result.total
  } catch (error) {
    state.errorMessage = toErrorMessage(error, '评论加载失败，请稍后重试')
  } finally {
    state.loading = false
  }
}

async function loadMoreComments(moment: MomentItem): Promise<void> {
  const state = ensureCommentState(moment.id)
  if (state.loadingMore || state.list.length >= state.total) return

  state.loadingMore = true
  try {
    const result = await getComments(moment.id, state.page + 1, COMMENT_PAGE_SIZE)
    state.list.push(...result.list)
    state.total = result.total
    state.page = result.page
    moment.commentCount = result.total
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '加载更多评论失败，请稍后重试'))
  } finally {
    state.loadingMore = false
  }
}

async function handleSendComment(moment: MomentItem): Promise<void> {
  const state = ensureCommentState(moment.id)
  const content = state.draft.trim()

  // 空字符串不发送（后端也会拦 4001，前端先给出提示）
  if (content === '') {
    ElMessage.warning('评论内容不能为空')
    return
  }
  if (state.sending) return

  state.sending = true
  try {
    const comment = await createComment(moment.id, content)
    // 评论尚未加载成功时，不能拿本地 0 覆盖动态流里的真实计数
    const baseTotal = state.loaded ? state.total : moment.commentCount
    state.list.push(comment)
    state.total = baseTotal + 1
    moment.commentCount = state.total
    state.draft = ''
    ElMessage.success('评论已发送')
  } catch (error) {
    ElMessage.error(toErrorMessage(error, '评论发送失败，请稍后重试'))
    if (isMomentNotFound(error)) {
      await loadFirstPage()
    }
  } finally {
    state.sending = false
  }
}

/** 静默刷新在飞的标记：visibilitychange 与 focus 会同时触发，靠它去重 */
const refreshing = ref(false)

/**
 * 回前台刷新。
 *
 * 朋友圈的动态只由后台定时任务产出（契约 §8 没有手动触发接口），
 * 但这个页面只在 onMounted 拉一次 —— 用户切到别的标签页待一会儿再回来，
 * 看到的还是离开前那一份，必须先刷新整页才能看到新动态。
 */
async function refreshSilently(): Promise<void> {
  // 初次加载正在进行时交给 loadFirstPage，不并发拉两份
  if (refreshing.value || loading.value) return

  // 去重必须覆盖两个分支：从别的应用切回来时 visibilitychange 与 focus 会各触发一次，
  // 只在整份替换那支加锁的话，已翻页的用户会被并发打两次同一个请求
  refreshing.value = true
  try {
    const result = await getMoments(1, MOMENT_PAGE_SIZE)
    total.value = result.total

    // 已经翻过页就不能整份替换：会把人正看的第 N 页冲成第 1 页、滚动位置跳回顶部。
    // 此时只更新总数，让「加载更多」拿到新的 total 即可。
    if (page.value > 1) return

    moments.value = result.list
    page.value = result.page
  } catch {
    // 静默刷新失败不弹提示：页面已有可用数据，下次回前台再试
  } finally {
    refreshing.value = false
  }
}

function handleForeground(): void {
  // 切到后台时 visibilitychange 也会触发，那一次不需要刷新
  if (document.visibilityState !== 'visible') return
  void refreshSilently()
}

onMounted(() => {
  void loadFirstPage()
  document.addEventListener('visibilitychange', handleForeground)
  window.addEventListener('focus', handleForeground)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleForeground)
  window.removeEventListener('focus', handleForeground)
})
</script>

<template>
  <div class="moments-page">
    <header class="page-header">
      <h2 class="page-title">朋友圈</h2>
    </header>

    <el-skeleton v-if="loading" :rows="4" animated />

    <div v-else-if="errorMessage !== ''" class="page-error">
      <el-alert :title="errorMessage" type="error" show-icon :closable="false" />
      <el-button @click="loadFirstPage">重试</el-button>
    </div>

    <el-empty v-else-if="moments.length === 0" description="还没有动态">
      <p class="empty-hint">AI 伴侣会定时发布动态，稍后再来看看。</p>
      <RouterLink :to="{ name: 'Persona' }">
        <el-button>去人设管理</el-button>
      </RouterLink>
    </el-empty>

    <template v-else>
      <div class="moment-list">
        <el-card
          v-for="view in momentViews"
          :key="view.moment.id"
          class="moment-card"
          shadow="never"
        >
          <div class="moment-head">
            <span
              class="avatar"
              :style="{ backgroundColor: avatarColor(view.moment.personaName) }"
            >
              {{ avatarText(view.moment.personaName) }}
            </span>
            <div class="head-meta">
              <span class="persona-name">{{ view.moment.personaName }}</span>
              <span class="moment-time">{{ formatDateTime(view.moment.createdAt) }}</span>
            </div>
          </div>

          <p class="moment-content">{{ view.moment.content }}</p>

          <div class="moment-actions">
            <button
              type="button"
              class="action-btn"
              :class="{ 'action-btn-liked': view.moment.liked }"
              :disabled="likingIds.has(view.moment.id)"
              @click="handleLike(view.moment)"
            >
              <svg class="action-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 20.7l-1.1-1C5.3 14.6 2 11.6 2 8.1 2 5.3 4.2 3 7 3c1.6 0 3.1.7 4.2 1.9L12 6l.8-.9C13.9 3.7 15.4 3 17 3c2.8 0 5 2.3 5 5.1 0 3.5-3.3 6.5-8.9 11.6l-1.1 1z"
                  :fill="view.moment.liked ? 'currentColor' : 'none'"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linejoin="round"
                />
              </svg>
              <span>{{ view.moment.likeCount }}</span>
            </button>

            <button type="button" class="action-btn" @click="toggleComments(view.moment)">
              <svg class="action-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M21 11.5a8.4 8.4 0 0 1-8.5 8.3H8.2L3 22l1.1-4.7A8.3 8.3 0 0 1 2 11.5C2 6.8 6.2 3 11.5 3S21 6.8 21 11.5z"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.6"
                  stroke-linejoin="round"
                />
              </svg>
              <span>{{ view.moment.commentCount }}</span>
            </button>
          </div>

          <section
            v-if="view.comments !== null && view.comments.open"
            class="comment-section"
          >
            <p v-if="view.comments.loading" class="comment-hint">评论加载中…</p>

            <div v-else-if="view.comments.errorMessage !== ''" class="comment-error">
              <span class="comment-error-text">{{ view.comments.errorMessage }}</span>
              <el-button size="small" @click="loadComments(view.moment)">重试</el-button>
            </div>

            <template v-else>
              <p v-if="view.comments.list.length === 0" class="comment-empty">
                还没有评论，来说点什么吧
              </p>
              <ul v-else class="comment-list">
                <li v-for="comment in view.comments.list" :key="comment.id" class="comment-item">
                  <div class="comment-line">
                    <span class="comment-author">{{ comment.authorName }}</span>
                    <span class="comment-content">{{ comment.content }}</span>
                  </div>
                  <span class="comment-time">{{ formatDateTime(comment.createdAt) }}</span>
                </li>
              </ul>

              <el-button
                v-if="view.comments.list.length < view.comments.total"
                class="comment-more"
                size="small"
                text
                :loading="view.comments.loadingMore"
                @click="loadMoreComments(view.moment)"
              >
                加载更多评论
              </el-button>
            </template>

            <div class="comment-form">
              <el-input
                v-model="view.comments.draft"
                size="small"
                placeholder="写点什么…"
                :disabled="view.comments.sending"
                @keyup.enter="handleSendComment(view.moment)"
              />
              <el-button
                type="primary"
                size="small"
                :loading="view.comments.sending"
                @click="handleSendComment(view.moment)"
              >
                发送
              </el-button>
            </div>
          </section>
        </el-card>
      </div>

      <div class="load-more">
        <el-button v-if="hasMore" :loading="loadingMore" @click="loadMore">加载更多</el-button>
        <span v-else class="list-end">没有更多了</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
.moments-page {
  max-width: var(--app-width-prose);
  margin: 0 auto;
}

.page-header {
  margin-bottom: var(--app-gap);
}

.page-error {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.empty-hint {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.moment-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.moment-head {
  display: flex;
  gap: 10px;
  align-items: center;
}

.avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  color: var(--el-color-white);
  font-size: 16px;
  font-weight: 500;
}

.head-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.persona-name {
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.moment-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.moment-content {
  margin: 12px 0;
  font-size: 14px;
  line-height: 1.7;
  color: var(--el-text-color-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.moment-actions {
  display: flex;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.action-btn {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  padding: 4px 10px;
  border: none;
  border-radius: var(--el-border-radius-round);
  background: transparent;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  cursor: pointer;
  transition:
    background-color 0.15s,
    color 0.15s;
}

.action-btn:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.action-btn:disabled {
  cursor: default;
  opacity: 0.6;
}

.action-btn-liked,
.action-btn-liked:hover {
  color: var(--el-color-danger);
}

.action-icon {
  width: 16px;
  height: 16px;
}

.comment-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.comment-hint,
.comment-empty {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.comment-error {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.comment-error-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--el-color-danger);
}

.comment-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0 0 8px;
  padding: 0;
  list-style: none;
}

.comment-item {
  display: flex;
  gap: 8px;
  align-items: baseline;
  justify-content: space-between;
}

.comment-line {
  min-width: 0;
  font-size: 13px;
  line-height: 1.6;
}

.comment-author {
  margin-right: 6px;
  font-weight: 500;
  color: var(--el-text-color-regular);
}

.comment-content {
  color: var(--el-text-color-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-time {
  flex-shrink: 0;
  font-size: var(--el-font-size-extra-small);
  color: var(--el-text-color-placeholder);
}

.comment-more {
  margin-bottom: 4px;
}

.comment-form {
  display: flex;
  gap: 8px;
  align-items: center;
}

.load-more {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}

.list-end {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
</style>
