import { request } from '@/api/request'
import type { PortraitResponse } from '@/types/profile'

/**
 * 某个人设的画像。
 *
 * `personaId` 必传（契约 §7）：画像是一人设一份，缺了它后端返回 4001。
 * `user_id` 永远不传 —— 从 Token 取。
 */
export function getPortrait(personaId: number): Promise<PortraitResponse> {
  return request.get<PortraitResponse>('/profile/portrait', { params: { personaId } })
}
