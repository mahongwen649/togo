import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserSupportedModel,
  UserSupportedModelPricing
} from '@/api/channels'
import { formatFlexibleDecimal } from '@/utils/pricing'

export interface ModelMarketGroup extends UserAvailableGroup {
  user_rate_multiplier: number | null
}

export interface ModelMarketCard {
  key: string
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
  groups: ModelMarketGroup[]
  channels: string[]
  promptCaching: boolean
}

export interface ModelMarketData {
  cards: ModelMarketCard[]
  groups: ModelMarketGroup[]
  platforms: string[]
  stats: {
    totalModels: number
    platformCounts: Record<string, number>
    promptCachingModels: number
  }
}

export interface ModelMarketFilters {
  search?: string
  groupId?: number | null
  platform?: string | null
}

function modelKey(model: UserSupportedModel): string {
  return `${model.platform || 'unknown'}::${model.name}`
}

function hasPromptCaching(pricing: UserSupportedModelPricing | null): boolean {
  return Boolean(
    pricing &&
    ((pricing.cache_write_price != null && pricing.cache_write_price > 0) ||
      (pricing.cache_read_price != null && pricing.cache_read_price > 0))
  )
}

function uniqueById(groups: ModelMarketGroup[]): ModelMarketGroup[] {
  const seen = new Set<number>()
  return groups.filter((group) => {
    if (seen.has(group.id)) return false
    seen.add(group.id)
    return true
  })
}

export function buildModelMarket(
  channels: UserAvailableChannel[],
  userGroupRates: Record<number, number>
): ModelMarketData {
  const cardMap = new Map<string, ModelMarketCard>()
  const allGroups: ModelMarketGroup[] = []

  for (const channel of channels) {
    for (const section of channel.platforms) {
      const sectionGroups = section.groups.map((group): ModelMarketGroup => ({
        ...group,
        user_rate_multiplier: userGroupRates[group.id] ?? null
      }))
      allGroups.push(...sectionGroups)

      for (const model of section.supported_models) {
        const key = modelKey(model)
        const existing = cardMap.get(key)
        if (existing) {
          existing.groups = uniqueById([...existing.groups, ...sectionGroups])
          if (!existing.channels.includes(channel.name)) existing.channels.push(channel.name)
          existing.promptCaching ||= hasPromptCaching(model.pricing)
          continue
        }

        cardMap.set(key, {
          key,
          name: model.name,
          platform: model.platform || section.platform,
          pricing: model.pricing,
          groups: uniqueById(sectionGroups),
          channels: [channel.name],
          promptCaching: hasPromptCaching(model.pricing)
        })
      }
    }
  }

  const cards = Array.from(cardMap.values()).sort((a, b) => {
    const platformCompare = a.platform.localeCompare(b.platform)
    return platformCompare || a.name.localeCompare(b.name)
  })
  const platformCounts = cards.reduce<Record<string, number>>((counts, card) => {
    counts[card.platform] = (counts[card.platform] ?? 0) + 1
    return counts
  }, {})

  return {
    cards,
    groups: uniqueById(allGroups).sort((a, b) => a.name.localeCompare(b.name)),
    platforms: Object.keys(platformCounts).sort(),
    stats: {
      totalModels: cards.length,
      platformCounts,
      promptCachingModels: cards.filter((card) => card.promptCaching).length
    }
  }
}

export function filterModelMarketCards(
  cards: ModelMarketCard[],
  filters: ModelMarketFilters
): ModelMarketCard[] {
  const search = filters.search?.trim().toLowerCase() ?? ''
  return cards.filter((card) => {
    if (filters.platform && card.platform !== filters.platform) return false
    if (filters.groupId && !card.groups.some((group) => group.id === filters.groupId)) return false
    if (!search) return true
    return (
      card.name.toLowerCase().includes(search) ||
      card.platform.toLowerCase().includes(search) ||
      card.groups.some((group) => group.name.toLowerCase().includes(search))
    )
  })
}

export function resolveEffectiveRate(card: ModelMarketCard, groupId: number | null): number {
  const group = groupId == null ? card.groups[0] : card.groups.find((item) => item.id === groupId)
  return group?.user_rate_multiplier ?? group?.rate_multiplier ?? 1
}

export function findLowestRateGroupId(groups: ModelMarketGroup[]): number | null {
  const lowest = [...groups].sort((a, b) => {
    const rateA = a.user_rate_multiplier ?? a.rate_multiplier ?? 1
    const rateB = b.user_rate_multiplier ?? b.rate_multiplier ?? 1
    return rateA - rateB || a.name.localeCompare(b.name) || a.id - b.id
  })[0]
  return lowest?.id ?? null
}

export function formatModelMarketPrice(value: number | null | undefined, rate: number, currencySymbol = '$'): string {
  if (value == null) return '-'
  const perMillion = value * rate * 1_000_000
  if (perMillion === 0) return `${currencySymbol}0/M`
  const decimals = perMillion < 1 ? 3 : 2
  return `${currencySymbol}${formatFlexibleDecimal(perMillion, decimals)}/M`
}
