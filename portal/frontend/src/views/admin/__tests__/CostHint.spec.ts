import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import CostHint from '../CostHint.vue'

describe('CostHint', () => {
  it('uses Core-compatible semantic colors for each currency value', () => {
    const wrapper = mount(CostHint, {
      props: {
        actual: 140.36,
        account: 294.2857,
        standard: 1950,
      },
    })

    const values = wrapper.findAll('p > span')
    expect(values[0].text()).toBe('¥140.36')
    expect(values[0].classes()).toContain('text-green-600')
    expect(values[2].text()).toBe('¥2.06K')
    expect(values[2].classes()).toContain('text-orange-500')
    expect(values[4].text()).toBe('$1.95K')
    expect(values[4].classes()).toContain('text-gray-400')
  })
})
