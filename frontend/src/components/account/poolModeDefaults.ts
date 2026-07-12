export const DEFAULT_POOL_MODE_RETRY_COUNT = 1
export const MAX_POOL_MODE_RETRY_COUNT = 10
export const DEFAULT_POOL_MODE_RETRY_STATUS_CODES = [401, 403, 429, 502, 503, 504] as const

export interface GatewayPoolDefaultsInput {
  default_pool_mode_retry_count?: number
  default_pool_mode_retry_status_codes?: number[]
}

export interface GatewayPoolDefaults {
  retryCount: number
  retryStatusCodes: number[]
}

export function normalizeGatewayPoolDefaults(
  settings?: GatewayPoolDefaultsInput
): GatewayPoolDefaults {
  const retryCount = Number(settings?.default_pool_mode_retry_count)
  const retryStatusCodes = settings?.default_pool_mode_retry_status_codes
  return {
    retryCount: Number.isInteger(retryCount) && retryCount >= 0 && retryCount <= MAX_POOL_MODE_RETRY_COUNT
      ? retryCount
      : DEFAULT_POOL_MODE_RETRY_COUNT,
    retryStatusCodes: Array.isArray(retryStatusCodes)
      ? [...new Set(retryStatusCodes.filter(
          (code) => Number.isInteger(code) && code >= 100 && code <= 599
        ))].sort((left, right) => left - right)
      : [...DEFAULT_POOL_MODE_RETRY_STATUS_CODES]
  }
}

export function parsePoolModeRetryStatusCodes(input: string): number[] {
  if (!input || !input.trim()) return []
  const statusCodes = input
    .split(/[,\s]+/)
    .map((token) => Number(token.trim()))
    .filter((code) => Number.isInteger(code) && code >= 100 && code <= 599)
  return [...new Set(statusCodes)].sort((left, right) => left - right)
}

export function normalizePoolModeRetryCount(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_POOL_MODE_RETRY_COUNT
  return Math.min(MAX_POOL_MODE_RETRY_COUNT, Math.max(0, Math.trunc(value)))
}

export function writePoolModeCredentials(
  credentials: Record<string, unknown>,
  enabled: boolean,
  retryCount: number,
  retryStatusCodesInput: string
) {
  credentials.pool_mode = enabled
  if (!enabled) return

  credentials.pool_mode_retry_count = normalizePoolModeRetryCount(retryCount)
  credentials.pool_mode_retry_status_codes = parsePoolModeRetryStatusCodes(retryStatusCodesInput)
}
