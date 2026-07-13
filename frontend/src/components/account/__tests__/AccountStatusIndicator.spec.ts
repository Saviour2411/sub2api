import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountStatusIndicator from '../AccountStatusIndicator.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatCountdown: () => '1h'
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'antigravity',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

describe('AccountStatusIndicator', () => {
  it('Grok 账号额度限流时显示自动恢复时间而非临时不可调度', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 5,
          name: 'grok-free-1',
          platform: 'grok',
          rate_limited_at: '2026-07-11T12:00:00Z',
          rate_limit_reset_at: '2099-07-11T13:00:00Z',
          temp_unschedulable_until: '2099-07-11T12:30:00Z',
          temp_unschedulable_reason: 'legacy grok rate limited'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('.badge-warning').text()).toBe('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('admin.accounts.status.rateLimitedAutoResume')
    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
  })

  it('模型限流 + overages 启用 + 无 AICredits key → 显示 ⚡ (credits_active)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 1,
          name: 'ag-1',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('⚡')
    expect(wrapper.text()).toContain('CSon45')
  })

  it('模型限流 + overages 未启用 → 普通限流样式（无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 2,
          name: 'ag-2',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
  })

  it('AICredits key 生效 → 显示积分已用尽 (credits_exhausted)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 3,
          name: 'ag-3',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('模型限流 + overages 启用 + AICredits key 生效 → 普通限流样式（积分耗尽，无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 4,
          name: 'ag-4',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              },
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    // 模型限流 + 积分耗尽 → 不应显示 ⚡
    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
    // AICredits 积分耗尽状态应显示
    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('活动账号带持久停调度 marker 时显示不可调度和结构化原因', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          schedulable: false,
          extra: {
            failure_strategy_unscheduled: {
              source: 'first_token_timeout',
              reason: 'first token timeout after 25 seconds',
              status_code: 504,
              consecutive_count: 3,
              threshold: 3,
              model: 'gpt-5.6-terra',
              at: '2026-07-12T08:30:00Z',
              incident_id: 'incident-1',
              timeout_seconds: 25
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.unschedulable')
    expect(wrapper.text()).not.toContain('admin.accounts.status.paused')
    expect(wrapper.get('[data-test="account-unschedulable-reason-value"]').text()).toBe(
      'first token timeout after 25 seconds'
    )
    expect(wrapper.get('[data-test="account-unschedulable-status-code"]').text()).toBe('504')
    expect(wrapper.get('[data-test="account-unschedulable-consecutive-count"]').text()).toBe('3')
    expect(wrapper.get('[data-test="account-unschedulable-threshold"]').text()).toBe('3')
    expect(wrapper.get('[data-test="account-unschedulable-model"]').text()).toBe('gpt-5.6-terra')
    expect(wrapper.get('[data-test="account-unschedulable-timeout-seconds"]').text()).toBe('25')
    expect(wrapper.get('[data-test="account-unschedulable-occurred-at"]').text()).toContain('2026')
  })

  it('持久事故主状态优先于残留的临时限流状态', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          schedulable: false,
          rate_limit_reset_at: new Date(Date.now() + 60_000).toISOString(),
          temp_unschedulable_until: new Date(Date.now() + 60_000).toISOString(),
          extra: {
            failure_strategy_unscheduled: {
              source: 'upstream_error',
              reason: '连续上游错误'
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('.badge').text()).toBe('admin.accounts.status.unschedulable')
    expect(wrapper.find('button.badge').exists()).toBe(false)
  })

  it('旧 marker 使用原始 reason 作为不可调度原因', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          schedulable: false,
          extra: {
            failure_strategy_unscheduled: {
              reason: 'legacy upstream failure',
              status_code: 503,
              at: '2026-07-12T08:30:00Z'
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-test="account-unschedulable-reason-value"]').text()).toBe(
      'legacy upstream failure'
    )
    expect(wrapper.get('[data-test="account-unschedulable-status-code"]').text()).toBe('503')
  })

  it('marker 字段异常时保留原始 reason 并忽略无效详情', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          schedulable: false,
          extra: {
            failure_strategy_unscheduled: {
              source: ['unexpected'],
              reason: 'legacy raw reason',
              status_code: '503',
              consecutive_count: Number.NaN,
              at: 'invalid-date'
            }
          } as Account['extra']
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('.badge').text()).toBe('admin.accounts.status.unschedulable')
    expect(wrapper.get('[data-test="account-unschedulable-reason-value"]').text()).toBe(
      'legacy raw reason'
    )
    expect(wrapper.find('[data-test="account-unschedulable-status-code"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="account-unschedulable-consecutive-count"]').exists()).toBe(
      false
    )
    expect(wrapper.find('[data-test="account-unschedulable-occurred-at"]').exists()).toBe(false)
  })

  it('管理员手动暂停且没有 marker 时仍显示暂停且不显示原因问号', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({ schedulable: false, extra: {} })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.paused')
    expect(wrapper.text()).not.toContain('admin.accounts.status.unschedulable')
    expect(wrapper.find('[data-test="account-unschedulable-reason"]').exists()).toBe(false)
  })
})
