import { request } from '@/api/request'
import type { SettingsResponse, UpdateSettingsPayload } from '@/types/proactive'

/** 读取某人设的主动消息配置（契约 §9，`personaId` 必传） */
export function getSettings(personaId: number): Promise<SettingsResponse> {
  return request.get<SettingsResponse>('/proactive/settings', { params: { personaId } })
}

/** 更新配置：四个可改字段整体提交，响应是服务端规范化后的最新值 */
export function updateSettings(payload: UpdateSettingsPayload): Promise<SettingsResponse> {
  return request.put<SettingsResponse>('/proactive/settings', payload)
}
