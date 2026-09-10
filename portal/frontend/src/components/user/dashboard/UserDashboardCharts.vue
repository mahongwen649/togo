<template>
  <div class="space-y-6">
    <ChartRangeToolbar
      :start-date="startDate"
      :end-date="endDate"
      :granularity="granularity"
      :granularity-options="granularityOptions"
      :loading="loading"
      show-refresh
      @update:start-date="$emit('update:startDate', $event)"
      @update:end-date="$emit('update:endDate', $event)"
      @update:granularity="$emit('update:granularity', String($event))"
      @range-change="$emit('dateRangeChange', $event)"
      @granularity-change="$emit('granularityChange')"
      @refresh="$emit('refresh')"
    />

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <ModelDistributionChart
        :model-stats="models"
        :loading="loading"
        :show-account-cost="false"
      />
      <TokenUsageTrend :trend-data="trend" :loading="loading" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ChartRangeToolbar from '@/components/common/ChartRangeToolbar.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import type { TrendDataPoint, ModelStat } from '@/types'

defineProps<{
  loading: boolean
  startDate: string
  endDate: string
  granularity: string
  trend: TrendDataPoint[]
  models: ModelStat[]
}>()

defineEmits([
  'update:startDate',
  'update:endDate',
  'update:granularity',
  'dateRangeChange',
  'granularityChange',
  'refresh',
])

const { t } = useI18n()
const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('dashboard.day') },
  { value: 'hour', label: t('dashboard.hour') },
])
</script>
