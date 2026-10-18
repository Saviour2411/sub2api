import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildEmbeddedUrl, detectTheme } from '../embedded-url'

describe('embedded-url', () => {
  const originalLocation = window.location

  beforeEach(() => {
    Object.defineProperty(window, 'location', {
      value: {
        origin: 'https://app.example.com',
        href: 'https://app.example.com/user/purchase',
      },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
    document.documentElement.classList.remove('dark')
    vi.restoreAllMocks()
  })

  it('默认仅添加展示和来源参数，保留目标 URL 自有参数', () => {
    const result = buildEmbeddedUrl(
      'https://pay.example.com/checkout?plan=pro',
      42,
      'token-123',
      'dark',
      'zh-CN',
    )

    const url = new URL(result)
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.has('user_id')).toBe(false)
    expect(url.searchParams.has('token')).toBe(false)
    expect(url.searchParams.get('theme')).toBe('dark')
    expect(url.searchParams.get('lang')).toBe('zh-CN')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    expect(url.searchParams.get('src_host')).toBe('https://app.example.com')
    expect(url.searchParams.get('src_url')).toBe('https://app.example.com/user/purchase')
  })

  it.each([
    { options: { passUserId: false, passAuthToken: false }, userId: null, token: null },
    { options: { passUserId: true }, userId: '42', token: null },
    { options: { passAuthToken: true }, userId: null, token: 'token-123' },
    { options: { passUserId: true, passAuthToken: true }, userId: '42', token: 'token-123' },
  ])('仅在对应开关显式开启时透传身份：$options', ({ options, userId, token }) => {
    const url = new URL(buildEmbeddedUrl('https://example.com/page', 42, 'token-123', 'light', 'en', options))
    expect(url.searchParams.get('user_id')).toBe(userId)
    expect(url.searchParams.get('token')).toBe(token)
  })

  it.each([{}, { passUserId: true, passAuthToken: true }])('来源 URL 始终清理查询参数和片段：%j', (options) => {
    window.location.href = 'https://app.example.com/custom/docs?token=query-secret&code=oauth-secret#access_token=fragment-secret'
    const result = buildEmbeddedUrl('https://example.com/page?plan=pro#section', 42, 'token-123', 'dark', 'zh-CN', options)
    const url = new URL(result)
    expect(url.searchParams.get('src_url')).toBe('https://app.example.com/custom/docs')
    expect(url.searchParams.get('src_host')).toBe('https://app.example.com')
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.hash).toBe('#section')
    expect(result).not.toMatch(/query-secret|oauth-secret|fragment-secret/)
  })

  it('不覆盖目标页面自行配置的身份参数', () => {
    const url = new URL(buildEmbeddedUrl('https://example.com/page?token=page-token&user_id=page-user', 42, 'login-token'))
    expect(url.searchParams.get('token')).toBe('page-token')
    expect(url.searchParams.get('user_id')).toBe('page-user')
  })

  it('omits optional params when they are empty', () => {
    const result = buildEmbeddedUrl('https://pay.example.com/checkout', undefined, '', 'light', undefined, {
      passUserId: true,
      passAuthToken: true,
    })

    const url = new URL(result)
    expect(url.searchParams.get('theme')).toBe('light')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    expect(url.searchParams.has('user_id')).toBe(false)
    expect(url.searchParams.has('token')).toBe(false)
    expect(url.searchParams.has('lang')).toBe(false)
  })

  it('returns original string for invalid url input', () => {
    expect(buildEmbeddedUrl('not a url', 1, 'token')).toBe('not a url')
  })

  it.each(['', 'javascript:alert(1)', 'data:text/html,test', 'ftp://example.com/file'])('不向非 HTTP 页面添加身份参数：%s', (baseUrl) => {
    expect(buildEmbeddedUrl(baseUrl, 42, 'token-123', 'light', 'en', {
      passUserId: true,
      passAuthToken: true,
    })).toBe(baseUrl)
  })

  it('detects dark mode from document root class', () => {
    document.documentElement.classList.add('dark')
    expect(detectTheme()).toBe('dark')
  })
})
