import { describe, expect, it } from 'vitest'

import { getUserAvatarInitial } from '../userDisplay'

describe('getUserAvatarInitial', () => {
  it('prefers email and falls back to username for email-less Portal users', () => {
    expect(getUserAvatarInitial({ email: 'alice@example.com', username: 'ignored' })).toBe('A')
    expect(getUserAvatarInitial({ email: '', username: 'user1' })).toBe('U')
    expect(getUserAvatarInitial({ email: '  ', username: '张三' })).toBe('张')
    expect(getUserAvatarInitial(null)).toBe('U')
  })
})
