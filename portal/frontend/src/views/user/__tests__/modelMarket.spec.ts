import { describe, expect, it } from 'vitest'
import {
  buildModelMarket,
  filterModelMarketCards,
  formatModelMarketPrice,
  resolveEffectiveRate
} from '../modelMarket'
import type { UserAvailableChannel } from '@/api/channels'

const channels: UserAvailableChannel[] = [
  {
    name: 'Primary',
    description: 'Main channel',
    platforms: [
      {
        platform: 'openai',
        groups: [
          {
            id: 1,
            name: 'Default',
            platform: 'openai',
            subscription_type: 'standard',
            rate_multiplier: 1,
            peak_rate_enabled: false,
            peak_start: '',
            peak_end: '',
            peak_rate_multiplier: 1,
            is_exclusive: false
          },
          {
            id: 2,
            name: 'VIP',
            platform: 'openai',
            subscription_type: 'standard',
            rate_multiplier: 0.5,
            peak_rate_enabled: false,
            peak_start: '',
            peak_end: '',
            peak_rate_multiplier: 1,
            is_exclusive: true
          }
        ],
        supported_models: [
          {
            name: 'gpt-5.4',
            platform: 'openai',
            pricing: {
              billing_mode: 'token',
              input_price: 0.0000025,
              output_price: 0.000015,
              cache_write_price: 0,
              cache_read_price: 0.00000025,
              image_input_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: []
            }
          }
        ]
      }
    ]
  },
  {
    name: 'Backup',
    description: 'Fallback channel',
    platforms: [
      {
        platform: 'openai',
        groups: [],
        supported_models: [
          {
            name: 'gpt-5.4',
            platform: 'openai',
            pricing: null
          }
        ]
      }
    ]
  }
]

describe('modelMarket helpers', () => {
  it('deduplicates models and preserves channels and accessible groups', () => {
    const market = buildModelMarket(channels, { 2: 0.25 })

    expect(market.cards).toHaveLength(1)
    expect(market.cards[0].channels).toEqual(['Primary', 'Backup'])
    expect(market.cards[0].groups.map((group) => group.name)).toEqual(['Default', 'VIP'])
    expect(market.stats).toMatchObject({ totalModels: 1, platformCounts: { openai: 1 } })
  })

  it('filters by search, group, and platform', () => {
    const market = buildModelMarket(channels, {})

    expect(filterModelMarketCards(market.cards, { search: 'gpt' })).toHaveLength(1)
    expect(filterModelMarketCards(market.cards, { groupId: 2 })).toHaveLength(1)
    expect(filterModelMarketCards(market.cards, { platform: 'anthropic' })).toHaveLength(0)
  })

  it('uses the user-specific rate and formats per-million prices', () => {
    const card = buildModelMarket(channels, { 2: 0.25 }).cards[0]

    expect(resolveEffectiveRate(card, 2)).toBe(0.25)
    expect(resolveEffectiveRate(card, 1)).toBe(1)
    expect(formatModelMarketPrice(0.0000025, 0.5)).toBe('$1.25/M')
    expect(formatModelMarketPrice(4e-10, 1)).toBe('$0.0004/M')
    expect(formatModelMarketPrice(null, 1)).toBe('-')
  })
})
