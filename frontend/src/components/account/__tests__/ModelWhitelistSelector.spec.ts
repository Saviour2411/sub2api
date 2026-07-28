import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { accountsAPI } from '@/api/admin/accounts'

const copyToClipboard = vi.hoisted(() => vi.fn().mockResolvedValue(true))
const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'common.copy') return '复制'
        return `${key}${params ? JSON.stringify(params) : ''}`
      },
    }),
  }
})

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

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

function mountSelector() {
  return mount(ModelWhitelistSelector, {
    props: {
      modelValue: [],
      platform: 'openai',
    },
    global: {
      stubs: {
        ModelIcon: true,
      },
    },
  })
}

function findModelRow(wrapper: ReturnType<typeof mountSelector>, modelId: string) {
  const row = wrapper
    .findAll('[data-testid="model-option"]')
    .find((candidate) => candidate.text().includes(modelId))

  if (!row) {
    throw new Error(`未找到模型行：${modelId}`)
  }

  return row
}

describe('ModelWhitelistSelector', () => {
  beforeEach(() => {
    vi.mocked(accountsAPI.syncUpstreamModels).mockReset()
    vi.mocked(accountsAPI.syncUpstreamModelsPreview).mockReset()
    copyToClipboard.mockClear()
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
  })

  it('在账号保存前使用预览凭据同步上游模型', async () => {
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

  it('复制模型 ID 时不选择模型', async () => {
    const wrapper = mountSelector()
    await wrapper.get('div.cursor-pointer').trigger('click')

    const row = findModelRow(wrapper, 'gpt-5.6-sol')
    const copyButton = row.get('[data-testid="copy-model-id"]')
    expect(copyButton.attributes('aria-label')).toBe('复制 gpt-5.6-sol')

    await copyButton.trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenCalledWith('gpt-5.6-sol')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('保留已有模型选择行为', async () => {
    const wrapper = mountSelector()
    await wrapper.get('div.cursor-pointer').trigger('click')

    const row = findModelRow(wrapper, 'gpt-5.6-sol')
    await row.get('[data-testid="select-model"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[['gpt-5.6-sol']]])
    expect(copyToClipboard).not.toHaveBeenCalled()
  })
})
