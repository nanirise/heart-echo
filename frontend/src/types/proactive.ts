/**
 * 主动消息（Proactive）—— 与契约 §9 的响应结构逐字对齐，字段名 camelCase。
 *
 * 配置跟记忆、日程一样是「一人设一份」：`personaId` 必传，缺失时后端在参数层
 * 就返回 `4001`（契约 §9）。`user_id` 永远不从请求体传，后端从 Token 取。
 */

/** 配置实体，对应 `GET /proactive/settings` 与 `PUT /proactive/settings` 的响应 data */
export interface SettingsResponse {
  personaId: number
  enabled: boolean
  /** 空闲判定随机阈值的下界；契约约束 1-1440 且 < intervalMax */
  intervalMin: number
  /** 空闲判定随机阈值的上界；契约约束 1-1440 且 > intervalMin */
  intervalMax: number
  /** 每日主动消息上限；契约约束 1-10，与日程提醒共用这一份计数 */
  dailyLimit: number
  /** 最近一次主动消息时间；**只读**，`PUT` 忽略它，唯一写入方是后端的触发链路 */
  lastNudgeAt: string | null
}

/**
 * 更新配置的请求体。
 *
 * 四个可改字段全部必传（整体替换，不是部分更新）—— 契约 §9 的 `PUT` 只有
 * 「设定值」一种语义。`lastNudgeAt` 刻意不在这里：它是只读记录列。
 */
export interface UpdateSettingsPayload {
  personaId: number
  enabled: boolean
  intervalMin: number
  intervalMax: number
  dailyLimit: number
}
