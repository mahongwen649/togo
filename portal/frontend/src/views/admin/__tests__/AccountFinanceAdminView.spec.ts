import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AccountFinanceAdminView from '../AccountFinanceAdminView.vue'

const { transactions, push } = vi.hoisted(() => ({ transactions: vi.fn(), push: vi.fn() }))

vi.mock('@/api/accountBusiness', () => ({ accountAdminAPI: { transactions } }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push }),
}))

describe('AccountFinanceAdminView', () => {
  beforeEach(() => {
    transactions.mockReset()
    push.mockReset()
    transactions.mockImplementation(({ type }: { type: string }) => Promise.resolve({
      items: type === 'adjustment'
        ? [{ id: 2, kind: 'adjustment', user_id: 8, user: { id: 8, email: 'member@example.com', username: 'member' }, reference: 'ADJ-2', note: '管理员手动扣减账号套餐余额：退款冲正', amount: -11, status: 'COMPLETED', created_at: '2026-09-14T09:00:00Z' }]
        : [{ id: 1, kind: 'payment', user_id: 8, user: { id: 8, email: 'member@example.com', username: 'member' }, reference: 'acct_1', amount: 50, pay_amount: 51, status: 'COMPLETED', created_at: '2026-09-14T08:00:00Z' }],
      total: 1,
      page: 1,
      page_size: 30,
      pages: 1,
      summary: { total_amount: type === 'adjustment' ? -11 : 50, completed_count: 1 },
    }))
  })

  it('shows searchable online recharge records and switches to redemptions', async () => {
    const wrapper = mount(AccountFinanceAdminView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } } })
    await flushPromises()

    expect(wrapper.text()).toContain('充值与兑换记录')
    expect(wrapper.text()).toContain('member@example.com')
    expect(wrapper.text()).toContain('¥50.00')
    expect(transactions).toHaveBeenCalledWith(expect.objectContaining({ type: 'payment', page_size: 30 }))

    const redemptionTab = wrapper.findAll('button').find(button => button.text().includes('卡密兑换记录'))
    expect(redemptionTab).toBeDefined()
    await redemptionTab!.trigger('click')
    await flushPromises()
    expect(transactions).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'redemption' }))

    const adjustmentTab = wrapper.findAll('button').find(button => button.text().includes('手动调整记录'))
    expect(adjustmentTab).toBeDefined()
    await adjustmentTab!.trigger('click')
    await flushPromises()
    expect(transactions).toHaveBeenLastCalledWith(expect.objectContaining({ type: 'adjustment' }))
    expect(wrapper.text()).toContain('管理员手动扣款')
    expect(wrapper.text()).toContain('-¥11.00')
  })
})
