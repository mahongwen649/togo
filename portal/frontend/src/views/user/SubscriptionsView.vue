<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="page-header flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 class="page-title">{{ t('userSubscriptions.title') }}</h1>
          <p class="page-description">{{ t('userSubscriptions.description') }}</p>
        </div>
        <RouterLink to="/recharge" class="btn btn-secondary">
          {{ t('payment.portal.recharge') }}
        </RouterLink>
      </div>

      <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>

      <div v-else-if="subscriptions.length === 0" class="card p-10 text-center">
        <Icon name="creditCard" size="xl" class="mx-auto text-gray-400" />
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <div v-else class="grid gap-5 lg:grid-cols-2">
        <article
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="card overflow-hidden border-l-4"
          :class="platformBorderClass(subscription.group?.platform || '')"
        >
          <div class="flex items-start justify-between gap-4 border-b border-gray-100 p-5 dark:border-dark-700">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
                  {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                </h2>
                <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(subscription.group?.platform || '')]">
                  {{ platformLabel(subscription.group?.platform || '') }}
                </span>
              </div>
              <p v-if="subscription.group?.description" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ subscription.group.description }}
              </p>
              <div class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                <span>{{ t('payment.planCard.rate') }}: ×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                <span v-if="hasPeakRate(subscription.group)">
                  {{ t('payment.planCard.peakRate') }}: {{ formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)) }}
                </span>
              </div>
            </div>
            <span :class="['shrink-0 rounded-full px-2 py-1 text-xs font-medium', statusClass(subscription.status)]">
              {{ t(`userSubscriptions.status.${subscription.status}`) }}
            </span>
          </div>

          <div class="space-y-4 p-5">
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{ t('userSubscriptions.expires') }}</span>
              <span :class="subscription.expires_at ? expirationClass(subscription.expires_at) : 'text-gray-700 dark:text-gray-300'">
                {{ subscription.expires_at ? formatExpiration(subscription.expires_at) : t('userSubscriptions.noExpiration') }}
              </span>
            </div>

            <UsageBar v-if="subscription.group?.daily_limit_usd" :label="t('userSubscriptions.daily')" :used="subscription.daily_usage_usd" :limit="subscription.group.daily_limit_usd" />
            <UsageBar v-if="subscription.group?.weekly_limit_usd" :label="t('userSubscriptions.weekly')" :used="subscription.weekly_usage_usd" :limit="subscription.group.weekly_limit_usd" />
            <UsageBar v-if="subscription.group?.monthly_limit_usd" :label="t('userSubscriptions.monthly')" :used="subscription.monthly_usage_usd" :limit="subscription.group.monthly_limit_usd" />

            <div
              v-if="!subscription.group?.daily_limit_usd && !subscription.group?.weekly_limit_usd && !subscription.group?.monthly_limit_usd"
              class="rounded-lg bg-emerald-50 px-4 py-4 text-center dark:bg-emerald-900/20"
            >
              <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">{{ t('userSubscriptions.unlimited') }}</p>
              <p class="mt-1 text-xs text-emerald-600/70 dark:text-emerald-400/70">{{ t('userSubscriptions.unlimitedDesc') }}</p>
            </div>
          </div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { defineComponent, h, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBadgeClass, platformBorderClass, platformLabel } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()
const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

const UsageBar = defineComponent({
  props: {
    label: { type: String, required: true },
    used: { type: Number, default: 0 },
    limit: { type: Number, required: true },
  },
  setup(props) {
    return () => {
      const percentage = Math.min((props.used / props.limit) * 100, 100)
      const barClass = percentage >= 90 ? 'bg-red-500' : percentage >= 70 ? 'bg-orange-500' : 'bg-emerald-500'
      return h('div', { class: 'space-y-2' }, [
        h('div', { class: 'flex items-center justify-between text-sm' }, [
          h('span', { class: 'font-medium text-gray-700 dark:text-gray-300' }, props.label),
          h('span', { class: 'text-gray-500 dark:text-dark-400' }, `$${props.used.toFixed(2)} / $${props.limit.toFixed(2)}`),
        ]),
        h('div', { class: 'h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600' }, [
          h('div', { class: `h-full rounded-full ${barClass}`, style: { width: `${percentage}%` } }),
        ]),
      ])
    }
  },
})

async function loadSubscriptions() {
  try {
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function statusClass(status: UserSubscription['status']): string {
  if (status === 'active') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
  if (status === 'expired') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
  return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
}

function formatExpiration(value: string): string {
  const date = new Date(value)
  const timestamp = date.getTime()
  if (!Number.isFinite(timestamp)) return ''
  const diff = timestamp - Date.now()
  if (diff <= 0) return t('userSubscriptions.status.expired')
  const days = Math.ceil(diff / (24 * 60 * 60 * 1000))
  const formatted = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
  return `${t('userSubscriptions.daysRemaining', { days })} (${formatted})`
}

function expirationClass(value: string): string {
  const diff = new Date(value).getTime() - Date.now()
  const days = Math.ceil(diff / (24 * 60 * 60 * 1000))
  if (diff <= 0) return 'font-medium text-red-600 dark:text-red-400'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

onMounted(loadSubscriptions)
</script>
