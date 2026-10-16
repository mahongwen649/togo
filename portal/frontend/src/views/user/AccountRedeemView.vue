<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header">
        <h1 class="page-title">账号余额中心</h1>
        <p class="page-description">账号套餐余额独立于中转站余额，仅用于购买和租赁账号。</p>
      </header>
      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700">
        <button class="border-b-2 border-primary-500 px-4 py-3 text-sm font-medium text-primary-600">卡密兑换</button>
        <button class="px-4 py-3 text-sm text-gray-500">充值记录</button>
      </div>
      <section class="card max-w-2xl space-y-5 p-6">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">兑换卡密</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">在第三方商城购买卡密后，返回这里输入并兑换账号套餐余额。</p>
        </div>
        <label class="block text-sm font-medium text-gray-700 dark:text-dark-200">卡密</label>
        <input class="input w-full" placeholder="请输入卡密" />
        <button class="btn btn-primary w-full">立即兑换</button>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
</script>
