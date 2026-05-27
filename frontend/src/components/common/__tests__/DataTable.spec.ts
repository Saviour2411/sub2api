import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

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
})
