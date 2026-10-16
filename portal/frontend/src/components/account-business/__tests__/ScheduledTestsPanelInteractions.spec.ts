import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import ScheduledTestsPanel from '@/components/account-business/ScheduledTestsPanel.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { adminAPI } from '@/api/admin'
import type { ScheduledTestPlan, ScheduledTestResult } from '@/types'

const { showSuccess, showError } = vi.hoisted(() => ({
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    scheduledTests: {
      listByAccount: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      listResults: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))

const scheduledTests = vi.mocked(adminAPI.scheduledTests, true)
const accountId = 42
const editSelector = 'button[title="admin.scheduledTests.editPlan"]'
const deleteSelector = 'button[title="admin.scheduledTests.deletePlan"]'
const resultsSelector = '.border-t.border-gray-100'
const mountedPanels: VueWrapper[] = []

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function makePlan(id: number, modelId: string): ScheduledTestPlan {
  return {
    id,
    account_id: accountId,
    model_id: modelId,
    cron_expression: '*/30 * * * *',
    enabled: true,
    max_results: 100,
    auto_recover: false,
    last_run_at: null,
    next_run_at: '2026-09-12T12:30:00Z',
    created_at: '2026-09-12T12:00:00Z',
    updated_at: '2026-09-12T12:00:00Z',
  }
}

function makePlans() {
  return [makePlan(101, 'model-a'), makePlan(202, 'model-b')]
}

function makeResult(planId: number, text: string, latency: number): ScheduledTestResult {
  return {
    id: planId * 10,
    plan_id: planId,
    status: 'success',
    response_text: text,
    error_message: '',
    latency_ms: latency,
    started_at: '2026-09-12T12:00:00Z',
    finished_at: '2026-09-12T12:00:01Z',
    created_at: '2026-09-12T12:00:01Z',
  }
}

async function openPanel(plans = makePlans()) {
  scheduledTests.listByAccount.mockResolvedValue(plans)
  const wrapper = mount(ScheduledTestsPanel, {
    props: {
      show: false,
      accountId,
      modelOptions: ['model-a', 'model-b', 'model-new'].map((value) => ({ value, label: value })),
    },
    global: {
      // Keep the real dialog, confirmation buttons, inputs, select and toggles.
      stubs: { teleport: true, HelpTooltip: true, Icon: true },
    },
  })
  mountedPanels.push(wrapper)
  await wrapper.setProps({ show: true })
  await flushPromises()
  expect(scheduledTests.listByAccount).toHaveBeenCalledWith(accountId)
  return wrapper
}

function buttonWithText(scope: VueWrapper | DOMWrapper<Element>, text: string) {
  const button = scope.findAll<HTMLButtonElement>('button').find((item) => item.text() === text)
  if (!button) throw new Error(`Button not found: ${text}`)
  return button
}

function planHeader(wrapper: VueWrapper, modelId: string) {
  const header = wrapper.findAll<HTMLElement>('div.cursor-pointer').find((item) =>
    item.find(editSelector).exists() && item.text().includes(modelId),
  )
  if (!header) throw new Error(`Plan header not found: ${modelId}`)
  return header
}

function planCard(wrapper: VueWrapper, modelId: string) {
  return new DOMWrapper(planHeader(wrapper, modelId).element.parentElement!)
}

async function expandResponseDetail(card: DOMWrapper<Element>) {
  const detail = card.findAll('div.cursor-pointer').find((item) =>
    item.text() === 'admin.scheduledTests.responseText',
  )
  if (!detail) throw new Error('Response detail control not found')
  await detail.trigger('click')
}

describe('ScheduledTestsPanel mounted interactions', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    scheduledTests.listResults.mockResolvedValue([])
  })

  afterEach(() => {
    mountedPanels.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('submits creation once for same-tick and pending repeat clicks', async () => {
    const request = deferred<ScheduledTestPlan>()
    scheduledTests.create.mockReturnValue(request.promise)
    const wrapper = await openPanel([])

    await buttonWithText(wrapper, 'admin.scheduledTests.addPlan').trigger('click')
    await wrapper.get('button[aria-label="Select option"]').trigger('click')
    const option = wrapper.findAll('[role="option"]').find((item) => item.text() === 'model-new')
    expect(option).toBeDefined()
    await option!.trigger('click')
    await wrapper.get('input[type="text"]').setValue('0 * * * *')
    await wrapper.get('input[type="number"]').setValue('25')
    await wrapper.findAll('button[role="switch"]')[1].trigger('click')

    const save = buttonWithText(wrapper, 'common.save')
    // Do not await between these clicks: exercise the handler before disabled is patched.
    save.element.click()
    save.element.click()
    await nextTick()
    expect(save.element.disabled).toBe(true)
    expect(save.find('.animate-spin').exists()).toBe(true)
    await save.trigger('click')
    expect(scheduledTests.create).toHaveBeenCalledTimes(1)
    expect(scheduledTests.create).toHaveBeenCalledWith({
      account_id: accountId,
      model_id: 'model-new',
      cron_expression: '0 * * * *',
      enabled: true,
      max_results: 25,
      auto_recover: true,
    })

    const created = { ...makePlan(303, 'model-new'), cron_expression: '0 * * * *', max_results: 25, auto_recover: true }
    scheduledTests.listByAccount.mockResolvedValue([created])
    request.resolve(created)
    await flushPromises()
    expect(scheduledTests.listByAccount).toHaveBeenCalledTimes(2)
    expect(planHeader(wrapper, 'model-new').text()).toContain('0 * * * *')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(showError).not.toHaveBeenCalled()
  })

  it('submits deletion once and locks confirmation and cancellation while pending', async () => {
    const request = deferred<void>()
    scheduledTests.delete.mockReturnValue(request.promise)
    const wrapper = await openPanel()
    await planHeader(wrapper, 'model-a').get(deleteSelector).trigger('click')
    const dialog = wrapper.getComponent(ConfirmDialog)
    const confirm = buttonWithText(dialog, 'common.delete')

    confirm.element.click()
    confirm.element.click()
    await nextTick()
    expect(scheduledTests.delete).toHaveBeenCalledTimes(1)
    expect(dialog.props('confirmLoading')).toBe(true)
    const pendingConfirm = buttonWithText(dialog, 'common.delete')
    const cancel = buttonWithText(dialog, 'common.cancel')
    expect(pendingConfirm.element.disabled).toBe(true)
    expect(cancel.element.disabled).toBe(true)
    expect(pendingConfirm.find('.animate-spin').exists()).toBe(true)
    await pendingConfirm.trigger('click')
    await cancel.trigger('click')
    await dialog.get('button[aria-label="Close modal"]').trigger('click')
    expect(dialog.props('show')).toBe(true)
    expect(scheduledTests.delete).toHaveBeenCalledTimes(1)
    expect(scheduledTests.delete).toHaveBeenCalledWith(accountId, 101)

    request.resolve(undefined)
    await flushPromises()
    expect(dialog.props('show')).toBe(false)
    expect(wrapper.text()).not.toContain('model-a')
    expect(planHeader(wrapper, 'model-b').get<HTMLButtonElement>(editSelector).element.disabled).toBe(false)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(showError).not.toHaveBeenCalled()
  })

  it('clears the selected deletion after cancellation and allows the plan to be toggled', async () => {
    const plans = makePlans()
    const wrapper = await openPanel(plans)
    const header = planHeader(wrapper, 'model-a')
    await header.get(deleteSelector).trigger('click')
    expect(header.get<HTMLButtonElement>('button[role="switch"]').element.disabled).toBe(true)

    const dialog = wrapper.getComponent(ConfirmDialog)
    await buttonWithText(dialog, 'common.cancel').trigger('click')
    expect(dialog.props('show')).toBe(false)
    expect(header.get<HTMLButtonElement>('button[role="switch"]').element.disabled).toBe(false)
    expect(header.get<HTMLButtonElement>(editSelector).element.disabled).toBe(false)
    expect(header.get<HTMLButtonElement>(deleteSelector).element.disabled).toBe(false)
    expect(scheduledTests.delete).not.toHaveBeenCalled()

    scheduledTests.update.mockResolvedValue({ ...plans[0], enabled: false })
    await header.get('button[role="switch"]').trigger('click')
    await flushPromises()
    expect(scheduledTests.update).toHaveBeenCalledTimes(1)
    expect(scheduledTests.update).toHaveBeenCalledWith(accountId, 101, { enabled: false })
    expect(header.get('button[role="switch"]').attributes('aria-checked')).toBe('false')
  })

  it('blocks duplicate saves, cancellation, toggling and deletion of the plan being edited', async () => {
    const plans = makePlans()
    const request = deferred<ScheduledTestPlan>()
    scheduledTests.update.mockReturnValue(request.promise)
    const wrapper = await openPanel(plans)
    const header = planHeader(wrapper, 'model-a')
    await header.get(editSelector).trigger('click')
    const card = planCard(wrapper, 'model-a')
    await card.get('input[type="text"]').setValue('0 6 * * *')
    const save = buttonWithText(card, 'common.save')
    save.element.click()
    save.element.click()
    await nextTick()

    expect(save.element.disabled).toBe(true)
    expect(save.find('.animate-spin').exists()).toBe(true)
    const conflicts = [
      buttonWithText(card, 'common.cancel'),
      header.get<HTMLButtonElement>(editSelector),
      header.get<HTMLButtonElement>(deleteSelector),
      header.get<HTMLButtonElement>('button[role="switch"]'),
    ]
    for (const control of conflicts) {
      expect(control.element.disabled).toBe(true)
      await control.trigger('click')
    }
    expect(scheduledTests.update).toHaveBeenCalledTimes(1)
    expect(scheduledTests.update).toHaveBeenCalledWith(accountId, 101, {
      model_id: 'model-a',
      cron_expression: '0 6 * * *',
      max_results: 100,
      enabled: true,
      auto_recover: false,
    })
    expect(scheduledTests.delete).not.toHaveBeenCalled()
    expect(wrapper.getComponent(ConfirmDialog).props('show')).toBe(false)
    expect(card.get<HTMLInputElement>('input[type="text"]').element.value).toBe('0 6 * * *')

    request.resolve({ ...plans[0], cron_expression: '0 6 * * *' })
    await flushPromises()
    expect(card.find('input').exists()).toBe(false)
    expect(header.text()).toContain('0 6 * * *')
    expect(header.get<HTMLButtonElement>(editSelector).element.disabled).toBe(false)
    expect(planHeader(wrapper, 'model-b').text()).toContain('*/30 * * * *')
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps the in-flight edit target locked when another plan edit is clicked', async () => {
    const plans = makePlans()
    const request = deferred<ScheduledTestPlan>()
    scheduledTests.update.mockReturnValue(request.promise)
    const wrapper = await openPanel(plans)
    await planHeader(wrapper, 'model-a').get(editSelector).trigger('click')
    await planCard(wrapper, 'model-a').get('input[type="text"]').setValue('0 6 * * *')
    await buttonWithText(planCard(wrapper, 'model-a'), 'common.save').trigger('click')
    await planHeader(wrapper, 'model-b').get(editSelector).trigger('click')

    expect.soft(planCard(wrapper, 'model-a').find('input').exists(), 'Saving A must retain its edit target').toBe(true)
    expect.soft(planCard(wrapper, 'model-b').find('input').exists(), 'A shared pending edit must not be retargeted to B').toBe(false)
    expect.soft(planHeader(wrapper, 'model-a').get<HTMLButtonElement>(deleteSelector).element.disabled,
      'Plan A must remain protected from deletion until its update settles').toBe(true)

    request.resolve({ ...plans[0], cron_expression: '0 6 * * *' })
    await flushPromises()
    expect(scheduledTests.update).toHaveBeenCalledTimes(1)
    expect(planHeader(wrapper, 'model-a').text()).toContain('0 6 * * *')
    expect(planHeader(wrapper, 'model-b').text()).toContain('*/30 * * * *')
  })

  it.each(['resolve', 'reject'] as const)('ignores an obsolete A result that %ss after B has rendered', async (outcome) => {
    const first = deferred<ScheduledTestResult[]>()
    const second = deferred<ScheduledTestResult[]>()
    scheduledTests.listResults.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = await openPanel()
    await planHeader(wrapper, 'model-a').trigger('click')
    expect(planCard(wrapper, 'model-a').get(resultsSelector).text()).toContain('common.loading')
    await planHeader(wrapper, 'model-b').trigger('click')
    expect(planCard(wrapper, 'model-a').find(resultsSelector).exists()).toBe(false)
    expect(scheduledTests.listResults).toHaveBeenNthCalledWith(1, accountId, 101, 20)
    expect(scheduledTests.listResults).toHaveBeenNthCalledWith(2, accountId, 202, 20)

    second.resolve([makeResult(202, 'current B response', 222)])
    await flushPromises()
    await expandResponseDetail(planCard(wrapper, 'model-b'))
    expect(planCard(wrapper, 'model-b').get('pre').text()).toBe('current B response')

    if (outcome === 'resolve') first.resolve([makeResult(101, 'obsolete A response', 111)])
    else first.reject(new Error('obsolete A request failed'))
    await flushPromises()
    expect(planCard(wrapper, 'model-b').get('pre').text()).toBe('current B response')
    expect(planCard(wrapper, 'model-b').text()).toContain('222ms')
    expect(wrapper.text()).not.toContain('111ms')
    expect(wrapper.text()).not.toContain('common.loading')
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps B loading when the earlier A request settles first', async () => {
    const first = deferred<ScheduledTestResult[]>()
    const second = deferred<ScheduledTestResult[]>()
    scheduledTests.listResults.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = await openPanel()
    await planHeader(wrapper, 'model-a').trigger('click')
    await planHeader(wrapper, 'model-b').trigger('click')
    first.resolve([makeResult(101, 'obsolete A response', 111)])
    await flushPromises()
    expect(planCard(wrapper, 'model-b').get(resultsSelector).text()).toContain('common.loading')
    expect(wrapper.text()).not.toContain('111ms')

    second.resolve([makeResult(202, 'current B response', 222)])
    await flushPromises()
    await expandResponseDetail(planCard(wrapper, 'model-b'))
    expect(planCard(wrapper, 'model-b').get('pre').text()).toBe('current B response')
    expect(wrapper.text()).not.toContain('common.loading')
  })

  it('does not refill result state after the plan has been collapsed', async () => {
    const request = deferred<ScheduledTestResult[]>()
    scheduledTests.listResults.mockReturnValueOnce(request.promise)
    const wrapper = await openPanel()
    const header = planHeader(wrapper, 'model-a')
    await header.trigger('click')
    await header.trigger('click')
    expect(planCard(wrapper, 'model-a').find(resultsSelector).exists()).toBe(false)

    request.resolve([makeResult(101, 'collapsed stale response', 111)])
    await flushPromises()
    expect(planCard(wrapper, 'model-a').find(resultsSelector).exists()).toBe(false)
    // The hidden DOM alone cannot prove that a stale response did not refill state.
    const state = wrapper.vm as unknown as { results: ScheduledTestResult[]; loadingResults: boolean }
    expect(state.results).toEqual([])
    expect(state.loadingResults).toBe(false)
    expect(scheduledTests.listResults).toHaveBeenCalledTimes(1)
    expect(scheduledTests.listResults).toHaveBeenCalledWith(accountId, 101, 20)
    expect(showError).not.toHaveBeenCalled()
  })

  it('ignores the old response when the same plan is collapsed and reopened before completion', async () => {
    const first = deferred<ScheduledTestResult[]>()
    const second = deferred<ScheduledTestResult[]>()
    scheduledTests.listResults.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = await openPanel()
    const header = planHeader(wrapper, 'model-a')
    await header.trigger('click')
    await header.trigger('click')
    await header.trigger('click')
    first.resolve([makeResult(101, 'old A response', 111)])
    await flushPromises()
    expect(planCard(wrapper, 'model-a').get(resultsSelector).text()).toContain('common.loading')
    expect(wrapper.text()).not.toContain('111ms')

    second.resolve([makeResult(101, 'fresh A response', 333)])
    await flushPromises()
    await expandResponseDetail(planCard(wrapper, 'model-a'))
    expect(planCard(wrapper, 'model-a').get('pre').text()).toBe('fresh A response')
    expect(planCard(wrapper, 'model-a').text()).toContain('333ms')
    expect(scheduledTests.listResults).toHaveBeenCalledTimes(2)
    expect(showError).not.toHaveBeenCalled()
  })
})
