import { beforeEach, describe, expect, it, vi } from 'vitest'

const post = vi.fn()

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

describe('totp step-up api', () => {
  beforeEach(() => {
    post.mockReset()
    sessionStorage.clear()
  })

  it('stores the short-lived Portal step-up token for request retries', async () => {
    post.mockResolvedValue({
      data: { verified: true, expires_in: 300, step_up_token: 'signed-step-up' }
    })
    const { stepUp } = await import('@/api/totp')

    await stepUp('123456')

    expect(sessionStorage.getItem('portal_step_up_token')).toBe('signed-step-up')
    expect(Number(sessionStorage.getItem('portal_step_up_expires_at'))).toBeGreaterThan(Date.now())
  })
})
