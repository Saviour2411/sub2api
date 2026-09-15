import { describe, expect, it } from 'vitest'
import { parseAnthropicStreamDiagnostics } from '../anthropicStreamDiagnostic'

describe('Claude 流诊断解析', () => {
  it('展示停止原因、HTTP真实状态和恢复情况，但不透传正文', () => {
    const rows = parseAnthropicStreamDiagnostics(JSON.stringify([{
      account_id: 740, upstream_status_code: 200, upstream_request_id: 'rid', at_unix_ms: 123,
      upstream_response_body: '不得展示的正文',
      stream_diagnostic: { attempt: 2, stop_reason: 'budget_exhausted', wire_status: 504, logical_status: 504,
        recovered: true, elapsed_ms: 300000, budget_remaining_ms: 0, last_event_type: 'ping', content: '私密思考' }
    }]))
    expect(rows[0]).toMatchObject({ attempt: 2, reason: '预算耗尽', upstreamStatus: 200, wireStatus: 504, recovered: true })
    expect(JSON.stringify(rows)).not.toContain('私密思考')
    expect(JSON.stringify(rows)).not.toContain('不得展示的正文')
  })
  it('兼容旧事件、空值和非法JSON', () => {
    for (const raw of [undefined, '', 'null', '{}', '[{}]', '{bad}']) expect(parseAnthropicStreamDiagnostics(raw)).toEqual([])
  })
})
