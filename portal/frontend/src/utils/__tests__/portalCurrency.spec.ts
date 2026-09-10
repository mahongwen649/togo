import { describe, expect, it } from 'vitest'

import {
  PORTAL_USD_CNY_RATE,
  customerAmountCny,
  customerEffectiveMultiplier,
  upstreamCostCny,
} from '../portalCurrency'

describe('portal currency conversion', () => {
  it('keeps customer balance and actual charges at the 1:1 CNY value', () => {
    expect(customerAmountCny(12.5)).toBe(12.5)
  })

  it('converts upstream USD costs using the fixed operating rate', () => {
    expect(PORTAL_USD_CNY_RATE).toBe(7)
    expect(upstreamCostCny(12.5)).toBe(87.5)
  })

  it('converts the configured rate to an effective CNY multiplier', () => {
    expect(customerEffectiveMultiplier(0.18)).toBeCloseTo(0.18 / 7)
  })

  it('normalizes missing and invalid values', () => {
    expect(customerAmountCny(undefined)).toBe(0)
    expect(upstreamCostCny(Number.NaN)).toBe(0)
  })
})
