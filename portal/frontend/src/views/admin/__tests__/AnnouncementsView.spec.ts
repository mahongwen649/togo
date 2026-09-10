import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import AnnouncementsView from '../AnnouncementsView.vue'

const { listAnnouncements, getAllGroups, showError } = vi.hoisted(() => ({
  listAnnouncements: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin/announcements', () => ({
  default: {
    list: listAnnouncements,
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn()
  }
}))

vi.mock('@/api/admin/groups', () => ({
  groupsAPI: {
    getAll: getAllGroups
  },
  default: {
    getAll: getAllGroups
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data', 'loading'],
  template: `
    <div>
      <slot v-if="!loading && data.length === 0" name="empty" />
    </div>
  `
}

const EmptyStateStub = {
  props: ['title', 'description', 'actionText'],
  template: `
    <div>
      <h3>{{ title }}</h3>
      <p>{{ description }}</p>
      <button v-if="actionText">{{ actionText }}</button>
    </div>
  `
}

describe('admin AnnouncementsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listAnnouncements.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()
    getAllGroups.mockResolvedValue([])
  })

  function mountView() {
    return mount(AnnouncementsView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          BaseDialog: true,
          ConfirmDialog: true,
          Select: true,
          EmptyState: EmptyStateStub,
          Icon: true,
          AnnouncementTargetingEditor: true,
          AnnouncementReadStatusDialog: true,
          Teleport: true
        }
      }
    })
  }

  it('shows a normal empty state when the announcement list loads successfully with no rows', async () => {
    listAnnouncements.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.announcements.empty')
    expect(wrapper.text()).toContain('admin.announcements.emptyDescription')
    expect(wrapper.text()).not.toContain('admin.announcements.failedToLoad')
  })

  it('keeps the failure copy for a real announcement list load error', async () => {
    listAnnouncements.mockRejectedValue(new Error('network unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.announcements.failedToLoad')
    expect(showError).toHaveBeenCalledWith('admin.announcements.failedToLoad')
  })
})
