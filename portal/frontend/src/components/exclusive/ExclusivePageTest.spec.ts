import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ExclusivePageTest from './ExclusivePageTest.vue'
import { pagePreviewDocument } from './pagePreview'
import type { ChannelAccount } from '@/api/exclusiveChannels'

const api = vi.hoisted(() => ({ downgradeTest: vi.fn(), startDowngradeTest: vi.fn() }))
vi.mock('@/api/exclusiveChannels', () => ({ exclusiveChannels: api }))
const html = '<!doctype html><html><body><svg></svg><script>window.demo=1</script></body></html>'
const saved = { state: 'succeeded' as const, started_at: null, generated_at: '2026-09-16T01:00:00Z', has_page: true, html }
const account: ChannelAccount = { id: 7, name: '账号 01', available: true, five_hour_remaining: null, weekly_remaining: null, observed_at: null, page_test: saved }
const wrappers: ReturnType<typeof mount>[] = []
function render(canTest = false, value = account) {
  const w = mount(ExclusivePageTest, { props: { channelId: 2, account: value, canTest }, global: { stubs: { BaseDialog: { props: ['show'], emits: ['close'], template: '<div v-if="show" role="dialog"><button aria-label="Close" @click="$emit(\'close\')">Close</button><slot /></div>' } } } })
  wrappers.push(w); return w
}
beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); api.downgradeTest.mockResolvedValue(saved); api.startDowngradeTest.mockResolvedValue(undefined) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.useRealTimers() })

describe('Account downgrade test', () => {
  it('loads and displays the saved page inline for non-holders', async () => {
    const w = render()
    await flushPromises()
    expect(w.text()).toContain('账号 01 · 降智测试')
    expect(w.text()).not.toContain('开始降智测试')
    expect(api.downgradeTest).toHaveBeenCalledWith(2, 7)
    const frame = w.get('iframe[title="降智测试页面预览"]')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('srcdoc')).toContain("frame-src 'none'")
    expect(w.find('svg script').exists()).toBe(false)
    expect(api.startDowngradeTest).not.toHaveBeenCalled()
  })
  it('opens the large preview only when requested', async () => {
    const w = render()
    await flushPromises()
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    await w.findAll('button').find(b => b.text() === '放大预览')!.trigger('click')
    expect(w.find('[role="dialog"]').exists()).toBe(true)
    expect(w.findAll('iframe')).toHaveLength(2)
  })
  it('keeps compact preview details and the thumbnail in separate layout regions', async () => {
    const w = render()
    await flushPromises()
    expect(w.get('.test-details').text()).toContain('账号 01 · 降智测试')
    expect(w.get('.inline-preview').find('iframe').exists()).toBe(true)
  })
  it('does not refetch the page when parent polling replaces equivalent metadata', async () => {
    const w = render()
    await flushPromises()
    expect(api.downgradeTest).toHaveBeenCalledTimes(1)
    await w.setProps({ account: { ...account, page_test: { ...saved, html: undefined } } })
    await flushPromises()
    expect(api.downgradeTest).toHaveBeenCalledTimes(1)
    expect(w.get('iframe[title="降智测试页面预览"]').attributes('srcdoc')).toContain('window.demo')
  })
  it('starts inline without opening the dialog and recovers the result by polling', async () => {
    const emptyAccount = { ...account, page_test: null }
    const w = render(true, emptyAccount)
    api.downgradeTest.mockResolvedValueOnce({ ...saved, state: 'running', has_page: false, html: undefined }).mockResolvedValueOnce({ ...saved, html: html.replace('demo', 'newDemo') })
    await w.findAll('button').find(b => b.text() === '开始降智测试')!.trigger('click')
    await flushPromises()
    expect(api.startDowngradeTest).toHaveBeenCalledTimes(1)
    expect(api.startDowngradeTest).toHaveBeenCalledWith(2, 7)
    expect(w.findAll('button').find(b => b.text().includes('测试中'))!.attributes('disabled')).toBeDefined()
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    expect(w.text()).toContain('服务器正在生成本轮页面')
    await vi.advanceTimersByTimeAsync(2500)
    await flushPromises()
    expect(w.get('iframe[title="降智测试页面预览"]').attributes('srcdoc')).toContain('newDemo')
    expect(w.emitted('changed')).toHaveLength(2)
  })
  it('keeps inline polling after the large preview is closed', async () => {
    api.downgradeTest.mockResolvedValueOnce({ ...saved, state: 'running' }).mockResolvedValueOnce({ ...saved, html: html.replace('demo', 'afterClose') })
    const w = render()
    await flushPromises()
    await w.findAll('button').find(b => b.text() === '放大预览')!.trigger('click')
    await w.get('[aria-label="Close"]').trigger('click')
    await vi.advanceTimersByTimeAsync(2500)
    await flushPromises()
    expect(api.downgradeTest).toHaveBeenCalledTimes(2)
    expect(w.get('iframe[title="降智测试页面预览"]').attributes('srcdoc')).toContain('afterClose')
    expect(w.find('[role="dialog"]').exists()).toBe(false)
  })
  it('retains the latest successful page after a failed generation', async () => {
    api.downgradeTest.mockResolvedValue({ ...saved, state: 'failed', error: '生成失败' })
    const w = render(true)
    await flushPromises()
    expect(w.get('iframe[title="降智测试页面预览"]').attributes('srcdoc')).toContain('window.demo')
    expect(w.text()).toContain('生成失败')
  })
  it('escapes generated markup inside the trusted policy wrapper', () => {
    const result = pagePreviewDocument('</iframe><script>evil()</script><iframe src="https://bad.invalid">')
    expect(result.match(/<iframe /g)).toHaveLength(1)
    expect(result).not.toContain('<script>evil()')
    expect(result).toContain('&lt;script&gt;evil()')
    expect(result).not.toContain('allow-same-origin')
    expect(result).toContain("connect-src 'none'")
    expect(result).toContain("form-action 'none'")
  })
  it('builds a sandboxed fit-to-window document for the inline preview', () => {
    const result = pagePreviewDocument(html, 'fit')
    expect(result).toContain('page-preview-size')
    expect(result).toContain("frame.style.transform='scale('")
    expect(result).toContain('Math.min(innerWidth/width,innerHeight/height)')
    expect(result).not.toContain('allow-same-origin')
    expect(result).toContain("connect-src 'none'")
  })
})
