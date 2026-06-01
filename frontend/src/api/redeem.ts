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
  prizes?: DailyCheckinPrize[]
  decay?: DailyCheckinDecay
  today_result?: DailyCheckinReward
  recent_records?: DailyCheckinRecord[]
}

export interface DailyCheckinResult {
  reward_amount: number
  new_balance: number
  checked_in_at: string
  prize?: DailyCheckinReward
  prizes?: DailyCheckinPrize[]
  decay?: DailyCheckinDecay
}

export interface DailyCheckinPrize {
  id: string
  name: string
  type: 'balance' | 'concurrency' | 'subscription' | 'none' | string
  probability_bps: number
  effective_probability_bps: number
  enabled: boolean
  sort_order: number
  balance_mode?: 'fixed' | 'range' | string
  amount?: number
  min_amount?: number
  max_amount?: number
  concurrency?: number
  group_id?: number
  validity_days?: number
}

export interface DailyCheckinDecay {
  paid: boolean
  exempt: boolean
  exempt_reason?: string
  account_age_days: number
  factor_bps: number
  full_days: number
}

export interface DailyCheckinReward {
  prize_id: string
  prize_name: string
  type: string
  amount?: number
  new_balance?: number
  concurrency?: number
  new_concurrency?: number
  group_id?: number
  group_name?: string
  validity_days?: number
  subscription_expires_at?: string
  checked_in_at: string
}

export interface DailyCheckinRecord {
  id: number
  prize_id: string
  prize_name: string
  type: string
  amount?: number
  concurrency?: number
  group_id?: number
  validity_days?: number
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
