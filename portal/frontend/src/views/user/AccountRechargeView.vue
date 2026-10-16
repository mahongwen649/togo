<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="page-header flex flex-wrap items-end justify-between gap-3">
        <div><h1 class="page-title">购买卡密</h1><p class="page-description">在第三方商城购买账号系统卡密，购买后返回“卡密兑换”完成入账。</p></div>
        <a v-if="externalUrl" :href="externalUrl" target="_blank" rel="noopener" class="btn btn-secondary">新窗口打开</a>
      </div>
      <div v-if="loading" class="card flex min-h-[520px] items-center justify-center"><LoadingSpinner /></div>
      <div v-else-if="!embedUrl" class="card p-8 text-center"><h2 class="text-lg font-semibold text-gray-900 dark:text-white">充值入口尚未配置</h2><p class="mt-2 text-sm text-gray-500 dark:text-dark-400">请联系管理员配置第三方卡密商城地址。</p></div>
      <div v-else class="card overflow-hidden p-2">
        <iframe :src="embedUrl" title="第三方充值商城" class="h-[calc(100vh-180px)] min-h-[620px] w-full rounded-lg border-0 bg-white" />
      </div>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { apiClient } from '@/api/client'
const embedUrl = ref(''); const externalUrl = ref(''); const loading = ref(true)
onMounted(async () => { try { const { data } = await apiClient.get<{ url: string }>('/account-market/recharge'); embedUrl.value = data.url; externalUrl.value = data.url } finally { loading.value = false } })
</script>
