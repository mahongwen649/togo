import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ModelMarketView from '../ModelMarketView.vue'

const { getAvailable, getAvailableGroups, getUserGroupRates, showError, copyToClipboard } = vi.hoisted(() => ({
  getAvailable: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/channels', () => ({ default: { getAvailable } }))
vi.mock('@/api/groups', () => ({
  default: { getAvailable: getAvailableGroups, getUserGroupRates }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard })
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'modelMarket.stats.total': 'Models',
    'modelMarket.stats.label': 'Model market summary',
    'modelMarket.stats.platforms': 'Platforms',
    'modelMarket.stats.groups': 'Groups',
    'modelMarket.stats.unit': 'Pricing unit',
    'modelMarket.stats.promptCaching': 'Prompt caching',
    'modelMarket.searchPlaceholder': 'Search models...',
    'modelMarket.allGroups': 'All groups',
    'modelMarket.allPlatforms': 'All platforms',
    'modelMarket.showRatedPrice': 'Show rated price',
    'modelMarket.refresh': 'Refresh',
    'modelMarket.empty': 'No models found',
    'modelMarket.emptyNoGroups': 'No available groups',
    'modelMarket.emptyNoGroupsDescription': 'Ask an administrator to assign an active group.',
    'modelMarket.noPricing': 'No pricing',
    'modelMarket.baseRate': 'Base rate',
    'modelMarket.basePrice': 'Base price',
    'modelMarket.ratedPrice': 'Rated price',
    'modelMarket.rateBadge': '{rate}x rate',
    'modelMarket.modelCount': '{count} models',
    'modelMarket.viewModelDetails': 'View details for {model}',
    'modelMarket.available': 'Available',
    'modelMarket.copyModel': 'Copy model name',
    'modelMarket.groupsAvailable': '{count} available groups',
    'modelMarket.viewDetails': 'View details',
    'modelMarket.basePricing': 'Base pricing',
    'modelMarket.tierPricing': 'Tier pricing',
    'modelMarket.groupPricing': 'Group pricing',
    'modelMarket.rate': 'Rate',
    'modelMarket.availability': 'Availability',
    'modelMarket.channelCount': '{count} available channels',
    'modelMarket.requestUnit': 'request',
    'modelMarket.billing.token': 'Per token',
    'modelMarket.billing.per_request': 'Per request',
    'modelMarket.billing.image': 'Per image',
    'modelMarket.filters.label': 'Model filters',
    'modelMarket.filters.platform': 'Platform',
    'modelMarket.filters.group': 'Group',
    'modelMarket.filters.search': 'Search',
    'modelMarket.filters.priceMode': 'Price display mode',
    'modelMarket.loadError': 'Load failed',
    'modelMarket.pricing.input': 'Input',
    'modelMarket.pricing.output': 'Output',
    'modelMarket.pricing.cacheWrite': 'Cache write',
    'modelMarket.pricing.cacheRead': 'Cache read',
    'modelMarket.pricing.perRequest': 'Per request',
    'modelMarket.pricing.imageOutput': 'Image output'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const value = messages[key] ?? key
        return Object.entries(params ?? {}).reduce(
          (result, [name, replacement]) => result.replace(`{${name}}`, String(replacement)),
          value
        )
      }
    })
  }
})

const groups = [{
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
}]
const channels = [{
  name: 'Primary',
  description: 'Main',
  platforms: [{
    platform: 'openai',
    groups,
    supported_models: [{
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
    }]
  }]
}]

const global = {
  stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    Icon: { props: ['name'], template: '<span :data-icon="name" />' },
    ModelIcon: true,
    PlatformIcon: true
  }
}

describe('ModelMarketView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getAvailable.mockResolvedValue(channels)
    getAvailableGroups.mockResolvedValue(groups)
    getUserGroupRates.mockResolvedValue({})
    copyToClipboard.mockResolvedValue(true)
  })

  it('renders available models and prices', async () => {
    const wrapper = mount(ModelMarketView, { global })
    await flushPromises()

    expect(wrapper.text()).toContain('gpt-5.4')
    expect(wrapper.text()).toContain('¥2.50/M')
    expect(wrapper.text()).toContain('¥15.00/M')
    expect(wrapper.text()).toContain('Platforms1')
    expect(wrapper.text()).toContain('Groups1')
    expect(wrapper.text()).toContain('CNY / 1M')
    expect(wrapper.findAll('[data-test="model-card"]')).toHaveLength(1)

    const basePriceButton = wrapper.findAll('button').find((button) => button.text() === 'Base price')
    expect(basePriceButton).toBeDefined()
    await basePriceButton!.trigger('click')
    expect(wrapper.text()).toContain('$2.50/M')
    expect(wrapper.text()).toContain('USD / 1M')
  })

  it('selects the group with the lowest effective rate by default', async () => {
    const lowerDefaultGroup = {
      ...groups[0],
      id: 2,
      name: 'Lower default',
      rate_multiplier: 0.2,
    }
    getAvailable.mockResolvedValue([{
      ...channels[0],
      platforms: [{
        ...channels[0].platforms[0],
        groups: [groups[0], lowerDefaultGroup],
      }],
    }])
    getAvailableGroups.mockResolvedValue([groups[0], lowerDefaultGroup])
    getUserGroupRates.mockResolvedValue({ 1: 0.05 })

    const wrapper = mount(ModelMarketView, { global })
    await flushPromises()

    const defaultGroupButton = wrapper.findAll('.filter-pill')
      .find((button) => button.text().includes('Default'))
    expect(defaultGroupButton?.attributes('aria-pressed')).toBe('true')
    expect(wrapper.text()).toContain('¥0.125/M')
    expect(wrapper.text()).toContain('CNY / 1M')
  })

	 it('shows only the fixed request price for per-request models', async () => {
		getAvailable.mockResolvedValue([{
			...channels[0],
			platforms: [{
				...channels[0].platforms[0],
				supported_models: [{
					name: 'gpt-image-2',
					platform: 'openai',
					pricing: {
						billing_mode: 'per_request',
						input_price: 0.000005,
						output_price: 0.00001,
						cache_write_price: 0,
						cache_read_price: 0.00000125,
						image_input_price: 0.000008,
						image_output_price: 0.00003,
						per_request_price: 0.07,
						intervals: []
					}
				}]
			}]
		}])

		const wrapper = mount(ModelMarketView, { global, attachTo: document.body })
		await flushPromises()

		const card = wrapper.get('[data-test="model-card"]')
    expect(card.text()).toContain('¥0.070/request')
		expect(card.text()).not.toContain('$5.00/M')
		expect(card.text()).not.toContain('$10.00/M')

		await card.trigger('click')
		await flushPromises()
		const dialog = document.body.querySelector('[role="dialog"]')
		expect(dialog?.textContent).toContain('$0.070/request')
		expect(dialog?.textContent).not.toContain('$5.00/M')
		wrapper.unmount()
	})

  it('keeps tiny per-request prices visible instead of rounding them to zero', async () => {
    getAvailable.mockResolvedValue([{
      ...channels[0],
      platforms: [{
        ...channels[0].platforms[0],
        supported_models: [{
          name: 'gpt-5.6-luna',
          platform: 'openai',
          pricing: {
            billing_mode: 'per_request',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: null,
            image_output_price: null,
            per_request_price: 0.0004,
            intervals: []
          }
        }]
      }]
    }])

    const wrapper = mount(ModelMarketView, { global })
    await flushPromises()

    const card = wrapper.get('[data-test="model-card"]')
    expect(card.text()).toContain('¥0.0004/request')
    expect(card.text()).not.toContain('¥0.000/request')

    await card.trigger('click')
    await flushPromises()
    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog?.textContent).toContain('$0.0004/request')
    expect(dialog?.textContent).not.toContain('$0.000/request')
    wrapper.unmount()
  })

  it('filters by platform and group and opens model pricing details', async () => {
    const wrapper = mount(ModelMarketView, { global, attachTo: document.body })
    await flushPromises()

    const platformFilter = wrapper.findAll('.filter-pill').find((button) => button.text().includes('OpenAI'))
    expect(platformFilter).toBeDefined()
    await platformFilter!.trigger('click')
    expect(wrapper.findAll('[data-test="model-card"]')).toHaveLength(1)

    const groupFilter = wrapper.findAll('.filter-pill').find((button) => button.text().includes('Default'))
    expect(groupFilter).toBeDefined()
    await groupFilter!.trigger('click')

    await wrapper.get('[data-test="model-card"]').trigger('click')
    await flushPromises()
    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog?.textContent).toContain('Base pricing')
    expect(dialog?.textContent).toContain('Group pricing')
    expect(dialog?.textContent).toContain('$2.50/M')

    wrapper.unmount()
  })

  it('filters models and explains an account without groups', async () => {
    const wrapper = mount(ModelMarketView, { global })
    await flushPromises()
    await wrapper.find('input[type="search"]').setValue('missing')
    expect(wrapper.text()).toContain('No models found')

    wrapper.unmount()
    getAvailable.mockResolvedValue([])
    getAvailableGroups.mockResolvedValue([])
    const emptyWrapper = mount(ModelMarketView, { global })
    await flushPromises()
    expect(emptyWrapper.text()).toContain('No available groups')
  })

  it('reports channel loading failures', async () => {
    getAvailable.mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(ModelMarketView, { global })
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Load failed')
  })
})
