import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import UserCustomizationsPanel from '../UserCustomizationsPanel.vue'
import type { UserCustomization } from '@/api/admin/userCustomizations'

const mocks = vi.hoisted(() => ({
  api: { list: vi.fn(), paymentMethods: vi.fn(), savePaymentMethods: vi.fn(), save: vi.fn(), getLink: vi.fn(), rotateLink: vi.fn(), setLinkEnabled: vi.fn(), restoreCredit: vi.fn() },
  app: { showError: vi.fn(), showSuccess: vi.fn() }, clipboard: vi.fn()
}))
vi.mock('@/api/admin/userCustomizations', () => ({ default: mocks.api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks.app }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string, args?: unknown) => key + (args ? JSON.stringify(args) : '') }) }))
const user: UserCustomization = { user_id: 7, username: '用户甲', balance: 50, status: 'active', has_link: true, link_enabled: true, link_version: 3, auto_credit_enabled: true, credit_threshold: 1000, credit_amount: 200, credit_generation: 2, credit_used_at: '2026-09-14T00:00:00Z' }
const dialogs = {
  BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>' },
  ConfirmDialog: { props: ['show', 'message'], emits: ['confirm', 'cancel'], template: '<div v-if="show" data-test="confirm">{{ message }}<button data-test="confirm-action" @click="$emit(\'confirm\')">确认</button></div>' }
}

describe('用户定制管理', () => {
  let wrapper: VueWrapper | undefined
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.api.list.mockResolvedValue({ items: [structuredClone(user)], total: 1 })
    mocks.api.paymentMethods.mockResolvedValue([{ key: '方式甲', value: '甲' }, { key: '方式乙', value: '乙' }])
    mocks.api.savePaymentMethods.mockImplementation(async (methods) => methods)
    mocks.api.getLink.mockResolvedValue('https://example.test/balance-query#随机码')
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: mocks.clipboard } })
  })
  afterEach(() => wrapper?.unmount())
  const open = async () => { wrapper = mount(UserCustomizationsPanel, { global: { stubs: dialogs } }); await flushPromises(); return wrapper }

  it('充值方式可排序和保存，内容按键值对提交', async () => {
    const view = await open()
    await view.findAll('[aria-label="userCustomization.moveDown"]')[0].trigger('click')
    const save = view.findAll('button').find(button => button.text() === 'common.save')!
    await save.trigger('click')
    await flushPromises()
    expect(mocks.api.savePaymentMethods).toHaveBeenCalledWith([{ key: '方式乙', value: '乙' }, { key: '方式甲', value: '甲' }])
  })

  it('加载失败时禁止用空数组覆盖全局充值方式', async () => {
    mocks.api.paymentMethods.mockRejectedValue(new Error('失败'))
    const view = await open()
    expect(view.findAll('button').find(button => button.text() === 'common.save')!.attributes('disabled')).toBeDefined()
    expect(mocks.api.savePaymentMethods).not.toHaveBeenCalled()
  })

  it('复制、停用和重置链接，不触发授信恢复', async () => {
    const view = await open()
    await view.get('[aria-label="userCustomization.copyLink"]').trigger('click')
    await flushPromises()
    expect(mocks.clipboard).toHaveBeenCalledWith('https://example.test/balance-query#随机码')
    await view.findAll('button').find(button => button.text() === 'userCustomization.disableLink')!.trigger('click')
    await flushPromises()
    expect(mocks.api.setLinkEnabled).toHaveBeenCalledWith(user, false)
    await view.findAll('button').find(button => button.text() === 'userCustomization.resetLink')!.trigger('click')
    expect(mocks.api.rotateLink).not.toHaveBeenCalled()
    await view.get('[data-test="confirm-action"]').trigger('click')
    await flushPromises()
    expect(mocks.api.rotateLink).toHaveBeenCalledWith(user)
    expect(mocks.api.restoreCredit).not.toHaveBeenCalled()
  })

  it('恢复确认显示目标用户、阈值和单次金额并阻止重复点击', async () => {
    const view = await open()
    await view.findAll('button').find(button => button.text() === 'userCustomization.restoreCredit')!.trigger('click')
    const message = view.get('[data-test="confirm"]').text()
    expect(message).toContain('用户甲')
    expect(message).toContain('1,000.00')
    expect(message).toContain('200.00')
    let resolve!: () => void
    mocks.api.restoreCredit.mockReturnValue(new Promise<void>(done => { resolve = done }))
    await view.get('[data-test="confirm-action"]').trigger('click')
    await view.get('[data-test="confirm-action"]').trigger('click')
    expect(mocks.api.restoreCredit).toHaveBeenCalledTimes(1)
    resolve()
    await flushPromises()
    expect(view.find('[data-test="confirm"]').exists()).toBe(false)
  })

  it('普通保存不提交只读授信状态和链接字段', async () => {
    const view = await open()
    await view.get('[aria-label="userCustomization.configure"]').trigger('click')
    await view.get('#user-credit-settings').trigger('submit')
    await flushPromises()
    expect(mocks.api.save).toHaveBeenCalledWith(7, { auto_credit_enabled: true, credit_threshold: 1000, credit_amount: 200 })
    expect(mocks.api.restoreCredit).not.toHaveBeenCalled()
  })
})
