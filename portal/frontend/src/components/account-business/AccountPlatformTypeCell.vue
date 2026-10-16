<template>
  <div class="flex min-w-0 flex-col gap-1">
    <div class="flex flex-wrap items-center gap-1">
      <PlatformTypeBadge
        v-if="platform && type"
        :platform="platform"
        :type="type"
        :auth-mode="authMode"
        :plan-type="planType"
        :privacy-mode="privacyMode"
        :subscription-expires-at="subscriptionExpiresAt"
      />
      <span v-else class="text-xs text-gray-500 dark:text-gray-400">
        {{ account.platform || '平台未知' }} · {{ account.type || '类型未知' }}
      </span>
    </div>
    <div
      v-if="compactMeta"
      :class="[
        'inline-flex items-center gap-1.5 pl-0.5 text-[11px] font-medium leading-4',
        compactMeta.className,
      ]"
      :title="compactTitle"
    >
      <span :class="['h-1.5 w-1.5 rounded-full', compactMeta.dotClass]" />
      <span>{{ compactMeta.label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import type { Account } from '@/types'

const props = defineProps<{ account: Account }>()
const { t } = useI18n()

const badgePlatforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok'] as const
const badgeTypes = ['oauth', 'setup-token', 'apikey', 'upstream', 'bedrock', 'service_account'] as const
type BadgePlatform = (typeof badgePlatforms)[number]
type BadgeType = (typeof badgeTypes)[number]

const platform = computed<BadgePlatform | null>(() =>
  badgePlatforms.find((value) => value === props.account.platform) || null
)
const type = computed<BadgeType | null>(() =>
  badgeTypes.find((value) => value === props.account.type) || null
)

function metadataString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined
}

const authMode = computed(() => metadataString(props.account.credentials?.auth_mode))
const planType = computed(() => metadataString(props.account.credentials?.plan_type))
const privacyMode = computed(() => metadataString(props.account.extra?.privacy_mode))
const subscriptionExpiresAt = computed(() =>
  metadataString(props.account.credentials?.subscription_expires_at)
)

type CompactState = 'active' | 'blocked' | 'auto'

const compactState = computed<CompactState | null>(() => {
  if (props.account.platform !== 'openai' || (props.account.type !== 'oauth' && props.account.type !== 'apikey')) {
    return null
  }
  const mode = metadataString(props.account.extra?.openai_compact_mode) || 'auto'
  if (mode === 'force_on') return 'active'
  if (mode === 'force_off') return 'blocked'
  if (typeof props.account.extra?.openai_compact_supported === 'boolean') {
    return props.account.extra.openai_compact_supported ? 'active' : 'blocked'
  }
  return 'auto'
})

const compactMeta = computed(() => {
  switch (compactState.value) {
    case 'active':
      return {
        label: t('admin.accounts.openai.compactSupported'),
        className: 'text-emerald-600 dark:text-emerald-300',
        dotClass: 'bg-emerald-500 shadow-[0_0_0_2px_rgba(16,185,129,0.14)]',
      }
    case 'blocked':
      return {
        label: t('admin.accounts.openai.compactUnsupported'),
        className: 'text-rose-600 dark:text-rose-300',
        dotClass: 'bg-rose-500 shadow-[0_0_0_2px_rgba(244,63,94,0.14)]',
      }
    case 'auto':
      return {
        label: t('admin.accounts.openai.compactAuto'),
        className: 'text-slate-500 dark:text-slate-400',
        dotClass: 'bg-slate-300 dark:bg-slate-500',
      }
    default:
      return null
  }
})

const compactTitle = computed(() => {
  const label = compactMeta.value?.label || ''
  const checkedAt = metadataString(props.account.extra?.openai_compact_checked_at)
  if (!checkedAt) return label
  const date = new Date(checkedAt)
  if (Number.isNaN(date.getTime())) return label
  return `${label} | ${t('admin.accounts.openai.compactLastChecked')}: ${date.toLocaleString('zh-CN')}`
})
</script>
