import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../../../../../')
const read = (base: string, file: string) => readFileSync(resolve(root, base, file), 'utf8')

describe('Portal scheduled test panel parity', () => {
  it('keeps the Core plan and result interaction surface', () => {
    const core = read('core/frontend/src', 'components/admin/account/ScheduledTestsPanel.vue')
    const portal = read('portal/frontend/src', 'components/account-business/ScheduledTestsPanel.vue')

    for (const marker of [
      'showAddForm',
      'handleCreate',
      'handleToggleEnabled',
      'startEdit',
      'handleEdit',
      'confirmDeletePlan',
      'handleDelete',
      'toggleExpand',
      'loadingResults',
      'noResults',
      'responseText',
      'errorMessage',
      'autoRecover',
      'ConfirmDialog'
    ]) {
      expect(core).toContain(marker)
      expect(portal, `missing Core scheduled-test marker: ${marker}`).toContain(marker)
    }
  })

  it('uses account-scoped Portal routes for every plan operation', () => {
    const api = read('portal/frontend/src', 'api/admin/scheduledTests.ts')
    for (const marker of [
      '/account-market/accounts/${accountId}/scheduled-test-plans',
      'listByAccount',
      'create',
      'update',
      'deletePlan',
      'listResults'
    ]) {
      expect(api).toContain(marker)
    }
  })

  it('guards plan mutations and stale result responses', () => {
    const portal = read('portal/frontend/src', 'components/account-business/ScheduledTestsPanel.vue')
    for (const marker of [
      'const deleting = ref(false)',
      'if (creating.value ||',
      'const planId = editingPlanId.value',
      'const plan = deletingPlan.value',
      'let resultsRequestSequence =',
      'requestSequence !== resultsRequestSequence',
      'confirm-loading="deleting"',
      'cancelDelete',
      'isPlanBusy'
    ]) {
      expect(portal, `missing scheduled-test safety marker: ${marker}`).toContain(marker)
    }
  })
})
