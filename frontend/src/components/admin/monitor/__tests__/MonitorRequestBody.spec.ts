import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import type { BodyOverrideMode, ChannelMonitor } from '@/api/admin/channelMonitor'
import type { ChannelMonitorTemplate } from '@/api/admin/channelMonitorTemplate'
import MonitorAdvancedRequestConfig from '../MonitorAdvancedRequestConfig.vue'
import MonitorFormDialog from '../MonitorFormDialog.vue'
import MonitorTemplateManagerDialog from '../MonitorTemplateManagerDialog.vue'

const { updateMonitor, updateTemplate, listTemplates } = vi.hoisted(() => ({
  updateMonitor: vi.fn(), updateTemplate: vi.fn(), listTemplates: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: { update: updateMonitor },
    channelMonitorTemplate: { list: listTemplates, update: updateTemplate },
    accounts: { list: vi.fn().mockResolvedValue({ items: [] }) },
  },
}))
vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn() } }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getUserGroupRates: vi.fn() } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null, showError: vi.fn(), showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

enableAutoUnmount(afterEach)
const prefix = 'admin.channelMonitor.advanced.'
const global = {
  stubs: {
    BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
    MonitorAdvancedRequestConfig: false,
  },
}

async function clickButton(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll('button').find(candidate => candidate.text() === text)
  expect(button, `未找到按钮：${text}`).toBeDefined()
  await button!.trigger('click')
}

async function mountEditor(kind: 'monitor' | 'template', mode: BodyOverrideMode = 'off', body: Record<string, unknown> | null = null) {
  const common = {
    id: 7, name: '测试监控', provider: 'anthropic' as const, api_mode: 'chat_completions' as const,
    extra_headers: {}, body_override_mode: mode, body_override: body, stream_enabled: false,
  }
  if (kind === 'monitor') {
    const wrapper = shallowMount(MonitorFormDialog, {
      props: { show: true, monitor: {
        ...common, endpoint: 'https://example.com', primary_model: 'claude-test', extra_models: [],
        check_mode: 'probe', interval_seconds: 60, enabled: true,
      } as ChannelMonitor },
      global,
    })
    await flushPromises()
    return { wrapper, save: () => wrapper.get('form').trigger('submit'), update: updateMonitor }
  }
  listTemplates.mockResolvedValue({ items: [{ ...common, description: '', associated_monitors: 0 } as ChannelMonitorTemplate] })
  const wrapper = shallowMount(MonitorTemplateManagerDialog, { props: { show: true }, global })
  await flushPromises()
  await clickButton(wrapper, 'common.edit')
  return { wrapper, save: () => clickButton(wrapper, 'common.update'), update: updateTemplate }
}

describe.each(['monitor', 'template'] as const)('%s 请求体保存', kind => {
  beforeEach(() => {
    vi.resetAllMocks()
    listTemplates.mockResolvedValue({ items: [] })
    updateMonitor.mockResolvedValue({})
    updateTemplate.mockResolvedValue({})
  })

  it.each(['', '{}'])('合并模式输入 %s 时提交空对象，占位示例不作为请求体', async text => {
    const { wrapper, save, update } = await mountEditor(kind)
    await clickButton(wrapper, prefix + 'bodyModeMerge')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.get('textarea').attributes('placeholder')).toContain('system')
    await wrapper.get('textarea').setValue(text)
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override_mode: 'merge', body_override: {} }))
  })

  it('提交时解析尚未失焦的新 JSON', async () => {
    const { wrapper, save, update } = await mountEditor(kind, 'merge', { max_tokens: 20 })
    await wrapper.get('textarea').setValue('{"max_tokens":40}')
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override: { max_tokens: 40 } }))
  })

  it('清空旧请求体时发送空对象，避免 null 被解释为不修改', async () => {
    const { wrapper, save, update } = await mountEditor(kind, 'merge', { system: '旧配置' })
    await wrapper.get('textarea').setValue('   ')
    await wrapper.get('textarea').trigger('blur')
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override_mode: 'merge', body_override: {} }))
  })

  it.each(['{', '[]', 'null', '42'])('阻止非法请求体 %s 提交旧值', async text => {
    const { wrapper, save, update } = await mountEditor(kind, 'merge', { max_tokens: 20 })
    await wrapper.get('textarea').setValue(text)
    await save()
    await flushPromises()
    expect(update).not.toHaveBeenCalled()
    expect(wrapper.getComponent(MonitorAdvancedRequestConfig).find('.text-red-500').exists()).toBe(true)
  })

  it.each(['', '{}'])('覆盖模式拒绝空请求体 %s', async text => {
    const { wrapper, save, update } = await mountEditor(kind)
    await clickButton(wrapper, prefix + 'bodyModeReplace')
    await wrapper.get('textarea').setValue(text)
    await save()
    await flushPromises()
    expect(update).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain(prefix + 'bodyJsonRequired')
  })

  it('从空覆盖切换到合并后可以保存', async () => {
    const { wrapper, save, update } = await mountEditor(kind)
    await clickButton(wrapper, prefix + 'bodyModeReplace')
    await save()
    await clickButton(wrapper, prefix + 'bodyModeMerge')
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledTimes(1)
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override_mode: 'merge', body_override: {} }))
  })

  it('完整覆盖请求体仍可保存', async () => {
    const { wrapper, save, update } = await mountEditor(kind, 'replace')
    const body = { model: 'claude-test', messages: [{ role: 'user', content: 'ok' }], max_tokens: 20 }
    await wrapper.get('textarea').setValue(JSON.stringify(body))
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override_mode: 'replace', body_override: body }))
  })

  it('放弃非法 JSON 并切换默认后不再保留旧请求体', async () => {
    const { wrapper, save, update } = await mountEditor(kind, 'merge', { system: '旧配置' })
    await wrapper.get('textarea').setValue('{')
    await clickButton(wrapper, prefix + 'bodyModeOff')
    await save()
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ body_override_mode: 'off', body_override: {} }))
  })
})
