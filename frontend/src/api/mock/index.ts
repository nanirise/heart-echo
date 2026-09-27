import * as real from '@/api/chat'
import * as mock from '@/api/mock/chat'

// 开关只在这一个地方判断，业务代码永远 import 这个出口
export const chatApi = import.meta.env.VITE_USE_MOCK === 'true' ? mock : real

export { MOCK_PERSONAS } from '@/api/mock/chat'
export type { MockPersona } from '@/api/mock/chat'
