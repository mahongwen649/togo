import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ExclusiveChannelsView from '../ExclusiveChannelsView.vue'

const state = vi.hoisted(() => ({ path: '/exclusive-channels' }))
const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() }))
const appStore = vi.hoisted(() => ({ fetchPublicSettings: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('../UsageView.vue', () => ({ default: { name: 'UsageView', props: { embedded: Boolean, scope: String }, template: '<div data-testid="shared-usage" />' } }))
vi.mock('vue-router', () => ({ useRoute: () => state }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/client', () => ({ apiClient: client }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const account = { id: 1, name: 'Account 1', platform: 'openai', type: 'oauth', plan_type: 'Plus', available: true, five_hour_remaining: 72, weekly_remaining: null, observed_at: null }
const channel = { id: 1, name: 'Channel A', state: 'available', durations: [30, 1440], cooldown_seconds: 60, accounts: [account] }
const options = { global: { stubs: { BaseDialog: { props: ['show'], template: '<section v-if="show" role="dialog"><slot /><slot name="footer" /></section>' } } } }
const mounted: ReturnType<typeof mount>[] = []
const render = () => { const wrapper = mount(ExclusiveChannelsView, options); mounted.push(wrapper); return wrapper }

beforeEach(() => {
  vi.clearAllMocks()
  state.path = '/exclusive-channels'
  client.get.mockImplementation(async (path: string) => {
    if (path.endsWith('/mine')) return { data: { lease: null, channel: null, balance: 20, server_time: new Date().toISOString() } }
    if (path.endsWith('/accounts')) return { data: { items: [account], page: 1, pages: 1 } }
    if (path.endsWith('/downgrade-test')) return { data: { state: 'succeeded', started_at: new Date().toISOString(), generated_at: new Date().toISOString(), has_page: true, html: '<html><body>ok</body></html>' } }
    return { data: { items: path.endsWith('/events') ? [] : [channel], page: 1, pages: 1, total: 1 } }
  })
  client.post.mockResolvedValue({ data: { id: 1 } })
  client.delete.mockResolvedValue({ data: { id: 1 } })
  appStore.fetchPublicSettings.mockResolvedValue({ site_name: 'TogoAPI', hide_ccs_import_button: false })
})
afterEach(() => { mounted.splice(0).forEach(w => w.unmount()); vi.restoreAllMocks() })

describe('Exclusive channels', () => {
  it('reuses package usage records with the exclusive request scope', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.findAll('.ex-tabs button')[2].trigger('click')
    await flushPromises()
    const usage = wrapper.findComponent({ name: 'UsageView' })
    expect(usage.props()).toMatchObject({ embedded: true, scope: 'exclusive' })
    expect(client.get.mock.calls.some(([path]) => path.endsWith('/history'))).toBe(false)
    expect(wrapper.text()).not.toContain('领取时间')
  })
  it('copies credentials with a toast and imports the exclusive endpoint to CCS', async () => {
    const key = 'pxc_1234567890'
    const baseUrl = 'https://togoapi.com/exclusive/v1'
    const lease = { id: 9, channel_id: 1, state: 'active', started_at: new Date().toISOString(), expires_at: new Date(Date.now() + 3600000).toISOString(), last_charge_at: new Date().toISOString(), ended_at: null, billing_checked_at: new Date().toISOString(), reason: '', cost: 0, requests: 0, tokens: 0, inflight: 0, uncertain: 0, mine: true, key, base_url: baseUrl }
    client.get.mockImplementation(async (path: string) => {
      if (path.endsWith('/mine')) return { data: { lease, channel: { ...channel, state: 'active' }, balance: 20, server_time: new Date().toISOString() } }
      return { data: { items: [{ ...channel, state: 'active', lease }], page: 1, pages: 1, total: 1 } }
    })
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    vi.spyOn(window, 'open').mockReturnValue(null)
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)

    const w = render(); await flushPromises()
    await w.findAll('button').find(button => button.text() === '我的通道')!.trigger('click')
    await flushPromises()
    await w.get('[aria-label="复制 Base URL"]').trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith(baseUrl)
    expect(appStore.showSuccess).toHaveBeenCalledWith('复制成功')

    await w.get('[aria-label="导入到 CCS"]').trigger('click')
    const url = new URL(String(vi.mocked(window.open).mock.calls[0][0]))
    expect(url.protocol).toBe('ccswitch:')
    expect(url.searchParams.get('app')).toBe('codex')
    expect(url.searchParams.get('endpoint')).toBe(baseUrl)
    expect(url.searchParams.get('apiKey')).toBe(key)
    expect(url.searchParams.get('name')).toBe('TogoAPI · Channel A')
    expect(url.searchParams.has('usageEnabled')).toBe(false)
  })

  it('renders downgrade tests per account instead of at channel level', async () => {
    const w = render(); await flushPromises()
    expect(w.text()).toContain('Account 1 · 降智测试')
    expect(w.text()).not.toContain('通道测试')
    expect(w.findAll('.downgrade-test')).toHaveLength(1)
  })

  it('lets administrators start an account downgrade test without owning the channel', async () => {
    state.path = '/admin/exclusive-channels'
    const w = render(); await flushPromises()
    await w.get('[aria-label="测试 Channel A 账号"]').trigger('click')
    expect(w.get('[role="dialog"]').text()).toContain('Account 1 · 降智测试')
    await w.findAll('button').find(button => button.text() === '开始降智测试')!.trigger('click')
    await flushPromises()
    expect(client.post).toHaveBeenCalledWith('/exclusive-channels/1/accounts/1/downgrade-test')
  })

  it('sends image defaults and per-model prices, and restores them on edit', async () => {
    state.path = '/admin/exclusive-channels'
    client.put.mockResolvedValue({ data: { id: 1 } })
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, image_price: 0.08, model_pricing: [{ model: 'gpt-6-astra', input_price: 0.0000025, output_price: null, cache_read_price: 0 }] }], page: 1, pages: 1, total: 1 } })
    const w = render(); await flushPromises()
    await w.get('[aria-label="编辑 Channel A"]').trigger('click')
    expect((w.get('[aria-label="模型 1 输入价格"]').element as HTMLInputElement).value).toBe('2.5')
    await w.get('[aria-label="图片单价（元/张）"]').setValue('')
    expect((w.get('[aria-label="图片倍率"]').element as HTMLInputElement).value).toBe('1')
    await w.get('[aria-label="图片倍率"]').setValue('')
    await w.get('[aria-label="模型 1 输出价格"]').setValue('10')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(client.put).toHaveBeenCalledWith('/admin/exclusive-channels/1', expect.objectContaining({ image_price: 0.05, image_rate_multiplier: 1, model_pricing: [{ model: 'gpt-6-astra', input_price: 0.0000025, output_price: 0.00001, cache_read_price: 0 }] }))
  })

  it('includes the displayed pricing revision in a claim', async () => {
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, pricing_revision: 3, pricing_synced: true, image_price: 0.08 }], page: 1, pages: 1, total: 1 } })
    const w = render(); await flushPromises()
    await w.findAll('button').find(b => b.text() === '领取通道')!.trigger('click')
    expect(w.get('[role="dialog"]').text()).toContain('¥0.08')
    await w.findAll('button').find(b => b.text().startsWith('确认领取'))!.trigger('click'); await flushPromises()
    expect(client.post).toHaveBeenCalledWith('/exclusive-channels/1/claim', { duration: 1440, rate_multiplier: 1, pricing_revision: 3 })
  })

  it('blocks claims while legacy image prices await synchronization', async () => {
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, pricing_synced: false }], page: 1, pages: 1, total: 1 } })
    const w = render(); await flushPromises()
    expect(w.get('.ex-claim').attributes('disabled')).toBeDefined()
    expect(w.get('.ex-claim').text()).toBe('价格同步中')
  })

  it('shows quota inline and claims with the default 24 hours after warning', async () => {
    const w = render()
    await flushPromises()
    expect(w.text()).toContain('72%')
    expect(w.get('.exclusive-account-badge').text()).toContain('OpenAI')
    expect(w.get('.exclusive-account-badge').text()).toContain('OAuth')
    expect(w.get('.exclusive-account-badge').text()).toContain('Plus')
    expect(w.text()).toContain('暂无数据')
    expect(w.text()).toContain('额度更新时间未知')
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    await w.findAll('button').find(b => b.text() === '领取通道')!.trigger('click')
    expect((w.get('select[aria-label="使用时长"]').element as HTMLSelectElement).value).toBe('1440')
    expect(w.get('.ex-warning-box').text()).toContain('连续 30 分钟未产生消费将自动回收')
    expect(client.post).not.toHaveBeenCalled()
    await w.findAll('button').find(b => b.text().startsWith('确认领取'))!.trigger('click')
    await flushPromises()
    expect(client.post).toHaveBeenCalledWith('/exclusive-channels/1/claim', { duration: 1440, rate_multiplier: 1 })
  })

  it('does not probe subscription quota windows for API Key accounts', async () => {
    const apiKeyAccount = { ...account, type: 'apikey', plan_type: '', five_hour_remaining: null, weekly_remaining: null }
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, accounts: [apiKeyAccount] }], page: 1, pages: 1, total: 1 } })
    const w = render(); await flushPromises()
    expect(w.text()).toContain('API Key 账号不提供订阅额度窗口')
    expect(w.text()).not.toContain('5 小时窗口')
    expect(w.find('[aria-label="刷新 账号 01 用量"]').exists()).toBe(false)
  })

  it('publishes a name and selected accounts with administrator cooldown', async () => {
    state.path = '/admin/exclusive-channels'
    const w = render()
    await flushPromises()
    await w.findAll('button').find(b => b.text() === '发布通道')!.trigger('click')
    await flushPromises()
    await w.get('input[aria-label="通道名称"]').setValue('New channel')
    expect((w.get('input[aria-label="通道倍率"]').element as HTMLInputElement).value).toBe('1')
    expect(w.get('[role="dialog"] .exclusive-account-badge').text()).toContain('OAuth')
    await w.get('input[aria-label="Account 1"]').setValue(true)
    await w.get('input[aria-label="归还后冷却时间（秒）"]').setValue(120)
    await w.get('input[aria-label="通道倍率"]').setValue(1.25)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(client.post).toHaveBeenCalledWith('/admin/exclusive-channels', expect.objectContaining({ name: 'New channel', account_ids: [1], cooldown_seconds: 120, rate_multiplier: 1.25 }))
  })

  it('keeps failed publication visible without showing success', async () => {
    state.path = '/admin/exclusive-channels'
    client.post.mockRejectedValue(new Error('Core unavailable'))
    const w = render()
    await flushPromises()
    await w.findAll('button').find(b => b.text() === '发布通道')!.trigger('click')
    await flushPromises()
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('Core unavailable')
    expect(w.text()).not.toContain('通道配置已保存')
  })

  it('edits an available channel multiplier and shows it in the administrator list', async () => {
    state.path = '/admin/exclusive-channels'
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, rate_multiplier: 1.75 }], page: 1, pages: 1, total: 1 } })
    client.put.mockResolvedValue({ data: { id: 1 } })
    const w = render(); await flushPromises()
    expect(w.text()).toContain('1.75×')
    await w.get('[aria-label="编辑 Channel A"]').trigger('click')
    expect((w.get('input[aria-label="通道倍率"]').element as HTMLInputElement).value).toBe('1.75')
    await w.get('input[aria-label="通道倍率"]').setValue(0.75)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(client.put).toHaveBeenCalledWith('/admin/exclusive-channels/1', expect.objectContaining({ rate_multiplier: 0.75 }))
  })

  it('supports unbinding, replacing and adding accounts while editing', async () => {
    state.path = '/admin/exclusive-channels'
    const appended = { ...account, id: 2, name: 'Account 2' }
    client.get.mockImplementation(async (path: string) => {
      if (path.endsWith('/mine')) return { data: { lease: null, channel: null, balance: 20, server_time: new Date().toISOString() } }
      if (path === '/admin/exclusive-channels/accounts') return { data: { items: [appended], page: 1, pages: 1 } }
      if (path.endsWith('/events')) return { data: { items: [] } }
      return { data: { items: [{ ...channel, accounts: [account] }], page: 1, pages: 1, total: 1 } }
    })
    client.put.mockResolvedValue({ data: { id: 1 } })
    const w = render(); await flushPromises()
    await w.get('[aria-label="编辑 Channel A"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('可取消勾选解绑，也可重新勾选或选择新账号')
    await w.get('details.ex-picker summary').trigger('click')
    await w.get('input[aria-label="Account 1"]').setValue(false)
    await w.get('input[aria-label="Account 2"]').setValue(true)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(client.put).toHaveBeenCalledWith('/admin/exclusive-channels/1', expect.objectContaining({ account_ids: [2] }))
  })

  it('deletes an idle channel only after confirmation', async () => {
    state.path = '/admin/exclusive-channels'
    const w = render(); await flushPromises()
    await w.get('[aria-label="删除 Channel A"]').trigger('click')
    expect(w.get('[role="dialog"]').text()).toContain('删除后无法恢复')
    await w.findAll('button').find(button => button.text() === '确认删除')!.trigger('click')
    await flushPromises()
    expect(client.delete).toHaveBeenCalledWith('/admin/exclusive-channels/1')
  })

  it('allows occupied channels to save account and pricing edits for later activation', async () => {
    state.path = '/admin/exclusive-channels'
    const lease = { id: 9, channel_id: 1, user_id: 7, state: 'active', started_at: new Date().toISOString(), expires_at: new Date(Date.now() + 3600000).toISOString(), last_charge_at: null, ended_at: null }
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, state: 'active', rate_multiplier: 2, lease }], page: 1, pages: 1, total: 1 } })
    client.put.mockResolvedValue({ data: { id: 1, pending: true } })
    const w = render(); await flushPromises()
    await w.get('[aria-label="编辑 Channel A"]').trigger('click')
    expect(w.get('input[aria-label="通道倍率"]').attributes('disabled')).toBeUndefined()
    expect(w.get('input[aria-label="Account 1"]').attributes('disabled')).toBeUndefined()
    expect(w.text()).toContain('保存后将在当前使用结束并完成冷却后自动生效')
    await w.get('input[aria-label="通道倍率"]').setValue(1.5)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(client.put).toHaveBeenCalledWith('/admin/exclusive-channels/1', expect.objectContaining({ rate_multiplier: 1.5, account_ids: [1] }))
  })

  it('restores pending account and pricing edits when reopening the editor', async () => {
    state.path = '/admin/exclusive-channels'
    const pendingAccount = { ...account, id: 2, name: 'Account 2' }
    client.get.mockResolvedValueOnce({ data: { items: [{ ...channel, state: 'active', rate_multiplier: 2, pending_accounts: [pendingAccount], pending_config: { account_ids: [2], rate_multiplier: 1.5, model_pricing: [], image_price: 0.08, image_rate_multiplier: 1 } }], page: 1, pages: 1, total: 1 } })
    const w = render(); await flushPromises()
    expect(w.get('.ex-pricing-cell').text()).toContain('1.5×')
    expect(w.get('.ex-pricing-cell').text()).toContain('待生效 · 当前 2×')
    expect(w.get('.ex-pricing-cell').text()).toContain('待生效图片 ¥0.08 / 张 × 1')
    await w.get('[aria-label="编辑 Channel A"]').trigger('click'); await flushPromises()
    expect((w.get('input[aria-label="通道倍率"]').element as HTMLInputElement).value).toBe('1.5')
    expect((w.get('input[aria-label="Account 1"]').element as HTMLInputElement).checked).toBe(false)
    expect((w.get('input[aria-label="Account 2"]').element as HTMLInputElement).checked).toBe(true)
    expect((w.get('[aria-label="图片单价（元/张）"]').element as HTMLInputElement).value).toBe('0.08')
  })
})
