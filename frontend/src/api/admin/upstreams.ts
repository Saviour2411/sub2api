import { apiClient } from '../client'

export type UpstreamPlatform = 'sub2api' | 'newapi'
export type UpstreamAuthMode = 'password' | 'token'
export type UpstreamStatus = 'pending' | 'syncing' | 'healthy' | 'error'

export interface UpstreamSite {
  id: number
  name: string
  base_url: string
  platform: UpstreamPlatform
  auth_mode: UpstreamAuthMode
  account: string
  enabled: boolean
  status: UpstreamStatus
  error_message: string | null
  balance_usd: number | null
  today_tokens: number
  today_cost_usd: number
  total_tokens: number
  total_cost_usd: number
  tracking_started_at: string
  last_synced_at: string | null
  created_at: string
  updated_at: string
  has_password: boolean
  has_token: boolean
}

export interface UpstreamGroup {
  id: number
  site_id: number
  remote_id: string
  name: string
  platform: string
  multiplier: number | null
  today_tokens: number
  today_cost_usd: number
  last_synced_at: string
}

export interface UpstreamDailyStat {
  id: number
  site_id: number
  date: string
  balance_usd: number | null
  tokens: number
  cost_usd: number
  created_at: string
  updated_at: string
}

export interface UpstreamWritePayload {
  name: string
  base_url: string
  platform: UpstreamPlatform
  auth_mode: UpstreamAuthMode
  account: string
  password?: string
  access_token?: string
  refresh_token?: string
  enabled: boolean
}

export interface UpstreamListParams {
  page?: number
  page_size?: number
  search?: string
  platform?: UpstreamPlatform | ''
  enabled?: boolean
}

export interface PaginatedUpstreams {
  items: UpstreamSite[]
  total: number
  page: number
  page_size: number
  pages: number
}

export async function list(params: UpstreamListParams = {}): Promise<PaginatedUpstreams> {
  const { data } = await apiClient.get<PaginatedUpstreams>('/admin/custom-features/upstreams', { params })
  return data
}

export async function create(payload: UpstreamWritePayload): Promise<UpstreamSite> {
  const { data } = await apiClient.post<UpstreamSite>('/admin/custom-features/upstreams', payload, { timeout: 60000 })
  return data
}

export async function update(id: number, payload: UpstreamWritePayload): Promise<UpstreamSite> {
  const { data } = await apiClient.put<UpstreamSite>(`/admin/custom-features/upstreams/${id}`, payload, { timeout: 60000 })
  return data
}

export async function setEnabled(id: number, enabled: boolean): Promise<UpstreamSite> {
  const { data } = await apiClient.patch<UpstreamSite>(`/admin/custom-features/upstreams/${id}/enabled`, { enabled })
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/custom-features/upstreams/${id}`)
}

export async function sync(id: number): Promise<void> {
  await apiClient.post(`/admin/custom-features/upstreams/${id}/sync`)
}

export async function syncAll(): Promise<{ queued: number }> {
  const { data } = await apiClient.post<{ queued: number }>('/admin/custom-features/upstreams/sync-all')
  return data
}

export async function groups(id: number): Promise<UpstreamGroup[]> {
  const { data } = await apiClient.get<UpstreamGroup[]>(`/admin/custom-features/upstreams/${id}/groups`)
  return data
}

export async function history(id: number, days: 7 | 30 | 90): Promise<UpstreamDailyStat[]> {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - days + 1)
  const dateOnly = (value: Date) => {
    const year = value.getFullYear()
    const month = String(value.getMonth() + 1).padStart(2, '0')
    const day = String(value.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }
  const { data } = await apiClient.get<UpstreamDailyStat[]>(`/admin/custom-features/upstreams/${id}/history`, {
    params: { from: dateOnly(start), to: dateOnly(end) }
  })
  return data
}

export default { list, create, update, setEnabled, remove, sync, syncAll, groups, history }
