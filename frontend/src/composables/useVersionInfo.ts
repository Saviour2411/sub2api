import { onMounted, onScopeDispose, ref, watch, type Ref } from 'vue'
import { getUpstreamVersion, getVersion, type SystemVersionInfo, type UpstreamVersionInfo } from '@/api/admin/system'

const CHECK_INTERVAL = 30 * 60 * 1000
const FAILURE_BACKOFF = 60 * 1000

export function useVersionInfo(admin: Ref<boolean>, identity: Ref<string | number | undefined>) {
  const versionInfo = ref<SystemVersionInfo | null>(null)
  const upstream = ref<UpstreamVersionInfo | null>(null)
  const checking = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined
  let generation = 0
  let nextCheckAt = 0
  let loadingVersion = false
  let disposed = false

  function clearTimer() {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }

  function schedule() {
    clearTimer()
    if (disposed || !admin.value || document.hidden || checking.value) return
    timer = setTimeout(resume, Math.max(1000, nextCheckAt - Date.now()))
  }

  async function loadVersion(current: number) {
    if (loadingVersion || versionInfo.value) return
    loadingVersion = true
    try {
      const result = await getVersion(controller?.signal)
      if (generation === current && !disposed) versionInfo.value = result
    } catch {
      // 当前版本仍可从公共设置展示；下一次可见检查时补查来源信息。
    } finally {
      if (generation === current) loadingVersion = false
    }
  }

  async function checkUpstream(current: number) {
    if (checking.value) return
    checking.value = true
    try {
      const result = await getUpstreamVersion(controller?.signal)
      if (generation !== current || disposed) return
      upstream.value = result
      const checked = result.checked_at ? Date.parse(result.checked_at) : NaN
      const now = Date.now()
      const expires = checked + CHECK_INTERVAL
      // 服务端缓存仍有效但浏览器时钟领先时，避免到期时间落在过去而反复查询。
      nextCheckAt = result.status === 'ok'
        ? Math.min(now + CHECK_INTERVAL, Number.isFinite(expires) && expires > now ? expires : now + CHECK_INTERVAL)
        : now + FAILURE_BACKOFF
    } catch {
      if (generation !== current || disposed) return
      upstream.value = {
        latest_version: upstream.value?.latest_version ?? '',
        release_url: upstream.value?.release_url ?? '',
        has_update: upstream.value?.has_update ?? null,
        checked_at: upstream.value?.checked_at ?? null,
        status: 'unavailable',
        stale: Boolean(upstream.value?.latest_version)
      }
      nextCheckAt = Date.now() + FAILURE_BACKOFF
    } finally {
      if (generation === current && !disposed) {
        checking.value = false
        schedule()
      }
    }
  }

  function resume() {
    clearTimer()
    if (disposed || !admin.value || document.hidden) return
    void loadVersion(generation)
    if (Date.now() >= nextCheckAt) void checkUpstream(generation)
    else schedule()
  }

  watch([admin, identity], () => {
    generation++
    controller?.abort()
    controller = undefined
    clearTimer()
    versionInfo.value = null
    upstream.value = null
    checking.value = false
    loadingVersion = false
    nextCheckAt = 0
    if (admin.value) {
      controller = new AbortController()
      resume()
    }
  }, { immediate: true })

  onMounted(() => document.addEventListener('visibilitychange', resume))
  onScopeDispose(() => {
    disposed = true
    generation++
    controller?.abort()
    clearTimer()
    document.removeEventListener('visibilitychange', resume)
  })

  return { versionInfo, upstream, checking }
}
