<template>
  <router-link
    :to="path"
    class="group flex items-center gap-3 rounded-lg bg-gray-50 p-3 text-left transition-colors hover:bg-primary-50 dark:bg-dark-800/50 dark:hover:bg-primary-900/20"
  >
    <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg" :class="colorClass">
      <Icon :name="icon" size="md" :stroke-width="2" />
    </span>
    <span class="min-w-0 flex-1">
      <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ title }}</span>
      <span class="block text-xs text-gray-500 dark:text-gray-400">{{ description }}</span>
    </span>
    <Icon name="chevronRight" size="sm" class="text-gray-400 group-hover:text-primary-500" />
  </router-link>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

type DashboardQuickActionIconName = 'grid' | 'creditCard'

defineProps<{
  path: string
  icon: DashboardQuickActionIconName
  colorClass: string
  title: string
  description: string
}>()
</script>
