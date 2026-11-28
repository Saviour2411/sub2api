import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelQualitySettings from '../ModelQualitySettings.vue'
const mocks = vi.hoisted(() => ({
  config: vi.fn(),
  save: vi.fn(),
  keys: vi.fn(),
  groups: vi.fn(),
  runNow: vi.fn(),
  all: vi.fn()
}))
vi.mock('@/api/modelQuality', async (original) => ({
  ...(await original<typeof import('@/api/modelQuality')>()),
  modelQualityAPI: mocks
}))
vi.mock('@/api/admin/groups', () => ({
  groupsAPI: {
    getAll: mocks.all,
    getModelAllowlistCandidates: vi.fn().mockResolvedValue(['model-a'])
  }
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ fetchPublicSettings: vi.fn(), showSuccess: vi.fn() })
}))
let wrapper: ReturnType<typeof mount> | undefined
const call = (group_id: number) => ({
  group_id,
  key_id: 1,
  protocol: 'responses',
  model: 'model-a',
  max_output_tokens: 8192,
  timeout_seconds: 600
})
beforeEach(() => {
  vi.clearAllMocks()
  mocks.all.mockResolvedValue([
    { id: 7, name: '国产模型组', platform: 'openai', status: 'active' },
    { id: 8, name: 'Claude 组', platform: 'anthropic', status: 'active' }
  ])
  mocks.config.mockResolvedValue({
    schema_version: 2,
    enabled: false,
    version: 1,
    initialization_required: true,
    endpoint: 'https://api.example.net/v1',
    questions: [
      {
        id: 'art',
        name: '动画题',
        prompt: '生成动画',
        output_type: 'svg',
        rule: {
          kind: 'model',
          judge_prompt: '判断',
          judge: { ...call(8), protocol: 'messages', model: 'judge-model' }
        }
      }
    ],
    groups: [
      {
        group_id: 7,
        interval_minutes: 60,
        targets: [{ ...call(7), id: 'target-a', question_ids: ['art'] }]
      }
    ]
  })
  mocks.keys.mockResolvedValue([{ id: 1, name: '测试密钥', masked_key: 'sk-****abcd' }])
  mocks.groups.mockResolvedValue([])
  mocks.save.mockImplementation((config) => Promise.resolve(config))
})
afterEach(() => wrapper?.unmount())
describe('多平台自定义降智检测设置', () => {
  it('新增模型采用128000输出上限和1200秒超时', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    const add = wrapper.findAll('button').find((button) => button.text() === '添加模型')!
    await add.trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save.mock.lastCall?.[0].groups[0].targets[1]).toMatchObject({
      max_output_tokens: 128000,
      timeout_seconds: 1200
    })
    expect(mocks.save.mock.lastCall?.[0].groups[0].targets[0]).toMatchObject({
      max_output_tokens: 8192,
      timeout_seconds: 600
    })
  })
  it('移除旧独立判题配置，只保存待测模型和判题提示词', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    expect(mocks.all).toHaveBeenCalledWith()
    expect(mocks.keys).toHaveBeenCalledWith(7)
    expect(mocks.keys).not.toHaveBeenCalledWith(8)
    expect(wrapper.findAllComponents({ name: 'QualityCallFields' })).toHaveLength(1)
    expect(wrapper.text()).toContain('sk-****abcd')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({
        questions: [
          expect.objectContaining({
            rule: { kind: 'model', judge_prompt: '判断' }
          })
        ],
        groups: [
          expect.objectContaining({
            targets: [expect.objectContaining({ group_id: 7, protocol: 'responses' })]
          })
        ]
      })
    )
  })
  it('无效扩展JSON阻止保存，修正后可以保存', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    const extra = wrapper.findAll('.quality-extra textarea')[0]
    await extra.setValue('{bad')
    await wrapper.get('form').trigger('submit')
    expect(mocks.save).not.toHaveBeenCalled()
    await extra.setValue('{"temperature":0}')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledTimes(1)
  })
  it('删除题目同步清除模型绑定，草稿允许保存', async () => {
    wrapper = mount(ModelQualitySettings)
    await flushPromises()
    await wrapper.get('[aria-label="删除题目"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({
        questions: [],
        groups: [
          expect.objectContaining({
            targets: [expect.objectContaining({ question_ids: [] })]
          })
        ]
      })
    )
  })
})
