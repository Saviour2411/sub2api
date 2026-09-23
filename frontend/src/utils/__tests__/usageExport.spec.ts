import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { exportUsagePages, isExportCanceled } from '../usageExport'

describe('使用记录游标导出', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('跨越 60 页仍按游标读取，不依赖列表总数', async () => {
    let page = 0
    const load = vi.fn(async (_cursor: string, _signal: AbortSignal) => ({ items: [++page], next_cursor: page < 61 ? String(1000 - page) : '' }))
    const consume = vi.fn()
    const start = Date.now()
    const result = exportUsagePages({ load, consume, signal: new AbortController().signal })
    await vi.runAllTimersAsync()
    expect(await result).toBe(61)
    expect(load).toHaveBeenCalledTimes(61)
    expect(consume).toHaveBeenCalledTimes(61)
    expect(Date.now() - start).toBeGreaterThanOrEqual(60 * 1250)
    expect(load.mock.calls[1][0]).toBe('999')
  })

  it('429 按 Retry-After 重试相同游标且不重复写入', async () => {
    const load = vi.fn()
      .mockRejectedValueOnce({ status: 429, retryAfter: '2' })
      .mockResolvedValue({ items: [1], next_cursor: '' })
    const consume = vi.fn()
    const result = exportUsagePages({ load, consume, signal: new AbortController().signal })
    await vi.advanceTimersByTimeAsync(2000)
    expect(load).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(250)
    expect(await result).toBe(1)
    expect(load.mock.calls.map(call => call[0])).toEqual(['', ''])
    expect(consume).toHaveBeenCalledTimes(1)
    expect(consume).toHaveBeenCalledWith([1])
  })

  it('等待重试期间取消会立即停止并清理计时器', async () => {
    const controller = new AbortController()
    const load = vi.fn().mockRejectedValue({ status: 429 })
    const result = exportUsagePages({ load, consume: vi.fn(), signal: controller.signal }).catch(error => error)
    await vi.advanceTimersByTimeAsync(0)
    controller.abort()
    expect(isExportCanceled(await result)).toBe(true)
    expect(vi.getTimerCount()).toBe(0)
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('永久错误不重试，重复游标不无限循环', async () => {
    const load = vi.fn().mockRejectedValue({ status: 403 })
    await expect(exportUsagePages({ load, consume: vi.fn(), signal: new AbortController().signal })).rejects.toEqual({ status: 403 })
    expect(load).toHaveBeenCalledTimes(1)
    load.mockResolvedValue({ items: [1], next_cursor: '10' })
    const result = exportUsagePages({ load, consume: vi.fn(), signal: new AbortController().signal }).catch(error => error)
    await vi.runAllTimersAsync()
    expect(await result).toMatchObject({ message: '导出接口返回了无效游标' })
  })

  it('网络错误最多重试五次，失败时不写入空批次', async () => {
    const load = vi.fn().mockRejectedValue({ status: 0, code: 'ERR_NETWORK' })
    const consume = vi.fn()
    const result = exportUsagePages({ load, consume, signal: new AbortController().signal }).catch(error => error)
    await vi.runAllTimersAsync()
    expect(await result).toMatchObject({ code: 'ERR_NETWORK' })
    expect(load).toHaveBeenCalledTimes(6)
    expect(consume).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('大于安全整数的游标保持字符串精度', async () => {
    const load = vi.fn()
      .mockResolvedValueOnce({ items: [1], next_cursor: '9223372036854775806' })
      .mockResolvedValueOnce({ items: [2], next_cursor: '9223372036854775805' })
      .mockResolvedValueOnce({ items: [3], next_cursor: '' })
    const result = exportUsagePages({ load, consume: vi.fn(), signal: new AbortController().signal })
    await vi.runAllTimersAsync()
    expect(await result).toBe(3)
    expect(load.mock.calls.map(call => call[0])).toEqual(['', '9223372036854775806', '9223372036854775805'])
  })
})
