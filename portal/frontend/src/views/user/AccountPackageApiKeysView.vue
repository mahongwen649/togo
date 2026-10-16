<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="page-title">账号专用 Key</h1>
          <p class="page-description">沿用 Core API Key 的真实能力，Key 只能访问你当前有效套餐绑定的账号。</p>
        </div>
        <button class="btn btn-primary" :disabled="!canCreate || loading" @click="showCreate = true">
          <Icon name="plus" size="sm" class="mr-2" />创建 Key
        </button>
      </header>

      <div v-if="error" role="alert" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>

      <section class="card p-5">
        <div class="flex flex-wrap items-center gap-x-8 gap-y-3 text-sm">
          <div>
            <div class="text-gray-500 dark:text-dark-400">可用套餐</div>
            <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ activePackages.length }} 个</div>
          </div>
          <div>
            <div class="text-gray-500 dark:text-dark-400">授权账号</div>
            <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ authorizedAccountCount }} 个</div>
          </div>
          <div>
            <div class="text-gray-500 dark:text-dark-400">已创建 Key</div>
            <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ visibleKeys.length }} 个</div>
          </div>
        </div>
        <p v-if="!loading && !canCreate" class="mt-4 text-sm text-amber-700 dark:text-amber-300">
          当前没有已完成且绑定 Core 分组的有效套餐，管理员完成账号分配后才能创建账号专用 Key。
        </p>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">我的账号套餐 Key</h2>
        </div>
        <div v-if="loading" class="p-10 text-center text-sm text-gray-500">正在同步 Core Key...</div>
        <div v-else-if="visibleKeys.length === 0" class="p-10 text-center text-sm text-gray-500">暂无账号专用 Key</div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="key in visibleKeys" :key="key.id" data-test="package-key-row" class="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
            <div class="min-w-0 flex-1 basis-48 break-words">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ key.name }}</span>
                <span class="badge" :class="key.status === 'active' ? 'badge-success' : key.status === 'expired' ? 'badge-danger' : 'badge-gray'">
                  {{ statusLabel(key.status) }}
                </span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-2 font-mono text-xs text-gray-500 dark:text-dark-400">
                <span>{{ maskApiKey(key.key) }}</span>
              </div>
              <div class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                绑定套餐：{{ packageName(key.group_id) }} · 到期：{{ formatDate(key.expires_at) }}
              </div>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <button v-if="!publicSettings?.hide_ccs_import_button" type="button" class="flex min-w-16 flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:opacity-50 dark:hover:bg-blue-900/20 dark:hover:text-blue-400" title="导入到 CCS" aria-label="导入到 CCS" :disabled="!settingsReady || deleting" @click="importToCcswitch(key)">
                <Icon name="upload" size="sm" />
                <span class="whitespace-nowrap text-xs">导入到 CCS</span>
              </button>
              <button type="button" class="flex min-w-11 flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 disabled:opacity-50 dark:hover:bg-red-900/20 dark:hover:text-red-400" title="删除 Key" aria-label="删除 Key" :disabled="deleting" @click="confirmDelete(key)">
                <Icon name="trash" size="sm" />
                <span class="text-xs">删除</span>
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>

    <BaseDialog :show="showCreate" title="创建账号专用 Key" width="normal" @close="closeCreate">
      <form class="space-y-4" @submit.prevent="createKey">
        <div>
          <label class="input-label">Key 名称</label>
          <input v-model.trim="form.name" class="input" maxlength="80" required placeholder="例如：我的 Plus 套餐" />
        </div>
        <div>
          <label class="input-label">绑定套餐</label>
          <select v-model.number="form.packageId" class="input" required>
            <option v-for="pkg in activePackages" :key="pkg.id" :value="pkg.id">{{ pkg.name }} · {{ pkg.accounts.length }} 个账号 · 到期 {{ formatDate(pkg.expires_at) }}</option>
          </select>
          <p class="input-hint">分组、代理 IP 和账号归属由管理员管理，用户端不能选择其他分组。</p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="closeCreate">取消</button>
          <button class="btn btn-primary" type="button" :disabled="creating || !form.name || !form.packageId" @click="createKey">{{ creating ? '创建中...' : '创建 Key' }}</button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog :show="Boolean(deleteTarget)" title="删除账号专用 Key" :message="`删除后 ${deleteTarget?.name || '该 Key'} 将不能继续使用，确认删除吗？`" :confirm-text="deleting ? '删除中...' : '删除'" :confirm-loading="deleting" danger @cancel="deleteTarget = null" @confirm="deleteKey">
      <p v-if="deleteError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ deleteError }}</p>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { authAPI } from '@/api/auth'
import { getPublicGatewayBaseURL } from '@/api/url'
import { userGroupsAPI } from '@/api/groups'
import { listAccountPackages, type AccountBusinessPackage } from '@/api/accountBusiness'
import { buildCcSwitchImportDeeplink, CC_SWITCH_USAGE_SCRIPT } from '@/utils/ccswitchImport'
import { maskApiKey } from '@/utils/maskApiKey'
import type { ApiKey, Group, PublicSettings } from '@/types'

const loading = ref(true)
const error = ref('')
const allKeys = ref<ApiKey[]>([])
const packages = ref<AccountBusinessPackage[]>([])
const groups = ref<Group[]>([])
const showCreate = ref(false)
const creating = ref(false)
const publicSettings = ref<PublicSettings | null>(null)
const settingsReady = ref(false)
const deleting = ref(false)
const deleteError = ref('')
let importTimer: ReturnType<typeof setTimeout> | undefined
const deleteTarget = ref<ApiKey | null>(null)
const form = reactive({ name: '', packageId: 0 })

const activePackages = computed(() => packages.value.filter((pkg) => pkg.status === 'active' && pkg.core_group_id && pkg.accounts.length > 0))
const authorizedGroupIds = computed(() => new Set(activePackages.value.map((pkg) => Number(pkg.core_group_id))))
const visibleKeys = computed(() => allKeys.value.filter((key) => key.group_id != null && authorizedGroupIds.value.has(Number(key.group_id))))
const canCreate = computed(() => activePackages.value.length > 0)
const authorizedAccountCount = computed(() => activePackages.value.reduce((total, pkg) => total + pkg.accounts.length, 0))

const formatDate = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN') : '-'
const statusLabel = (status: string) => ({ active: '正常', inactive: '已停用', expired: '已过期', quota_exhausted: '额度耗尽' }[status] || status)
const packageName = (groupID: number | null) => {
  const pkg = activePackages.value.find((item) => Number(item.core_group_id) === Number(groupID))
  if (pkg) return pkg.name
  const group = groups.value.find((item) => item.id === groupID)
  return group?.name || '已授权套餐'
}
const expiryDays = (pkg: AccountBusinessPackage) => {
  if (!pkg.expires_at) return undefined
  return Math.max(1, Math.ceil((new Date(pkg.expires_at).getTime() - Date.now()) / 86400000))
}

const load = async () => {
  loading.value = true
  error.value = ''
  try {
    const [packageItems, keyPage, availableGroups] = await Promise.all([
      listAccountPackages(),
      keysAPI.list(1, 100, { sort_by: 'created_at', sort_order: 'desc' }),
      userGroupsAPI.getAvailable()
    ])
    packages.value = packageItems
    allKeys.value = keyPage.items
    groups.value = availableGroups
    if (!form.packageId) form.packageId = activePackages.value[0]?.id || 0
  } catch (err: any) {
    error.value = err?.message || '账号套餐 Key 加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

const closeCreate = () => {
  showCreate.value = false
  form.name = ''
  form.packageId = activePackages.value[0]?.id || 0
}

const createKey = async () => {
  if (creating.value || !form.name.trim()) return
  const pkg = activePackages.value.find((item) => item.id === form.packageId)
  if (!pkg?.core_group_id) return
  creating.value = true
  try {
    await keysAPI.create(form.name.trim(), pkg.core_group_id, undefined, [], [], undefined, expiryDays(pkg))
    closeCreate()
    await load()
  } catch (err: any) {
    error.value = err?.message || 'Key 创建失败，请确认套餐仍在有效期内'
  } finally {
    creating.value = false
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await authAPI.getPublicSettings()
  } catch {
    // Match the standard keys page's public gateway fallback.
    publicSettings.value = null
  } finally {
    settingsReady.value = true
  }
}

const importToCcswitch = (key: ApiKey) => {
  if (!settingsReady.value || deleting.value) return
  error.value = ''
  const platform = key.group?.platform || groups.value.find(group => group.id === key.group_id)?.platform || 'openai'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl: publicSettings.value?.api_base_url || getPublicGatewayBaseURL(),
    platform,
    clientType: platform === 'gemini' ? 'gemini' : 'claude',
    providerName: (publicSettings.value?.site_name || 'sub2api').trim() || 'sub2api',
    apiKey: key.key,
    usageScript: CC_SWITCH_USAGE_SCRIPT
  })
  const notInstalled = '未能打开 CCS，请确认已安装 CC-Switch 并注册导入协议。'
  clearTimeout(importTimer)
  try {
    window.open(deeplink, '_self')
    importTimer = setTimeout(() => {
      if (document.hasFocus()) error.value = notInstalled
    }, 100)
  } catch {
    error.value = notInstalled
  }
}

const confirmDelete = (key: ApiKey) => {
  if (deleting.value) return
  deleteError.value = ''
  deleteTarget.value = key
}
const deleteKey = async () => {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    await keysAPI.delete(deleteTarget.value.id)
    deleteTarget.value = null
    await load()
  } catch (err: any) {
    deleteError.value = err?.message || 'Key 删除失败'
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  void load()
  void loadPublicSettings()
})
onUnmounted(() => clearTimeout(importTimer))
</script>
