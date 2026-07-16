import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({
  put: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: { put },
}))

import { replaceGroupBindings, type UpstreamGroup } from '@/api/admin/upstreams'

describe('admin upstream bindings API', () => {
  beforeEach(() => {
    put.mockReset()
  })

  it('原子替换指定上游分组的账号绑定', async () => {
    const group = {
      id: 23,
      site_id: 7,
      remote_id: 'vip',
      name: 'VIP',
      platform: 'OpenAI',
      description: '',
      multiplier: 0.5,
      today_tokens: 0,
      today_cost_usd: 0,
      displayed: true,
      available: true,
      last_synced_at: '2026-07-16T00:00:00Z',
      bindings: [],
    } satisfies UpstreamGroup
    put.mockResolvedValue({ data: group })

    const bindings = [
      { local_group_id: 3, account_id: 11 },
      { local_group_id: 5, account_id: 12 },
    ]

    await expect(replaceGroupBindings(7, 23, bindings)).resolves.toEqual(group)
    expect(put).toHaveBeenCalledWith(
      '/admin/custom-features/upstreams/7/groups/23/bindings',
      { bindings }
    )
  })
})
