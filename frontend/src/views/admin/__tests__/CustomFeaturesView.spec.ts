import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import CustomFeaturesView from '../CustomFeaturesView.vue'
import type { CustomFeatureSettings } from '@/api/admin/customFeatures'

const {
  getSettings,
  updateModelMarketplace,
  updateDailyCheckin,
  getGroups,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateModelMarketplace: vi.fn(),
  updateDailyCheckin: vi.fn(),
  getGroups: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/customFeatures', () => ({
  default: {
    getSettings,
    updateModelMarketplace,
    updateDailyCheckin,
  },
}))

vi.mock('@/api/admin/groups', () => ({
  default: {
    getAll: getGroups,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        key.replace(/\{(\w+)\}/g, (_match, token) => String(params?.[token] ?? `{${token}}`)),
    }),
  }
})

const ToggleStub = defineComponent({
  inheritAttrs: false,
  props: { modelValue: { type: Boolean, default: false } },
  emits: ['update:modelValue'],
  template: '<button type="button" v-bind="$attrs" @click="$emit(\'update:modelValue\', !modelValue)">{{ modelValue ? "on" : "off" }}</button>',
})

const SelectStub = defineComponent({
  props: {
    modelValue: { type: [Number, String], default: null },
    options: { type: Array, default: () => [] },
  },
  emits: ['update:modelValue'],
  template: '<select :value="modelValue ?? \'\'" @change="$emit(\'update:modelValue\', Number($event.target.value) || null)"><option value=""></option><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
})

function settingsFixture(): CustomFeatureSettings {
  return {
    model_marketplace: {
      enabled: true,
      intro: '模型说明',
      group_ids: [],
    },
    daily_checkin: {
      enabled: true,
      prizes: [
        { id: 'none', name: '谢谢参与', type: 'none', probability_bps: 1000, enabled: true, sort_order: 0 },
        { id: 'balance', name: '余额', type: 'balance', probability_bps: 4000, enabled: true, sort_order: 1, balance_mode: 'fixed', amount: 0.1 },
        { id: 'concurrency', name: '并发', type: 'concurrency', probability_bps: 2500, enabled: true, sort_order: 2, concurrency: 1 },
        { id: 'subscription', name: '订阅', type: 'subscription', probability_bps: 2500, enabled: true, sort_order: 3, group_id: 2, validity_days: 7 },
      ],
      unpaid_full_days: 7,
      unpaid_decay_rules: [
        { after_days: 7, factor_bps: 5000 },
        { after_days: 14, factor_bps: 2000 },
        { after_days: 30, factor_bps: 500 },
      ],
      linuxdo_exempt_enabled: true,
    },
  }
}

function mountView() {
  return mount(CustomFeaturesView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Toggle: ToggleStub,
        Select: SelectStub,
      },
    },
  })
}

describe('admin CustomFeaturesView', () => {
  beforeEach(() => {
    getSettings.mockReset()
    updateModelMarketplace.mockReset()
    updateDailyCheckin.mockReset()
    getGroups.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    getSettings.mockResolvedValue(settingsFixture())
    getGroups.mockResolvedValue([
      { id: 1, name: '公开组', platform: 'openai', status: 'active', subscription_type: 'standard', is_exclusive: false, description: '' },
      { id: 2, name: '订阅组', platform: 'anthropic', status: 'active', subscription_type: 'subscription', is_exclusive: false, description: '' },
    ])
    updateModelMarketplace.mockImplementation(async (payload) => payload)
    updateDailyCheckin.mockImplementation(async (payload) => payload)
  })

  it('loads the independent forms and switches tabs', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSettings).toHaveBeenCalledOnce()
    expect(getGroups).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="model-marketplace-form"]').exists()).toBe(true)

    await wrapper.get('[data-test="custom-feature-tab-daily-checkin"]').trigger('click')
    expect(wrapper.find('[data-test="daily-checkin-form"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-test^="daily-prize-"]')).toHaveLength(4)
  })

  it('saves the model marketplace intro and selected groups independently', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="model-marketplace-intro"]').setValue('  新说明  ')
    const firstGroup = wrapper.findAll<HTMLInputElement>('input[type="checkbox"]')[0]
    await firstGroup.setValue(true)
    await wrapper.get('[data-test="model-marketplace-form"]').trigger('submit')
    await flushPromises()

    expect(updateModelMarketplace).toHaveBeenCalledWith({
      enabled: true,
      intro: '新说明',
      group_ids: [1],
    })
    expect(updateDailyCheckin).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
  })

  it('blocks saving when enabled prize probabilities do not total 100%', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="custom-feature-tab-daily-checkin"]').trigger('click')

    const firstPrize = wrapper.get('[data-test="daily-prize-0"]')
    await firstPrize.get('input[type="number"]').setValue('999')
    await wrapper.get('[data-test="daily-checkin-form"]').trigger('submit')
    await flushPromises()

    expect(updateDailyCheckin).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.customFeatures.dailyCheckin.validation.probabilityTotal')
  })

  it('submits balance, concurrency, subscription and no-reward prizes', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="custom-feature-tab-daily-checkin"]').trigger('click')
    await wrapper.get('[data-test="daily-checkin-form"]').trigger('submit')
    await flushPromises()

    expect(updateDailyCheckin).toHaveBeenCalledOnce()
    const payload = updateDailyCheckin.mock.calls[0][0]
    expect(payload.prizes.map((prize: { type: string }) => prize.type)).toEqual([
      'none',
      'balance',
      'concurrency',
      'subscription',
    ])
    expect(payload.unpaid_decay_rules).toHaveLength(3)
    expect(payload.linuxdo_exempt_enabled).toBe(true)
    expect(showSuccess).toHaveBeenCalled()
  })
})
