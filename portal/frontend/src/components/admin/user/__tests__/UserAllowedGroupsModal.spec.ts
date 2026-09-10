import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup, AdminUser } from '@/types'
import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const { getAllGroups, getUserById, updateUser, showSuccess } = vi.hoisted(() => ({
  getAllGroups: vi.fn(),
  getUserById: vi.fn(),
  updateUser: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/groups', () => ({
  groupsAPI: {
    getAll: getAllGroups,
  },
}))

vi.mock('@/api/admin/users', () => ({
  usersAPI: {
    getById: getUserById,
    update: updateUser,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const user: AdminUser = {
  id: 42,
  email: 'portal-user-42@internal.invalid',
  username: 'member42',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-07-20T00:00:00Z',
  updated_at: '2026-07-20T00:00:00Z',
  notes: '',
}

const groups: AdminGroup[] = [
  {
    id: 3,
    name: 'exclusive-core-group',
    description: '',
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: true,
    status: 'active',
    subscription_type: 'standard',
    sort_order: 1,
    created_at: '2026-07-20T00:00:00Z',
    updated_at: '2026-07-20T00:00:00Z',
  },
  {
    id: 4,
    name: 'public-core-group',
    description: '',
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    sort_order: 2,
    created_at: '2026-07-20T00:00:00Z',
    updated_at: '2026-07-20T00:00:00Z',
  },
]

describe('UserAllowedGroupsModal', () => {
  beforeEach(() => {
    getAllGroups.mockReset().mockResolvedValue(groups)
    getUserById.mockReset().mockResolvedValue({
      ...user,
      allowed_groups: [3],
      group_rates: { 3: 1.25 },
    })
    updateUser.mockReset().mockResolvedValue(user)
    showSuccess.mockReset()
  })

  it('loads and saves Core group facts through the existing user workflow', async () => {
    const wrapper = mount(UserAllowedGroupsModal, {
      props: { show: false, user },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot/><slot name="footer"/></div>',
          },
          PlatformIcon: true,
          Icon: true,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalledOnce()
    expect(getUserById).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('exclusive-core-group')
    expect(wrapper.text()).toContain('public-core-group')

    const save = wrapper.findAll('button').find((button) => button.text() === 'common.save')
    expect(save).toBeDefined()
    await save!.trigger('click')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(42, {
      allowed_groups: [3],
      group_rates: { 3: 1.25 },
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.users.groupConfigUpdated')
  })
})
