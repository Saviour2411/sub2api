import type { QualityGroup, QualityRun } from '@/api/modelQuality'

export function qualityRun(
  id = 9,
  overrides: Partial<QualityRun> = {}
): QualityRun {
  return {
    id,
    group_id: 7,
    group_name: '测试分组',
    model: 'model-a',
    reasoning_effort: 'high',
    outcome: 'normal',
    started_at: '2026-10-25T01:00:00Z',
    finished_at: '2026-10-25T01:01:00Z',
    expires_at: '2099-01-01T00:00:00Z',
    tests: [
      {
        benchmark: 'pelican',
        status: 'completed',
        verdict: 'normal',
        reason: '动画完整',
        has_html: true,
        html: '<canvas></canvas>',
        duration_ms: 1000,
        finished_at: '2026-10-25T01:01:00Z'
      },
      {
        benchmark: 'candy',
        status: 'completed',
        verdict: 'normal',
        reason: '回答正确',
        answer: '<script>danger()</script>\n完整糖果回答',
        has_html: false,
        duration_ms: 1000,
        finished_at: '2026-10-25T01:01:00Z'
      }
    ],
    ...overrides
  }
}
export const qualityGroups: QualityGroup[] = [
  {
    group_id: 7,
    name: '测试分组',
    models: ['model-a', 'model-b'],
    methods: ['candy', 'pelican'],
    status: 'normal',
    running: false,
    last_started_at: null,
    next_run_at: null
  },
  {
    group_id: 8,
    name: '另一分组',
    models: ['model-c'],
    methods: ['candy'],
    status: 'paused',
    running: false,
    pause_reason: '待绑定密钥',
    last_started_at: null,
    next_run_at: null
  }
]
export function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}
