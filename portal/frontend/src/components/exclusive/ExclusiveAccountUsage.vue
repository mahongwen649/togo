<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { exclusiveChannels, type ChannelAccount } from '@/api/exclusiveChannels'
import Icon from '@/components/icons/Icon.vue'
import ExclusiveAccountBadge from './ExclusiveAccountBadge.vue'
import ExclusivePageTest from './ExclusivePageTest.vue'
const props = defineProps<{ accounts: ChannelAccount[]; channelId?: number; canTest?: boolean }>()
const emit = defineEmits<{ refreshed: [account: ChannelAccount] }>()
const updates = ref<Record<number, ChannelAccount>>({})
const pending = ref<Record<number, boolean>>({})
const errors = ref<Record<number, string>>({})
const accounts = computed(() => props.accounts.map(a => updates.value[a.id] || a))
watch(() => props.accounts, () => { updates.value = {} })
const windows = [{ key: 'five_hour_remaining', reset: 'five_hour_resets_at', label: '5 小时窗口' }, { key: 'weekly_remaining', reset: 'weekly_resets_at', label: '7 天窗口' }] as const
const resetLabel = (value?: string | null) => value ? `${new Date(value).toLocaleString('zh-CN', { hour12: false })} 重置` : '重置时间未知'
async function refresh(id: number) {
  if (!props.channelId || pending.value[id]) return
  pending.value[id] = true; errors.value[id] = ''
  try {
    const account = await exclusiveChannels.refreshUsage(props.channelId, id)
    updates.value[id] = account
    emit('refreshed', account)
  } catch (e) {
    const err = e as { response?: { data?: { message?: string } }; message?: string }
    errors.value[id] = err.response?.data?.message || err.message || '刷新失败，请稍后重试'
  } finally { pending.value[id] = false }
}
</script>
<template>
  <div class="ex-accounts">
    <div v-for="a in accounts" :key="a.id" class="ex-account">
      <div class="ex-account-heading"><strong>{{ a.name }}</strong><div class="ex-account-actions"><span :class="a.available ? 'ex-ok' : 'ex-warning'">{{ a.available ? '可用' : '暂不可用' }}</span><button v-if="channelId && a.type !== 'apikey'" type="button" class="ex-refresh" :disabled="pending[a.id]" :aria-label="`刷新 ${a.name} 用量`" :title="`刷新 ${a.name} 用量`" @click="refresh(a.id)"><Icon name="refresh" size="sm" :class="{ 'animate-spin': pending[a.id] }" /></button></div></div>
      <ExclusiveAccountBadge :account="a" />
      <div v-if="a.type === 'apikey'" class="ex-api-key-usage">API Key 账号不提供订阅额度窗口</div>
      <div v-else class="ex-quotas"><div v-for="window in windows" :key="window.key" :data-level="a[window.key] == null ? 'unknown' : a[window.key]! <= 10 ? 'low' : 'normal'"><div class="ex-quota-label"><span>{{ window.label }}</span><b>{{ a[window.key] == null ? '暂无数据' : `已用 ${(100 - a[window.key]!).toFixed(0)}%` }}</b></div><progress v-if="a[window.key] != null" :value="100 - a[window.key]!" max="100" :aria-label="`${a.name} ${window.label}已用`" :class="{ weekly: window.key === 'weekly_remaining' }"></progress><div v-else class="ex-no-quota"></div><div v-if="a[window.key] != null" class="ex-quota-remaining">剩余 {{ a[window.key]!.toFixed(0) }}%</div><div class="ex-quota-error">{{ resetLabel(a[window.reset]) }}</div></div></div>
      <div v-if="errors[a.id]" class="ex-refresh-error" role="alert">{{ errors[a.id] }}</div>
      <div v-if="a.error" class="ex-quota-error">{{ a.error }}</div>
      <div v-if="a.type !== 'apikey'" class="ex-quota-error">{{ a.observed_at ? `额度更新于 ${new Date(a.observed_at).toLocaleString('zh-CN', { hour12: false })}` : '额度更新时间未知' }}</div>
      <ExclusivePageTest v-if="channelId" :channel-id="channelId" :account="a" :can-test="canTest" compact @changed="$emit('refreshed', a)" />
    </div>
  </div>
</template>
<style scoped>
.ex-account { padding: 10px 0; border-bottom: 1px solid var(--ex-border, #e3e7ea); }
.ex-account-actions { display:flex; align-items:center; gap:10px; }.ex-refresh { display:grid; place-items:center; width:28px; height:28px; border:1px solid var(--ex-border,#e3e7ea); border-radius:4px; color:#107e6c; }.ex-refresh:disabled { opacity:.5; cursor:wait; }.ex-refresh-error { font-size:12px; color:#a66a19; margin-top:8px; }
.ex-account-heading { display: flex; justify-content: space-between; gap: 10px; font-size: 12px; margin-bottom: 10px; }
.ex-account-heading strong { font-weight: 500; overflow-wrap: anywhere; }
.ex-api-key-usage { padding:8px 10px; border:1px solid var(--ex-border,#e3e7ea); border-radius:6px; color:var(--ex-muted,#6b7280); font-size:12px; }
.ex-ok { color: #107e6c; flex-shrink: 0; }.ex-warning { color: #a66a19; flex-shrink: 0; }
.ex-quotas { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 18px; }
.ex-quota-label { display: flex; justify-content: space-between; gap: 5px; margin-bottom: 7px; font-size: 11px; color: var(--ex-muted, #6b7280); }
.ex-quota-label b { font-weight: 500; white-space: nowrap; }
progress,.ex-no-quota { display: block; appearance: none; width: 100%; height: 6px; background: #e8edef; border: none; border-radius: 3px; overflow: hidden; }
progress::-webkit-progress-bar { background: #e8edef; }progress::-webkit-progress-value { background: #107e6c; }progress::-moz-progress-bar { background: #107e6c; }
progress.weekly::-webkit-progress-value { background: #4c87c5; }progress.weekly::-moz-progress-bar { background: #4c87c5; }
.ex-quota-error { font-size: 11px; color: var(--ex-muted, #6b7280); margin-top: 8px; }
.ex-quota-error { overflow-wrap:anywhere; }.ex-quota-label { flex-wrap:wrap; }
.dark .ex-ok { color: #7acdbb; }.dark .ex-warning { color: #e6b765; }
.ex-account-heading strong { font-size:13px; font-weight:600; }.ex-account-actions { flex-shrink:0; }
.ex-quota-remaining { color:#107e6c; font-size:13px; font-weight:600; margin-top:8px; font-variant-numeric:tabular-nums; }.ex-quotas>div:nth-child(2) .ex-quota-remaining { color:#2563b4; }
.ex-quotas>div[data-level='low'] .ex-quota-remaining { color:#a66a19; }.ex-quotas>[data-level='low'] progress::-webkit-progress-value { background:#cc921f; }.ex-quotas>[data-level='low'] progress::-moz-progress-bar { background:#cc921f; }
.ex-quota-error { line-height:1.6; }.ex-quota-label b { color:var(--ex-muted,#6b7280); }.ex-refresh { transition:background-color .15s ease; }.ex-refresh:hover:not(:disabled) { background:#e7f5f0; }.ex-refresh:focus-visible { outline:2px solid #3980cf; outline-offset:2px; }
.dark .ex-quota-remaining { color:#7acdbb; }.dark .ex-quotas>div:nth-child(2) .ex-quota-remaining { color:#85b6fa; }.dark .ex-quotas>div[data-level='low'] .ex-quota-remaining { color:#e6b765; }.dark .ex-refresh:hover:not(:disabled) { background:#2b423b; }
@media(prefers-reduced-motion:reduce){.ex-refresh { transition:none; }}
</style>
