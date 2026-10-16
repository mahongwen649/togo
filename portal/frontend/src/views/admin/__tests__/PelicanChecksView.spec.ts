import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PelicanChecksView from '../PelicanChecksView.vue'

const mocks = vi.hoisted(() => ({ get: vi.fn(), options: vi.fn(), run: vi.fn(), save: vi.fn(), remove: vi.fn(), error: vi.fn(), success: vi.fn(), fullAdmin: false }))
vi.mock('@/api/admin/pelicanChecks', () => ({ getPelicanChecks: mocks.get, getPelicanCheckOptions: mocks.options, runPelicanCheckNow: mocks.run, updatePelicanCheckConfig: mocks.save, deletePelicanArtwork: mocks.remove }))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isFullAdmin: mocks.fullAdmin }) }))

function response() {
  const at = new Date().toISOString()
  return { latest_run_id: 3, active_run_id: 0, generated_at: at, config: { reasoning_effort: 'high', model: 'gpt-6-astra', prompt: '默认题目', enabled: true, group_id: 1, api_key_id: 2 }, scheduler_healthy: true, next_run_at: new Date(Date.now() + 30 * 60_000).toISOString(), summary: { completed: 3, succeeded: 2, failed: 1, running: 0, pass_rate: 66.666 }, items: [{ group_id: 1, group_name: 'Group', api_key_id: 2, api_key_name: 'New Key', state: 'succeeded', started_at: at, latency_ms: 2000, timeline: [
    { id: 1, api_key_id: 1, api_key_name: 'Old Key', group_name: 'Group', state: 'succeeded', started_at: at, latency_ms: 2000, attempt: 1 },
    { id: 2, api_key_id: 2, api_key_name: 'New Key', group_name: 'Group', state: 'failed', started_at: at, latency_ms: 2000, attempt: 2, error: '上游连接中断' },
    { id: 3, api_key_id: 2, api_key_name: 'New Key', group_name: 'Group', state: 'succeeded', started_at: at, latency_ms: 2000, attempt: 1 }
  ] }], artworks: [] }
}
function render() {
  return mount(PelicanChecksView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, BaseDialog: true, ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<button v-if="show" class="confirm-delete" @click="$emit(\'confirm\')">确认删除</button>' }, Toggle: true, Icon: true } } })
}
beforeEach(() => { vi.useFakeTimers(); vi.clearAllMocks(); mocks.fullAdmin = false; localStorage.clear(); sessionStorage.clear(); mocks.get.mockResolvedValue(response()); mocks.options.mockResolvedValue({ groups: [], keys: [], models: ['custom-model'] }); mocks.remove.mockResolvedValue(undefined) })
afterEach(() => { vi.useRealTimers() })

describe('pelican actual records and scheduling status', () => {
  it('shows only artworks and omits rates, timelines and failure messages', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.find('.metric-grid').exists()).toBe(false)
    expect(wrapper.find('.timeline-section').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('通过率')
    expect(wrapper.text()).not.toContain('上游连接中断')
    expect(wrapper.text()).not.toContain('66.7%')
    expect(wrapper.text()).not.toContain('保存配置')
    wrapper.unmount()
  })
  it('saves custom prompt and model before an immediate check', async () => {
    mocks.fullAdmin = true
    mocks.options.mockResolvedValue({ groups: [{ id: 1, name: 'Group' }], keys: [{ id: 2, name: 'Key', preview: '****' }], models: ['custom-model'] })
    mocks.run.mockResolvedValue({ state: 'running', run_id: 4 })
    const wrapper = render(); await flushPromises()
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('默认题目')
    await wrapper.findAll('.prompt-config select')[0].setValue('custom-model')
    await wrapper.findAll('.prompt-config select')[1].setValue('low')
    await wrapper.find('textarea').setValue(' 绘制太阳系动画 ')
    await wrapper.findAll('button').find(button => button.text().includes('立即检查'))!.trigger('click')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith({ enabled: true, group_id: 1, api_key_id: 2, reasoning_effort: 'low', model: 'custom-model', prompt: '绘制太阳系动画' })
    expect(mocks.save.mock.invocationCallOrder[0]).toBeLessThan(mocks.run.mock.invocationCallOrder[0])
    wrapper.unmount()
  })
  it('rejects blank prompts and preserves edits during status refresh', async () => {
    mocks.fullAdmin = true
    const wrapper = render(); await flushPromises()
    await wrapper.find('textarea').setValue('正在编辑的题目')
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('正在编辑的题目')
    await wrapper.find('textarea').setValue('   ')
    expect(wrapper.findAll('button').find(button => button.text().includes('保存配置'))!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('automatically saves edited settings and reloads the persisted prompt', async () => {
    mocks.fullAdmin = true
    const server = response()
    mocks.get.mockImplementation(async () => structuredClone(server))
    mocks.save.mockImplementation(async input => { Object.assign(server.config, input); return server.config })
    const wrapper = render(); await flushPromises()
    await wrapper.find('textarea').setValue('苍鹭滑滑板')
    await vi.advanceTimersByTimeAsync(800); await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ prompt: '苍鹭滑滑板' }))
    expect(wrapper.text()).toContain('已自动保存')
    wrapper.unmount()
    const reopened = render(); await flushPromises()
    expect((reopened.find('textarea').element as HTMLTextAreaElement).value).toBe('苍鹭滑滑板')
    reopened.unmount()
  })
  it('restores an unsaved draft after leaving before debounce', async () => {
    mocks.fullAdmin = true
    const wrapper = render(); await flushPromises()
    await wrapper.find('textarea').setValue('未提交的题目')
    wrapper.unmount()
    const reopened = render(); await flushPromises()
    expect((reopened.find('textarea').element as HTMLTextAreaElement).value).toBe('未提交的题目')
    reopened.unmount()
  })
  it('shows overdue and unavailable scheduler states instead of an endless starting label', async () => {
    mocks.fullAdmin = true
    const data = response(); data.next_run_at = new Date(Date.now() - 60_000).toISOString()
    mocks.get.mockResolvedValue(data)
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('调度延迟')
    data.scheduler_healthy = false
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.text()).toContain('调度服务未响应')
    wrapper.unmount()
  })
  it('keeps polling an active job even with automatic refresh disabled', async () => {
    mocks.fullAdmin = true
    localStorage.setItem('pelican-check-auto-refresh', 'false')
    const data = response(); data.summary.running = 1; data.items[0].state = 'running'
    mocks.get.mockResolvedValue(data)
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('正在生成作品')
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(mocks.get).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('finishes manual polling by run ID even when the completed total is unchanged', async () => {
    mocks.fullAdmin = true
    localStorage.setItem('pelican-check-auto-refresh', 'false')
    mocks.run.mockResolvedValue({ state: 'running', run_id: 4 })
    mocks.options.mockResolvedValue({ groups: [{ id: 1, name: 'Group' }], keys: [{ id: 2, name: 'New Key', preview: '****' }] })
    const wrapper = render(); await flushPromises()
    const button = wrapper.findAll('button').find(button => button.text().includes('立即检查'))!
    await button.trigger('click'); await flushPromises()
    expect(mocks.run).toHaveBeenCalledTimes(1)
    const finished = response()
    finished.latest_run_id = 4
    finished.active_run_id = 0
    mocks.get.mockResolvedValue(finished)
    await vi.advanceTimersByTimeAsync(500); await flushPromises()
    expect(button.text()).toContain('立即检查')
    expect(button.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('finishes a skipped manual run without exposing a failure record', async () => {
    mocks.fullAdmin = true
    localStorage.setItem('pelican-check-auto-refresh', 'false')
    mocks.run.mockResolvedValue({ state: 'running', run_id: 4 })
    mocks.options.mockResolvedValue({ groups: [{ id: 1, name: 'Group' }], keys: [{ id: 2, name: 'New Key', preview: '****' }] })
    const wrapper = render(); await flushPromises()
    const button = wrapper.findAll('button').find(button => button.text().includes('立即检查'))!
    await button.trigger('click'); await flushPromises()
    const skipped = { ...response(), latest_run_id: 4, active_run_id: 0, items: [] }
    mocks.get.mockResolvedValue(skipped)
    await vi.advanceTimersByTimeAsync(500); await flushPromises()
    expect(button.text()).toContain('立即检查')
    expect(button.attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).not.toContain('失败')
    wrapper.unmount()
  })

  it('shows account identity and deletion only to full administrators', async () => {
    const data: any = response()
    data.artworks = [{ id: 9, group_id: 1, group_name: 'Group', api_key_id: 2, api_key_name: 'New Key', started_at: data.generated_at, finished_at: data.generated_at, latency_ms: 1000, html: '<html></html>', account_id: 42, account_name: '绘图账号' }]
    mocks.get.mockResolvedValue(data)

    const member = render(); await flushPromises()
    expect(member.text()).not.toContain('生成账号')
    expect(member.find('.artwork-delete').exists()).toBe(false)
    member.unmount()

    mocks.fullAdmin = true
    const admin = render(); await flushPromises()
    expect(admin.text()).toContain('生成账号：绘图账号 · #42')
    await admin.find('.artwork-delete').trigger('click')
    await admin.find('.confirm-delete').trigger('click')
    await flushPromises()
    expect(mocks.remove).toHaveBeenCalledWith(9)
    expect(mocks.success).toHaveBeenCalledWith('作品已删除')
    admin.unmount()
  })
})
