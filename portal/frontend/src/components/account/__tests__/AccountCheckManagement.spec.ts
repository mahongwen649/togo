import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountCheckManagement from '../AccountCheckManagement.vue'

const mocks = vi.hoisted(() => ({
  updateGroups: vi.fn(),
  listPlans: vi.fn(),
  createPlan: vi.fn(),
  updatePlan: vi.fn(),
  deletePlan: vi.fn(),
  listResults: vi.fn(),
  listLegacy: vi.fn(), pauseLegacy: vi.fn()
}))

vi.mock('@/api/admin/accountChecks', () => ({
  updateCheckAccountGroups: mocks.updateGroups,
  listScheduledChecks: mocks.listPlans,
  createScheduledCheck: mocks.createPlan,
  updateScheduledCheck: mocks.updatePlan,
  deleteScheduledCheck: mocks.deletePlan,
  listScheduledCheckResults: mocks.listResults,
  listLegacyScheduledChecks: mocks.listLegacy, pauseLegacyScheduledCheck: mocks.pauseLegacy
}))

const account = { id: 42, name: '账号 42', platform: 'openai', group_ids: [7], groups: [{ id: 7, name: '分组 A' }] }
const groups = [
  { id: 7, name: '分组 A', platform: 'openai', status: 'active' as const },
  { id: 8, name: '分组 B', platform: 'openai', status: 'active' as const }
]
const plan = { id: 9, account_id: 42, model_id: 'gpt-6-astra', reasoning_effort: 'high', cron_expression: '0 * * * *', enabled: true, max_results: 3, last_run_at: '2026-09-19T06:00:00Z', next_run_at: '2026-09-19T07:00:00Z', created_at: '', updated_at: '' }
const results = [1, 2, 3].map(id => ({ id, plan_id: 9, status: 'success', response_text: `结果 ${id}`, error_message: '', latency_ms: 100 + id, started_at: `2026-09-19T0${id}:00:00Z`, finished_at: `2026-09-19T0${id}:00:01Z`, created_at: '' }))

function render() {
  return mount(AccountCheckManagement, { props: { account, groups, groupsLoading: false, models: [{ id: 'gpt-6-astra', display_name: 'Astra' }] }, global: { stubs: { Icon: true } } })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.updateGroups.mockResolvedValue(account)
  mocks.listPlans.mockResolvedValue([plan])
  mocks.listLegacy.mockResolvedValue([])
  mocks.createPlan.mockImplementation(async (_id, payload) => ({ ...plan, ...payload }))
  mocks.updatePlan.mockImplementation(async (_id, _planID, payload) => ({ ...plan, ...payload }))
  mocks.listResults.mockImplementation(async (_accountId: number, _planId: number, limit: number) => limit === 1 ? [results[0]] : results)
})

describe('account check management', () => {
  async function openSchedule(wrapper: ReturnType<typeof render>) {
    await wrapper.findAll('button').find(button => button.text() === '定时检查')!.trigger('click')
    await flushPromises()
  }

  it('opens an empty account directly into setup and starts the chosen model, effort and Beijing time', async () => {
    mocks.listPlans.mockResolvedValue([])
    const wrapper = render()
    await wrapper.setProps({ selectedModel: 'gpt-6-sol', models: [{ id: 'gpt-6-astra', display_name: 'Astra' }, { id: 'gpt-6-sol', display_name: 'Sol' }] })
    await openSchedule(wrapper)
    expect(wrapper.get<HTMLSelectElement>('[aria-label="定时检查模型"]').element.value).toBe('gpt-6-sol')
    expect(wrapper.find('[aria-label="Cron 表达式"]').exists()).toBe(false)
    await wrapper.get('[aria-label="定时检查思考程度"]').setValue('xhigh')
    await wrapper.get('[aria-label="执行频率"]').setValue('daily')
    await wrapper.get('[aria-label="执行时间"]').setValue('21:30')
    expect(wrapper.text()).toContain('每天 21:30（北京时间）')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.createPlan).toHaveBeenCalledWith(42, { model_id: 'gpt-6-sol', reasoning_effort: 'xhigh', prompt: 'hi', cron_expression: 'CRON_TZ=Asia/Shanghai 30 21 * * *', enabled: true })
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('定时运行中')
    expect(wrapper.text()).toContain('超高（xhigh）')
    expect(wrapper.text()).toContain('下次执行（北京时间）')
  })

  it('keeps advanced schedules and paused state on edit, and allows resetting effort to model default', async () => {
    mocks.listPlans.mockResolvedValue([{ ...plan, enabled: false }])
    const wrapper = render()
    await openSchedule(wrapper)
    await wrapper.get('[aria-label="编辑计划 9"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('[aria-label="Cron 表达式"]').element.value).toBe('0 * * * *')
    await wrapper.get('[aria-label="定时检查思考程度"]').setValue('')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.updatePlan).toHaveBeenCalledWith(42, 9, { model_id: 'gpt-6-astra', reasoning_effort: '', prompt: 'hi', cron_expression: '0 * * * *', enabled: false })
    expect(wrapper.text()).toContain('已暂停')
    expect(wrapper.text()).not.toContain('下次执行（北京时间）')
  })

  it('preserves the setup after a failed save so the user can retry', async () => {
    mocks.listPlans.mockResolvedValue([])
    mocks.createPlan.mockRejectedValue(new Error('保存失败'))
    const wrapper = render()
    await openSchedule(wrapper)
    await wrapper.get('[aria-label="执行频率"]').setValue('weekly')
    await wrapper.get('[aria-label="星期"]').setValue('5')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('保存失败')
    expect(wrapper.get<HTMLSelectElement>('[aria-label="星期"]').element.value).toBe('5')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('pauses and resumes without overwriting model, effort or schedule', async () => {
    const wrapper = render()
    await openSchedule(wrapper)
    await wrapper.findAll('button').find(button => button.text() === '暂停')!.trigger('click')
    await flushPromises()
    expect(mocks.updatePlan).toHaveBeenLastCalledWith(42, 9, { enabled: false })
    await wrapper.findAll('button').find(button => button.text() === '继续')!.trigger('click')
    await flushPromises()
    expect(mocks.updatePlan).toHaveBeenLastCalledWith(42, 9, { enabled: true })
  })

  it('copies the current question into a plan and keeps existing Core plans visible', async () => {
    mocks.listPlans.mockResolvedValue([])
    mocks.listLegacy.mockResolvedValue([plan])
    mocks.pauseLegacy.mockResolvedValue({ ...plan, enabled: false })
    const wrapper = render()
    await wrapper.setProps({ prompt: '生成 SVG 动画' })
    await openSchedule(wrapper)
    expect(wrapper.get<HTMLTextAreaElement>('[aria-label="定时检查题目"]').element.value).toBe('生成 SVG 动画')
    expect(wrapper.text()).toContain('旧版定时检查 #9')
    await wrapper.findAll('button').find(button => button.text() === '暂停旧计划')!.trigger('click')
    await flushPromises()
    expect(mocks.pauseLegacy).toHaveBeenCalledWith(42, 9)
    expect(wrapper.text()).toContain('旧版定时检查 #9 · 已暂停')
    expect(mocks.updatePlan).not.toHaveBeenCalled()
  })

  it('updates the selected account groups and reports the new IDs', async () => {
    const wrapper = render()
    await wrapper.get('button').trigger('click')
    await wrapper.get('[data-group-id="8"]').setValue(true)
    const save = wrapper.findAll('button').find(button => button.text() === '保存')!
    await save.trigger('click')
    await flushPromises()
    expect(mocks.updateGroups).toHaveBeenCalledWith(42, [7, 8])
    expect(wrapper.emitted('groups-updated')).toEqual([[[7, 8]]])
  })

  it('shows the latest scheduled result first and loads only three records on demand', async () => {
    const wrapper = render()
    const scheduleButton = wrapper.findAll('button').find(button => button.text().includes('定时检查'))!
    await scheduleButton.trigger('click')
    await flushPromises()
    expect(mocks.listPlans).toHaveBeenCalledWith(42)
    expect(mocks.listResults).toHaveBeenCalledWith(42, 9, 1)
    expect(wrapper.get('[data-schedule-plan="9"]').text()).toContain('结果 1')
    expect(wrapper.get('[data-schedule-plan="9"]').text()).toContain('09/19')
    await wrapper.get('[data-plan-history="9"]').trigger('click')
    await flushPromises()
    expect(mocks.listResults).toHaveBeenLastCalledWith(42, 9, 3)
    expect(wrapper.get('[data-schedule-plan="9"]').text()).toContain('结果 3')
  })
})
