import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BalanceQueryError, queryPublicBalance } from '../balanceQuery'

describe('公开余额查询客户端', () => {
  const fetchMock = vi.fn()
  beforeEach(() => { vi.stubGlobal('fetch', fetchMock); fetchMock.mockReset() })

  it('随机码仅通过请求头发送，不携带登录凭据或缓存', async () => {
    const data = { user: { id: 9, balance: 10 }, records: [] }
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ data })))
    expect(await queryPublicBalance('随机码', 2)).toEqual(data)
    const [url, options] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/public/balance-query?page=2&page_size=20')
    expect(url).not.toContain('随机码')
    expect(options).toMatchObject({ credentials: 'omit', cache: 'no-store', referrerPolicy: 'no-referrer', headers: { 'X-Balance-Query-Token': '随机码' } })
    expect(options.headers).not.toHaveProperty('Authorization')
  })

  it.each([404, 429, 503])('保留 %i 状态供页面区分错误', async (status) => {
    fetchMock.mockResolvedValue(new Response('{}', { status, headers: { 'Retry-After': '12' } }))
    await expect(queryPublicBalance('随机码')).rejects.toMatchObject({ status, retryAfter: 12 })
    await expect(queryPublicBalance('随机码')).rejects.toBeInstanceOf(BalanceQueryError)
  })
})
