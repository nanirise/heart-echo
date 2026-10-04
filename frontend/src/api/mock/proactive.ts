import type { SettingsResponse, UpdateSettingsPayload } from '@/types/proactive'

/**
 * Mock 配置存在内存里：同一次会话内「改完切走再切回」能读到改后的值，
 * 刷新页面回到种子值。真实持久化由后端负责（VITE_USE_MOCK=false 时走真接口）。
 */
const DEFAULT_SETTINGS: Omit<SettingsResponse, 'personaId'> = {
  enabled: true,
  intervalMin: 30,
  intervalMax: 120,
  dailyLimit: 3,
  lastNudgeAt: null,
}

/** 种子差异：id=3 演示「已关闭 + 更稀疏 + 近期触发过」的另一种形态 */
const SEEDS = new Map<number, Partial<Omit<SettingsResponse, 'personaId'>>>([
  [
    3,
    {
      enabled: false,
      intervalMin: 60,
      intervalMax: 180,
      dailyLimit: 1,
      lastNudgeAt: '2026-09-26T21:40:00Z',
    },
  ],
])

const settingsByPersona = new Map<number, SettingsResponse>()

function getRecord(personaId: number): SettingsResponse {
  const existing = settingsByPersona.get(personaId)

  if (existing !== undefined) {
    return existing
  }

  const record: SettingsResponse = {
    personaId,
    ...DEFAULT_SETTINGS,
    ...(SEEDS.get(personaId) ?? {}),
  }

  settingsByPersona.set(personaId, record)
  return record
}

/** 返回副本：调用方若改到 Mock 内部状态，「保存后回读一致」的行为就和真实实现不一致了 */
export function getSettings(personaId: number): Promise<SettingsResponse> {
  return Promise.resolve({ ...getRecord(personaId) })
}

/** 与真实现同签名；`lastNudgeAt` 只读，PUT 只覆盖四个可改字段（契约 §9） */
export function updateSettings(payload: UpdateSettingsPayload): Promise<SettingsResponse> {
  const updated: SettingsResponse = {
    personaId: payload.personaId,
    enabled: payload.enabled,
    intervalMin: payload.intervalMin,
    intervalMax: payload.intervalMax,
    dailyLimit: payload.dailyLimit,
    lastNudgeAt: getRecord(payload.personaId).lastNudgeAt,
  }

  settingsByPersona.set(payload.personaId, updated)
  return Promise.resolve({ ...updated })
}
