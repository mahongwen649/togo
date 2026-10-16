import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AccountMarketView from '../AccountMarketView.vue'

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: api }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const product = { id: 1, name: 'Plus', mode: 'rental', validity_days: 30, unit_price: 60, stock_remaining: 5, description: 'Plus 30 days', purchase_notice: '', image_url: '' }
const order = { id: 6, product_name: 'Plus saved order', mode: 'rental', validity_days: 30, quantity: 2, unit_price: 55, total_price: 110, status: 'pending', created_at: '2026-09-12T08:00:00Z', expires_at: null, core_account_ids: [], auto_renew: false }
let detailsFail = false
const options = { global: { stubs: {
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' },
} } }

beforeEach(() => {
  vi.clearAllMocks()
  detailsFail = false
  api.get.mockImplementation(async (path: string) => {
    if (path.endsWith('/products')) return { data: { items: [product] } }
    if (path.endsWith('/wallet')) return { data: { balance: 880 } }
    if (detailsFail) throw new Error('Details unavailable')
    return { data: { items: [{ ...order, id: 99, product_name: 'Other order' }, order] } }
  })
  api.post.mockResolvedValue({ data: { id: 6, balance: 880 } })
})

describe('Account package checkout and card shop', () => {
  it('retires card purchasing routes in favor of direct recharge', () => {
    const router = readFileSync(resolve(__dirname, '../../../router/index.ts'), 'utf8')
    expect(router).toMatch(/path: '\/card-shop',\s+name: 'CardShop',\s+redirect: '\/recharge'/)
    expect(router).toMatch(/path: '\/account-business\/balance\/purchase',\s+name: 'AccountBusinessBalancePurchase',\s+redirect: '\/recharge'/)
    expect(router).not.toContain("component: () => import('@/views/user/AccountBalancePurchaseView.vue')")
  })
  it('uses direct recharge from package checkout without exposing the card shop', async () => {
    const market = mount(AccountMarketView, options)
    await flushPromises()
    expect(market.find('iframe').exists()).toBe(false)
    expect(market.text()).toContain('¥880.00')
    expect(market.get('a[href="/recharge"]').text()).toBe('余额充值')
    expect(market.find('a[href="/card-shop"]').exists()).toBe(false)
    market.unmount()
  })

  it('opens the exact saved order after checkout, using server amounts and timestamps', async () => {
    const wrapper = mount(AccountMarketView, options)
    await flushPromises()
    await wrapper.get('input').setValue(2)
    await wrapper.findAll('button').find(button => button.text() === '立即下单')!.trigger('click')
    await flushPromises()
    expect(api.post).not.toHaveBeenCalled()
    expect(wrapper.get('[role="dialog"]').text()).toContain('确认立即下单')
    await wrapper.findAll('button').find(button => button.text() === '确认并下单')!.trigger('click')
    await flushPromises()
    expect(api.post).toHaveBeenCalledWith('/account-market/orders', { product_id: 1, quantity: 2 })
    const dialog = wrapper.get('[role="dialog"]')
    for (const text of ['订单 #6', order.product_name, '租号 · 30 天', '2 个账号', '¥55.00', '¥110.00', '待处理', '等待管理员分配', '未开启', new Date(order.created_at).toLocaleString('zh-CN')]) {
      expect(dialog.text()).toContain(text)
    }
    expect(dialog.text()).not.toContain('Other order')
    expect(dialog.get('a').attributes('href')).toBe('/account-business/orders')
    wrapper.unmount()
  })

  it('does not show success when checkout fails', async () => {
    api.post.mockRejectedValue(new Error('Insufficient balance'))
    const wrapper = mount(AccountMarketView, options)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '立即下单')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '确认并下单')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain('Insufficient balance')
    wrapper.unmount()
  })

  it('retries detail reads without charging again when the order has succeeded', async () => {
    detailsFail = true
    const wrapper = mount(AccountMarketView, options)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '立即下单')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '确认并下单')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('不要重复下单')
    detailsFail = false
    await wrapper.findAll('button').find(button => button.text() === '重新加载明细')!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain(order.product_name)
    expect(api.post).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('blocks repeated submits while a checkout is in flight', async () => {
    let complete!: (value: unknown) => void
    api.post.mockImplementation(() => new Promise(resolve => { complete = resolve }))
    const wrapper = mount(AccountMarketView, options)
    await flushPromises()
    const button = wrapper.findAll('button').find(item => item.text() === '立即下单')!
    await button.trigger('click')
    await wrapper.findAll('button').find(item => item.text() === '确认并下单')!.trigger('click')
    expect(api.post).toHaveBeenCalledTimes(1)
    expect(button.attributes('disabled')).toBeDefined()
    complete({ data: { id: 6, balance: 880 } })
    await flushPromises()
    wrapper.unmount()
  })
})
