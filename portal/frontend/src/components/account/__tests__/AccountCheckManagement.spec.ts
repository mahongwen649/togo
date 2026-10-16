import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AccountCheckManagement from '../AccountCheckManagement.vue'

const mocks = vi.hoisted(() => ({
  updateGroups: vi.fn(),
  listPlans: vi.fn(),
  createPlan: vi.fn(),
  updatePlan: vi.fn(),
  deletePlan: vi.fn(),
  listResults: vi.fn()
}))

vi.mock('@/api/admin/accountChecks', () => ({
  updateCheckAccountGroups: mocks.updateGroups,
  listScheduledChecks: mocks.listPlans,
  createScheduledCheck: mocks.createPlan,
  updateScheduledCheck: mocks.updatePlan,
  deleteScheduledCheck: mocks.deletePlan,
  listScheduledCheckResults: mocks.listResults
}))

const account = { id: 42, name: '账号 42', platform: 'openai', group_ids: [7], groups: [{ id: 7, name: '分组 A' }] }
const groups = [
  { id: 7, name: '分组 A', platform: 'openai', status: 'active' as const },
  { id: 8, name: '分组 B', platform: 'openai', status: 'active' as const }
]
const plan = { id: 9, account_id: 42, model_id: 'gpt-6-astra', cron_expression: '0 * * * *', enabled: true, max_results: 3, last_run_at: '2026-09-19T06:00:00Z', next_run_at: '2026-09-19T07:00:00Z', created_at: '', updated_at: '' }
const results = [1, 2, 3].map(id => ({ id, plan_id: 9, status: 'success', response_text: `结果 ${id}`, error_message: '', latency_ms: 100 + id, started_at: `2026-09-19T0${id}:00:00Z`, finished_at: `2026-09-19T0${id}:00:01Z`, created_at: '' }))

function render() {
  return mount(AccountCheckManagement, { props: { account, groups, groupsLoading: false, models: [{ id: 'gpt-6-astra', display_name: 'Astra' }] }, global: { stubs: { Icon: true } } })
}

beforeEach(() => {
  vi.clearAllMocks()
  mocks.updateGroups.mockResolvedValue(account)
  mocks.listPlans.mockResolvedValue([plan])
  mocks.listResults.mockImplementation(async (_accountId: number, _planId: number, limit: number) => limit === 1 ? [results[0]] : results)
})

describe('account check management', () => {
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
