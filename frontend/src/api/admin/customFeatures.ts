import { apiClient } from '../client'

export type DailyCheckinPrizeType = 'balance' | 'concurrency' | 'subscription' | 'none'
export type DailyCheckinBalanceMode = 'fixed' | 'range'

export interface DailyCheckinPrizeConfig {
  id: string
  name: string
  type: DailyCheckinPrizeType
  probability_bps: number
  enabled: boolean
  sort_order: number
  balance_mode?: DailyCheckinBalanceMode
  amount?: number
  min_amount?: number
  max_amount?: number
  concurrency?: number
  group_id?: number | null
  validity_days?: number
}

export interface DailyCheckinDecayRule {
  after_days: number
  factor_bps: number
}

export interface ModelMarketplaceSettings {
  enabled: boolean
  intro: string
  group_ids: number[]
}

export interface DailyCheckinSettings {
  enabled: boolean
  prizes: DailyCheckinPrizeConfig[]
  unpaid_full_days: number
  unpaid_decay_rules: DailyCheckinDecayRule[]
  linuxdo_exempt_enabled: boolean
}

export interface CustomFeatureSettings {
  model_marketplace: ModelMarketplaceSettings
  daily_checkin: DailyCheckinSettings
}

export async function getSettings(): Promise<CustomFeatureSettings> {
  const { data } = await apiClient.get<CustomFeatureSettings>('/admin/custom-features')
  return data
}

export async function updateModelMarketplace(
  settings: ModelMarketplaceSettings
): Promise<ModelMarketplaceSettings> {
  const { data } = await apiClient.put<ModelMarketplaceSettings>(
    '/admin/custom-features/model-marketplace',
    settings
  )
  return data
}

export async function updateDailyCheckin(
  settings: DailyCheckinSettings
): Promise<DailyCheckinSettings> {
  const { data } = await apiClient.put<DailyCheckinSettings>(
    '/admin/custom-features/daily-checkin',
    settings
  )
  return data
}

export default {
  getSettings,
  updateModelMarketplace,
  updateDailyCheckin
}
