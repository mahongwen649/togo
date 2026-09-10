import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  create: vi.fn(),
}))

vi.mock('@/api/admin/users', () => ({
  usersAPI: {
    create: apiMocks.create,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

import UserCreateModal from '../UserCreateModal.vue'

function mountModal(props = {}) {
  return mount(UserCreateModal, {
    props: { show: true, ...props },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title', 'width'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        Icon: true,
      },
    },
  })
}

describe('UserCreateModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.create.mockResolvedValue({ id: 1 })
  })

  it('邮箱和用户名都是必填', () => {
    const wrapper = mountModal()
    const emailInput = wrapper.find('input[type="email"]')
    const usernameInput = wrapper.find('input[placeholder="admin.users.enterUsername"]')

    expect(emailInput.exists()).toBe(true)
    expect(emailInput.attributes('required')).toBeDefined()
    expect(usernameInput.attributes('required')).toBeDefined()
  })

  it('提交时携带规范化用户名和邮箱', async () => {
    const wrapper = mountModal()

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('member@example.com')
    await inputs[1].setValue('pass1234')
    await inputs[2].setValue(' manual-user ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.create).toHaveBeenCalledTimes(1)
    const payload = apiMocks.create.mock.calls[0][0]
    expect(payload).toMatchObject({
      password: 'pass1234',
      email: 'member@example.com',
      username: 'manual-user',
      role: 'user',
      concurrency: 1,
      rpm_limit: 0,
    })
  })

  it('作为公司成员创建表单时隐藏并省略 Core 角色', async () => {
    const submitOverride = vi.fn().mockResolvedValue({ id: 1 })
    const wrapper = mountModal({ submitOverride, showRole: false })

    expect(wrapper.text()).not.toContain('admin.users.form.roleLabel')

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('member@example.com')
    await inputs[1].setValue('pass1234')
    await inputs[2].setValue(' company-member ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(submitOverride).toHaveBeenCalledTimes(1)
    expect(submitOverride.mock.calls[0][0]).toMatchObject({
      password: 'pass1234',
      email: 'member@example.com',
      username: 'company-member',
      concurrency: 1,
      rpm_limit: 0,
    })
    expect(submitOverride.mock.calls[0][0]).not.toHaveProperty('role')
  })
})
