import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../../../../../')
const read = (base: string, file: string) => readFileSync(resolve(root, base, file), 'utf8')

describe('account package Core usage parity', () => {
  it('preserves Core usage-window and platform surfaces', () => {
    const core = read('core/frontend/src', 'components/account/AccountUsageCell.vue')
    const portal = read('portal/frontend/src', 'components/account/AccountUsageCell.vue')
    for (const marker of ['UsageProgressBar', 'five_hour', 'seven_day', 'seven_day_sonnet', 'seven_day_fable', 'window_stats', 'activeQuery', 'passiveSampled', 'OpenAIQuotaResetCell', 'GrokQuotaProbeCell', 'CNProviderQuotaCell', 'CNProviderBalanceCell', 'OllamaCloudUsageCell']) {
      expect(core).toContain(marker)
      expect(portal, `missing Core marker: ${marker}`).toContain(marker)
    }
  })

  it('preserves the full UsageProgressBar information contract', () => {
    const source = read('portal/frontend/src', 'components/account/UsageProgressBar.vue')
    for (const marker of ['windowStats', 'formatRequests', 'formatTokens', 'formatAccountCost', 'formatUserCost', 'estimatedTotalCost', 'resetsAt', 'showNowWhenIdle', 'resetPending', 'resetNow']) expect(source).toContain(marker)
  })

  it('passes Core window statistics to every Anthropic usage window', () => {
    const source = read('portal/frontend/src', 'components/account/AccountUsageCell.vue')
    for (const windowName of ['five_hour', 'seven_day', 'seven_day_sonnet', 'seven_day_fable']) {
      expect(source).toContain(`:window-stats="usageInfo.${windowName}.window_stats"`)
    }
  })

  it('preserves Core test-dialog interaction markers', () => {
    const core = read('core/frontend/src', 'components/admin/account/AccountTestModal.vue')
    const portal = read('portal/frontend/src', 'components/account-business/AccountTestDialog.vue')
    for (const marker of ['selectTestModel', 'testMode', 'supportsPromptInput', 'supportsImageUpload', 'connecting', 'loadingModels', 'handleClose']) {
      expect(core).toContain(marker)
      expect(portal, `missing test-dialog marker: ${marker}`).toContain(marker)
    }
  })

  it('keeps the Core OpenAI count-query and reset-credit surface', () => {
    const core = read('core/frontend/src', 'components/account/OpenAIQuotaResetCell.vue')
    const portal = read('portal/frontend/src', 'components/account/OpenAIQuotaResetCell.vue')
    for (const marker of [
      'countButtonTitle',
      'handleQuery',
      'availableResetCount',
      'resetCreditExpirations',
      'reset-credit-expiry-toggle',
      'confirmReset',
      'cache_persisted',
      'resetSuccess',
    ]) {
      expect(core).toContain(marker)
      expect(portal, `missing Core count-query marker: ${marker}`).toContain(marker)
    }
  })
})
