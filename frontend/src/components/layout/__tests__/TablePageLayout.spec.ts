import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'

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

  it('表格还能继续向下滚动时不拦截滚轮', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        table: '<div class="table-wrapper"></div>'
      }
    })
    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    setScrollMetrics(tableWrapper, { scrollHeight: 900, clientHeight: 300, scrollTop: 120 })

    const result = dispatchWheel(wrapper.find('.table-scroll-container').element, 120)

    expect(result.allowed).toBe(true)
    expect(result.event.defaultPrevented).toBe(false)
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
})
