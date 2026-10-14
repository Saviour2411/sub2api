import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import QualityPreview from '../QualityPreview.vue'

const detail = vi.hoisted(() => vi.fn())
vi.mock('@/api/modelQuality', () => ({ modelQualityAPI: { detail } }))
let intersection: (entries: Partial<IntersectionObserverEntry>[]) => void
let wrapper: ReturnType<typeof mount> | undefined
beforeEach(() => {
  detail.mockReset()
  detail.mockResolvedValue({ tests: [{ benchmark: 'pelican', html: '<canvas></canvas>' }] })
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(callback: typeof intersection) {
        intersection = callback
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
  vi.unstubAllGlobals()
})
describe('鹈鹕隔离预览', () => {
  it('进入视口才请求作品，离屏卸载 iframe', async () => {
    wrapper = mount(QualityPreview, { props: { runId: 5 } })
    expect(detail).not.toHaveBeenCalled()
    expect(wrapper.find('iframe').exists()).toBe(false)
    intersection([{ isIntersecting: true }])
    await flushPromises()
    expect(detail).toHaveBeenCalledOnce()
    expect(wrapper.get('iframe').attributes('sandbox')).toBe('allow-scripts')
    expect(wrapper.get('iframe').attributes('src')).toBe('/api/v1/model-quality/preview')
    expect(wrapper.get('iframe').attributes()).not.toHaveProperty('srcdoc')
    intersection([{ isIntersecting: false }])
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
  })
  it('作品过期时不执行内容', async () => {
    detail.mockRejectedValue({ status: 404 })
    wrapper = mount(QualityPreview, { props: { runId: 5 } })
    intersection([{ isIntersecting: true }])
    await flushPromises()
    expect(wrapper.text()).toContain('作品已过期或暂不可用')
    expect(wrapper.find('iframe').exists()).toBe(false)
  })
})
