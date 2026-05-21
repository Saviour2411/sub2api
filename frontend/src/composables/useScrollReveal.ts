import { onBeforeUnmount, onMounted, type Ref } from 'vue'

export interface UseScrollRevealOptions {
  /** CSS selector applied within the root to find reveal targets (default '[data-reveal]') */
  selector?: string
  /** Class to toggle when element enters viewport (default 'is-visible') */
  visibleClass?: string
  /** Stagger delay between sibling reveals in ms (default 80) */
  stagger?: number
  /** IntersectionObserver threshold (default 0.15) */
  threshold?: number
  /** Once revealed, stop observing (default true) */
  once?: boolean
}

/**
 * IntersectionObserver-based scroll reveal helper.
 * Targets must carry data-reveal (or custom selector) and a base class like
 * `reveal-up` / `reveal-fade` for the keyframes to take over.
 *
 * Optional per-element override: data-reveal-delay="200" sets explicit delay (ms).
 */
export function useScrollReveal(
  root: Ref<HTMLElement | null> | (() => HTMLElement | Document | null),
  options: UseScrollRevealOptions = {}
) {
  const {
    selector = '[data-reveal]',
    visibleClass = 'is-visible',
    stagger = 80,
    threshold = 0.15,
    once = true
  } = options

  let observer: IntersectionObserver | null = null
  let mutationObserver: MutationObserver | null = null
  const indexMap = new WeakMap<Element, number>()
  let revealed = 0

  function resolveRoot(): Element | Document | null {
    if (typeof root === 'function') return root()
    return root.value
  }

  function reduced() {
    if (typeof window === 'undefined') return true
    return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  }

  function reveal(target: Element) {
    const el = target as HTMLElement
    const explicit = el.dataset.revealDelay
    const delay = explicit
      ? Number(explicit)
      : (indexMap.get(target) ?? revealed) * stagger
    if (delay > 0) {
      el.style.animationDelay = `${delay}ms`
    }
    el.classList.add(visibleClass)
  }

  function observeAll() {
    const host = resolveRoot()
    if (!host) return
    const nodeList = host instanceof Document
      ? host.querySelectorAll(selector)
      : (host as Element).querySelectorAll(selector)
    if (reduced()) {
      nodeList.forEach((node) => node.classList.add(visibleClass))
      return
    }
    nodeList.forEach((node, i) => {
      if (!indexMap.has(node)) indexMap.set(node, i)
      observer?.observe(node)
    })
  }

  onMounted(() => {
    if (reduced()) {
      // mark all visible immediately, no observer
      const host = resolveRoot()
      host?.querySelectorAll?.(selector).forEach((node) =>
        node.classList.add(visibleClass)
      )
      return
    }
    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          reveal(entry.target)
          revealed += 1
          if (once) observer?.unobserve(entry.target)
        }
      },
      { threshold }
    )
    observeAll()

    // Re-scan when DOM mutates
    const host = resolveRoot()
    if (host && host instanceof Element) {
      mutationObserver = new MutationObserver(() => observeAll())
      mutationObserver.observe(host, { childList: true, subtree: true })
    }
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
    mutationObserver?.disconnect()
    mutationObserver = null
  })

  return { rescan: observeAll }
}
