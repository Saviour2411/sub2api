import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamManagementPanel from '../UpstreamManagementPanel.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), setEnabled: vi.fn(), remove: vi.fn(),
  listAll: vi.fn(), updateSortOrder: vi.fn(), probeCapabilities: vi.fn(), sync: vi.fn(), syncAll: vi.fn(), groups: vi.fn(), setGroupDisplayed: vi.fn(), replaceGroupBindings: vi.fn(), history: vi.fn(), multiplierHistory: vi.fn(),
  showSuccess: vi.fn(), showError: vi.fn(),
}))

const bindingAPIs = vi.hoisted(() => ({
  getGroups: vi.fn(),
  listAccounts: vi.fn(),
}))

vi.mock('@/api/admin/upstreams', () => ({ default: api }))
vi.mock('@/api/admin/groups', () => ({ default: { getAll: bindingAPIs.getGroups } }))
vi.mock('@/api/admin/accounts', () => ({ default: { list: bindingAPIs.listAccounts } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: api.showSuccess, showError: api.showError }) }))
vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  extractApiErrorCode: (error: unknown) => error && typeof error === 'object' && 'reason' in error ? String((error as { reason: unknown }).reason) : undefined,
}))
vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div data-test="history-chart" :data-stepped="data.datasets?.[0]?.stepped || \'\'" :data-first-x="data.datasets?.[0]?.data?.[0]?.x ?? \'\'" :data-datasets="data.datasets?.map((item) => item.yAxisID).join(\',\') || \'\'" />',
  },
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.customFeatures.upstream.bindings.deleteWarning') return `${key}:${params?.count}`
        return key.replace(/\{(\w+)\}/g, (_match, token) => String(params?.[token] ?? `{${token}}`))
      },
    }),
  }
})

const DataTableStub = defineComponent({
  props: {
    columns: { type: Array, default: () => [] },
    data: { type: Array, default: () => [] },
    expandedRowKeys: { type: Array, default: () => [] },
    expandableActions: { type: Boolean, default: true },
    loading: Boolean,
  },
  template: `
    <div data-test="data-table" :data-loading="String(loading)">
      <div v-for="row in data" :key="row.id || row.remote_id" data-test="data-row">
        <slot name="cell-site" :row="row" />
        <slot name="cell-platform" :row="row" />
        <slot name="cell-status" :row="row" />
        <slot name="cell-today" :row="row" />
        <slot name="cell-total" :row="row" />
        <slot name="cell-last_synced_at" :row="row" />
        <slot name="cell-actions" :row="row" />
        <slot name="cell-name" :row="row" />
        <slot name="cell-account" :row="row" />
        <slot name="cell-local_group_name" :row="row" />
        <slot name="cell-account_status" :row="row" />
        <slot name="cell-account_priority" :row="row" />
        <slot name="cell-priority" :row="row" />
        <slot name="cell-multiplier" :row="row" />
        <slot name="cell-today_tokens" :row="row" />
        <slot name="cell-today_cost_usd" :row="row" />
        <slot v-if="expandedRowKeys.includes(row.id)" name="row-details" :row="row" />
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
    token_metrics_available: true,
    tracking_started_at: '2026-07-01T00:00:00Z',
    last_synced_at: '2026-07-15T00:00:00Z',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-15T00:00:00Z',
    has_password: true,
    has_token: false,
    displayed_group_count: 0,
    binding_count: 0,
    ...overrides,
  }
}

function siteListResult(site = siteFixture()) {
  return { items: [site], total: 1, page: 1, page_size: 20, pages: 1 }
}

function bindingFixture(overrides = {}) {
  return {
    id: 1,
    upstream_group_id: 1,
    local_group_id: 10,
    local_group_name: '本地 OpenAI',
    account_id: 101,
    account_name: '账号一',
    account_platform: 'openai',
    account_status: 'active',
    account_priority: 10,
    created_at: '2026-07-16T00:00:00Z',
    ...overrides,
  }
}

function displayedGroupFixture(overrides = {}) {
  return {
    id: 1,
    site_id: 1,
    remote_id: 'vip',
    name: 'VIP',
    platform: 'OpenAI',
    description: '低倍率分组',
    multiplier: 0.5,
    today_tokens: 100,
    today_cost_usd: 0.1,
    token_metrics_available: true,
    displayed: true,
    available: true,
    last_synced_at: '2026-07-15T00:00:00Z',
    bindings: [],
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

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((complete, fail) => { resolve = complete; reject = fail })
  return { promise, resolve, reject }
}

describe('UpstreamManagementPanel', () => {
  beforeEach(() => {
    Object.values(api).forEach((mock) => mock.mockReset())
    Object.values(bindingAPIs).forEach((mock) => mock.mockReset())
    api.list.mockResolvedValue({ items: [siteFixture()], total: 1, page: 1, page_size: 20, pages: 1 })
    api.create.mockResolvedValue(siteFixture({ id: 2 }))
    api.probeCapabilities.mockResolvedValue({ base_url: 'https://upstream.example.com', platform: 'sub2api', turnstile_enabled: false, token_auth_recommended: false })
    api.listAll.mockResolvedValue([siteFixture(), siteFixture({ id: 2, name: '上游二号' })])
    api.updateSortOrder.mockResolvedValue({ updated: 2 })
    api.update.mockResolvedValue(siteFixture())
    api.setEnabled.mockResolvedValue(siteFixture({ enabled: false }))
    api.remove.mockResolvedValue(undefined)
    api.sync.mockResolvedValue(undefined)
    api.syncAll.mockResolvedValue({ queued: 1 })
    api.replaceGroupBindings.mockResolvedValue({})
    bindingAPIs.getGroups.mockResolvedValue([])
    bindingAPIs.listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '高优先级分组', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, displayed: false, available: true, last_synced_at: '2026-07-15T00:00:00Z' }])
    api.setGroupDisplayed.mockImplementation((_id: number, _remoteID: string, displayed: boolean) => Promise.resolve({
      group: { id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '高优先级分组', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, displayed, available: true, last_synced_at: '2026-07-15T00:00:00Z' },
      displayed_group_count: displayed ? 1 : 0,
    }))
    api.history.mockResolvedValue([{ id: 1, site_id: 1, date: '2026-07-15T00:00:00Z', balance_usd: 10, tokens: 100, cost_usd: 0.1, created_at: '', updated_at: '' }])
    api.multiplierHistory.mockResolvedValue([
      { remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '高优先级分组', current_multiplier: 1.5, points: [{ recorded_at: '2026-07-15T00:00:00Z', multiplier: 1.5 }] },
      { remote_id: 'free', name: 'Free', platform: 'OpenAI', description: '免费分组', current_multiplier: null, points: [{ recorded_at: '2026-07-15T00:00:00Z', multiplier: null }] },
    ])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('默认不在账号下方展示未添加分组', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[data-test="upstream-expand-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(false)
    expect(api.groups).not.toHaveBeenCalled()
    wrapper.unmount()
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

  it('从可用分组添加展示并隐藏最后一个分组', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('button[title="admin.customFeatures.upstream.details"]').trigger('click')
    await flushPromises()

    const displayButton = wrapper.get('[data-test="upstream-group-display-vip"]')
    expect(displayButton.text()).toContain('admin.customFeatures.upstream.addGroupDisplay')
    await displayButton.trigger('click')
    await flushPromises()
    expect(api.setGroupDisplayed).toHaveBeenCalledWith(1, 'vip', true)
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('VIP')

    await wrapper.get('[data-test="upstream-group-display-vip"]').trigger('click')
    await flushPromises()
    expect(api.setGroupDisplayed).toHaveBeenLastCalledWith(1, 'vip', false)
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('展示暂不可用分组的末次指标并允许隐藏', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([{
      id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '末次描述',
      multiplier: 1.5, today_tokens: 10_388_595_898, today_cost_usd: 71.56, displayed: true, available: false,
      last_synced_at: '2026-07-14T12:00:00Z',
    }])
    api.setGroupDisplayed.mockResolvedValue({
      group: { id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '末次描述', multiplier: 1.5, today_tokens: 10_388_595_898, today_cost_usd: 71.56, displayed: false, available: false, last_synced_at: '2026-07-14T12:00:00Z' },
      displayed_group_count: 0,
    })
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.customFeatures.upstream.unavailable')
    expect(wrapper.text()).toContain('10.4B')
    expect(wrapper.text()).toContain('$71.56')
    await wrapper.get('button[title="admin.customFeatures.upstream.details"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="upstream-group-display-vip"]').trigger('click')
    await flushPromises()
    expect(api.setGroupDisplayed).toHaveBeenCalledWith(1, 'vip', false)
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

  it('检测 Turnstile 后自动切换令牌认证并从登录响应导入令牌', async () => {
    api.probeCapabilities.mockResolvedValue({
      base_url: 'https://walkcoding.top', platform: 'sub2api', turnstile_enabled: true, token_auth_recommended: true,
    })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="upstream-add"]').trigger('click')
    await wrapper.get('#upstream-name').setValue('Walk Coding')
    await wrapper.get('#upstream-url').setValue('https://walkcoding.top')
    await wrapper.get('#upstream-url').trigger('blur')
    await flushPromises()

    expect(api.probeCapabilities).toHaveBeenCalledWith({ base_url: 'https://walkcoding.top', platform: 'sub2api' })
    expect(wrapper.get<HTMLSelectElement>('#upstream-auth-mode').element.value).toBe('token')
    expect(wrapper.find('[data-test="upstream-turnstile-notice"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="upstream-open-login"]').attributes('href')).toBe('https://walkcoding.top/login')
    expect(wrapper.find('#upstream-password').exists()).toBe(false)

    await wrapper.get('[data-test="upstream-login-response"]').setValue(JSON.stringify({
      code: 0,
      data: { access_token: 'access-from-login', refresh_token: 'refresh-from-login' },
    }))
    await wrapper.get('[data-test="upstream-import-login-response"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('#upstream-access-token').element.value).toBe('access-from-login')
    expect(wrapper.get<HTMLInputElement>('#upstream-refresh-token').element.value).toBe('refresh-from-login')
    expect(wrapper.get<HTMLInputElement>('#upstream-user-agent').element.value).toBe(navigator.userAgent)
    expect(wrapper.get<HTMLTextAreaElement>('[data-test="upstream-login-response"]').element.value).toBe('')

    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({
      base_url: 'https://walkcoding.top',
      platform: 'sub2api',
      auth_mode: 'token',
      access_token: 'access-from-login',
      refresh_token: 'refresh-from-login',
      user_agent: navigator.userAgent,
    }))
    wrapper.unmount()
  })

  it('创建接口返回 Turnstile 专用错误时保留表单并切换令牌认证', async () => {
    api.probeCapabilities.mockRejectedValue(new Error('旧版本无公开设置'))
    api.create.mockRejectedValue({ reason: 'UPSTREAM_TURNSTILE_REQUIRED' })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="upstream-add"]').trigger('click')
    await wrapper.get('#upstream-name').setValue('Turnstile 上游')
    await wrapper.get('#upstream-url').setValue('https://turnstile.example.com')
    await wrapper.get('#upstream-account').setValue('admin@example.com')
    await wrapper.get('#upstream-password').setValue('secret')
    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()

    expect(api.create).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="upstream-turnstile-notice"]').exists()).toBe(true)
    expect(wrapper.get<HTMLSelectElement>('#upstream-auth-mode').element.value).toBe('token')
    expect(api.showError).toHaveBeenCalledWith('admin.customFeatures.upstream.turnstileRequiresToken')
    wrapper.unmount()
  })

  it('编辑密码站点切换令牌认证后要求新令牌并移除密码载荷', async () => {
    api.probeCapabilities.mockResolvedValue({
      base_url: 'https://upstream.example.com', platform: 'sub2api', turnstile_enabled: true, token_auth_recommended: true,
    })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('button[title="admin.customFeatures.upstream.edit"]').trigger('click')
    await flushPromises()

    expect(wrapper.get<HTMLSelectElement>('#upstream-auth-mode').element.value).toBe('token')
    expect(wrapper.get('#upstream-access-token').attributes('placeholder')).toBe('')
    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()
    expect(api.update).not.toHaveBeenCalled()
    expect(api.showError).toHaveBeenCalledWith('admin.customFeatures.upstream.tokenRequired')

    await wrapper.get('#upstream-access-token').setValue('new-access-token')
    await wrapper.get('[data-test="upstream-form"]').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(1, expect.objectContaining({
      auth_mode: 'token',
      account: '',
      password: undefined,
      access_token: 'new-access-token',
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

  it('用 K/M/B 展示 Token，并通过 title 保留精确值，实际消耗直接取接口值', async () => {
    api.list.mockResolvedValue({
      items: [siteFixture({ today_tokens: 10_388_595_898, total_tokens: 1_234_567, today_cost_usd: 71.56, total_cost_usd: 88.12 })],
      total: 1, page: 1, page_size: 20, pages: 1,
    })
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('10.4B')
    expect(wrapper.text()).toContain('1.2M')
    expect(wrapper.text()).toContain('$71.56')
    expect(wrapper.find('[title="10,388,595,898"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('New API 隐藏站点、分组和历史 Token，同时保留费用展示', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({
      platform: 'newapi', token_metrics_available: false, displayed_group_count: 1,
      today_tokens: 10_388_595_898, total_tokens: 1_234_567, today_cost_usd: 71.56, total_cost_usd: 88.12,
    })))
    api.groups.mockResolvedValue([displayedGroupFixture({
      token_metrics_available: false, today_tokens: 9_876_543, today_cost_usd: 12.34,
    })])
    api.history.mockResolvedValue([{
      id: 1, site_id: 1, date: '2026-07-15T00:00:00Z', balance_usd: 10,
      tokens: 10_388_595_898, cost_usd: 71.56, token_metrics_available: false, created_at: '', updated_at: '',
    }])
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[title="10,388,595,898"]').exists()).toBe(false)
    expect(wrapper.find('[title="9,876,543"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('$71.56')
    expect(wrapper.text()).toContain('$12.34')

    await wrapper.get('button[title="admin.customFeatures.upstream.details"]').trigger('click')
    await flushPromises()
    const usageTab = wrapper.findAll('button').find((button) => button.text() === 'admin.customFeatures.upstream.detailTabs.usage')
    await usageTab?.trigger('click')
    expect(wrapper.get('[data-test="history-chart"]').attributes('data-datasets')).toBe('currency,currency')

    const platformSelect = wrapper.get('select[aria-label="admin.customFeatures.upstream.platform"]')
    await platformSelect.setValue('newapi')
    expect(wrapper.find('option[value="today_tokens_desc"]').exists()).toBe(false)
    expect(wrapper.find('option[value="today_tokens_asc"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('使用紧凑列宽和两行操作区完整展示右侧统计信息', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const table = wrapper.getComponent(DataTableStub)
    const columns = table.props('columns') as Array<{ key: string; class?: string }>
    expect(columns.find((column) => column.key === 'total')?.class).toContain('max-w-32')
    expect(columns.find((column) => column.key === 'last_synced_at')?.class).toContain('max-w-36')
    expect(columns.find((column) => column.key === 'actions')?.class).toContain('max-w-32')
    expect(table.props('expandableActions')).toBe(false)

    const actions = wrapper.get('[data-test="upstream-actions-1"]')
    expect(actions.classes()).toContain('grid-cols-3')
    expect(actions.findAll('a, button')).toHaveLength(6)
    expect(wrapper.get('[data-test="upstream-last-sync-1"]').classes()).toContain('whitespace-normal')
    wrapper.unmount()
  })

  it('按分组类型筛选站点并支持余额、今日 Token 排序', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([
      { id: 1, site_id: 1, remote_id: 'gpt', name: 'GPT', platform: 'OpenAI', description: '', multiplier: 0.2, today_tokens: 10, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' },
      { id: 2, site_id: 1, remote_id: 'claude', name: 'Claude', platform: 'Anthropic', description: '', multiplier: 0.1, today_tokens: 20, today_cost_usd: 0.2, displayed: true, available: true, last_synced_at: '' },
    ])
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('select[aria-label="admin.customFeatures.upstream.groupPlatform"]').setValue('Anthropic')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ group_platform: 'Anthropic' }))
    expect(wrapper.get('[data-test="expanded-groups-grid"]').text()).toContain('Claude')
    expect(wrapper.get('[data-test="expanded-groups-grid"]').text()).not.toContain('GPT')

    await wrapper.get('select[aria-label="admin.customFeatures.upstream.sortBy"]').setValue('today_tokens_desc')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ sort_by: 'today_tokens', sort_order: 'desc' }))
    wrapper.unmount()
  })

  it('按平台优先级和倍率升序整理账号下方分组，并可保存站点拖拽排序', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([
      { id: 1, site_id: 1, remote_id: 'high-anthropic', name: 'Claude 高', platform: 'Anthropic', description: '', multiplier: 0.5, today_tokens: 1, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' },
      { id: 2, site_id: 1, remote_id: 'openai', name: 'GPT', platform: 'OpenAI', description: '', multiplier: 0.9, today_tokens: 1, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' },
      { id: 3, site_id: 1, remote_id: 'low-anthropic', name: 'Claude 低', platform: 'Anthropic', description: '', multiplier: 0.1, today_tokens: 1, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' },
    ])
    const wrapper = mountPanel()
    await flushPromises()
    const headings = wrapper.get('[data-test="expanded-groups-grid"]').findAll('h4').map((heading) => heading.text())
    expect(headings).toEqual(['GPT', 'Claude 低', 'Claude 高'])

    await wrapper.get('[data-test="upstream-sort"]').trigger('click')
    await flushPromises()
    expect(api.listAll).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="upstream-sortable-sites"]').exists()).toBe(true)
    await wrapper.get('[data-test="dialog"] button.btn-primary').trigger('click')
    await flushPromises()
    expect(api.updateSortOrder).toHaveBeenCalledWith([
      { id: 1, sort_order: 0 },
      { id: 2, sort_order: 10 },
    ])
    wrapper.unmount()
  })

  it('支持多个站点独立展开，并缓存已加载分组', async () => {
    api.list.mockResolvedValue({
      items: [siteFixture({ displayed_group_count: 1 }), siteFixture({ id: 2, name: '上游二号', base_url: 'https://two.example.com', displayed_group_count: 1 })],
      total: 2, page: 1, page_size: 20, pages: 1,
    })
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '2026-07-15T00:00:00Z' }])
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="upstream-groups-2"]').exists()).toBe(true)
    expect(api.groups).toHaveBeenCalledWith(1)
    expect(api.groups).toHaveBeenCalledWith(2)

    await wrapper.get('[data-test="upstream-expand-1"]').trigger('click')
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(false)
    await wrapper.get('[data-test="upstream-expand-1"]').trigger('click')
    await flushPromises()
    expect(api.groups.mock.calls.filter(([id]) => id === 1)).toHaveLength(1)
    const gridClasses = wrapper.get('[data-test="expanded-groups-grid"]').classes()
    expect(gridClasses).toContain('lg:grid-cols-3')
    expect(gridClasses).toContain('2xl:grid-cols-4')
    wrapper.unmount()
  })

  it('点击账号下方生效倍率直接打开并选中对应倍率趋势', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '2026-07-15T00:00:00Z' }])
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="upstream-group-multiplier-1-vip"]').trigger('click')
    await flushPromises()

    expect(api.history).toHaveBeenCalledWith(1, 30)
    expect(api.multiplierHistory).toHaveBeenCalledWith(1, 30)
    expect(wrapper.get<HTMLSelectElement>('#upstream-multiplier-group').element.value).toBe('vip')
    const multiplierTab = wrapper.findAll('button').find((button) => button.text() === 'admin.customFeatures.upstream.detailTabs.multiplier')
    expect(multiplierTab?.classes()).toContain('border-primary-500')
    wrapper.unmount()
  })

  it('修正旧同步数据中被标记为 New API 的明确模型平台', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ platform: 'newapi', displayed_group_count: 2 })))
    api.groups.mockResolvedValue([
      { id: 1, site_id: 1, remote_id: 'claude-aws', name: 'Claude-AWS 99%高缓存', platform: 'New API', description: 'AWS 渠道', multiplier: 0.3, today_tokens: 293, today_cost_usd: 0.0027, displayed: true, available: true, last_synced_at: '' },
      { id: 2, site_id: 1, remote_id: 'cheap-gpt', name: '临时GPT低价分组', platform: 'New API', description: '稳定低价分组', multiplier: 0.02, today_tokens: 100, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' },
    ])
    api.multiplierHistory.mockResolvedValue([
      { remote_id: 'claude-aws', name: 'Claude-AWS 99%高缓存', platform: 'New API', description: 'AWS 渠道', current_multiplier: 0.3, points: [{ recorded_at: '2026-07-15T00:00:00Z', multiplier: 0.3 }] },
      { remote_id: 'cheap-gpt', name: '临时GPT低价分组', platform: 'New API', description: '稳定低价分组', current_multiplier: 0.02, points: [{ recorded_at: '2026-07-15T00:00:00Z', multiplier: 0.02 }] },
    ])
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('[data-test="upstream-group-platform-1-claude-aws"]').text()).toBe('Anthropic')
    expect(wrapper.get('[data-test="upstream-group-platform-1-cheap-gpt"]').text()).toBe('OpenAI')
    await wrapper.get('[data-test="upstream-group-multiplier-1-claude-aws"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="multiplier-history-platform"]').text()).toBe('Anthropic')
    wrapper.unmount()
  })

  it('站点尚未同步时首次展开仍会加载并缓存分组', async () => {
    api.list.mockResolvedValue({
      items: [siteFixture({ status: 'pending', last_synced_at: null, displayed_group_count: 1 })],
      total: 1, page: 1, page_size: 20, pages: 1,
    })
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '', multiplier: 1.5, today_tokens: 100, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '' }])
    const wrapper = mountPanel()
    await flushPromises()

    expect(api.groups).toHaveBeenCalledTimes(1)
    expect(api.groups).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('VIP')

    await wrapper.get('[data-test="upstream-expand-1"]').trigger('click')
    await wrapper.get('[data-test="upstream-expand-1"]').trigger('click')
    await flushPromises()
    expect(api.groups).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('忽略旧的列表成功响应，并保留最新数据的轮询节奏', async () => {
    vi.useFakeTimers()
    const stale = deferred<ReturnType<typeof siteListResult>>()
    const current = deferred<ReturnType<typeof siteListResult>>()
    api.list.mockReset()
      .mockReturnValueOnce(stale.promise)
      .mockReturnValueOnce(current.promise)
      .mockResolvedValue(siteListResult(siteFixture({ name: '轮询站点', status: 'healthy' })))
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('input[type="search"]').setValue('latest')
    await wrapper.get('input[type="search"]').trigger('keyup', { key: 'Enter' })
    current.resolve(siteListResult(siteFixture({ name: '最新站点', status: 'pending' })))
    await flushPromises()
    expect(wrapper.text()).toContain('最新站点')
    expect(wrapper.get('[data-test="data-table"]').attributes('data-loading')).toBe('false')

    stale.resolve(siteListResult(siteFixture({ name: '过期站点', status: 'healthy' })))
    await flushPromises()
    expect(wrapper.text()).toContain('最新站点')
    expect(wrapper.text()).not.toContain('过期站点')

    await vi.advanceTimersByTimeAsync(2_000)
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(3)
    wrapper.unmount()
  })

  it('轮询刷新不会重新展开本次会话手动收起的账号', async () => {
    vi.useFakeTimers()
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([{ id: 1, site_id: 1, remote_id: 'vip', name: 'VIP', platform: 'OpenAI', description: '', multiplier: 1.5, today_tokens: 1, today_cost_usd: 0.1, displayed: true, available: true, last_synced_at: '2026-07-15T00:00:00Z' }])
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(true)

    await wrapper.get('[data-test="upstream-expand-1"]').trigger('click')
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="upstream-groups-1"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('旧列表请求失败时不覆盖新请求的错误和 loading 状态', async () => {
    const stale = deferred<ReturnType<typeof siteListResult>>()
    const current = deferred<ReturnType<typeof siteListResult>>()
    api.list.mockReset().mockReturnValueOnce(stale.promise).mockReturnValueOnce(current.promise)
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('input[type="search"]').setValue('latest')
    await wrapper.get('input[type="search"]').trigger('keyup', { key: 'Enter' })
    stale.reject(new Error('stale failure'))
    await flushPromises()
    expect(wrapper.find('[data-test="data-table"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="data-table"]').attributes('data-loading')).toBe('true')

    current.resolve(siteListResult(siteFixture({ name: '最新站点' })))
    await flushPromises()
    expect(wrapper.text()).toContain('最新站点')
    expect(wrapper.get('[data-test="data-table"]').attributes('data-loading')).toBe('false')
    expect(wrapper.text()).not.toContain('admin.customFeatures.upstream.loadFailed')
    wrapper.unmount()
  })

  it('站点同步时间变化后刷新展开缓存，单站失败可重试', async () => {
    vi.useFakeTimers()
    api.list
      .mockResolvedValueOnce({ items: [siteFixture({ displayed_group_count: 1 })], total: 1, page: 1, page_size: 20, pages: 1 })
      .mockResolvedValue({ items: [siteFixture({ last_synced_at: '2026-07-15T01:00:00Z', displayed_group_count: 1 })], total: 1, page: 1, page_size: 20, pages: 1 })
    api.groups.mockRejectedValueOnce(new Error('network')).mockResolvedValue([])
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[data-test="upstream-groups-retry-1"]').exists()).toBe(true)
    await wrapper.get('[data-test="upstream-groups-retry-1"]').trigger('click')
    await flushPromises()
    expect(api.groups.mock.calls.filter(([id]) => id === 1)).toHaveLength(2)

    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()
    expect(api.groups.mock.calls.filter(([id]) => id === 1)).toHaveLength(3)
    wrapper.unmount()
  })

  it('展示分组描述和空倍率，切换倍率分组不会重复请求', async () => {
    api.groups.mockResolvedValue([
      { id: 1, site_id: 1, remote_id: 'free', name: 'Free', platform: 'OpenAI', description: '免费分组描述', multiplier: null, today_tokens: 10_388_595_898, today_cost_usd: 71.56, displayed: false, available: true, last_synced_at: '2026-07-15T00:00:00Z' },
    ])
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('button[title="admin.customFeatures.upstream.details"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('免费分组描述')
    expect(wrapper.text()).toContain('—')
    expect(wrapper.find('[title="10,388,595,898"]').exists()).toBe(true)

    const multiplierTab = wrapper.findAll('button').find((button) => button.text() === 'admin.customFeatures.upstream.detailTabs.multiplier')
    expect(multiplierTab).toBeTruthy()
    await multiplierTab!.trigger('click')
    await flushPromises()
    expect(api.multiplierHistory).toHaveBeenCalledTimes(1)
    expect(api.multiplierHistory).toHaveBeenCalledWith(1, 30)
    await wrapper.get('#upstream-multiplier-group').setValue('free')
    await flushPromises()
    expect(api.multiplierHistory).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('免费分组')
    expect(wrapper.find('[data-test="multiplier-history-chart"]').exists()).toBe(false)

    await wrapper.get('#upstream-multiplier-group').setValue('vip')
    await flushPromises()
    const multiplierChart = wrapper.get('[data-test="multiplier-history-chart"] [data-test="history-chart"]')
    expect(multiplierChart.attributes('data-stepped')).toBe('after')
    expect(multiplierChart.attributes('data-first-x')).toBe(String(new Date('2026-07-15T00:00:00Z').getTime()))

    const sevenDays = wrapper.findAll('button').find((button) => button.text() === '7d')
    await sevenDays!.trigger('click')
    await flushPromises()
    expect(api.multiplierHistory).toHaveBeenLastCalledWith(1, 7)
    wrapper.unmount()
  })

  it('快速切换详情站点时忽略先前站点的延迟响应', async () => {
    api.list.mockResolvedValue({
      items: [siteFixture(), siteFixture({ id: 2, name: '上游二号', base_url: 'https://two.example.com' })],
      total: 2, page: 1, page_size: 20, pages: 1,
    })
    const groupsA = deferred<Array<Record<string, unknown>>>()
    const groupsB = deferred<Array<Record<string, unknown>>>()
    const historyA = deferred<Array<Record<string, unknown>>>()
    const historyB = deferred<Array<Record<string, unknown>>>()
    api.groups.mockReset().mockReturnValueOnce(groupsA.promise).mockReturnValueOnce(groupsB.promise)
    api.history.mockReset().mockReturnValueOnce(historyA.promise).mockReturnValueOnce(historyB.promise)
    const wrapper = mountPanel()
    await flushPromises()

    const detailButtons = wrapper.findAll('button[title="admin.customFeatures.upstream.details"]')
    await detailButtons[0].trigger('click')
    await detailButtons[1].trigger('click')
    groupsB.resolve([{ id: 2, site_id: 2, remote_id: 'b', name: 'B 当前分组', platform: 'New API', description: '', multiplier: 1, today_tokens: 1, today_cost_usd: 0.1, displayed: false, available: true, last_synced_at: '' }])
    historyB.resolve([])
    await flushPromises()
    expect(wrapper.text()).toContain('B 当前分组')

    groupsA.resolve([{ id: 1, site_id: 1, remote_id: 'a', name: 'A 延迟分组', platform: 'OpenAI', description: '', multiplier: 1, today_tokens: 1, today_cost_usd: 0.1, displayed: false, available: true, last_synced_at: '' }])
    historyA.resolve([])
    await flushPromises()
    expect(wrapper.text()).toContain('B 当前分组')
    expect(wrapper.text()).not.toContain('A 延迟分组')
    wrapper.unmount()
  })

  it('从已展示分组跨本地分组暂存账号并统一保存绑定', async () => {
    const upstreamGroup = displayedGroupFixture()
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([upstreamGroup])
    bindingAPIs.getGroups.mockResolvedValue([
      { id: 10, name: '本地 OpenAI', status: 'active' },
      { id: 20, name: '本地 Anthropic', status: 'active' },
      { id: 30, name: '停用分组', status: 'inactive' },
    ])
    bindingAPIs.listAccounts.mockImplementation((_page: number, _pageSize: number, filters: { group: string }) => Promise.resolve({
      items: filters.group === '20'
        ? [{ id: 202, name: '账号二', platform: 'anthropic', status: 'active', priority: 20 }]
        : [{ id: 101, name: '账号一', platform: 'openai', status: 'active', priority: 10 }],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    }))
    const updatedGroup = displayedGroupFixture({
      bindings: [bindingFixture(), bindingFixture({ id: 2, local_group_id: 20, local_group_name: '本地 Anthropic', account_id: 202, account_name: '账号二', account_platform: 'anthropic', account_priority: 15 })],
    })
    api.replaceGroupBindings.mockResolvedValue(updatedGroup)

    const wrapper = mountPanel()
    await flushPromises()

    const bindingButton = wrapper.get('[data-test="upstream-group-bindings-1-vip"]')
    expect(bindingButton.text()).toContain('0')
    await bindingButton.trigger('click')
    await flushPromises()

    expect(bindingAPIs.getGroups).toHaveBeenCalledOnce()
    expect(bindingAPIs.listAccounts).toHaveBeenCalledWith(1, 10, { group: '10', search: '' })
    expect(wrapper.text()).toContain('admin.customFeatures.upstream.bindings.globalPriorityWarning')

    await wrapper.get('#upstream-binding-account-search').setValue('账号一')
    await wrapper.get('#upstream-binding-account-search').trigger('keyup', { key: 'Enter' })
    await flushPromises()
    expect(bindingAPIs.listAccounts).toHaveBeenLastCalledWith(1, 10, { group: '10', search: '账号一' })

    await wrapper.get('[data-test="add-upstream-binding-101"]').trigger('click')
    await wrapper.get('#upstream-binding-local-group').setValue('20')
    await flushPromises()
    expect(bindingAPIs.listAccounts).toHaveBeenLastCalledWith(1, 10, { group: '20', search: '' })
    await wrapper.get('[data-test="add-upstream-binding-202"]').trigger('click')
    await wrapper.get('[data-test="save-upstream-bindings"]').trigger('click')
    await flushPromises()

    expect(api.replaceGroupBindings).toHaveBeenCalledWith(1, 1, [
      { local_group_id: 10, account_id: 101 },
      { local_group_id: 20, account_id: 202 },
    ])
    expect(wrapper.get('[data-test="upstream-group-bindings-1-vip"]').text()).toContain('2')
    wrapper.unmount()
  })

  it('打开绑定弹窗前刷新服务端最新绑定', async () => {
    const staleGroup = displayedGroupFixture()
    const latestGroup = displayedGroupFixture({ bindings: [bindingFixture()] })
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1, binding_count: 1 })))
    api.groups.mockReset().mockResolvedValueOnce([staleGroup]).mockResolvedValueOnce([latestGroup])
    bindingAPIs.getGroups.mockResolvedValue([{ id: 10, name: '本地 OpenAI', status: 'active' }])

    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.get('[data-test="upstream-group-bindings-1-vip"]').text()).toContain('0')

    await wrapper.get('[data-test="upstream-group-bindings-1-vip"]').trigger('click')
    await flushPromises()

    expect(api.groups).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="remove-upstream-binding-101"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('忽略被后续站点打开操作抢占的绑定刷新', async () => {
    const firstOpen = deferred<Array<Record<string, unknown>>>()
    let firstSiteCalls = 0
    api.list.mockResolvedValue({
      items: [
        siteFixture({ id: 1, displayed_group_count: 1 }),
        siteFixture({ id: 2, name: '上游二号', base_url: 'https://two.example.com', displayed_group_count: 1 }),
      ],
      total: 2, page: 1, page_size: 20, pages: 1,
    })
    api.groups.mockImplementation((siteID: number) => {
      if (siteID === 1) {
        firstSiteCalls++
        if (firstSiteCalls > 1) return firstOpen.promise
        return Promise.resolve([displayedGroupFixture({ id: 1, site_id: 1, remote_id: 'first', multiplier: 1 })])
      }
      return Promise.resolve([displayedGroupFixture({ id: 2, site_id: 2, remote_id: 'second', multiplier: 2 })])
    })

    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="upstream-group-bindings-1-first"]').trigger('click')
    await wrapper.get('[data-test="upstream-group-bindings-2-second"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="upstream-bindings-dialog"]').text()).toContain('2×')

    firstOpen.resolve([displayedGroupFixture({ id: 1, site_id: 1, remote_id: 'first', multiplier: 1 })])
    await flushPromises()
    expect(wrapper.get('[data-test="upstream-bindings-dialog"]').text()).toContain('2×')
    wrapper.unmount()
  })

  it('弹窗打开后服务端绑定变化时关闭旧草稿', async () => {
    const initialGroup = displayedGroupFixture()
    const changedGroup = displayedGroupFixture({ bindings: [bindingFixture()] })
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockReset()
      .mockResolvedValueOnce([initialGroup])
      .mockResolvedValueOnce([initialGroup])
      .mockResolvedValueOnce([changedGroup])

    const wrapper = mountPanel()
    await flushPromises()
    const button = wrapper.get('[data-test="upstream-group-bindings-1-vip"]')
    await button.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="upstream-bindings-dialog"]').exists()).toBe(true)

    await wrapper.get('[data-test="upstream-group-bindings-1-vip"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="upstream-bindings-dialog"]').exists()).toBe(false)
    expect(api.showError).toHaveBeenCalledWith('admin.customFeatures.upstream.bindings.dataChanged')
    wrapper.unmount()
  })

  it('首次账号请求未完成时不锁定本地分组选择器', async () => {
    const accountsRequest = deferred<{ items: never[]; total: number; page: number; page_size: number; pages: number }>()
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([displayedGroupFixture()])
    bindingAPIs.getGroups.mockResolvedValue([{ id: 10, name: '本地 OpenAI', status: 'active' }])
    bindingAPIs.listAccounts.mockReturnValue(accountsRequest.promise)

    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="upstream-group-bindings-1-vip"]').trigger('click')
    await flushPromises()

    const selector = wrapper.get('#upstream-binding-local-group').element as HTMLSelectElement
    expect(selector.disabled).toBe(false)

    accountsRequest.resolve({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
    await flushPromises()
    wrapper.unmount()
  })

  it('上游分组不可用时只允许解除已有绑定', async () => {
    const upstreamGroup = displayedGroupFixture({ available: false, bindings: [bindingFixture()] })
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1, binding_count: 1 })))
    api.groups.mockResolvedValue([upstreamGroup])
    api.replaceGroupBindings.mockResolvedValue(displayedGroupFixture({ available: false, bindings: [] }))
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="upstream-group-bindings-1-vip"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="upstream-bindings-frozen"]').exists()).toBe(true)
    expect(wrapper.find('#upstream-binding-local-group').exists()).toBe(false)
    expect(bindingAPIs.getGroups).not.toHaveBeenCalled()

    await wrapper.get('[data-test="remove-upstream-binding-101"]').trigger('click')
    await wrapper.get('[data-test="save-upstream-bindings"]').trigger('click')
    await flushPromises()
    expect(api.replaceGroupBindings).toHaveBeenCalledWith(1, 1, [])
    wrapper.unmount()
  })

  it('账号已绑定其他上游分组时保留弹窗并显示冲突提示', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ displayed_group_count: 1 })))
    api.groups.mockResolvedValue([displayedGroupFixture()])
    bindingAPIs.getGroups.mockResolvedValue([{ id: 10, name: '本地 OpenAI', status: 'active' }])
    bindingAPIs.listAccounts.mockResolvedValue({
      items: [{ id: 101, name: '账号一', platform: 'openai', status: 'active', priority: 10 }],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })
    api.replaceGroupBindings.mockRejectedValue({ reason: 'UPSTREAM_BINDING_CONFLICT' })
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="upstream-group-bindings-1-vip"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="add-upstream-binding-101"]').trigger('click')
    await wrapper.get('[data-test="save-upstream-bindings"]').trigger('click')
    await flushPromises()

    expect(api.showError).toHaveBeenCalledWith('admin.customFeatures.upstream.bindings.conflict')
    expect(wrapper.find('[data-test="upstream-bindings-dialog"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('删除站点时提示将同步解除的账号绑定数量', async () => {
    api.list.mockResolvedValue(siteListResult(siteFixture({ binding_count: 3 })))
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('button[title="admin.customFeatures.upstream.delete"]').trigger('click')
    const warning = wrapper.get('[data-test="upstream-delete-binding-warning"]')
    expect(warning.text()).toContain('3')
    wrapper.unmount()
  })
})
