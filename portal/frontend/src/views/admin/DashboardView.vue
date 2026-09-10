<template>
  <AppLayout>
    <div class="min-w-0" :class="isMobileH5 ? 'space-y-4' : 'space-y-6'">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else>
        <div class="grid grid-cols-2 lg:grid-cols-4" :class="isMobileH5 ? 'gap-2' : 'gap-4'">
          <DashboardStatCard icon="key" icon-class="bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400" :label="t('admin.dashboard.apiKeys')" :value="formatNumber(stats.total_api_keys)" :hint="`${formatNumber(stats.active_api_keys)} ${t('common.active')}`" hint-class="text-green-600 dark:text-green-400" />
          <DashboardStatCard icon="server" icon-class="bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400" :label="t('admin.dashboard.accounts')" :value="formatNumber(stats.total_accounts)" :hint="`${formatNumber(stats.normal_accounts)} ${t('common.active')}`" hint-class="text-green-600 dark:text-green-400" />
          <DashboardStatCard icon="chart" icon-class="bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400" :label="t('admin.dashboard.todayRequests')" :value="formatNumber(stats.today_requests)" :hint="`${t('common.total')}: ${formatNumber(stats.total_requests)}`" />
          <DashboardStatCard icon="userPlus" icon-class="bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400" :label="t('admin.dashboard.users')" :value="`+${formatNumber(stats.today_new_users)}`" :hint="`${t('common.total')}: ${formatNumber(stats.total_users)}`" value-class="text-emerald-600 dark:text-emerald-400" />
        </div>

        <div class="grid grid-cols-2 lg:grid-cols-4" :class="isMobileH5 ? 'gap-2' : 'gap-4'">
          <DashboardStatCard icon="cube" icon-class="bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" :label="t('admin.dashboard.todayTokens')" :value="formatTokens(stats.today_tokens)">
            <template #hint>
              <div class="space-y-1">
                <CostHint :actual="stats.today_actual_cost" :account="stats.today_account_cost" :standard="stats.today_cost" />
                <p class="text-xs font-medium text-cyan-600 dark:text-cyan-400">{{ t('admin.dashboard.cacheHitRate') }} {{ formatCacheHitRate(stats.today_input_tokens, stats.today_cache_read_tokens) }}</p>
              </div>
            </template>
          </DashboardStatCard>
          <DashboardStatCard icon="database" icon-class="bg-indigo-100 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400" :label="t('admin.dashboard.totalTokens')" :value="formatTokens(stats.total_tokens)">
            <template #hint>
              <div class="space-y-1">
                <CostHint :actual="stats.total_actual_cost" :account="stats.total_account_cost" :standard="stats.total_cost" />
                <p class="text-xs font-medium text-cyan-600 dark:text-cyan-400">{{ t('admin.dashboard.cacheHitRate') }} {{ formatCacheHitRate(stats.total_input_tokens, stats.total_cache_read_tokens) }}</p>
              </div>
            </template>
          </DashboardStatCard>
          <div class="card min-w-0" :class="isMobileH5 ? 'p-2' : 'p-4'">
            <div class="flex items-center" :class="isMobileH5 ? 'gap-2' : 'gap-3'">
              <div class="shrink-0 rounded-lg bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400" :class="isMobileH5 ? 'p-1.5' : 'p-2'">
                <Icon name="bolt" size="md" :stroke-width="2" />
              </div>
              <div class="min-w-0 flex-1">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.performance') }}</p>
                <div class="flex items-baseline gap-2">
                  <p class="font-bold text-gray-900 dark:text-white" :class="isMobileH5 ? 'text-lg' : 'text-xl'">{{ formatTokens(stats.rpm) }}</p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
                </div>
                <div class="flex items-baseline gap-2">
                  <p class="text-sm font-semibold text-violet-600 dark:text-violet-400">{{ formatTokens(stats.tpm) }}</p>
                  <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
                </div>
              </div>
            </div>
          </div>
          <DashboardStatCard icon="clock" icon-class="bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-400" :label="t('admin.dashboard.avgResponse')" :value="formatDuration(stats.average_duration_ms)" :hint="`${formatNumber(stats.active_users)} ${t('admin.dashboard.activeUsers')}`" />
        </div>

        <div :class="isMobileH5 ? 'space-y-4' : 'space-y-6'">
          <ChartRangeToolbar
            v-model:start-date="startDate"
            v-model:end-date="endDate"
            v-model:granularity="granularity"
            :granularity-options="granularityOptions"
            :loading="chartsLoading"
            show-refresh
            @range-change="onDateRangeChange"
            @granularity-change="loadChartData"
            @refresh="loadDashboardStats"
          />

          <div class="grid min-w-0 grid-cols-1 lg:grid-cols-2" :class="isMobileH5 ? 'gap-4' : 'gap-6'">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <div class="min-w-0">
            <button
              v-if="isMobileH5 && !recentUsageExpanded"
              type="button"
              class="mobile-dashboard-section-toggle"
              @click="recentUsageExpanded = true"
            >
              <span>{{ t('admin.dashboard.recentUsage') }} (Top 12)</span>
              <Icon name="chevronDown" size="sm" />
            </button>
            <div v-show="!isMobileH5 || recentUsageExpanded" class="card min-w-0" :class="isMobileH5 ? 'p-3' : 'p-4'">
              <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.recentUsage') }} (Top 12)
              </h3>
              <div :class="isMobileH5 ? 'h-56' : 'h-64'">
                <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                  <LoadingSpinner size="md" />
                </div>
                <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
                <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.dashboard.noDataAvailable') }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip
} from 'chart.js'
import { Line } from 'vue-chartjs'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import ChartRangeToolbar from '@/components/common/ChartRangeToolbar.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import DashboardStatCard from './DashboardStatCard.vue'
import CostHint from './CostHint.vue'
import { useDeviceMode } from '@/composables/useDeviceMode'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { DashboardStats, ModelStat, TrendDataPoint, UserSpendingRankingItem, UserUsageTrendPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const emptyStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

const stats = ref<DashboardStats>(emptyStats())
const loading = ref(false)
const recentUsageExpanded = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const { isMobileH5 } = useDeviceMode()

const formatLocalDate = (date: Date): string => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const defaultEnd = new Date()
const defaultStart = new Date(defaultEnd.getTime() - 24 * 60 * 60 * 1000)
const granularity = ref<'day' | 'hour'>('hour')
const startDate = ref(formatLocalDate(defaultStart))
const endDate = ref(formatLocalDate(defaultEnd))

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { intersect: false, mode: 'index' as const },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: { color: chartColors.value.text, usePointStyle: true, pointStyle: 'circle', padding: 15, font: { size: 11 } }
    },
    tooltip: {
      itemSort: (a: any, b: any) => Number(b?.raw ?? b?.parsed?.y ?? 0) - Number(a?.raw ?? a?.parsed?.y ?? 0),
      callbacks: { label: (context: any) => `${context.dataset.label}: ${formatTokens(context.raw)}` }
    }
  },
  scales: {
    x: { grid: { color: chartColors.value.grid }, ticks: { color: chartColors.value.text, font: { size: 10 } } },
    y: { grid: { color: chartColors.value.grid }, ticks: { color: chartColors.value.text, font: { size: 10 }, callback: (value: string | number) => formatTokens(Number(value)) } }
  }
}))

const userTrendChartData = computed(() => {
  if (!userTrend.value.length) return null
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()
  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    if (!userGroups.has(point.user_id)) {
      userGroups.set(point.user_id, { name: point.username?.trim() || point.email?.trim() || `#${point.user_id}`, data: new Map() })
    }
    userGroups.get(point.user_id)!.data.set(point.date, point.tokens)
  })
  const sortedDates = Array.from(allDates).sort()
  const colors = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316', '#6366f1', '#84cc16', '#06b6d4', '#a855f7']
  return {
    labels: sortedDates,
    datasets: Array.from(userGroups.values()).map((group, idx) => ({
      label: group.name,
      data: sortedDates.map((date) => group.data.get(date) || 0),
      borderColor: colors[idx % colors.length],
      backgroundColor: `${colors[idx % colors.length]}20`,
      fill: false,
      tension: 0.3
    }))
  }
})

function toFiniteNumber(value: unknown): number {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

function formatNumber(value: number | null | undefined): string {
  return toFiniteNumber(value).toLocaleString()
}

function formatTokens(value: number | undefined): string {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1_000_000_000) return `${(safeValue / 1_000_000_000).toFixed(2)}B`
  if (safeValue >= 1_000_000) return `${(safeValue / 1_000_000).toFixed(2)}M`
  if (safeValue >= 1_000) return `${(safeValue / 1_000).toFixed(2)}K`
  return safeValue.toLocaleString()
}

function formatCacheHitRate(inputTokens: number | undefined, cacheReadTokens: number | undefined): string {
  const cacheRead = toFiniteNumber(cacheReadTokens)
  const totalInput = toFiniteNumber(inputTokens) + cacheRead
  if (totalInput <= 0) return '0.0%'
  return `${((cacheRead / totalInput) * 100).toFixed(1)}%`
}

function formatDuration(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`
}

function goToUserUsage(item: UserSpendingRankingItem): void {
  void router.push({ path: '/admin/usage', query: { user_id: String(item.user_id), start_date: startDate.value, end_date: endDate.value } })
}

function onDateRangeChange(range: { startDate: string; endDate: string; preset: string | null }): void {
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  granularity.value = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24)) <= 1 ? 'hour' : 'day'
  void loadChartData()
}

async function loadDashboardSnapshot(includeStats: boolean): Promise<void> {
  const currentSeq = ++chartLoadSeq
  if (includeStats) loading.value = true
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) stats.value = { ...emptyStats(), ...response.stats }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

async function loadUsersTrend(): Promise<void> {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value, limit: 12 })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) userTrendLoading.value = false
  }
}

async function loadUserSpendingRanking(): Promise<void> {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({ start_date: startDate.value, end_date: endDate.value, limit: 12 })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) rankingLoading.value = false
  }
}

async function loadDashboardStats(): Promise<void> {
  await Promise.all([loadDashboardSnapshot(true), loadUsersTrend(), loadUserSpendingRanking()])
}

async function loadChartData(): Promise<void> {
  await Promise.all([loadDashboardSnapshot(false), loadUsersTrend(), loadUserSpendingRanking()])
}

onMounted(() => {
  void loadDashboardStats()
})
</script>

<style scoped>
.mobile-dashboard-section-toggle {
  @apply flex w-full items-center justify-between rounded-lg border border-slate-200 bg-white px-4 py-4 text-left text-sm font-semibold text-gray-900 shadow-card dark:border-dark-700 dark:bg-dark-800 dark:text-white;
}

.mobile-dashboard-section-toggle :deep(svg) {
  @apply text-gray-400;
}
</style>
