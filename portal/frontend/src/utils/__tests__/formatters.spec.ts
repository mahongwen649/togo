import { describe, expect, it } from 'vitest'

import { formatMultiplier } from '@/utils/formatters'

describe('formatMultiplier', () => {
  it('preserves significant precision for fractional billing multipliers', () => {
    expect(formatMultiplier(0.075)).toBe('0.075')
    expect(formatMultiplier(0.015)).toBe('0.015')
  })

  it('keeps common multipliers compact', () => {
    expect(formatMultiplier(1)).toBe('1')
    expect(formatMultiplier(1.2)).toBe('1.2')
    expect(formatMultiplier(1.25)).toBe('1.25')
    expect(formatMultiplier(0.01)).toBe('0.01')
  })

  it('keeps tiny multipliers readable', () => {
    expect(formatMultiplier(0.001)).toBe('0.001')
    expect(formatMultiplier(0.0001)).toBe('0.0001')
  })
})
