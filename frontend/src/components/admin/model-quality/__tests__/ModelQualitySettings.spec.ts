import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelQualitySettings from '../ModelQualitySettings.vue'

const mocks = vi.hoisted(() => ({
  config: vi.fn(),
  save: vi.fn(),
  keys: vi.fn(),
  groups: vi.fn(),
  runNow: vi.fn()
}))
vi.mock('@/api/modelQuality', async (original) => ({
  ...(await original<typeof import('@/api/modelQuality')>()),
  modelQualityAPI: mocks
}))
vi.mock('@/api/admin/groups', () => ({
  default: {
    getAll: vi.fn().mockResolvedValue([{ id: 7, name: 'OpenAI 分组', status: 'active' }]),
    getModelAllowlistCandidates: vi.fn().mockResolvedValue(['model-a'])
  }
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ fetchPublicSettings: vi.fn(), showSuccess: vi.fn() })
}))
let wrapper: ReturnType<typeof mount> | undefined
beforeEach(() => {
  vi.clearAllMocks()
  mocks.config.mockResolvedValue({
    enabled: false,
    version: 1,
    endpoint: 'https://api.example.net/v1',
    groups: [
      {
        group_id: 7,
        key_id: null,
        interval_minutes: 60,
        models: [],
        methods: ['candy', 'pelican'],
        reasoning_effort: 'low'
      }
    ]
  })
  mocks.keys.mockResolvedValue([{ id: 1, name: '测试密钥', masked_key: 'sk-••••abcd' }])
  mocks.groups.mockResolvedValue([])
  mocks.save.mockImplementation((config) => Promise.resolve(config))
})
afterEach(() => wrapper?.unmount())
describe('降智检测设置', () => {
  it('至少选择一个模型，允许未绑定密钥保存', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(wrapper.text()).toContain('至少选择一个检测模型')
    expect(mocks.save).not.toHaveBeenCalled()
    await wrapper.get('[aria-label="补充模型 ID"]').setValue('custom-model')
    await wrapper.get('[aria-label="添加模型"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({
        enabled: false,
        groups: [
          expect.objectContaining({
            key_id: null,
            models: ['custom-model'],
            methods: ['candy', 'pelican'],
            interval_minutes: 60,
            reasoning_effort: 'low'
          })
        ]
      })
    )
    expect(wrapper.text()).toContain('待绑定密钥')
  })
  it('只展示脱敏密钥并说明第三方检测消耗', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    expect(mocks.keys).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('sk-••••abcd')
    expect(wrapper.text()).toContain('交给满血AI')
    expect(wrapper.text()).toContain('消耗该密钥额度')
  })
})
