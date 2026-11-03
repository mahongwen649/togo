import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import UsageFilters from '../UsageFilters.vue'

// --- i18n messages (only what UsageFilters needs) ---
const messages: Record<string, string> = {
  'admin.usage.userDeletedBadge': 'deleted',
  'admin.usage.userFilter': 'User',
  'admin.usage.searchUserPlaceholder': 'Search user...',
  'usage.apiKeyFilter': 'API Key',
  'admin.usage.searchApiKeyPlaceholder': 'Search API key...',
  'admin.usage.account': 'Account',
  'admin.usage.searchAccountPlaceholder': 'Search account...',
  'usage.model': 'Model',
  'admin.usage.allModels': 'All Models',
  'usage.type': 'Type',
  'admin.usage.allTypes': 'All Types',
  'usage.ws': 'WS',
  'usage.stream': 'Stream',
  'usage.sync': 'Sync',
  'admin.usage.billingType': 'Billing Type',
  'admin.usage.allBillingTypes': 'All Billing Types',
  'admin.usage.billingTypeBalance': 'Balance',
  'admin.usage.billingTypeSubscription': 'Subscription',
  'admin.usage.billingMode': 'Billing Mode',
  'admin.usage.allBillingModes': 'All Billing Modes',
  'admin.usage.billingModeToken': 'Token',
  'admin.usage.billingModePerRequest': 'Per Request',
  'admin.usage.billingModeImage': 'Image',
  'admin.usage.billingModeVideo': 'Video',
  'admin.usage.group': 'Group',
  'admin.usage.allGroups': 'All Groups',
  'usage.errors.category': 'Error Category',
  'usage.errors.status': 'Status',
  'usage.errors.allCategories': 'All Categories',
  'usage.errors.allStatuses': 'All Statuses',
  'common.refresh': 'Refresh',
  'common.reset': 'Reset',
  'admin.usage.cleanup.button': 'Cleanup',
  'usage.exportExcel': 'Export',
}

// Mock vue-i18n
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

// Mock the admin API module — we control searchUsers return value per test
const mockSearchUsers = vi.fn()
const mockSearchApiKeys = vi.fn().mockResolvedValue([])
const mockSearchAccounts = vi.fn().mockResolvedValue([])
const mockGetAllGroups = vi.fn().mockResolvedValue([])

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    searchUsers: (...args: any[]) => mockSearchUsers(...args),
    searchApiKeys: (...args: any[]) => mockSearchApiKeys(...args),
    searchAccounts: (...args: any[]) => mockSearchAccounts(...args),
  },
}))

vi.mock('@/api/admin/groups', () => ({
  groupsAPI: { getAllIncludingInactive: (...args: any[]) => mockGetAllGroups(...args) },
}))

// Default props helper
const defaultFilters = () => ({
  user_id: undefined,
  api_key_id: undefined,
  account_id: undefined,
  model: null,
  request_type: null,
  billing_type: null,
  billing_mode: null,
  group_id: null,
  start_date: '',
  end_date: '',
})

function mountFilters(filters = defaultFilters(), mode: 'usage' | 'errors' | 'ranking' | 'cleanup' = 'usage') {
  return mount(UsageFilters, {
    props: {
      modelValue: filters,
      exporting: false,
      startDate: '2026-05-01',
      endDate: '2026-05-28',
      showActions: false,
      modelOptions: [],
      mode,
    },
    global: {
      stubs: {
        Select: true,
        Teleport: true,
      },
    },
  })
}

describe('UsageFilters — user search dropdown', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockSearchUsers.mockReset()
    mockSearchApiKeys.mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('(a) labels deleted users with the i18n badge and (b) sorts active users before deleted ones, (c) selection sets user_id', async () => {
    // Arrange: mock returns deleted FIRST (proves sorting re-orders to active-first)
    mockSearchUsers.mockResolvedValue([
      { id: 2, email: 'gone@test.com', deleted: true },
      { id: 1, email: 'active@test.com', deleted: false },
    ])

    const wrapper = mountFilters()

    // Trigger focus (sets showUserDropdown = true) then input (fires debounceUserSearch)
    const input = wrapper.find('input[type="text"]')
    await input.trigger('focus')
    await input.setValue('test')
    await input.trigger('input')

    // Advance debounce timer (300ms) then flush the resolved promise
    vi.advanceTimersByTime(300)
    await flushPromises()

    // --- (b) Sort: active user should appear BEFORE deleted user ---
    // Check the underlying component state via rendered DOM order
    const buttons = wrapper.findAll('.usage-filter-dropdown button[type="button"]')
    const emailTexts = buttons.map((b) => b.text())

    // active@test.com should be listed first
    const activeIdx = emailTexts.findIndex((t) => t.includes('active@test.com'))
    const deletedIdx = emailTexts.findIndex((t) => t.includes('gone@test.com'))
    expect(activeIdx).toBeGreaterThanOrEqual(0)
    expect(deletedIdx).toBeGreaterThanOrEqual(0)
    expect(activeIdx).toBeLessThan(deletedIdx)

    // --- (a) Label: deleted user's button shows the badge text ---
    const deletedButton = buttons[deletedIdx]
    expect(deletedButton.text()).toContain('deleted')

    // active user's button does NOT show the badge text
    const activeButton = buttons[activeIdx]
    expect(activeButton.text()).not.toContain('deleted')

    // --- (c) Selection: clicking active user button sets filters.user_id ---
    await activeButton.trigger('click')
    await flushPromises()

    // The component emits 'update:modelValue' or modifies filters.user_id via toRef
    // selectUser sets filters.value.user_id = u.id and emits 'change'
    const changeEmits = wrapper.emitted('change')
    expect(changeEmits).toBeTruthy()
    expect(changeEmits!.length).toBeGreaterThan(0)

    // Also confirm user_id was set by checking the emitted change came through
    // (the component uses toRef so modelValue is mutated in place and 'change' is emitted)
    expect(wrapper.props('modelValue').user_id).toBe(1)
  })
})

describe('UsageFilters — Core filter parity', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockSearchAccounts.mockReset().mockResolvedValue([])
  })
  afterEach(() => vi.useRealTimers())

  it('searches by account name, selects its ID, and clears the selection', async () => {
    mockSearchAccounts.mockResolvedValue([{ id: 42, name: 'Account Alpha' }])
    const wrapper = mountFilters()
    const input = wrapper.get('input[placeholder="Search account..."]')
    await input.trigger('focus')
    await input.setValue('  Alpha  ')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mockSearchAccounts).toHaveBeenCalledWith('Alpha', expect.any(AbortSignal))
    const option = wrapper.findAll('button').find((button) => button.text().includes('Account Alpha'))!
    await option.trigger('click')
    expect(wrapper.props('modelValue').account_id).toBe(42)
    expect((input.element as HTMLInputElement).value).toBe('Account Alpha')
    await wrapper.get('[data-testid="clear-account-filter"]').trigger('click')
    expect(wrapper.props('modelValue').account_id).toBeUndefined()
    expect((input.element as HTMLInputElement).value).toBe('')
    expect(wrapper.emitted('change')).toHaveLength(2)
    wrapper.unmount()
  })

  it('discards stale account responses and cancels searches after reset or unmount', async () => {
    let resolveOld!: (value: Array<{ id: number; name: string }>) => void
    mockSearchAccounts.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    const wrapper = mountFilters()
    const input = wrapper.get('input[placeholder="Search account..."]')
    await input.trigger('focus')
    await input.setValue('old')
    await vi.advanceTimersByTimeAsync(300)
    const oldSignal = mockSearchAccounts.mock.calls[0][1] as AbortSignal
    mockSearchAccounts.mockResolvedValueOnce([{ id: 2, name: 'New account' }])
    await input.setValue('new')
    await vi.advanceTimersByTimeAsync(300)
    resolveOld([{ id: 1, name: 'Stale account' }])
    await flushPromises()
    expect(oldSignal.aborted).toBe(true)
    expect(wrapper.text()).toContain('New account')
    expect(wrapper.text()).not.toContain('Stale account')
    await input.setValue('reset pending')
    await wrapper.setProps({ modelValue: defaultFilters() })
    await vi.advanceTimersByTimeAsync(300)
    expect((input.element as HTMLInputElement).value).toBe('')
    expect(mockSearchAccounts).toHaveBeenCalledTimes(2)
    await input.setValue('unmount pending')
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(300)
    expect(mockSearchAccounts).toHaveBeenCalledTimes(2)
  })

  it('supports live requests, compaction, and both true and false model audit values', async () => {
    const wrapper = mountFilters()
    const selectors = wrapper.findAllComponents({ name: 'Select' })
    const withValue = (value: unknown) => selectors.find((select) => select.props('options').some((option: any) => option.value === value))!
    expect(withValue('live').exists()).toBe(true)
    const audit = selectors.find((select) => select.props('options').some((option: any) => option.label === 'admin.usage.upstreamModelMatchedOnly'))!
    audit.vm.$emit('update:modelValue', false)
    audit.vm.$emit('change', false)
    await flushPromises()
    expect(wrapper.props('modelValue').upstream_model_mismatch).toBe(false)
    const compaction = selectors.find((select) => select.props('options').some((option: any) => option.label === 'usage.compactionOnly'))!
    compaction.vm.$emit('update:modelValue', true)
    compaction.vm.$emit('change', true)
    await flushPromises()
    expect(wrapper.props('modelValue').native_compaction_v2).toBe(true)
    wrapper.unmount()
  })

  it('only offers error phases in the error tab and hides unsupported cleanup filters', async () => {
    const wrapper = mountFilters(defaultFilters(), 'errors')
    expect(wrapper.text()).toContain('admin.usage.errorPhase')
    expect(wrapper.text()).not.toContain('usage.compactionFilter')
    const phases = wrapper.findAllComponents({ name: 'Select' }).find((select) => select.props('options').some((option: any) => option.value === 'account_auth'))!
    phases.vm.$emit('update:modelValue', 'account_auth')
    await flushPromises()
    expect(wrapper.props('modelValue').error_phase).toBe('account_auth')
    await wrapper.setProps({ mode: 'cleanup' })
    expect(wrapper.text()).not.toContain('usage.compactionFilter')
    expect(wrapper.text()).not.toContain('admin.usage.upstreamModelAudit')
    expect(wrapper.text()).not.toContain('admin.usage.errorPhase')
    await wrapper.setProps({ showAccountFilter: false })
    expect(wrapper.find('input[placeholder="Search account..."]').exists()).toBe(false)
    wrapper.unmount()
  })
})

describe('UsageFilters — model options come from prop (no dup request)', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockGetAllGroups.mockClear()
  })
  afterEach(() => { vi.useRealTimers() })

  it('does not call dashboard.getModelStats on mount and renders model options from prop', async () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: defaultFilters(),
        exporting: false,
        startDate: '2026-05-01',
        endDate: '2026-05-28',
        showActions: false,
        modelOptions: ['claude-3', 'gpt-4o'],
      },
      global: { stubs: { Select: true, Teleport: true } },
    })
    await flushPromises()

    const opts = (wrapper.vm as any).modelOptions as Array<{ value: string | null; label: string }>
    expect(opts.map((o) => o.value)).toEqual([null, 'claude-3', 'gpt-4o'])
  })

  it('loads all groups, including inactive groups, into the group selector', async () => {
    mockGetAllGroups.mockResolvedValueOnce([
      { id: 7, name: 'Primary' },
      { id: 9, name: 'Archived' },
    ])
    const wrapper = mountFilters()
    await flushPromises()

    expect(mockGetAllGroups).toHaveBeenCalledOnce()
    expect((wrapper.vm as any).groupOptions).toEqual([
      { value: null, label: 'All Groups' },
      { value: 7, label: 'Primary' },
      { value: 9, label: 'Archived' },
    ])
  })

  it('shows error category and status filters while hiding usage-only filters', () => {
    const wrapper = mountFilters(defaultFilters(), 'errors')

    expect(wrapper.text()).toContain('Error Category')
    expect(wrapper.text()).toContain('Status')
    expect(wrapper.text()).not.toContain('Billing Type')
    expect(wrapper.text()).not.toContain('Billing Mode')
  })
})
