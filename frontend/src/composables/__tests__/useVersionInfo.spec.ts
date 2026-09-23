import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { useVersionInfo } from '../useVersionInfo'

const { getVersion, getUpstreamVersion } = vi.hoisted(() => ({ getVersion: vi.fn(), getUpstreamVersion: vi.fn() }))
vi.mock('@/api/admin/system', () => ({ getVersion, getUpstreamVersion }))

describe('只读版本信息生命周期', () => {
  let wrappers: VueWrapper[] = []
  let hidden = false
  const latest = () => ({ latest_version: '0.2.8', release_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.8', has_update: true, checked_at: new Date().toISOString(), status: 'ok', stale: false })

  function create(adminValue = true) {
    const admin = ref(adminValue)
    const identity = ref<string | number | undefined>(1)
    let state!: ReturnType<typeof useVersionInfo>
    const wrapper = mount(defineComponent({
      setup() { state = useVersionInfo(admin, identity); return () => h('div') }
    }))
    wrappers.push(wrapper)
    return { state, admin, identity, wrapper }
  }

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
    hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    getVersion.mockReset().mockResolvedValue({ version: '0.1.244', upstream_version: '0.2.7' })
    getUpstreamVersion.mockReset().mockImplementation(async () => latest())
  })

  afterEach(() => {
    wrappers.forEach(wrapper => wrapper.unmount())
    wrappers = []
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('普通用户不读取管理员版本接口', async () => {
    const { state } = create(false)
    await flushPromises()
    expect(state.versionInfo.value).toBeNull()
    expect(getVersion).not.toHaveBeenCalled()
    expect(getUpstreamVersion).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('当前构建与同步版本不等待外部查询', async () => {
    let resolve!: (value: ReturnType<typeof latest>) => void
    getUpstreamVersion.mockReturnValue(new Promise(done => { resolve = done }))
    const { state } = create()
    await flushPromises()
    expect(state.versionInfo.value?.version).toBe('0.1.244')
    expect(state.versionInfo.value?.upstream_version).toBe('0.2.7')
    expect(state.checking.value).toBe(true)
    resolve(latest())
    await flushPromises()
    expect(state.checking.value).toBe(false)
    expect(state.upstream.value?.has_update).toBe(true)
  })

  it('页面可见时每三十分钟检查，后台暂停且恢复后补查', async () => {
    create()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(30 * 60 * 1000)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(2)
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(vi.getTimerCount()).toBe(0)
    await vi.advanceTimersByTimeAsync(60 * 60 * 1000)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(2)
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(getUpstreamVersion).toHaveBeenCalledTimes(3)
    expect(getVersion).toHaveBeenCalledTimes(1)
  })

  it('首次处于后台时不检查，恢复后仍尊重有效缓存', async () => {
    hidden = true
    create()
    expect(getUpstreamVersion).not.toHaveBeenCalled()
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(1000)
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(getUpstreamVersion).toHaveBeenCalledTimes(1)
  })

  it('检查失败保留成功信息并退避六十秒', async () => {
    const { state } = create()
    await flushPromises()
    getUpstreamVersion.mockRejectedValueOnce(new Error('网络中断'))
    await vi.advanceTimersByTimeAsync(30 * 60 * 1000)
    expect(state.upstream.value).toMatchObject({ latest_version: '0.2.8', status: 'unavailable', stale: true })
    await vi.advanceTimersByTimeAsync(59999)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(state.upstream.value?.status).toBe('ok')
    expect(getUpstreamVersion).toHaveBeenCalledTimes(3)
  })

  it('浏览器时钟领先服务器时不会每秒重复检查有效缓存', async () => {
    getUpstreamVersion.mockResolvedValue({ ...latest(), checked_at: '2026-09-23T11:00:00Z' })
    create()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(29 * 60 * 1000)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(60 * 1000)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(2)
  })

  it('无成功历史时失败不会表示已经最新', async () => {
    getUpstreamVersion.mockRejectedValue(new Error('请求失败'))
    const { state } = create()
    await flushPromises()
    expect(state.upstream.value).toMatchObject({ has_update: null, status: 'unavailable', stale: false })
  })

  it('降为普通用户后中止请求并忽略迟到结果', async () => {
    let resolve!: (value: ReturnType<typeof latest>) => void
    getUpstreamVersion.mockReturnValue(new Promise(done => { resolve = done }))
    const { state, admin } = create()
    const signal = getUpstreamVersion.mock.calls[0][0] as AbortSignal
    admin.value = false
    await nextTick()
    expect(signal.aborted).toBe(true)
    resolve(latest())
    await flushPromises()
    expect(state.upstream.value).toBeNull()
    expect(state.versionInfo.value).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('切换管理员身份与卸载都会清理原请求和计时器', async () => {
    const { identity, wrapper } = create()
    await flushPromises()
    const signal = getUpstreamVersion.mock.calls[0][0] as AbortSignal
    identity.value = 2
    await nextTick()
    await flushPromises()
    expect(signal.aborted).toBe(true)
    expect(getVersion).toHaveBeenCalledTimes(2)
    expect(getUpstreamVersion).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    wrappers = []
    expect(vi.getTimerCount()).toBe(0)
  })
})
