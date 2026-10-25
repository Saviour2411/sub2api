import { modelQualityAPI, type QualityRun } from '@/api/modelQuality'

type Priority = 'detail' | 'visible' | 'prefetch'
type Invalidation = 'changed' | 'expired' | 'reset' | 'denied'
interface CacheEntry {
  run: QualityRun
  until: number
  bytes: number
}
interface Job {
  id: number
  priority: number
  started: boolean
  consumers: number
  controller: AbortController
  promise: Promise<QualityRun>
  resolve: (run: QualityRun) => void
  reject: (error: unknown) => void
}
interface LoaderOptions {
  fetch?: typeof modelQualityAPI.detail
  now?: () => number
  maxEntries?: number
  maxBytes?: number
  ttl?: number
  concurrency?: number
}

export function qualityRunVersion(run: QualityRun): string {
  return JSON.stringify([
    run.outcome,
    run.finished_at,
    run.expires_at,
    run.tests.map((t) => [
      t.benchmark,
      t.status,
      t.verdict,
      t.finished_at,
      t.has_html
    ])
  ])
}
export function qualityRequestStatus(error: unknown): number | undefined {
  const e = error as { status?: number; response?: { status?: number } }
  return e?.response?.status ?? e?.status
}
export function isQualityAbort(error: unknown): boolean {
  const e = error as { name?: string; code?: string }
  return (
    e?.name === 'AbortError' ||
    e?.name === 'CanceledError' ||
    e?.code === 'ERR_CANCELED'
  )
}
const cancelled = () => new DOMException('作品请求已取消', 'AbortError')

export function scheduleQualityExpiry(expiresAt: string, expire: () => void) {
  let timer: ReturnType<typeof setTimeout> | undefined
  const check = () => {
    const remaining = Date.parse(expiresAt) - Date.now()
    if (!Number.isFinite(remaining) || remaining <= 0) expire()
    else timer = setTimeout(check, Math.min(2147483647, remaining))
  }
  check()
  return () => clearTimeout(timer)
}

export function createModelQualityLoader(options: LoaderOptions = {}) {
  const fetchRun = options.fetch ?? modelQualityAPI.detail
  const now = options.now ?? Date.now
  const maxEntries = options.maxEntries ?? 24
  const maxBytes = options.maxBytes ?? 32 * 1024 * 1024
  const ttl = options.ttl ?? 5 * 60 * 1000
  const concurrency = options.concurrency ?? 4
  const cache = new Map<number, CacheEntry>()
  const versions = new Map<number, string>()
  const jobs = new Map<number, Job>()
  const active = new Set<Job>()
  const listeners = new Set<(id: number | null, reason: Invalidation) => void>()
  let bytes = 0
  let allowedGroups: Set<number> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  const notify = (id: number | null, reason: Invalidation) =>
    listeners.forEach((fn) => fn(id, reason))
  function remove(id: number) {
    const entry = cache.get(id)
    if (entry) bytes -= entry.bytes
    cache.delete(id)
  }
  function sweep() {
    clearTimeout(timer)
    timer = undefined
    for (const [id, entry] of cache) {
      if (entry.until > now()) continue
      remove(id)
      if (Date.parse(entry.run.expires_at) <= now()) notify(id, 'expired')
    }
    if (cache.size) {
      const next = Math.min(
        ...Array.from(cache.values(), (entry) => entry.until)
      )
      timer = setTimeout(sweep, Math.max(1, next - now()))
    }
  }
  function cancelJob(job: Job) {
    jobs.delete(job.id)
    job.controller.abort()
    job.reject(cancelled())
  }
  function invalidate(id: number, reason: Invalidation = 'changed') {
    remove(id)
    const job = jobs.get(id)
    if (job) cancelJob(job)
    notify(id, reason)
    sweep()
    queueMicrotask(pump)
  }
  function clear(reason: Invalidation = 'reset') {
    clearTimeout(timer)
    timer = undefined
    cache.clear()
    versions.clear()
    allowedGroups = undefined
    bytes = 0
    for (const job of jobs.values()) cancelJob(job)
    notify(null, reason)
  }
  function remember(run: QualityRun) {
    if (!run.finished_at || run.outcome === 'running') return
    const until = Math.min(now() + ttl, Date.parse(run.expires_at))
    if (!Number.isFinite(until) || until <= now()) return
    // 按 UTF-16 正文估算预算，不代表浏览器堆内存上限。
    const size = JSON.stringify(run).length * 2
    remove(run.id)
    if (size > maxBytes) return
    cache.set(run.id, { run, until, bytes: size })
    bytes += size
    while (cache.size > maxEntries || bytes > maxBytes)
      remove(cache.keys().next().value!)
    sweep()
  }
  function pump() {
    const queue = [...jobs.values()]
      .filter((job) => !job.started)
      .sort((a, b) => a.priority - b.priority)
    for (const job of queue) {
      if (active.size >= concurrency) break
      if (jobs.get(job.id) !== job) continue
      job.started = true
      active.add(job)
      void Promise.resolve()
        .then(() => fetchRun(job.id, job.controller.signal))
        .then((run) => {
          if (jobs.get(job.id) !== job || job.controller.signal.aborted)
            throw cancelled()
          if (
            !Number.isFinite(Date.parse(run.expires_at)) ||
            Date.parse(run.expires_at) <= now() ||
            (allowedGroups && !allowedGroups.has(run.group_id))
          ) {
            throw { status: 410, message: '作品已超过保留期限' }
          }
          versions.set(run.id, qualityRunVersion(run))
          remember(run)
          jobs.delete(job.id)
          job.resolve(run)
        })
        .catch((error) => {
          if (jobs.get(job.id) === job) {
            const status = qualityRequestStatus(error)
            if (status === 401 || status === 403) clear('denied')
            else if (status === 404 || status === 410) {
              remove(job.id)
              notify(job.id, 'expired')
            }
          }
          job.reject(error)
        })
        .finally(() => {
          active.delete(job)
          if (jobs.get(job.id) === job) jobs.delete(job.id)
          pump()
        })
    }
  }
  function wait(job: Job, signal?: AbortSignal): Promise<QualityRun> {
    job.consumers++
    return new Promise((resolve, reject) => {
      let done = false
      const finish = () => {
        if (done) return false
        done = true
        signal?.removeEventListener('abort', abort)
        job.consumers--
        if (!job.started && !job.consumers && jobs.get(job.id) === job)
          cancelJob(job)
        return true
      }
      const abort = () => {
        if (finish()) reject(cancelled())
      }
      signal?.addEventListener('abort', abort, { once: true })
      job.promise.then(
        (value) => {
          if (finish()) resolve(value)
        },
        (error) => {
          if (finish()) reject(error)
        }
      )
      if (signal?.aborted) abort()
    })
  }
  function load(
    id: number,
    settings: {
      priority?: Priority
      signal?: AbortSignal
      force?: boolean
    } = {}
  ) {
    if (settings.signal?.aborted) return Promise.reject(cancelled())
    sweep()
    const entry = cache.get(id)
    if (entry && !settings.force) {
      cache.delete(id)
      cache.set(id, entry)
      return Promise.resolve(entry.run)
    }
    if (settings.force) remove(id)
    const priority = { detail: 0, visible: 1, prefetch: 2 }[
      settings.priority ?? 'visible'
    ]
    let job = jobs.get(id)
    if (!job) {
      let resolve!: Job['resolve'], reject!: Job['reject']
      const promise = new Promise<QualityRun>((yes, no) => {
        resolve = yes
        reject = no
      })
      job = {
        id,
        promise,
        resolve,
        reject,
        priority,
        started: false,
        consumers: 0,
        controller: new AbortController()
      }
      jobs.set(id, job)
    } else job.priority = Math.min(job.priority, priority)
    const result = wait(job, settings.signal)
    queueMicrotask(pump)
    return result
  }
  function sync(runs: QualityRun[]) {
    const current = new Set(runs.map((run) => run.id))
    for (const run of runs) {
      const version = qualityRunVersion(run)
      const previous = versions.get(run.id)
      versions.set(run.id, version)
      if (Date.parse(run.expires_at) <= now()) invalidate(run.id, 'expired')
      else if (previous !== undefined && previous !== version)
        invalidate(run.id)
    }
    for (const id of versions.keys()) {
      if (!current.has(id) && !cache.has(id) && !jobs.has(id))
        versions.delete(id)
    }
  }
  return {
    load,
    sync,
    clear,
    invalidate,
    setGroups(ids: number[]) {
      allowedGroups = new Set(ids)
      for (const [id, entry] of cache)
        if (!allowedGroups.has(entry.run.group_id)) invalidate(id, 'expired')
    },
    prioritize(id: number) {
      const job = jobs.get(id)
      if (job) job.priority = Math.min(job.priority, 1)
    },
    subscribe(listener: (id: number | null, reason: Invalidation) => void) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    }
  }
}
