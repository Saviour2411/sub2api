import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as Record<string, boolean> | null,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

describe('custom feature flag visibility', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
  })

  it('keeps daily check-in opt-in', () => {
    expect(isFeatureFlagEnabled(FeatureFlags.dailyCheckin)).toBe(false)

    appStore.cachedPublicSettings = { daily_checkin_enabled: true }
    expect(isFeatureFlagEnabled(FeatureFlags.dailyCheckin)).toBe(true)

    appStore.cachedPublicSettings = { daily_checkin_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.dailyCheckin)).toBe(false)
  })

  it('keeps model marketplace opt-out', () => {
    expect(isFeatureFlagEnabled(FeatureFlags.modelMarketplace)).toBe(true)

    appStore.cachedPublicSettings = { model_marketplace_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.modelMarketplace)).toBe(false)
  })
})
