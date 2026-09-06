import { describe, expect, it } from 'vitest'
import type { Account } from '@/types'
import { accountTestRequestModel, pickAccountTestDefaultModel } from '../accountTestModels'

const account = (platform: Account['platform'], type: Account['type'] = 'apikey', credentials: Record<string, unknown> = {}) => ({
  platform, type, credentials
})

describe('账号测试默认模型', () => {
  it.each([
    ['openai', 'gpt-6-astra'],
    ['anthropic', 'claude-opus-5'],
    ['gemini', 'gemini-3.8-flash'],
    ['grok', 'grok-4.6'],
    ['antigravity', 'claude-opus-5']
  ] as const)('%s 优先预选 %s', (platform, preferred) => {
    expect(pickAccountTestDefaultModel(account(platform), [{ id: 'old-model' }, { id: preferred }])).toBe(preferred)
  })

  it('保留受限接入的默认值', () => {
    expect(pickAccountTestDefaultModel(account('anthropic', 'bedrock'), [
      { id: 'claude-opus-5' }, { id: 'claude-sonnet-4-5-20250929' }
    ])).toBe('claude-sonnet-4-5-20250929')
    expect(pickAccountTestDefaultModel(account('gemini', 'oauth', { oauth_type: 'google_one' }), [
      { id: 'gemini-3.8-flash' }, { id: 'gemini-2.0-flash' }
    ])).toBe('gemini-2.0-flash')
  })

  it.each(['kimi', 'zhipu', 'deepseek'] as const)('%s 使用后端确定的首选项并保留单次映射语义', platform => {
    const a = account(platform)
    const preferred = pickAccountTestDefaultModel(a, [{ id: 'native-model' }, { id: 'another-model' }])
    expect(preferred).toBe('native-model')
    expect(accountTestRequestModel(a, preferred, preferred)).toBe('')
    expect(accountTestRequestModel(a, 'another-model', preferred)).toBe('another-model')
  })

  it('不覆盖自定义选项或显式模型', () => {
    const a = account('openai')
    expect(pickAccountTestDefaultModel(a, [{ id: 'custom-model' }])).toBe('custom-model')
    expect(pickAccountTestDefaultModel(a, [])).toBe('')
    expect(accountTestRequestModel(a, 'explicit-model', 'gpt-6-astra')).toBe('explicit-model')
  })
})
