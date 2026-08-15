import { describe, expect, it } from 'vitest'

import { isTrustedCanvasMessage } from '@/utils/canvasBridge'

describe('无限画布消息桥', () => {
  it('仅接受同源且来自目标 iframe 的消息', () => {
    const frameWindow = window
    expect(isTrustedCanvasMessage({ origin: window.location.origin, source: frameWindow }, frameWindow)).toBe(true)
    expect(isTrustedCanvasMessage({ origin: 'https://attacker.example', source: frameWindow }, frameWindow)).toBe(false)
    expect(isTrustedCanvasMessage({ origin: window.location.origin, source: null }, frameWindow)).toBe(false)
  })
})
