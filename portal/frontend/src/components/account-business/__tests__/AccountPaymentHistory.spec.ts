import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountPaymentHistory from '../AccountPaymentHistory.vue'

const api = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn() }))
vi.mock('@/api/payment', () => ({ accountPaymentAPI: api }))

const pending = { id: 7, out_trade_no: 'acct_test', amount: 10, pay_amount: 10.25, status: 'PENDING', created_at: '2026-09-14T00:00:00Z' }
const wrappers: ReturnType<typeof mount>[] = []
let popup: { opener: unknown; location: { href: string }; close: ReturnType<typeof vi.fn> }
async function render() {
  const wrapper = mount(AccountPaymentHistory)
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
beforeEach(() => {
  vi.resetAllMocks()
  api.list.mockResolvedValue({ items: [pending], total: 1, page: 1, page_size: 20, pages: 1 })
  popup = { opener: {}, location: { href: '' }, close: vi.fn() }
  vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window)
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  vi.restoreAllMocks()
})

describe('Account payment history', () => {
  it('resumes the existing pending order and prevents double clicks', async () => {
    let finish!: (value: unknown) => void
    api.get.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = await render()
    const resume = wrapper.findAll('button').find(button => button.text() === '继续支付')!
    await resume.trigger('click')
    await resume.trigger('click')
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.get).toHaveBeenCalledWith(7)
    finish({ ...pending, pay_url: 'https://gateway.example.test/submit.php?out_trade_no=acct_test' })
    await flushPromises()
    expect(popup.opener).toBeNull()
    expect(popup.location.href).toContain('out_trade_no=acct_test')
    expect(popup.close).not.toHaveBeenCalled()
  })

  it.each(['COMPLETED', 'EXPIRED'])('does not reopen payment after the server reports %s', async status => {
    api.get.mockResolvedValue({ ...pending, status })
    const wrapper = await render()
    await wrapper.findAll('button').find(button => button.text() === '继续支付')!.trigger('click')
    await flushPromises()
    expect(popup.close).toHaveBeenCalledTimes(1)
    expect(popup.location.href).toBe('')
    expect(wrapper.findAll('button').some(button => button.text() === '继续支付')).toBe(false)
    expect(wrapper.text()).toContain(status === 'COMPLETED' ? '已到账' : '已过期')
  })

  it('closes the empty window and reports a failed order lookup', async () => {
    api.get.mockRejectedValue(new Error('unavailable'))
    const wrapper = await render()
    await wrapper.findAll('button').find(button => button.text() === '继续支付')!.trigger('click')
    await flushPromises()
    expect(popup.close).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[role="alert"]').text()).toContain('读取支付链接失败')
  })
})
