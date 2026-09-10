<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card p-4">
        <div :class="isMobileH5 ? 'grid grid-cols-2 gap-2' : 'flex flex-wrap items-center gap-3'">
          <div :class="isMobileH5 ? 'col-span-2 min-w-0' : ''">
            <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="applyFilters" />
          </div>
          <div :class="isMobileH5 ? 'min-w-0' : 'w-full sm:w-64'">
            <SearchInput v-model="keyword" placeholder="用户ID / 用户名 / 邮箱" @search="applyFilters" />
          </div>
          <div :class="isMobileH5 ? 'min-w-0' : 'w-full sm:w-56'">
            <Input v-model="nearTime" placeholder="时间附近，例如 2026-07-29 18:44:40" @enter="applyFilters" />
          </div>
          <div :class="isMobileH5 ? 'min-w-0' : 'w-full sm:w-32'">
            <Select v-model="nearMinutes" :options="nearMinuteOptions" @change="applyFilters" />
          </div>
          <div :class="isMobileH5 ? 'min-w-0' : 'w-full sm:w-32'">
            <Select v-model="paymentType" placeholder="全部支付" :options="paymentTypeOptions" @change="applyFilters" />
          </div>
          <div :class="isMobileH5 ? 'min-w-0' : 'w-full sm:w-32'">
            <Select v-model="status" :options="statusOptions" @change="applyFilters" />
          </div>
          <button :class="['btn btn-secondary', { 'w-full': isMobileH5 }]" :disabled="loading" @click="reload">刷新</button>
          <button :class="['btn btn-primary', isMobileH5 ? 'w-full' : 'ml-auto']" @click="resetToToday">今天</button>
        </div>
      </section>

      <section class="grid grid-cols-2 gap-4 lg:grid-cols-4 2xl:grid-cols-8">
        <DashboardStatCard icon="dollar" icon-class="bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400" label="累计总充值" :value="formatOptionalMoney(summary.all_time_total_recharged)" />
        <DashboardStatCard icon="users" icon-class="bg-teal-100 text-teal-600 dark:bg-teal-900/30 dark:text-teal-400" label="充值用户总余额" :value="formatOptionalMoney(summary.recharge_users_balance)" />
        <DashboardStatCard icon="dollar" icon-class="bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400" label="筛选充值金额" :value="formatMoney(summary.total_pay_amount)" />
        <DashboardStatCard icon="chart" icon-class="bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400" label="充值笔数" :value="formatNumber(summary.order_count)" />
        <DashboardStatCard icon="users" icon-class="bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400" label="充值用户数" :value="formatNumber(summary.user_count)" />
        <DashboardStatCard icon="calculator" icon-class="bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400" label="平均单笔" :value="formatMoney(summary.average_pay_amount)" />
        <DashboardStatCard icon="trendingUp" icon-class="bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-400" label="最大单笔" :value="formatMoney(summary.max_pay_amount)" />
        <DashboardStatCard icon="clock" icon-class="bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-400" label="最新充值" :value="latestRechargeUser" />
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center gap-2 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            type="button"
            class="inline-flex items-center gap-1.5 border-b-2 px-3 py-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:border-dark-500 dark:hover:text-gray-200'"
            @click="selectTab(tab.value)"
          >
            {{ tab.label }}
          </button>
        </div>

        <DataTable
          v-if="activeTab === 'orders'"
          :columns="orderColumns"
          :data="orders"
          :loading="loading"
          row-key="id"
          :sticky-actions-column="false"
          :expandable-actions="false"
        >
          <template #cell-id="{ row }">
            <span class="font-mono text-xs text-gray-600 dark:text-gray-300">#{{ row.id }}</span>
          </template>
          <template #cell-user="{ row }">
            <div class="font-medium text-gray-900 dark:text-white">{{ displayUser(row) }}</div>
            <div class="text-xs text-gray-500">
              #{{ row.user_id }}<span v-if="!isMobileH5"> · {{ row.user_email || '-' }}</span>
            </div>
          </template>
          <template #cell-pay_amount="{ row }">
            <span class="font-semibold text-green-600 dark:text-green-400">{{ formatMoney(row.pay_amount) }}</span>
          </template>
          <template #cell-current_balance="{ row }">
            <span class="font-semibold text-primary-600 dark:text-primary-400">{{ formatMoney(row.current_balance) }}</span>
          </template>
          <template #cell-total_recharged="{ row }">
            <span class="font-semibold text-green-600 dark:text-green-400">{{ formatOptionalMoney(row.total_recharged) }}</span>
          </template>
        </DataTable>

        <DataTable
          v-else-if="activeTab === 'users'"
          :columns="userColumns"
          :data="users"
          :loading="loading"
          row-key="user_id"
          :sticky-actions-column="false"
          :expandable-actions="false"
        >
          <template #cell-user="{ row }">
            <div class="font-medium text-gray-900 dark:text-white">{{ displayUser(row) }}</div>
            <div class="text-xs text-gray-500">
              #{{ row.user_id }}<span v-if="!isMobileH5"> · {{ row.user_email || '-' }}</span>
            </div>
          </template>
          <template #cell-total_pay_amount="{ row }">
            <span class="font-semibold text-green-600 dark:text-green-400">{{ formatMoney(row.total_pay_amount) }}</span>
          </template>
          <template #cell-current_balance="{ row }">
            <span class="font-semibold text-primary-600 dark:text-primary-400">{{ formatMoney(row.current_balance) }}</span>
          </template>
        </DataTable>

        <DataTable
          v-else
          :columns="timeColumns"
          :data="timeseries"
          :loading="loading"
          row-key="bucket"
          :sticky-actions-column="false"
          :expandable-actions="false"
        >
          <template #cell-total_pay_amount="{ row }">
            <div class="min-w-36">
              <div class="font-semibold text-green-600 dark:text-green-400">{{ formatMoney(row.total_pay_amount) }}</div>
              <div class="mt-2 h-2 rounded-full bg-gray-100 dark:bg-dark-700">
                <div class="h-2 rounded-full bg-primary-600" :style="{ width: scaleBar(row.total_pay_amount) }"></div>
              </div>
            </div>
          </template>
        </DataTable>
      </section>

      <Pagination :page="page" :total="activeTotal" :page-size="pageSize" :show-jump="true" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Input from '@/components/common/Input.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DashboardStatCard from './DashboardStatCard.vue'
import { adminAPI } from '@/api/admin'
import type { RechargeFilters, RechargeOrder, RechargeSummary, RechargeTimeStat, RechargeUserStat } from '@/api/admin/recharges'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { useDeviceMode } from '@/composables/useDeviceMode'

type RechargeUserLike = Pick<RechargeOrder | RechargeUserStat, 'user_id' | 'user_name' | 'user_email'>

const appStore = useAppStore()
const { isMobileH5 } = useDeviceMode()

const today = formatDate(new Date())
const startDate = ref(today)
const endDate = ref(today)
const keyword = ref('')
const nearTime = ref('')
const nearMinutes = ref(5)
const paymentType = ref('')
const status = ref('COMPLETED')
const activeTab = ref<'orders' | 'users' | 'trend'>('orders')
const loading = ref(false)
const orders = ref<RechargeOrder[]>([])
const users = ref<RechargeUserStat[]>([])
const timeseries = ref<RechargeTimeStat[]>([])
const summary = ref<RechargeSummary>({
  total_amount: 0,
  total_pay_amount: 0,
  order_count: 0,
  user_count: 0,
  average_pay_amount: 0,
  max_pay_amount: 0,
  latest: null,
  start_time: '',
  end_time: '',
  granularity: 'hour'
})
const orderTotal = ref(0)
const userTotal = ref(0)
const timeseriesTotal = ref(0)
const page = ref(1)
const pageSize = ref(50)

const tabs = [
  { value: 'orders', label: '充值明细' },
  { value: 'users', label: '用户统计' },
  { value: 'trend', label: '时间统计' }
] as const

const nearMinuteOptions: SelectOption[] = [
  { value: 1, label: '±1分钟' },
  { value: 5, label: '±5分钟' },
  { value: 10, label: '±10分钟' },
  { value: 30, label: '±30分钟' }
]

const paymentTypeOptions: SelectOption[] = [
  { value: '', label: '全部支付' },
  { value: 'alipay', label: '支付宝' },
  { value: 'wechat', label: '微信' },
  { value: 'stripe', label: 'Stripe' }
]

const statusOptions: SelectOption[] = [
  { value: 'COMPLETED', label: '已完成' },
  { value: 'PAID', label: '已支付' },
  { value: 'SUCCESS', label: '成功' },
  { value: 'all', label: '全部' }
]

const orderColumns: Column[] = [
  { key: 'id', label: '订单' },
  { key: 'user', label: '用户' },
  { key: 'current_balance', label: '当前余额', class: 'text-right' },
  { key: 'total_recharged', label: '累计充值', class: 'text-right' },
  { key: 'pay_amount', label: '金额', class: 'text-right' },
  { key: 'payment_type', label: '支付方式', formatter: valueOrDash },
  { key: 'status', label: '状态', formatter: valueOrDash },
  { key: 'effective_time', label: '到账时间', formatter: valueOrDash }
]

const userColumns: Column[] = [
  { key: 'user', label: '用户' },
  { key: 'current_balance', label: '当前余额', class: 'text-right' },
  { key: 'total_pay_amount', label: '总金额', class: 'text-right' },
  { key: 'order_count', label: '笔数', class: 'text-right', formatter: formatNumber },
  { key: 'average_pay_amount', label: '平均', class: 'text-right', formatter: formatMoney },
  { key: 'max_pay_amount', label: '最大', class: 'text-right', formatter: formatMoney },
  { key: 'latest_time', label: '最近充值', formatter: valueOrDash }
]

const timeColumns: Column[] = [
  { key: 'bucket', label: '时间' },
  { key: 'total_pay_amount', label: '金额' },
  { key: 'order_count', label: '笔数', class: 'text-right', formatter: formatNumber },
  { key: 'user_count', label: '用户数', class: 'text-right', formatter: formatNumber }
]

const maxBar = computed(() => Math.max(1, ...timeseries.value.map((item) => item.total_pay_amount || 0)))
const latestRechargeUser = computed(() => summary.value.latest ? displayUser(summary.value.latest) : '-')
const activeTotal = computed(() => {
  if (activeTab.value === 'users') return userTotal.value
  if (activeTab.value === 'trend') return timeseriesTotal.value
  return orderTotal.value
})

function formatDate(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function formatMoney(value: number | string): string {
  const numeric = Number(value || 0)
  return new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(numeric)
}

function formatOptionalMoney(value: number | string | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(Number(value))) return '-'
  return formatMoney(value)
}

function formatNumber(value: number | string): string {
  return new Intl.NumberFormat().format(Number(value || 0))
}

function valueOrDash(value: unknown): string {
  return value ? String(value) : '-'
}

function displayUser(row: RechargeUserLike): string {
  return row.user_name || row.user_email || `#${row.user_id}`
}

function scaleBar(value: number): string {
  return `${Math.max(4, Math.round(((value || 0) / maxBar.value) * 100))}%`
}

function buildParams(): RechargeFilters {
  return {
    start_date: startDate.value,
    end_date: endDate.value,
    keyword: keyword.value || undefined,
    near_time: nearTime.value || undefined,
    near_minutes: nearTime.value ? nearMinutes.value : undefined,
    payment_type: paymentType.value || undefined,
    status: status.value || undefined,
    page: page.value,
    page_size: pageSize.value,
    granularity: endDate.value === startDate.value ? 'hour' : 'day'
  }
}

function applyFilters() {
  page.value = 1
  void reload()
}

async function reload() {
  loading.value = true
  try {
    const payload = await adminAPI.recharges.list(buildParams())
    orders.value = payload.items
    users.value = payload.users
    timeseries.value = payload.timeseries
    summary.value = payload.summary
    orderTotal.value = payload.total
    userTotal.value = payload.users_total ?? payload.users.length
    timeseriesTotal.value = payload.timeseries_total ?? payload.timeseries.length
    page.value = payload.page
    pageSize.value = payload.page_size
  } catch (error: any) {
    appStore.showError(error?.message || '充值记录加载失败')
  } finally {
    loading.value = false
  }
}

function resetToToday() {
  const value = formatDate(new Date())
  startDate.value = value
  endDate.value = value
  keyword.value = ''
  nearTime.value = ''
  nearMinutes.value = 5
  paymentType.value = ''
  status.value = 'COMPLETED'
  page.value = 1
  void reload()
}

function selectTab(tab: 'orders' | 'users' | 'trend') {
  if (activeTab.value === tab) return
  activeTab.value = tab
  page.value = 1
  void reload()
}

function handlePageChange(nextPage: number) {
  page.value = nextPage
  void reload()
}

function handlePageSizeChange(nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void reload()
}

onMounted(() => {
  void reload()
})
</script>
