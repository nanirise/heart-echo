/**
 * 业务错误码 —— 与后端 `backend/pkg/errcode/errcode.go` 逐条对应。
 *
 * 后端遵循「一 code 一 msg」：一个错误码只对应一句固定文案。
 * 本文件只镜像「常量」与「文案」两部分，不镜像 HTTP 状态码映射
 * （HTTP 状态在 axios 响应的 status 上，前端不需要自己算）。
 *
 * 常量名与后端保持完全一致（含 Err 前缀），便于两侧逐条比对。
 * 权威来源是 errcode.go，不是 API_CONTRACT.md 的文字表。
 * 修改本文件前请先确认后端已同步。
 */

/** 业务错误码常量。用 as const 保留字面量类型，便于穷尽检查 */
export const ErrorCode = {
  /** 成功 */
  Success: 200,

  /** 参数校验失败 */
  ErrInvalidParams: 4001,
  /** 必填参数缺失 */
  ErrParamMissing: 4002,
  /** 该邮箱已被注册 */
  ErrEmailExists: 4003,
  /** 该用户名已被占用 */
  ErrUsernameTaken: 4004,

  /** 未登录或登录已过期 */
  ErrUnauthorized: 4010,
  /** Token 无效 */
  ErrTokenInvalid: 4011,
  /** Token 已过期 */
  ErrTokenExpired: 4012,
  /** 用户名或密码错误 */
  ErrPasswordWrong: 4013,
  /** 刷新令牌无效，请重新登录 */
  ErrRefreshInvalid: 4014,
  /** 原密码不正确 */
  ErrOldPasswordWrong: 4015,

  /** 无权限使用该功能（功能越权专用；资源越权走 ErrPersonaNotFound） */
  ErrForbidden: 4030,

  /** 资源不存在 */
  ErrNotFound: 4040,
  /** 用户不存在 */
  ErrUserNotFound: 4041,
  /** 人设不存在（同时承担资源越权语义，前端不区分） */
  ErrPersonaNotFound: 4043,

  /** 服务端内部错误 */
  ErrInternal: 5000,
  /** AI 回复生成失败，请稍后重试 */
  ErrLLMFailed: 5001,
  /** AI 服务暂时不可用 */
  ErrAIUnavailable: 5002,
  /** 数据库操作失败 */
  ErrDBFailed: 5003,
} as const

/** 错误码的取值类型，例如 200 | 4001 | 4002 | ... */
export type ErrorCodeValue = (typeof ErrorCode)[keyof typeof ErrorCode]

/**
 * 错误码到中文文案的映射，作为「后端未返回 message」时的兜底。
 * 正常流程下应优先使用后端返回的 message。
 */
export const ErrorCodeMessages: Record<ErrorCodeValue, string> = {
  [ErrorCode.Success]: 'success',

  [ErrorCode.ErrInvalidParams]: '参数校验失败',
  [ErrorCode.ErrParamMissing]: '必填参数缺失',
  [ErrorCode.ErrEmailExists]: '该邮箱已被注册',
  [ErrorCode.ErrUsernameTaken]: '该用户名已被占用',

  [ErrorCode.ErrUnauthorized]: '未登录或登录已过期',
  [ErrorCode.ErrTokenInvalid]: 'Token 无效',
  [ErrorCode.ErrTokenExpired]: 'Token 已过期',
  [ErrorCode.ErrPasswordWrong]: '用户名或密码错误',
  [ErrorCode.ErrRefreshInvalid]: '刷新令牌无效，请重新登录',
  [ErrorCode.ErrOldPasswordWrong]: '原密码不正确',

  [ErrorCode.ErrForbidden]: '无权限使用该功能',

  [ErrorCode.ErrNotFound]: '资源不存在',
  [ErrorCode.ErrUserNotFound]: '用户不存在',
  [ErrorCode.ErrPersonaNotFound]: '人设不存在',

  [ErrorCode.ErrInternal]: '服务端内部错误',
  [ErrorCode.ErrLLMFailed]: 'AI 回复生成失败，请稍后重试',
  [ErrorCode.ErrAIUnavailable]: 'AI 服务暂时不可用',
  [ErrorCode.ErrDBFailed]: '数据库操作失败',
}
