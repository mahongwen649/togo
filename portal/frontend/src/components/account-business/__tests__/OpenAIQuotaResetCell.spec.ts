import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIQuotaResetCell from '@/components/account/OpenAIQuotaResetCell.vue'
import type { Account } from '@/types'
import { refreshOpenAIQuota } from '@/api/admin/accounts'

vi.mock('@/api/admin/accounts', () => ({
  refreshOpenAIQuota: vi.fn(),
  resetOpenAIQuota: vi.fn(),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.count !== undefined ? `${key}${params.count}` : key,
    }),
  }
})

function makeAccount(): Account {
  return {
    id: 1,
    name: 'openai-oauth',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
  }
}

describe('Portal OpenAI count query parity', () => {
  beforeEach(() => vi.mocked(refreshOpenAIQuota).mockReset())

  it('queries Core reset-credit count and expands all expiry details', async () => {
    vi.mocked(refreshOpenAIQuota).mockResolvedValue({
      fetched_at: 1770000000,
      cache_persisted: true,
      rate_limit_reset_credits: {
        available_count: 3,
        credits: [
          { expires_at: '2099-07-05T04:05:06Z' },
          { expires_at: '2099-07-03T04:05:06Z' },
          { expires_at: '2099-07-04T04:05:06Z' },
        ],
      },
    })

    const wrapper = mount(OpenAIQuotaResetCell, { props: { account: makeAccount() } })
    const buttons = wrapper.findAll('button')
    await buttons[0].trigger('click')
    await flushPromises()

    expect(refreshOpenAIQuota).toHaveBeenCalledWith(1)
    expect(wrapper.text()).toContain('admin.accounts.openaiQuotaReset.count3')
    expect(wrapper.text()).toContain('+2')

    await wrapper.get('[data-testid="reset-credit-expiry-toggle"]').trigger('click')
    expect(wrapper.find('[data-testid="reset-credit-expiry-details"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('07/03')
    expect(wrapper.text()).toContain('07/04')
    expect(wrapper.text()).toContain('07/05')
  })
})
