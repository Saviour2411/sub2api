import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { h } from 'vue'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import ColumnSettingsDropdown from '../ColumnSettingsDropdown.vue'

const columnSettingsPages = [
  'src/views/user/UsageView.vue',
  'src/views/user/KeysView.vue',
  'src/views/admin/UsageView.vue',
  'src/views/admin/UsersView.vue',
  'src/views/admin/GroupsView.vue',
  'src/views/admin/SubscriptionsView.vue',
]

describe('ColumnSettingsDropdown', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1280 })
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 900 })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('挂载到 body 并使用 fixed 定位，避免被表格卡片和 sticky 表头覆盖', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 100,
      right: 1000,
      bottom: 140,
      left: 880,
      width: 120,
      height: 40,
      x: 880,
      y: 100,
      toJSON: () => ({}),
    })

    const wrapper = mount(ColumnSettingsDropdown, {
      slots: {
        trigger: ({ toggle, open }: { toggle: () => void; open: boolean }) =>
          h('button', {
            class: 'column-trigger',
            'aria-expanded': String(open),
            onClick: toggle,
          }, '列设置'),
        default: () => h('button', { class: 'column-option' }, '模型'),
      },
    })

    await wrapper.get('.column-trigger').trigger('click')
    await flushPromises()

    const dropdown = document.body.querySelector<HTMLElement>('.column-settings-dropdown')
    expect(dropdown).not.toBeNull()
    expect(wrapper.element.contains(dropdown)).toBe(false)
    expect(dropdown?.className).toContain('fixed')
    expect(dropdown?.className).toContain('z-[100000020]')
    expect(dropdown?.style.top).toBe('146px')
    expect(dropdown?.style.left).toBe('808px')
    expect(dropdown?.style.width).toBe('192px')
    expect(dropdown?.style.maxHeight).toBe('320px')

    document.body.click()
    await flushPromises()
    expect(document.body.querySelector('.column-settings-dropdown')).toBeNull()

    wrapper.unmount()
  })

  it('按钮靠近视口底部时向上展开', async () => {
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 800 })
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      top: 700,
      right: 1000,
      bottom: 740,
      left: 880,
      width: 120,
      height: 40,
      x: 880,
      y: 700,
      toJSON: () => ({}),
    })

    const wrapper = mount(ColumnSettingsDropdown, {
      slots: {
        trigger: ({ toggle }: { toggle: () => void }) =>
          h('button', { class: 'column-trigger', onClick: toggle }, '列设置'),
        default: () => h('button', '模型'),
      },
    })

    await wrapper.get('.column-trigger').trigger('click')
    await flushPromises()

    const dropdown = document.body.querySelector<HTMLElement>('.column-settings-dropdown')
    expect(dropdown?.style.top).toBe('auto')
    expect(dropdown?.style.bottom).toBe('106px')
    expect(dropdown?.style.maxHeight).toBe('320px')

    wrapper.unmount()
  })

  it.each(columnSettingsPages)('%s 统一使用 body 浮层实现', (page) => {
    const source = readFileSync(resolve(process.cwd(), page), 'utf8')
    expect(source).toContain('<ColumnSettingsDropdown')
    expect(source).not.toContain('showColumnDropdown')
    expect(source).not.toContain('columnDropdownRef')
  })
})
