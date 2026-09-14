import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import RedeemView from '../RedeemView.vue'

const { getHistory, getPublicSettings, refreshUser, redeem, fetchActiveSubscriptions, showError, showWarning, showSuccess } = vi.hoisted(() => ({
  getHistory: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn(),
  redeem: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    redeem
  },
  authAPI: {
    getPublicSettings
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: {
      balance: 12.34,
      concurrency: 2
    },
    refreshUser
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showWarning,
    showSuccess
  })
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const messages: Record<string, string> = {
          'redeem.currentBalance': '当前余额',
          'redeem.concurrency': '并发',
          'redeem.requests': '请求',
          'redeem.redeemCodeLabel': '兑换码',
          'redeem.redeemCodePlaceholder': '请输入兑换码',
          'redeem.redeemCodeHint': '兑换码区分大小写',
          'redeem.redeemButton': '兑换',
          'redeem.aboutCodes': '关于兑换码',
          'redeem.codeRule1': '规则1',
          'redeem.codeRule2': '规则2',
          'redeem.codeRule3': '规则3',
          'redeem.codeRule4': '规则4',
          'redeem.recentActivity': '最近活动',
          'redeem.balanceAddedDailyCheckin': '每日签到奖励',
          'redeem.dailyCheckinReward': '签到奖励',
          'redeem.adminAdjustment': '管理员调整'
        }
        return messages[key] ?? key
      }
    })
  }
})

function mountView() {
  return mount(RedeemView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: { template: '<span />' }
      }
    }
  })
}

describe('RedeemView', () => {
  beforeEach(() => {
    getHistory.mockReset()
    getPublicSettings.mockReset().mockResolvedValue({ contact_info: '' })
    refreshUser.mockReset()
  })

  it('shows daily check-in balance rewards in recent activity without pseudo code', async () => {
    getHistory.mockResolvedValue([
      {
        id: -1000000000007,
        code: 'CHK-7',
        type: 'daily_checkin_balance',
        value: 2.37,
        status: 'used',
        used_at: '2026-06-07T12:00:00Z',
        created_at: '2026-06-07T12:00:00Z'
      }
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('每日签到奖励')
    expect(wrapper.text()).toContain('+$2.37')
    expect(wrapper.text()).toContain('签到奖励')
    expect(wrapper.text()).not.toContain('CHK-7')
    wrapper.unmount()
  })
})

async function submitCode() {
  const wrapper = mount(RedeemView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
  })
  await flushPromises()
  await wrapper.get('input#code').setValue(' REDEEM-CODE ')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  return wrapper
}

describe('RedeemView refresh after redemption', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getPublicSettings.mockResolvedValue({ contact_info: '' })
    redeem.mockResolvedValue({ type: 'balance', value: 20, message: 'Code applied' })
    getHistory.mockResolvedValue([])
    refreshUser.mockResolvedValue({ balance: 30, concurrency: 2 })
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it.each(['balance', 'concurrency', 'subscription'])(
    'keeps a successful %s redemption when profile refresh fails', async (type) => {
      redeem.mockResolvedValue({ type, value: 20, message: 'Code applied' })
      refreshUser.mockRejectedValue({ status: 503, message: 'Service unavailable' })
      getHistory.mockResolvedValueOnce([]).mockResolvedValueOnce([{
        id: 1, code: 'REDEEM-CODE', type, value: 20, used_at: '2026-03-08T00:00:00Z',
      }])

      const wrapper = await submitCode()

      expect(redeem).toHaveBeenCalledWith('REDEEM-CODE')
      expect(showError).not.toHaveBeenCalled()
      expect(showWarning).toHaveBeenCalledWith('redeem.userRefreshFailed')
      expect(showSuccess).toHaveBeenCalledWith('redeem.codeRedeemSuccess')
      expect(wrapper.text()).toContain('Code applied')
      expect(wrapper.text()).not.toContain('redeem.failedToRedeem')
      expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('')
      expect((wrapper.get('input#code').element as HTMLInputElement).disabled).toBe(false)
      expect(getHistory).toHaveBeenCalledTimes(2)
      expect(wrapper.text()).toContain('REDEEM-C...')
      if (type === 'subscription') {
        expect(fetchActiveSubscriptions).toHaveBeenCalledWith(true)
      } else {
        expect(fetchActiveSubscriptions).not.toHaveBeenCalled()
      }
      wrapper.unmount()
    }
  )

  it('finishes normally without a warning when profile refresh succeeds', async () => {
    const wrapper = await submitCode()

    expect(refreshUser).toHaveBeenCalledOnce()
    expect(showWarning).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('redeem.codeRedeemSuccess')
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })

  it('preserves the existing subscription refresh warning after successful redemption', async () => {
    redeem.mockResolvedValue({ type: 'subscription', value: 20, message: 'Code applied' })
    fetchActiveSubscriptions.mockRejectedValue(new Error('Network Error'))
    const wrapper = await submitCode()

    expect(showWarning).toHaveBeenCalledWith('redeem.subscriptionRefreshFailed')
    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('redeem.codeRedeemSuccess')
    expect(getHistory).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('keeps the code and reports failure when the redemption request itself fails', async () => {
    redeem.mockRejectedValue({ response: { data: { detail: 'Invalid code' } } })
    const wrapper = await submitCode()

    expect(showError).toHaveBeenCalledWith('redeem.redeemFailed')
    expect(wrapper.text()).toContain('Invalid code')
    expect(wrapper.text()).not.toContain('Code applied')
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe(' REDEEM-CODE ')
    expect(refreshUser).not.toHaveBeenCalled()
    expect(fetchActiveSubscriptions).not.toHaveBeenCalled()
    expect(getHistory).toHaveBeenCalledOnce()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showWarning).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
