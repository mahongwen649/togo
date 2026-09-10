import { describe, expect, it } from 'vitest'
import { formatAmount, maskEmail, scaledBudget } from './format'

describe('lottery formatting', () => {
  it('masks account emails', () => {
    expect(maskEmail('Example@TogoAPI.com')).toBe('ex***@togoapi.com')
  })

  it('scales the random budget by the actual winner count', () => {
    expect(scaledBudget({ randomBudget: 100, randomLimit: 10 }, 4)).toBe(40)
    expect(scaledBudget({ randomBudget: 100, randomLimit: 10 }, 20)).toBe(100)
  })

  it('formats invalid amounts safely', () => {
    expect(formatAmount(Number.NaN)).toBe('0.00')
  })
})
