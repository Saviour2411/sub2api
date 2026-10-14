import { apiClient } from './client'

export type QualityMethod = 'candy' | 'pelican'
export interface QualityGroupConfig {
  group_id: number
  key_id: number | null
  interval_minutes: number
  methods: QualityMethod[]
  models: string[]
  reasoning_effort: 'low' | 'medium' | 'high'
}
export interface QualityConfig {
  enabled: boolean
  version: number
  groups: QualityGroupConfig[]
  endpoint?: string
  pause_reason?: string
}
export interface QualityGroup {
  group_id: number
  name: string
  models: string[]
  methods: QualityMethod[]
  status: string
  running: boolean
  pause_reason?: string
  last_started_at: string | null
  next_run_at: string | null
}
export interface QualityTest {
  benchmark: QualityMethod
  status: string
  verdict: string
  reason: string
  answer?: string
  html?: string
  has_html: boolean
  duration_ms: number
  input_tokens?: number
  output_tokens?: number
  reasoning_tokens?: number
  finished_at: string | null
}
export interface QualityRun {
  id: number
  group_id: number
  group_name: string
  model: string
  reasoning_effort: string
  outcome: string
  started_at: string
  finished_at: string | null
  expires_at: string
  tests: QualityTest[]
}
export interface QualityKey {
  id: number
  name: string
  masked_key: string
}
export interface QualityFilters {
  group_id: number
  model?: string
  method?: string
  outcome?: string
  page: number
}
export interface QualityPage {
  items: QualityRun[]
  total: number
  page: number
  page_size: number
  pages: number
}

export const modelQualityAPI = {
  async config() {
    return (await apiClient.get<QualityConfig>('/admin/custom-features/model-quality')).data
  },
  async save(config: QualityConfig) {
    return (await apiClient.put<QualityConfig>('/admin/custom-features/model-quality', config)).data
  },
  async keys(group_id: number) {
    return (
      await apiClient.get<QualityKey[]>('/admin/custom-features/model-quality/keys', {
        params: { group_id }
      })
    ).data
  },
  async runNow(id: number) {
    return (await apiClient.post(`/admin/model-quality/groups/${id}/run`)).data
  },
  async groups(signal?: AbortSignal) {
    return (await apiClient.get<QualityGroup[]>('/model-quality/groups', { signal })).data
  },
  async runs(params: QualityFilters, signal?: AbortSignal) {
    return (await apiClient.get<QualityPage>('/model-quality/runs', { params, signal })).data
  },
  async detail(id: number, signal?: AbortSignal) {
    return (await apiClient.get<QualityRun>(`/model-quality/runs/${id}`, { signal })).data
  }
}

export const qualityLabels: Record<string, string> = {
  normal: '正常',
  degraded: '疑似降智',
  failed: '检测失败',
  running: '检测中',
  pending: '等待检测',
  submitting: '提交中',
  uncertain: '提交异常',
  cancelled: '已取消',
  unknown: '无法判定',
  paused: '已暂停',
  completed: '已完成'
}
export function qualityTestState(test: QualityTest) {
  return test.status === 'completed' ? test.verdict : test.status
}
export function qualityTime(value?: string | null) {
  return value
    ? new Date(value).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
      })
    : '—'
}
