import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'

import DateRangePicker from '../DateRangePicker.vue'

const messages: Record<string, string> = {
  'dates.today': 'Today',
  'dates.yesterday': 'Yesterday',
  'dates.last24Hours': 'Last 24 Hours',
  'dates.last7Days': 'Last 7 Days',
  'dates.last14Days': 'Last 14 Days',
  'dates.last30Days': 'Last 30 Days',
  'dates.thisMonth': 'This Month',
  'dates.lastMonth': 'Last Month',
  'dates.startDate': 'Start Date',
  'dates.endDate': 'End Date',
  'dates.apply': 'Apply',
  'dates.selectDateRange': 'Select date range'
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
    locale: ref('en')
  })
}))

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const mountedWrappers: ReturnType<typeof mount>[] = []

const waitForTransition = () => new Promise((resolve) => setTimeout(resolve, 250))

const mountPicker = (startDate: string, endDate: string) => {
  const wrapper = mount(DateRangePicker, {
    attachTo: document.body,
    props: {
      startDate,
      endDate
    },
    global: {
      stubs: {
        Icon: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const findPresetButton = (text: string): HTMLButtonElement | undefined =>
  Array.from(document.body.querySelectorAll<HTMLButtonElement>('.date-picker-preset')).find((node) =>
    node.textContent?.includes(text)
  )

afterEach(() => {
  for (const wrapper of mountedWrappers.splice(0)) {
    wrapper.unmount()
  }
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('DateRangePicker', () => {
  it('uses last 24 hours as the default recognized preset', () => {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    const wrapper = mountPicker(formatLocalDate(yesterday), formatLocalDate(now))

    expect(wrapper.text()).toContain('Last 24 Hours')
  })

  it('emits range updates with last24Hours preset when applied', async () => {
    const now = new Date()
    const today = formatLocalDate(now)

    const wrapper = mountPicker(today, today)

    await wrapper.find('.date-picker-trigger').trigger('click')
    await nextTick()
    const presetButton = findPresetButton('Last 24 Hours')
    expect(presetButton).toBeDefined()

    presetButton!.click()
    await nextTick()
    document.body.querySelector<HTMLButtonElement>('.date-picker-apply')!.click()
    await nextTick()

    const nowAfterClick = new Date()
    const yesterdayAfterClick = new Date(nowAfterClick.getTime() - 24 * 60 * 60 * 1000)
    const expectedStart = formatLocalDate(yesterdayAfterClick)
    const expectedEnd = formatLocalDate(nowAfterClick)

    expect(wrapper.emitted('update:startDate')?.[0]).toEqual([expectedStart])
    expect(wrapper.emitted('update:endDate')?.[0]).toEqual([expectedEnd])
    expect(wrapper.emitted('change')?.[0]).toEqual([
      {
        startDate: expectedStart,
        endDate: expectedEnd,
        preset: 'last24Hours'
      }
    ])
  })

  it('teleports the dropdown to body and uses fixed positioning', async () => {
    const today = formatLocalDate(new Date())
    const wrapper = mountPicker(today, today)

    await wrapper.find('.date-picker-trigger').trigger('click')
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.date-picker-dropdown')
    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.position).toBe('fixed')
    expect(wrapper.element.contains(dropdown)).toBe(false)
  })

  it('opens above the trigger when there is not enough space below', async () => {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 520 })
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 40,
      y: 400,
      top: 400,
      right: 180,
      bottom: 440,
      left: 40,
      width: 140,
      height: 40,
      toJSON: () => ({})
    } as DOMRect)

    const today = formatLocalDate(new Date())
    const wrapper = mountPicker(today, today)

    await wrapper.find('.date-picker-trigger').trigger('click')
    await nextTick()
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.date-picker-dropdown')
    expect(dropdown?.style.bottom).toBe('128px')
    expect(dropdown?.style.top).toBe('')
  })

  it('keeps the dropdown open when clicking inside and closes when clicking outside', async () => {
    const today = formatLocalDate(new Date())
    const wrapper = mountPicker(today, today)

    await wrapper.find('.date-picker-trigger').trigger('click')
    await nextTick()

    const dropdown = document.body.querySelector<HTMLElement>('.date-picker-dropdown')
    expect(dropdown).not.toBeNull()

    dropdown!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(document.body.querySelector('.date-picker-dropdown')).not.toBeNull()

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    await waitForTransition()
    expect(document.body.querySelector('.date-picker-dropdown')).toBeNull()
  })
})
