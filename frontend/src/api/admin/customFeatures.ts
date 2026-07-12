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

export interface GatewaySettings {
  default_pool_mode_retry_count: number
  default_pool_mode_retry_status_codes: number[]
  auto_managed_probe_backoff_minutes: number[]
  first_token_timeout_seconds: number
  first_token_timeout_consecutive_threshold: number
  upstream_error_status_codes: number[]
  upstream_error_consecutive_threshold: number
  image_group_success_rate_visible: boolean
  anthropic_claude_code_mimicry_enabled: boolean
}

export interface ImageGroupSuccessRatesResetResult {
  reset_at: string
}

export interface CustomFeatureSettings {
  model_marketplace: ModelMarketplaceSettings
  daily_checkin: DailyCheckinSettings
  gateway: GatewaySettings
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

export async function updateGateway(settings: Partial<GatewaySettings>): Promise<GatewaySettings> {
  const { data } = await apiClient.put<GatewaySettings>(
    '/admin/custom-features/gateway',
    settings
  )
  return data
}

export async function resetImageGroupSuccessRates(): Promise<ImageGroupSuccessRatesResetResult> {
  const { data } = await apiClient.post<ImageGroupSuccessRatesResetResult>(
    '/admin/custom-features/gateway/image-group-success-rates/reset'
  )
  return data
}

export default {
  getSettings,
  updateModelMarketplace,
  updateDailyCheckin,
  updateGateway,
  resetImageGroupSuccessRates
}
