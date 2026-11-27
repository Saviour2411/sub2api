import { apiClient } from './client'

export type QualityRunID = number | string
export type QualityMethod = string
export type QualityProtocol = '' | 'chat_completions' | 'responses' | 'messages' | 'gemini'
export interface QualityCall {
  group_id: number
  key_id: number | null
  protocol: QualityProtocol
  model: string
  reasoning_effort?: string
  thinking_mode?: string
  thinking_budget?: number | null
  max_output_tokens: number
  chat_token_field?: string
  timeout_seconds: number
  extra_body?: Record<string, unknown>
}
export interface QualityQuestion {
  id: string
  name: string
  prompt: string
  system_prompt?: string
  output_type: 'text' | 'svg' | 'html'
  rule: {
    kind: 'number' | 'string' | 'model'
    expected?: string
    judge_prompt?: string
    judge?: QualityCall
  }
}
export interface QualityTarget extends QualityCall {
  id: string
  question_ids: string[]
}
export interface QualityGroupConfig {
  group_id: number
  interval_minutes: number
  targets: QualityTarget[]
}
export interface QualityConfig {
  schema_version: number
  enabled: boolean
  version: number
  questions: QualityQuestion[]
  groups: QualityGroupConfig[]
  endpoint?: string
  pause_reason?: string
  initialization_required: boolean
}
export interface QualityPhase {
  status: string
  answer?: string
  reason?: string
  finish_reason?: string
  duration_ms: number
  input_tokens?: number
  output_tokens?: number
  reasoning_tokens?: number
  attempts: number
  finished_at?: string
}
export const newQualityCall = (group_id = 0): QualityCall => ({
  group_id,
  key_id: null,
  protocol: '',
  model: '',
  chat_token_field: 'max_completion_tokens',
  max_output_tokens: 8192,
  timeout_seconds: 600
})
export interface QualityGroup {
  group_id: number
  name: string
  models: string[]
  methods: string[]
  questions?: { id: string; name: string; output_type: string }[]
  status: string
  running: boolean
  pause_reason?: string
  last_started_at: string | null
  next_run_at: string | null
}
export interface QualityTest {
  benchmark: string
  id?: string
  name?: string
  output_type?: string
  rule_kind?: string
  judge_model?: string
  judge_protocol?: string
  generation?: QualityPhase
  judgment?: QualityPhase
  previous_judgments?: QualityPhase[]
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
  id: QualityRunID
  source?: string
  protocol?: string
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
    return (await apiClient.get<QualityConfig>('/admin/custom-features/model-quality/v2')).data
  },
  async save(config: QualityConfig) {
    return (await apiClient.put<QualityConfig>('/admin/custom-features/model-quality/v2', config))
      .data
  },
  async keys(group_id: number) {
    return (
      await apiClient.get<QualityKey[]>('/admin/custom-features/model-quality/v2/keys', {
        params: { group_id }
      })
    ).data
  },
  async runNow(id: number) {
    return (await apiClient.post(`/admin/model-quality/v2/groups/${id}/run`)).data
  },
  async rejudge(id: string) {
    return (await apiClient.post(`/admin/model-quality/v2/tasks/${id}/rejudge`)).data
  },
  async groups(signal?: AbortSignal) {
    return (
      await apiClient.get<QualityGroup[]>('/model-quality/v2/groups', {
        signal
      })
    ).data
  },
  async runs(params: QualityFilters, signal?: AbortSignal) {
    return (
      await apiClient.get<QualityPage>('/model-quality/v2/runs', {
        params,
        signal
      })
    ).data
  },
  async detail(id: QualityRunID, signal?: AbortSignal) {
    const run = (await apiClient.get<QualityRun>(`/model-quality/v2/runs/${id}`, { signal })).data
    return { ...run, id }
  }
}

export const qualityLabels: Record<string, string> = {
  normal: '正常',
  degraded: '疑似降智',
  failed: '检测失败',
  running: '检测中',
  generating: '生成中',
  ready_judge: '等待判题',
  judging: '判题中',
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
