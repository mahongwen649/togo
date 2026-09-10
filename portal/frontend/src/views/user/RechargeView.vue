<template>
  <AppLayout>
    <div class="mx-auto max-w-[950px] space-y-6">
      <div class="page-header flex flex-wrap items-end justify-between gap-3">
        <div><h1 class="page-title">{{ t('payment.portal.recharge') }}</h1><p class="page-description">{{ t('payment.portal.description') }}</p></div>
        <RouterLink to="/orders" class="btn btn-secondary">{{ t('payment.portal.viewOrders') }}</RouterLink>
      </div>

      <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>
      <div v-else-if="loadError || !checkout" class="card p-6 text-sm text-red-600 dark:text-red-400">{{ loadError || t('payment.portal.loadFailed') }}</div>
      <div v-else-if="checkout.balance_disabled" class="card p-8 text-center text-gray-600 dark:text-dark-300">{{ t('payment.portal.disabled') }}</div>
      <div v-else class="grid gap-6 lg:grid-cols-[1fr_320px]">
        <section class="card p-6">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.portal.amount') }}</h2>
          <div class="mt-4 grid grid-cols-3 gap-3 sm:grid-cols-5">
            <button
              v-for="preset in presets"
              :key="preset.amount"
              type="button"
              :aria-label="preset.bonus ? t('payment.portal.campaignAmountLabel', { amount: preset.amount, bonus: preset.bonus, hot: preset.hot ? t('payment.portal.hot') : '' }) : `¥${preset.amount}`"
              :class="[
                'relative flex min-h-[84px] items-end justify-center overflow-hidden rounded-lg border px-2 pb-3 text-sm transition-colors',
                amount === preset.amount
                  ? 'border-primary-500 bg-primary-50 text-primary-700 shadow-sm dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 bg-white text-gray-700 hover:border-primary-300 hover:bg-primary-50/40 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-primary-700'
              ]"
              @click="amount = preset.amount"
            >
              <span
                v-if="preset.bonus"
                :class="[
                  'campaign-badge absolute right-0 top-0 rounded-bl-md px-2 py-1 text-[10px] font-bold leading-none',
                  preset.hot ? 'campaign-badge--hot' : 'campaign-badge--bonus'
                ]"
              >
                <span class="relative z-10 inline-flex items-center gap-0.5">
                  <Icon v-if="preset.hot" name="fire" size="xs" class="campaign-flame" :stroke-width="2.25" />
                  <span v-if="preset.hot">{{ t('payment.portal.hot') }} · </span>{{ t('payment.portal.bonusBadge', { amount: preset.bonus }) }}
                </span>
              </span>
              <span class="text-base font-bold">¥{{ preset.amount }}</span>
            </button>
          </div>
          <label class="mt-5 block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('payment.portal.customAmount') }}</label>
          <div class="relative mt-2"><span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">¥</span><input v-model.number="amount" type="number" :min="minAmount" :max="maxAmount || undefined" step="1" class="input w-full pl-8" /></div>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('payment.portal.range', { min: formatMoney(minAmount), max: maxAmount ? formatMoney(maxAmount) : '∞' }) }}</p>

          <h2 class="mt-7 text-base font-semibold text-gray-900 dark:text-white">{{ t('payment.portal.alipay') }}</h2>
          <div :class="['mt-3 flex items-center gap-3 rounded-lg border p-4', alipayAvailable ? 'border-primary-500 bg-primary-50/60 dark:bg-primary-900/10' : 'border-gray-200 opacity-60 dark:border-dark-600']">
            <img :src="alipayIcon" alt="Alipay" class="h-9 w-9" /><div class="min-w-0 flex-1"><p class="font-medium text-gray-900 dark:text-white">{{ t('payment.portal.alipay') }}</p><p v-if="!alipayAvailable" class="text-xs text-red-500">{{ t('payment.portal.unavailable') }}</p></div>
            <span v-if="alipayAvailable" class="h-4 w-4 rounded-full border-[5px] border-primary-500 bg-white"></span>
          </div>
        </section>

        <aside class="card h-fit p-6">
          <dl class="space-y-4 text-sm">
            <div class="flex justify-between"><dt class="text-gray-500 dark:text-dark-400">{{ t('payment.portal.rate') }}</dt><dd class="font-medium">¥1 = ¥{{ multiplierText }}</dd></div>
            <div class="flex justify-between"><dt class="text-gray-500 dark:text-dark-400">{{ t('payment.portal.fee') }}</dt><dd class="font-medium">¥{{ formatMoney(fee) }}</dd></div>
            <div class="border-t border-gray-100 pt-4 dark:border-dark-700"><dt class="text-gray-500 dark:text-dark-400">{{ t('payment.portal.receive') }}</dt><dd class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ formatMoney(receiveAmount) }}</dd></div>
          </dl>
          <button type="button" class="btn btn-primary mt-6 w-full" :disabled="submitting || !validAmount || !alipayAvailable" @click="startPayment">{{ submitting ? t('payment.portal.processing') : t('payment.portal.payNow') }}</button>
          <p v-if="notice" :class="['mt-3 text-center text-xs', noticeError ? 'text-red-500' : 'text-emerald-600 dark:text-emerald-400']">{{ notice }}</p>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import alipayIcon from '@/assets/icons/alipay.svg'
import { createPaymentOrder, getPaymentCheckoutInfo, getPaymentOrder } from '@/api/payment'
import { useAuthStore } from '@/stores/auth'
import type { PaymentCheckoutInfo } from '@/types/payment'

const { t } = useI18n()
const authStore = useAuthStore()
const presets = [
  { amount: 10, bonus: 0, hot: false },
  { amount: 20, bonus: 0, hot: false },
  { amount: 50, bonus: 5, hot: false },
  { amount: 100, bonus: 12, hot: true },
  { amount: 200, bonus: 25, hot: false }
]
const amount = ref(10)
const checkout = ref<PaymentCheckoutInfo | null>(null)
const loading = ref(true)
const submitting = ref(false)
const loadError = ref('')
const notice = ref('')
const noticeError = ref(false)
let pollTimer: number | undefined

const alipayMethod = computed(() => checkout.value?.methods.alipay)
const alipayAvailable = computed(() => Boolean(alipayMethod.value))
const minAmount = computed(() => alipayMethod.value?.single_min || checkout.value?.global_min || 1)
const maxAmount = computed(() => alipayMethod.value?.single_max || checkout.value?.global_max || 0)
const multiplier = computed(() => checkout.value?.balance_recharge_multiplier || 1)
const multiplierText = computed(() => Number(multiplier.value).toFixed(2).replace(/\.00$/, ''))
const feeRate = computed(() => alipayMethod.value?.fee_rate ?? checkout.value?.recharge_fee_rate ?? 0)
const fee = computed(() => Math.max(0, Number(amount.value) || 0) * feeRate.value)
const campaignBonus = computed(() => presets.find((preset) => preset.amount === Number(amount.value))?.bonus || 0)
const receiveAmount = computed(() => Math.max(0, Number(amount.value) || 0) * multiplier.value + campaignBonus.value)
const validAmount = computed(() => Number(amount.value) >= minAmount.value && (!maxAmount.value || Number(amount.value) <= maxAmount.value))
const formatMoney = (value: number) => Number(value || 0).toFixed(2)

async function startPayment() {
  if (!validAmount.value) { notice.value = t('payment.portal.invalidAmount'); noticeError.value = true; return }
  const popup = window.open('', '_blank')
  if (!popup) { notice.value = t('payment.portal.popupBlocked'); noticeError.value = true; return }
  submitting.value = true; notice.value = ''; noticeError.value = false
  try {
    const order = await createPaymentOrder(Number(amount.value))
    if (!order.pay_url) throw new Error(t('payment.portal.createFailed'))
    popup.opener = null
    popup.location.href = order.pay_url
    notice.value = t('payment.portal.opened')
    pollTimer = window.setInterval(async () => {
      try {
        const current = await getPaymentOrder(order.order_id)
        if (current.status === 'COMPLETED') {
          window.clearInterval(pollTimer)
          await authStore.refreshUser().catch(() => undefined)
          notice.value = t('payment.portal.success')
        }
      } catch { /* keep polling until the user leaves */ }
    }, 3000)
  } catch (error: any) {
    popup.close(); notice.value = error?.message || t('payment.portal.createFailed'); noticeError.value = true
  } finally { submitting.value = false }
}

onMounted(async () => { try { checkout.value = await getPaymentCheckoutInfo() } catch (error: any) { loadError.value = error?.message || t('payment.portal.loadFailed') } finally { loading.value = false } })
onBeforeUnmount(() => { if (pollTimer) window.clearInterval(pollTimer) })
</script>

<style scoped>
.campaign-badge {
  isolation: isolate;
  overflow: hidden;
}

.campaign-badge::after {
  position: absolute;
  inset: 0;
  z-index: 0;
  background: linear-gradient(105deg, transparent 25%, rgb(255 255 255 / 45%) 48%, transparent 72%);
  content: '';
  transform: translateX(-130%);
  animation: campaign-shimmer 3s ease-in-out infinite;
}

.campaign-badge--bonus {
  border: 1px solid rgb(45 212 191 / 45%);
  background: linear-gradient(135deg, #ccfbf1, #99f6e4);
  color: #115e59;
  box-shadow: 0 2px 8px rgb(20 184 166 / 18%);
}

.campaign-badge--hot {
  background: linear-gradient(135deg, #0d9488, #10b981);
  color: white;
  box-shadow: 0 3px 10px rgb(13 148 136 / 35%);
  animation: campaign-hot-glow 1.6s ease-in-out infinite;
}

.campaign-flame {
  color: #ff725e;
  fill: currentColor;
  flex: none;
  height: 13px;
  width: 13px;
  filter: drop-shadow(0 0 2px rgb(255 87 87 / 72%));
  paint-order: stroke fill;
  stroke: #ffe4df;
  stroke-width: 1.4;
  transform-origin: 50% 85%;
  animation: campaign-flame 0.75s ease-in-out infinite alternate;
}

@keyframes campaign-shimmer {
  0%, 55% { transform: translateX(-130%); }
  85%, 100% { transform: translateX(130%); }
}

@keyframes campaign-hot-glow {
  0%, 100% { box-shadow: 0 3px 9px rgb(13 148 136 / 28%); }
  50% { box-shadow: 0 4px 14px rgb(16 185 129 / 52%); }
}

@keyframes campaign-flame {
  from {
    filter: brightness(0.95) drop-shadow(0 0 2px rgb(255 87 87 / 62%));
    transform: translateY(1px) rotate(-5deg) scale(0.9, 0.94);
  }
  to {
    filter: brightness(1.18) drop-shadow(0 0 4px rgb(255 114 94 / 88%));
    transform: translateY(-1px) rotate(4deg) scale(1.05, 1.12);
  }
}

@media (prefers-reduced-motion: reduce) {
  .campaign-badge::after,
  .campaign-badge--hot,
  .campaign-flame {
    animation: none;
  }
}
</style>
