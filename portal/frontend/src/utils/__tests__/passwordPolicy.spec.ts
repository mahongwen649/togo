import { describe, expect, it } from 'vitest'
import { PASSWORD_MIN_LENGTH, meetsPasswordLength } from '../passwordPolicy'

describe('passwordPolicy', () => {
  it('requires at least eight characters', () => {
    expect(PASSWORD_MIN_LENGTH).toBe(8)
    expect(meetsPasswordLength('1234567')).toBe(false)
    expect(meetsPasswordLength('12345678')).toBe(true)
  })
})
