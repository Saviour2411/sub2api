import { onBeforeUnmount, onMounted, ref } from 'vue'

export interface UseSystemClockOptions {
  /** Update interval in ms (default 1000) */
  interval?: number
  /** Pause updates when tab is hidden (default true) */
  pauseWhenHidden?: boolean
}

function pad2(n: number) {
  return n < 10 ? `0${n}` : `${n}`
}

function formatUtcOffset(date: Date) {
  const offset = -date.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  const hh = Math.floor(abs / 60)
  const mm = abs % 60
  return mm === 0 ? `UTC${sign}${hh}` : `UTC${sign}${hh}:${pad2(mm)}`
}

/**
 * Reactive wall-clock with formatted strings, tuned for compact HUD displays.
 * Auto-pauses while the page is hidden to save CPU.
 */
export function useSystemClock(options: UseSystemClockOptions = {}) {
  const { interval = 1000, pauseWhenHidden = true } = options

  const now = ref<Date>(new Date())
  const time = ref<string>(formatTime(now.value))
  const date = ref<string>(formatDate(now.value))
  const tz = ref<string>(formatUtcOffset(now.value))

  let timerId: ReturnType<typeof setInterval> | null = null

  function formatTime(d: Date) {
    return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
  }

  function formatDate(d: Date) {
    return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
  }

  function tick() {
    const d = new Date()
    now.value = d
    time.value = formatTime(d)
    date.value = formatDate(d)
    tz.value = formatUtcOffset(d)
  }

  function start() {
    if (timerId !== null) return
    tick()
    timerId = setInterval(tick, interval)
  }

  function stop() {
    if (timerId === null) return
    clearInterval(timerId)
    timerId = null
  }

  function onVisibilityChange() {
    if (!pauseWhenHidden) return
    if (document.hidden) stop()
    else start()
  }

  onMounted(() => {
    start()
    if (pauseWhenHidden) {
      document.addEventListener('visibilitychange', onVisibilityChange)
    }
  })

  onBeforeUnmount(() => {
    stop()
    if (pauseWhenHidden) {
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  })

  return { now, time, date, tz, start, stop }
}
