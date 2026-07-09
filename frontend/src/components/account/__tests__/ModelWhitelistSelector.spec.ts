import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { accountsAPI } from '@/api/admin/accounts'

const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showInfo,
    showSuccess,
  }),
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels: vi.fn(),
    syncUpstreamModelsPreview: vi.fn(),
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => `${key}${params ? JSON.stringify(params) : ''}`,
    }),
  }
})

describe('ModelWhitelistSelector', () => {
  beforeEach(() => {
    vi.mocked(accountsAPI.syncUpstreamModels).mockReset()
    vi.mocked(accountsAPI.syncUpstreamModelsPreview).mockReset()
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
  })

  it('syncs upstream models with preview credentials before an account is saved', async () => {
    vi.mocked(accountsAPI.syncUpstreamModelsPreview).mockResolvedValue({
      models: ['gpt-5.1', 'gpt-5.2', 'gpt-5.1'],
      source: 'preview',
    } as any)

    const previewSyncRequest = {
      platform: 'openai',
      account_type: 'apikey',
      base_url: 'https://api.openai.com/v1',
      api_key: 'sk-test',
    }
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: ['gpt-4.1'],
        platform: 'openai',
        previewSyncRequest,
      },
      global: {
        stubs: {
          ModelIcon: true,
          Icon: true,
        },
      },
    })

    const syncButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.accounts.syncUpstreamModels'))

    expect(syncButton).toBeDefined()
    await syncButton?.trigger('click')
    await flushPromises()

    expect(accountsAPI.syncUpstreamModelsPreview).toHaveBeenCalledWith(previewSyncRequest)
    expect(accountsAPI.syncUpstreamModels).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([
      'gpt-4.1',
      'gpt-5.1',
      'gpt-5.2',
    ])
    expect(showSuccess).toHaveBeenCalled()
  })
})
