import { reactive } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useModelQualityStore } from '../modelQuality'
import { qualityRun } from '@/__tests__/fixtures/modelQuality'

const fetch = vi.hoisted(() => vi.fn())
const auth = reactive({
  user: { id: 1 } as { id: number } | null,
  isAuthenticated: true
})
const app = reactive({ cachedPublicSettings: { model_quality_enabled: true } })
vi.mock('@/api/modelQuality', () => ({ modelQualityAPI: { detail: fetch } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
beforeEach(() => {
  auth.user = { id: 1 }
  auth.isAuthenticated = true
  app.cachedPublicSettings.model_quality_enabled = true
  setActivePinia(createPinia())
  fetch.mockReset()
  fetch.mockResolvedValue(qualityRun())
})
afterEach(() => {
  useModelQualityStore().loader.clear()
  useModelQualityStore().$dispose()
})
describe('检测会话缓存隔离', () => {
  it('同一账号刷新资料时保留有效缓存', async () => {
    const store = useModelQualityStore()
    await store.loader.load(9)
    const generation = store.generation
    auth.user = { id: 1 }
    expect(store.generation).toBe(generation)
    await store.loader.load(9)
    expect(fetch).toHaveBeenCalledTimes(1)
  })
  it.each(['退出登录', '切换账号', '关闭功能'])(
    '%s立即清空正文及画廊快照',
    async (action) => {
      const store = useModelQualityStore()
      await store.loader.load(9)
      store.gallery = {
        key: '测试',
        groups: [],
        result: { items: [], page: 1, pages: 1, page_size: 12, total: 0 },
        scrollY: 200
      }
      const generation = store.generation
      if (action === '退出登录') auth.isAuthenticated = false
      else if (action === '切换账号') auth.user = { id: 2 }
      else app.cachedPublicSettings.model_quality_enabled = false
      expect(store.generation).toBeGreaterThan(generation)
      expect(store.gallery).toBeNull()
      await store.loader.load(9)
      expect(fetch).toHaveBeenCalledTimes(2)
    }
  )
})
