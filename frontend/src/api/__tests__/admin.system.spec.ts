import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))

import systemAPI, * as system from '@/api/admin/system'

describe('只读系统版本接口', () => {
  beforeEach(() => vi.clearAllMocks())

  it('读取当前版本，不检查上游发布', async () => {
    get.mockResolvedValue({ data: { version: '0.1.241' } })
    await expect(systemAPI.getVersion()).resolves.toEqual({ version: '0.1.241' })
    expect(get).toHaveBeenCalledTimes(1)
    expect(get).toHaveBeenCalledWith('/admin/system/version')
  })

  it('不导出更新、回滚或重启操作', () => {
    expect(Object.keys(systemAPI)).toEqual(['getVersion', 'getUpstreamVersion'])
    expect(Object.keys(system).sort()).toEqual(['default', 'getUpstreamVersion', 'getVersion', 'systemAPI'])
  })

  it('只读检查使用独立接口并传递取消信号', async () => {
    const signal = new AbortController().signal
    const result = { latest_version: '0.2.8', has_update: true, status: 'ok', stale: false }
    get.mockResolvedValue({ data: result })
    await expect(systemAPI.getUpstreamVersion(signal)).resolves.toEqual(result)
    expect(get).toHaveBeenCalledWith('/admin/system/upstream-version', { signal })
  })
})
