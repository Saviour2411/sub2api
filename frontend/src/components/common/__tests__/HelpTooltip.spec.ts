import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

enableAutoUnmount(afterEach)

describe('HelpTooltip', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it.each([
    { x: 0, y: 4, left: '8px', top: '28px' },
    { x: 304, y: 4, left: '32px', top: '28px' },
    { x: 0, y: 576, left: '8px', top: '468px' },
    { x: 304, y: 576, left: '32px', top: '468px' },
  ])('视口边缘自动翻转并保持水平可见：$x / $y', async ({ x, y, left, top }) => {
    vi.stubGlobal('innerWidth', 320)
    vi.stubGlobal('innerHeight', 600)
    vi.stubGlobal('scrollX', 100)
    vi.stubGlobal('scrollY', 1200)
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: '边缘提示' } })
    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue(new DOMRect(x, y, 16, 16))
    vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 280, 100))
    Object.defineProperty(tooltip.firstElementChild, 'scrollHeight', { configurable: true, value: 100 })

    await trigger.trigger('mouseenter')
    await nextTick()

    expect(tooltip.parentElement).toBe(document.body)
    expect(tooltip.style.left).toBe(left)
    expect(tooltip.style.top).toBe(top)
    expect(trigger.attributes('aria-describedby')).toBe(tooltip.id)
  })

  it('超长内容限制高度，滚动和缩放后重新定位', async () => {
    vi.stubGlobal('innerWidth', 800)
    vi.stubGlobal('innerHeight', 600)
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: '超长说明' } })
    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()
    const content = tooltip.firstElementChild as HTMLElement
    const rect = vi.spyOn(trigger.element, 'getBoundingClientRect')
      .mockReturnValue(new DOMRect(700, 100, 16, 16))
    vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 280, 1000))
    Object.defineProperty(content, 'scrollHeight', { configurable: true, value: 1000 })

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(content.style.maxHeight).toBe('468px')
    expect(tooltip.style.top).toBe('124px')
    expect(tooltip.style.left).toBe('512px')

    rect.mockReturnValue(new DOMRect(300, 500, 16, 16))
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(content.style.maxHeight).toBe('484px')
    expect(tooltip.style.top).toBe('8px')
    vi.stubGlobal('innerWidth', 320)
    window.dispatchEvent(new Event('resize'))
    await nextTick()
    expect(tooltip.style.left).toBe('32px')
  })

  it('键盘可打开与关闭提示，禁用后不保留空浮层', async () => {
    const wrapper = mount(HelpTooltip, { attachTo: document.body, props: { content: '键盘提示' } })
    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()
    await trigger.trigger('focusin')
    expect(tooltip.style.display).not.toBe('none')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')
    await trigger.trigger('mouseenter')
    await wrapper.setProps({ disabled: true })
    expect(tooltip.style.display).toBe('none')
    await trigger.trigger('mouseenter')
    expect(tooltip.style.display).toBe('none')
  })

  it('嵌套提示保持内层高于外层，避免父提示遮挡子说明', async () => {
    const wrapper = mount(defineComponent({
      components: { HelpTooltip },
      template: '<HelpTooltip content="外层诊断"><template #trigger><HelpTooltip content="内层帮助" /></template></HelpTooltip>',
    }), { attachTo: document.body })
    for (const trigger of wrapper.findAll('.group')) {
      await trigger.trigger('mouseenter')
    }
    const tooltips = [...document.body.querySelectorAll<HTMLElement>('[role="tooltip"]')]
    const outer = tooltips.find(el => el.textContent?.includes('外层诊断'))!
    const inner = tooltips.find(el => el.textContent?.includes('内层帮助'))!
    expect(outer.style.display).not.toBe('none')
    expect(inner.style.display).not.toBe('none')
    expect(Number(inner.style.zIndex)).toBeGreaterThan(Number(outer.style.zIndex))
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps a hover tooltip open while the pointer moves between the trigger and the tooltip', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'copyable details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave', { relatedTarget: tooltip })
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: trigger.element }))
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: null }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })
})
