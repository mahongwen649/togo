<template>
  <AppLayout>
    <div class="space-y-4">
      <header class="page-header flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 class="page-title">购买卡密</h1>
          <p class="page-description">选购适用于不同业务类型的卡密。</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/redeem" class="btn btn-secondary">兑换中转卡密</RouterLink>
          <RouterLink to="/account-business/balance/redeem" class="btn btn-primary">兑换账号套餐卡密</RouterLink>
        </div>
      </header>
      <iframe title="云猫卡密店铺" src="https://wzyp.cn/shop/D8EXRALQ" class="block h-[calc(100dvh-12rem)] min-h-[480px] w-full border-0 bg-white" referrerpolicy="strict-origin-when-cross-origin" />
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
</script>
