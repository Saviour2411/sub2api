import { apiClient } from '../client'

export interface SystemVersionInfo {
  version: string
  upstream_repo?: string
  upstream_version?: string
  upstream_commit?: string
  upstream_synced_at?: string
}

export interface UpstreamVersionInfo {
  latest_version: string
  release_url: string
  has_update: boolean | null
  checked_at: string | null
  status: 'ok' | 'unavailable'
  stale: boolean
}

// 构建版本与发布检查分离，外部网络故障不会阻塞当前版本展示。
export async function getVersion(signal?: AbortSignal): Promise<SystemVersionInfo> {
  const { data } = signal
    ? await apiClient.get<SystemVersionInfo>('/admin/system/version', { signal })
    : await apiClient.get<SystemVersionInfo>('/admin/system/version')
  return data
}

export async function getUpstreamVersion(signal?: AbortSignal): Promise<UpstreamVersionInfo> {
  const { data } = await apiClient.get<UpstreamVersionInfo>('/admin/system/upstream-version', { signal })
  return data
}

export const systemAPI = { getVersion, getUpstreamVersion }

export default systemAPI
