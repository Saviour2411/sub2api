import type { Account } from '@/types'
import { getProviderProfile } from '@/constants/platformCatalog'

type TestAccount = Pick<Account, 'platform' | 'type' | 'credentials'>
type TestModel = { id: string }

const defaultModels: Record<string, string> = {
  openai: 'gpt-6-astra',
  anthropic: 'claude-opus-5',
  gemini: 'gemini-3.8-flash',
  grok: 'grok-4.6',
  opencode_go: 'glm-5.3',
  antigravity: 'claude-opus-5'
}

export function pickAccountTestDefaultModel(account: TestAccount, models: TestModel[]): string {
  if (getProviderProfile(account.platform)) return models[0]?.id || ''
  let preferred = defaultModels[account.platform]
  if (account.type === 'bedrock') preferred = 'claude-sonnet-4-5-20250929'
  if (account.platform === 'gemini' && account.type === 'oauth' && account.credentials?.oauth_type === 'google_one') {
    preferred = 'gemini-2.0-flash'
  }
  // 自定义白名单或旧服务端未提供新默认模型时，仍保留已提供的测试选项。
  return models.find(model => model.id === preferred)?.id || models[0]?.id || ''
}

export function accountTestRequestModel(account: TestAccount, selected: string, defaultModel: string): string {
  // 多协议供应商的默认项交由后端动态解析，防止上游目标再次命中通配符映射。
  return getProviderProfile(account.platform) && selected === defaultModel ? '' : selected
}
