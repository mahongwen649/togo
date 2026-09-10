/**
 * Redeem code API endpoints
 * Handles redeem code redemption for users
 */

import { apiClient } from './client'
import type { RedeemCodeRequest } from '@/types'

let pendingRedeemOperation: { code: string; key: string } | null = null

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
	const normalizedCode = code.trim()
	const payload: RedeemCodeRequest = { code: normalizedCode }
	if (!pendingRedeemOperation || pendingRedeemOperation.code !== normalizedCode) {
		pendingRedeemOperation = {
			code: normalizedCode,
			key: `redeem-${crypto.randomUUID()}`
		}
	}

	try {
		const { data } = await apiClient.post<{
			message: string
			type: string
			value: number
			new_balance?: number
			new_concurrency?: number
		}>('/redeem', payload, { headers: { 'Idempotency-Key': pendingRedeemOperation.key } })
		pendingRedeemOperation = null
		return data
	} catch (error: any) {
		const status = Number(error?.status || error?.response?.status || 0)
		if (status > 0 && status !== 502 && status !== 503 && status !== 504) {
			pendingRedeemOperation = null
		}
		throw error
	}
}

/**
 * Get user's redemption history
 * @returns List of redeemed codes
 */
export async function getHistory(): Promise<RedeemHistoryItem[]> {
  const { data } = await apiClient.get<RedeemHistoryItem[]>('/redeem/history')
  return data
}

export const redeemAPI = {
  redeem,
  getHistory
}

export default redeemAPI
