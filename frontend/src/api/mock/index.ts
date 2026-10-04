import * as realChat from '@/api/chat'
import * as realPersona from '@/api/persona'
import * as realProactive from '@/api/proactive'
import * as mockChat from '@/api/mock/chat'
import * as mockPersona from '@/api/mock/persona'
import * as mockProactive from '@/api/mock/proactive'

const useMock = import.meta.env.VITE_USE_MOCK === 'true'

// 开关只在这一个地方判断，业务代码永远 import 这个出口
export const chatApi = useMock ? mockChat : realChat

// 拿真实实现的签名给 Mock 当模板：两边一旦漂移，typecheck 立刻报错
export const listPersonas: typeof realPersona.listPersonas = useMock
  ? mockPersona.listPersonas
  : realPersona.listPersonas

export const proactiveApi: typeof realProactive = useMock ? mockProactive : realProactive
