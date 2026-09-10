import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { post } }))

import { redeem } from '@/api/redeem'

describe('user redeem API', () => {
  beforeEach(() => {
    post.mockReset()
    vi.spyOn(globalThis.crypto, 'randomUUID')
      .mockReturnValueOnce('11111111-1111-4111-8111-111111111111')
      .mockReturnValueOnce('22222222-2222-4222-8222-222222222222')
  })

  it('reuses the key for the same code after an ambiguous failure', async () => {
    post.mockRejectedValueOnce(new Error('timeout'))
    await expect(redeem(' CODE-1 ')).rejects.toThrow('timeout')
    post.mockResolvedValueOnce({ data: { message: 'ok', type: 'balance', value: 1 } })
    await redeem('CODE-1')

    expect(post.mock.calls[0][1]).toEqual({ code: 'CODE-1' })
    expect(post.mock.calls[1][2].headers).toEqual(post.mock.calls[0][2].headers)
  })

  it('clears the key after a deterministic error', async () => {
    post.mockRejectedValueOnce({ status: 404 })
    await expect(redeem('CODE-2')).rejects.toEqual({ status: 404 })
    post.mockResolvedValueOnce({ data: { message: 'ok', type: 'balance', value: 1 } })
    await redeem('CODE-2')

    expect(post.mock.calls[1][2].headers['Idempotency-Key']).toBe(
      'redeem-22222222-2222-4222-8222-222222222222'
    )
  })
})
