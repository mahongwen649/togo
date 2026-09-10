import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail, transferAffiliateQuota, refreshUser, showSuccess } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
  transferAffiliateQuota: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/user', () => ({ default: { getAffiliateDetail, transferAffiliateQuota } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const detail = {
  user_id: 1,
  aff_code: 'AFF123',
  inviter_id: null,
  aff_count: 2,
  aff_quota: 15,
  aff_frozen_quota: 3,
  aff_history_quota: 20,
  effective_rebate_rate_percent: 15,
  invitees: []
}

describe('AffiliateView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getAffiliateDetail.mockResolvedValue(detail)
    transferAffiliateQuota.mockResolvedValue({ transferred_quota: 15, balance: 25 })
    refreshUser.mockResolvedValue(undefined)
    copyToClipboard.mockResolvedValue(true)
  })

  it('renders the affiliate summary and functional invite link', async () => {
    const wrapper = mount(AffiliateView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('AFF123')
    expect(wrapper.text()).toContain('15%')
    expect(wrapper.text()).toContain('/register?aff=AFF123')
    expect(wrapper.text()).toContain('affiliate.stats.frozenQuota')
  })

  it('transfers available affiliate quota and refreshes the user', async () => {
    const wrapper = mount(AffiliateView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } }
    })
    await flushPromises()

    const transferButton = wrapper.findAll('button').find((button) => button.text().includes('affiliate.transfer.button'))
    expect(transferButton).toBeDefined()
    await transferButton!.trigger('click')
    await flushPromises()

    expect(transferAffiliateQuota).toHaveBeenCalledOnce()
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(showSuccess).toHaveBeenCalled()
  })
})
