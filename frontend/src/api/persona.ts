import { ApiError, NETWORK_ERROR_CODE, request } from '@/api/request'
import type { PageResult } from '@/types/api'
import { ErrorCode } from '@/types/errcode'
import type { Persona, PersonaPayload } from '@/types/persona'

/**
 * 每页条数。契约默认 20、上限 100，这里显式传而不是靠后端默认值 ——
 * 分页器的 `total > PAGE_SIZE` 判据必须和后端实际用的 pageSize 是同一个数。
 */
export const PERSONA_PAGE_SIZE = 20

/** 人设列表（同时就是对话列表）。排序权在后端，前端不要再排一遍 */
export function listPersonas(
  page = 1,
  pageSize = PERSONA_PAGE_SIZE,
): Promise<PageResult<Persona>> {
  return request.get<PageResult<Persona>>('/personas', { params: { page, pageSize } })
}

/** 新建人设（后端会在同一事务里播种主动消息配置，前端不参与） */
export function createPersona(payload: PersonaPayload): Promise<Persona> {
  return request.post<Persona>('/personas', payload)
}

/** 编辑人设：三个字段整体替换，`state` / `familiarity` 不进请求体（后端也忽略） */
export function updatePersona(id: number, payload: PersonaPayload): Promise<Persona> {
  return request.put<Persona>(`/personas/${id}`, payload)
}

/** 删除人设：硬删，消息 / 记忆 / 画像 / 配置由数据库外键级联清理 */
export function deletePersona(id: number): Promise<null> {
  return request.delete<null>(`/personas/${id}`)
}

/**
 * 把请求失败转成一句可展示的文案。
 * 后端「一 code 一 msg」，message 就是权威文案；只有请求根本没到后端时（-1）才用兜底 ——
 * 那时 message 是 axios 的英文原文（`Network Error` / `timeout of 15000ms exceeded`）。
 */
export function toErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.code !== NETWORK_ERROR_CODE) {
    return error.message
  }
  return fallback
}

/**
 * `4043` 的唯一含义：这条人设现在不属于你，或者已经不存在（另一标签页删掉了 / 换了账号）。
 * 页面据此判断「界面数据已过期，需要重拉」，而不是把它当普通错误弹一下就完。
 */
export function isPersonaNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === ErrorCode.ErrPersonaNotFound
}
