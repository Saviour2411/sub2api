/**
 * 管理端兑换码 API。
 * 处理管理员生成和管理兑换码的请求。
 */

import { apiClient } from '../client'
import type {
  RedeemCode,
  RedeemCodeUsage,
  GenerateRedeemCodesRequest,
  GenerateRedeemCodeType,
  BatchUpdateRedeemCodeFields,
  RedeemCodeType,
  PaginatedResponse
} from '@/types'

/**
 * 分页查询兑换码列表。
 * @param page 页码，默认 1
 * @param pageSize 每页数量，默认 20
 * @param filters 可选筛选条件
 * @returns 分页兑换码列表
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    type?: RedeemCodeType
    status?: 'active' | 'used' | 'expired' | 'unused' | 'disabled'
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<RedeemCode>> {
  const { data } = await apiClient.get<PaginatedResponse<RedeemCode>>('/admin/redeem-codes', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

/**
 * 按 ID 查询兑换码详情。
 * @param id 兑换码 ID
 * @returns 兑换码详情
 */
export async function getById(id: number): Promise<RedeemCode> {
  const { data } = await apiClient.get<RedeemCode>(`/admin/redeem-codes/${id}`)
  return data
}

/**
 * 生成兑换码。
 * @param count 生成数量
 * @param type 兑换码类型
 * @param value 面值
 * @param groupId 订阅分组 ID，订阅类型必填
 * @param validityDays 订阅有效天数
 * @param expiresInDays 兑换码自身过期天数
 * @returns 生成后的兑换码列表
 */
export async function generate(
  count: number,
  type: GenerateRedeemCodeType,
  value: number,
  maxUses?: number,
  groupId?: number | null,
  validityDays?: number,
  expiresInDays?: number | null
): Promise<RedeemCode[]> {
  const payload: GenerateRedeemCodesRequest = {
    count,
    type,
    value,
    max_uses: maxUses && maxUses > 1 ? maxUses : undefined
  }

  // 订阅类型专用字段
  if (type === 'subscription') {
    payload.group_id = groupId
    if (validityDays && validityDays > 0) {
      payload.validity_days = validityDays
    }
  }
  if (expiresInDays && expiresInDays > 0) {
    payload.expires_in_days = expiresInDays
  }

  const { data } = await apiClient.post<RedeemCode[]>('/admin/redeem-codes/generate', payload)
  return data
}

export async function getUsages(
  id: number,
  page: number = 1,
  pageSize: number = 20
): Promise<PaginatedResponse<RedeemCodeUsage>> {
  const { data } = await apiClient.get<PaginatedResponse<RedeemCodeUsage>>(
    `/admin/redeem-codes/${id}/usages`,
    {
      params: {
        page,
        page_size: pageSize
      }
    }
  )
  return data
}

/**
 * 删除兑换码。
 * @param id 兑换码 ID
 * @returns 删除结果
 */
export async function deleteCode(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/redeem-codes/${id}`)
  return data
}

/**
 * 批量删除兑换码。
 * @param ids 兑换码 ID 列表
 * @returns 删除结果
 */
export async function batchDelete(ids: number[]): Promise<{
  deleted: number
  message: string
}> {
  const { data } = await apiClient.post<{
    deleted: number
    message: string
  }>('/admin/redeem-codes/batch-delete', { ids })
  return data
}

/**
 * 批量更新兑换码字段。
 * @param ids 兑换码 ID 列表
 * @param fields 要更新的字段集合
 * @returns 更新结果
 */
export async function batchUpdate(
  ids: number[],
  fields: BatchUpdateRedeemCodeFields
): Promise<{
  updated: number
  message: string
}> {
  const { data } = await apiClient.post<{
    updated: number
    message: string
  }>('/admin/redeem-codes/batch-update', { ids, fields })
  return data
}

/**
 * 手动过期兑换码。
 * @param id 兑换码 ID
 * @returns 更新后的兑换码
 */
export async function expire(id: number): Promise<RedeemCode> {
  const { data } = await apiClient.post<RedeemCode>(`/admin/redeem-codes/${id}/expire`)
  return data
}

/**
 * 查询兑换码统计。
 * @returns 兑换码统计信息
 */
export async function getStats(): Promise<{
  total_codes: number
  active_codes: number
  used_codes: number
  expired_codes: number
  total_value_distributed: number
  by_type: Record<RedeemCodeType, number>
}> {
  const { data } = await apiClient.get<{
    total_codes: number
    active_codes: number
    used_codes: number
    expired_codes: number
    total_value_distributed: number
    by_type: Record<RedeemCodeType, number>
  }>('/admin/redeem-codes/stats')
  return data
}

/**
 * 导出兑换码 CSV。
 * @param filters 可选筛选条件
 * @returns CSV Blob
 */
export async function exportCodes(filters?: {
  type?: RedeemCodeType
  status?: 'used' | 'expired' | 'unused' | 'disabled'
  search?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}): Promise<Blob> {
  const response = await apiClient.get('/admin/redeem-codes/export', {
    params: filters,
    responseType: 'blob'
  })
  return response.data
}

export const redeemAPI = {
  list,
  getById,
  generate,
  delete: deleteCode,
  batchDelete,
  batchUpdate,
  expire,
  getUsages,
  getStats,
  exportCodes
}

export default redeemAPI
