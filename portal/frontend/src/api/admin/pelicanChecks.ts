import { apiClient } from '../client'

export type PelicanCheckState = 'none' | 'running' | 'succeeded' | 'failed'

export interface PelicanTimelinePoint {
	 id: number
	 api_key_id: number
	 api_key_name: string
	 group_name: string
	 attempt: number
	 progress: string
  state: Exclude<PelicanCheckState, 'none'>
  started_at: string
  finished_at: string | null
  latency_ms: number
  error?: string
}

export interface PelicanCheckItem {
	 attempt: number
	 progress: string
  group_id: number
  group_name: string
  api_key_id: number
  api_key_name: string
  state: PelicanCheckState
  started_at: string | null
  finished_at: string | null
  latency_ms: number
  error?: string
  timeline: PelicanTimelinePoint[]
}

export interface PelicanArtwork {
  id: number
  group_id: number
  group_name: string
  api_key_id: number
  api_key_name: string
  started_at: string
  finished_at: string
  latency_ms: number
  html: string
  account_id?: number
  account_name?: string
}

export interface PelicanCheckResponse {
  interval_minutes: number
  generated_at: string
  last_run_at: string | null
  next_run_at: string | null
	 scheduler_healthy: boolean
  prompt: string
  config: PelicanCheckConfig
  summary: { running: number }
  latest_run_id: number
  active_run_id: number
  artworks: PelicanArtwork[]
  latest_artwork: PelicanArtwork | null
}

export interface PelicanCheckConfig {
  reasoning_effort?: string
  model?: string
  prompt?: string
  enabled: boolean
  group_id: number | null
  group_name: string
  api_key_id: number | null
  api_key_name: string
  updated_at: string
}

export interface PelicanGroupOption {
  id: number
  name: string
  platform: string
}

export interface PelicanKeyOption {
  id: number
  name: string
  preview: string
  status: string
}

export interface PelicanCheckOptions {
  models: string[]
  groups: PelicanGroupOption[]
  keys: PelicanKeyOption[]
}

export async function getPelicanChecks(): Promise<PelicanCheckResponse> {
  const { data } = await apiClient.get<PelicanCheckResponse>('/admin/pelican-checks')
  return data
}

export async function getPelicanCheckOptions(groupId?: number | null, keyId?: number | null): Promise<PelicanCheckOptions> {
  const { data } = await apiClient.get<PelicanCheckOptions>('/admin/pelican-checks/options', {
    params: groupId ? { group_id: groupId, api_key_id: keyId || undefined } : undefined
  })
  return data
}

export async function updatePelicanCheckConfig(input: {
  reasoning_effort: string
  model: string
  prompt: string
  enabled: boolean
  group_id: number
  api_key_id: number
}): Promise<PelicanCheckConfig> {
  const { data } = await apiClient.put<PelicanCheckConfig>('/admin/pelican-checks/config', input)
  return data
}

export async function runPelicanCheckNow(): Promise<{ state: 'running'; run_id: number }> {
  const { data } = await apiClient.post<{ state: 'running'; run_id: number }>('/admin/pelican-checks/run')
  return data
}

export async function deletePelicanArtwork(artworkId: number): Promise<void> {
  await apiClient.delete(`/admin/pelican-checks/artworks/${artworkId}`)
}
