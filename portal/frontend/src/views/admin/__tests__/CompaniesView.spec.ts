import { describe, expect, it, vi, beforeEach } from 'vitest'
import { config, flushPromises, mount } from '@vue/test-utils'

import CompaniesView from '../CompaniesView.vue'

const { getAll, getMembers, getManagers, listUsers, createCompany, updateCompany, deactivateCompany, assignMember, createMember, removeMember, addManager, removeManager, allocateBalance, refreshManagementScope, showError, showSuccess } = vi.hoisted(() => ({
  getAll: vi.fn(),
  getMembers: vi.fn(),
  getManagers: vi.fn(),
  listUsers: vi.fn(),
  createCompany: vi.fn(),
  updateCompany: vi.fn(),
  deactivateCompany: vi.fn(),
  assignMember: vi.fn(),
  createMember: vi.fn(),
  removeMember: vi.fn(),
  addManager: vi.fn(),
  removeManager: vi.fn(),
  allocateBalance: vi.fn(),
  refreshManagementScope: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

let authState = {
  isFullAdmin: false,
  isCompanyManager: true,
  managedCompanyIds: [7],
}

vi.mock('@/api/admin/companies', () => ({
  default: {
    getAll,
    getMembers,
    getManagers,
    create: createCompany,
    update: updateCompany,
    deactivate: deactivateCompany,
    assignMember,
    createMember,
    removeMember,
    addManager,
    removeManager,
    allocateBalance,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get isFullAdmin() {
      return authState.isFullAdmin
    },
    get isCompanyManager() {
      return authState.isCompanyManager
    },
    get managedCompanyIds() {
      return authState.managedCompanyIds
    },
    refreshManagementScope,
  }),
}))

const messages: Record<string, string> = {
  'admin.companies.members': '公司成员',
  'admin.companies.user': '成员',
  'admin.companies.balance': '余额',
  'admin.companies.apiKeys': 'API KEY',
  'admin.companies.totalCost': '累计消费',
  'admin.companies.requests': '请求数',
  'admin.companies.lastRequest': '最近请求',
  'admin.companies.noMembers': '暂无公司成员',
  'admin.companies.allocate': '分配',
  'admin.companies.amount': '金额',
  'admin.companies.notes': '备注',
  'admin.companies.allocateSuccess': '分配成功',
  'admin.companies.masterData': '公司资料',
  'admin.companies.createCompany': '新建公司',
  'admin.companies.editCompany': '编辑公司',
  'admin.companies.companyName': '公司名称',
  'admin.companies.status': '状态',
  'admin.companies.active': '启用',
  'admin.companies.disabled': '停用',
  'admin.companies.saveCompany': '保存公司',
  'admin.companies.createSuccess': '公司已创建',
  'admin.companies.updateSuccess': '公司已更新',
  'admin.companies.deactivate': '停用',
  'admin.companies.deactivateSuccess': '公司已停用',
  'admin.companies.addMember': '添加成员',
  'admin.companies.removeMember': '移出公司',
  'admin.companies.managers': '公司经理',
  'admin.companies.noManagers': '暂无公司经理',
  'admin.companies.addManager': '添加经理',
  'admin.companies.removeManager': '移除经理',
  'admin.companies.candidate': '选择用户',
  'admin.companies.managerCandidatePlaceholder': '搜索可添加经理',
  'admin.companies.noCandidates': '暂无可添加用户',
  'admin.companies.selectedCandidate': '已选择 {user} #{id}',
  'admin.companies.addManagerSuccess': '经理已添加',
  'admin.companies.removeManagerSuccess': '经理已移除',
  'admin.companies.candidatesLoadFailed': '加载候选用户失败',
  'admin.companies.failedToLoadManagers': '加载经理失败',
  'admin.companies.managerMutationFailed': '经理更新失败',
  'admin.companies.userId': '用户 ID',
  'admin.companies.addMemberSuccess': '成员已加入公司',
  'admin.companies.removeMemberSuccess': '成员已移出公司',
  'admin.companies.failedToLoadCompanies': '加载公司失败',
  'admin.companies.failedToLoadMembers': '加载成员失败',
  'admin.companies.allocateFailed': '分配失败',
  'admin.companies.saveFailed': '保存公司失败',
  'admin.companies.deactivateFailed': '停用公司失败',
  'admin.companies.memberMutationFailed': '成员归属更新失败',
  'common.refresh': '刷新',
  'common.actions': '操作',
  'common.loading': '加载中',
  'common.cancel': '取消',
  'common.confirm': '确认',
  'common.saving': '保存中',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const IconStub = { template: '<span />' }
const UserCreateModalStub = {
  props: ['show', 'submitOverride', 'successMessage'],
  emits: ['close', 'success'],
  template: '<form v-if="show" data-test="company-member-form" @submit.prevent="submit"><button type="submit" data-test="company-member-create-submit">create</button></form>',
  methods: {
    async submit() {
      await this.submitOverride({
        email: 'new-member@example.com',
        username: 'new-member',
        password: 'secret123',
        notes: 'company member',
        balance: 10,
        concurrency: 2,
        rpm_limit: 60,
      })
      this.$emit('success')
      this.$emit('close')
    },
  },
}

config.global.stubs = {
  ...(config.global.stubs || {}),
  Teleport: true,
}

describe('CompaniesView', () => {
  beforeEach(() => {
    authState = {
      isFullAdmin: false,
      isCompanyManager: true,
      managedCompanyIds: [7],
    }
    getAll.mockReset()
    getMembers.mockReset()
    getManagers.mockReset()
    listUsers.mockReset()
    createCompany.mockReset()
    updateCompany.mockReset()
    deactivateCompany.mockReset()
    assignMember.mockReset()
    createMember.mockReset()
    removeMember.mockReset()
    addManager.mockReset()
    removeManager.mockReset()
    allocateBalance.mockReset()
    refreshManagementScope.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    refreshManagementScope.mockResolvedValue(null)
    getAll.mockResolvedValue([
      { id: 7, name: '鲸奇科技', status: 'active' },
      { id: 8, name: '词元智能', status: 'active' },
    ])
    getMembers.mockResolvedValue({
      items: [
        {
          user_id: 42,
          email: 'member@example.com',
          username: 'member',
          role: 'user',
          status: 'active',
          balance: 12.5,
          api_key_count: 2,
          total_cost: 3.25,
          request_count: 19,
          last_request_at: '2026-07-08T08:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    getManagers.mockResolvedValue([
      {
        company_id: 7,
        user_id: 88,
        email: 'manager@example.com',
        username: 'manager',
        role: 'user',
        status: 'active',
      },
    ])
    allocateBalance.mockResolvedValue({ ok: true })
    createCompany.mockResolvedValue({ id: 9, name: '新公司', status: 'active' })
    updateCompany.mockResolvedValue({ id: 7, name: '鲸奇科技华东', status: 'disabled' })
    deactivateCompany.mockResolvedValue({ id: 7, name: '鲸奇科技', status: 'disabled' })
    assignMember.mockResolvedValue({ company_id: 7, user_id: 43, updated_members: 1 })
    createMember.mockResolvedValue({ id: 43, email: 'new-member@example.com', username: 'new-member' })
    removeMember.mockResolvedValue({ company_id: 7, user_id: 42, updated_members: 1 })
    addManager.mockResolvedValue({ company_id: 7, user_id: 88 })
    removeManager.mockResolvedValue({ company_id: 7, user_id: 88 })
  })

  it('filters visible companies to the company manager scope and loads members', async () => {
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.findAll('[data-test="company-selector"] option')).toHaveLength(0)
    expect(wrapper.find('[data-test="company-selector"]').text()).toContain('鲸奇科技')
    expect(getMembers).toHaveBeenLastCalledWith(7, { page: 1, page_size: 100 })
    expect(wrapper.text()).toContain('member@example.com')
    expect(getManagers).toHaveBeenLastCalledWith(7)
    expect(wrapper.text()).toContain('manager@example.com')
    expect(wrapper.text()).toContain('¥12.50')
    expect(wrapper.text()).toContain('¥3.25')
  })

  it('allows full admins to select any company', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }

    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.findAll('[data-test="company-selector"] option')).toHaveLength(0)
    expect(wrapper.find('[data-test="company-selector"]').text()).toContain('鲸奇科技')
  })

  it('gives company managers company-scoped member operations but hides system controls', async () => {
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-test="company-create-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-edit-7"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-deactivate-7"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-add-member-button"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="company-member-allocate-42"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="company-member-remove-42"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="company-add-manager-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-manager-remove-88"]').exists()).toBe(false)
  })

  it('lets company managers create company members through the user creation modal', async () => {
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
          UserCreateModal: UserCreateModalStub,
        },
      },
    })

    await flushPromises()
    getMembers.mockClear()

    await wrapper.find('[data-test="company-add-member-button"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-test="company-member-form"]').trigger('submit')
    await flushPromises()

    expect(assignMember).not.toHaveBeenCalled()
    expect(createMember).toHaveBeenCalledWith(7, {
      email: 'new-member@example.com',
      username: 'new-member',
      password: 'secret123',
      notes: 'company member',
      balance: 10,
      concurrency: 2,
      rpm_limit: 60,
    })
    expect(showSuccess).toHaveBeenCalledWith('成员已加入公司')
    expect(getMembers).toHaveBeenCalledWith(7, { page: 1, page_size: 100 })
  })

  it('allows full admins to create a company and reload companies', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    getAll.mockClear()

    await wrapper.find('[data-test="company-create-button"]').trigger('click')
    await wrapper.find('[data-test="company-form-name"]').setValue('新公司')
    await wrapper.find('[data-test="company-form"]').trigger('submit')
    await flushPromises()

    expect(createCompany).toHaveBeenCalledWith({ name: '新公司', status: 'active' })
    expect(showSuccess).toHaveBeenCalledWith('公司已创建')
    expect(getAll).toHaveBeenCalled()
  })

  it('allows full admins to update and deactivate a company', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    getAll.mockClear()

    await wrapper.find('[data-test="company-edit-7"]').trigger('click')
    await wrapper.find('[data-test="company-form-name"]').setValue('鲸奇科技华东')
    await wrapper.find('[data-test="company-form-status"]').setValue('disabled')
    await wrapper.find('[data-test="company-form"]').trigger('submit')
    await flushPromises()

    expect(updateCompany).toHaveBeenCalledWith(7, { name: '鲸奇科技华东', status: 'disabled' })
    expect(showSuccess).toHaveBeenCalledWith('公司已更新')
    expect(getAll).toHaveBeenCalled()

    getAll.mockClear()
    await wrapper.find('[data-test="company-deactivate-7"]').trigger('click')
    await flushPromises()

    expect(deactivateCompany).toHaveBeenCalledWith(7)
    expect(showSuccess).toHaveBeenCalledWith('公司已停用')
    expect(getAll).toHaveBeenCalled()
  })

  it('allocates member balance and reloads members', async () => {
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    getMembers.mockClear()

    await wrapper.find('[data-test="company-member-allocate-42"]').trigger('click')
    await wrapper.find('[data-test="company-allocation-amount"]').setValue('25.5')
    await wrapper.find('[data-test="company-allocation-notes"]').setValue('quarter budget')
    await wrapper.find('[data-test="company-allocation-form"]').trigger('submit')
    await flushPromises()

    expect(allocateBalance).toHaveBeenCalledWith(7, 42, {
      amount: 25.5,
      notes: 'quarter budget',
    })
    expect(showSuccess).toHaveBeenCalledWith('分配成功')
    expect(getMembers).toHaveBeenCalledWith(7, { page: 1, page_size: 100 })
    expect(wrapper.find('[data-test="company-allocation-amount"]').exists()).toBe(false)
  })

  it('allows full admins to add and remove company members', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
          UserCreateModal: UserCreateModalStub,
        },
      },
    })

    await flushPromises()
    getMembers.mockClear()

    await wrapper.find('[data-test="company-add-member-button"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-test="company-member-form"]').trigger('submit')
    await flushPromises()

    expect(assignMember).not.toHaveBeenCalled()
    expect(createMember).toHaveBeenCalledWith(7, {
      email: 'new-member@example.com',
      username: 'new-member',
      password: 'secret123',
      notes: 'company member',
      balance: 10,
      concurrency: 2,
      rpm_limit: 60,
    })
    expect(showSuccess).toHaveBeenCalledWith('成员已加入公司')
    expect(getMembers).toHaveBeenCalledWith(7, { page: 1, page_size: 100 })

    getMembers.mockClear()
    await wrapper.find('[data-test="company-member-remove-42"]').trigger('click')
    await flushPromises()

    expect(removeMember).toHaveBeenCalledWith(7, 42)
    expect(showSuccess).toHaveBeenCalledWith('成员已移出公司')
    expect(getMembers).toHaveBeenCalledWith(7, { page: 1, page_size: 100 })
  })

  it('opens user creation instead of member candidates for full admin member creation', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
          UserCreateModal: UserCreateModalStub,
        },
      },
    })

    await flushPromises()
    getMembers.mockClear()

    await wrapper.find('[data-test="company-add-member-button"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-test="company-member-form"]').trigger('submit')
    await flushPromises()

    expect(assignMember).not.toHaveBeenCalled()
    expect(createMember).toHaveBeenCalledWith(7, {
      email: 'new-member@example.com',
      username: 'new-member',
      password: 'secret123',
      notes: 'company member',
      balance: 10,
      concurrency: 2,
      rpm_limit: 60,
    })
    expect(getMembers).toHaveBeenCalledWith(7, { page: 1, page_size: 100 })
  })

  it('allows full admins to add and remove company managers', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    getManagers.mockClear()

    await wrapper.find('[data-test="company-add-manager-button"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="company-manager-form"]').classes()).toContain('min-h-[30rem]')
    await wrapper.find('[data-test="company-manager-candidate-42"]').trigger('mousedown')
    await wrapper.find('[data-test="company-manager-candidate-42"]').trigger('click')
    await wrapper.find('[data-test="company-manager-form"]').trigger('submit')
    await flushPromises()

    expect(addManager).toHaveBeenCalledWith(7, { user_id: 42 })
    expect(showSuccess).toHaveBeenCalledWith('经理已添加')
    expect(getManagers).toHaveBeenCalledWith(7)

    getManagers.mockClear()
    await wrapper.find('[data-test="company-manager-remove-88"]').trigger('click')
    await flushPromises()

    expect(removeManager).toHaveBeenCalledWith(7, 88)
    expect(showSuccess).toHaveBeenCalledWith('经理已移除')
    expect(getManagers).toHaveBeenCalledWith(7)
  })

  it('lets full admins select company manager candidates from current company members only', async () => {
    authState = {
      isFullAdmin: true,
      isCompanyManager: false,
      managedCompanyIds: [],
    }
    getMembers.mockResolvedValue({
      items: [
        {
          user_id: 1,
          email: 'admin-member@example.com',
          username: 'admin-member',
          role: 'admin',
          status: 'active',
          balance: 0,
          api_key_count: 0,
          total_cost: 0,
          request_count: 0,
          last_request_at: null,
        },
        {
          user_id: 42,
          email: 'member@example.com',
          username: 'member',
          role: 'user',
          status: 'active',
          balance: 12.5,
          api_key_count: 2,
          total_cost: 3.25,
          request_count: 19,
          last_request_at: '2026-07-08T08:00:00Z',
        },
        {
          user_id: 44,
          email: 'disabled-member@example.com',
          username: 'disabled-member',
          role: 'user',
          status: 'disabled',
          balance: 0,
          api_key_count: 0,
          total_cost: 0,
          request_count: 0,
          last_request_at: null,
        },
        {
          user_id: 88,
          email: 'manager@example.com',
          username: 'manager',
          role: 'user',
          status: 'active',
          balance: 0,
          api_key_count: 0,
          total_cost: 0,
          request_count: 0,
          last_request_at: null,
        },
      ],
      total: 4,
      page: 1,
      page_size: 100,
      pages: 1,
    })
    const wrapper = mount(CompaniesView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    getManagers.mockClear()

    await wrapper.find('[data-test="company-add-manager-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="company-manager-form-user-id"]').exists()).toBe(false)
    expect(listUsers).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="company-manager-candidate-42"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="company-manager-candidate-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-manager-candidate-44"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-manager-candidate-88"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="company-manager-candidate-89"]').exists()).toBe(false)

    await wrapper.find('[data-test="company-manager-candidate-42"]').trigger('mousedown')
    await wrapper.find('[data-test="company-manager-candidate-42"]').trigger('click')
    await wrapper.find('[data-test="company-manager-form"]').trigger('submit')
    await flushPromises()

    expect(addManager).toHaveBeenCalledWith(7, { user_id: 42 })
    expect(getManagers).toHaveBeenCalledWith(7)
  })
})
