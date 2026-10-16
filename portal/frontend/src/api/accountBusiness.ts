import { apiClient } from './client'
import { buildApiUrl } from './url'
import type { Account, AccountUsageInfo, ClaudeModel,
  GroupStat,
  ModelStat,
  PaginatedResponse,
  TrendDataPoint,
  UsageLog,
  UsageQueryParams,
  UsageStatsResponse,
  UserErrorRequest,
  UserErrorRequestDetail,
  UserErrorListParams,
} from '@/types'

export type CoreAccountUsage = AccountUsageInfo

export type AccountBusinessCoreAccount = Account & { usage: AccountUsageInfo; usage_error?: string }

export interface AccountBusinessPackage {
  id: number
  order_id: number
  name: string
  product_type: string
  mode: 'rental' | 'purchase' | string
  validity_days: number
  quantity: number
  status: 'active' | 'expired' | string
  auto_renew?: boolean
  starts_at?: string | null
  expires_at?: string | null
  accounts: AccountBusinessCoreAccount[]
  core_group_id?: number | null
}

export interface RefreshAccountUsageResponse {
  account_id: number
  usage: CoreAccountUsage
}

export async function refreshAccountUsage(accountID: number): Promise<CoreAccountUsage> {
  const { data } = await apiClient.get<CoreAccountUsage>(
    `/account-market/accounts/${accountID}/usage`,
    { params: { source: 'active', force: true } }
  )
  return data
}

export async function listAccountPackages(): Promise<AccountBusinessPackage[]> {
  const { data } = await apiClient.get<{ items: AccountBusinessPackage[] }>('/account-market/packages')
  return data.items || []
}

export async function getAccount(accountID: number): Promise<AccountBusinessCoreAccount> {
  const { data } = await apiClient.get<{ account: AccountBusinessCoreAccount; usage: CoreAccountUsage }>(
    `/account-market/accounts/${accountID}`
  )
  return { ...data.account, usage: data.usage || data.account.usage }
}

export async function recoverAccount(accountID: number): Promise<AccountBusinessCoreAccount> {
  const { data } = await apiClient.post<AccountBusinessCoreAccount>(`/account-market/accounts/${accountID}/recover`)
  return data
}

export async function listAccountModels(accountID: number): Promise<ClaudeModel[]> {
  const { data } = await apiClient.get<ClaudeModel[]>(`/account-market/accounts/${accountID}/models`)
  return data
}

export async function testAccountStream(accountID: number, payload: { model_id: string; prompt: string; mode?: string; image_data_url?: string; audio_data_url?: string }, signal?: AbortSignal): Promise<Response> {
  return fetch(buildApiUrl(`/account-market/accounts/${accountID}/test`), {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`,
      'Content-Type': 'application/json',
      'X-User-UI-Request': '1'
    },
    body: JSON.stringify(payload),
    signal
  })
}

export function createScopedUsageAPI(accountUsagePath: string) {
  return {
    query(params: UsageQueryParams & { sort_by?: string; sort_order?: 'asc' | 'desc' }, config: { signal?: AbortSignal } = {}) {
      return apiClient.get<PaginatedResponse<UsageLog>>(accountUsagePath, { ...config, params }).then(({ data }) => data)
    },
    getStats(params: UsageQueryParams) {
      return apiClient.get<UsageStatsResponse>(`${accountUsagePath}/stats`, { params }).then(({ data }) => data)
    },
    getDashboardModels(params?: UsageQueryParams & { model_source?: 'requested' }) {
      return apiClient.get<{ models: ModelStat[]; start_date: string; end_date: string }>(`${accountUsagePath}/dashboard/models`, { params }).then(({ data }) => data)
    },
    getDashboardSnapshotV2(params?: UsageQueryParams & { granularity?: 'day' | 'hour'; include_trend?: boolean; include_model_stats?: boolean; include_group_stats?: boolean }) {
      return apiClient.get<{ generated_at: string; start_date: string; end_date: string; granularity: string; trend?: TrendDataPoint[]; models?: ModelStat[]; groups?: GroupStat[] }>(`${accountUsagePath}/dashboard/snapshot-v2`, { params }).then(({ data }) => data)
    },
    listMyErrorRequests(params: UserErrorListParams) {
      return apiClient.get<PaginatedResponse<UserErrorRequest>>(`${accountUsagePath}/errors`, { params }).then(({ data }) => data)
    },
    getMyErrorDetail(id: number) {
      return apiClient.get<UserErrorRequestDetail>(`${accountUsagePath}/errors/${id}`).then(({ data }) => data)
    },
  }
}

export const accountUsageAPI = createScopedUsageAPI('/account-market/usage')

export async function setAccountOrderAutoRenew(orderID: number, enabled: boolean) {
  const { data } = await apiClient.put<{ id: number; auto_renew: boolean; expires_at: string | null }>(`/account-market/orders/${orderID}/auto-renew`, { enabled })
  return data
}

export interface AccountRedeemHistoryItem {
  id: number
  batch_id: number
  batch_name: string
  code_suffix: string
  amount: number
  remaining_amount: number
  status: 'redeemed' | 'partially_used' | 'used' | 'expired' | string
  expires_at: string | null
  redeemed_at: string | null
  fully_used_at: string | null
  created_at: string
}

export const accountRedeemAPI = {
  redeem(code: string) {
    return apiClient.post<{ amount: number; balance: number; code_suffix: string; status: string }>(
      '/account-market/redeem-codes/redeem',
      { code },
    ).then(({ data }) => data)
  },
  history() {
    return apiClient.get<{ items: AccountRedeemHistoryItem[] }>('/account-market/redeem-codes/history').then(({ data }) => data.items || [])
  },
}

export interface AccountRedeemAdminCode {
  id: number
  batch_id: number
  batch_name: string
  code?: string
  code_suffix: string
  amount: number
  remaining_amount: number
  status: 'active' | 'redeemed' | 'partially_used' | 'used' | 'expired' | 'disabled' | string
  expires_at: string | null
  redeemed_user_id: number | null
  redeemed_at: string | null
  fully_used_at: string | null
  created_at: string
}

export const accountRedeemAdminAPI = {
  list(params: { page?: number; page_size?: number; status?: string; search?: string } = {}) {
    return apiClient.get<{ items: AccountRedeemAdminCode[]; total: number; page: number; page_size: number; pages: number }>(
      '/admin/account-market/redeem-codes',
      { params },
    ).then(({ data }) => data)
  },
  generate(payload: { name?: string; amount: number; count: number; expires_in_days?: number; note?: string }) {
    return apiClient.post<{ batch_id: number; name: string; amount: number; count: number; expires_at: string | null; codes: AccountRedeemAdminCode[] }>(
      '/admin/account-market/redeem-codes/generate',
      payload,
    ).then(({ data }) => data)
  },
  disable(id: number) {
    return apiClient.post<{ id: number; status: string }>(`/admin/account-market/redeem-codes/${id}/disable`).then(({ data }) => data)
  },
}

export interface AccountAdminUserBrief {
  id: number
  email: string
  username: string
  status: string
  balance: number
}

export interface AccountAdminTransaction {
  id: number
  kind: 'payment' | 'redemption' | 'adjustment'
  user_id: number
  user?: AccountAdminUserBrief
  reference: string
  batch_name?: string
  note?: string
  operator_id?: number
  amount: number
  pay_amount?: number
  status: string
  created_at: string
  completed_at?: string | null
}

export interface AccountAdminTransactionResponse {
  items: AccountAdminTransaction[]
  total: number
  page: number
  page_size: number
  pages: number
  summary: {
    total_amount: number
    completed_count: number
  }
}

export interface AccountAdminPackage {
  id: number
  name: string
  mode: string
  validity_days: number
  quantity: number
  total_price: number
  status: string
  package_status: string
  auto_renew: boolean
  core_account_ids: number[]
  starts_at?: string | null
  expires_at?: string | null
  created_at: string
}

export interface AccountAdminUserOverview {
  user: AccountAdminUserBrief
  account_balance: number
  usage: {
    total_requests?: number
    total_cost?: number
    total_tokens?: number
  }
  usage_available: boolean
  summary: {
    total_payments: number
    payment_count: number
    total_redemptions: number
    redemption_count: number
    total_adjustments: number
    adjustment_count: number
    total_deductions: number
    deduction_count: number
    total_spent: number
    total_packages: number
    active_packages: number
  }
  packages: AccountAdminPackage[]
  payments: Array<{
    id: number
    out_trade_no: string
    amount: number
    pay_amount: number
    status: string
    created_at: string
    completed_at?: string | null
  }>
  redemptions: AccountRedeemAdminCode[]
}

export const accountAdminAPI = {
  transactions(params: { type: 'payment' | 'redemption' | 'adjustment'; page?: number; page_size?: number; status?: string; search?: string }) {
    return apiClient.get<AccountAdminTransactionResponse>('/admin/account-market/transactions', { params }).then(({ data }) => data)
  },
  userOverview(userID: number) {
    return apiClient.get<AccountAdminUserOverview>(`/admin/account-market/users/${userID}/overview`).then(({ data }) => data)
  },
  adjustBalance(userID: number, payload: { operation: 'credit' | 'debit'; amount: number; note?: string }) {
    return apiClient.post<{ user_id: number; operation: 'credit' | 'debit'; amount: number; balance_before: number; balance_after: number; operator_id: number; note: string }>(
      `/admin/account-market/users/${userID}/balance`,
      payload,
    ).then(({ data }) => data)
  },
  creditBalance(userID: number, payload: { amount: number; note?: string }) {
    return apiClient.post<{ user_id: number; operation: 'credit'; amount: number; balance_before: number; balance_after: number; operator_id: number; note: string }>(
      `/admin/account-market/users/${userID}/balance`,
      { operation: 'credit', ...payload },
    ).then(({ data }) => data)
  },
}
