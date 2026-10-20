import { ApiError, request } from '@/api/request'
import { ErrorCode } from '@/types/errcode'
import type { PageResult } from '@/types/api'
import type { ScheduleItem, ScheduleTriggerResult } from '@/types/schedule'

/**
 * 每页条数。契约默认 20、上限 100，显式传而不是靠后端默认值 ——
 * 「是否还有下一页」的判据必须和后端实际用的 pageSize 是同一个数。
 */
export const SCHEDULE_PAGE_SIZE = 20

/**
 * 某个人的日程列表。
 *
 * `personaId` 是必传的：日程和记忆一样是「一人设一份」，缺了它后端返回 4001
 * （参数层就拦下，此时还没有归属可供校验）。`user_id` 永远不传，从 Token 取。
 */
export function listSchedules(
  personaId: number,
  page = 1,
  pageSize = SCHEDULE_PAGE_SIZE,
): Promise<PageResult<ScheduleItem>> {
  return request.get<PageResult<ScheduleItem>>('/schedules', {
    params: { personaId, page, pageSize },
  })
}

/**
 * 取消日程。**软删除**：服务端把 status 置为 cancelled，不物理删行，
 * 所以返回值是 null，列表里那条仍然存在。重复删除同一条是幂等的。
 */
export function cancelSchedule(id: number): Promise<null> {
  return request.delete<null>(`/schedules/${id}`)
}

/** 4040：这条日程已不存在或不属于当前账号，界面数据已过期，需要重拉 */
export function isScheduleNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === ErrorCode.ErrNotFound
}

/**
 * 立即触发一条日程的提醒，不必等到 `remindAt`。
 *
 * 无请求体：`personaId` 由服务端从这条日程行上取，前端不传也不该传。
 *
 * **只有 `pending` 的行能触发** —— 服务端对 `cancelled`（软删除）和 `sent`
 * （已经提醒过）一律返回 4040，所以按钮只在待提醒的卡片上出现，不能做成通用按钮。
 *
 * 成功返回的是刚生成的助手回复；提醒本身是注入到对话里的一条消息，
 * 调用方应引导用户去对话页查看。
 */
export function triggerSchedule(id: number): Promise<ScheduleTriggerResult> {
  return request.post<ScheduleTriggerResult>(`/schedules/${id}/trigger`)
}
