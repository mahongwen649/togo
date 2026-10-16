<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header flex flex-wrap items-end justify-between gap-4">
        <div><h1 class="page-title">账号套餐充值记录</h1><p class="page-description">支付宝充值与卡密兑换记录。</p></div>
        <RouterLink to="/recharge" class="btn btn-primary">余额充值</RouterLink>
      </header>
      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>
      <div role="tablist" aria-label="充值类型" class="flex gap-6 border-b border-gray-200 dark:border-dark-700"><button id="online-payment-tab" role="tab" aria-controls="online-payment-panel" :aria-selected="tab === 'online'" :class="['border-b-2 py-3 text-sm font-medium', tab === 'online' ? 'border-primary-500 text-primary-700' : 'border-transparent text-gray-500']" @click="tab = 'online'">在线充值</button><button id="card-payment-tab" role="tab" aria-controls="card-payment-panel" :aria-selected="tab === 'cards'" :class="['border-b-2 py-3 text-sm font-medium', tab === 'cards' ? 'border-primary-500 text-primary-700' : 'border-transparent text-gray-500']" @click="tab = 'cards'; loadCards()">卡密兑换</button></div>
      <div v-if="tab === 'online'" id="online-payment-panel" role="tabpanel" aria-labelledby="online-payment-tab"><AccountPaymentHistory /></div>
      <section v-else id="card-payment-panel" role="tabpanel" aria-labelledby="card-payment-tab" class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 p-5 dark:border-dark-700"><div><div class="text-sm text-gray-500 dark:text-dark-400">当前账号套餐余额</div><div class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">¥{{ balance.toFixed(2) }}</div></div><span class="text-xs text-gray-500 dark:text-dark-400">共 {{ rows.length }} 张已兑换卡密</span></div>
        <div v-if="loading" class="p-8 text-center text-sm text-gray-500">正在加载记录...</div>
        <div v-else-if="rows.length === 0" class="p-8 text-center text-sm text-gray-500">暂无卡密兑换记录</div>
        <div v-else class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">批次</th><th class="px-5 py-3">卡密</th><th class="px-5 py-3">面值</th><th class="px-5 py-3">剩余可用</th><th class="px-5 py-3">状态</th><th class="px-5 py-3">兑换时间</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in rows" :key="row.id"><td class="px-5 py-4 text-gray-900 dark:text-white">{{ row.batch_name || `批次 #${row.batch_id}` }}</td><td class="px-5 py-4 font-mono text-xs text-gray-500">****-{{ row.code_suffix }}</td><td class="px-5 py-4 font-medium text-emerald-600">¥{{ row.amount.toFixed(2) }}</td><td class="px-5 py-4 text-gray-700 dark:text-dark-200">¥{{ row.remaining_amount.toFixed(2) }}</td><td class="px-5 py-4"><span class="badge" :class="statusClass(row.status)">{{ statusText(row.status) }}</span></td><td class="px-5 py-4 text-gray-500 dark:text-dark-400">{{ formatDate(row.redeemed_at) }}</td></tr></tbody></table></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AccountPaymentHistory from '@/components/account-business/AccountPaymentHistory.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { apiClient } from '@/api/client'
import { accountRedeemAPI, type AccountRedeemHistoryItem } from '@/api/accountBusiness'

const balance = ref(0)
const rows = ref<AccountRedeemHistoryItem[]>([])
const loading = ref(true)
const error = ref('')
const tab = ref<'online' | 'cards'>('online')

function statusText(status: string) { return ({ redeemed: '已兑换', partially_used: '部分使用', used: '已用完', expired: '已过期' }[status] || status) }
function statusClass(status: string) { return status === 'used' ? 'badge-gray' : status === 'partially_used' ? 'badge-warning' : status === 'redeemed' ? 'badge-success' : 'badge-danger' }
function formatDate(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN') : '-' }

async function loadCards() {
  loading.value = true
  error.value = ''
  try {
    const [{ data }, history] = await Promise.all([
      apiClient.get<{ balance: number }>('/account-market/wallet'),
      accountRedeemAPI.history(),
    ])
    balance.value = Number(data.balance || 0)
    rows.value = history.map(row => ({ ...row, amount: Number(row.amount || 0), remaining_amount: Number(row.remaining_amount || 0) }))
  } catch (err: any) {
    error.value = err?.message || '充值记录加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>
