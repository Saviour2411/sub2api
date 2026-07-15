import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import UpstreamManagementPanel from '../UpstreamManagementPanel.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), update: vi.fn(), setEnabled: vi.fn(), remove: vi.fn(),
  sync: vi.fn(), syncAll: vi.fn(), groups: vi.fn(), setGroupDisplayed: vi.fn(), history: vi.fn(), multiplierHistory: vi.fn(),
  showSuccess: vi.fn(), showError: vi.fn(),
}))

vi.mock('@/api/admin/upstreams', () => ({ default: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: api.showSuccess, showError: api.showError }) }))
vi.mock('@/utils/apiError', () => ({ extractApiErrorMessage: (_error: unknown, fallback: string) => fallback }))
vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div data-test="history-chart" :data-stepped="data.datasets?.[0]?.stepped || \'\'" :data-first-x="data.datasets?.[0]?.data?.[0]?.x ?? \'\'" />',
  },
}))
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
  props: {
    data: { type: Array, default: () => [] },
    expandedRowKeys: { type: Array, default: () => [] },
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
        <slot name="cell-actions" :row="row" />
        <slot name="cell-name" :row="row" />
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
    tracking_started_at: '2026-07-01T00:00:00Z',
    last_synced_at: '2026-07-15T00:00:00Z',
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-15T00:00:00Z',
    has_password: true,
    has_token: false,
    displayed_group_count: 0,
    ...overrides,
  }
}

function siteListResult(site = siteFixture()) {
  return { items: [site], total: 1, page: 1, page_size: 20, pages: 1 }
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
    api.list.mockResolvedValue({ items: [siteFixture()], total: 1, page: 1, page_size: 20, pages: 1 })
    api.create.mockResolvedValue(siteFixture({ id: 2 }))
    api.update.mockResolvedValue(siteFixture())
    api.setEnabled.mockResolvedValue(siteFixture({ enabled: false }))
    api.remove.mockResolvedValue(undefined)
    api.sync.mockResolvedValue(undefined)
    api.syncAll.mockResolvedValue({ queued: 1 })
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
    expect(wrapper.get('[data-test="expanded-groups-grid"]').classes()).toContain('md:grid-cols-2')
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
})
