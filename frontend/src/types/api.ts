/**
 * 前后端数据契约 —— 与后端 `backend/pkg/response/response.go` 一一对应。
 *
 * 本文件是所有接口响应的类型源头，请求层与业务层统一从这里 import，
 * 不允许在别处重复定义响应体结构。
 */

/** 后端统一响应体，对应 Go 侧 `Response[T]` */
export interface ApiResponse<T = unknown> {
  /** 业务码，200 表示成功；非 200 时 data 为 null */
  code: number
  /** 中文文案，由后端错误码表提供 */
  message: string
  /** 业务数据，失败时为 null */
  data: T
  /** 毫秒时间戳 */
  timestamp: number
}

/** 分页响应体，用于所有列表接口 */
export interface PageResult<T = unknown> {
  list: T[]
  total: number
  page: number
  pageSize: number
}
