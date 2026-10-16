import { apiClient } from '../client'
import type { Account, AccountUsageInfo, AccountUsageStatsResponse, ClaudeModel, TempUnschedulableStatus, WindowStats } from '@/types'
export interface OpenAIQuotaUsage {
  user_id?: string
  account_id?: string
  email?: string
  plan_type?: string
  rate_limit?: Record<string, unknown> | null
  additional_rate_limits?: Array<Record<string, unknown>>
  rate_limit_reset_credits?: {
    available_count: number
    credits?: Array<{ expires_at?: string }>
  } | null
  fetched_at: number
  cache_persisted?: boolean
}
export interface OpenAIQuotaResetResult {
  code: string
  credit?: Record<string, unknown> | null
  windows_reset: number
  quota?: OpenAIQuotaUsage | null
  account?: Account | null
  cache_refreshed: boolean
  account_state_recovered: boolean
  warning_code?: string
}
export interface BatchAccountUsageResponse { usage: Record<string, AccountUsageInfo>; errors: Record<string, string> }
export interface BatchTodayStatsResponse { stats: Record<string, WindowStats> }
export async function getUsage(id: number, source?: 'passive' | 'active', force?: boolean): Promise<AccountUsageInfo> { const { data } = await apiClient.get<AccountUsageInfo>(`/account-market/accounts/${id}/usage`, { params: { ...(source ? { source } : {}), ...(force ? { force: true } : {}) } }); return data }
export async function getBatchUsage(accountIds: number[], force = false): Promise<BatchAccountUsageResponse> { const { data } = await apiClient.post<BatchAccountUsageResponse>('/account-market/accounts/usage/batch', { account_ids: accountIds, force }); return data }
export async function getTodayStats(id: number): Promise<WindowStats> { const { data } = await apiClient.get<WindowStats>(`/account-market/accounts/${id}/today-stats`); return data }
export async function getStats(id: number, days: number = 30): Promise<AccountUsageStatsResponse> { const { data } = await apiClient.get<AccountUsageStatsResponse>(`/account-market/accounts/${id}/stats`, { params: { days } }); return data }
export async function getBatchTodayStats(accountIds: number[]): Promise<BatchTodayStatsResponse> { const { data } = await apiClient.post<BatchTodayStatsResponse>('/account-market/accounts/today-stats/batch', { account_ids: accountIds }); return data }
export async function recoverState(id: number): Promise<Account> { const { data } = await apiClient.post<Account>(`/account-market/accounts/${id}/recover`); return data }
export async function getTempUnschedulableStatus(id: number): Promise<TempUnschedulableStatus> { const { data } = await apiClient.get<TempUnschedulableStatus>(`/account-market/accounts/${id}/temp-unschedulable`); return data }
export async function resetTempUnschedulable(id: number): Promise<{ message: string }> { const { data } = await apiClient.delete<{ message: string }>(`/account-market/accounts/${id}/temp-unschedulable`); return data }
export async function setSchedulable(id: number, schedulable: boolean): Promise<Account> { const { data } = await apiClient.post<Account>(`/account-market/accounts/${id}/schedulable`, { schedulable }); return data }
export async function getAvailableModels(id: number): Promise<ClaudeModel[]> { const { data } = await apiClient.get<ClaudeModel[]>(`/account-market/accounts/${id}/models`); return data }
export async function refreshOpenAIQuota(id: number): Promise<OpenAIQuotaUsage> { const { data } = await apiClient.post<OpenAIQuotaUsage>(`/account-market/accounts/${id}/core-action/openai-refresh`); return data }
export async function resetOpenAIQuota(id: number): Promise<OpenAIQuotaResetResult> { const { data } = await apiClient.post<OpenAIQuotaResetResult>(`/account-market/accounts/${id}/core-action/openai-reset`, undefined, { timeout: 90_000 }); return data }
export async function resetAccountQuota(id: number): Promise<Account> { const { data } = await apiClient.post<Account>(`/account-market/accounts/${id}/core-action/account-reset-quota`); return data }
export async function refreshOllamaCloudUsage(id: number) { const { data } = await apiClient.post(`/account-market/accounts/${id}/core-action/ollama-refresh`); return data }
export const accountsAPI = { getUsage, getBatchUsage, getTodayStats, getStats, getBatchTodayStats, recoverState, getTempUnschedulableStatus, resetTempUnschedulable, setSchedulable, getAvailableModels, refreshOpenAIQuota, resetOpenAIQuota, resetAccountQuota, refreshOllamaCloudUsage }
export default accountsAPI
