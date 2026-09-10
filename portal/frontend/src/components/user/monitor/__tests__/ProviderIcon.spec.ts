import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ProviderIcon from '../ProviderIcon.vue'

describe('ProviderIcon', () => {
  it.each(['openai', 'gemini', 'anthropic'])(
    'renders a branded %s icon instead of fallback text',
    (provider) => {
      const wrapper = mount(ProviderIcon, { props: { provider, size: 24 } })
      const icon = wrapper.get(`[data-provider-icon="${provider}"]`)
      expect(wrapper.text()).toBe('')
      expect(icon.attributes('viewBox')).toBe('0 0 24 24')
    }
  )

  it('renders Gemini with the commercial gradient sparkle icon', () => {
    const wrapper = mount(ProviderIcon, { props: { provider: 'gemini', size: 24 } })
    const paths = wrapper.findAll('path')
    expect(paths).toHaveLength(4)
    expect(paths[0].attributes('fill')).toBe('#3186FF')
    expect(paths.slice(1).every((path) => path.attributes('fill')?.startsWith('url(#gemini-gradient-'))).toBe(true)
    expect(wrapper.findAll('linearGradient')).toHaveLength(3)
  })

  it('normalizes provider names and falls back for unknown providers', () => {
    expect(mount(ProviderIcon, { props: { provider: 'OpenAI' } }).find('[data-provider-icon="openai"]').exists()).toBe(true)
    expect(mount(ProviderIcon, { props: { provider: 'unknown-provider' } }).text()).toBe('U')
  })
})


