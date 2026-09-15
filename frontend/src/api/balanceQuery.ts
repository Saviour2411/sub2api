import { buildApiUrl } from './url'
import type { UserPaymentMethod } from './admin/userCustomizations'

export interface PublicBalanceQuery {
  user: { id: number; username: string; balance: number }
  records: Array<{ created_at: string; amount: number; type: string; note: string }>
  payment_methods: UserPaymentMethod[]
  total: number
  page: number
  page_size: number
}

export class BalanceQueryError extends Error {
  constructor(public status: number, public retryAfter = 0) { super('余额查询失败') }
}

export async function queryPublicBalance(token: string, page = 1, signal?: AbortSignal): Promise<PublicBalanceQuery> {
  // 不使用会附带登录令牌、刷新会话和时区参数的通用客户端。
  const response = await fetch(buildApiUrl(`/public/balance-query?page=${page}&page_size=20`), {
    method: 'GET',
    headers: { 'X-Balance-Query-Token': token },
    credentials: 'omit',
    cache: 'no-store',
    referrerPolicy: 'no-referrer',
    signal
  })
  if (!response.ok) throw new BalanceQueryError(response.status, Number(response.headers.get('Retry-After')) || 0)
  const body = await response.json()
  return body.data as PublicBalanceQuery
}
