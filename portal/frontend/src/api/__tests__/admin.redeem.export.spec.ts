import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { get } }))

import { exportCodes } from '@/api/admin/redeem'

describe('admin redeem export API', () => {
  beforeEach(() => get.mockReset())

  it('returns the CSV blob', async () => {
    const csv = new Blob(['id,code\n'], { type: 'text/csv' })
    get.mockResolvedValueOnce({ data: csv })

    await expect(exportCodes({ status: 'used' })).resolves.toBe(csv)
  })
})
