/**
 * 契约里的时间统一是 RFC3339（可能带 6 位小数）。
 * 交给 Date 解析，不按字符串切片 —— 精度或时区一变，切片就会静默错位。
 */
export function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'

  const pad = (part: number): string => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`
}
