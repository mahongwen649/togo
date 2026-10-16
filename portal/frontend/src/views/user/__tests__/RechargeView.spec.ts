import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import RechargeView from '../RechargeView.vue'
import { verifyPaymentOrder } from '@/api/payment'

const features = vi.hoisted(() => ({ ACCOUNT_PACKAGES_ENABLED: true }))
vi.mock('@/config/portalFeatures', () => features)

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
const auth = vi.hoisted(() => ({ user: { id: 1, balance: 16.66 }, refreshUser: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const checkout = { methods: { alipay: { single_min: 1, single_max: 1000, fee_rate: 0 } }, balance_recharge_multiplier: 1, balance_disabled: false }
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(RechargeView, { global: { stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } } } })
  wrappers.push(wrapper)
  return wrapper
}
const account = (wrapper: ReturnType<typeof render>) => wrapper.get('[aria-labelledby="account-recharge-title"]')
let popup: { opener: unknown; location: { href: string }; close: ReturnType<typeof vi.fn> }
beforeEach(() => {
  features.ACCOUNT_PACKAGES_ENABLED = true
  setActivePinia(createPinia())
  vi.resetAllMocks()
  api.get.mockImplementation(async (path: string) => ({ data: path.endsWith('/checkout-info') ? checkout : { balance: 99 } }))
  api.post.mockResolvedValue({ data: { order_id: 7, pay_url: 'https://example.com/pay' } })
  popup = { opener: {}, location: { href: '' }, close: vi.fn() }
  vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window)
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('Direct Alipay recharge wallets', () => {
  it('shows only relay recharge and does not load package APIs when packages are hidden', async () => {
    features.ACCOUNT_PACKAGES_ENABLED = false
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[aria-labelledby="account-recharge-title"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('账号套餐')
    expect(wrapper.find('.recharge-sections--dual').exists()).toBe(false)
    expect(wrapper.get('#relay-recharge-amount').exists()).toBe(true)
    expect(api.get.mock.calls.some(([path]) => path.startsWith('/account-market'))).toBe(false)
  })

  it('shows two independent Alipay forms without card redemption', async () => {
    const wrapper = render()
    await flushPromises()
    const relay = wrapper.get('[aria-labelledby="relay-recharge-title"]')
    expect(relay.text()).toContain('¥16.66')
    expect(account(wrapper).text()).toContain('¥99.00')
    expect(relay.get('a').attributes('href')).toBe('/orders')
    expect(account(wrapper).get('a').attributes('href')).toBe('/account-business/balance/history')
    expect(account(wrapper).get('img[alt="Alipay"]').exists()).toBe(true)
    expect(account(wrapper).get('.account-new-badge').text()).toBe('NEW')
    expect(account(wrapper).find('input[type="text"]').exists()).toBe(false)
    expect(account(wrapper).text()).not.toContain('卡密')
    expect(account(wrapper).text()).not.toContain('赠')
  })

  it('creates an account payment without posting a Core balance order', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#account-recharge-amount').setValue(50)
    await account(wrapper).get('form').trigger('submit')
    await flushPromises()
    expect(api.post).toHaveBeenCalledTimes(1)
    expect(api.post).toHaveBeenCalledWith('/account-market/payments/orders', expect.objectContaining({ amount: 50, payment_type: 'alipay' }))
    expect(popup.location.href).toBe('https://example.com/pay')
    expect(popup.opener).toBeNull()
    expect(account(wrapper).text()).toContain('¥99.00')
    expect(auth.user.balance).toBe(16.66)
    expect(account(wrapper).get('button[type="submit"]').attributes('disabled')).toBeDefined()
  })

  it('refreshes only the account wallet after confirmed server completion', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await flushPromises()
    await account(wrapper).get('form').trigger('submit')
    await flushPromises()
    api.get.mockImplementation(async (path: string) => ({ data: path.includes('/payments/orders/') ? { id: 7, status: 'COMPLETED', amount: 10 } : { balance: 109 } }))
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(account(wrapper).get('[role="status"]').text()).toContain('已到账账号套餐余额')
    expect(account(wrapper).text()).toContain('¥109.00')
    expect(auth.refreshUser).not.toHaveBeenCalled()
    expect(auth.user.balance).toBe(16.66)
    const calls = api.get.mock.calls.length
    await vi.advanceTimersByTimeAsync(10000)
    expect(api.get).toHaveBeenCalledTimes(calls)
  })

  it('does not credit on order failure or query-string status', async () => {
    api.post.mockRejectedValue(new Error('支付通道不可用'))
    const wrapper = render()
    await flushPromises()
    await account(wrapper).get('form').trigger('submit')
    await flushPromises()
    expect(account(wrapper).get('[role="alert"]').text()).toBe('支付通道不可用')
    expect(account(wrapper).text()).toContain('¥99.00')
    expect(popup.close).toHaveBeenCalled()
  })

  it('prevents repeated orders while submitting and waiting for payment', async () => {
    const wrapper = render()
    await flushPromises()
    await account(wrapper).get('form').trigger('submit')
    await account(wrapper).get('form').trigger('submit')
    await flushPromises()
    await account(wrapper).get('form').trigger('submit')
    expect(api.post).toHaveBeenCalledTimes(1)
  })

  it('keeps account payments available when relay checkout fails', async () => {
    api.get.mockImplementation(async (path: string) => {
      if (path === '/payment/checkout-info') throw new Error('Checkout unavailable')
      return { data: path.endsWith('checkout-info') ? checkout : { balance: 99 } }
    })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Checkout unavailable')
    expect(account(wrapper).get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('does not fall back to relay payment when account checkout is unavailable', async () => {
    api.get.mockImplementation(async (path: string) => {
      if (path === '/account-market/payments/checkout-info') throw new Error('Account unavailable')
      return { data: path.endsWith('checkout-info') ? checkout : { balance: 99 } }
    })
    const wrapper = render()
    await flushPromises()
    expect(account(wrapper).get('[role="alert"]').text()).toContain('账号套餐支付信息加载失败')
    expect(account(wrapper).text()).not.toContain('Account unavailable')
    expect(account(wrapper).find('form').exists()).toBe(false)
    expect(wrapper.find('#relay-recharge-amount').exists()).toBe(true)
    expect(api.post).not.toHaveBeenCalled()
  })

  it('rejects sub-cent amounts and calculates provider percentage fees', async () => {
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith('checkout-info') ? { ...checkout, methods: { alipay: { single_min: 1, single_max: 1000, fee_rate: 2.5 } } } : { balance: 99 } }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#account-recharge-amount').setValue(100)
    expect(account(wrapper).text()).toContain('¥2.50')
    expect(account(wrapper).text()).toContain('¥102.50')
    await wrapper.get('#account-recharge-amount').setValue(1.001)
    expect(account(wrapper).get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await account(wrapper).get('form').trigger('submit')
    expect(api.post).not.toHaveBeenCalled()
  })

  it.each([404, 503])('shows an unavailable state for HTTP %s without exposing transport errors', async status => {
    api.get.mockImplementation(async (path: string) => {
      if (path === '/account-market/payments/checkout-info') throw { status, message: `Request failed with status code ${status}` }
      return { data: path.endsWith('checkout-info') ? checkout : { balance: 99 } }
    })
    const wrapper = render()
    await flushPromises()
    expect(account(wrapper).get('[role="status"]').text()).toContain('账号套餐在线充值暂未开放')
    expect(account(wrapper).find('form').exists()).toBe(false)
    expect(account(wrapper).text()).not.toContain('Request failed')
    expect(wrapper.find('#relay-recharge-amount').exists()).toBe(true)
    expect(api.post).not.toHaveBeenCalled()
    api.get.mockImplementation(async (path: string) => ({ data: path.endsWith('checkout-info') ? checkout : { balance: 99 } }))
    await account(wrapper).get('[role="status"] button').trigger('click')
    await flushPromises()
    expect(account(wrapper).find('form').exists()).toBe(true)
    expect(account(wrapper).find('[role="status"]').exists()).toBe(false)
  })

  it.each([{ ...checkout, balance_disabled: true }, { ...checkout, methods: {} }])('does not offer checkout when Alipay is disabled', async config => {
    api.get.mockImplementation(async (path: string) => ({ data: path === '/account-market/payments/checkout-info' ? config : path.endsWith('checkout-info') ? checkout : { balance: 99 } }))
    const wrapper = render()
    await flushPromises()
    expect(account(wrapper).get('[role="status"]').text()).toContain('暂未开放')
    expect(account(wrapper).find('form').exists()).toBe(false)
    expect(api.post).not.toHaveBeenCalled()
  })

  it('cleans up account payment polling after navigation', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await flushPromises()
    await account(wrapper).get('form').trigger('submit')
    await flushPromises()
    wrapper.unmount()
    const calls = api.get.mock.calls.length
    await vi.advanceTimersByTimeAsync(10000)
    expect(api.get).toHaveBeenCalledTimes(calls)
  })

  it('preserves the relay Alipay order type', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('#relay-recharge-amount').setValue(20)
    await wrapper.findAll('button').find(button => button.text() === 'payment.portal.payNow')!.trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/payment/orders', expect.objectContaining({ amount: 20, order_type: 'balance', payment_type: 'alipay' }))
    expect(account(wrapper).text()).toContain('¥99.00')
  })

  it('verifies returned account orders using the account endpoint only', async () => {
    api.post.mockResolvedValue({ data: { status: 'COMPLETED', order_type: 'account_balance' } })
    await verifyPaymentOrder('acct_test')
    expect(api.post).toHaveBeenCalledWith('/account-market/payments/verify', { out_trade_no: 'acct_test' })
    await verifyPaymentOrder('sub2_test')
    expect(api.post).toHaveBeenCalledWith('/payment/orders/verify', { out_trade_no: 'sub2_test' })
  })
})
