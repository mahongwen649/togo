import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AccountUserLookupAdminView from '../AccountUserLookupAdminView.vue'

const { listUsers, userOverview, adjustBalance, replace, showSuccess, showError } = vi.hoisted(() => ({ listUsers: vi.fn(), userOverview: vi.fn(), adjustBalance: vi.fn(), replace: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin/users', () => ({ usersAPI: { list: listUsers } }))
vi.mock('@/api/accountBusiness', () => ({ accountAdminAPI: { userOverview, adjustBalance } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace, push: vi.fn() }),
}))

describe('AccountUserLookupAdminView', () => {
  beforeEach(() => {
    listUsers.mockReset()
    userOverview.mockReset()
    replace.mockReset()
    adjustBalance.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    replace.mockResolvedValue(undefined)
    listUsers.mockResolvedValue({ items: [{ id: 8, email: 'member@example.com', username: 'member', balance: 17.25, status: 'active' }] })
    userOverview.mockResolvedValue({
      user: { id: 8, email: 'member@example.com', username: 'member', balance: 17.25, status: 'active' },
      account_balance: 88.5,
      usage: { total_requests: 12, total_cost: 3.5, total_tokens: 456 },
      usage_available: true,
      summary: { total_payments: 50, payment_count: 1, total_redemptions: 25, redemption_count: 1, total_adjustments: 0, adjustment_count: 0, total_deductions: 0, deduction_count: 0, total_spent: 20, total_packages: 1, active_packages: 1 },
      packages: [{ id: 10, name: '30天套餐', mode: 'rental', validity_days: 30, quantity: 1, total_price: 20, status: 'completed', package_status: 'active', auto_renew: true, core_account_ids: [11], starts_at: '2026-09-01T00:00:00Z', expires_at: '2026-10-01T00:00:00Z', created_at: '2026-09-01T00:00:00Z' }],
      payments: [],
      redemptions: [],
    })
    adjustBalance.mockImplementation((_userID: number, payload: { operation: 'credit' | 'debit'; amount: number }) => Promise.resolve({ user_id: 8, operation: payload.operation, amount: payload.amount, balance_before: 88.5, balance_after: payload.operation === 'credit' ? 101 : 76, operator_id: 7, note: '测试调整' }))
  })

  it('searches users and renders both balances, usage and package status', async () => {
    const wrapper = mount(AccountUserLookupAdminView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, Icon: true } } })
    await wrapper.get('input').setValue('member@example.com')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(1, 20, { search: 'member@example.com', role: 'user' })
    expect(userOverview).toHaveBeenCalledWith(8)
    expect(wrapper.text()).toContain('中转余额')
    expect(wrapper.text()).toContain('套餐余额')
    expect(wrapper.text()).toContain('¥17.25')
    expect(wrapper.text()).toContain('¥88.50')
    expect(wrapper.text()).toContain('本月使用')
    expect(wrapper.text()).toContain('30天套餐')
    expect(wrapper.text()).toContain('使用中')

    const rechargeButton = wrapper.findAll('button').find(button => button.text().includes('充值套餐余额'))
    await rechargeButton!.trigger('click')
    await wrapper.get('input[type="number"]').setValue('12.5')
    await wrapper.get('textarea').setValue('售后补偿')
    await wrapper.get('#account-balance-adjustment-form').trigger('submit')
    await flushPromises()

    expect(adjustBalance).toHaveBeenCalledWith(8, { operation: 'credit', amount: 12.5, note: '售后补偿' })
    expect(showSuccess).toHaveBeenCalledWith('已充值 ¥12.50，当前套餐余额 ¥101.00')

    const deductionButton = wrapper.findAll('button').find(button => button.text().includes('扣减套餐余额'))
    await deductionButton!.trigger('click')
    await wrapper.get('input[type="number"]').setValue('12.5')
    await wrapper.get('textarea').setValue('违规退款冲正')
    await wrapper.get('#account-balance-adjustment-form').trigger('submit')
    await flushPromises()

    expect(adjustBalance).toHaveBeenCalledWith(8, { operation: 'debit', amount: 12.5, note: '违规退款冲正' })
    expect(showSuccess).toHaveBeenCalledWith('已扣款 ¥12.50，当前套餐余额 ¥76.00')
  })
})
