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
