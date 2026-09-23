export interface UsageExportPage<T> {
  items: T[]
  next_cursor: string
}

export const USAGE_EXPORT_PAGE_SIZE = 1000
const PAGE_INTERVAL_MS = 1250
const MAX_RETRIES = 5

export function isExportCanceled(error: unknown): boolean {
  const value = error as { name?: string; code?: string } | null
  return value?.name === 'AbortError' || value?.code === 'ERR_CANCELED'
}

function throwIfCanceled(signal: AbortSignal) {
  if (signal.aborted) throw new DOMException('导出已取消', 'AbortError')
}

export function waitForExport(ms: number, signal: AbortSignal): Promise<void> {
  throwIfCanceled(signal)
  return new Promise((resolve, reject) => {
    const cancel = () => {
      clearTimeout(timer)
      signal.removeEventListener('abort', cancel)
      reject(new DOMException('导出已取消', 'AbortError'))
    }
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', cancel)
      resolve()
    }, ms)
    signal.addEventListener('abort', cancel, { once: true })
  })
}

function retryDelay(error: unknown, attempt: number): number | null {
  const value = error as { status?: number; retryAfter?: string | number } | null
  if (![0, 429, 500, 502, 503, 504].includes(value?.status ?? -1)) return null
  if (value?.retryAfter != null) {
    const raw = String(value.retryAfter)
    const delay = /^\d+(\.\d+)?$/.test(raw) ? Number(raw) * 1000 : Date.parse(raw) - Date.now()
    if (Number.isFinite(delay) && delay > 0) return Math.min(delay, 300_000) + 250
  }
  return value?.status === 429 ? 60_250 : Math.min(1000 * 2 ** attempt, 15_000)
}

// 每次仅保留一批原始记录；重试相同游标，处理成功后才推进，避免重复输出。
export async function exportUsagePages<T>(options: {
  load: (cursor: string, signal: AbortSignal) => Promise<UsageExportPage<T>>
  consume: (items: T[]) => void | Promise<void>
  signal: AbortSignal
  onProgress?: (count: number) => void
}): Promise<number> {
  let cursor = ''
  let count = 0
  while (true) {
    throwIfCanceled(options.signal)
    let page: UsageExportPage<T>
    for (let attempt = 0; ; attempt++) {
      try {
        page = await options.load(cursor, options.signal)
        break
      } catch (error) {
        throwIfCanceled(options.signal)
        const delay = retryDelay(error, attempt)
        if (isExportCanceled(error) || delay === null || attempt >= MAX_RETRIES) throw error
        await waitForExport(delay, options.signal)
      }
    }
    throwIfCanceled(options.signal)
    const next = page.next_cursor
    if (typeof next !== 'string' || (next && (!/^[1-9]\d*$/.test(next) || (cursor && BigInt(next) >= BigInt(cursor)) || page.items.length === 0))) {
      throw new Error('导出接口返回了无效游标')
    }
    await options.consume(page.items)
    count += page.items.length
    options.onProgress?.(count)
    if (!next) return count
    cursor = next
    await waitForExport(PAGE_INTERVAL_MS, options.signal)
  }
}
