import { ApiError, request } from '@/api/request'
import { ErrorCode } from '@/types/errcode'
import type { PageResult } from '@/types/api'
import type {
  CommentItem,
  CreateCommentRequest,
  LikeResponse,
  MomentItem,
} from '@/types/moment'

/**
 * 每页条数。契约默认 20、上限 100，显式传而不是靠后端默认值 ——
 * 「是否还有下一页」的判据必须和后端实际用的 pageSize 是同一个数。
 */
export const MOMENT_PAGE_SIZE = 20

/** 评论区展开时的每页条数 */
export const COMMENT_PAGE_SIZE = 20

/** 动态流。没有 personaId 过滤参数，返回当前账号全部人设的动态（契约 §8） */
export function getMoments(
  page = 1,
  pageSize = MOMENT_PAGE_SIZE,
): Promise<PageResult<MomentItem>> {
  return request.get<PageResult<MomentItem>>('/moments', { params: { page, pageSize } })
}

/** 点赞（幂等）。重复点击不会累加，返回最新的 likeCount / liked */
export function likeMoment(id: number): Promise<LikeResponse> {
  return request.post<LikeResponse>(`/moments/${id}/like`)
}

/** 某条动态的评论列表；动态不属于当前账号时后端返回 4040，不是空列表 */
export function getComments(
  id: number,
  page = 1,
  pageSize = COMMENT_PAGE_SIZE,
): Promise<PageResult<CommentItem>> {
  return request.get<PageResult<CommentItem>>(`/moments/${id}/comments`, {
    params: { page, pageSize },
  })
}

/** 以当前登录用户身份发表评论（AI 评论由后端定时任务写库，不走这里） */
export function createComment(id: number, content: string): Promise<CommentItem> {
  const payload: CreateCommentRequest = { content }
  return request.post<CommentItem>(`/moments/${id}/comments`, payload)
}

/** 4040：这条动态已不存在或不属于当前账号，页面数据已过期，需要重拉 */
export function isMomentNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.code === ErrorCode.ErrNotFound
}
