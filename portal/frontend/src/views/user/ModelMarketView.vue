<template>
  <AppLayout>
    <div class="model-market-page">
      <section class="market-summary" :aria-label="t('modelMarket.stats.label')">
        <div class="summary-item">
          <span>{{ t('modelMarket.stats.total') }}</span>
          <strong>{{ market.stats.totalModels }}</strong>
        </div>
        <div class="summary-item">
          <span>{{ t('modelMarket.stats.platforms') }}</span>
          <strong>{{ market.platforms.length }}</strong>
        </div>
        <div class="summary-item">
          <span>{{ t('modelMarket.stats.groups') }}</span>
          <strong>{{ market.groups.length }}</strong>
        </div>
        <div class="summary-item summary-item-accent">
          <span>{{ t('modelMarket.stats.unit') }}</span>
          <strong>{{ showRatedPrice ? 'CNY / 1M' : 'USD / 1M' }}</strong>
        </div>
      </section>

      <section class="market-filters" :aria-label="t('modelMarket.filters.label')">
        <div class="filter-row">
          <div class="filter-heading">{{ t('modelMarket.filters.platform') }}</div>
          <div class="filter-options">
            <button
              type="button"
              class="filter-pill"
              :class="selectedPlatform === '' && 'filter-pill-active'"
              :aria-pressed="selectedPlatform === ''"
              @click="selectedPlatform = ''"
            >
              {{ t('modelMarket.allPlatforms') }}
              <span>{{ market.stats.totalModels }}</span>
            </button>
            <button
              v-for="platform in market.platforms"
              :key="platform"
              type="button"
              class="filter-pill"
              :class="selectedPlatform === platform && 'filter-pill-active'"
              :aria-pressed="selectedPlatform === platform"
              @click="selectedPlatform = platform"
            >
              {{ platformLabel(platform) }}
              <span>{{ market.stats.platformCounts[platform] }}</span>
            </button>
          </div>
        </div>

        <div class="filter-row">
          <div class="filter-heading">{{ t('modelMarket.filters.group') }}</div>
          <div class="filter-options">
            <button
              type="button"
              class="filter-pill"
              :class="selectedGroupId === 0 && 'filter-pill-active'"
              :aria-pressed="selectedGroupId === 0"
              @click="selectedGroupId = 0"
            >
              {{ t('modelMarket.allGroups') }}
              <span>{{ market.stats.totalModels }}</span>
            </button>
            <button
              v-for="group in market.groups"
              :key="group.id"
              type="button"
              class="filter-pill"
              :class="selectedGroupId === group.id && 'filter-pill-active'"
              :aria-pressed="selectedGroupId === group.id"
              @click="selectedGroupId = group.id"
            >
              {{ group.name }}
              <span>{{ groupModelCount(group.id) }}</span>
            </button>
          </div>
        </div>

        <div class="filter-tools">
          <label class="search-field">
            <span>{{ t('modelMarket.filters.search') }}</span>
            <span class="search-input-wrap">
              <Icon name="search" size="sm" />
              <input
                v-model="searchQuery"
                type="search"
                :placeholder="t('modelMarket.searchPlaceholder')"
              />
            </span>
          </label>

          <div class="price-mode" :aria-label="t('modelMarket.filters.priceMode')">
            <button
              type="button"
              :class="!showRatedPrice && 'price-mode-active'"
              :aria-pressed="!showRatedPrice"
              @click="showRatedPrice = false"
            >
              {{ t('modelMarket.basePrice') }}
            </button>
            <button
              type="button"
              :class="showRatedPrice && 'price-mode-active'"
              :aria-pressed="showRatedPrice"
              @click="showRatedPrice = true"
            >
              {{ t('modelMarket.ratedPrice') }}
            </button>
          </div>

          <button
            type="button"
            class="refresh-button"
            :disabled="loading"
            :title="t('modelMarket.refresh')"
            @click="loadModels"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </section>

      <section v-if="filteredCards.length === 0" class="market-empty">
        <Icon :name="emptyStateIcon" size="xl" />
        <p>{{ emptyStateTitle }}</p>
        <span v-if="emptyStateDescription">{{ emptyStateDescription }}</span>
      </section>

      <div v-else class="platform-sections">
        <section v-for="section in platformSections" :key="section.platform" class="platform-section">
          <header class="platform-header">
            <span class="platform-mark">
              <PlatformIcon :platform="normalizePlatform(section.platform)" size="sm" />
            </span>
            <h2>{{ platformLabel(section.platform) }}</h2>
            <span>{{ t('modelMarket.modelCount', { count: section.cards.length }) }}</span>
          </header>

          <div class="model-grid">
            <article
              v-for="(card, index) in section.cards"
              :key="`${filterRevision}:${card.key}`"
              data-test="model-card"
              class="model-card"
              tabindex="0"
              role="button"
              :aria-label="t('modelMarket.viewModelDetails', { model: card.name })"
              :style="cardAnimationStyle(index)"
              @click="openDetails(card)"
              @keydown.enter.prevent="openDetails(card)"
              @keydown.space.prevent="openDetails(card)"
              @mousemove="tiltCard"
              @mouseleave="resetCardTilt"
            >
              <div class="availability-dot" :title="t('modelMarket.available')"></div>

              <div class="model-card-header">
                <ModelIcon :model="card.name" size="32px" class="model-icon" />
                <div class="model-identity">
                  <div class="model-name-row">
                    <h3>{{ card.name }}</h3>
                    <button
                      type="button"
                      class="copy-model-button"
                      :title="t('modelMarket.copyModel')"
                      @click.stop="copyModelName(card.name)"
                    >
                      <Icon name="copy" size="xs" />
                    </button>
                  </div>
                  <div class="model-badges">
                    <span class="platform-badge">{{ platformLabel(card.platform) }}</span>
                    <span>{{ billingModeLabel(card) }}</span>
                    <span v-if="card.promptCaching" class="cache-badge">
                      <Icon name="sparkles" size="xs" />
                      Cache
                    </span>
                  </div>
                </div>
              </div>

              <div v-if="card.pricing" class="price-grid">
                <div v-for="item in cardPriceItems(card)" :key="item.key" class="price-cell">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                </div>
              </div>
              <div v-else class="no-price-state">{{ t('modelMarket.noPricing') }}</div>

              <div class="group-badges">
                <span v-for="group in card.groups.slice(0, 3)" :key="group.id">
                  {{ group.name }} · {{ groupRateLabel(group) }}
                </span>
                <span v-if="card.groups.length > 3">+{{ card.groups.length - 3 }}</span>
              </div>

              <footer class="model-card-footer">
                <span>{{ t('modelMarket.groupsAvailable', { count: card.groups.length }) }}</span>
                <span class="details-link">
                  {{ t('modelMarket.viewDetails') }}
                  <Icon name="chevronRight" size="xs" />
                </span>
              </footer>
            </article>
          </div>
        </section>
      </div>
    </div>

    <Teleport to="body">
      <Transition name="market-modal">
        <div
          v-if="selectedCard"
          class="model-modal-overlay"
          role="dialog"
          aria-modal="true"
          :aria-label="selectedCard.name"
          @click.self="closeDetails"
        >
          <section ref="dialogRef" class="model-modal" tabindex="-1">
            <header class="model-modal-header">
              <div class="model-modal-title">
                <ModelIcon :model="selectedCard.name" size="34px" />
                <div>
                  <h2>{{ selectedCard.name }}</h2>
                  <div class="model-badges">
                    <span class="platform-badge">{{ platformLabel(selectedCard.platform) }}</span>
                    <span>{{ billingModeLabel(selectedCard) }}</span>
                    <span>{{ t('modelMarket.groupsAvailable', { count: selectedCard.groups.length }) }}</span>
                  </div>
                </div>
              </div>
              <button ref="closeButtonRef" type="button" class="modal-close" :aria-label="t('common.close')" @click="closeDetails">
                <Icon name="x" size="md" />
              </button>
            </header>

            <div class="model-modal-body">
              <section>
                <h3>{{ t('modelMarket.basePricing') }}</h3>
                <div v-if="selectedCard.pricing" class="detail-price-grid">
                  <div v-for="item in detailPriceItems(selectedCard, 1)" :key="item.key">
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value }}</strong>
                  </div>
                </div>
                <p v-else class="modal-empty">{{ t('modelMarket.noPricing') }}</p>
              </section>

              <section v-if="selectedCard.pricing?.intervals?.length">
                <h3>{{ t('modelMarket.tierPricing') }}</h3>
                <div class="tier-grid">
                  <div v-for="(interval, index) in selectedCard.pricing.intervals" :key="`${interval.tier_label || index}`" class="tier-card">
                    <strong>{{ intervalLabel(interval) }}</strong>
                    <dl>
                      <template v-for="item in intervalPriceItems(interval)" :key="item.key">
                        <dt>{{ item.label }}</dt>
                        <dd>{{ item.value }}</dd>
                      </template>
                    </dl>
                  </div>
                </div>
              </section>

              <section v-if="selectedCard.groups.length">
                <h3>{{ t('modelMarket.groupPricing') }}</h3>
                <div class="group-price-table-wrap">
                  <table class="group-price-table">
                    <thead>
                      <tr>
                        <th>{{ t('modelMarket.filters.group') }}</th>
                        <th>{{ t('modelMarket.rate') }}</th>
                        <th v-for="field in visiblePriceFields" :key="field.key">{{ field.label }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="group in selectedCard.groups" :key="group.id">
                        <td><strong>{{ group.name }}</strong></td>
                        <td>{{ groupRateLabel(group) }}</td>
                        <td v-for="field in visiblePriceFields" :key="field.key">
                          {{ formatPriceField(field, selectedCard.pricing, groupRate(group), true) }}
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </section>

              <section class="model-meta-section">
                <h3>{{ t('modelMarket.availability') }}</h3>
                <div class="meta-list">
                  <span><Icon name="checkCircle" size="sm" />{{ t('modelMarket.available') }}</span>
                  <span><Icon name="server" size="sm" />{{ t('modelMarket.channelCount', { count: selectedCard.channels.length }) }}</span>
                  <span v-if="selectedCard.promptCaching"><Icon name="sparkles" size="sm" />Prompt Caching</span>
                </div>
              </section>
            </div>
          </section>
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import userChannelsAPI, {
  type UserPricingInterval,
  type UserSupportedModelPricing
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  buildModelMarket,
  findLowestRateGroupId,
  filterModelMarketCards,
  formatModelMarketPrice,
  resolveEffectiveRate,
  type ModelMarketCard,
  type ModelMarketGroup
} from './modelMarket'
import { formatFlexibleDecimal } from '@/utils/pricing'

interface PriceField {
  key: keyof Pick<UserSupportedModelPricing, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_read_price' | 'image_output_price' | 'per_request_price'>
  label: string
  mode: 'token' | 'request'
}

interface PriceItem {
  key: string
  label: string
  value: string
}

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const loading = ref(false)
const searchQuery = ref('')
const selectedGroupId = ref(0)
const selectedPlatform = ref('')
const showRatedPrice = ref(true)
const market = ref(buildModelMarket([], {}))
const availableGroupCount = ref<number | null>(null)
const loadFailed = ref(false)
const selectedCard = ref<ModelMarketCard | null>(null)
const dialogRef = ref<HTMLElement | null>(null)
const closeButtonRef = ref<HTMLButtonElement | null>(null)
const filterRevision = ref(0)
const knownPlatforms = new Set<GroupPlatform>(['anthropic', 'openai', 'gemini', 'antigravity', 'grok'])
let previousActiveElement: HTMLElement | null = null
let initialGroupApplied = false

const filteredCards = computed(() => filterModelMarketCards(market.value.cards, {
  search: searchQuery.value,
  groupId: selectedGroupId.value || null,
  platform: selectedPlatform.value || null
}))
const platformSections = computed(() => market.value.platforms
  .map((platform) => ({
    platform,
    cards: filteredCards.value.filter((card) => card.platform === platform)
  }))
  .filter((section) => section.cards.length > 0))
const hasNoAvailableGroups = computed(() => !loading.value && availableGroupCount.value === 0 && market.value.cards.length === 0)
const emptyStateIcon = computed(() => hasNoAvailableGroups.value || loadFailed.value ? 'shield' : 'inbox')
const emptyStateTitle = computed(() => {
  if (loadFailed.value) return t('modelMarket.loadError')
  return hasNoAvailableGroups.value ? t('modelMarket.emptyNoGroups') : t('modelMarket.empty')
})
const emptyStateDescription = computed(() => hasNoAvailableGroups.value ? t('modelMarket.emptyNoGroupsDescription') : '')
const priceFields = computed<PriceField[]>(() => [
  { key: 'input_price', label: t('modelMarket.pricing.input'), mode: 'token' },
  { key: 'output_price', label: t('modelMarket.pricing.output'), mode: 'token' },
  { key: 'cache_write_price', label: t('modelMarket.pricing.cacheWrite'), mode: 'token' },
  { key: 'cache_read_price', label: t('modelMarket.pricing.cacheRead'), mode: 'token' },
  { key: 'image_output_price', label: t('modelMarket.pricing.imageOutput'), mode: 'token' },
  { key: 'per_request_price', label: t('modelMarket.pricing.perRequest'), mode: 'request' }
])
const visiblePriceFields = computed(() => {
	const pricing = selectedCard.value?.pricing
	return pricing ? priceFieldsForMode(pricing.billing_mode).filter((field) => pricing[field.key] != null) : []
})

watch([searchQuery, selectedGroupId, selectedPlatform], () => {
  filterRevision.value += 1
})

watch(selectedCard, async (card) => {
  if (card) {
    previousActiveElement = document.activeElement as HTMLElement
    document.body.classList.add('modal-open')
    await nextTick()
    closeButtonRef.value?.focus()
  } else {
    document.body.classList.remove('modal-open')
    previousActiveElement?.focus()
    previousActiveElement = null
  }
})

function normalizePlatform(platform: string): GroupPlatform | undefined {
  return knownPlatforms.has(platform as GroupPlatform) ? platform as GroupPlatform : undefined
}

function platformLabel(platform: string): string {
  const labels: Record<string, string> = {
    anthropic: 'Claude',
    openai: 'OpenAI',
    gemini: 'Gemini',
    antigravity: 'Antigravity',
    grok: 'xAI'
  }
  return labels[platform.toLowerCase()] ?? platform
}

function groupModelCount(groupId: number): number {
  return market.value.cards.filter((card) => card.groups.some((group) => group.id === groupId)).length
}

function groupRate(group: ModelMarketGroup): number {
  return group.user_rate_multiplier ?? group.rate_multiplier ?? 1
}

function groupRateLabel(group: ModelMarketGroup): string {
  return `${groupRate(group).toFixed(3).replace(/\.?0+$/, '')}x`
}

function billingModeLabel(card: ModelMarketCard): string {
  const mode = card.pricing?.billing_mode ?? 'token'
  return t(`modelMarket.billing.${mode}`)
}

function formatRequestPrice(value: number | null | undefined, rate: number, currencySymbol = '$'): string {
  if (value == null) return '-'
  const total = value * rate
  const digits = total < 1 ? 3 : 2
  return `${currencySymbol}${formatFlexibleDecimal(total, digits)}/${t('modelMarket.requestUnit')}`
}

function formatPriceField(field: PriceField, pricing: UserSupportedModelPricing | null, rate: number, rated = false): string {
  if (!pricing) return '-'
  const value = pricing[field.key]
  const currencySymbol = rated ? '¥' : '$'
  return field.mode === 'request'
    ? formatRequestPrice(value, rate, currencySymbol)
    : formatModelMarketPrice(value, rate, currencySymbol)
}

function priceFieldsForMode(mode: UserSupportedModelPricing['billing_mode']): PriceField[] {
	if (mode === 'per_request') {
		return priceFields.value.filter((field) => field.key === 'per_request_price')
	}
	if (mode === 'image') {
		return priceFields.value.filter((field) => field.key === 'image_output_price')
	}
	return priceFields.value.filter((field) => field.mode === 'token')
}

function detailPriceItems(card: ModelMarketCard, rate: number, rated = false): PriceItem[] {
	if (!card.pricing) return []
	return priceFieldsForMode(card.pricing.billing_mode)
    .filter((field) => card.pricing?.[field.key] != null)
    .map((field) => ({
      key: field.key,
      label: field.label,
      value: formatPriceField(field, card.pricing, rate, rated)
    }))
}

function cardPriceItems(card: ModelMarketCard): PriceItem[] {
  const rate = showRatedPrice.value ? resolveEffectiveRate(card, selectedGroupId.value || null) : 1
  return detailPriceItems(card, rate, showRatedPrice.value).slice(0, 4)
}

function intervalLabel(interval: UserPricingInterval): string {
  if (interval.tier_label) return interval.tier_label
  const start = interval.min_tokens.toLocaleString()
  const end = interval.max_tokens == null ? '+' : interval.max_tokens.toLocaleString()
  return `${start} - ${end} tokens`
}

function intervalPriceItems(interval: UserPricingInterval): PriceItem[] {
  const fields = priceFields.value.filter((field) => field.key !== 'image_output_price')
  return fields
    .filter((field) => interval[field.key as keyof UserPricingInterval] != null)
    .map((field) => {
      const value = interval[field.key as keyof UserPricingInterval] as number | null
      return {
        key: field.key,
        label: field.label,
        value: field.mode === 'request' ? formatRequestPrice(value, 1) : formatModelMarketPrice(value, 1)
      }
    })
}

function cardAnimationStyle(index: number): Record<string, string> {
  return { animationDelay: `${Math.min(index, 8) * 35}ms` }
}

function tiltCard(event: MouseEvent): void {
  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches) return
  const card = event.currentTarget as HTMLElement
  const rect = card.getBoundingClientRect()
  const x = (event.clientX - rect.left) / rect.width
  const y = (event.clientY - rect.top) / rect.height
  card.style.setProperty('--tilt-x', `${((0.5 - y) * 3.6).toFixed(2)}deg`)
  card.style.setProperty('--tilt-y', `${((x - 0.5) * 4.4).toFixed(2)}deg`)
  card.style.setProperty('--spot-x', `${(x * 100).toFixed(1)}%`)
  card.style.setProperty('--spot-y', `${(y * 100).toFixed(1)}%`)
}

function resetCardTilt(event: MouseEvent): void {
  const card = event.currentTarget as HTMLElement
  card.style.setProperty('--tilt-x', '0deg')
  card.style.setProperty('--tilt-y', '0deg')
  card.style.setProperty('--spot-x', '50%')
  card.style.setProperty('--spot-y', '0%')
}

function openDetails(card: ModelMarketCard): void {
  selectedCard.value = card
}

function closeDetails(): void {
  selectedCard.value = null
}

function handleEscape(event: KeyboardEvent): void {
  if (event.key === 'Escape' && selectedCard.value) closeDetails()
}

async function copyModelName(name: string): Promise<void> {
  await copyToClipboard(name, t('modelMarket.modelCopied'))
}

async function loadModels(): Promise<void> {
  loading.value = true
  loadFailed.value = false
  try {
    const [channels, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch(() => ({})),
      userGroupsAPI.getAvailable().then((groups) => {
        availableGroupCount.value = groups.length
      }).catch(() => {
        availableGroupCount.value = null
      })
    ])
    market.value = buildModelMarket(channels, rates)
    if (!initialGroupApplied) {
      selectedGroupId.value = findLowestRateGroupId(market.value.groups) ?? 0
      initialGroupApplied = true
    }
  } catch (error: unknown) {
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('modelMarket.loadError')))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
  loadModels()
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  document.body.classList.remove('modal-open')
})
</script>

<style scoped>
.model-market-page {
  --market-accent: 6 182 212;
  --market-accent-strong: 8 145 178;
  position: relative;
  isolation: isolate;
  display: grid;
  gap: 18px;
}

.model-market-page::before {
  position: absolute;
  inset: -20px -12px auto;
  z-index: -1;
  height: 260px;
  content: '';
  pointer-events: none;
  background:
    radial-gradient(circle at 12% 10%, rgb(var(--market-accent) / 0.12), transparent 34%),
    radial-gradient(circle at 82% 2%, rgb(59 130 246 / 0.08), transparent 30%);
}

.market-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid rgb(226 232 240 / 0.9);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.72);
  box-shadow: 0 12px 30px rgb(15 23 42 / 0.05);
  backdrop-filter: blur(18px);
}

.summary-item {
  min-width: 0;
  padding: 16px 20px;
  border-right: 1px solid rgb(226 232 240 / 0.9);
}

.summary-item:last-child {
  border-right: 0;
}

.summary-item span {
  display: block;
  color: rgb(100 116 139);
  font-size: 12px;
  font-weight: 600;
}

.summary-item strong {
  display: block;
  margin-top: 4px;
  color: rgb(15 23 42);
  font-size: 22px;
  font-weight: 800;
  line-height: 1.2;
}

.summary-item-accent {
  background: linear-gradient(135deg, rgb(var(--market-accent) / 0.09), rgb(59 130 246 / 0.05));
}

.market-filters {
  display: grid;
  gap: 14px;
  padding: 16px;
  border: 1px solid rgb(226 232 240 / 0.9);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.78);
  box-shadow: 0 14px 34px rgb(15 23 42 / 0.06);
  backdrop-filter: blur(18px);
}

.filter-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  align-items: start;
  gap: 10px;
}

.filter-heading,
.search-field > span:first-child {
  padding-top: 7px;
  color: rgb(71 85 105);
  font-size: 12px;
  font-weight: 700;
}

.filter-options {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 6px;
}

.filter-pill {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  gap: 7px;
  padding: 5px 10px;
  border: 1px solid transparent;
  border-radius: 7px;
  color: rgb(71 85 105);
  font-size: 13px;
  font-weight: 650;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, transform 160ms ease;
}

.filter-pill:hover {
  border-color: rgb(var(--market-accent) / 0.18);
  color: rgb(var(--market-accent-strong));
  background: rgb(var(--market-accent) / 0.06);
}

.filter-pill:active {
  transform: scale(0.97);
}

.filter-pill span {
  color: rgb(148 163 184);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.filter-pill-active {
  border-color: rgb(var(--market-accent) / 0.18);
  color: rgb(var(--market-accent-strong));
  background: linear-gradient(135deg, rgb(var(--market-accent) / 0.13), rgb(59 130 246 / 0.06));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.8);
}

.filter-tools {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) auto 40px;
  align-items: end;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid rgb(226 232 240 / 0.75);
}

.search-field {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}

.search-input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.search-input-wrap svg {
  position: absolute;
  left: 12px;
  color: rgb(148 163 184);
}

.search-input-wrap input {
  width: 100%;
  height: 40px;
  padding: 0 14px 0 38px;
  border: 1px solid rgb(226 232 240);
  border-radius: 7px;
  color: rgb(15 23 42);
  background: rgb(248 250 252 / 0.82);
  font-size: 13px;
  outline: none;
  transition: border-color 160ms ease, box-shadow 160ms ease, background-color 160ms ease;
}

.search-input-wrap input:focus {
  border-color: rgb(var(--market-accent) / 0.65);
  background: white;
  box-shadow: 0 0 0 3px rgb(var(--market-accent) / 0.1);
}

.price-mode {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  width: 210px;
  height: 40px;
  padding: 3px;
  border: 1px solid rgb(226 232 240);
  border-radius: 7px;
  background: rgb(241 245 249 / 0.9);
}

.price-mode button {
  border-radius: 5px;
  color: rgb(100 116 139);
  font-size: 12px;
  font-weight: 700;
  transition: color 160ms ease, background-color 160ms ease, box-shadow 160ms ease;
}

.price-mode .price-mode-active {
  color: rgb(var(--market-accent-strong));
  background: white;
  box-shadow: 0 2px 7px rgb(15 23 42 / 0.09);
}

.refresh-button,
.copy-model-button,
.modal-close {
  display: inline-grid;
  place-items: center;
  border: 1px solid rgb(226 232 240);
  color: rgb(100 116 139);
  background: rgb(255 255 255 / 0.85);
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease, transform 160ms ease;
}

.refresh-button {
  width: 40px;
  height: 40px;
  border-radius: 7px;
}

.refresh-button:hover,
.copy-model-button:hover,
.modal-close:hover {
  border-color: rgb(var(--market-accent) / 0.35);
  color: rgb(var(--market-accent-strong));
  background: rgb(var(--market-accent) / 0.07);
}

.refresh-button:active,
.copy-model-button:active,
.modal-close:active {
  transform: scale(0.94);
}

.platform-sections {
  display: grid;
  gap: 24px;
}

.platform-section {
  min-width: 0;
}

.platform-header {
  display: flex;
  align-items: center;
  gap: 9px;
  margin-bottom: 12px;
}

.platform-mark {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: 1px solid rgb(var(--market-accent) / 0.15);
  border-radius: 8px;
  color: rgb(var(--market-accent-strong));
  background: rgb(var(--market-accent) / 0.08);
}

.platform-header h2 {
  color: rgb(15 23 42);
  font-size: 17px;
  font-weight: 800;
}

.platform-header > span:last-child {
  color: rgb(100 116 139);
  font-size: 12px;
  font-weight: 600;
}

.model-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}

.model-card {
  --tilt-x: 0deg;
  --tilt-y: 0deg;
  --spot-x: 50%;
  --spot-y: 0%;
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 318px;
  flex-direction: column;
  overflow: hidden;
  padding: 17px;
  border: 1px solid rgb(226 232 240 / 0.94);
  border-top: 2px solid rgb(var(--market-accent) / 0.62);
  border-radius: 8px;
  cursor: pointer;
  background:
    radial-gradient(circle at var(--spot-x) var(--spot-y), rgb(var(--market-accent) / 0.14), transparent 29%),
    linear-gradient(145deg, rgb(255 255 255 / 0.96), rgb(248 250 252 / 0.82));
  box-shadow: 0 12px 30px rgb(15 23 42 / 0.065);
  transform: perspective(900px) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
  transform-style: preserve-3d;
  animation: card-rise 520ms both cubic-bezier(0.2, 0.75, 0.25, 1);
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
}

.model-card:hover,
.model-card:focus-visible {
  border-color: rgb(var(--market-accent) / 0.44);
  border-top-color: rgb(var(--market-accent));
  box-shadow: 0 18px 42px rgb(15 23 42 / 0.1), 0 8px 22px rgb(var(--market-accent) / 0.08);
  outline: none;
}

.availability-dot {
  position: absolute;
  top: 17px;
  right: 17px;
  width: 9px;
  height: 9px;
  border: 2px solid white;
  border-radius: 999px;
  background: rgb(16 185 129);
  box-shadow: 0 0 0 3px rgb(16 185 129 / 0.12);
}

.model-card-header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 10px;
  padding-right: 16px;
}

.model-icon {
  flex: 0 0 auto;
}

.model-identity {
  min-width: 0;
  flex: 1;
}

.model-name-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.model-name-row h3 {
  min-width: 0;
  overflow: hidden;
  color: rgb(15 23 42);
  font-size: 15px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copy-model-button {
  width: 27px;
  height: 27px;
  flex: 0 0 auto;
  border-radius: 6px;
}

.model-badges,
.group-badges,
.meta-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.model-badges {
  margin-top: 7px;
}

.model-badges > span,
.group-badges > span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 22px;
  padding: 3px 7px;
  border-radius: 6px;
  color: rgb(71 85 105);
  background: rgb(241 245 249 / 0.9);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
}

.model-badges .platform-badge {
  color: rgb(var(--market-accent-strong));
  background: rgb(var(--market-accent) / 0.1);
}

.model-badges .cache-badge {
  color: rgb(180 83 9);
  background: rgb(245 158 11 / 0.11);
}

.price-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 16px;
}

.price-cell {
  min-width: 0;
  min-height: 64px;
  padding: 10px;
  border: 1px solid rgb(226 232 240 / 0.86);
  border-radius: 7px;
  background: rgb(255 255 255 / 0.62);
}

.price-cell span,
.detail-price-grid span,
.tier-card dt {
  display: block;
  overflow: hidden;
  color: rgb(100 116 139);
  font-size: 11px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.price-cell strong,
.detail-price-grid strong,
.tier-card dd {
  display: block;
  margin-top: 5px;
  overflow: hidden;
  color: rgb(15 23 42);
  font-size: 13px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.price-cell:nth-child(2n) strong {
  color: rgb(var(--market-accent-strong));
}

.group-badges {
  align-content: flex-start;
  min-height: 50px;
  margin-top: 13px;
}

.no-price-state {
  display: grid;
  min-height: 136px;
  place-items: center;
  margin-top: 16px;
  border: 1px dashed rgb(203 213 225);
  border-radius: 7px;
  color: rgb(148 163 184);
  font-size: 13px;
}

.model-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: auto;
  padding-top: 14px;
  color: rgb(100 116 139);
  font-size: 11px;
  font-weight: 700;
}

.details-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: rgb(51 65 85);
  transition: color 160ms ease;
}

.model-card:hover .details-link {
  color: rgb(var(--market-accent-strong));
}

.market-empty {
  display: grid;
  min-height: 300px;
  place-items: center;
  align-content: center;
  gap: 8px;
  color: rgb(148 163 184);
}

.market-empty p {
  color: rgb(51 65 85);
  font-size: 14px;
  font-weight: 750;
}

.market-empty span {
  max-width: 480px;
  text-align: center;
  font-size: 13px;
}

.model-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgb(15 23 42 / 0.56);
  backdrop-filter: blur(9px);
}

.model-modal {
  width: min(920px, 100%);
  max-height: min(820px, calc(100vh - 40px));
  overflow: hidden;
  border: 1px solid rgb(255 255 255 / 0.66);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.92);
  box-shadow: 0 26px 70px rgb(15 23 42 / 0.24), 0 8px 24px rgb(var(--market-accent) / 0.08);
  outline: none;
}

.model-modal-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 22px;
  border-bottom: 1px solid rgb(226 232 240 / 0.86);
  background: linear-gradient(135deg, rgb(var(--market-accent) / 0.08), rgb(59 130 246 / 0.04));
}

.model-modal-title {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 12px;
}

.model-modal-title h2 {
  overflow-wrap: anywhere;
  color: rgb(15 23 42);
  font-size: 20px;
  font-weight: 850;
}

.modal-close {
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  border-radius: 7px;
}

.model-modal-body {
  display: grid;
  max-height: calc(min(820px, 100vh - 40px) - 92px);
  gap: 24px;
  overflow-y: auto;
  padding: 22px;
}

.model-modal-body > section {
  min-width: 0;
}

.model-modal-body h3 {
  margin-bottom: 11px;
  color: rgb(30 41 59);
  font-size: 14px;
  font-weight: 800;
}

.detail-price-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(248 250 252 / 0.75);
}

.detail-price-grid > div {
  min-width: 0;
  padding: 13px;
  border-right: 1px solid rgb(226 232 240);
}

.detail-price-grid > div:last-child {
  border-right: 0;
}

.tier-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.tier-card {
  padding: 14px;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(248 250 252 / 0.72);
}

.tier-card > strong {
  color: rgb(var(--market-accent-strong));
  font-size: 12px;
}

.tier-card dl {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 7px 12px;
  margin-top: 10px;
}

.tier-card dd {
  margin-top: 0;
}

.group-price-table-wrap {
  overflow-x: auto;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
}

.group-price-table {
  width: 100%;
  min-width: 680px;
  border-collapse: collapse;
  font-size: 12px;
}

.group-price-table th,
.group-price-table td {
  padding: 11px 12px;
  border-bottom: 1px solid rgb(226 232 240);
  text-align: left;
  white-space: nowrap;
}

.group-price-table th {
  color: rgb(100 116 139);
  background: rgb(248 250 252);
  font-weight: 700;
}

.group-price-table td {
  color: rgb(51 65 85);
  font-variant-numeric: tabular-nums;
}

.group-price-table tbody tr:last-child td {
  border-bottom: 0;
}

.group-price-table tbody tr:hover td {
  background: rgb(var(--market-accent) / 0.04);
}

.model-meta-section {
  padding-top: 18px;
  border-top: 1px solid rgb(226 232 240);
}

.meta-list span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  border-radius: 7px;
  color: rgb(71 85 105);
  background: rgb(241 245 249);
  font-size: 11px;
  font-weight: 700;
}

.meta-list svg {
  color: rgb(var(--market-accent-strong));
}

.modal-empty {
  color: rgb(148 163 184);
  font-size: 13px;
}

.market-modal-enter-active,
.market-modal-leave-active {
  transition: opacity 180ms ease;
}

.market-modal-enter-active .model-modal,
.market-modal-leave-active .model-modal {
  transition: opacity 180ms ease, transform 180ms cubic-bezier(0.2, 0.75, 0.25, 1);
}

.market-modal-enter-from,
.market-modal-leave-to,
.market-modal-enter-from .model-modal,
.market-modal-leave-to .model-modal {
  opacity: 0;
}

.market-modal-enter-from .model-modal,
.market-modal-leave-to .model-modal {
  transform: translateY(8px) scale(0.97);
}

@keyframes card-rise {
  from {
    opacity: 0;
    transform: perspective(900px) translateY(14px);
  }
  to {
    opacity: 1;
    transform: perspective(900px) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
  }
}

:global(.dark) .model-market-page {
  --market-accent: 34 211 238;
  --market-accent-strong: 103 232 249;
}

:global(.dark) .market-summary,
:global(.dark) .market-filters {
  border-color: rgb(51 65 85 / 0.82);
  background: rgb(15 23 42 / 0.72);
  box-shadow: 0 16px 38px rgb(2 6 23 / 0.22);
}

:global(.dark) .summary-item,
:global(.dark) .filter-tools {
  border-color: rgb(51 65 85 / 0.75);
}

:global(.dark) .summary-item span,
:global(.dark) .filter-heading,
:global(.dark) .search-field > span:first-child,
:global(.dark) .platform-header > span:last-child,
:global(.dark) .model-card-footer,
:global(.dark) .price-cell span,
:global(.dark) .detail-price-grid span,
:global(.dark) .tier-card dt {
  color: rgb(148 163 184);
}

:global(.dark) .summary-item strong,
:global(.dark) .platform-header h2,
:global(.dark) .model-name-row h3,
:global(.dark) .price-cell strong,
:global(.dark) .detail-price-grid strong,
:global(.dark) .tier-card dd,
:global(.dark) .model-modal-title h2,
:global(.dark) .model-modal-body h3 {
  color: rgb(241 245 249);
}

:global(.dark) .filter-pill {
  color: rgb(203 213 225);
}

:global(.dark) .search-input-wrap input,
:global(.dark) .price-mode,
:global(.dark) .refresh-button,
:global(.dark) .copy-model-button,
:global(.dark) .modal-close {
  border-color: rgb(51 65 85);
  color: rgb(148 163 184);
  background: rgb(15 23 42 / 0.78);
}

:global(.dark) .search-input-wrap input {
  color: rgb(241 245 249);
}

:global(.dark) .price-mode .price-mode-active {
  color: rgb(var(--market-accent-strong));
  background: rgb(30 41 59);
}

:global(.dark) .model-card {
  border-color: rgb(51 65 85 / 0.9);
  border-top-color: rgb(var(--market-accent) / 0.62);
  background:
    radial-gradient(circle at var(--spot-x) var(--spot-y), rgb(var(--market-accent) / 0.13), transparent 30%),
    linear-gradient(145deg, rgb(30 41 59 / 0.94), rgb(15 23 42 / 0.86));
  box-shadow: 0 14px 34px rgb(2 6 23 / 0.24);
}

:global(.dark) .model-card:hover,
:global(.dark) .model-card:focus-visible {
  border-color: rgb(var(--market-accent) / 0.4);
  box-shadow: 0 20px 46px rgb(2 6 23 / 0.34), 0 8px 22px rgb(var(--market-accent) / 0.08);
}

:global(.dark) .price-cell,
:global(.dark) .detail-price-grid,
:global(.dark) .tier-card {
  border-color: rgb(51 65 85 / 0.86);
  background: rgb(15 23 42 / 0.48);
}

:global(.dark) .model-badges > span,
:global(.dark) .group-badges > span,
:global(.dark) .meta-list span {
  color: rgb(203 213 225);
  background: rgb(51 65 85 / 0.66);
}

:global(.dark) .model-modal {
  border-color: rgb(71 85 105 / 0.75);
  background: rgb(15 23 42 / 0.94);
}

:global(.dark) .model-modal-header,
:global(.dark) .model-meta-section,
:global(.dark) .group-price-table th,
:global(.dark) .group-price-table td,
:global(.dark) .group-price-table-wrap {
  border-color: rgb(51 65 85);
}

:global(.dark) .group-price-table th {
  color: rgb(148 163 184);
  background: rgb(30 41 59);
}

:global(.dark) .group-price-table td {
  color: rgb(226 232 240);
}

:global(.dark) .group-price-table tbody tr:hover td {
  background: rgb(var(--market-accent) / 0.06);
}

@media (max-width: 1279px) {
  .model-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 767px) {
  .model-market-page {
    gap: 14px;
  }

  .market-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .summary-item:nth-child(2) {
    border-right: 0;
  }

  .summary-item:nth-child(-n + 2) {
    border-bottom: 1px solid rgb(226 232 240 / 0.9);
  }

  .filter-row,
  .search-field {
    grid-template-columns: 1fr;
    gap: 5px;
  }

  .filter-heading,
  .search-field > span:first-child {
    padding-top: 0;
  }

  .filter-tools {
    grid-template-columns: minmax(0, 1fr) 40px;
  }

  .search-field {
    grid-column: 1 / -1;
  }

  .price-mode {
    width: 100%;
  }

  .model-grid,
  .tier-grid {
    grid-template-columns: 1fr;
  }

  .model-card {
    min-height: 0;
  }

  .model-modal-overlay {
    align-items: end;
    padding: 0;
  }

  .model-modal {
    width: 100%;
    max-height: 92vh;
    border-right: 0;
    border-bottom: 0;
    border-left: 0;
    border-radius: 8px 8px 0 0;
  }

  .model-modal-body {
    max-height: calc(92vh - 90px);
    padding: 18px;
  }

  .detail-price-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .detail-price-grid > div:nth-child(2n) {
    border-right: 0;
  }

  .detail-price-grid > div:nth-child(n + 3) {
    border-top: 1px solid rgb(226 232 240);
  }
}

@media (max-width: 420px) {
  .summary-item {
    padding: 13px 14px;
  }

  .summary-item strong {
    font-size: 18px;
  }

  .market-filters {
    padding: 13px;
  }

  .filter-tools {
    grid-template-columns: minmax(0, 1fr) 38px;
  }

  .refresh-button {
    width: 38px;
  }

  .price-cell {
    padding: 9px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .model-card,
  .filter-pill,
  .refresh-button,
  .copy-model-button,
  .modal-close,
  .market-modal-enter-active,
  .market-modal-leave-active,
  .market-modal-enter-active .model-modal,
  .market-modal-leave-active .model-modal {
    animation: none;
    transition-duration: 1ms;
  }

  .model-card,
  .model-card:hover,
  .model-card:focus-visible {
    transform: none;
  }
}
</style>
