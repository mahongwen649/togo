export default {
  channelStatus: {
    title: 'Channel Status',
    description: 'Inspect channel availability, latency and recent status',
    searchPlaceholder: 'Search channels...',
    allProviders: 'All Providers',
    lastUpdated: 'Last updated {time}',
    sortLabel: 'Sort',
    windowLabel: 'Availability window',
    sort: {
      custom: 'Default',
      group: 'Group',
      model: 'Model',
      availability: 'Availability',
      latency: 'Latency'
    },
    loadError: 'Failed to load channel status',
    detailLoadError: 'Failed to load channel detail',
    detailTitle: 'Channel Detail',
    closeDetail: 'Close',
    windowTab: { '7d': '7 days', '15d': '15 days', '30d': '30 days' },
    overall: {
      operational: 'OPERATIONAL',
      degraded: 'DEGRADED',
      unavailable: 'UNAVAILABLE'
    },
    detailColumns: {
      model: 'Model',
      latestStatus: 'Latest Status',
      latestLatency: 'Latest Latency (ms)',
      availability7d: '7d Availability',
      availability15d: '15d Availability',
      availability30d: '30d Availability',
      avgLatency7d: '7d Avg Latency (ms)'
    },
    empty: {
      title: 'No channels available',
      description: 'No monitored channels have been configured yet.'
    }
  }
}
