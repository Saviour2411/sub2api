import type { Account } from '@/types'

type TestAccount = Pick<Account, 'platform' | 'type' | 'credentials'>
type TestModel = { id: string }

const defaultModels: Record<string, string> = {
  openai: 'gpt-6-astra',
  anthropic: 'claude-opus-5',
  gemini: 'gemini-3.8-flash',
  grok: 'grok-4.6',
  antigravity: 'claude-opus-5'
}

const cnPlatforms = new Set(['kimi', 'zhipu', 'deepseek', 'minimax'])

export function pickAccountTestDefaultModel(account: TestAccount, models: TestModel[]): string {
  if (cnPlatforms.has(account.platform)) return models[0]?.id || ''
  let preferred = defaultModels[account.platform]
  if (account.type === 'bedrock') preferred = 'claude-sonnet-4-5-20250929'
  if (account.platform === 'gemini' && account.type === 'oauth' && account.credentials?.oauth_type === 'google_one') {
    preferred = 'gemini-2.0-flash'
  }
  // 自定义白名单或旧服务端未提供新默认模型时，仍保留已提供的测试选项。
  return models.find(model => model.id === preferred)?.id || models[0]?.id || ''
}

export function accountTestRequestModel(account: TestAccount, selected: string, defaultModel: string): string {
  // 国产默认项交由后端按账号配置解析，防止具体上游目标再次命中通配符映射。
  return cnPlatforms.has(account.platform) && selected === defaultModel ? '' : selected
}
