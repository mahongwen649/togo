import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface FinanceFilters {
  start_date?: string
  end_date?: string
  group_id?: number
  group_ids?: number[]
  company_id?: number
  company_ids?: number[]
  platform?: string
  page?: number
  page_size?: number
}

export interface FinanceSummaryRow {
  company_id?: number
  company_name?: string
  platform?: string
  group_id: number
  group_name: string
  member_count: number
  request_count: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  image_output_tokens: number
  actual_charged_amount: number
}

export interface FinanceMemberRow {
  company_id?: number
  company_name?: string
  platform?: string
  group_id: number
  group_name: string
  user_id: number
  username: string
  user_email?: string
  user_deleted_at?: string
  request_count: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  image_output_tokens: number
  actual_charged_amount: number
}

export async function summary(filters: FinanceFilters = {}): Promise<FinanceSummaryRow[]> {
  const { data } = await apiClient.get<FinanceSummaryRow[]>('/admin/finance/summary', { params: serializeFinanceFilters(filters) })
  return data
}

export async function members(filters: FinanceFilters = {}): Promise<PaginatedResponse<FinanceMemberRow>> {
  const { data } = await apiClient.get<PaginatedResponse<FinanceMemberRow>>('/admin/finance/members', { params: serializeFinanceFilters(filters) })
  return data
}

export async function memberBreakdown(userId: number, filters: FinanceFilters = {}): Promise<FinanceMemberRow[]> {
  const { data } = await apiClient.get<FinanceMemberRow[]>(`/admin/finance/members/${userId}/breakdown`, { params: serializeFinanceFilters(filters) })
  return data
}

export async function exportCsv(filters: FinanceFilters = {}): Promise<Blob> {
  const response = await apiClient.get('/admin/finance/export', {
    params: serializeFinanceFilters(filters),
    responseType: 'blob'
  })
  return response.data as Blob
}

export async function exportMemberCsv(userId: number, filters: FinanceFilters = {}): Promise<Blob> {
  const response = await apiClient.get(`/admin/finance/members/${userId}/export`, {
    params: serializeFinanceFilters(filters),
    responseType: 'blob'
  })
  return response.data as Blob
}

function serializeFinanceFilters(filters: FinanceFilters): Record<string, unknown> {
  const { group_ids, company_ids, ...rest } = filters
  return {
    ...rest,
    group_ids: group_ids && group_ids.length > 0 ? group_ids.join(',') : undefined,
    company_ids: company_ids && company_ids.length > 0 ? company_ids.join(',') : undefined,
  }
}

export default { summary, members, memberBreakdown, exportCsv, exportMemberCsv }
