import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserDashboardStats from '../UserDashboardStats.vue'

const features = vi.hoisted(() => ({ ACCOUNT_PACKAGES_ENABLED: true }))
vi.mock('@/config/portalFeatures', () => features)

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const stats = {
  total_api_keys: 4,
  active_api_keys: 3,
  today_requests: 10,
  total_requests: 20,
  today_actual_cost: 1,
  today_cost: 1,
  total_actual_cost: 2,
  total_cost: 2,
  today_tokens: 100,
  today_input_tokens: 60,
  today_output_tokens: 40,
  total_tokens: 200,
  total_input_tokens: 120,
  total_output_tokens: 80,
  rpm: 2,
  tpm: 100,
  average_duration_ms: 800,
  by_platform: [],
}

describe('UserDashboardStats wallet balances', () => {
  it('hides the package balance when packages are disabled', () => {
    features.ACCOUNT_PACKAGES_ENABLED = false
    const wrapper = mount(UserDashboardStats, { props: { stats: stats as any, balance: 16.57, accountBalance: 99, isSimple: false } })
    expect(wrapper.find('[data-wallet="account"]').exists()).toBe(false)
    expect(wrapper.get('[data-wallet="relay"]').text()).toContain('¥16.57')
    features.ACCOUNT_PACKAGES_ENABLED = true
  })

  it('shows relay and account-package wallets as separate labeled icon groups', () => {
    const wrapper = mount(UserDashboardStats, {
      props: {
        stats: stats as any,
        balance: 16.57,
        accountBalance: 99,
        isSimple: false,
      },
    })

    const balances = wrapper.get('[data-testid="dashboard-wallet-balances"]')
    const relay = balances.get('[data-wallet="relay"]')
    const account = balances.get('[data-wallet="account"]')

    expect(relay.text()).toContain('中转余额')
    expect(relay.text()).toContain('¥16.57')
    expect(relay.find('svg').exists()).toBe(true)
    expect(account.text()).toContain('套餐余额')
    expect(account.text()).toContain('¥99.00')
    expect(account.find('svg').exists()).toBe(true)
  })

  it('does not substitute relay balance while account balance is unavailable', () => {
    const wrapper = mount(UserDashboardStats, {
      props: {
        stats: stats as any,
        balance: 16.57,
        accountBalance: null,
        isSimple: false,
      },
    })

    expect(wrapper.get('[data-wallet="account"]').text()).toContain('--')
    expect(wrapper.get('[data-wallet="account"]').text()).not.toContain('16.57')
  })
})
