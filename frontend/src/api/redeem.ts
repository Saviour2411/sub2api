/**
 * Redeem code API endpoints
 * Handles redeem code redemption for users
 */

import { apiClient } from './client'
import type { RedeemCodeRequest } from '@/types'

export interface RedeemHistoryItem {
  id: number
  code: string
  type: string
  value: number
  status: string
  used_at: string
  created_at: string
  // Notes from admin for admin_balance/admin_concurrency types
  notes?: string
  // Subscription-specific fields
  group_id?: number
  validity_days?: number
  group?: {
    id: number
    name: string
  }
}

export interface DailyCheckinStatus {
  enabled: boolean
  checked_in_today: boolean
  reward_mode: 'fixed' | 'range' | string
  reward_amount: number
  reward_min: number
  reward_max: number
  today_reward?: number
  checked_in_at?: string
}

export interface DailyCheckinResult {
  reward_amount: number
  new_balance: number
  checked_in_at: string
}

/**
 * Redeem a code
 * @param code - Redeem code string
 * @returns Redemption result with updated balance or concurrency
 */
export async function redeem(code: string): Promise<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
}> {
  const payload: RedeemCodeRequest = { code }

  const { data } = await apiClient.post<{
    message: string
    type: string
    value: number
    new_balance?: number
    new_concurrency?: number
  }>('/redeem', payload)

  return data
}

/**
 * Get user's redemption history
 * @returns List of redeemed codes
 */
export async function getHistory(): Promise<RedeemHistoryItem[]> {
  const { data } = await apiClient.get<RedeemHistoryItem[]>('/redeem/history')
  return data
}

export async function getDailyCheckinStatus(): Promise<DailyCheckinStatus> {
  const { data } = await apiClient.get<DailyCheckinStatus>('/user/checkin/status')
  return data
}

export async function dailyCheckin(): Promise<DailyCheckinResult> {
  const { data } = await apiClient.post<DailyCheckinResult>('/user/checkin')
  return data
}

export const redeemAPI = {
  redeem,
  getHistory,
  getDailyCheckinStatus,
  dailyCheckin
}

export default redeemAPI
