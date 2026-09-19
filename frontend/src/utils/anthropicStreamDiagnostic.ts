// 仅提取明确允许展示的诊断字段，避免把事件里的任意正文透传到页面。
export interface AnthropicStreamDiagnosticRow {
  attempt: number
  account: string
  upstreamRequestId: string
  time: number
  upstreamStatus: number
  wireStatus: number
  logicalStatus: number
  event: string
  terminal: boolean
  committed: boolean
  earlyKeepaliveSent: boolean
  pendingFrameBytes: number
  terminalCandidate: boolean
  preludeBytes: number
  elapsedMs: number
  remainingMs: number
  lastReadAgeMs: number
  decision: string
  reason: string
  recovered: boolean
}
const text = (v: unknown): string => typeof v === 'string' ? v.slice(0, 160) : ''
const number = (v: unknown): number => typeof v === 'number' && Number.isFinite(v) ? v : 0
const reasons: Record<string, string> = {
  output_committed: '已交付内容', client_canceled: '客户端取消', replay_forbidden: '禁止重放',
  retries_exhausted: '次数耗尽', budget_exhausted: '预算耗尽', no_available_account: '无可用账号',
  prelude_overflow: '缓存超限', idle_timeout: '上游流空闲超时', empty_stream: '空流',
  missing_terminal: '缺终止事件', truncated_event: '事件不完整', invalid_json: '事件JSON无效',
  invalid_event: '事件无效', event_type_mismatch: '事件类型不一致', upstream_error_event: '上游错误事件',
  stream_read_error: '上游流读取失败', first_token_timeout: '首 Token 超时', first_content_timeout: '首有效内容超时',
  client_write_error: '客户端写入失败', upstream_http_error: '上游HTTP错误'
}
export function parseAnthropicStreamDiagnostics(raw?: string): AnthropicStreamDiagnosticRow[] {
  if (!raw) return []
  try {
    const events: unknown = JSON.parse(raw)
    if (!Array.isArray(events)) return []
    return events.slice(-20).flatMap(event => {
      if (!event || typeof event !== 'object') return []
      const d = event.stream_diagnostic
      if (!d || typeof d !== 'object') return []
      const reason = text(d.stop_reason || d.failure_kind)
      return [{
        attempt: number(d.attempt), account: String(number(event.account_id)), upstreamRequestId: text(event.upstream_request_id),
        time: number(event.at_unix_ms), upstreamStatus: number(event.upstream_status_code), wireStatus: number(d.wire_status),
        logicalStatus: number(d.logical_status), event: text(d.last_event_type), terminal: d.terminal_complete === true,
        committed: d.output_committed === true, pendingFrameBytes: number(d.pending_frame_bytes),
        earlyKeepaliveSent: d.early_keepalive_sent === true,
        terminalCandidate: d.terminal_candidate_seen === true, preludeBytes: number(d.prelude_bytes), elapsedMs: number(d.elapsed_ms),
        remainingMs: number(d.budget_remaining_ms), lastReadAgeMs: number(d.last_read_age_ms),
        decision: d.decision === 'retry' ? '继续重试' : '停止重试', reason: reasons[reason] || reason,
        recovered: d.recovered === true
      }]
    })
  } catch { return [] }
}
