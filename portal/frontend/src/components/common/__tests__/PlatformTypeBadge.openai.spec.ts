import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('OpenAI plan labels match Core', () => {
  it.each([
    ['prolite', 'Pro 5x', 'violet'],
    [' PRO_LITE ', 'Pro 5x', 'violet'],
    ['pro', 'Pro 20x', 'violet'],
    ['chatgpt_pro', 'Pro 20x', 'violet'],
    ['plus', 'Plus', 'sky'],
    ['team', 'Business Standard', 'indigo'],
    ['self_serve_business_prolite', 'Business Premium', 'indigo'],
    ['free', 'Free', 'gray'],
  ])('renders %s as %s', (planType, label, color) => {
    const w = mount(PlatformTypeBadge, { props: { platform: 'openai', type: 'oauth', planType } })
    const badge = w.findAll('span').find(node => node.classes().includes(`bg-${color}-100`))!
    expect(badge.text()).toBe(label)
    expect(badge.classes()).toContain(`dark:bg-${color}-${color === 'gray' ? '700' : '900/30'}`)
    w.unmount()
  })

  it.each(['antigravity', 'grok', 'anthropic', 'gemini'] as const)('does not rename %s plans using OpenAI tiers', platform => {
    const w = mount(PlatformTypeBadge, { props: { platform, type: 'oauth', planType: 'pro' } })
    expect(w.text()).toContain('Pro')
    expect(w.text()).not.toContain('20x')
    w.unmount()
  })

  it('preserves unknown plans and expiration display', () => {
    const w = mount(PlatformTypeBadge, { props: { platform: 'openai', type: 'oauth', planType: 'Future Plan', subscriptionExpiresAt: '2026-10-13T12:00:00Z' } })
    expect(w.text()).toContain('Future Plan')
    expect(w.text()).toContain('2026-10-13')
    w.unmount()
  })
})
