<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="page-header flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 class="page-title">{{ t('payment.portal.ordersTitle') }}</h1>
          <p class="page-description">{{ t('payment.portal.description') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('payment.portal.refresh') }}</button>
          <RouterLink to="/recharge" class="btn btn-primary">{{ t('payment.portal.recharge') }}</RouterLink>
        </div>
      </div>
      <div class="card overflow-hidden">
        <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>
        <div v-else-if="error" class="p-6 text-sm text-red-500">{{ error }}</div>
        <div v-else-if="!page.items.length" class="empty-state"><p class="empty-state-title">{{ t('payment.portal.empty') }}</p></div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400"><tr><th class="px-5 py-3 font-medium">{{ t('payment.portal.orderNo') }}</th><th class="px-5 py-3 font-medium">{{ t('payment.portal.paidAmount') }}</th><th class="px-5 py-3 font-medium">{{ t('payment.portal.creditedAmount') }}</th><th class="px-5 py-3 font-medium">{{ t('payment.portal.orderStatus') }}</th><th class="px-5 py-3 font-medium">{{ t('payment.portal.createdAt') }}</th><th class="px-5 py-3 font-medium">{{ t('payment.portal.action') }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <template v-for="order in page.items" :key="order.id">
                <tr class="text-gray-700 dark:text-gray-200"><td class="max-w-[230px] truncate px-5 py-4 font-mono text-xs" :title="order.out_trade_no">{{ order.out_trade_no }}</td><td class="px-5 py-4 font-medium">¥{{ money(order.pay_amount) }}</td><td class="px-5 py-4">¥{{ money(order.amount) }}</td><td class="px-5 py-4"><span :class="['inline-flex rounded-md px-2 py-1 text-xs font-medium', statusClass(order.status)]">{{ statusLabel(order.status) }}</span></td><td class="whitespace-nowrap px-5 py-4 text-gray-500 dark:text-dark-400">{{ dateTime(order.created_at) }}</td><td class="px-5 py-4"><button v-if="order.status === 'PENDING'" class="text-xs font-medium text-red-600 hover:text-red-700" @click="cancel(order.id)">{{ t('payment.portal.cancel') }}</button><span v-else class="text-gray-300">-</span></td></tr>
                <tr v-if="Number(order.campaign_bonus) > 0" class="bg-primary-50/40 text-gray-700 dark:bg-primary-900/10 dark:text-gray-200">
                  <td class="px-5 py-3.5"><span class="inline-flex items-center gap-2 font-medium text-primary-700 dark:text-primary-300"><Icon name="gift" size="sm" />{{ t('payment.portal.campaignBonusRow') }}</span></td>
                  <td class="px-5 py-3.5 text-gray-400 dark:text-dark-500">-</td>
                  <td class="px-5 py-3.5 font-semibold text-primary-700 dark:text-primary-300">+¥{{ money(order.campaign_bonus || 0) }}</td>
                  <td class="px-5 py-3.5"><span :class="['inline-flex rounded-md px-2 py-1 text-xs font-medium', campaignStatusClass(order.campaign_bonus_status)]">{{ campaignStatusLabel(order.campaign_bonus_status) }}</span></td>
                  <td class="whitespace-nowrap px-5 py-3.5 text-gray-500 dark:text-dark-400">{{ dateTime(order.completed_at || order.created_at) }}</td>
                  <td class="px-5 py-3.5 text-gray-300">-</td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
      <div v-if="page.pages > 1" class="flex items-center justify-center gap-3"><button class="btn btn-secondary btn-sm" :disabled="page.page <= 1" @click="go(page.page - 1)">‹</button><span class="text-sm text-gray-500">{{ page.page }} / {{ page.pages }}</span><button class="btn btn-secondary btn-sm" :disabled="page.page >= page.pages" @click="go(page.page + 1)">›</button></div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { cancelPaymentOrder, getMyPaymentOrders } from '@/api/payment'
import type { PaymentOrderPage, PaymentOrderStatus } from '@/types/payment'

const { t, locale } = useI18n(); const loading = ref(false); const error = ref('')
const page = reactive<PaymentOrderPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
const money = (value: number) => Number(value || 0).toFixed(2)
const dateTime = (value: string) => new Intl.DateTimeFormat(locale.value.startsWith('zh') ? 'zh-CN' : 'en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
const statusLabel = (status: PaymentOrderStatus) => t(`payment.portal.statuses.${status}`)
const statusClass = (status: PaymentOrderStatus) => status === 'COMPLETED' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : ['FAILED', 'EXPIRED', 'CANCELLED'].includes(status) ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
const campaignStatusLabel = (status?: string) => t(`payment.portal.campaignBonusStatuses.${status || 'PENDING'}`)
const campaignStatusClass = (status?: string) => status === 'APPLIED' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : status === 'FAILED' ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300' : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
async function load() { loading.value = true; error.value = ''; try { Object.assign(page, await getMyPaymentOrders(page.page, page.page_size)) } catch (e: any) { error.value = e?.message || t('payment.portal.loadFailed') } finally { loading.value = false } }
async function go(value: number) { page.page = value; await load() }
async function cancel(id: number) { try { await cancelPaymentOrder(id); await load() } catch (e: any) { error.value = e?.message || t('payment.portal.loadFailed') } }
onMounted(load)
</script>
