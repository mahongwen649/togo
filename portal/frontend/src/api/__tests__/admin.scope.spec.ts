import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { getManagementScope } from '@/api/admin/scope'

describe('admin management scope api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads Portal management scope from the backend route', async () => {
    get.mockResolvedValue({
      data: {
        user_id: 11,
        is_admin: false,
        is_group_manager: false,
        is_company_manager: true,
        managed_group_ids: [],
        managed_company_ids: [3],
        can_access_admin_area: true,
      },
    })

    const result = await getManagementScope()

    expect(get).toHaveBeenCalledWith('/admin/management/scope')
    expect(result.is_company_manager).toBe(true)
  })
})
