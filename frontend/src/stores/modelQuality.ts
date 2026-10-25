import { defineStore } from 'pinia'
import { markRaw, ref, watch } from 'vue'
import type { QualityGroup, QualityPage } from '@/api/modelQuality'
import { useAuthStore } from './auth'
import { useAppStore } from './app'
import { createModelQualityLoader } from '@/utils/modelQualityLoader'

export interface QualityGallerySnapshot {
  key: string
  groups: QualityGroup[]
  result: QualityPage
  anchorId?: number
  anchorTop?: number
  scrollY: number
}
export const useModelQualityStore = defineStore('modelQuality', () => {
  const auth = useAuthStore()
  const app = useAppStore()
  const loader = markRaw(createModelQualityLoader())
  const generation = ref(0)
  const gallery = ref<QualityGallerySnapshot | null>(null)
  loader.subscribe((id) => {
    if (id !== null) return
    gallery.value = null
    generation.value++
  })
  watch(
    [() => auth.user?.id, () => auth.isAuthenticated],
    () => loader.clear(),
    { flush: 'sync' }
  )
  watch(
    () => app.cachedPublicSettings?.model_quality_enabled,
    (enabled) => {
      if (enabled === false) loader.clear()
    },
    { flush: 'sync' }
  )
  return { loader, generation, gallery }
})
