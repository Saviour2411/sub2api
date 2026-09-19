import { describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ default: { get } }))

import api from '@/api/admin/userCustomizations'

describe('用户定制搜索接口', () => {
  it('传递完整搜索词、页码并保留邮箱字段', async () => {
    const data = { items: [{ user_id: 1054, username: '', email: 'boxinsmart@example.test' }], total: 1 }
    get.mockResolvedValue({ data })
    await expect(api.list('box', 2)).resolves.toEqual(data)
    expect(get).toHaveBeenCalledTimes(1)
    expect(get).toHaveBeenCalledWith('/admin/custom-features/user-customizations', {
      params: { search: 'box', page: 2, page_size: 20 }
    })
  })
})
