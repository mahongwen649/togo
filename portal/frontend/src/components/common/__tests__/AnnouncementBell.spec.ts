import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'

import AnnouncementBell from '../AnnouncementBell.vue'

const apiMocks = vi.hoisted(() => ({
  list: vi.fn(),
  markRead: vi.fn(),
}))

vi.mock('@/api/announcements', () => ({
  default: apiMocks,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('AnnouncementBell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    apiMocks.list.mockReset()
    apiMocks.markRead.mockReset()
  })

  it('force refreshes announcements when the bell opens', async () => {
    apiMocks.list.mockResolvedValue([
      {
        id: 2,
        title: 'Public announcement',
        content: 'Visible to everyone',
        notify_mode: 'popup',
        created_at: '2026-07-26T00:28:36Z',
        read_at: null,
      },
    ])

    const wrapper = mount(AnnouncementBell, {
      global: {
        stubs: {
          Icon: true,
          Teleport: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(apiMocks.list).toHaveBeenCalledWith(false)
    expect(wrapper.text()).toContain('Public announcement')
  })
})
