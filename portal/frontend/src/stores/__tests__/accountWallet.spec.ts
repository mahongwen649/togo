import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAccountWalletStore } from '../accountWallet'

const api = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: api }))

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('account wallet store', () => {
  it('deduplicates concurrent loads and caches the current user balance', async () => {
    api.get.mockResolvedValue({ data: { balance: 99 } })
    const store = useAccountWalletStore()

    const [first, second] = await Promise.all([store.load(1), store.load(1)])

    expect(first).toBe(99)
    expect(second).toBe(99)
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(store.balance).toBe(99)

    await store.load(1)
    expect(api.get).toHaveBeenCalledTimes(1)
  })

  it('clears the old balance before loading a different user', async () => {
    api.get.mockResolvedValueOnce({ data: { balance: 99 } }).mockResolvedValueOnce({ data: { balance: 20 } })
    const store = useAccountWalletStore()

    await store.load(1)
    await store.load(2)

    expect(api.get).toHaveBeenCalledTimes(2)
    expect(store.balance).toBe(20)
  })

  it('keeps a failed account wallet distinct from relay balance', async () => {
    api.get.mockRejectedValue(new Error('unavailable'))
    const store = useAccountWalletStore()

    await store.load(1)

    expect(store.balance).toBeNull()
    expect(store.error).toBe('账号套餐余额加载失败')
  })
})
