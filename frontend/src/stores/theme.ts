import { defineStore } from 'pinia'

/** localStorage 键；index.html 的预置脚本也读同一个键，改名必须两处同步 */
const THEME_STORAGE_KEY = 'heart-echo-theme'

/**
 * 合法主题的完整列表，cycleTheme 按这个顺序循环。
 * 新增主题时在这里加一项，并同步 THEME_CLASS 与 index.html 的首屏映射。
 */
export const THEMES = ['light', 'dark', 'pain', 'custom-zhang'] as const
export type Theme = (typeof THEMES)[number]

/** 每个主题挂到 <html> 上的类名；light 用 :root 默认值，不需要额外类名 */
const THEME_CLASS: Record<Theme, string | null> = {
  light: null,
  dark: 'dark',
  pain: 'theme-pain',
  'custom-zhang': 'theme-zhang',
}

function isTheme(value: string | null): value is Theme {
  return value !== null && (THEMES as readonly string[]).includes(value)
}

function readStoredTheme(): Theme {
  const stored = localStorage.getItem(THEME_STORAGE_KEY)

  return isTheme(stored) ? stored : 'light'
}

/**
 * 把主题落到 DOM。
 *
 * 先清掉所有 `theme-*` 与 `dark` 再挂新类，避免切换时残留：
 * 比如 dark → pain 若只加不减，`html.dark.theme-pain` 会让两套变量同时命中。
 */
export function applyTheme(theme: Theme): void {
  const root = document.documentElement

  // classList 是实时集合，边遍历边删会跳项，先快照成数组
  for (const className of Array.from(root.classList)) {
    if (className === 'dark' || className.startsWith('theme-')) {
      root.classList.remove(className)
    }
  }

  const className = THEME_CLASS[theme]

  if (className !== null) {
    root.classList.add(className)
  }
}

/**
 * 主题 store —— 只保存「用户选了什么」。
 *
 * 值的来源只有 localStorage：刷新后恢复上次选择，没有选择或值非法时回到 light。
 * DOM 同步由 cycleTheme 与 App.vue 的 watch 统一走 `applyTheme()`，
 * 不在这里维护第二份「当前 class 是什么」的状态。
 */
export const useThemeStore = defineStore('theme', {
  state: () => ({
    theme: readStoredTheme(),
  }),

  actions: {
    /** 按 THEMES 的顺序循环到下一套主题，并持久化选择 */
    cycleTheme(): void {
      const index = THEMES.indexOf(this.theme)
      this.theme = THEMES[(index + 1) % THEMES.length]
      localStorage.setItem(THEME_STORAGE_KEY, this.theme)
      applyTheme(this.theme)
    },
  },
})
