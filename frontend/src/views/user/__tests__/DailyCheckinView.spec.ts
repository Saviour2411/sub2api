import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import DailyCheckinView from '../DailyCheckinView.vue'

const {
  dailyCheckin,
  getDailyCheckinStatus,
  refreshUser,
  showError,
  showInfo,
  showSuccess
} = vi.hoisted(() => ({
  dailyCheckin: vi.fn(),
  getDailyCheckinStatus: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn()
}))

const messages: Record<string, string> = {
  'dailyCheckin.balance': '当前余额',
  'dailyCheckin.done': '今日已签',
  'dailyCheckin.empty': '暂无记录',
  'dailyCheckin.factor': '专属奖池',
  'dailyCheckin.kicker': '每日福利',
  'dailyCheckin.loading': '加载中',
  'dailyCheckin.days': '天',
  'dailyCheckin.prizePool': '奖品列表',
  'dailyCheckin.ready': '今日可签到',
  'dailyCheckin.recent': '最近记录',
  'dailyCheckin.result': '中奖结果',
  'dailyCheckin.spin': '开始抽奖',
  'dailyCheckin.spinning': '抽奖中',
  'dailyCheckin.title': '每日签到',
  'dailyCheckin.types.balance': '余额',
  'dailyCheckin.types.concurrency': '并发',
  'dailyCheckin.types.subscription': '订阅',
  'dailyCheckin.types.none': '谢谢参与'
}

const prizes = [
  { id: 'p1', name: '余额1刀', type: 'balance', sort_order: 1, balance_mode: 'fixed', amount: 1 },
  { id: 'p2', name: '谢谢参与', type: 'none', sort_order: 2 },
  { id: 'p3', name: '余额10刀', type: 'balance', sort_order: 3, balance_mode: 'fixed', amount: 10 },
  { id: 'p4', name: '奖励+1.3刀', type: 'concurrency', sort_order: 4, concurrency: 1 }
]

const reward = {
  prize_id: 'p3',
  prize_name: '余额10刀',
  type: 'balance',
  amount: 10,
  checked_in_at: '2026-06-02T00:00:00Z'
}

function status(overrides: Record<string, unknown> = {}) {
  return {
    enabled: true,
    checked_in_today: false,
    reward_mode: 'fixed',
    reward_amount: 1,
    reward_min: 1,
    reward_max: 3,
    prizes,
    recent_records: [],
    ...overrides
  }
}

function mountView() {
  return mount(DailyCheckinView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: { template: '<span />' }
      }
    }
  })
}

vi.mock('@/api', () => ({
  redeemAPI: {
    dailyCheckin,
    getDailyCheckinStatus
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showInfo,
    showSuccess
  }),
  useAuthStore: () => ({
    refreshUser,
    user: { balance: 0 }
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'dailyCheckin.success') return `签到成功：${params?.reward || ''}`
        return messages[key] ?? key
      }
    })
  }
})

describe('DailyCheckinView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    dailyCheckin.mockReset()
    getDailyCheckinStatus.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showInfo.mockReset()
    showSuccess.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('为每个奖品标签提供明确的反向旋转角度', async () => {
    getDailyCheckinStatus.mockResolvedValue(status())

    const wrapper = mountView()
    await flushPromises()

    const labels = wrapper.findAll('.wheel-label')
    expect(labels).toHaveLength(prizes.length)
    for (const label of labels) {
      const style = label.attributes('style') || ''
      expect(style).toContain('--label-angle:')
      expect(style).toContain('--label-counter-angle:')
      expect(style).not.toContain('* -1')
    }
  })

  it('等转盘动画结束后再显示成功通知和中奖结果', async () => {
    getDailyCheckinStatus
      .mockResolvedValueOnce(status())
      .mockResolvedValue(status({
        checked_in_today: true,
        today_result: reward
      }))
    dailyCheckin.mockResolvedValue({
      reward_amount: 10,
      new_balance: 10,
      checked_in_at: reward.checked_in_at,
      prize: reward,
      prizes
    })
    refreshUser.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.spin-button').trigger('click')
    await flushPromises()

    expect(showSuccess).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('中奖结果')

    await vi.advanceTimersByTimeAsync(3300)
    await flushPromises()

    expect(showSuccess).toHaveBeenCalledWith('签到成功：余额10刀 $10.00')
    expect(wrapper.text()).toContain('中奖结果')
    expect(wrapper.text()).toContain('余额10刀')
    expect(wrapper.text()).toContain('余额10刀 $10.00')
  })

  it('自定义奖品名不会重复追加奖励数值', async () => {
    const customConcurrency = {
      prize_id: 'p4',
      prize_name: '永久并发+1',
      type: 'concurrency',
      concurrency: 6,
      checked_in_at: '2026-06-02T00:00:00Z'
    }
    getDailyCheckinStatus.mockResolvedValue(status({
      checked_in_today: true,
      today_result: customConcurrency,
      recent_records: [
        customConcurrency,
        {
          id: 5,
          prize_id: 'p5',
          prize_name: '余额10刀',
          type: 'balance',
          amount: 10,
          checked_in_at: '2026-06-01T00:00:00Z'
        },
        {
          id: 6,
          prize_id: 'p6',
          prize_name: '月卡30天',
          type: 'subscription',
          validity_days: 30,
          checked_in_at: '2026-05-31T00:00:00Z'
        }
      ]
    }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('永久并发+1')
    expect(wrapper.text()).not.toContain('永久并发+1 +6')
    expect(wrapper.text()).toContain('余额10刀')
    expect(wrapper.text()).toContain('余额10刀 $10.00')
    expect(wrapper.text()).toContain('月卡30天')
    expect(wrapper.text()).not.toContain('月卡30天 30天')
  })

  it('默认奖品名仍会追加奖励数值', async () => {
    getDailyCheckinStatus.mockResolvedValue(status({
      checked_in_today: true,
      today_result: {
        prize_id: 'p4',
        prize_name: '并发奖励',
        type: 'concurrency',
        concurrency: 1,
        checked_in_at: '2026-06-02T00:00:00Z'
      },
      recent_records: [
        {
          id: 7,
          prize_id: 'p1',
          prize_name: '余额奖励',
          type: 'balance',
          amount: 2,
          checked_in_at: '2026-06-01T00:00:00Z'
        },
        {
          id: 8,
          prize_id: 'p2',
          prize_name: '订阅奖励',
          type: 'subscription',
          validity_days: 7,
          checked_in_at: '2026-05-31T00:00:00Z'
        }
      ]
    }))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('并发奖励 +1')
    expect(wrapper.text()).toContain('余额奖励 $2.00')
    expect(wrapper.text()).toContain('订阅奖励 7天')
  })

  it('签到失败时立即显示错误通知', async () => {
    getDailyCheckinStatus.mockResolvedValue(status())
    dailyCheckin.mockRejectedValue({ message: 'internal error' })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.spin-button').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('dailyCheckin.failed: internal error')
  })
})
