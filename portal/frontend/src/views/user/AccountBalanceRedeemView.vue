<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header">
        <div>
          <h1 class="page-title">卡密兑换</h1>
          <p class="page-description">兑换账号系统专用卡密，余额只用于购买和续订账号套餐。</p>
        </div>
      </header>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>
      <div v-if="message" class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-300">{{ message }}</div>

      <section class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(260px,0.55fr)]">
        <div class="card max-w-2xl p-6">
          <div class="flex items-center gap-3">
            <div class="rounded-lg bg-primary-50 p-3 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300"><Icon name="gift" size="lg" /></div>
            <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">兑换账号套餐余额</h2><p class="mt-1 text-sm text-gray-500 dark:text-dark-400">每张卡密只能兑换一次，兑换后的余额可分次使用。</p></div>
          </div>
          <label class="mt-6 block text-sm font-medium text-gray-700 dark:text-dark-200" for="account-redeem-code">账号系统卡密</label>
          <input id="account-redeem-code" v-model="code" class="input mt-2 w-full font-mono uppercase" placeholder="请输入 ACCT-XXXX-XXXX-XXXX" @keyup.enter="redeem" />
          <button type="button" class="btn btn-primary mt-4 w-full" :disabled="submitting || !code.trim()" @click="redeem">{{ submitting ? '兑换中...' : '立即兑换' }}</button>
        </div>
        <div class="card p-6">
          <div class="text-sm text-gray-500 dark:text-dark-400">账号套餐余额</div>
          <div class="mt-2 text-3xl font-bold text-primary-600 dark:text-primary-400">¥{{ balance.toFixed(2) }}</div>
          <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">与中转站余额完全独立，仅用于账号业务。</p>
          <RouterLink to="/account-business/balance/history" class="btn btn-secondary mt-5 w-full">查看充值记录</RouterLink>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { apiClient } from '@/api/client'
import { accountRedeemAPI } from '@/api/accountBusiness'

const code = ref('')
const balance = ref(0)
const message = ref('')
const error = ref('')
const submitting = ref(false)

async function loadBalance() {
  const { data } = await apiClient.get<{ balance: number }>('/account-market/wallet')
  balance.value = Number(data.balance || 0)
}

async function redeem() {
  if (!code.value.trim() || submitting.value) return
  submitting.value = true
  error.value = ''
  message.value = ''
  try {
    const result = await accountRedeemAPI.redeem(code.value)
    balance.value = result.balance
    message.value = `兑换成功，账号套餐余额增加 ¥${result.amount.toFixed(2)}。`
    code.value = ''
  } catch (err: any) {
    error.value = err?.message || '卡密兑换失败，请检查卡密是否正确或已使用。'
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  try { await loadBalance() } catch (err: any) { error.value = err?.message || '余额加载失败，请稍后重试' }
})
</script>
