import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RedeemView from '../RedeemView.vue'

const { getHistory, getPublicSettings, refreshUser } = vi.hoisted(() => ({
  getHistory: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    redeem: vi.fn()
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
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions: vi.fn()
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
  })
})
