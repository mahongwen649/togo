import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import OrdersView from '@/views/user/OrdersView.vue'

const { copyToClipboard, getMyPaymentOrders } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getMyPaymentOrders: vi.fn()
}))

vi.mock('@/api/payment', () => ({
  cancelPaymentOrder: vi.fn(),
  getMyPaymentOrders
}))

vi.mock('@/composables/useClipboard', async () => {
  const { ref } = await import('vue')
  return {
    useClipboard: () => ({ copied: ref(false), copyToClipboard })
  }
})

vi.mock('vue-i18n', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  const messages: Record<string, string> = {
    'payment.portal.ordersTitle': '充值记录',
    'payment.portal.description': '使用支付宝为账户充值人民币余额',
    'payment.portal.invoiceApply': '申请发票',
    'payment.portal.invoiceDialogTitle': '申请发票',
    'payment.portal.invoiceRequirementTitle': '开票说明',
    'payment.portal.invoiceRequirement': '充值金额满 ¥100 后可申请开具发票。',
    'payment.portal.invoiceWechatLabel': '微信客服',
    'payment.portal.invoiceContactHint': '添加客服微信后，请提供充值账号、订单号和开票信息，客服会协助处理。',
    'payment.portal.refundApply': '申请退款',
    'payment.portal.refundDialogTitle': '申请退款',
    'payment.portal.refundRequirementTitle': '退款说明',
    'payment.portal.refundRequirement': '如需申请退款，请联系微信客服提交申请。',
    'payment.portal.refundContactHint': '添加客服微信后，请提供充值账号、订单号和退款原因，客服会协助处理。',
    'payment.portal.copyWechat': '复制微信号',
    'payment.portal.wechatCopied': '微信号已复制',
    'payment.portal.invoiceClose': '我知道了',
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
    BaseDialog: {
      props: ['show', 'title'],
      template: '<div v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>'
    },
    LoadingSpinner: true,
    Icon: true,
    RouterLink: { template: '<a><slot /></a>' }
  }
}

describe('OrdersView invoice contact', () => {
  it('shows the invoice requirement and copies the support WeChat ID', async () => {
    getMyPaymentOrders.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    copyToClipboard.mockResolvedValue(true)

    const wrapper = mount(OrdersView, { global })
    await flushPromises()

    const applyButton = wrapper.findAll('button').find(button => button.text().includes('申请发票'))
    expect(applyButton).toBeTruthy()
    await applyButton!.trigger('click')

    expect(wrapper.get('[role="dialog"]').text()).toContain('充值金额满 ¥100')
    expect(wrapper.get('[role="dialog"]').text()).toContain('mahw649')

    const copyButton = wrapper.findAll('button').find(button => button.text().includes('复制微信号'))
    expect(copyButton).toBeTruthy()
    await copyButton!.trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('mahw649', '微信号已复制')

    const closeButton = wrapper.findAll('button').find(button => button.text().includes('我知道了'))
    await closeButton!.trigger('click')

    const refundButton = wrapper.findAll('button').find(button => button.text().includes('申请退款'))
    expect(refundButton).toBeTruthy()
    await refundButton!.trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('请联系微信客服提交申请')
    expect(wrapper.get('[role="dialog"]').text()).toContain('退款原因')
    expect(wrapper.get('[role="dialog"]').text()).toContain('mahw649')
  })
})

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
