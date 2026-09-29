import { PERSONA_PAGE_SIZE } from '@/api/persona'
import type { PageResult } from '@/types/api'
import type { Persona } from '@/types/persona'

/**
 * Mock 人设列表 —— 字段与契约 §4 逐字对齐（Persona 有 8 个字段，一个都不能省）。
 * 顺序就是后端 GET /personas 的返回顺序：按 lastMessageAt 倒序，从未聊过的排最后。
 * 排序权在后端，前端拿到什么顺序就渲染什么顺序，不要再排一遍。
 */
export const MOCK_PERSONAS: Persona[] = [
  {
    id: 2,
    name: '阿哲',
    personalityDesc: '大学室友式的存在，爱吐槽，但记得住你说过的小事。',
    speakingStyle: '口语，常用「你别说」「行吧」，偶尔自问自答。',
    state: {},
    familiarity: 58,
    lastMessageAt: '2026-09-27T10:03:00Z',
    createdAt: '2026-09-18T21:30:00Z',
  },
  {
    id: 3,
    name: '林医生',
    personalityDesc: '温和克制，习惯先确认你的感受，再给建议。',
    speakingStyle: '语速慢，先复述你的话，再往下说。',
    state: {},
    familiarity: 31,
    lastMessageAt: '2026-09-26T22:15:00Z',
    createdAt: '2026-09-19T08:00:00Z',
  },
  {
    id: 1,
    name: '小雨',
    personalityDesc: '刚认识不久，话少，但你发的每一句都会认真回。',
    speakingStyle: '短句，很少用感叹号。',
    state: {},
    familiarity: 12,
    lastMessageAt: null,
    createdAt: '2026-09-20T09:00:00Z',
  },
]

/**
 * 与真实 listPersonas 同签名 —— 包括外层那个分页壳 PageResult。
 * 分页在这里真的生效（不是摆设），是为了让侧栏以后接分页器时不用改调用方。
 */
export function listPersonas(
  page = 1,
  pageSize = PERSONA_PAGE_SIZE,
): Promise<PageResult<Persona>> {
  const start = (page - 1) * pageSize

  return Promise.resolve({
    list: MOCK_PERSONAS.slice(start, start + pageSize),
    total: MOCK_PERSONAS.length,
    page,
    pageSize,
  })
}
