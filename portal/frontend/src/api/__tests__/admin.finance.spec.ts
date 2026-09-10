import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { exportMemberCsv, memberBreakdown, summary } from '@/api/admin/finance'

describe('admin finance api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('serializes company ids and platform filters for finance summary', async () => {
    get.mockResolvedValue({ data: [] })

    await summary({ company_ids: [1, 2], platform: 'anthropic' })

    expect(get).toHaveBeenCalledWith('/admin/finance/summary', {
      params: expect.objectContaining({
        company_ids: '1,2',
        platform: 'anthropic',
      }),
    })
  })

  it('loads and exports member breakdown with serialized filters', async () => {
    get.mockResolvedValue({ data: [] })

    await memberBreakdown(42, { company_ids: [1, 2], group_ids: [3, 4], platform: 'openai' })
    await exportMemberCsv(42, { company_ids: [1, 2], group_ids: [3, 4], platform: 'openai' })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/finance/members/42/breakdown', {
      params: expect.objectContaining({
        company_ids: '1,2',
        group_ids: '3,4',
        platform: 'openai',
      }),
    })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/finance/members/42/export', {
      params: expect.objectContaining({
        company_ids: '1,2',
        group_ids: '3,4',
        platform: 'openai',
      }),
      responseType: 'blob',
    })
  })
})
