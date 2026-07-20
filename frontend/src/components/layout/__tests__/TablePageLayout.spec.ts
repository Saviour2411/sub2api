import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TablePageLayout from '../TablePageLayout.vue'

const setScrollMetrics = (el: HTMLElement, metrics: {
  scrollHeight: number
  clientHeight: number
  scrollTop: number
}) => {
  Object.defineProperty(el, 'scrollHeight', {
    configurable: true,
    value: metrics.scrollHeight
  })
  Object.defineProperty(el, 'clientHeight', {
    configurable: true,
    value: metrics.clientHeight
  })
  Object.defineProperty(el, 'scrollTop', {
    configurable: true,
    writable: true,
    value: metrics.scrollTop
  })
}

const dispatchWheel = (el: Element, deltaY: number) => {
  const event = new WheelEvent('wheel', {
    bubbles: true,
    cancelable: true,
    deltaY
  })
  const allowed = el.dispatchEvent(event)
  return { event, allowed }
}

describe('TablePageLayout', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      writable: true,
      value: 1280
    })
  })

  it('表格内容不足一屏时阻止滚轮传给页面外层', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        table: '<div class="table-wrapper"></div>'
      }
    })
    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    setScrollMetrics(tableWrapper, { scrollHeight: 300, clientHeight: 300, scrollTop: 0 })

    const result = dispatchWheel(wrapper.find('.table-scroll-container').element, 120)

    expect(result.allowed).toBe(false)
    expect(result.event.defaultPrevented).toBe(true)
  })

  it('表格还能继续向下滚动时在表格内消费滚轮', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        table: '<div class="table-wrapper"></div>'
      }
    })
    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    setScrollMetrics(tableWrapper, { scrollHeight: 900, clientHeight: 300, scrollTop: 120 })

    const result = dispatchWheel(wrapper.find('.table-scroll-container').element, 120)

    expect(result.allowed).toBe(false)
    expect(result.event.defaultPrevented).toBe(true)
    expect(tableWrapper.scrollTop).toBe(240)
  })

  it('表格滚到底时阻止继续向下滚动外层页面', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        table: '<div class="table-wrapper"></div>'
      }
    })
    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    setScrollMetrics(tableWrapper, { scrollHeight: 900, clientHeight: 300, scrollTop: 600 })

    const result = dispatchWheel(wrapper.find('.table-scroll-container').element, 120)

    expect(result.allowed).toBe(false)
    expect(result.event.defaultPrevented).toBe(true)
  })

  it('表格滚轮事件不会继续冒泡到页面外层', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        table: '<div class="table-wrapper"></div>'
      }
    })
    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    setScrollMetrics(tableWrapper, { scrollHeight: 900, clientHeight: 300, scrollTop: 600 })
    const documentWheel = vi.fn()
    document.addEventListener('wheel', documentWheel)

    dispatchWheel(tableWrapper, 120)

    expect(documentWheel).not.toHaveBeenCalled()
    document.removeEventListener('wheel', documentWheel)
  })
})

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('does not disable the table horizontal scroll container in mobile mode', () => {
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toContain('overflow-x-auto')
    expect(mobileBlocks.every(([, , declarations]) => !declarations.includes('overflow-visible'))).toBe(
      true
    )
  })
})
