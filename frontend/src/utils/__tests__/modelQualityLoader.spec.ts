import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import {
  createModelQualityLoader,
  scheduleQualityExpiry
} from '../modelQualityLoader'
import { deferred, qualityRun } from '@/__tests__/fixtures/modelQuality'
import type { QualityRun } from '@/api/modelQuality'

vi.mock('@/api/modelQuality', () => ({ modelQualityAPI: { detail: vi.fn() } }))
let loader: ReturnType<typeof createModelQualityLoader>
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-25T02:00:00Z'))
})
afterEach(() => {
  loader?.clear()
  vi.useRealTimers()
})

describe('共享作品加载器', () => {
  it('合并同记录请求并复用完成缓存', async () => {
    const pending = deferred<QualityRun>(),
      fetch = vi.fn(() => pending.promise)
    loader = createModelQualityLoader({ fetch })
    const first = loader.load(9),
      second = loader.load(9, { priority: 'detail' })
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    pending.resolve(qualityRun())
    expect(await first).toEqual(await second)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(1)
  })
  it('最多四个并发，排队详情和可见作品优先于预加载', async () => {
    const requests = new Map<number, ReturnType<typeof deferred<QualityRun>>>()
    const fetch = vi.fn((id: number) => {
      const pending = deferred<QualityRun>()
      requests.set(id, pending)
      return pending.promise
    })
    loader = createModelQualityLoader({ fetch })
    const first = [1, 2, 3, 4].map((id) => loader.load(id))
    await flushPromises()
    const prefetch = loader.load(5, { priority: 'prefetch' })
    const visible = loader.load(6)
    const detail = loader.load(7, { priority: 'detail' })
    await flushPromises()
    expect(fetch.mock.calls.map((call) => call[0])).toEqual([1, 2, 3, 4])
    requests.get(1)!.resolve(qualityRun(1))
    await flushPromises()
    expect(fetch.mock.lastCall?.[0]).toBe(7)
    requests.get(2)!.resolve(qualityRun(2))
    await flushPromises()
    expect(fetch.mock.lastCall?.[0]).toBe(6)
    requests.get(3)!.resolve(qualityRun(3))
    await flushPromises()
    expect(fetch.mock.lastCall?.[0]).toBe(5)
    for (const [id, request] of requests) request.resolve(qualityRun(id))
    await Promise.all([...first, prefetch, visible, detail])
  })
  it('取消一个等待者不影响其他等待者，已发出请求可完成缓存', async () => {
    const pending = deferred<QualityRun>(),
      fetch = vi.fn(() => pending.promise)
    loader = createModelQualityLoader({ fetch })
    const a = new AbortController(),
      b = new AbortController()
    const first = loader.load(9, { signal: a.signal }).catch((e) => e)
    const second = loader.load(9, { signal: b.signal })
    await flushPromises()
    a.abort()
    expect((await first).name).toBe('AbortError')
    expect(fetch.mock.calls[0]).toBeDefined()
    pending.resolve(qualityRun())
    expect((await second).id).toBe(9)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(1)
  })
  it('排队请求无订阅者时取消，清理后迟到响应不能回填', async () => {
    const pending = deferred<QualityRun>(),
      fetch = vi.fn(() => pending.promise)
    loader = createModelQualityLoader({ fetch, concurrency: 1 })
    const first = loader.load(9).catch((e) => e)
    await flushPromises()
    const controller = new AbortController()
    const queued = loader
      .load(10, { signal: controller.signal })
      .catch((e) => e)
    controller.abort()
    expect((await queued).name).toBe('AbortError')
    loader.clear()
    expect((await first).name).toBe('AbortError')
    pending.resolve(qualityRun())
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(1)
    fetch.mockResolvedValue(qualityRun())
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(2)
  })
  it('默认缓存最多24条并按最近访问淘汰', async () => {
    const fetch = vi.fn(async (id: number) => qualityRun(id))
    loader = createModelQualityLoader({ fetch })
    for (let id = 1; id <= 24; id++) await loader.load(id)
    await loader.load(1)
    await loader.load(25)
    await loader.load(1)
    expect(fetch).toHaveBeenCalledTimes(25)
    await loader.load(2)
    expect(fetch).toHaveBeenCalledTimes(26)
  })
  it('正文总预算与超大单条不突破容量限制', async () => {
    const run = qualityRun(),
      size = JSON.stringify(run).length * 2
    const fetch = vi.fn(async (id: number) => qualityRun(id))
    loader = createModelQualityLoader({ fetch, maxBytes: size + 10 })
    await loader.load(9)
    await loader.load(8)
    await loader.load(8)
    expect(fetch).toHaveBeenCalledTimes(2)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(3)
    loader.clear()
    loader = createModelQualityLoader({ fetch, maxBytes: size - 1 })
    await loader.load(9)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(5)
  })
  it('五分钟后重新读取，记录到期时通知卸载且拒绝正文', async () => {
    const fetch = vi.fn(async () => qualityRun())
    loader = createModelQualityLoader({ fetch })
    await loader.load(9)
    await vi.advanceTimersByTimeAsync(299999)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(2)
    const expire = new Date(Date.now() + 1000).toISOString(),
      notify = vi.fn()
    loader.subscribe(notify)
    fetch.mockResolvedValue(qualityRun(10, { expires_at: expire }))
    await loader.load(10)
    await vi.advanceTimersByTimeAsync(1000)
    expect(notify).toHaveBeenCalledWith(10, 'expired')
    await expect(loader.load(10)).rejects.toMatchObject({ status: 410 })
  })
  it('进行中不缓存，状态或完成时间改变使缓存失效', async () => {
    const running = qualityRun(9, { outcome: 'running', finished_at: null })
    const fetch = vi.fn(async () => running)
    loader = createModelQualityLoader({ fetch })
    await loader.load(9)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(2)
    fetch.mockResolvedValue(qualityRun())
    loader.sync([qualityRun()])
    await loader.load(9)
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(3)
    loader.sync([qualityRun(9, { finished_at: '2026-10-25T01:02:00Z' })])
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(4)
  })
  it.each([401, 403])('权限错误%d清空其他缓存', async (status) => {
    const fetch = vi.fn(async () => qualityRun())
    loader = createModelQualityLoader({ fetch })
    await loader.load(9)
    fetch.mockRejectedValueOnce({ status })
    await expect(loader.load(10)).rejects.toBeDefined()
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(3)
  })
  it('分组移除立即失效，旧分组的迟到响应不写入缓存', async () => {
    const pending = deferred<QualityRun>(),
      fetch = vi.fn(() => pending.promise)
    loader = createModelQualityLoader({ fetch })
    const request = loader.load(9)
    await flushPromises()
    loader.setGroups([8])
    pending.resolve(qualityRun())
    await expect(request).rejects.toMatchObject({ status: 410 })
    loader.setGroups([7])
    await loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(2)
  })
  it('超长到期时间不会溢出为立即失效', async () => {
    const expire = vi.fn(),
      cancel = scheduleQualityExpiry('2099-01-01T00:00:00Z', expire)
    await vi.advanceTimersByTimeAsync(2147483648)
    expect(expire).not.toHaveBeenCalled()
    cancel()
  })
})
