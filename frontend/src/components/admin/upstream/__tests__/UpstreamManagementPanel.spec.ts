import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamManagementPanel from '../UpstreamManagementPanel.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), setEnabled: vi.fn(), remove: vi.fn(),
  sync: vi.fn(), syncAll: vi.fn(), groups: vi.fn(), history: vi.fn(),
  showSuccess: vi.fn(), showError: vi.fn(),
}))

vi.mock('@/api/admin/upstreams', () => ({ default: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: api.showSuccess, showError: api.showError }) }))
vi.mock('@/utils/apiError', () => ({ extractApiErrorMessage: (_error: unknown, fallback: string) => fallback }))
vi.mock('vue-chartjs', () => ({ Line: { template: '<div data-test="history-chart" />' } }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => key.replace(/\{(\w+)\}/g, (_match, token) => String(params?.[token] ?? `{${token}}`)),
    }),
  }
})

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id || row.remote_id" data-test="data-row">
        <slot name="cell-site" :row="row" />
        <slot name="cell-platform" :row="row" />
        <slot name="cell-status" :row="row" />
        <slot name="cell-actions" :row="row" />
        <slot name="cell-multiplier" :row="row" />
        <slot name="cell-today_tokens" :row="row" />
        <slot name="cell-today_cost_usd" :row="row" />
      </div>
      <slot v-if="data.length === 0" name="empty" />
    </div>
  `,
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean, title: String },
  emits: ['close'],
  template: '<div v-if="show" data-test="dialog"><h3>{{ title }}</h3><slot /><div><slot name="footer" /></div></div>',
})

const ToggleStub = defineComponent({
  props: { modelValue: Boolean },
  emits: ['update:modelValue'],
  template: '<button type="button" data-test="toggle" @click="$emit(\'update:modelValue\', !modelValue)">{{ modelValue }}</button>',
})

function siteFixture(overrides = {}) {
  return {
    id: 1,
    name: '上游一号',
    base_url: 'https://upstream.example.com',
    platform: 'sub2api',
    auth_mode: 'password',
    account: 'admin@example.com',
    enabled: true,
    status: 'healthy',
    error_message: null,
    balance_usd: 10,
    today_tokens: 100,
    today_cost_usd: 0.1,
    total_tokens: 1000,
    total_cost_usd: 1,
    tracking_started_at: '2026-07-01T00:00:00Z',
    last_synced_at: '2026-07-15T00:00:00Z',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-15T00:00:00Z',
    has_password: true,
    has_token: false,
    ...overrides,
  }
}

function mountPanel() {
  return mount(UpstreamManagementPanel, {
    global: {
      stubs: { DataTable: DataTableStub, BaseDialog: BaseDialogStub, Toggle: ToggleStub, Icon: true },
    },
  })
}

describe('UpstreamManagementPanel', () => {
  beforeEach(() => {
    Object.values(api).forEach((mock) => mock.mockReset())
    api.list.mockResolvedValue({ items: [siteFixture()], total: 1, page: 1, page_size: 20, pages: 1 })
    api.create.mockResolvedValue(siteFixture({ id: 2 }))
    api.update.mockResolvedValue(siteFixture())
    api.setEnabled.mockResolvedValue(siteFixture({ enabled: false }))
    api.remove.mockResolvedValue(undefined)
    api.sync.mockResolvedValue(undefined)
    api.syncAll.mockResolvedValue({ queued: 1 })
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'openai', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, last_synced_at: '2026-07-15T00:00:00Z' }])
    api.history.mockResolvedValue([{ id: 1, site_id: 1, date: '2026-07-15T00:00:00Z', balance_usd: 10, tokens: 100, cost_usd: 0.1, created_at: '', updated_at: '' }])
  })

  it('展示安全外链并支持同步、启停和详情数据', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const links = wrapper.findAll('a[href="https://upstream.example.com"]')
    expect(links.length).toBeGreaterThan(0)
    links.forEach((link) => {
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toBe('noopener noreferrer')
    })

    await wrapper.get('button[title="admin.customFeatures.upstream.sync"]').trigger('click')
    expect(api.sync).toHaveBeenCalledWith(1)

    await wrapper.get('button[title="admin.customFeatures.upstream.disable"]').trigger('click')
    await flushPromises()
    expect(api.setEnabled).toHaveBeenCalledWith(1, false)

    await wrapper.get('button[title="admin.customFeatures.upstream.details"]').trigger('click')
    await flushPromises()
    expect(api.groups).toHaveBeenCalledWith(1)
    expect(api.history).toHaveBeenCalledWith(1, 30)
    expect(wrapper.text()).toContain('1.5×')
    wrapper.unmount()
  })

  it('New API 表单固定密码认证并提交新增站点', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="upstream-add"]').trigger('click')

    await wrapper.get('#upstream-name').setValue('New API 上游')
    await wrapper.get('#upstream-url').setValue('https://newapi.example.com')
    await wrapper.get('#upstream-platform').setValue('newapi')
    await wrapper.get('#upstream-account').setValue('admin')
    await wrapper.get('#upstream-password').setValue('secret')

    expect(wrapper.get<HTMLSelectElement>('#upstream-auth-mode').element.disabled).toBe(true)
    expect(wrapper.get<HTMLSelectElement>('#upstream-auth-mode').element.value).toBe('password')
    expect(wrapper.find('#upstream-access-token').exists()).toBe(false)

    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      name: 'New API 上游',
      base_url: 'https://newapi.example.com',
      platform: 'newapi',
      auth_mode: 'password',
      account: 'admin',
      password: 'secret',
    }))
    wrapper.unmount()
  })

  it('编辑时不回填敏感凭证且允许空凭证更新', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('button[title="admin.customFeatures.upstream.edit"]').trigger('click')

    expect(wrapper.get<HTMLInputElement>('#upstream-password').element.value).toBe('')
    expect(wrapper.get('#upstream-password').attributes('placeholder')).toBe('admin.customFeatures.upstream.keepCredential')
    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(1, expect.objectContaining({ password: '' }))
    wrapper.unmount()
  })
})
