/**
 * 画像响应 —— 对应 API_CONTRACT §7 `GET /profile/portrait`。
 *
 * `profileData` 的键集合**不由前端定义**：契约 §2.2 明确把「键集合定义」列为不做，
 * 键由 AI 提取链路决定，Go 侧只做 JSONB 透传。所以这里只能是 `Record<string, unknown>`，
 * 页面按 entries 泛化渲染 —— 定死 Occupation / Interests 反而会让未知键静默丢失。
 */
export interface PortraitResponse {
  personaId: number
  profileData: Record<string, unknown>
  /**
   * 契约 §7：尚未生成画像时为 `null`（不是空串，也不是「此刻」）。
   *
   * 渲染前必须挡掉 —— `new Date(null)` 会被当成 epoch，格式化成 1970 而不是兜底符。
   */
  updatedAt: string | null
}
