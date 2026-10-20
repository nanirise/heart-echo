/**
 * 日程实体 —— 对应 API_CONTRACT §10。
 *
 * 注意 status 的三个取值都是服务端状态，前端只读；
 * 「取消」是软删除，走 DELETE 之后行仍在列表里，状态变成 cancelled。
 */
export type ScheduleStatus = 'pending' | 'sent' | 'cancelled'

export interface ScheduleItem {
  id: number
  personaId: number
  /** 用户原话里的事件描述（"开会"），不是 AI 生成的提醒语 */
  content: string
  /** 绝对时间，RFC3339 带时区；由对话中的时间解析产生 */
  remindAt: string
  status: ScheduleStatus
  createdAt: string
}

/**
 * POST /schedules/:id/trigger 的响应 data —— 契约 §10，字段与 §9 的
 * `/proactive/trigger` 逐字相同。
 *
 * 这里拿到的是**刚生成的助手回复**，不是被注入的那行 `[nudge]` 提醒本身；
 * 提醒会以一条消息的形式落到对话里，所以触发之后要提示用户去对话页看。
 *
 * `createdAt` 允许为 null：Go 侧是指针，零值序列化成 null，
 * 不是 `0001-01-01`（§9 的类型说明）。
 */
export interface ScheduleTriggerResult {
  messageId: number
  content: string
  createdAt: string | null
}
