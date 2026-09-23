import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import VersionBadge from '../VersionBadge.vue'
import zh from '@/i18n/locales/zh/custom'

const { getVersion, getUpstreamVersion } = vi.hoisted(() => ({ getVersion: vi.fn(), getUpstreamVersion: vi.fn() }))
vi.mock('@/api/admin/system', () => ({ getVersion, getUpstreamVersion }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params: Record<string, unknown> = {}) => {
  const text = zh.versionInfo[key.split('.')[1] as keyof typeof zh.versionInfo] || key
  return text.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
} }) }))

describe('二开与上游版本展示', () => {
  let wrapper: VueWrapper | undefined
  beforeEach(() => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    getVersion.mockReset().mockResolvedValue({ version: '0.1.244', upstream_version: '0.2.7', upstream_commit: 'a'.repeat(40), upstream_synced_at: '2026-09-20T23:09:09+08:00' })
    getUpstreamVersion.mockReset().mockResolvedValue({ latest_version: '0.2.8', release_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8', has_update: true, checked_at: new Date().toISOString(), status: 'ok', stale: false })
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks(); document.body.innerHTML = '' })

  it('普通用户只看到当前二开版本，不检查上游', async () => {
    wrapper = mount(VersionBadge, { props: { version: 'v0.1.244' } })
    expect(wrapper.text()).toBe('v0.1.244')
    await wrapper.setProps({ version: '0.1.245' })
    expect(wrapper.text()).toBe('v0.1.245')
    expect(wrapper.find('a, button').exists()).toBe(false)
    expect(getVersion).not.toHaveBeenCalled()
    expect(getUpstreamVersion).not.toHaveBeenCalled()
  })

  it('管理员展示三类版本且没有更新操作按钮', async () => {
    wrapper = mount(VersionBadge, { props: { version: '0.1.244', admin: true, identity: 1 } })
    await flushPromises()
    expect(wrapper.text()).toContain('二开v0.1.244')
    expect(wrapper.text()).toContain('已同步原版v0.2.7')
    expect(wrapper.text()).toContain('原版新版本 v0.2.8')
    expect(wrapper.get('a').attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('没有版本时明确显示未知，缺失同步信息不猜测', async () => {
    getVersion.mockResolvedValue({ version: '' })
    wrapper = mount(VersionBadge, { props: { admin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('版本未知')
    expect(wrapper.text()).toContain('未记录')
  })

  it('检查失败仍保留当前与同步版本', async () => {
    getUpstreamVersion.mockRejectedValue(new Error('检查失败'))
    wrapper = mount(VersionBadge, { props: { version: '0.1.244', admin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('v0.1.244')
    expect(wrapper.text()).toContain('v0.2.7')
    expect(wrapper.text()).toContain('版本检查暂不可用')
    expect(wrapper.text()).not.toContain('暂无新版本')
  })

  it('陈旧结果保留更新提示并明确标记缓存', async () => {
    getUpstreamVersion.mockResolvedValue({ latest_version: '0.2.8', release_url: 'javascript:alert(1)', has_update: true, status: 'unavailable', stale: true })
    wrapper = mount(VersionBadge, { props: { admin: true } })
    await flushPromises()
    expect(wrapper.text()).toContain('原版新版本 v0.2.8')
    expect(wrapper.text()).toContain('（缓存）')
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('折叠侧栏可打开详情并通过 Escape 关闭', async () => {
    wrapper = mount(VersionBadge, { props: { version: '0.1.244', admin: true, collapsed: true }, attachTo: document.body })
    await flushPromises()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain('v0.2.7')
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })
})
