import type { LocationQuery } from 'vue-router'
import type {
  QualityFilters,
  QualityRun,
  QualityTest
} from '@/api/modelQuality'
import { qualityLabels, qualityTestState } from '@/api/modelQuality'

export function qualityFilters(query: LocationQuery): QualityFilters {
  const text = (key: string) =>
    typeof query[key] === 'string' ? (query[key] as string) : ''
  const positive = (key: string, fallback: number) => {
    const value = Number(text(key))
    return Number.isSafeInteger(value) && value > 0 ? value : fallback
  }
  const method = text('method'),
    outcome = text('outcome')
  return {
    group_id: positive('group_id', 0),
    page: positive('page', 1),
    model: text('model'),
    method: ['candy', 'pelican'].includes(method) ? method : '',
    outcome: ['normal', 'degraded', 'failed', 'running', 'unknown'].includes(
      outcome
    )
      ? outcome
      : ''
  }
}
export function qualityFilterQuery(
  filters: QualityFilters
): Record<string, string> {
  const query: Record<string, string> = {}
  if (filters.group_id) query.group_id = String(filters.group_id)
  if (filters.page > 1) query.page = String(filters.page)
  for (const key of ['model', 'method', 'outcome'] as const)
    if (filters[key]) query[key] = filters[key]!
  return query
}
export const qualityMethodName = (method: string) =>
  method === 'pelican' ? '鹈鹕' : '糖果'
export function qualityTestLabel(test: QualityTest) {
  const state = qualityTestState(test)
  return state === 'normal'
    ? '通过'
    : state === 'degraded'
      ? '未通过'
      : qualityLabels[state] || '无法判定'
}
export function qualityDuration(run: QualityRun) {
  return run.finished_at
    ? `${Math.max(0, (Date.parse(run.finished_at) - Date.parse(run.started_at)) / 1000).toFixed(1)} 秒`
    : '进行中'
}
