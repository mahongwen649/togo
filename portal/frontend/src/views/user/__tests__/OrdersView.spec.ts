import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import OrdersView from '@/views/user/OrdersView.vue'

const { getMyPaymentOrders } = vi.hoisted(() => ({
  getMyPaymentOrders: vi.fn()
}))

vi.mock('@/api/payment', () => ({
  cancelPaymentOrder: vi.fn(),
  getMyPaymentOrders
}))

vi.mock('vue-i18n', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  const messages: Record<string, string> = {
    'payment.portal.ordersTitle': '充值记录',
    'payment.portal.description': '使用支付宝为账户充值人民币余额',
    'payment.portal.refresh': '刷新',
    'payment.portal.recharge': '余额充值',
    'payment.portal.empty': '暂无充值记录',
    'payment.portal.campaignBonusRow': '充值赠送额度',
    'payment.portal.campaignBonusStatuses.APPLIED': '已到账'
  }
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => messages[key] || key, locale: ref('zh-CN') })
  }
})

const global = {
  stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    LoadingSpinner: true,
    Icon: true,
    RouterLink: { template: '<a><slot /></a>' }
  }
}

describe('OrdersView campaign bonus', () => {
  it('renders the applied bonus as a separate credit row', async () => {
    getMyPaymentOrders.mockResolvedValue({
      items: [{
        id: 165,
        out_trade_no: 'sub2_campaign_order',
        amount: 50,
        pay_amount: 50,
        fee_rate: 0,
        payment_type: 'alipay',
        status: 'COMPLETED',
        order_type: 'balance',
        created_at: '2026-08-15T09:20:00Z',
        expires_at: '2026-08-15T09:50:00Z',
        completed_at: '2026-08-15T09:21:00Z',
        campaign_bonus: 5,
        credited_amount: 55,
        campaign_bonus_status: 'APPLIED'
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(OrdersView, { global })
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('¥50.00')
    expect(rows[1].text()).toContain('充值赠送额度')
    expect(rows[1].text()).toContain('+¥5.00')
    expect(rows[1].text()).toContain('已到账')
  })
})
