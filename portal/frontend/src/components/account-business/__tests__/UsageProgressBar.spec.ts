import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string, params?: { cost?: number }) => params?.cost !== undefined ? String(params.cost) : key }) }))
import UsageProgressBar from '@/components/account/UsageProgressBar.vue'

describe('Portal Core UsageProgressBar regression', () => {
  const mountBar = (props: Record<string, unknown>) => mount(UsageProgressBar, {
    props: { label: '5h', utilization: 42, color: 'indigo', ...props },
    global: { mocks: { $t: (key: string) => key } },
  })
  it('renders requests, tokens, A/U costs and estimated total cost', () => {
    const wrapper = mountBar({ windowStats: { requests: 167, tokens: 20800000, cost: 23.73, user_cost: 0.85 }, estimatedTotalCost: 2326.82 })
    expect(wrapper.text()).toContain('167 req'); expect(wrapper.text()).toContain('20.8M'); expect(wrapper.text()).toContain('A $23.73'); expect(wrapper.text()).toContain('U $0.85'); expect(wrapper.text()).toContain('2326.82')
  })
  it('renders 7d and reset-now state', () => { const wrapper = mountBar({ label: '7d', utilization: 0, showNowWhenIdle: true }); expect(wrapper.text()).toContain('7d'); expect(wrapper.text()).toContain('usage.resetNow') })
  it('hides cost chips for an empty window', () => { const wrapper = mountBar({ windowStats: { requests: 0, tokens: 0, cost: 0, user_cost: 0 } }); expect(wrapper.text()).not.toContain('A $'); expect(wrapper.text()).not.toContain('U $') })
})
