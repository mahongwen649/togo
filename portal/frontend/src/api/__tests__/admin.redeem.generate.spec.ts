import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({ apiClient: { post } }))

import { generate } from '@/api/admin/redeem'

describe('admin redeem generate API', () => {
  beforeEach(() => {
    post.mockReset()
    vi.spyOn(globalThis.crypto, 'randomUUID')
      .mockReturnValueOnce('11111111-1111-4111-8111-111111111111')
      .mockReturnValueOnce('22222222-2222-4222-8222-222222222222')
  })

  it('reuses the same key after an ambiguous failure and clears it after success', async () => {
    post.mockRejectedValueOnce(new Error('network timeout'))
    await expect(generate(2, 'balance', 1)).rejects.toThrow('network timeout')

    post.mockResolvedValueOnce({ data: [] })
    await generate(2, 'balance', 1)
    post.mockResolvedValueOnce({ data: [] })
    await generate(2, 'balance', 1)

    expect(post.mock.calls[1][2].headers).toEqual(post.mock.calls[0][2].headers)
    expect(post.mock.calls[2][2].headers['Idempotency-Key']).toBe(
      'redeem-generate-22222222-2222-4222-8222-222222222222'
    )
  })

  it('clears the key after a deterministic validation error', async () => {
    post.mockRejectedValueOnce({ status: 400 })
    await expect(generate(1, 'balance', 1)).rejects.toEqual({ status: 400 })
    post.mockResolvedValueOnce({ data: [] })
    await generate(1, 'balance', 1)

    expect(post.mock.calls[1][2].headers['Idempotency-Key']).toBe(
      'redeem-generate-22222222-2222-4222-8222-222222222222'
    )
  })
})
