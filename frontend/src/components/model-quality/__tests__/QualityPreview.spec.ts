import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import QualityPreview from '../QualityPreview.vue'
import { useModelQualityStore } from '@/stores/modelQuality'
import { qualityRun, deferred } from '@/__tests__/fixtures/modelQuality'
import type { QualityRun } from '@/api/modelQuality'

const detail = vi.hoisted(() => vi.fn())
vi.mock('@/api/modelQuality', () => ({ modelQualityAPI: { detail } }))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 1 }, isAuthenticated: true })
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: { model_quality_enabled: true } })
}))
type Intersect = (entries: Partial<IntersectionObserverEntry>[]) => void
let observers: { callback: Intersect; options: IntersectionObserverInit }[]
let wrapper: ReturnType<typeof mount> | undefined
const intersect = async (index: number, visible: boolean) => {
  observers[index].callback([{ isIntersecting: visible }])
  await flushPromises()
}
beforeEach(() => {
  vi.useFakeTimers()
  setActivePinia(createPinia())
  detail.mockReset()
  detail.mockResolvedValue(qualityRun())
  observers = []
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(callback: Intersect, options: IntersectionObserverInit) {
        observers.push({ callback, options })
      }
      observe() {}
      disconnect() {}
    }
  )
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    }
  )
})
afterEach(() => {
  wrapper?.unmount()
  useModelQualityStore().loader.clear()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
describe('鹈鹕隔离预览', () => {
  it('400px 内预加载，进入视口才创建 iframe，离屏再进入复用正文', async () => {
    wrapper = mount(QualityPreview, { props: { runId: 9 } })
    expect(detail).not.toHaveBeenCalled()
    expect(observers[1].options.rootMargin).toBe('400px 0px')
    await intersect(1, true)
    expect(detail).toHaveBeenCalledTimes(1)
    expect(wrapper.find('iframe').exists()).toBe(false)
    await intersect(0, true)
    expect(detail).toHaveBeenCalledTimes(1)
    expect(wrapper.get('iframe').attributes('sandbox')).toBe('allow-scripts')
    expect(wrapper.get('iframe').attributes('src')).toBe(
      '/api/v1/model-quality/preview'
    )
    expect(wrapper.get('iframe').attributes()).not.toHaveProperty('srcdoc')
    await intersect(0, false)
    expect(wrapper.find('iframe').exists()).toBe(false)
    await intersect(0, true)
    expect(detail).toHaveBeenCalledTimes(1)
  })
  it('只接受当前 iframe 的握手，相同内容持续可见时不重建或重复发送', async () => {
    wrapper = mount(QualityPreview, {
      props: { runId: 9 },
      attachTo: document.body
    })
    await intersect(0, true)
    const frame = wrapper.get('iframe').element as HTMLIFrameElement
    const post = vi.spyOn(frame.contentWindow!, 'postMessage')
    window.dispatchEvent(
      new MessageEvent('message', {
        source: window,
        data: { type: 'quality-ready' }
      })
    )
    expect(post).not.toHaveBeenCalled()
    window.dispatchEvent(
      new MessageEvent('message', {
        source: frame.contentWindow,
        data: { type: 'quality-ready' }
      })
    )
    await intersect(0, true)
    window.dispatchEvent(
      new MessageEvent('message', {
        source: frame.contentWindow,
        data: { type: 'quality-ready' }
      })
    )
    expect(post).toHaveBeenCalledTimes(1)
    expect(wrapper.get('iframe').element).toBe(frame)
  })
  it('浏览器后台卸载执行实例，返回前台后恢复且不重复请求', async () => {
    wrapper = mount(QualityPreview, { props: { runId: 9 } })
    await intersect(0, true)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(true)
    expect(detail).toHaveBeenCalledTimes(1)
  })
  it('过期后清除执行实例，不因再次进入视口恢复', async () => {
    detail.mockResolvedValue(
      qualityRun(9, { expires_at: new Date(Date.now() + 1000).toISOString() })
    )
    wrapper = mount(QualityPreview, { props: { runId: 9 } })
    await intersect(0, true)
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(wrapper.text()).toContain('作品已过期或暂不可用')
    expect(wrapper.find('iframe').exists()).toBe(false)
    await intersect(0, false)
    await intersect(0, true)
    expect(detail).toHaveBeenCalledTimes(1)
  })
  it('清理后的迟到响应不恢复正文或执行实例', async () => {
    const pending = deferred<QualityRun>()
    detail.mockReturnValue(pending.promise)
    wrapper = mount(QualityPreview, { props: { runId: 9 } })
    await intersect(0, true)
    useModelQualityStore().loader.clear()
    pending.resolve(qualityRun())
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).toContain('作品已过期或暂不可用')
  })
  it('超出已有正文大小上限时不执行', async () => {
    wrapper = mount(QualityPreview, {
      props: { runId: 9, html: 'x'.repeat(5242881) }
    })
    await intersect(0, true)
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(detail).not.toHaveBeenCalled()
  })
})
