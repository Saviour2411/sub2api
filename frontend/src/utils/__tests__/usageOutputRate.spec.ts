import { describe, expect, it } from 'vitest'
import { calculateUsageOutputRate, formatUsageOutputRate } from '../usageOutputRate'
import type { UsageLog } from '@/types'

const row = { stream: true, request_type: 'stream', output_tokens: 1000, duration_ms: 12000, first_token_ms: 2000, image_count: 0, image_output_tokens: 0, billing_mode: 'token' } as UsageLog

describe('使用记录输出速率', () => {
  it('扣除首字等待并使用 tk/s 单位', () => {
    expect(calculateUsageOutputRate(row)).toBe(100)
    expect(formatUsageOutputRate(row)).toBe('100.00 tk/s')
    expect(formatUsageOutputRate({ ...row, output_tokens: 1234 })).toBe('123.40 tk/s')
  })
  it('支持 WS 文本逐轮和旧版流式标记', () => {
    expect(calculateUsageOutputRate({ ...row, request_type: 'ws_v2' })).toBe(100)
    expect(calculateUsageOutputRate({ ...row, request_type: undefined })).toBe(100)
    expect(calculateUsageOutputRate({ ...row, request_type: undefined, openai_ws_mode: true })).toBe(100)
    expect(calculateUsageOutputRate({ ...row, first_token_ms: 0 })).toBeCloseTo(83.333)
  })
  it.each([
    { request_type: 'sync' }, { request_type: 'live' }, { request_type: 'cyber' }, { request_type: 'unknown' },
    { duration_ms: null }, { first_token_ms: null }, { first_token_ms: -1 }, { first_token_ms: 12000 },
    { first_token_ms: 15000 }, { duration_ms: Infinity }, { first_token_ms: NaN },
    { output_tokens: 0 }, { output_tokens: -1 }, { output_tokens: NaN },
    { image_count: 1 }, { image_output_tokens: 5 }, { billing_mode: 'image' }, { billing_mode: 'video' }
  ])('无效或非文本数据不计算：%j', (change) => {
    expect(formatUsageOutputRate({ ...row, ...change } as UsageLog)).toBe('-')
  })
})
