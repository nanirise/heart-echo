import { ApiError, NETWORK_ERROR_CODE } from '@/api/request'

/**
 * 把请求失败统一转成可展示的中文文案。
 * 后端「一 code 一 msg」，message 就是权威文案；
 * 只有请求根本没到后端时（code === -1）才用调用方给的兜底文案。
 */
export function toErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.code !== NETWORK_ERROR_CODE) {
    return error.message
  }
  return fallback
}