import { onBeforeUnmount, onMounted, type Ref } from 'vue'

export interface UseTiltOptions {
  /** Maximum tilt in degrees on each axis (default 8) */
  max?: number
  /** Lerp / smoothing 0-1, smaller = smoother (default 0.18) */
  ease?: number
  /** Optional Z translation on hover (px) for "lift" feel (default 4) */
  lift?: number
  /** Disable tilt below this viewport width (default 768) */
  minWidth?: number
  /** Apply a follow-light spotlight on the element (sets --tilt-mx/--tilt-my) */
  spotlight?: boolean
}

/**
 * Mouse-driven 3D tilt with optional follow-light spotlight.
 * Respects prefers-reduced-motion and hides on mobile breakpoints.
 */
export function useTilt(target: Ref<HTMLElement | null>, options: UseTiltOptions = {}) {
  const {
    max = 8,
    ease = 0.18,
    lift = 4,
    minWidth = 768,
    spotlight = true
  } = options

  let rafId = 0
  let active = false
  let currentX = 0
  let currentY = 0
  let targetX = 0
  let targetY = 0
  let isInside = false

  function reduced() {
    if (typeof window === 'undefined') return true
    return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  }

  function tooNarrow() {
    if (typeof window === 'undefined') return true
    return window.innerWidth < minWidth
  }

  function tick() {
    const el = target.value
    if (!el) {
      rafId = 0
      return
    }
    currentX += (targetX - currentX) * ease
    currentY += (targetY - currentY) * ease

    const z = isInside ? lift : 0
    el.style.transform = `perspective(1100px) rotateX(${currentY.toFixed(2)}deg) rotateY(${currentX.toFixed(2)}deg) translateZ(${z}px)`

    const settled =
      Math.abs(currentX - targetX) < 0.02 && Math.abs(currentY - targetY) < 0.02
    if (settled && !isInside) {
      el.style.transform = ''
      el.classList.remove('is-tilting')
      rafId = 0
      return
    }
    rafId = requestAnimationFrame(tick)
  }

  function onMove(e: MouseEvent) {
    const el = target.value
    if (!el || !active) return
    const rect = el.getBoundingClientRect()
    const px = (e.clientX - rect.left) / rect.width
    const py = (e.clientY - rect.top) / rect.height
    targetX = (px - 0.5) * 2 * max
    targetY = -(py - 0.5) * 2 * max
    if (spotlight) {
      el.style.setProperty('--tilt-mx', `${(px * 100).toFixed(2)}%`)
      el.style.setProperty('--tilt-my', `${(py * 100).toFixed(2)}%`)
    }
    if (!rafId) rafId = requestAnimationFrame(tick)
  }

  function onEnter() {
    const el = target.value
    if (!el) return
    isInside = true
    el.classList.add('is-tilting')
    if (!rafId) rafId = requestAnimationFrame(tick)
  }

  function onLeave() {
    isInside = false
    targetX = 0
    targetY = 0
    if (!rafId) rafId = requestAnimationFrame(tick)
  }

  function attach() {
    const el = target.value
    if (!el || active) return
    if (reduced() || tooNarrow()) return
    active = true
    el.addEventListener('mousemove', onMove)
    el.addEventListener('mouseenter', onEnter)
    el.addEventListener('mouseleave', onLeave)
  }

  function detach() {
    const el = target.value
    if (!el) return
    active = false
    el.removeEventListener('mousemove', onMove)
    el.removeEventListener('mouseenter', onEnter)
    el.removeEventListener('mouseleave', onLeave)
    if (rafId) cancelAnimationFrame(rafId)
    rafId = 0
    el.style.transform = ''
    el.classList.remove('is-tilting')
  }

  function onResize() {
    if (tooNarrow()) detach()
    else attach()
  }

  onMounted(() => {
    attach()
    window.addEventListener('resize', onResize, { passive: true })
  })

  onBeforeUnmount(() => {
    window.removeEventListener('resize', onResize)
    detach()
  })

  return { detach, attach }
}
