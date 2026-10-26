<script setup lang="ts">
import { computed } from 'vue'
import type { CSSProperties } from 'vue'

import { avatarColorOf, avatarInitialOf } from '@/utils/avatar'

/**
 * 统一的头像展示：有地址就显示图片，没有（或加载失败）就显示名字首字 + 底色。
 *
 * 「加载失败也回落」不是自己实现的：el-avatar 内部用 hasLoadError 标记，
 * img 的 onError 置位后改渲染默认插槽，并且会 watch src 重置标记。
 * 在这里再写一遍只会多出一份会和上游行为对不上的逻辑。
 *
 * 用户头像与人设头像共用这个组件，两边兜底表现才一致。
 */
const props = withDefaults(
  defineProps<{
    /** 图片地址；null / undefined / 空串都表示没有头像 */
    src?: string | null
    /** 取首字与底色都靠它，所以换名字等于换兜底样式 */
    name?: string | null
    size?: number
  }>(),
  {
    src: null,
    name: null,
    size: 40,
  },
)

/**
 * 空串要归一成 undefined。只判 null 是不够的 —— 输入框清空、后端存了空白串，
 * 都会让 el-avatar 拿一个空地址去发请求，然后失败一次才回落，白闪一下。
 */
const normalizedSrc = computed<string | undefined>(() => {
  const raw = props.src

  if (raw === null || raw === undefined) return undefined

  const trimmed = raw.trim()

  return trimmed === '' ? undefined : trimmed
})

/**
 * 走内联 style 而不是 class：这两个值随 props 变化，class 表达不了。
 * `--el-avatar-bg-color` 是 el-avatar 自己的变量，内联声明优先级最高，
 * 不会被组件库里的默认值盖掉（祖先元素上声明才会被盖）。
 */
const badgeStyle = computed<CSSProperties>(() => ({
  '--el-avatar-bg-color': avatarColorOf(props.name),
  fontSize: `${Math.round(props.size * 0.42)}px`,
}))
</script>

<template>
  <el-avatar class="avatar-badge" :size="size" :src="normalizedSrc" :style="badgeStyle">
    {{ avatarInitialOf(name) }}
  </el-avatar>
</template>

<style scoped>
/* 预设图是正方形，圆形容器里居中裁切，别让非正方形的图被拉变形 */
.avatar-badge :deep(img) {
  object-fit: cover;
}

.avatar-badge {
  font-weight: 500;
  color: #ffffff;
  flex-shrink: 0;
}
</style>
