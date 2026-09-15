import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import { BalanceQueryError, type PublicBalanceQuery } from '@/api/balanceQuery'
import BalanceQueryView from '../BalanceQueryView.vue'

const mocks = vi.hoisted(() => ({ query: vi.fn(), route: { hash: '' } }))
vi.mock('@/api/balanceQuery', async (original) => ({ ...await original<typeof import('@/api/balanceQuery')>(), queryPublicBalance: mocks.query }))
vi.mock('vue-router', () => ({ useRoute: () => mocks.route }))
vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const result: PublicBalanceQuery = {
  user: { id: 7, username: '', balance: 123.12345678 },
  payment_methods: [{ key: '币安 ID', value: '<script>仅纯文本</script>' }],
  records: [{ created_at: '2026-09-14T00:00:00Z', amount: 100, type: 'temporary_credit', note: '临时授信100' }],
  page: 1, page_size: 20, total: 1
}

describe('公开余额查询页', () => {
  let wrapper: VueWrapper | undefined
  beforeEach(() => {
    mocks.route = reactive({ hash: '#' + 'A'.repeat(43) })
    mocks.query.mockReset().mockResolvedValue(structuredClone(result))
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.useRealTimers() })
  const open = async () => { wrapper = mount(BalanceQueryView); await flushPromises(); return wrapper }

  it('展示最小信息、ID 回退、纯文本充值方式和安全授信备注', async () => {
    const view = await open()
    expect(view.text()).toContain('#7')
    expect(view.text()).toContain('123.12345678')
    expect(view.text()).toContain('临时授信100')
    expect(view.text()).toContain('<script>仅纯文本</script>')
    expect(view.find('script').exists()).toBe(false)
    expect(document.querySelector('meta[name="referrer"]')?.getAttribute('content')).toBe('no-referrer')
    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toContain('noindex')
  })

  it('只在打开、手动刷新及翻页时请求，不自动轮询', async () => {
    vi.useFakeTimers()
    const view = await open()
    await vi.advanceTimersByTimeAsync(120000)
    expect(mocks.query).toHaveBeenCalledTimes(1)
    await view.get('[aria-label="balanceQuery.refresh"]').trigger('click')
    await flushPromises()
    expect(mocks.query).toHaveBeenCalledTimes(2)
  })

  it.each([[404, 'unavailable'], [429, 'rateLimited'], [503, 'serviceUnavailable']])('正确展示 %i 状态', async (status, key) => {
    mocks.query.mockRejectedValue(new BalanceQueryError(Number(status), 12))
    const view = await open()
    expect(view.get('[role="alert"]').text()).toContain(`balanceQuery.${key}`)
    if (status === 429) expect(view.get('button').attributes('disabled')).toBeDefined()
  })

  it('无效片段不发请求，切换链接不会显示旧用户数据', async () => {
    const view = await open()
    mocks.route.hash = '#无效'
    await nextTick()
    expect(view.text()).not.toContain('123.12345678')
    expect(view.text()).toContain('balanceQuery.unavailable')
    expect(mocks.query).toHaveBeenCalledTimes(1)
  })

  it('展示空记录并支持分页', async () => {
    mocks.query.mockResolvedValue({ ...result, records: [], total: 21 })
    const view = await open()
    expect(view.text()).toContain('balanceQuery.empty')
    await view.get('[aria-label="common.next"]').trigger('click')
    expect(mocks.query).toHaveBeenLastCalledWith('A'.repeat(43), 2, expect.any(AbortSignal))
  })
})
