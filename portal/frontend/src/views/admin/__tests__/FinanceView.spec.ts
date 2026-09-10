import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import FinanceView from '../FinanceView.vue'

const { authState, getAllGroups, getAllCompanies, summary, members } = vi.hoisted(() => ({
  authState: {
    isCompanyManager: false,
    isFullAdmin: true,
  },
  getAllGroups: vi.fn(),
  getAllCompanies: vi.fn(),
  summary: vi.fn(),
  members: vi.fn(),
}))

vi.mock('@/api/admin/groups', () => ({
  groupsAPI: {
    getAll: getAllGroups,
  },
  default: {
    getAll: getAllGroups,
  },
}))

vi.mock('@/api/admin/companies', () => ({
  default: {
    getAll: getAllCompanies,
  },
}))

vi.mock('@/api/admin/finance', () => ({
  default: {
    summary,
    members,
    exportCsv: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('file-saver', () => ({
  saveAs: vi.fn(),
}))

const messages: Record<string, string> = {
  'admin.finance.allGroups': '全部分组',
  'admin.finance.allCompanies': '全部公司',
  'admin.finance.allPlatforms': '全部平台',
  'admin.finance.noGroupsSelected': '未选择分组',
  'admin.finance.noCompaniesSelected': '未选择公司',
  'admin.finance.selectedGroups': '已选 {count} 个分组',
  'admin.finance.selectedCompanies': '已选 {count} 个公司',
  'admin.finance.selectAllGroups': '全选',
  'admin.finance.selectAllCompanies': '全选',
  'admin.finance.clearGroups': '清空',
  'admin.finance.clearCompanies': '清空',
  'admin.finance.totalCharge': '消费',
  'admin.finance.requestCount': '请求数',
  'admin.finance.memberCount': '成员数',
  'admin.finance.tokenCount': 'Token 数',
  'admin.finance.export': '导出',
  'admin.finance.companySummary': '公司汇总',
  'admin.finance.company': '公司',
  'admin.finance.platform': '平台',
  'admin.finance.groupSummary': '分组汇总',
  'admin.finance.group': '分组',
  'admin.finance.inputTokens': '输入 Token',
  'admin.finance.outputTokens': '输出 Token',
  'admin.finance.cacheCreationTokens': '缓存写入 Token',
  'admin.finance.cacheReadTokens': '缓存读取 Token',
  'admin.finance.imageOutputTokens': '图片输出 Token',
  'admin.finance.actualChargedAmount': '消费',
  'admin.finance.memberDetails': '成员明细',
  'admin.finance.user': '用户',
  'common.refresh': '刷新',
  'common.loading': '加载中',
  'common.noData': '暂无数据',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let message = messages[key] ?? key
        if (params) {
          for (const [name, value] of Object.entries(params)) {
            message = message.replace(`{${name}}`, String(value))
          }
        }
        return message
      },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const DateRangePickerStub = {
  template: '<div data-test="date-range-picker" />',
}
const PaginationStub = { template: '<div data-test="pagination" />' }

describe('FinanceView display', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-24T10:00:00+08:00'))
    getAllGroups.mockReset()
    getAllCompanies.mockReset()
    summary.mockReset()
    members.mockReset()
    authState.isCompanyManager = false
    authState.isFullAdmin = true

    getAllGroups.mockResolvedValue([{ id: 1, name: '默认分组' }])
    getAllCompanies.mockResolvedValue([{ id: 10, name: '默认公司', status: 'active' }])
    summary.mockResolvedValue([
      {
        company_id: 10,
        company_name: '默认公司',
        platform: '',
        group_id: 0,
        group_name: '',
        member_count: 7,
        request_count: 7,
        input_tokens: 100000,
        output_tokens: 50000,
        cache_creation_tokens: 10000,
        cache_read_tokens: 578,
        image_output_tokens: 0,
        actual_charged_amount: 0.037799,
      },
    ])
    members.mockResolvedValue({
      items: [],
      total: 0,
      pages: 1,
      page: 1,
      page_size: 20,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses dashboard stat card styling and displays consumption in CNY', async () => {
    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()

    const cards = wrapper.findAll('[data-test="finance-stat-card"]')
    expect(cards).toHaveLength(4)
    expect(cards[0].classes()).toEqual(expect.arrayContaining(['card', 'p-4']))
    expect(cards[0].find('[data-test="finance-stat-content"]').classes()).toEqual(expect.arrayContaining(['flex', 'items-center', 'gap-3']))
    expect(cards[0].find('[data-test="finance-stat-icon"]').classes()).toEqual(expect.arrayContaining(['rounded-lg', 'bg-green-100', 'p-2']))
    expect(cards[0].text()).toContain('消费')
    expect(cards[0].text()).toContain('¥0.037799')
    expect(cards[0].text()).not.toContain('US$')
    expect(cards[0].text()).not.toContain('实际扣费')
    expect(cards[0].find('[data-test="finance-stat-value"]').classes()).toEqual(expect.arrayContaining(['text-xl', 'font-bold', 'text-green-600']))
  })

  it('defaults the finance range to this month', async () => {
    mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()

    expect(summary).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-06-01',
      end_date: '2026-06-24',
      company_id: 10,
      company_ids: [10],
    }))
    expect(members).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-06-01',
      end_date: '2026-06-24',
      company_id: 10,
      company_ids: [10],
    }))
  })

  it('supports selecting multiple groups and reloads with group_ids', async () => {
    getAllGroups.mockResolvedValue([
      { id: 1, name: '默认分组' },
      { id: 2, name: '体验组' },
      { id: 3, name: '备用组' },
    ])

    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()
    summary.mockClear()
    members.mockClear()

    await wrapper.find('[data-test="finance-group-filter-trigger"]').trigger('click')
    await wrapper.find('[data-test="finance-group-option-3"]').setValue(false)
    await flushPromises()

    expect(summary).toHaveBeenLastCalledWith(expect.objectContaining({ group_ids: [1, 2] }))
    expect(members).toHaveBeenLastCalledWith(expect.objectContaining({ group_ids: [1, 2] }))
    expect(wrapper.find('[data-test="finance-group-filter-trigger"]').text()).toContain('已选 2 个分组')
  })

  it('checks every group when all groups is selected', async () => {
    getAllGroups.mockResolvedValue([
      { id: 1, name: '鲸奇' },
      { id: 2, name: '词元' },
    ])

    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()
    await wrapper.find('[data-test="finance-group-filter-trigger"]').trigger('click')

    expect((wrapper.find('[data-test="finance-group-option-1"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.find('[data-test="finance-group-option-2"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('sends the legacy group_id when a single group is selected', async () => {
    getAllGroups.mockResolvedValue([
      { id: 1, name: '鲸奇' },
      { id: 2, name: '词元' },
    ])

    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()
    summary.mockClear()
    members.mockClear()

    await wrapper.find('[data-test="finance-group-filter-trigger"]').trigger('click')
    await wrapper.find('[data-test="finance-group-option-1"]').setValue(false)
    await flushPromises()

    expect(summary).toHaveBeenLastCalledWith(expect.objectContaining({ group_id: 2, group_ids: [2] }))
    expect(members).toHaveBeenLastCalledWith(expect.objectContaining({ group_id: 2, group_ids: [2] }))
  })

  it('treats no selected groups as all groups', async () => {
    getAllGroups.mockResolvedValue([
      { id: 1, name: '鲸奇' },
      { id: 2, name: '词元' },
    ])

    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()
    summary.mockClear()
    members.mockClear()

    await wrapper.find('[data-test="finance-group-filter-trigger"]').trigger('click')
    await wrapper.find('button:nth-of-type(2)').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="finance-group-filter-trigger"]').text()).toContain('全部分组')
    expect((wrapper.find('[data-test="finance-all-groups-option"]').element as HTMLInputElement).checked).toBe(true)
    expect(summary).toHaveBeenLastCalledWith(expect.not.objectContaining({
      group_id: expect.any(Number),
      group_ids: expect.any(Array),
    }))
    expect(members).toHaveBeenLastCalledWith(expect.not.objectContaining({
      group_id: expect.any(Number),
      group_ids: expect.any(Array),
    }))
  })

  it('does not call the full-admin groups endpoint for company managers', async () => {
    authState.isCompanyManager = true
    authState.isFullAdmin = false

    const wrapper = mount(FinanceView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Pagination: PaginationStub,
        },
      },
    })

    await flushPromises()

    expect(getAllGroups).not.toHaveBeenCalled()
    expect(summary).toHaveBeenLastCalledWith(expect.not.objectContaining({
      group_id: expect.any(Number),
      group_ids: expect.any(Array),
    }))
    expect(wrapper.find('[data-test="finance-group-filter-trigger"]').text()).toContain('全部分组')
  })
})
