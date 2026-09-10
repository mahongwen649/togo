<template>
  <AppLayout>
    <MonitorHero
      :overall-status="overallStatus"
      :interval-seconds="DEFAULT_INTERVAL_SECONDS"
      :window="currentWindow"
      :loading="loading"
      :auto-refresh="autoRefresh"
      @update:window="handleWindowChange"
      @refresh="manualReload"
    />

    <section class="mb-5 space-y-4">
      <div class="relative max-w-md">
        <Icon
          name="search"
          size="sm"
          class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
        />
        <input
          v-model="searchQuery"
          type="search"
          :placeholder="t('channelStatus.searchPlaceholder')"
          class="h-10 w-full rounded-xl border border-gray-200 bg-white pl-9 pr-3 text-sm text-gray-900 outline-none transition-colors placeholder:text-gray-400 focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/15 dark:border-dark-700 dark:bg-dark-900 dark:text-white dark:placeholder:text-dark-400"
        >
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          data-test-provider="all"
          class="rounded-xl border px-3 py-1.5 text-sm transition-colors"
          :class="selectedProvider === '' ? activePillClass : idlePillClass"
          @click="selectedProvider = ''"
        >
          {{ t('channelStatus.allProviders') }}
        </button>
        <button
          v-for="provider in providers"
          :key="provider"
          type="button"
          :data-test-provider="provider"
          class="rounded-xl border px-3 py-1.5 text-sm transition-colors"
          :class="selectedProvider === provider ? activePillClass : idlePillClass"
          @click="selectedProvider = provider"
        >
          {{ providerLabel(provider) }}
        </button>
      </div>

      <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
        <div class="flex flex-wrap items-center gap-2">
          <span class="mr-1 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
            {{ t('channelStatus.sortLabel') }}
          </span>
          <button
            v-for="option in sortOptions"
            :key="option.value"
            type="button"
            :data-test-sort="option.value"
            class="rounded-xl border px-3 py-1.5 text-sm transition-colors"
            :class="sortMode === option.value ? activePillClass : idlePillClass"
            @click="sortMode = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <span class="mr-1 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
            {{ t('channelStatus.windowLabel') }}
          </span>
          <button
            v-for="option in windowOptions"
            :key="option.value"
            type="button"
            class="rounded-xl border px-3 py-1.5 text-sm transition-colors"
            :class="currentWindow === option.value ? activePillClass : idlePillClass"
            @click="handleWindowChange(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div
        v-if="lastUpdatedAt"
        class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400"
      >
        <span>{{ overallStatus.toUpperCase() }}</span>
        <span>|</span>
        <span>{{ t('channelStatus.lastUpdated', { time: lastUpdatedAt }) }}</span>
      </div>
    </section>

    <MonitorCardGrid
      :items="visibleItems"
      :window="currentWindow"
      :countdown-seconds="countdown"
      :loading="loading"
      :detail-cache="detailCache"
      @card-click="openDetail"
    />

    <MonitorDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.id ?? null"
      :title="detailTitle"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listChannelMonitorViews,
  status as fetchChannelMonitorDetail,
  type UserMonitorView,
  type UserMonitorDetail,
} from '@/api/channelMonitor'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import MonitorHero, {
  type MonitorWindow,
  type OverallStatus,
} from '@/components/user/monitor/MonitorHero.vue'
import MonitorCardGrid from '@/components/user/monitor/MonitorCardGrid.vue'
import MonitorDetailDialog from '@/components/user/MonitorDetailDialog.vue'
import { DEFAULT_INTERVAL_SECONDS, STATUS_OPERATIONAL } from '@/constants/channelMonitor'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const { t } = useI18n()
const appStore = useAppStore()
const { providerLabel } = useChannelMonitorFormat()

// ── State ──
const items = ref<UserMonitorView[]>([])
const loading = ref(false)
const currentWindow = ref<MonitorWindow>('7d')
type MonitorSortMode = 'custom' | 'group' | 'model' | 'availability' | 'latency'
const searchQuery = ref('')
const selectedProvider = ref('')
const sortMode = ref<MonitorSortMode>('custom')
const lastUpdatedAt = ref<string | null>(null)
const detailCache = reactive<Record<number, UserMonitorDetail>>({})
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)

let abortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'channel-status-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => reload(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

// ── Computed ──
const overallStatus = computed<OverallStatus>(() => {
  if (items.value.length === 0) return 'operational'
  for (const it of items.value) {
    if (it.primary_status === 'failed' || it.primary_status === 'error') return 'degraded'
    if (it.primary_status !== STATUS_OPERATIONAL) return 'degraded'
  }
  return 'operational'
})

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('channelStatus.detailTitle')
})

const activePillClass = 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-500/30 dark:bg-cyan-500/15 dark:text-cyan-300'
const idlePillClass = 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300 dark:hover:bg-dark-800'

const sortOptions = computed<{ value: MonitorSortMode; label: string }[]>(() => [
  { value: 'custom', label: t('channelStatus.sort.custom') },
  { value: 'group', label: t('channelStatus.sort.group') },
  { value: 'model', label: t('channelStatus.sort.model') },
  { value: 'availability', label: t('channelStatus.sort.availability') },
  { value: 'latency', label: t('channelStatus.sort.latency') },
])

const windowOptions = computed<{ value: MonitorWindow; label: string }[]>(() => [
  { value: '7d', label: t('channelStatus.windowTab.7d') },
  { value: '15d', label: t('channelStatus.windowTab.15d') },
  { value: '30d', label: t('channelStatus.windowTab.30d') },
])

const providers = computed(() => {
  return Array.from(new Set(items.value.map(item => item.provider))).sort((a, b) =>
    String(a).localeCompare(String(b))
  )
})

const visibleItems = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  const filtered = items.value.filter((item) => {
    if (selectedProvider.value && item.provider !== selectedProvider.value) return false
    if (!query) return true
    const haystack = [
      item.name,
      item.provider,
      item.group_name,
      item.primary_model,
      ...item.extra_models.map(model => model.model),
    ].join(' ').toLowerCase()
    return haystack.includes(query)
  })

  const sorted = [...filtered]
  switch (sortMode.value) {
    case 'group':
      sorted.sort((a, b) => compareText(a.group_name, b.group_name) || compareText(a.name, b.name))
      break
    case 'model':
      sorted.sort((a, b) => compareText(a.primary_model, b.primary_model) || compareText(a.name, b.name))
      break
    case 'availability':
      sorted.sort((a, b) => b.availability_7d - a.availability_7d || compareText(a.name, b.name))
      break
    case 'latency':
      sorted.sort((a, b) => compareNullableLatency(a.primary_latency_ms, b.primary_latency_ms) || compareText(a.name, b.name))
      break
    case 'custom':
    default:
      break
  }
  return sorted
})

function compareText(a: string, b: string): number {
  return a.localeCompare(b)
}

function compareNullableLatency(a: number | null, b: number | null): number {
  if (a === null && b === null) return 0
  if (a === null) return 1
  if (b === null) return -1
  return a - b
}

// ── Loaders ──
async function reload(silent = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  try {
    const res = await listChannelMonitorViews({ signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
    lastUpdatedAt.value = new Date().toLocaleTimeString()
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.loadError')))
  } finally {
    if (abortController === ctrl) {
      if (!silent) loading.value = false
      countdown.value = DEFAULT_INTERVAL_SECONDS
      abortController = null
    }
  }
}

async function manualReload() {
  await reload(false)
  // After base reload, refresh any cached detail records so non-7d availability
  // values stay in sync without forcing the user to switch tabs again.
  if (currentWindow.value !== '7d') {
    await Promise.all(items.value.map(it => loadDetail(it.id, true)))
  }
}

async function loadDetail(id: number, force = false) {
  if (!force && detailCache[id]) return
  try {
    detailCache[id] = await fetchChannelMonitorDetail(id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
  }
}

async function ensureDetailsForWindow() {
  if (currentWindow.value === '7d') return
  await Promise.all(items.value.map(it => loadDetail(it.id)))
}

// ── Handlers ──
async function handleWindowChange(value: MonitorWindow) {
  currentWindow.value = value
  await ensureDetailsForWindow()
}

function openDetail(row: UserMonitorView) {
  detailTarget.value = row
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

watch(items, () => {
  void ensureDetailsForWindow()
})

watch(
  () => appStore.cachedPublicSettings?.channel_monitor_enabled,
  (enabled) => {
    if (enabled === false) autoRefresh.stop()
    else if (autoRefresh.enabled.value) autoRefresh.start()
  },
)

onMounted(() => {
  void reload(false)
  if (appStore.cachedPublicSettings?.channel_monitor_enabled !== false) {
    autoRefresh.setEnabled(true)
  }
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})
</script>


