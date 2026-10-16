<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="page-header flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="page-title">充值与兑换记录</h1>
          <p class="page-description">查看账号套餐余额的在线充值、卡密兑换和管理员手动调整流水。</p>
        </div>
        <button class="btn btn-secondary" type="button" title="刷新" aria-label="刷新" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" />
        </button>
      </header>

      <div class="border-b border-gray-200 dark:border-dark-700">
        <nav class="flex gap-6" aria-label="记录类型">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            class="flex items-center gap-2 border-b-2 px-1 py-3 text-sm font-medium transition-colors"
            :class="activeType === tab.key ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-white'"
            @click="switchType(tab.key)"
          >
            <Icon :name="tab.icon" size="sm" />{{ tab.label }}
          </button>
        </nav>
      </div>

      <section class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-700">
        <form class="flex min-w-0 flex-1 flex-wrap items-end gap-2" @submit.prevent="applySearch">
          <label class="min-w-[16rem] flex-1 space-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span>搜索用户或记录</span>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model.trim="searchDraft" class="input w-full pl-9" placeholder="用户 ID、邮箱、用户名、订单号或卡密尾号" maxlength="200" />
            </div>
          </label>
          <label class="space-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span>状态</span>
            <select v-model="status" class="input min-w-32" @change="applyFilters">
              <option value="">全部状态</option>
              <option v-for="option in statusOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <button class="btn btn-primary" type="submit"><Icon name="search" size="sm" />查询</button>
          <button v-if="search || status" class="btn btn-secondary" type="button" @click="resetFilters">清空</button>
        </form>
        <div class="flex gap-5 text-sm">
          <div><span class="text-gray-500 dark:text-dark-400">记录</span><strong class="ml-2 text-gray-900 dark:text-white">{{ total }}</strong></div>
          <div><span class="text-gray-500 dark:text-dark-400">{{ activeType === 'adjustment' ? '净调整金额' : '成功金额' }}</span><strong class="ml-2" :class="summary.total_amount < 0 ? 'text-red-600' : 'text-emerald-600'">{{ signedMoney(summary.total_amount, activeType === 'adjustment') }}</strong></div>
        </div>
      </section>

      <div v-if="error" class="border-l-4 border-red-500 bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>

      <section class="overflow-x-auto border-y border-gray-200 dark:border-dark-700">
        <table class="min-w-full text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            <tr>
              <th class="px-4 py-3">用户</th>
              <th class="px-4 py-3">记录</th>
              <th class="px-4 py-3">到账金额</th>
              <th v-if="activeType === 'payment'" class="px-4 py-3">实付金额</th>
              <th class="px-4 py-3">状态</th>
              <th class="px-4 py-3">时间</th>
              <th class="px-4 py-3 text-right">操作</th>
            </tr>
          </thead>
          <tbody v-if="!loading && items.length" class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="item in items" :key="`${item.kind}-${item.id}`">
              <td class="px-4 py-3">
                <div class="font-medium text-gray-900 dark:text-white">{{ item.user?.username || item.user?.email || `用户 #${item.user_id}` }}</div>
                <div class="mt-0.5 text-xs text-gray-500">{{ item.user?.email || `ID: ${item.user_id}` }}</div>
              </td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-2"><Icon :name="kindIcon(item.kind)" size="sm" :class="kindColor(item.kind)" /><span>{{ kindText(item.kind, item.amount) }}</span></div>
                <div class="mt-1 max-w-64 truncate font-mono text-xs text-gray-500" :title="item.reference">{{ item.kind === 'redemption' ? `****-${item.reference}` : item.reference }}</div>
                <div v-if="item.batch_name" class="mt-0.5 text-xs text-gray-500">{{ item.batch_name }}</div>
                <div v-if="item.note" class="mt-0.5 max-w-72 truncate text-xs text-gray-500" :title="item.note">{{ item.note }}<span v-if="item.operator_id"> · 管理员 #{{ item.operator_id }}</span></div>
              </td>
              <td class="px-4 py-3 font-semibold" :class="item.amount < 0 ? 'text-red-600' : 'text-emerald-600'">{{ signedMoney(item.amount, item.kind === 'adjustment') }}</td>
              <td v-if="activeType === 'payment'" class="px-4 py-3">¥{{ money(item.pay_amount) }}</td>
              <td class="px-4 py-3"><span class="badge" :class="statusClass(item.status)">{{ statusText(item.status) }}</span></td>
              <td class="whitespace-nowrap px-4 py-3 text-gray-500 dark:text-dark-400">{{ formatDate(item.completed_at || item.created_at) }}</td>
              <td class="px-4 py-3 text-right"><button class="btn btn-secondary btn-sm" type="button" @click="openUser(item.user_id)"><Icon name="user" size="sm" />用户详情</button></td>
            </tr>
          </tbody>
        </table>
        <div v-if="loading" class="p-12 text-center text-sm text-gray-500">正在加载记录...</div>
        <div v-else-if="!items.length" class="p-12 text-center text-sm text-gray-500">暂无符合条件的记录</div>
      </section>

      <div v-if="pages > 1" class="flex items-center justify-between text-sm">
        <span class="text-gray-500">第 {{ page }} / {{ pages }} 页</span>
        <div class="flex gap-2">
          <button class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="changePage(page - 1)">上一页</button>
          <button class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="changePage(page + 1)">下一页</button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { accountAdminAPI, type AccountAdminTransaction } from '@/api/accountBusiness'

type TransactionType = 'payment' | 'redemption' | 'adjustment'

const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'payment' as const, label: '在线充值记录', icon: 'creditCard' as const },
  { key: 'redemption' as const, label: '卡密兑换记录', icon: 'gift' as const },
  { key: 'adjustment' as const, label: '手动调整记录', icon: 'swap' as const },
]
const activeType = ref<TransactionType>(route.query.type === 'redemption' || route.query.type === 'adjustment' ? route.query.type : 'payment')
const searchDraft = ref(typeof route.query.search === 'string' ? route.query.search : '')
const search = ref(searchDraft.value)
const status = ref('')
const page = ref(1)
const pages = ref(1)
const total = ref(0)
const items = ref<AccountAdminTransaction[]>([])
const summary = ref({ total_amount: 0, completed_count: 0 })
const loading = ref(false)
const error = ref('')

const statusOptions = computed(() => activeType.value === 'payment'
  ? [{ value: 'COMPLETED', label: '已完成' }, { value: 'PENDING', label: '待支付' }, { value: 'EXPIRED', label: '已过期' }]
  : activeType.value === 'redemption'
    ? [{ value: 'redeemed', label: '已兑换' }, { value: 'partially_used', label: '部分使用' }, { value: 'used', label: '已用完' }]
    : [{ value: 'COMPLETED', label: '已完成' }])

function money(value: number | undefined) { return Number(value || 0).toFixed(2) }
function signedMoney(value: number | undefined, showPlus = false) { const amount = Number(value || 0); return `${amount < 0 ? '-' : showPlus && amount > 0 ? '+' : ''}¥${Math.abs(amount).toFixed(2)}` }
function formatDate(value?: string | null) { return value ? new Date(value).toLocaleString('zh-CN') : '-' }
function statusText(value: string) { return ({ COMPLETED: '已完成', PENDING: '待支付', EXPIRED: '已过期', redeemed: '已兑换', partially_used: '部分使用', used: '已用完' }[value] || value) }
function statusClass(value: string) { return value === 'COMPLETED' || value === 'redeemed' ? 'badge-success' : value === 'PENDING' || value === 'partially_used' ? 'badge-warning' : 'badge-gray' }
function kindText(value: AccountAdminTransaction['kind'], amount?: number) { return value === 'payment' ? '在线充值' : value === 'redemption' ? '卡密兑换' : Number(amount) < 0 ? '管理员手动扣款' : '管理员手动充值' }
function kindIcon(value: AccountAdminTransaction['kind']) { return value === 'payment' ? 'creditCard' : value === 'redemption' ? 'gift' : 'swap' }
function kindColor(value: AccountAdminTransaction['kind']) { return value === 'payment' ? 'text-sky-600' : value === 'redemption' ? 'text-violet-600' : 'text-gray-600' }

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await accountAdminAPI.transactions({ type: activeType.value, page: page.value, page_size: 30, status: status.value || undefined, search: search.value || undefined })
    items.value = result.items || []
    total.value = result.total || 0
    pages.value = result.pages || 1
    summary.value = result.summary || { total_amount: 0, completed_count: 0 }
  } catch (err: any) {
    error.value = err?.message || '记录加载失败'
    items.value = []
  } finally {
    loading.value = false
  }
}

function switchType(value: TransactionType) { activeType.value = value; status.value = ''; page.value = 1; void load() }
function applySearch() { search.value = searchDraft.value; page.value = 1; void load() }
function applyFilters() { page.value = 1; void load() }
function resetFilters() { searchDraft.value = ''; search.value = ''; status.value = ''; page.value = 1; void load() }
function changePage(value: number) { page.value = value; void load() }
function openUser(userID: number) { void router.push({ path: '/admin/account-business/users', query: { user_id: String(userID) } }) }

onMounted(() => { void load() })
</script>
