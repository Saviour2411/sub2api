import { apiClient } from './client'
import type { ModelMarketplaceResponse } from '@/types'

export async function getModelMarketplace(options?: {
  signal?: AbortSignal
}): Promise<ModelMarketplaceResponse> {
  const { data } = await apiClient.get<ModelMarketplaceResponse>('/model-marketplace', {
    signal: options?.signal
  })
  return data
}
