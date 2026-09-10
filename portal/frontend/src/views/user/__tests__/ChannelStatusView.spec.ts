import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ChannelStatusView from '../ChannelStatusView.vue'

const { listChannelMonitors, showError } = vi.hoisted(() => ({
  listChannelMonitors: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/channelMonitor', () => ({
  list: listChannelMonitors,
  status: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { channel_monitor_enabled: true },
    showError,
  }),
}))

vi.mock('@/composables/useAutoRefresh', () => ({
  useAutoRefresh: () => ({
    enabled: { value: false },
    intervalSeconds: { value: 60 },
    countdown: { value: 60 },
    intervals: [30, 60, 120],
    start: vi.fn(),
    stop: vi.fn(),
    setEnabled: vi.fn(),
    setInterval: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'channelStatus.searchPlaceholder': 'Search status...',
    'channelStatus.allProviders': 'All providers',
    'channelStatus.lastUpdated': 'Last updated {time}',
    'channelStatus.sortLabel': 'Sort',
    'channelStatus.windowLabel': 'Availability window',
    'channelStatus.sort.custom': 'Default',
    'channelStatus.sort.group': 'Group',
    'channelStatus.sort.model': 'Model',
    'channelStatus.sort.availability': 'Availability',
    'channelStatus.sort.latency': 'Latency',
    'channelStatus.detailTitle': 'Detail',
    'channelStatus.loadError': 'Load failed',
    'channelStatus.detailLoadError': 'Detail load failed',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const value = messages[key] ?? key
        return params ? value.replace('{time}', params.time) : value
      },
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const MonitorHeroStub = {
  props: ['overallStatus', 'intervalSeconds', 'window', 'loading', 'autoRefresh'],
  emits: ['update:window', 'refresh'],
  template: '<section data-test="hero"><slot /></section>',
}
const MonitorCardGridStub = {
  props: ['items', 'window', 'countdownSeconds', 'loading', 'detailCache'],
  emits: ['cardClick'],
  template: `
    <div data-test="grid">
      <article v-for="item in items" :key="item.id" data-test="monitor-card">
        {{ item.name }} {{ item.provider }} {{ item.group_name }} {{ item.primary_model }}
      </article>
    </div>
  `,
}
const MonitorDetailDialogStub = {
  props: ['show', 'monitorId', 'title'],
  emits: ['close'],
  template: '<div />',
}
const IconStub = { props: ['name'], template: '<span :data-icon="name" />' }

const monitors = [
  {
    id: 1,
    name: 'Slow Anthropic',
    provider: 'anthropic',
    group_name: 'Beta',
    primary_model: 'claude-sonnet-4',
    primary_status: 'operational',
    primary_latency_ms: 900,
    primary_ping_latency_ms: 90,
    availability_7d: 97,
    extra_models: [],
    timeline: [],
  },
  {
    id: 2,
    name: 'Fast OpenAI',
    provider: 'openai',
    group_name: 'Alpha',
    primary_model: 'gpt-5',
    primary_status: 'operational',
    primary_latency_ms: 120,
    primary_ping_latency_ms: 20,
    availability_7d: 99.9,
    extra_models: [],
    timeline: [],
  },
  {
    id: 3,
    name: 'Backup Gemini',
    provider: 'gemini',
    group_name: 'Gamma',
    primary_model: 'gemini-2.5-pro',
    primary_status: 'failed',
    primary_latency_ms: null,
    primary_ping_latency_ms: null,
    availability_7d: 80,
    extra_models: [],
    timeline: [],
  },
]

function mountView() {
  return mount(ChannelStatusView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        MonitorHero: MonitorHeroStub,
        MonitorCardGrid: MonitorCardGridStub,
        MonitorDetailDialog: MonitorDetailDialogStub,
        Icon: IconStub,
      },
    },
  })
}

describe('ChannelStatusView', () => {
  beforeEach(() => {
    listChannelMonitors.mockReset()
    showError.mockReset()
    listChannelMonitors.mockResolvedValue({ items: monitors })
  })

  it('filters monitor cards by search text and provider', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Fast OpenAI')
    expect(wrapper.text()).toContain('Slow Anthropic')

    await wrapper.find('input[type="search"]').setValue('gpt')
    expect(wrapper.text()).toContain('Fast OpenAI')
    expect(wrapper.text()).not.toContain('Slow Anthropic')

    await wrapper.find('input[type="search"]').setValue('')
    await wrapper.find('[data-test-provider="anthropic"]').trigger('click')
    expect(wrapper.text()).toContain('Slow Anthropic')
    expect(wrapper.text()).not.toContain('Fast OpenAI')
  })

  it('sorts monitor cards and shows the last update time', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test-sort="latency"]').trigger('click')
    const cards = wrapper.findAll('[data-test="monitor-card"]')

    expect(cards.map(card => card.text())).toEqual([
      expect.stringContaining('Fast OpenAI'),
      expect.stringContaining('Slow Anthropic'),
      expect.stringContaining('Backup Gemini'),
    ])
    expect(wrapper.text()).toContain('Last updated')
  })

  it('uses TIMI-style button groups instead of select dropdowns', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('select[aria-label="Provider"]').exists()).toBe(false)
    expect(wrapper.find('select[aria-label="Sort"]').exists()).toBe(false)
    expect(wrapper.find('[data-test-provider="all"]').exists()).toBe(true)
    expect(wrapper.find('[data-test-sort="custom"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Availability window')
  })
})


