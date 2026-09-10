import { describe, expect, it, vi } from 'vitest'

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
  getLocale: () => 'zh',
}))

import { formatCurrency } from '../format'

describe('formatCurrency', () => {
  it('uses a plain yuan sign for the Portal default currency', () => {
    expect(formatCurrency(168.8)).toBe('¥168.80')
  })

  it('places the negative sign before the yuan sign', () => {
    expect(formatCurrency(-1.25)).toBe('-¥1.25')
  })

  it('preserves explicit USD formatting for official base prices', () => {
    expect(formatCurrency(168.8, 'USD')).toBe('$168.80')
  })
})
