<script setup lang="ts">
import { computed } from 'vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import type { ChannelAccount } from '@/api/exclusiveChannels'
const props = defineProps<{ account: ChannelAccount }>()
const platforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok'] as const
const types = ['oauth', 'setup-token', 'apikey', 'upstream', 'bedrock', 'service_account'] as const
const platform = computed(() => platforms.find(p => p === props.account.platform))
const type = computed(() => types.find(t => t === props.account.type))
</script>
<template>
  <div class="exclusive-account-badge">
    <PlatformTypeBadge v-if="platform && type" :platform="platform" :type="type" :plan-type="account.plan_type" />
    <span v-else>{{ account.platform || '平台未知' }} · {{ account.type || '类型未知' }}<template v-if="account.plan_type"> · {{ account.plan_type }}</template></span>
  </div>
</template>
<style scoped>
.exclusive-account-badge { margin: 6px 0 10px; font-size: 12px; overflow-wrap: anywhere; }
</style>
