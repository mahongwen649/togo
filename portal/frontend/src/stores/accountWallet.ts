import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'

function normalizeUserID(userID: number | string | null | undefined): string {
  if (userID === null || userID === undefined) return ''
  return String(userID).trim()
}

export const useAccountWalletStore = defineStore('accountWallet', () => {
  const balance = ref<number | null>(null)
  const loading = ref(false)
  const error = ref('')
  const loadedUserID = ref('')
  let requestSequence = 0
  let pending: { userID: string; promise: Promise<number | null> } | null = null

  function reset(): void {
    requestSequence++
    pending = null
    balance.value = null
    loading.value = false
    error.value = ''
    loadedUserID.value = ''
  }

  async function load(
    userID: number | string | null | undefined,
    force = false,
  ): Promise<number | null> {
    const normalizedUserID = normalizeUserID(userID)
    if (!normalizedUserID) {
      reset()
      return null
    }

    if (loadedUserID.value !== normalizedUserID) {
      requestSequence++
      pending = null
      balance.value = null
      error.value = ''
      loadedUserID.value = normalizedUserID
    }

    if (!force && balance.value !== null) return balance.value
    if (pending?.userID === normalizedUserID) return pending.promise

    const requestID = ++requestSequence
    loading.value = true
    error.value = ''

    const promise = (async () => {
      try {
        const { data } = await apiClient.get<{ balance: number }>('/account-market/wallet')
        const nextBalance = Number(data.balance)
        if (!Number.isFinite(nextBalance)) throw new Error('Invalid account wallet balance')
        if (requestID === requestSequence && loadedUserID.value === normalizedUserID) {
          balance.value = nextBalance
        }
        return nextBalance
      } catch {
        if (requestID === requestSequence && loadedUserID.value === normalizedUserID) {
          error.value = '账号套餐余额加载失败'
        }
        return null
      } finally {
        if (requestID === requestSequence && loadedUserID.value === normalizedUserID) {
          loading.value = false
          pending = null
        }
      }
    })()

    pending = { userID: normalizedUserID, promise }
    return promise
  }

  return { balance, loading, error, load, reset }
})
