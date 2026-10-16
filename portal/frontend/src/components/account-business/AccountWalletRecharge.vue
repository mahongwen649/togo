<template>
  <section aria-labelledby="account-recharge-title" class="min-w-0">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h2 id="account-recharge-title" class="flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white"><Icon name="grid" class="text-sky-600" /><span>账号套餐余额充值</span><span class="account-new-badge" title="新功能">NEW</span></h2>
      <RouterLink to="/account-business/balance/history" class="inline-flex items-center gap-1 text-sm text-gray-500 hover:text-sky-600"><Icon name="clock" size="sm" />充值记录</RouterLink>
    </header>
    <div class="my-6 border-y border-gray-100 py-5 dark:border-dark-700" :aria-busy="balanceLoading">
      <p class="text-sm text-gray-500 dark:text-dark-400">当前账号套餐余额</p>
      <p class="mt-2 break-all text-3xl font-bold tabular-nums text-sky-700 dark:text-sky-300">{{ balance === null ? (balanceLoading ? '加载中...' : '暂不可用') : `¥${balance.toFixed(2)}` }}</p>
      <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">用于购买和续订账号套餐。</p>
      <div v-if="balanceError" role="alert" class="mt-3 flex flex-wrap items-center gap-2 text-sm text-red-600 dark:text-red-400"><span>{{ balanceError }}</span><button type="button" class="inline-flex items-center gap-1 underline" :disabled="balanceLoading" @click="loadBalance"><Icon name="refresh" size="sm" />重试</button></div>
    </div>
    <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>
    <div v-else-if="paymentUnavailable" role="status" class="flex flex-col items-start gap-3 py-6">
      <span class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300">暂未开放</span>
      <p class="text-sm text-gray-600 dark:text-dark-300">账号套餐在线充值暂未开放</p>
      <button type="button" class="inline-flex items-center gap-1 text-sm text-sky-700 hover:underline dark:text-sky-300" @click="loadCheckout"><Icon name="refresh" size="sm" />重新检查</button>
    </div>
    <div v-else-if="loadError || !checkout" role="alert" class="py-6 text-sm text-red-600 dark:text-red-400">{{ loadError || '在线充值暂不可用' }}<button type="button" class="ml-2 underline" @click="loadCheckout">重试</button></div>
    <form v-else @submit.prevent="startPayment">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">充值金额</h3>
      <div class="mt-4 grid grid-cols-3 gap-3 sm:grid-cols-5">
        <button v-for="preset in [10, 20, 50, 100, 200]" :key="preset" type="button" :disabled="submitting || pendingID !== null || preset < min || preset > max" :aria-pressed="amount === preset" :class="['flex min-h-[84px] items-end justify-center rounded-lg border px-2 pb-3 text-base font-bold transition-colors disabled:opacity-50', amount === preset ? 'border-sky-500 bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-300' : 'border-gray-200 text-gray-700 hover:border-sky-300 dark:border-dark-600 dark:text-gray-200']" @click="amount = preset">¥{{ preset }}</button>
      </div>
      <label for="account-recharge-amount" class="mt-5 block text-sm font-medium text-gray-700 dark:text-gray-200">自定义金额</label>
      <div class="relative mt-2"><span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">¥</span><input id="account-recharge-amount" v-model.number="amount" :disabled="submitting || pendingID !== null" type="number" :min="min" :max="max" step="0.01" class="input w-full pl-8" /></div>
      <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">单笔限额 ¥{{ min.toFixed(2) }} - ¥{{ max.toFixed(2) }}</p>
      <h3 class="mt-7 text-base font-semibold text-gray-900 dark:text-white">支付宝</h3>
      <div class="mt-3 flex items-center gap-3 rounded-lg border border-sky-500 bg-sky-50/60 p-4 dark:bg-sky-900/10"><img :src="alipayIcon" alt="Alipay" class="h-9 w-9" /><p class="min-w-0 flex-1 font-medium text-gray-900 dark:text-white">支付宝</p><Icon name="check" class="text-sky-600" /></div>
      <dl class="mt-6 space-y-4 border-t border-gray-100 pt-5 text-sm dark:border-dark-700">
        <div class="flex justify-between gap-3"><dt class="text-gray-500 dark:text-dark-400">到账账户</dt><dd class="font-medium">账号套餐余额</dd></div>
        <div class="flex justify-between gap-3"><dt class="text-gray-500 dark:text-dark-400">充值比例</dt><dd class="font-medium">¥1 = ¥1</dd></div>
        <div class="flex justify-between gap-3"><dt class="text-gray-500 dark:text-dark-400">手续费</dt><dd class="font-medium">¥{{ fee.toFixed(2) }}</dd></div>
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700"><dt class="text-gray-500 dark:text-dark-400">到账余额</dt><dd class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">¥{{ credit.toFixed(2) }}</dd></div>
        <div v-if="fee > 0" class="flex justify-between gap-3"><dt class="text-gray-500 dark:text-dark-400">实付金额</dt><dd>¥{{ (credit + fee).toFixed(2) }}</dd></div>
      </dl>
      <button type="submit" class="btn mt-6 w-full bg-sky-600 text-white hover:bg-sky-700 disabled:cursor-not-allowed disabled:opacity-50" :disabled="submitting || pendingID !== null || !validAmount || !checkout.methods.alipay">{{ submitting ? '创建订单中...' : pendingID !== null ? '等待支付结果' : '立即支付' }}</button>
      <p v-if="error" role="alert" class="mt-3 break-words text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      <p v-if="message" role="status" class="mt-3 break-words text-sm text-emerald-600 dark:text-emerald-400">{{ message }}</p>
      <div v-if="pendingID !== null" class="mt-3 flex flex-wrap gap-3 text-sm"><button type="button" class="inline-flex items-center gap-1 text-sky-700 underline dark:text-sky-300" :disabled="checking" @click="checkPayment"><Icon name="refresh" size="sm" />检查支付结果</button><RouterLink to="/account-business/balance/history" class="text-gray-500 underline">查看充值订单</RouterLink></div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import alipayIcon from '@/assets/icons/alipay.svg'
import { accountPaymentAPI } from '@/api/payment'
import type { PaymentCheckoutInfo } from '@/types/payment'
import { useAuthStore } from '@/stores/auth'
import { useAccountWalletStore } from '@/stores/accountWallet'

const authStore = useAuthStore()
const accountWalletStore = useAccountWalletStore()
const balance = computed(() => accountWalletStore.balance)
const balanceLoading = computed(() => accountWalletStore.loading)
const balanceError = computed(() => accountWalletStore.error)
const checkout = ref<PaymentCheckoutInfo | null>(null)
const loading = ref(true)
const loadError = ref('')
const checkoutUnavailable = ref(false)
const paymentUnavailable = computed(() => checkoutUnavailable.value || checkout.value?.balance_disabled || (checkout.value !== null && !checkout.value.methods?.alipay))
const amount = ref(10)
const submitting = ref(false)
const checking = ref(false)
const pendingID = ref<number | null>(null)
const error = ref('')
const message = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let attempts = 0
const min = computed(() => checkout.value?.methods.alipay?.single_min || 1)
const max = computed(() => checkout.value?.methods.alipay?.single_max || 1000000)
const credit = computed(() => Number.isFinite(Number(amount.value)) ? Math.max(0, Number(amount.value)) : 0)
const fee = computed(() => Math.ceil(credit.value * (checkout.value?.methods.alipay?.fee_rate || 0) - 1e-8) / 100)
const validAmount = computed(() => credit.value >= min.value && credit.value <= max.value && Math.abs(credit.value * 100 - Math.round(credit.value * 100)) < 1e-6)
const errorText = (err: unknown, fallback: string) => err && typeof err === 'object' && 'message' in err ? String(err.message) : fallback

async function loadBalance() {
  await accountWalletStore.load(authStore.user?.id, true)
}

async function loadCheckout() {
  loading.value = true
  loadError.value = ''
  checkoutUnavailable.value = false
  checkout.value = null
  try { checkout.value = await accountPaymentAPI.checkout() }
  catch (err) {
    const failure = err as { status?: number; code?: string } | null
    if (failure?.status === 404 || failure?.status === 503 || failure?.code === 'ACCOUNT_PAYMENT_UNAVAILABLE') {
      checkoutUnavailable.value = true
    } else {
      loadError.value = '账号套餐支付信息加载失败，请稍后重试'
    }
  }
  finally { loading.value = false }
}

async function checkPayment() {
  if (disposed || checking.value || pendingID.value === null) return
  clearTimeout(timer)
  checking.value = true
  try {
    const order = await accountPaymentAPI.get(pendingID.value)
    if (disposed) return
    if (order.status === 'COMPLETED') {
      pendingID.value = null
      message.value = `支付成功，¥${Number(order.amount).toFixed(2)} 已到账账号套餐余额。`
      await loadBalance()
    } else if (['EXPIRED', 'FAILED', 'CANCELLED'].includes(order.status)) {
      pendingID.value = null
      message.value = ''
      error.value = '订单已关闭；如已付款，请在充值记录中确认状态，勿重复支付。'
    }
  } catch { message.value = '支付结果确认中，可稍后在充值记录中查看。' }
  finally {
    checking.value = false
    attempts++
    if (!disposed && pendingID.value !== null && attempts < 100) timer = setTimeout(checkPayment, 3000)
  }
}

async function startPayment() {
  if (!validAmount.value || submitting.value || pendingID.value !== null || !checkout.value?.methods.alipay || checkout.value.balance_disabled) return
  const popup = window.open('', '_blank')
  if (!popup) { error.value = '请允许浏览器打开支付窗口后重试'; return }
  popup.opener = null
  submitting.value = true
  error.value = ''
  message.value = ''
  try {
    const order = await accountPaymentAPI.create(Number(amount.value))
    pendingID.value = order.order_id
    if (!order.pay_url) throw new Error('订单已创建但支付链接未返回，请查看充值记录，勿重复下单。')
    popup.location.href = order.pay_url
    message.value = '支付宝支付窗口已打开，付款后将直接计入账号套餐余额。'
    attempts = 0
    if (!disposed) timer = setTimeout(checkPayment, 3000)
  } catch (err) { popup.close(); error.value = errorText(err, '创建支付订单失败') }
  finally { submitting.value = false }
}

onMounted(() => { void loadBalance(); void loadCheckout() })
onBeforeUnmount(() => { disposed = true; clearTimeout(timer) })
</script>

<style scoped>
.account-new-badge {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  height: 1.125rem;
  padding: 0 0.375rem;
  border-radius: 0.25rem;
  background: rgb(244 63 94);
  box-shadow: 0 2px 8px rgb(244 63 94 / 0.35);
  color: white;
  font-size: 0.625rem;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0;
}

:global(.dark) .account-new-badge {
  background: rgb(251 113 133);
  box-shadow: 0 2px 10px rgb(251 113 133 / 0.3);
  color: rgb(40 10 18);
}
</style>
