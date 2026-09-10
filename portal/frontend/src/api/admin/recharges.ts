import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface RechargeFilters {
  start_date?: string
  end_date?: string
  keyword?: string
  status?: string
  payment_type?: string
  order_type?: string
  near_time?: string
  near_minutes?: number
  granularity?: 'hour' | 'day' | 'month'
  page?: number
  page_size?: number
}

export interface RechargeOrder {
  id: number
  user_id: number
  user_name: string
  user_email: string
  current_balance: number
  total_recharged?: number
  amount: number
  pay_amount: number
  payment_type: string
  status: string
  order_type: string
  created_at: string
  paid_at?: string | null
  completed_at?: string | null
  effective_time: string
  payment_trade_no?: string
  out_trade_no?: string
}

export interface RechargeSummary {
  total_amount: number
  total_pay_amount: number
  all_time_total_recharged?: number
  recharge_users_balance?: number
  order_count: number
  user_count: number
  average_pay_amount: number
  max_pay_amount: number
  latest?: RechargeOrder | null
  start_time: string
  end_time: string
  granularity: 'hour' | 'day' | 'month'
}

export interface RechargeUserStat {
  user_id: number
  user_name: string
  user_email: string
  current_balance: number
  total_amount: number
  total_pay_amount: number
  order_count: number
  average_pay_amount: number
  max_pay_amount: number
  latest_time: string
}

export interface RechargeTimeStat {
  bucket: string
  total_amount: number
  total_pay_amount: number
  order_count: number
  user_count: number
}

export interface RechargeResponse extends PaginatedResponse<RechargeOrder> {
  summary: RechargeSummary
  users: RechargeUserStat[]
  users_total: number
  users_pages: number
  timeseries: RechargeTimeStat[]
  timeseries_total: number
  timeseries_pages: number
}

export async function list(filters: RechargeFilters = {}): Promise<RechargeResponse> {
  const { data } = await apiClient.get<RechargeResponse>('/admin/recharges', {
    params: cleanFilters(filters)
  })
  return data
}

function cleanFilters(filters: RechargeFilters): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(filters).filter(([, value]) => value !== undefined && value !== null && value !== '')
  )
}

export default { list }
