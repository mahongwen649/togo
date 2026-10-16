<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header">
        <h1 class="page-title">用户套餐查询</h1>
        <p class="page-description">搜索用户并查看中转余额、套餐余额、使用情况、套餐状态及资金记录。</p>
      </header>

      <section class="border-y border-gray-200 py-4 dark:border-dark-700">
        <form class="flex flex-wrap items-end gap-2" @submit.prevent="searchUsers">
          <label class="min-w-[17rem] flex-1 space-y-1 text-xs text-gray-500 dark:text-dark-400">
            <span>用户搜索</span>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model.trim="keyword" class="input w-full pl-9" placeholder="输入用户 ID、邮箱或用户名" maxlength="200" />
            </div>
          </label>
          <button class="btn btn-primary" type="submit" :disabled="searching"><Icon name="search" size="sm" />{{ searching ? '查询中...' : '查询用户' }}</button>
        </form>

        <div v-if="searchResults.length" class="mt-4 overflow-x-auto border-t border-gray-100 dark:border-dark-700">
          <table class="min-w-full text-left text-sm">
            <thead class="text-xs text-gray-500 dark:text-dark-400"><tr><th class="px-3 py-2">用户</th><th class="px-3 py-2">中转余额</th><th class="px-3 py-2">状态</th><th class="px-3 py-2 text-right">操作</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="user in searchResults" :key="user.id" :class="overview?.user.id === user.id ? 'bg-primary-50/70 dark:bg-primary-950/20' : ''">
                <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ user.username || user.email }}</div><div class="text-xs text-gray-500">{{ user.email }} · ID {{ user.id }}</div></td>
                <td class="px-3 py-3 font-medium text-emerald-600">¥{{ money(user.balance) }}</td>
                <td class="px-3 py-3"><span class="badge" :class="user.status === 'active' ? 'badge-success' : 'badge-danger'">{{ user.status === 'active' ? '正常' : '已停用' }}</span></td>
                <td class="px-3 py-3 text-right"><button class="btn btn-secondary btn-sm" type="button" @click="selectUser(user.id)"><Icon name="eye" size="sm" />查看</button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else-if="searched && !searching" class="mt-4 text-sm text-gray-500">没有找到匹配用户</p>
      </section>

      <div v-if="error" class="border-l-4 border-red-500 bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</div>
      <div v-if="loading" class="py-16 text-center text-sm text-gray-500">正在加载用户套餐资料...</div>

      <template v-else-if="overview">
        <section class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 pb-5 dark:border-dark-700">
          <div class="flex items-center gap-3">
            <span class="flex h-11 w-11 items-center justify-center rounded-lg bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300"><Icon name="user" size="lg" /></span>
            <div><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ overview.user.username || overview.user.email }}</h2><p class="text-sm text-gray-500">{{ overview.user.email }} · 用户 ID {{ overview.user.id }}</p></div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-primary" type="button" @click="openRecharge"><Icon name="plus" size="sm" />充值套餐余额</button>
            <button class="btn btn-secondary text-red-600 hover:text-red-700 dark:text-red-400" type="button" :disabled="overview.account_balance <= 0" @click="openDeduction"><Icon name="minus" size="sm" />扣减套餐余额</button>
            <button class="btn btn-secondary" type="button" @click="openUsage"><Icon name="chart" size="sm" />详细使用记录</button>
            <button class="btn btn-secondary" type="button" @click="openTransactions"><Icon name="creditCard" size="sm" />充值兑换记录</button>
            <button class="btn btn-secondary" type="button" title="刷新" aria-label="刷新" @click="loadOverview(overview.user.id)"><Icon name="refresh" size="sm" /></button>
          </div>
        </section>

        <section class="grid grid-cols-2 border-y border-gray-200 dark:border-dark-700 lg:grid-cols-4">
          <div class="flex min-w-0 items-center gap-3 border-b border-r border-gray-200 p-4 dark:border-dark-700 lg:border-b-0">
            <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30"><Icon name="link" size="md" /></span>
            <div class="min-w-0"><div class="text-xs text-gray-500">中转余额</div><div class="truncate text-lg font-bold text-emerald-600">¥{{ money(overview.user.balance) }}</div></div>
          </div>
          <div class="flex min-w-0 items-center gap-3 border-b border-gray-200 p-4 dark:border-dark-700 lg:border-b-0 lg:border-r">
            <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-sky-100 text-sky-600 dark:bg-sky-900/30"><Icon name="grid" size="md" /></span>
            <div class="min-w-0"><div class="text-xs text-gray-500">套餐余额</div><div class="truncate text-lg font-bold text-sky-600">¥{{ money(overview.account_balance) }}</div></div>
          </div>
          <div class="flex min-w-0 items-center gap-3 border-r border-gray-200 p-4 dark:border-dark-700">
            <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-violet-100 text-violet-600 dark:bg-violet-900/30"><Icon name="chart" size="md" /></span>
            <div class="min-w-0"><div class="text-xs text-gray-500">本月使用</div><div class="truncate text-lg font-bold text-gray-900 dark:text-white">{{ overview.usage_available ? `${number(overview.usage.total_requests)} 次` : '--' }}</div><div class="text-xs text-gray-500">费用 ¥{{ overview.usage_available ? money(overview.usage.total_cost) : '--' }}</div></div>
          </div>
          <div class="flex min-w-0 items-center gap-3 p-4">
            <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md bg-amber-100 text-amber-600 dark:bg-amber-900/30"><Icon name="cube" size="md" /></span>
            <div class="min-w-0"><div class="text-xs text-gray-500">有效套餐</div><div class="truncate text-lg font-bold text-gray-900 dark:text-white">{{ overview.summary.active_packages }} / {{ overview.summary.total_packages }}</div><div class="text-xs text-gray-500">累计消费 ¥{{ money(overview.summary.total_spent) }}</div></div>
          </div>
        </section>

        <section class="space-y-3">
          <div class="flex flex-wrap items-end justify-between gap-3"><div><h2 class="text-base font-semibold text-gray-900 dark:text-white">套餐情况</h2><p class="mt-1 text-xs text-gray-500">展示该用户全部账号套餐及有效期。</p></div><div class="text-sm text-gray-500">在线充值 ¥{{ money(overview.summary.total_payments) }} · 手动充值 ¥{{ money(overview.summary.total_adjustments) }} · 手动扣款 ¥{{ money(overview.summary.total_deductions) }} · 卡密兑换 ¥{{ money(overview.summary.total_redemptions) }}</div></div>
          <div class="overflow-x-auto border-y border-gray-200 dark:border-dark-700">
            <table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">套餐</th><th class="px-4 py-3">类型</th><th class="px-4 py-3">账号</th><th class="px-4 py-3">金额</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">有效期</th><th class="px-4 py-3">自动续费</th></tr></thead>
              <tbody v-if="overview.packages.length" class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in overview.packages" :key="item.id"><td class="px-4 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ item.name }}</div><div class="text-xs text-gray-500">订单 #{{ item.id }}</div></td><td class="px-4 py-3">{{ item.mode === 'rental' ? `租号 ${item.validity_days} 天` : '账号购买' }}</td><td class="px-4 py-3">{{ item.core_account_ids.length ? item.core_account_ids.map(id => `#${id}`).join(', ') : `${item.quantity} 个待分配` }}</td><td class="px-4 py-3">¥{{ money(item.total_price) }}</td><td class="px-4 py-3"><span class="badge" :class="packageStatusClass(item.package_status)">{{ packageStatusText(item.package_status) }}</span></td><td class="whitespace-nowrap px-4 py-3 text-gray-500">{{ dateRange(item.starts_at, item.expires_at) }}</td><td class="px-4 py-3">{{ item.auto_renew ? '已开启' : '-' }}</td></tr></tbody>
            </table>
            <div v-if="!overview.packages.length" class="p-10 text-center text-sm text-gray-500">该用户暂无账号套餐</div>
          </div>
        </section>

        <section class="grid gap-6 xl:grid-cols-2">
          <div class="min-w-0 space-y-3"><div class="flex items-center justify-between"><h2 class="flex items-center gap-2 text-base font-semibold"><Icon name="creditCard" size="sm" class="text-sky-600" />最近在线充值</h2><span class="text-xs text-gray-500">成功 {{ overview.summary.payment_count }} 笔</span></div><div class="overflow-x-auto border-y border-gray-200 dark:border-dark-700"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-3 py-2">订单</th><th class="px-3 py-2">金额</th><th class="px-3 py-2">状态</th><th class="px-3 py-2">时间</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in overview.payments" :key="item.id"><td class="max-w-40 truncate px-3 py-3 font-mono text-xs" :title="item.out_trade_no">{{ item.out_trade_no }}</td><td class="px-3 py-3 text-emerald-600">¥{{ money(item.amount) }}</td><td class="px-3 py-3">{{ financeStatusText(item.status) }}</td><td class="whitespace-nowrap px-3 py-3 text-xs text-gray-500">{{ formatDate(item.completed_at || item.created_at) }}</td></tr></tbody></table><div v-if="!overview.payments.length" class="p-8 text-center text-sm text-gray-500">暂无在线充值</div></div></div>
          <div class="min-w-0 space-y-3"><div class="flex items-center justify-between"><h2 class="flex items-center gap-2 text-base font-semibold"><Icon name="gift" size="sm" class="text-violet-600" />最近卡密兑换</h2><span class="text-xs text-gray-500">共 {{ overview.summary.redemption_count }} 笔</span></div><div class="overflow-x-auto border-y border-gray-200 dark:border-dark-700"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-3 py-2">卡密</th><th class="px-3 py-2">批次</th><th class="px-3 py-2">金额</th><th class="px-3 py-2">时间</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in overview.redemptions" :key="item.id"><td class="px-3 py-3 font-mono text-xs">****-{{ item.code_suffix }}</td><td class="px-3 py-3">{{ item.batch_name || `批次 #${item.batch_id}` }}</td><td class="px-3 py-3 text-emerald-600">¥{{ money(item.amount) }}</td><td class="whitespace-nowrap px-3 py-3 text-xs text-gray-500">{{ formatDate(item.redeemed_at) }}</td></tr></tbody></table><div v-if="!overview.redemptions.length" class="p-8 text-center text-sm text-gray-500">暂无卡密兑换</div></div></div>
        </section>
      </template>

      <div v-else-if="!loading" class="border-y border-gray-200 py-16 text-center text-sm text-gray-500 dark:border-dark-700"><Icon name="users" size="xl" class="mx-auto mb-3 text-gray-300" /><p>搜索并选择用户后查看套餐和使用情况</p></div>
    </div>

    <BaseDialog :show="showAdjustment" :title="adjustmentMode === 'credit' ? '充值账号套餐余额' : '扣减账号套餐余额'" width="narrow" @close="closeAdjustment">
      <form v-if="overview" id="account-balance-adjustment-form" class="space-y-5" @submit.prevent="submitAdjustment">
        <div class="flex items-center gap-3 rounded-lg bg-gray-50 p-4 dark:bg-dark-800">
          <span class="flex h-10 w-10 items-center justify-center rounded-md bg-sky-100 text-sky-600 dark:bg-sky-900/30"><Icon name="grid" size="md" /></span>
          <div class="min-w-0 flex-1">
            <div class="truncate font-medium text-gray-900 dark:text-white">{{ overview.user.username || overview.user.email }}</div>
            <div class="text-sm text-gray-500">当前套餐余额 ¥{{ money(overview.account_balance) }}</div>
          </div>
        </div>
        <label class="block space-y-1.5">
          <span class="input-label">{{ adjustmentMode === 'credit' ? '充值金额' : '扣款金额' }}</span>
          <div class="relative"><span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">¥</span><input v-model.number="adjustmentAmount" class="input w-full pl-8" type="number" min="0.01" :max="adjustmentMax" step="0.01" required placeholder="0.00" /></div>
        </label>
        <label class="block space-y-1.5"><span class="input-label">操作备注</span><textarea v-model.trim="adjustmentNote" class="input w-full" rows="3" maxlength="200" :placeholder="adjustmentMode === 'credit' ? '例如：售后补偿、线下收款' : '请填写扣款原因'"></textarea></label>
        <div v-if="validAdjustmentAmount" class="flex items-center justify-between border-y border-gray-200 py-3 text-sm dark:border-dark-700"><span class="text-gray-500">操作后套餐余额</span><strong class="text-lg" :class="adjustmentMode === 'credit' ? 'text-emerald-600' : 'text-red-600'">¥{{ money(adjustedBalance) }}</strong></div>
        <p v-if="adjustmentMode === 'credit'" class="text-xs text-gray-500">此操作只增加账号套餐余额，不会改变用户的中转余额。</p>
        <p v-else class="border-l-4 border-red-500 bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300">扣款会立即减少该用户的账号套餐余额，且不能超过当前余额。操作将写入管理员资金流水。</p>
      </form>
      <template #footer>
        <button class="btn btn-secondary" type="button" :disabled="adjusting" @click="closeAdjustment">取消</button>
        <button class="btn" :class="adjustmentMode === 'credit' ? 'btn-primary' : 'bg-red-600 text-white hover:bg-red-700'" type="submit" form="account-balance-adjustment-form" :disabled="adjusting || !validAdjustmentAmount"><Icon :name="adjustmentMode === 'credit' ? 'plus' : 'minus'" size="sm" />{{ adjusting ? '处理中...' : adjustmentMode === 'credit' ? '确认充值' : '确认扣款' }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { usersAPI } from '@/api/admin/users'
import { accountAdminAPI, type AccountAdminUserOverview } from '@/api/accountBusiness'
import type { AdminUser } from '@/types'
import { useAppStore } from '@/stores/app'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const keyword = ref('')
const searchResults = ref<AdminUser[]>([])
const searched = ref(false)
const searching = ref(false)
const loading = ref(false)
const error = ref('')
const overview = ref<AccountAdminUserOverview | null>(null)
const showAdjustment = ref(false)
const adjustmentMode = ref<'credit' | 'debit'>('credit')
const adjustmentAmount = ref(0)
const adjustmentNote = ref('')
const adjusting = ref(false)
const adjustmentMax = computed(() => adjustmentMode.value === 'credit' ? 1_000_000 : Math.min(1_000_000, overview.value?.account_balance || 0))
const validAdjustmentAmount = computed(() => Number.isFinite(adjustmentAmount.value) && adjustmentAmount.value >= 0.01 && adjustmentAmount.value <= adjustmentMax.value && Math.abs(Math.round(adjustmentAmount.value * 100) - adjustmentAmount.value * 100) < 0.00000001)
const adjustedBalance = computed(() => (overview.value?.account_balance || 0) + (adjustmentMode.value === 'credit' ? adjustmentAmount.value : -adjustmentAmount.value))

function money(value?: number) { return Number(value || 0).toFixed(2) }
function number(value?: number) { return Number(value || 0).toLocaleString('zh-CN') }
function formatDate(value?: string | null) { return value ? new Date(value).toLocaleString('zh-CN') : '-' }
function dateRange(start?: string | null, end?: string | null) { return `${start ? new Date(start).toLocaleDateString('zh-CN') : '长期'} - ${end ? new Date(end).toLocaleDateString('zh-CN') : '长期'}` }
function packageStatusText(value: string) { return ({ active: '使用中', expired: '已过期', pending: '待处理', cancelled: '已取消' }[value] || value) }
function packageStatusClass(value: string) { return value === 'active' ? 'badge-success' : value === 'pending' ? 'badge-warning' : value === 'cancelled' ? 'badge-danger' : 'badge-gray' }
function financeStatusText(value: string) { return ({ COMPLETED: '已完成', PENDING: '待支付', EXPIRED: '已过期' }[value] || value) }

async function searchUsers() {
  if (!keyword.value) return
  searching.value = true
  searched.value = true
  error.value = ''
  try {
    const result = await usersAPI.list(1, 20, { search: keyword.value, role: 'user' })
    searchResults.value = result.items || []
    if (searchResults.value.length === 1) await selectUser(searchResults.value[0].id)
  } catch (err: any) {
    error.value = err?.message || '用户搜索失败'
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

async function loadOverview(userID: number) {
  loading.value = true
  error.value = ''
  try {
    overview.value = await accountAdminAPI.userOverview(userID)
  } catch (err: any) {
    overview.value = null
    error.value = err?.message || '用户套餐资料加载失败'
  } finally {
    loading.value = false
  }
}

async function selectUser(userID: number) {
  await router.replace({ query: { ...route.query, user_id: String(userID) } })
  await loadOverview(userID)
}

function openUsage() { if (overview.value) void router.push({ path: '/admin/usage', query: { user_id: String(overview.value.user.id) } }) }
function openTransactions() { if (overview.value) void router.push({ path: '/admin/account-business/transactions', query: { search: String(overview.value.user.id) } }) }
function openAdjustment(mode: 'credit' | 'debit') { adjustmentMode.value = mode; adjustmentAmount.value = 0; adjustmentNote.value = ''; showAdjustment.value = true }
function openRecharge() { openAdjustment('credit') }
function openDeduction() { openAdjustment('debit') }
function closeAdjustment() { if (!adjusting.value) showAdjustment.value = false }

async function submitAdjustment() {
  if (!overview.value || !validAdjustmentAmount.value) {
    appStore.showError(adjustmentMode.value === 'credit' ? '请输入有效的充值金额，最多保留两位小数' : '请输入不超过当前套餐余额的有效扣款金额')
    return
  }
  adjusting.value = true
  try {
    const result = await accountAdminAPI.adjustBalance(overview.value.user.id, { operation: adjustmentMode.value, amount: adjustmentAmount.value, note: adjustmentNote.value || undefined })
    overview.value.account_balance = result.balance_after
    appStore.showSuccess(`${adjustmentMode.value === 'credit' ? '已充值' : '已扣款'} ¥${money(result.amount)}，当前套餐余额 ¥${money(result.balance_after)}`)
    showAdjustment.value = false
    await loadOverview(overview.value.user.id)
  } catch (err: any) {
    appStore.showError(err?.message || '账号套餐余额调整失败')
  } finally {
    adjusting.value = false
  }
}

onMounted(() => {
  const userID = Number(route.query.user_id)
  if (Number.isInteger(userID) && userID > 0) void loadOverview(userID)
})
</script>
