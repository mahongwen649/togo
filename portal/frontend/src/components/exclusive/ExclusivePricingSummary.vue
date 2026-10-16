<script setup lang="ts">
import type { ExclusiveChannel } from '@/api/exclusiveChannels'
defineProps<{ channel: ExclusiveChannel }>()
const price = (n?: number | null) => n == null ? '默认' : Number((n * 1e6).toPrecision(12)).toLocaleString('zh-CN', { maximumFractionDigits: 8 })
</script>
<template>
  <div class="pricing-summary">
    <div class="image-price-line"><span>图片 · 按张计费</span><strong v-if="channel.pricing_synced !== false">¥{{ channel.image_price ?? 0.05 }}<small> / 张 × {{ channel.image_rate_multiplier ?? 1 }}</small></strong><span v-else class="pricing-pending">新价格待同步</span></div>
    <div v-if="channel.model_pricing?.length" class="model-prices"><table><caption>文本模型 · 元 / 百万 Token · × {{ channel.rate_multiplier ?? 1 }}</caption><thead><tr><th>模型</th><th>输入</th><th>输出</th><th>缓存读取</th></tr></thead><tbody><tr v-for="entry in channel.model_pricing" :key="entry.model"><th scope="row">{{ entry.model }}</th><td>{{ price(entry.input_price) }}</td><td>{{ price(entry.output_price) }}</td><td>{{ price(entry.cache_read_price) }}</td></tr></tbody></table></div>
    <div v-else class="default-text-price">文本模型使用默认价格 × {{ channel.rate_multiplier ?? 1 }}</div>
  </div>
</template>
<style scoped>
.pricing-summary { font-size:12px; padding:14px 0; border-top:1px solid var(--ex-border,#e3e7ea); }.image-price-line { display:flex; justify-content:space-between; align-items:baseline; flex-wrap:wrap; gap:8px; color:var(--ex-muted,#697582); }.image-price-line strong { color:#2563b4; font-size:16px; font-variant-numeric:tabular-nums; }.image-price-line small { font-size:11px; font-weight:400; color:var(--ex-muted,#697582); }.default-text-price { font-size:11px; color:var(--ex-muted,#697582); margin-top:8px; }.pricing-pending { color:#996515; }.model-prices { margin-top:12px; max-height:220px; overflow:auto; }table { width:100%; table-layout:fixed; border-collapse:collapse; font-size:11px; }caption { text-align:left; color:var(--ex-muted,#697582); padding-bottom:8px; }th,td { text-align:right; font-weight:400; padding:6px 3px; border-bottom:1px solid var(--ex-border,#e3e7ea); overflow-wrap:anywhere; }thead th { color:var(--ex-muted,#697582); }th:first-child { text-align:left; width:40%; }
.dark .image-price-line strong { color:#85b6fa; }.dark .pricing-pending { color:#e6b765; }
</style>
