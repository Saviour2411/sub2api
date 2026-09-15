import type { UsageLog } from '@/types'
import { resolveUsageRequestType } from './usageRequestType'
import { getDisplayBillingMode } from './billingMode'

type RateRow = Pick<UsageLog, 'output_tokens' | 'duration_ms' | 'first_token_ms' | 'stream' | 'request_type' | 'openai_ws_mode' | 'image_count' | 'image_output_tokens' | 'billing_mode'>

export function calculateUsageOutputRate(row: RateRow): number | null {
  const type = resolveUsageRequestType(row)
  const mode = getDisplayBillingMode(row)
  if ((type !== 'stream' && type !== 'ws_v2') || (mode && mode !== 'token' && mode !== 'per_request') || row.image_count > 0 || row.image_output_tokens > 0) return null
  const { output_tokens: tokens, duration_ms: duration, first_token_ms: first } = row
  if (!Number.isFinite(tokens) || tokens <= 0 || duration == null || first == null || !Number.isFinite(duration) || !Number.isFinite(first) || first < 0 || duration <= first) return null
  const rate = tokens * 1000 / (duration - first)
  return Number.isFinite(rate) ? rate : null
}

export function formatUsageOutputRate(row: RateRow): string {
  const rate = calculateUsageOutputRate(row)
  return rate == null ? '-' : `${rate.toFixed(2)} tk/s`
}
