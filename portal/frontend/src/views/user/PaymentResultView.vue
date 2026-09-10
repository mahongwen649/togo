<template>
  <AppLayout>
    <div class="mx-auto max-w-[620px]">
      <div class="card p-8 text-center">
        <div :class="['mx-auto flex h-14 w-14 items-center justify-center rounded-full text-2xl font-bold', tone]">{{ symbol }}</div>
        <h1 class="mt-5 text-xl font-semibold text-gray-900 dark:text-white">{{ title }}</h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ description }}</p>
        <p v-if="outTradeNo" class="mt-5 break-all font-mono text-xs text-gray-400">{{ t('payment.portal.orderNo') }}: {{ outTradeNo }}</p>
        <div class="mt-7 flex flex-wrap justify-center gap-3"><RouterLink to="/recharge" class="btn btn-secondary">{{ t('payment.portal.backRecharge') }}</RouterLink><RouterLink to="/dashboard" class="btn btn-primary">{{ t('payment.portal.backDashboard') }}</RouterLink></div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { verifyPaymentOrder } from '@/api/payment'
import type { PaymentOrderStatus } from '@/types/payment'
import { getSingleRouteQueryValue } from '@/utils/routeQuery'

const route = useRoute(); const { t } = useI18n()
const outTradeNo = computed(() => getSingleRouteQueryValue(route.query.out_trade_no))
const status = ref<PaymentOrderStatus | 'CHECKING'>('CHECKING')
let timer: number | undefined; let attempts = 0
const completed = computed(() => status.value === 'COMPLETED')
const terminalFailure = computed(() => ['FAILED', 'CANCELLED', 'EXPIRED'].includes(status.value))
const title = computed(() => completed.value ? t('payment.portal.success') : terminalFailure.value ? t('payment.portal.failed') : status.value === 'CHECKING' ? t('payment.portal.checking') : t('payment.portal.pending'))
const description = computed(() => completed.value ? t('payment.portal.successDesc') : terminalFailure.value ? t('payment.portal.failedDesc') : t('payment.portal.pendingDesc'))
const symbol = computed(() => completed.value ? '✓' : terminalFailure.value ? '!' : '…')
const tone = computed(() => completed.value ? 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30' : terminalFailure.value ? 'bg-red-100 text-red-600 dark:bg-red-900/30' : 'bg-primary-100 text-primary-600 dark:bg-primary-900/30')

async function check() {
  if (!outTradeNo.value) { status.value = 'FAILED'; return }
  try { const order = await verifyPaymentOrder(outTradeNo.value); status.value = order.status } catch { status.value = attempts > 0 ? status.value : 'PENDING' }
  attempts += 1
  if (!completed.value && !terminalFailure.value && attempts < 40) timer = window.setTimeout(check, 3000)
}
onMounted(check)
onBeforeUnmount(() => { if (timer) window.clearTimeout(timer) })
</script>
