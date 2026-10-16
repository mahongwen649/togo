import { apiClient, buildApiUrl } from '../client'

export interface CheckAccount {
  id: number
  name: string
  platform: string
  group_ids: number[] | null
  groups: Array<{ id: number; name: string }> | null
}

export interface AccountCheckResult {
  attempts?: number
  text: string
  html: string
  model: string
  error?: string
}

export interface AccountCheckRecord {
  id: number
  account: CheckAccount
  prompt: string
  model: string
  state: 'running' | 'succeeded' | 'failed' | 'cancelled'
  result: AccountCheckResult
  started_at: string
  finished_at: string | null
}

export async function listAccountCheckHistory(accountId?: number) {
  const path = accountId == null ? '/admin/account-checks/history' : `/admin/account-checks/accounts/${accountId}/history`
  const { data } = await apiClient.get<AccountCheckRecord[]>(path)
  return data || []
}

export async function getAccountCheckSelection() {
  const { data } = await apiClient.get<{ account_ids: number[] | null }>('/admin/account-checks/selection')
  return data.account_ids
}

export async function saveAccountCheckSelection(accountIds: number[]) {
  await apiClient.put('/admin/account-checks/selection', { account_ids: accountIds })
}

export interface CheckGroupOption {
  id: number
  name: string
  platform: string
  status: 'active' | 'inactive'
}

export interface ScheduledCheckPlan {
  id: number
  account_id: number
  model_id: string
  cron_expression: string
  enabled: boolean
  max_results: number
  last_run_at: string | null
  next_run_at: string | null
  created_at: string
  updated_at: string
}

export interface ScheduledCheckResult {
  id: number
  plan_id: number
  status: string
  response_text: string
  error_message: string
  latency_ms: number
  started_at: string
  finished_at: string
  created_at: string
}

const base = '/admin/account-checks/accounts'

export async function listCheckAccounts(page = 1, search = '', signal?: AbortSignal) {
  const { data } = await apiClient.get<{ items: CheckAccount[]; total: number; pages: number; page: number }>(base, { params: { page, search }, signal })
  return data
}

export async function listCheckModels(id: number, signal?: AbortSignal) {
  const { data } = await apiClient.get<Array<{ id: string; display_name: string }> | null>(`${base}/${id}/models`, { signal })
  return data || []
}

export async function listCheckGroups(signal?: AbortSignal) {
  const { data } = await apiClient.get<CheckGroupOption[]>('/admin/groups/all', { params: { platform: 'openai' }, signal })
  return data || []
}

export async function updateCheckAccountGroups(id: number, groupIds: number[]) {
  const { data } = await apiClient.put<CheckAccount>(`${base}/${id}/groups`, { group_ids: groupIds })
  return data
}

const scheduledPath = (accountId: number) => `${base}/${accountId}/scheduled-test-plans`

export async function listScheduledChecks(accountId: number) {
  const { data } = await apiClient.get<ScheduledCheckPlan[]>(scheduledPath(accountId))
  return data || []
}

export async function createScheduledCheck(accountId: number, payload: { model_id: string; cron_expression: string; enabled: boolean }) {
  const { data } = await apiClient.post<ScheduledCheckPlan>(scheduledPath(accountId), payload)
  return data
}

export async function updateScheduledCheck(accountId: number, planId: number, payload: { model_id?: string; cron_expression?: string; enabled?: boolean }) {
  const { data } = await apiClient.put<ScheduledCheckPlan>(`${scheduledPath(accountId)}/${planId}`, payload)
  return data
}

export async function deleteScheduledCheck(accountId: number, planId: number) {
  await apiClient.delete(`${scheduledPath(accountId)}/${planId}`)
}

export async function listScheduledCheckResults(accountId: number, planId: number, limit: 1 | 3 = 1) {
  const { data } = await apiClient.get<ScheduledCheckResult[]>(`${scheduledPath(accountId)}/${planId}/results`, { params: { limit } })
  return data || []
}

export async function runAccountCheck(id: number, payload: { prompt: string; model_id: string }, signal: AbortSignal): Promise<AccountCheckResult> {
  const response = await fetch(buildApiUrl(`${base}/${id}/test`), {
    method: 'POST',
    headers: { Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`, 'Content-Type': 'application/json', 'X-Admin-UI-Request': '1' },
    body: JSON.stringify(payload), signal
  })
  if (!response.ok) {
    const error = await response.json().catch(() => null)
    throw new Error(error?.message || `检查请求失败（HTTP ${response.status}）`)
  }
  const reader = response.body?.getReader()
  if (!reader) throw new Error('未收到检查响应')
  const decoder = new TextDecoder()
  let buffer = ''
  const parseLine = (line: string): AccountCheckResult | undefined => {
    if (!line.startsWith('data:')) return
    const result = JSON.parse(line.slice(5).trim()) as AccountCheckResult
    if (typeof result.text !== 'string' || typeof result.html !== 'string') throw new Error('检查响应格式无效')
    return result
  }
  try {
    while (true) {
      const { done, value } = await reader.read()
      buffer += decoder.decode(value, { stream: !done })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      for (const line of lines) {
        const result = parseLine(line)
        if (result) return result
      }
      if (done) {
        const result = parseLine(buffer)
        if (result) return result
        throw new Error('检查响应中断，未收到结果，请重试')
      }
    }
  } finally {
    await reader.cancel().catch(() => {})
    reader.releaseLock()
  }
}

export async function cancelAccountCheck(id: number) {
  const { data } = await apiClient.delete<{ cancelled: boolean; run_id: number }>(`${base}/${id}/test`)
  return data
}
