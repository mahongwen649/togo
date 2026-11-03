import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { adminUsageAPI } from '@/api/admin/usage'

describe('admin usage API Core parity', () => {
  beforeEach(() => get.mockReset())

  it('uses the sanitized Portal account search with cancellation', async () => {
    const accounts = [{ id: 42, name: 'Claude account' }]
    get.mockResolvedValue({ data: accounts })
    const controller = new AbortController()
    expect(await adminUsageAPI.searchAccounts('Claude', controller.signal)).toEqual(accounts)
    expect(get).toHaveBeenCalledWith('/admin/usage/search-accounts', { params: { q: 'Claude' }, signal: controller.signal })
  })

  it('preserves false audit values and account/compaction filters in queries', async () => {
    get.mockResolvedValue({ data: {} })
    const params = { account_id: 42, upstream_model_mismatch: false, native_compaction_v2: true }
    await adminUsageAPI.list(params)
    await adminUsageAPI.getStats(params)
    expect(get).toHaveBeenNthCalledWith(1, '/admin/usage', expect.objectContaining({ params }))
    expect(get).toHaveBeenNthCalledWith(2, '/admin/usage/stats', { params })
  })
})
