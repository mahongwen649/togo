<template>
  <div class="card" :class="isMobileH5 ? 'p-3' : 'p-4'">
    <div
      :class="isMobileH5
        ? 'grid grid-cols-[minmax(0,1fr)_7rem] items-end gap-2'
        : 'flex flex-wrap items-center gap-4'"
    >
      <div :class="isMobileH5 ? 'min-w-0 space-y-1.5' : 'flex items-center gap-2'">
        <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.dashboard.timeRange') }}:
        </span>
        <DateRangePicker
          class="min-w-0"
          :start-date="startDate"
          :end-date="endDate"
          @update:start-date="$emit('update:startDate', $event)"
          @update:end-date="$emit('update:endDate', $event)"
          @change="$emit('rangeChange', $event)"
        />
      </div>

      <button
        v-if="showRefresh && !isMobileH5"
        type="button"
        class="btn btn-secondary"
        :disabled="loading"
        @click="$emit('refresh')"
      >
        {{ t('common.refresh') }}
      </button>

      <div :class="isMobileH5 ? 'min-w-0 space-y-1.5' : 'ml-auto flex items-center gap-2'">
        <span class="block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.dashboard.granularity') }}:
        </span>
        <div :class="isMobileH5 ? 'w-full' : 'w-28'">
          <Select
            :model-value="granularity"
            :options="granularityOptions"
            @update:model-value="$emit('update:granularity', $event)"
            @change="$emit('granularityChange')"
          />
        </div>
      </div>

      <button
        v-if="showRefresh && isMobileH5"
        type="button"
        class="btn btn-secondary col-span-2 w-full"
        :disabled="loading"
        @click="$emit('refresh')"
      >
        {{ t('common.refresh') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DateRangePicker from './DateRangePicker.vue'
import Select, { type SelectOption } from './Select.vue'
import { useDeviceMode } from '@/composables/useDeviceMode'

withDefaults(defineProps<{
  startDate: string
  endDate: string
  granularity: string
  granularityOptions: SelectOption[]
  loading?: boolean
  showRefresh?: boolean
}>(), {
  loading: false,
  showRefresh: false,
})

defineEmits<{
  'update:startDate': [value: string]
  'update:endDate': [value: string]
  'update:granularity': [value: string | number | boolean | null]
  rangeChange: [value: { startDate: string; endDate: string; preset: string | null }]
  granularityChange: []
  refresh: []
}>()

const { t } = useI18n()
const { isMobileH5 } = useDeviceMode()
</script>
