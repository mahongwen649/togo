<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="page-title">我的套餐</h1>
          <p class="page-description">查看套餐状态和已分配账号的完整 Core 管理能力。</p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />刷新</button>
          <RouterLink to="/account-business/home" class="btn btn-primary"><Icon name="plus" size="sm" />购买套餐</RouterLink>
        </div>
      </header>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</div>
      <div v-if="loading" class="card p-10 text-center text-sm text-gray-500">正在加载账号...</div>
      <div v-else-if="!accounts.length" class="card p-10 text-center text-sm text-gray-500">暂无有效套餐账号</div>

      <section v-if="!loading && packages.length" class="grid gap-4 lg:grid-cols-2">
        <article v-for="packageItem in packages" :key="packageItem.id" class="card border-l-4 p-5" :class="packageItem.status === 'active' ? 'border-l-emerald-400' : 'border-l-gray-300 dark:border-l-dark-600'">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div><div class="flex items-center gap-2"><h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ packageItem.name }}</h2><span class="rounded-full px-2 py-0.5 text-xs font-medium" :class="packageItem.status === 'active' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">{{ packageStatus(packageItem.status) }}</span></div><p class="mt-1 text-xs text-gray-500">订单 #{{ packageItem.order_id }} · {{ packageItem.quantity }} 个账号 · {{ packageItem.accounts.length }} 个已同步</p></div>
            <RouterLink to="/account-business/orders" class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400">查看订单</RouterLink>
          </div>
          <div class="mt-4 grid gap-3 text-sm sm:grid-cols-3"><div><div class="text-xs text-gray-500">套餐周期</div><div class="mt-1 text-gray-800 dark:text-gray-200">{{ packageItem.mode === 'rental' ? `${packageItem.validity_days} 天` : '长期' }}</div></div><div><div class="text-xs text-gray-500">生效时间</div><div class="mt-1 text-gray-800 dark:text-gray-200">{{ formatTime(packageItem.starts_at) }}</div></div><div><div class="text-xs text-gray-500">到期时间</div><div class="mt-1 text-gray-800 dark:text-gray-200">{{ formatTime(packageItem.expires_at) }}</div></div><div v-if="supportsAutoRenew(packageItem)"><div class="text-xs text-gray-500">自动续订</div><div class="mt-1 text-gray-800 dark:text-gray-200">{{ packageItem.auto_renew ? '已开启' : '未开启' }}</div></div></div>
          <div v-if="packageItem.accounts.length" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="mb-2 flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">套餐账号</span>
              <span class="text-xs text-gray-400 dark:text-dark-500">{{ packageItem.accounts.length }} 个</span>
            </div>
            <div class="grid gap-2 sm:grid-cols-2">
              <div v-for="account in packageItem.accounts" :key="account.id" class="flex min-w-0 items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800/70">
                <div class="min-w-0"><div class="truncate text-sm text-gray-800 dark:text-gray-200">{{ account.name }}</div><div class="mt-0.5 text-[11px] text-gray-400">#{{ account.id }}</div><AccountPlatformTypeCell class="mt-1.5" :account="account" /></div>
                <span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium" :class="account.status === 'active' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-200 text-gray-600 dark:bg-dark-700 dark:text-dark-300'">{{ account.status === 'active' ? '正常' : account.status }}</span>
              </div>
            </div>
          </div>
        </article>
      </section>

      <section v-if="!loading && accounts.length" class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 bg-gray-50/80 px-5 py-4 dark:border-dark-700 dark:bg-dark-800/60">
          <div><h2 class="font-semibold text-gray-900 dark:text-white">套餐账号</h2><p class="mt-1 text-xs text-gray-500">{{ activeCount }} 个可用账号 · {{ accounts.length }} 个已绑定账号</p></div>
          <div class="flex items-center gap-2"><input v-model="search" class="input h-9 w-56" placeholder="搜索账号" /><select v-model="statusFilter" class="input h-9 w-32"><option value="all">全部状态</option><option value="active">正常</option><option value="error">异常</option><option value="inactive">暂停</option></select></div>
        </div>
        <div class="w-full overflow-x-auto">
          <table class="min-w-[1320px] w-full text-sm">
            <thead class="bg-white text-xs text-gray-500 dark:bg-dark-900 dark:text-dark-400"><tr><th class="px-5 py-3 text-left">名称</th><th class="px-5 py-3 text-left">平台/类型</th><th class="px-5 py-3 text-left">状态</th><th class="px-5 py-3 text-left">调度</th><th class="px-5 py-3 text-left">用量窗口</th><th class="px-5 py-3 text-left">最近使用</th><th class="px-5 py-3 text-right">操作</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in filteredAccounts" :key="row.account.id" class="align-top hover:bg-gray-50/70 dark:hover:bg-dark-800/40">
                <td class="px-5 py-4"><div class="font-medium text-gray-900 dark:text-white">{{ row.account.name }}</div><div class="mt-1 text-xs text-gray-500">#{{ row.account.id }}</div><div class="mt-2 text-xs text-gray-400">套餐：{{ row.package.name }}</div></td>
                <td class="min-w-[180px] px-5 py-4"><AccountPlatformTypeCell :account="row.account" /></td>
                <td class="px-5 py-4"><AccountStatusIndicator :account="row.account" @show-temp-unsched="showTempUnsched" /><div v-if="row.account.error_message" class="mt-1 max-w-[220px] truncate text-xs text-red-500" :title="row.account.error_message">{{ row.account.error_message }}</div></td>
                <td class="px-5 py-4"><button class="relative inline-flex h-6 w-11 items-center rounded-full transition disabled:cursor-not-allowed disabled:opacity-60" :class="row.account.schedulable ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'" :title="row.account.schedulable ? '暂停调度' : '启用调度'" :aria-checked="row.account.schedulable" role="switch" :disabled="isAccountBusy(row.account.id)" @click="toggleSchedule(row.account)"><Icon v-if="togglingSchedulableID === row.account.id" name="refresh" size="xs" class="absolute left-1/2 -translate-x-1/2 animate-spin text-white" /><span v-else class="inline-block h-5 w-5 transform rounded-full bg-white shadow transition" :class="row.account.schedulable ? 'translate-x-5' : 'translate-x-0.5'"></span></button><div class="mt-1 text-xs text-gray-500">{{ row.account.schedulable ? '已启用' : '已暂停' }}</div></td>
                <td class="min-w-[430px] px-5 py-4"><AccountUsageCell :account="row.account" :today-stats="todayStats[String(row.account.id)] || null" :today-stats-loading="todayStatsLoading" :manual-refresh-token="manualRefreshToken" :batched-usage="batchedUsage[String(row.account.id)] || null" :batched-usage-error="batchedUsageErrors[String(row.account.id)] || null" :batched-usage-loading="batchedUsageLoading[String(row.account.id)] === true" :request-batched-usage="requestBatchedUsage" @account-updated="updateAccount(row.account, $event)" /></td>
                <td class="whitespace-nowrap px-5 py-4 text-gray-600 dark:text-gray-300">{{ formatTime(row.account.last_used_at) }}</td>
                <td class="px-5 py-4 text-right"><div class="flex flex-wrap justify-end gap-2"><button class="btn btn-secondary px-2 py-1 text-xs" :disabled="isAccountBusy(row.account.id)" @click="refreshAccount(row.account)"><Icon name="refresh" size="xs" :class="refreshingAccountID === row.account.id ? 'animate-spin' : ''" />刷新</button><button v-if="hasRecoverableState(row.account)" class="btn btn-secondary px-2 py-1 text-xs" :disabled="isAccountBusy(row.account.id)" @click="recover(row.account)"><Icon name="sync" size="xs" :class="recoveringAccountID === row.account.id ? 'animate-spin' : ''" />恢复状态</button><button v-if="hasQuotaLimit(row.account)" class="btn btn-secondary px-2 py-1 text-xs" :disabled="isAccountBusy(row.account.id)" @click="resetQuota(row.account)"><Icon name="refresh" size="xs" :class="quotaResettingAccountID === row.account.id ? 'animate-spin' : ''" />重置配额</button><button class="btn btn-primary px-2 py-1 text-xs" :disabled="isAccountBusy(row.account.id)" @click="test(row.account)"><Icon name="play" size="xs" />测试账号</button></div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
    <AccountTestDialog :show="!!testing" :account="testing" @close="testing = null" />
    <TempUnschedStatusModal :show="!!tempUnsched" :account="tempUnsched" @close="tempUnsched = null" @reset="handleRecovered" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountTestDialog from '@/components/account-business/AccountTestDialog.vue'
import AccountPlatformTypeCell from '@/components/account-business/AccountPlatformTypeCell.vue'
import TempUnschedStatusModal from '@/components/account-business/TempUnschedStatusModal.vue'
import { getAccount, listAccountPackages, recoverAccount, type AccountBusinessCoreAccount, type AccountBusinessPackage } from '@/api/accountBusiness'
import type { Account, AccountUsageInfo } from '@/types'
import { accountsAPI } from '@/api/admin/accounts'

const packages = ref<AccountBusinessPackage[]>([])
const loading = ref(true)
const error = ref('')
const search = ref('')
const statusFilter = ref('all')
const testing = ref<AccountBusinessCoreAccount | null>(null)
const tempUnsched = ref<AccountBusinessCoreAccount | null>(null)
const manualRefreshToken = ref(0)
const refreshingAccountID = ref<number | null>(null)
const quotaResettingAccountID = ref<number | null>(null)
const recoveringAccountID = ref<number | null>(null)
const togglingSchedulableID = ref<number | null>(null)
const todayStats = ref<Record<string, any>>({})
const todayStatsLoading = ref(false)
const batchedUsage = ref<Record<string, AccountUsageInfo>>({})
const batchedUsageErrors = ref<Record<string, string>>({})
const batchedUsageLoading = ref<Record<string, boolean>>({})
let usageRequestPromise: Promise<void> | null = null
const accounts = computed(() => packages.value.flatMap((packageItem) => packageItem.status === 'active' ? packageItem.accounts.map((account) => ({ account, package: packageItem })) : []))
const activeCount = computed(() => accounts.value.filter((row) => row.account.status === 'active').length)
const filteredAccounts = computed(() => accounts.value.filter((row) => (!search.value || accountSearchText(row.account).includes(search.value.toLowerCase())) && (statusFilter.value === 'all' || row.account.status === statusFilter.value)))

function accountSearchText(account: AccountBusinessCoreAccount) {
  return `${account.name} ${account.platform} ${account.type} ${account.id} ${String(account.credentials?.plan_type || '')}`.toLowerCase()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    packages.value = await listAccountPackages()
    await Promise.all([loadTodayStats(), loadCoreUsage(true)])
  } catch (e: any) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}
async function loadTodayStats() {
  const ids = accounts.value.map((row) => row.account.id)
  if (!ids.length) return
  todayStatsLoading.value = true
  try { todayStats.value = (await accountsAPI.getBatchTodayStats(ids)).stats || {} } catch (e) { console.error(e) } finally { todayStatsLoading.value = false }
}
async function loadCoreUsage(force = false) {
  if (usageRequestPromise) return usageRequestPromise
  const ids = accounts.value.map((row) => row.account.id)
  if (!ids.length) return
  for (const id of ids) batchedUsageLoading.value[String(id)] = true
  usageRequestPromise = (async () => {
    try {
      const result = await accountsAPI.getBatchUsage(ids, force)
      batchedUsage.value = result.usage || {}
      batchedUsageErrors.value = result.errors || {}
    } catch (e) {
      console.error(e)
    } finally {
      for (const id of ids) batchedUsageLoading.value[String(id)] = false
      usageRequestPromise = null
    }
  })()
  return usageRequestPromise
}
function requestBatchedUsage(_account: Account, options?: { force?: boolean }) { void loadCoreUsage(options?.force === true) }
function updateAccount(account: AccountBusinessCoreAccount, updated: Account) { Object.assign(account, updated) }
function formatTime(value?: string | null) { if (!value) return '-'; return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function packageStatus(status: string) { return status === 'active' ? '使用中' : '已过期' }
function supportsAutoRenew(packageItem: Pick<AccountBusinessPackage, 'mode' | 'validity_days'>) { return packageItem.mode === 'rental' && packageItem.validity_days === 30 }
function isAccountBusy(accountID: number) {
  return refreshingAccountID.value === accountID || quotaResettingAccountID.value === accountID || recoveringAccountID.value === accountID || togglingSchedulableID.value === accountID
}
function hasRecoverableState(account: AccountBusinessCoreAccount) { const now = Date.now(); const extra = account.extra as Record<string, any> | undefined; const modelLimits = extra?.model_rate_limits as Record<string, any> | undefined; return account.status === 'error' || Boolean(account.rate_limited_at && (!account.rate_limit_reset_at || new Date(account.rate_limit_reset_at).getTime() > now)) || Boolean(account.overload_until && new Date(account.overload_until).getTime() > now) || Boolean(account.temp_unschedulable_until && new Date(account.temp_unschedulable_until).getTime() > now) || Boolean(modelLimits && Object.values(modelLimits).some((info: any) => info?.rate_limit_reset_at && new Date(info.rate_limit_reset_at).getTime() > now)) }
function hasQuotaLimit(account: AccountBusinessCoreAccount) { return (account.type === 'apikey' || account.type === 'bedrock') && ((account.quota_limit ?? 0) > 0 || (account.quota_daily_limit ?? 0) > 0 || (account.quota_weekly_limit ?? 0) > 0) }
async function recover(account: AccountBusinessCoreAccount) {
  if (isAccountBusy(account.id)) return
  if (account.temp_unschedulable_until && new Date(account.temp_unschedulable_until).getTime() > Date.now()) {
    tempUnsched.value = account
    return
  }
  recoveringAccountID.value = account.id
  try {
    Object.assign(account, await recoverAccount(account.id))
    await loadCoreUsage(true)
  } catch (e: any) {
    error.value = e?.message || '恢复失败'
  } finally {
    recoveringAccountID.value = null
  }
}
function handleRecovered(updated: Account) { if (tempUnsched.value) Object.assign(tempUnsched.value, updated); tempUnsched.value = null }
function showTempUnsched(account: Account) { tempUnsched.value = account as AccountBusinessCoreAccount }
async function toggleSchedule(account: AccountBusinessCoreAccount) {
  if (isAccountBusy(account.id)) return
  togglingSchedulableID.value = account.id
  try {
    Object.assign(account, await accountsAPI.setSchedulable(account.id, !account.schedulable))
  } catch (e: any) {
    error.value = e?.message || '调度更新失败'
  } finally {
    togglingSchedulableID.value = null
  }
}
async function resetQuota(account: AccountBusinessCoreAccount) {
  if (isAccountBusy(account.id)) return
  quotaResettingAccountID.value = account.id
  try {
    Object.assign(account, await accountsAPI.resetAccountQuota(account.id))
    await Promise.all([loadTodayStats(), loadCoreUsage(true)])
  } catch (e: any) {
    error.value = e?.message || '重置配额失败'
  } finally {
    quotaResettingAccountID.value = null
  }
}
function test(account: AccountBusinessCoreAccount) { testing.value = account }
async function refreshAccount(account: AccountBusinessCoreAccount) {
  if (isAccountBusy(account.id)) return
  refreshingAccountID.value = account.id
  try {
    const refreshed = await getAccount(account.id)
    Object.assign(account, refreshed)
    manualRefreshToken.value += 1
    await Promise.all([loadTodayStats(), loadCoreUsage(true)])
  } catch (e: any) {
    error.value = e?.message || '刷新账号失败'
  } finally {
    refreshingAccountID.value = null
  }
}
onMounted(load)
</script>
