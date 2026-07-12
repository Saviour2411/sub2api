import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'

const { listRequestDetails } = vi.hoisted(() => ({
  listRequestDetails: vi.fn(),
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: { listRequestDetails },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showWarning: vi.fn() }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

vi.mock('@/views/admin/ops/utils/opsFormatters', () => ({
  parseTimeRangeMinutes: () => 60,
  formatDateTime: (value: string) => value,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  emits: ['close'],
  template: '<div v-if="show"><slot /></div>',
})

describe('OpsRequestDetailsModal 详情返回层级', () => {
  it('打开单条错误时保留请求明细弹窗状态', async () => {
    listRequestDetails.mockResolvedValue({
      total: 1,
      items: [
        {
          kind: 'error',
          created_at: '2026-07-12T00:00:00Z',
          platform: 'openai',
          model: 'gpt-5',
          duration_ms: 120,
          status_code: 504,
          request_id: 'request-1',
          error_id: 77,
        },
      ],
    })

    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: false,
        timeRange: '1h',
        preset: { title: '请求明细', kind: 'all', sort: 'created_at_desc' },
        platform: '',
        groupId: null,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Pagination: true,
        },
      },
    })

    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    await wrapper.get('button.bg-red-50').trigger('click')

    expect(wrapper.emitted('openErrorDetail')).toEqual([[77]])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
