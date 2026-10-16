export interface CoreWindowStats {
  requests: number
  tokens: number
  cost: number
  standard_cost?: number
  user_cost?: number
}

export interface CoreUsageProgress {
  utilization: number
  resets_at: string | null
  remaining_seconds: number
  window_stats?: CoreWindowStats | null
}

export interface CoreAccountUsage {
  source?: 'passive' | 'active'
  updated_at: string | null
  five_hour: CoreUsageProgress | null
  seven_day: CoreUsageProgress | null
  seven_day_sonnet?: CoreUsageProgress | null
  seven_day_fable?: CoreUsageProgress | null
}

export interface AccountBusinessCoreAccount {
  id: number
  name: string
  platform: string
  type: string
  plan_type?: string
  status: 'active' | 'rate_limited' | 'error'
  schedulable: boolean
  error_message: string
  last_used_at: string | null
  expires_at: number | null
  portal_expires_at: string
  usage: CoreAccountUsage
}

export interface AccountBusinessPackage {
  id: number
  order_id: number
  name: string
  product_type: string
  mode: 'rental' | 'purchase'
  validity_days: number
  quantity: number
  status: 'active' | 'expired'
  starts_at: string
  expires_at: string
  accounts: AccountBusinessCoreAccount[]
}

const stats = (requests: number, tokens: number, cost: number, userCost: number): CoreWindowStats => ({
  requests,
  tokens,
  cost,
  standard_cost: cost,
  user_cost: userCost
})

const progress = (utilization: number, resetsAt: string | null, windowStats: CoreWindowStats | null): CoreUsageProgress => ({
  utilization,
  resets_at: resetsAt,
  remaining_seconds: resetsAt ? Math.max(0, Math.floor((new Date(resetsAt).getTime() - Date.now()) / 1000)) : 0,
  window_stats: windowStats
})

export const accountBusinessAccounts: AccountBusinessCoreAccount[] = [
  {
    id: 390,
    name: 'plus-user-01',
    platform: 'anthropic',
    type: 'oauth',
    plan_type: 'Plus',
    status: 'active',
    schedulable: true,
    error_message: '',
    last_used_at: '2026-09-11T15:24:00+08:00',
    expires_at: null,
    portal_expires_at: '2026-10-08',
    usage: {
      source: 'active',
      updated_at: '2026-09-11 15:26:00',
      five_hour: progress(0, null, stats(168, 18_000_000, 21.1, 1.22)),
      seven_day: progress(21, '2026-09-16T21:00:00+08:00', stats(737, 77_400_000, 98.88, 5.04)),
      seven_day_sonnet: null,
      seven_day_fable: null
    }
  },
  {
    id: 385,
    name: 'plus-user-02',
    platform: 'anthropic',
    type: 'oauth',
    plan_type: 'Plus',
    status: 'active',
    schedulable: true,
    error_message: '',
    last_used_at: '2026-09-11T15:21:00+08:00',
    expires_at: null,
    portal_expires_at: '2026-10-08',
    usage: {
      source: 'passive',
      updated_at: '2026-09-11 15:20:00',
      five_hour: progress(34, '2026-09-11T18:00:00+08:00', stats(92, 9_800_000, 12.4, 0.86)),
      seven_day: progress(48, '2026-09-15T09:00:00+08:00', stats(1_184, 124_000_000, 156.32, 8.11)),
      seven_day_sonnet: null,
      seven_day_fable: null
    }
  },
  {
    id: 373,
    name: 'pro5x-user-01',
    platform: 'anthropic',
    type: 'oauth',
    plan_type: 'Pro5x',
    status: 'rate_limited',
    schedulable: false,
    error_message: '429 · 上游账号当前限流',
    last_used_at: '2026-09-11T14:26:00+08:00',
    expires_at: null,
    portal_expires_at: '2026-10-10',
    usage: {
      source: 'active',
      updated_at: '2026-09-11 15:12:00',
      five_hour: progress(100, '2026-09-11T18:30:00+08:00', stats(1_100, 64_500_000, 97.04, 5.06)),
      seven_day: progress(100, '2026-09-14T03:00:00+08:00', stats(8_420, 812_000_000, 940.18, 51.2)),
      seven_day_sonnet: null,
      seven_day_fable: null
    }
  }
]

export const accountBusinessPackages: AccountBusinessPackage[] = [
  {
    id: 5101,
    order_id: 2401,
    name: 'Plus 账号套餐',
    product_type: 'Plus',
    mode: 'rental',
    validity_days: 30,
    quantity: 2,
    status: 'active',
    starts_at: '2026-09-08 14:22',
    expires_at: '2026-10-08 14:22',
    accounts: accountBusinessAccounts.filter((account) => account.id === 390 || account.id === 385)
  },
  {
    id: 5102,
    order_id: 2399,
    name: 'Pro5x 账号套餐',
    product_type: 'Pro5x',
    mode: 'rental',
    validity_days: 30,
    quantity: 1,
    status: 'active',
    starts_at: '2026-09-10 09:30',
    expires_at: '2026-10-10 09:30',
    accounts: accountBusinessAccounts.filter((account) => account.id === 373)
  }
]

export const accountBusinessOrders = [
  { id: 2401, name: 'Plus 账号套餐', quantity: 2, date: '2026-09-08 14:22', status: '已完成' },
  { id: 2399, name: 'Pro5x 账号套餐', quantity: 1, date: '2026-09-10 09:30', status: '已完成' },
  { id: 2402, name: 'Pro5x 账号套餐', quantity: 1, date: '2026-09-11 10:18', status: '待处理' }
]
