import apiClient from '../client'
import type { PaginatedResponse } from '@/types'

export interface UserPaymentMethod { key: string; value: string }
export interface UserCreditSettings {
  auto_credit_enabled: boolean
  credit_threshold: number
  credit_amount: number
}
export interface UserCustomization extends UserCreditSettings {
  user_id: number
  username: string
  email: string
  balance: number
  status: string
  has_link: boolean
  link_enabled: boolean
  link_version: number
  credit_generation: number
  credit_used_at: string | null
}

const base = '/admin/custom-features/user-customizations'
const writeOptions = () => ({ headers: { 'Idempotency-Key': `user-customization-${crypto.randomUUID()}` } })

export default {
  async list(search = '', page = 1): Promise<PaginatedResponse<UserCustomization>> {
    return (await apiClient.get<PaginatedResponse<UserCustomization>>(base, { params: { search, page, page_size: 20 } })).data
  },
  async save(id: number, input: UserCreditSettings): Promise<UserCustomization> {
    return (await apiClient.put<UserCustomization>(`${base}/${id}`, input, writeOptions())).data
  },
  async rotateLink(user: UserCustomization): Promise<UserCustomization> {
    return (await apiClient.post<UserCustomization>(`${base}/${user.user_id}/link`, { version: user.link_version }, writeOptions())).data
  },
  async setLinkEnabled(user: UserCustomization, enabled: boolean): Promise<UserCustomization> {
    return (await apiClient.patch<UserCustomization>(`${base}/${user.user_id}/link`, { version: user.link_version, enabled }, writeOptions())).data
  },
  async getLink(id: number): Promise<string> {
    const { data } = await apiClient.get<{ token: string }>(`${base}/${id}/link`)
    return new URL(`/balance-query#${data.token}`, window.location.origin).href
  },
  async restoreCredit(user: UserCustomization): Promise<UserCustomization> {
    return (await apiClient.post<UserCustomization>(`${base}/${user.user_id}/restore-credit`, { generation: user.credit_generation }, writeOptions())).data
  },
  async paymentMethods(): Promise<UserPaymentMethod[]> {
    return (await apiClient.get<{ payment_methods: UserPaymentMethod[] }>(`${base}/payment-methods`)).data.payment_methods
  },
  async savePaymentMethods(payment_methods: UserPaymentMethod[]): Promise<UserPaymentMethod[]> {
    return (await apiClient.put<{ payment_methods: UserPaymentMethod[] }>(`${base}/payment-methods`, { payment_methods }, writeOptions())).data.payment_methods
  }
}
