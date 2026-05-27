import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import DataTable from '../DataTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('DataTable 表格', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: true,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn()
      }))
    })
  })

  it('关闭虚拟滚动时渲染当前页全部行且不生成占位行', () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name' },
          { key: 'status', label: 'Status' }
        ],
        data: [
          { id: 1, name: 'Alpha', status: 'active' },
          { id: 2, name: 'Beta', status: 'paused' },
          { id: 3, name: 'Gamma', status: 'active' }
        ],
        rowKey: 'id',
        virtualized: false
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.findAll('tbody tr[data-row-id]')).toHaveLength(3)
    expect(wrapper.findAll('tbody tr[aria-hidden="true"]')).toHaveLength(0)
    expect(wrapper.text()).toContain('Alpha')
    expect(wrapper.text()).toContain('Gamma')
  })

  it('滚动重置键变化时会把表格滚回顶部', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name' },
          { key: 'status', label: 'Status' }
        ],
        data: [
          { id: 1, name: 'Alpha', status: 'active' },
          { id: 2, name: 'Beta', status: 'paused' },
          { id: 3, name: 'Gamma', status: 'active' }
        ],
        rowKey: 'id',
        virtualized: false,
        scrollResetKey: 'page-1'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    Object.defineProperty(tableWrapper, 'scrollHeight', {
      configurable: true,
      value: 900
    })
    Object.defineProperty(tableWrapper, 'clientHeight', {
      configurable: true,
      value: 300
    })
    Object.defineProperty(tableWrapper, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 240
    })

    await wrapper.setProps({ scrollResetKey: 'page-2' })
    await nextTick()

    expect(tableWrapper.scrollTop).toBe(0)
  })

  it('数据变少后会把旧滚动位置夹在合法范围内', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name' },
          { key: 'status', label: 'Status' }
        ],
        data: [
          { id: 1, name: 'Alpha', status: 'active' },
          { id: 2, name: 'Beta', status: 'paused' },
          { id: 3, name: 'Gamma', status: 'active' }
        ],
        rowKey: 'id',
        virtualized: false
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const tableWrapper = wrapper.find<HTMLElement>('.table-wrapper').element
    Object.defineProperty(tableWrapper, 'scrollHeight', {
      configurable: true,
      value: 360
    })
    Object.defineProperty(tableWrapper, 'clientHeight', {
      configurable: true,
      value: 300
    })
    Object.defineProperty(tableWrapper, 'scrollTop', {
      configurable: true,
      writable: true,
      value: 240
    })

    await wrapper.setProps({
      data: [
        { id: 1, name: 'Alpha', status: 'active' }
      ]
    })
    await nextTick()

    expect(tableWrapper.scrollTop).toBe(60)
  })
})
