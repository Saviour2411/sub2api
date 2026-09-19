import { describe, expect, it } from 'vitest'
import { parseAnthropicStreamDiagnostics } from '../anthropicStreamDiagnostic'

describe('Claude 流诊断解析', () => {
  it('区分提前保活和有效内容交付，兼容旧诊断', () => {
    const rows = parseAnthropicStreamDiagnostics(JSON.stringify([
      { stream_diagnostic: { early_keepalive_sent: true, output_committed: false, wire_status: 200, failure_kind: 'upstream_http_error' } },
      { stream_diagnostic: { output_committed: false, failure_kind: 'client_write_error' } },
    ]))
    expect(rows[0]).toMatchObject({ earlyKeepaliveSent: true, committed: false, wireStatus: 200, reason: '上游HTTP错误' })
    expect(rows[1]).toMatchObject({ earlyKeepaliveSent: false, committed: false, reason: '客户端写入失败' })
  })
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
  it('区分首有效内容超时与流空闲超时', () => {
    const rows = parseAnthropicStreamDiagnostics(JSON.stringify([{
      stream_diagnostic: { failure_kind: 'first_content_timeout', decision: 'retry', logical_status: 504 }
    }, {
      stream_diagnostic: { failure_kind: 'idle_timeout', decision: 'stop', logical_status: 504 }
    }]))
    expect(rows[0]).toMatchObject({ reason: '首有效内容超时', decision: '继续重试', logicalStatus: 504 })
    expect(rows[1]).toMatchObject({ reason: '上游流空闲超时', decision: '停止重试' })
  })
})
