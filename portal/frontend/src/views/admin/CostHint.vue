<template>
  <p class="flex flex-wrap items-center gap-x-1 text-xs tabular-nums">
    <span class="font-medium text-green-600 dark:text-green-400">¥{{ formatCost(customerAmountCny(actual)) }}</span>
    <span class="text-gray-300 dark:text-gray-600">/</span>
    <span class="font-medium text-orange-500 dark:text-orange-400">¥{{ formatCost(upstreamCostCny(account)) }}</span>
    <span class="text-gray-300 dark:text-gray-600">/</span>
    <span class="text-gray-400 dark:text-gray-500">${{ formatCost(standard) }}</span>
  </p>
</template>

<script setup lang="ts">
import { customerAmountCny, upstreamCostCny } from '@/utils/portalCurrency'

defineProps<{
  actual: number
  account: number
  standard: number
}>()

function formatCost(value: number | null | undefined): string {
  const numberValue = Number(value)
  const safeValue = Number.isFinite(numberValue) ? numberValue : 0
  if (safeValue >= 1000) return `${(safeValue / 1000).toFixed(2)}K`
  if (safeValue >= 1) return safeValue.toFixed(2)
  if (safeValue >= 0.01) return safeValue.toFixed(3)
  return safeValue.toFixed(4)
}
</script>
