<template>
  <section aria-label="账号套餐在线充值记录">
    <div class="mb-4 flex items-center justify-between gap-3">
      <span class="text-sm text-gray-500">共 {{ result.total }} 笔支付宝充值</span>
      <button type="button" class="btn btn-secondary gap-1" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />刷新</button>
    </div>
    <p v-if="error" role="alert" class="mb-4 text-sm text-red-600">{{ error }}</p>
    <div v-if="loading" class="py-10 text-center text-sm text-gray-500">正在加载充值订单...</div>
    <div v-else-if="!result.items.length" class="py-10 text-center text-sm text-gray-500">暂无在线充值记录</div>
    <div v-else class="overflow-x-auto">
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-gray-200 text-gray-500 dark:border-dark-700">
          <tr><th class="px-4 py-3">订单号</th><th class="px-4 py-3">到账金额</th><th class="px-4 py-3">实付金额</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">创建时间</th><th class="px-4 py-3">到账时间</th><th class="px-4 py-3">操作</th></tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="order in result.items" :key="order.id">
            <td class="max-w-[220px] break-all px-4 py-4 font-mono text-xs">{{ order.out_trade_no }}</td>
            <td class="whitespace-nowrap px-4 py-4 font-medium text-sky-700 dark:text-sky-300">¥{{ Number(order.amount).toFixed(2) }}</td>
            <td class="px-4 py-4">¥{{ Number(order.pay_amount).toFixed(2) }}</td>
            <td class="whitespace-nowrap px-4 py-4">{{ statusLabel(order.status) }}</td>
            <td class="whitespace-nowrap px-4 py-4 text-gray-500">{{ date(order.created_at) }}</td>
            <td class="whitespace-nowrap px-4 py-4 text-gray-500">{{ date(order.completed_at) }}</td>
            <td class="px-4 py-4">
              <div v-if="order.status !== 'COMPLETED'" class="flex flex-wrap gap-2">
                <button v-if="order.status === 'PENDING'" type="button" class="whitespace-nowrap text-sky-700 underline dark:text-sky-300" :disabled="checking !== null" @click="resume(order.id)">继续支付</button>
                <button type="button" class="whitespace-nowrap text-gray-500 underline" :disabled="checking !== null" @click="verify(order.id)">{{ checking === order.id ? '确认中...' : '确认支付状态' }}</button>
              </div>
              <span v-else class="text-gray-400">已到账</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="result.pages > 1" class="mt-5 flex items-center justify-center gap-3">
      <button type="button" class="btn btn-secondary" aria-label="上一页" :disabled="loading || page <= 1" @click="page--; load()"><Icon name="chevronLeft" size="sm" /></button>
      <span class="text-sm">{{ page }} / {{ result.pages }}</span>
      <button type="button" class="btn btn-secondary" aria-label="下一页" :disabled="loading || page >= result.pages" @click="page++; load()"><Icon name="chevronRight" size="sm" /></button>
    </div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { accountPaymentAPI } from '@/api/payment'
import type { PaymentOrder, PaymentOrderPage } from '@/types/payment'
const page = ref(1)
const result = ref<PaymentOrderPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
const error = ref('')
const loading = ref(false)
const checking = ref<number | null>(null)
function statusLabel(status: string) { return status === 'COMPLETED' ? '已到账' : status === 'EXPIRED' ? '已过期' : '待支付' }
function date(value?: string) { return value ? new Date(value).toLocaleString('zh-CN') : '-' }
function updateOrder(order: PaymentOrder) { result.value.items = result.value.items.map(item => item.id === order.id ? order : item) }
async function load() {
  loading.value = true
  error.value = ''
  try { result.value = await accountPaymentAPI.list(page.value) } catch { error.value = '充值记录加载失败，请稍后重试' } finally { loading.value = false }
}
async function verify(id: number) {
  if (checking.value !== null) return
  checking.value = id
  error.value = ''
  try {
    const order = await accountPaymentAPI.get(id)
    updateOrder(order)
    if (order.status !== 'COMPLETED') error.value = '暂未确认到账，请稍后刷新；如已付款，请勿重复支付。'
  } catch { error.value = '确认支付状态失败，请稍后重试' } finally { checking.value = null }
}
async function resume(id: number) {
  if (checking.value !== null) return
  const popup = window.open('', '_blank')
  if (!popup) { error.value = '请允许浏览器打开支付窗口后重试'; return }
  popup.opener = null
  checking.value = id
  error.value = ''
  try {
    const order = await accountPaymentAPI.get(id)
    updateOrder(order)
    if (order.status === 'PENDING' && order.pay_url) popup.location.href = order.pay_url
    else { popup.close(); if (order.status !== 'COMPLETED') error.value = '该订单已不可继续支付，请确认状态后重新充值。' }
  } catch { popup.close(); error.value = '读取支付链接失败，请稍后重试' } finally { checking.value = null }
}
onMounted(load)
</script>
