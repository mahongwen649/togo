<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header">
        <h1 class="page-title">我的订单</h1>
        <p class="page-description">查看账号套餐的处理状态、绑定账号和历史到期记录。</p>
      </header>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          class="btn"
          :class="activeTab === tab.value ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
          <span class="ml-1 text-xs opacity-75">{{ tabCount(tab.value) }}</span>
        </button>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>

      <div v-if="loading" class="card p-10 text-center text-sm text-gray-500">正在加载订单记录...</div>
      <div v-else-if="filteredOrders.length === 0" class="card p-10 text-center text-sm text-gray-500">暂无{{ activeTab === 'all' ? '' : currentTabLabel }}订单</div>

      <div v-else class="space-y-4">
        <article v-for="order in filteredOrders" :key="order.id" class="card p-5">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="font-semibold text-gray-900 dark:text-white">{{ order.product_name }}</h2>
                <span class="badge" :class="statusClass(order.status)">{{ statusText(order.status) }}</span>
              </div>
              <div class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                订单 #{{ order.id }} · {{ formatDate(order.created_at) }} · {{ order.quantity }} 个账号
              </div>
            </div>
            <div class="text-right">
              <div class="text-lg font-semibold text-gray-900 dark:text-white">¥{{ order.total_price.toFixed(2) }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">单价 ¥{{ order.unit_price.toFixed(2) }}</div>
            </div>
          </div>

          <div v-if="order.status === 'completed' || order.status === 'expired'" class="mt-4 grid gap-3 rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">套餐类型</div>
              <div class="mt-1 text-gray-900 dark:text-white">{{ order.mode === 'rental' ? `租号 · ${order.validity_days} 天` : '账号' }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">绑定账号</div>
              <div class="mt-1 font-mono text-gray-900 dark:text-white">{{ order.core_account_ids.length ? order.core_account_ids.map(id => `#${id}`).join(', ') : '-' }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-dark-400">到期时间</div>
              <div class="mt-1 text-gray-900 dark:text-white">{{ order.expires_at ? formatDate(order.expires_at) : '-' }}</div>
            </div>
            <div v-if="supportsAutoRenew(order)" class="flex items-center justify-between gap-3 sm:justify-start sm:gap-5 lg:justify-between">
              <div>
                <div class="text-xs text-gray-500 dark:text-dark-400">自动续订</div>
                <div class="mt-1 text-gray-900 dark:text-white">{{ order.auto_renew ? '已开启' : '未开启' }}</div>
              </div>
              <button
                type="button"
                class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors"
                :class="order.auto_renew ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'"
                :aria-pressed="order.auto_renew"
                :disabled="renewing === order.id"
                :title="order.auto_renew ? '关闭自动续订' : '开启自动续订'"
                @click="toggleAutoRenew(order)"
              >
                <span class="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform" :class="order.auto_renew ? 'translate-x-6' : 'translate-x-1'" />
              </button>
            </div>
          </div>

          <div v-if="showExpiryWarning(order)" class="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-300">
            套餐将在 {{ daysUntilExpiry(order) }} 天后到期，开启自动续订可避免账号服务中断。
          </div>
          <div v-if="supportsAutoRenew(order) && order.renewal_error" class="mt-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
            自动续订未成功：{{ order.renewal_error }}
          </div>
          <div v-if="order.status === 'pending'" class="mt-4 text-sm text-amber-600 dark:text-amber-400">管理员正在处理订单，账号绑定完成后会在“我的套餐”中显示。</div>
          <div v-else-if="order.status === 'cancelled'" class="mt-4 text-sm text-gray-500 dark:text-dark-400">订单已取消，相关余额已按系统处理。</div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { apiClient } from '@/api/client'
import { setAccountOrderAutoRenew } from '@/api/accountBusiness'

type OrderStatus = 'pending' | 'completed' | 'cancelled' | 'expired'
type OrderTab = 'all' | OrderStatus

interface Order {
  id: number
  product_name: string
  mode: 'rental' | 'purchase' | string
  validity_days: number
  quantity: number
  unit_price: number
  total_price: number
  status: OrderStatus
  expires_at: string | null
  core_group_id: number | null
  core_account_ids: number[]
  created_at: string
  auto_renew: boolean
  last_renewed_at: string | null
  renewal_error: string
}

const tabs: Array<{ value: OrderTab; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'pending', label: '待处理' },
  { value: 'completed', label: '已完成' },
  { value: 'expired', label: '已过期' },
  { value: 'cancelled', label: '已取消' },
]

const activeTab = ref<OrderTab>('all')
const orders = ref<Order[]>([])
const loading = ref(true)
const error = ref('')
const renewing = ref<number | null>(null)

const filteredOrders = computed(() => activeTab.value === 'all'
  ? orders.value
  : orders.value.filter(order => order.status === activeTab.value))

const currentTabLabel = computed(() => tabs.find(tab => tab.value === activeTab.value)?.label || '')

function tabCount(tab: OrderTab) {
  return tab === 'all' ? orders.value.length : orders.value.filter(order => order.status === tab).length
}

function statusText(status: OrderStatus) {
  return { pending: '待处理', completed: '已完成', cancelled: '已取消', expired: '已过期' }[status]
}

function statusClass(status: OrderStatus) {
  if (status === 'completed') return 'badge-success'
  if (status === 'pending') return 'badge-warning'
  if (status === 'expired') return 'badge-gray'
  return 'badge-danger'
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN')
}

function daysUntilExpiry(order: Order) {
  if (!order.expires_at) return 0
  return Math.max(1, Math.ceil((new Date(order.expires_at).getTime() - Date.now()) / 86400000))
}

function showExpiryWarning(order: Order) {
  if (order.status !== 'completed' || !supportsAutoRenew(order) || order.auto_renew || !order.expires_at) return false
  const remaining = new Date(order.expires_at).getTime() - Date.now()
  return remaining > 0 && remaining <= 3 * 86400000
}

function supportsAutoRenew(order: Pick<Order, 'mode' | 'validity_days'>) {
  return order.mode === 'rental' && order.validity_days === 30
}

async function loadOrders() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await apiClient.get<{ items: Order[] }>('/account-market/orders')
    orders.value = (data.items || []).map(order => ({
      ...order,
      unit_price: Number(order.unit_price || 0),
      total_price: Number(order.total_price || 0),
      core_account_ids: order.core_account_ids || [],
      auto_renew: Boolean(order.auto_renew),
      renewal_error: order.renewal_error || '',
    }))
  } catch (err: any) {
    error.value = err?.message || '订单加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

async function toggleAutoRenew(order: Order) {
  renewing.value = order.id
  error.value = ''
  try {
    const result = await setAccountOrderAutoRenew(order.id, !order.auto_renew)
    order.auto_renew = result.auto_renew
    order.renewal_error = ''
  } catch (err: any) {
    error.value = err?.message || '自动续订设置失败，请稍后重试'
  } finally {
    renewing.value = null
  }
}

onMounted(() => { void loadOrders() })
</script>
