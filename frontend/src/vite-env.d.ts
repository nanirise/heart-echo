/// <reference types="vite/client" />

/**
 * 本项目实际使用的环境变量声明。
 *
 * Vite 只会把 VITE_ 前缀的变量注入前端；没有在这里声明的变量
 * 会被推断成 any，与项目「禁用 any」的约定冲突，所以显式写出来。
 */
interface ImportMetaEnv {
  /** 后端接口基础地址，含 /api/v1 后缀 */
  readonly VITE_API_BASE_URL: string
  /** 是否启用 Mock 数据：字符串 'true' / 'false' */
  readonly VITE_USE_MOCK: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
