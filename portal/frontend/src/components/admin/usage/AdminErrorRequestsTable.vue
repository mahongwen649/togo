<template>
  <div class="flex min-h-0 flex-col">
    <IpGeoBatchToolbar :ips="rows.map((row) => row.client_ip)" @failed="emit('ipGeoBatchFailed')" />

    <DataTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      clickable-rows
      server-side-sort
      default-sort-key="created_at"
      default-sort-order="desc"
      @sort="onSort"
      @rowClick="openDetail"
    >
      <template #cell-user="{ row }">
        <div v-if="row.user_id" class="text-sm">
          <button
            v-if="row.user_email"
            class="font-medium text-primary-600 underline decoration-dashed underline-offset-2 dark:text-primary-400"
            :title="t('admin.usage.clickToViewBalance')"
            @click.stop="emit('userClick', row.user_id)"
          >
            {{ row.user_email }}
          </button>
          <span v-else>#{{ row.user_id }}</span>
        </div>
        <span v-else class="text-gray-400">-</span>
      </template>

      <template #cell-api_key="{ row }">
        <span>{{ row.api_key_name || (row.api_key_id ? `#${row.api_key_id}` : '-') }}</span>
        <span v-if="row.api_key_deleted" class="ml-1 text-xs text-rose-500">
          {{ t('usage.errors.keyDeleted') }}
        </span>
      </template>

      <template #cell-account="{ row }">
        <span>{{ row.account_name || (row.account_id ? `#${row.account_id}` : '-') }}</span>
      </template>

      <template #cell-model="{ row }">
        <div v-if="hasModelMapping(row)" class="space-y-0.5 text-xs">
          <div class="break-all font-medium">{{ row.requested_model }}</div>
          <div class="break-all text-gray-500"><span class="mr-0.5">↳</span>{{ row.upstream_model }}</div>
        </div>
        <span v-else>{{ displayModel(row) || '-' }}</span>
      </template>

      <template #cell-endpoint="{ row }">
        <div class="space-y-1 text-xs">
          <div class="break-all">
            <span class="text-gray-500">{{ t('usage.inbound') }}:</span>
            {{ row.inbound_endpoint || '-' }}
          </div>
          <div v-if="row.upstream_endpoint" class="break-all">
            <span class="text-gray-500">{{ t('usage.upstream') }}:</span>
            {{ row.upstream_endpoint }}
          </div>
        </div>
      </template>

      <template #cell-group="{ row }">
        <span v-if="row.group_id" class="inline-flex rounded bg-indigo-100 px-2 py-0.5 text-xs text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200">
          {{ row.group_name || `#${row.group_id}` }}
        </span>
        <span v-else class="text-gray-400">-</span>
      </template>

      <template #cell-category="{ row }">
        {{ t(`usage.errors.categories.${mapErrorCategory(row.phase, row.type)}`) }}
      </template>

      <template #cell-status="{ row }">
        <span class="inline-flex rounded px-2 py-0.5 text-xs font-medium" :class="statusCodeBadgeClass(row.status_code)">
          {{ row.status_code || '-' }}
        </span>
      </template>

      <template #cell-message="{ row }">
        <span class="block max-w-[280px] truncate" :title="row.message">{{ smartMessage(row.message) || '-' }}</span>
      </template>

      <template #cell-created_at="{ row }">
        <span class="whitespace-nowrap">{{ formatDateTime(row.created_at) }}</span>
      </template>

      <template #cell-user_agent="{ row }">
        <span class="block max-w-[320px] truncate" :title="row.user_agent">{{ row.user_agent || '-' }}</span>
      </template>

      <template #cell-client_ip="{ row }">
        <div v-if="row.client_ip" @click.stop>
          <span class="font-mono text-xs">{{ row.client_ip }}</span>
          <IpGeoCell :ip="row.client_ip" />
        </div>
        <span v-else class="text-gray-400">-</span>
      </template>

      <template #empty>
        <EmptyState :message="t('usage.errors.empty')" />
      </template>
    </DataTable>

    <Pagination
      v-if="total > 0"
      :page="page"
      :page-size="pageSize"
      :total="total"
      @update:page="emit('update:page', $event)"
      @update:pageSize="emit('update:pageSize', $event)"
    />

    <BaseDialog
      :show="showDetail"
      :title="t('usage.errors.detail.title')"
      width="wide"
      :close-on-click-outside="true"
      @close="showDetail = false"
    >
      <div v-if="detailLoading" class="flex justify-center py-10">
        <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-200 border-t-primary-600"></div>
      </div>
      <div v-else-if="detail" class="space-y-4 text-sm">
        <div class="grid grid-cols-2 gap-x-5 gap-y-3">
          <DetailField :label="t('usage.errors.time')" :value="formatDateTime(detail.created_at)" />
          <DetailField :label="t('admin.usage.user')" :value="detail.user_email || valueById(detail.user_id)" />
          <DetailField :label="t('usage.apiKeyFilter')" :value="detail.api_key_name || valueById(detail.api_key_id)" />
          <DetailField :label="t('admin.usage.account')" :value="detail.account_name || valueById(detail.account_id)" />
          <DetailField :label="t('usage.model')" :value="displayModel(detail) || '-'" />
          <DetailField :label="t('admin.usage.group')" :value="detail.group_name || valueById(detail.group_id)" />
          <DetailField :label="t('usage.errors.status')" :value="String(detail.status_code || '-')" />
          <DetailField :label="t('usage.errors.category')" :value="t(`usage.errors.categories.${mapErrorCategory(detail.phase, detail.type)}`)" />
        </div>
        <div>
          <div class="font-medium text-gray-500">{{ t('usage.errors.message') }}</div>
          <div class="mt-1 break-all text-gray-900 dark:text-gray-100">{{ detail.message || '-' }}</div>
        </div>
        <div v-if="detailBody">
          <div class="font-medium text-gray-500">{{ t('usage.errors.detail.responseBody') }}</div>
          <pre class="mt-1 max-h-[45vh] overflow-auto whitespace-pre-wrap break-all rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs dark:border-dark-700 dark:bg-dark-900">{{ prettyBody(detailBody) }}</pre>
        </div>
      </div>
      <div v-else class="py-8 text-center text-sm text-gray-500">
        {{ t('usage.errors.detail.loadFailed') }}
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import IpGeoBatchToolbar from '@/components/common/IpGeoBatchToolbar.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminUsageAPI } from '@/api/admin/usage'
import type { AdminErrorRequest, AdminErrorRequestDetail } from '@/api/admin/usage'
import type { Column } from '@/components/common/types'
import { mapErrorCategory } from '@/utils/errorCategory'
import { mapErrorSortKey, statusCodeBadgeClass } from '@/utils/errorBadges'
import { formatDateTime } from '@/utils/format'

defineProps<{
  rows: AdminErrorRequest[]
  total: number
  loading: boolean
  page: number
  pageSize: number
}>()

const emit = defineEmits<{
  (event: 'update:page', value: number): void
  (event: 'update:pageSize', value: number): void
  (event: 'sort', key: string, order: 'asc' | 'desc'): void
  (event: 'userClick', userId: number): void
  (event: 'ipGeoBatchFailed'): void
}>()

const { t } = useI18n()

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.usage.user') },
  { key: 'api_key', label: t('usage.apiKeyFilter') },
  { key: 'account', label: t('admin.usage.account') },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'endpoint', label: t('usage.endpoint') },
  { key: 'group', label: t('admin.usage.group') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('usage.errors.status'), sortable: true },
  { key: 'message', label: t('usage.errors.message') },
  { key: 'created_at', label: t('usage.errors.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent') },
  { key: 'client_ip', label: 'IP' }
])

const showDetail = ref(false)
const detailLoading = ref(false)
const detail = ref<AdminErrorRequestDetail | null>(null)
const detailBody = computed(() =>
  detail.value?.upstream_error_detail || detail.value?.error_body || detail.value?.upstream_error_message || ''
)

const DetailField = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup(fieldProps) {
    return () => h('div', [
      h('div', { class: 'font-medium text-gray-500' }, fieldProps.label),
      h('div', { class: 'mt-0.5 break-all text-gray-900 dark:text-gray-100' }, fieldProps.value)
    ])
  }
})

function onSort(key: string, order: 'asc' | 'desc') {
  emit('sort', mapErrorSortKey(key), order)
}

async function openDetail(row: AdminErrorRequest) {
  showDetail.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await adminUsageAPI.getErrorDetail(row.id)
  } catch (error) {
    console.error('Failed to load admin error detail:', error)
  } finally {
    detailLoading.value = false
  }
}

function hasModelMapping(row: AdminErrorRequest): boolean {
  return Boolean(row.requested_model && row.upstream_model && row.requested_model !== row.upstream_model)
}

function displayModel(row: AdminErrorRequest): string {
  return row.upstream_model || row.requested_model || row.model || ''
}

function smartMessage(message: string): string {
  if (!message) return ''
  try {
    const parsed = JSON.parse(message)
    return String(parsed?.error?.message || parsed?.message || message)
  } catch {
    return message
  }
}

function valueById(id: number | null | undefined): string {
  return id ? `#${id}` : '-'
}

function prettyBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}
</script>
