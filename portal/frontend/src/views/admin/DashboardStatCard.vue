<template>
  <div class="card min-w-0" :class="isMobileH5 ? 'p-2' : 'p-4'">
    <div class="flex items-center" :class="isMobileH5 ? 'gap-2' : 'gap-3'">
      <div class="shrink-0 rounded-lg" :class="[iconClass, isMobileH5 ? 'p-1.5' : 'p-2']">
        <Icon :name="icon" size="md" :stroke-width="2" />
      </div>
      <div class="min-w-0 flex-1">
        <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ label }}</p>
        <p class="break-words font-bold leading-tight tabular-nums" :class="[valueClass, isMobileH5 ? 'text-lg' : 'text-xl']" :title="value">{{ value }}</p>
        <slot name="hint">
          <p class="text-xs" :class="hintClass">{{ hint }}</p>
        </slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import { useDeviceMode } from '@/composables/useDeviceMode'

const { isMobileH5 } = useDeviceMode()

type DashboardIconName = 'key' | 'server' | 'chart' | 'userPlus' | 'cube' | 'database' | 'clock' | 'dollar' | 'users' | 'calculator' | 'trendingUp'

defineProps<{
  icon: DashboardIconName
  iconClass: string
  label: string
  value: string
  hint?: string
  valueClass?: string
  hintClass?: string
}>()
</script>
