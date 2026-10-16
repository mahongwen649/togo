import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountPlan(platform: 'openai' | 'antigravity' | 'grok', planType: string) {
  return mount(PlatformTypeBadge, {
    props: { platform, type: 'oauth', planType },
  })
}

describe('PlatformTypeBadge ChatGPT plan tiers', () => {
  it('renders Pro 5x with the same violet style as Core', () => {
    for (const planType of ['prolite', 'PROLITE', 'pro_lite']) {
      const wrapper = mountPlan('openai', planType)
      expect(wrapper.text()).toContain('Pro 5x')
      expect(wrapper.html()).toContain('bg-violet-100')
      expect(wrapper.text()).not.toContain('prolite')
    }
  })

  it('keeps OpenAI plan labels and colors aligned with Core', () => {
    const cases = [
      ['pro', 'Pro 20x', 'bg-violet-100'],
      ['team', 'Business Standard', 'bg-indigo-100'],
      ['self_serve_business_prolite', 'Business Premium', 'bg-indigo-100'],
      ['plus', 'Plus', 'bg-sky-100'],
      ['free', 'Free', 'bg-gray-100'],
    ] as const

    for (const [planType, label, colorClass] of cases) {
      const wrapper = mountPlan('openai', planType)
      expect(wrapper.text()).toContain(label)
      expect(wrapper.html()).toContain(colorClass)
    }
  })

  it('does not apply ChatGPT multiplier labels to other platforms', () => {
    for (const platform of ['antigravity', 'grok'] as const) {
      const wrapper = mountPlan(platform, 'pro')
      expect(wrapper.text()).toContain('Pro')
      expect(wrapper.text()).not.toContain('Pro 20x')
    }
  })
})
