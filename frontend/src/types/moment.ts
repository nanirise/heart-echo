/**
 * 朋友圈（Moments）—— 与契约 §8 / 后端 `dto.MomentItem`、`dto.CommentItem` 逐字对齐。
 *
 * 只有 AI 人设能发动态，所以没有「创建动态」的请求类型；
 * 用户侧能写的只有评论（`CreateCommentRequest`）。
 */

/** 动态实体。注意：没有 emotionLabel —— 情绪是内部信号，后端不下发 */
export interface MomentItem {
  id: number
  personaId: number
  personaName: string
  content: string
  /** 该动态的点赞总数 */
  likeCount: number
  /** 当前登录用户是否已点赞（决定按钮高亮） */
  liked: boolean
  /** 派生字段：由 moment_comments 聚合，不是表上的列 */
  commentCount: number
  createdAt: string
}

/** 评论实体。personaId 与 userId 恰好一个非空：AI 评论前者有值，用户评论后者有值 */
export interface CommentItem {
  id: number
  momentId: number
  personaId: number | null
  userId: number | null
  /** 展示用作者名，前端直接用，不要按 personaId / userId 自己推断 */
  authorName: string
  content: string
  createdAt: string
}

/** 点赞响应。本期不支持取消点赞，liked 恒为 true，但以响应为准、不硬编码 */
export interface LikeResponse {
  likeCount: number
  liked: boolean
}

/** 发表评论请求体 */
export interface CreateCommentRequest {
  content: string
}
