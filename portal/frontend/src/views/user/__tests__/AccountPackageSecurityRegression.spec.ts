import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../../../../../')
const read = (path: string) => readFileSync(resolve(root, path), 'utf8')

describe('Portal account package security and usage regressions', () => {
  it('keeps the complete usage-window contract', () => {
    const types = read('portal/frontend/src/types/index.ts')
    for (const field of [
      'five_hour',
      'seven_day',
      'seven_day_sonnet',
      'seven_day_fable',
      'thirty_day',
      'gemini_shared_daily',
      'gemini_pro_daily',
      'gemini_flash_daily',
      'gemini_shared_minute',
      'gemini_pro_minute',
      'gemini_flash_minute',
      'grok_request_quota',
      'grok_token_quota',
      'grok_local_usage',
      'grok_local_usage_24h',
      'grok_local_usage_7d',
      'grok_local_usage_monthly',
      'window_stats',
      'resets_at',
      'reset_at',
    ]) {
      expect(types, `missing usage-window type field: ${field}`).toContain(field)
    }
  })

  it('keeps Core count-query and reset operations wired to Portal API methods', () => {
    const api = read('portal/frontend/src/api/admin/accounts.ts')
    const cell = read('portal/frontend/src/components/account/OpenAIQuotaResetCell.vue')
    expect(api).toContain('/account-market/accounts/${id}/core-action/openai-refresh')
    expect(api).toContain('/account-market/accounts/${id}/core-action/openai-reset')
    expect(cell).toContain('refreshOpenAIQuota')
    expect(cell).toContain('resetOpenAIQuota')
    expect(cell).toContain('handleQuery')
    expect(cell).toContain('confirmReset')
  })

  it('does not expose forbidden account controls but keeps customer scheduling', () => {
    const view = read('portal/frontend/src/views/user/AccountPackageHomeView.vue')
    const api = read('portal/frontend/src/api/admin/accounts.ts')
    expect(view).not.toMatch(/编辑账号|删除账号|重新授权|刷新令牌|refreshCredentials/)
    expect(view).toMatch(/setSchedulable|调度/)
    expect(api).not.toContain('refreshCredentials')
    expect(api).toContain('setSchedulable')
    expect(api).not.toMatch(/deleteAccount|updateAccount|reauth|refreshCredentials/i)
  })

  it('does not expose customer-side account mutation routes in the public route table', () => {
    const routes = read('portal/backend/internal/httpapi/server.go')
    expect(routes).not.toMatch(/account-market\/accounts\/\{accountID\}.*(?:PUT|DELETE)/)
    expect(routes).not.toContain('/api/v1/account-market/accounts/{accountID}/edit')
    expect(routes).not.toContain('/api/v1/account-market/accounts/{accountID}/delete')
    expect(routes).not.toContain('/api/v1/account-market/accounts/{accountID}/reauthorize')
  })
})
