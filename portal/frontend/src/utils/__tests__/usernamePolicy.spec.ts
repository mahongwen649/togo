import { describe, expect, it } from 'vitest'
import { normalizeNewUsername, validateNewUsername } from '../usernamePolicy'

describe('usernamePolicy', () => {
  const validCases = [
    ['中文', '中文'],
    [' User_01-test ', 'User_01-test'],
    ['ab', 'ab'],
    ['123456789012345678901234567890', '123456789012345678901234567890'],
  ] as const

  it.each(validCases)('accepts %s', (input, normalized) => {
    expect(validateNewUsername(input)).toBe(true)
    expect(normalizeNewUsername(input)).toBe(normalized)
  })

  it.each([
    'a',
    '1234567890123456789012345678901',
    'two words',
    'user@example',
    '历史—用户',
    'user!',
    '用户😀',
  ])('rejects %s', (input) => {
    expect(validateNewUsername(input)).toBe(false)
  })
})
