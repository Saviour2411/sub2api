import { apiClient } from './client'
import type { PublicModelPricingResponse } from '@/types'

export async function getModelPricing(options?: {
  signal?: AbortSignal
}): Promise<PublicModelPricingResponse> {
  const { data } = await apiClient.get<PublicModelPricingResponse>('/model-pricing', {
    signal: options?.signal,
  })
  return data
}
