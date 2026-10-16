import { apiClient } from './client'
import { createScopedUsageAPI } from './accountBusiness'

export const exclusiveUsageAPI = createScopedUsageAPI('/exclusive-channels/usage')

export type ChannelState = 'provisioning' | 'available' | 'active' | 'releasing' | 'cooling' | 'maintenance' | 'unpublished' | 'error'
export interface ChannelAccount {
	page_test?: ChannelPageTest
  platform?: string; type?: string; plan_type?: string
  id: number; name: string; available: boolean
  five_hour_remaining: number | null; weekly_remaining: number | null
  five_hour_resets_at?: string | null; weekly_resets_at?: string | null
  observed_at: string | null; error?: string
}
export interface ChannelLease {
  id: number; channel_id: number; user_id?: number; channel_name?: string
  state: 'provisioning' | 'active' | 'releasing' | 'closed'
  started_at: string; expires_at: string; last_charge_at: string | null; ended_at: string | null
  billing_checked_at: string | null; reason: string; cost: number; requests: number; tokens: number
  error?: string; inflight: number; uncertain: number; mine: boolean; key?: string; base_url?: string
}
export interface ExclusiveChannel {
	image_rate_multiplier?: number
	model_pricing?: ChannelModelPrice[]
	image_price?: number
	pricing_revision?: number
	pricing_synced?: boolean
  rate_multiplier?: number
  id: number; name: string; state: ChannelState; core_group_id?: number; durations: number[]
  cooldown_seconds: number; cooldown_until: string | null; accounts: ChannelAccount[]
  pending_accounts?: ChannelAccount[]
  pending_config?: {
    account_ids: number[]; rate_multiplier: number; model_pricing: ChannelModelPrice[]
    image_price: number; image_rate_multiplier: number
  }
  error?: string; lease?: ChannelLease
}
export interface ChannelModelPrice { model: string; input_price: number | null; output_price: number | null; cache_read_price: number | null }
export interface ChannelForm { name: string; account_ids?: number[]; durations: number[]; cooldown_seconds: number; rate_multiplier?: number; model_pricing?: ChannelModelPrice[]; image_price?: number | null; image_rate_multiplier?: number | null }
export interface ChannelPageTest {
  state: 'none' | 'running' | 'succeeded' | 'failed' | 'uncertain'
  started_at: string | null; generated_at: string | null; has_page: boolean; html?: string; error?: string
}
export interface ChannelEvent { id: number; channel_id: number; channel_name: string; actor_id: number; action: string; created_at: string }
export interface ChannelPage { items: ExclusiveChannel[]; page: number; pages: number; total: number; server_time: string }
export interface MyChannel { lease: ChannelLease | null; channel: ExclusiveChannel | null; balance: number; server_time: string }
export const durationOptions = [30, 60, 120, 240, 480, 720, 1440]
const userBase = '/exclusive-channels'
const adminBase = '/admin/exclusive-channels'
export const exclusiveChannels = {
  async refreshUsage(channel: number, account: number) { return (await apiClient.post<ChannelAccount>(`${userBase}/${channel}/accounts/${account}/refresh-usage`)).data },
  async downgradeTest(channelID: number, accountID: number) { return (await apiClient.get<ChannelPageTest>(`${userBase}/${channelID}/accounts/${accountID}/downgrade-test`)).data },
  async startDowngradeTest(channelID: number, accountID: number) { await apiClient.post(`${userBase}/${channelID}/accounts/${accountID}/downgrade-test`) },
  async list(admin: boolean, page = 1) { return (await apiClient.get<ChannelPage>(admin ? adminBase : userBase, { params: { page } })).data },
  async mine() { return (await apiClient.get<MyChannel>(`${userBase}/mine`)).data },
  async history() { return (await apiClient.get<{ items: ChannelLease[] }>(`${userBase}/history`)).data.items },
  async events() { return (await apiClient.get<{ items: ChannelEvent[] }>(`${adminBase}/events`)).data.items },
  async accounts(page: number, search: string) { return (await apiClient.get<{ items: ChannelAccount[]; page: number; pages: number }>(`${adminBase}/accounts`, { params: { page, search } })).data },
  async claim(id: number, duration: number, rate?: number, revision?: number) { await apiClient.post(`${userBase}/${id}/claim`, { duration, ...(rate === undefined ? {} : { rate_multiplier: rate }), ...(revision === undefined ? {} : { pricing_revision: revision }) }) },
  async release(id: number, admin: boolean) { await apiClient.post(`${admin ? adminBase : userBase}/${id}/release`) },
  async save(form: ChannelForm, id?: number) {
    return id ? (await apiClient.put<{ id: number }>(`${adminBase}/${id}`, form)).data : (await apiClient.post<{ id: number; state: string; error?: string }>(adminBase, form)).data
  },
  async remove(id: number) { await apiClient.delete(`${adminBase}/${id}`) },
  async availability(id: number, state: string) { await apiClient.put(`${adminBase}/${id}/availability`, { state }) },
  async retry(id: number) { await apiClient.post(`${adminBase}/${id}/retry`) },
}
