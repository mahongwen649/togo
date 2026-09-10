export default {
  channelStatus: {
    title: '渠道状态',
    description: '查看渠道可用性、延迟和近期状态',
    searchPlaceholder: '搜索渠道...',
    allProviders: '全部供应商',
    lastUpdated: '更新于 {time}',
    sortLabel: '排序',
    windowLabel: '可用性区间',
    sort: {
      custom: '默认排序',
      group: '按分组',
      model: '按模型',
      availability: '按可用率',
      latency: '按延迟'
    },
    loadError: '加载渠道状态失败',
    detailLoadError: '加载渠道详情失败',
    detailTitle: '渠道详情',
    closeDetail: '关闭',
    windowTab: { '7d': '7 天', '15d': '15 天', '30d': '30 天' },
    overall: {
      operational: 'OPERATIONAL',
      degraded: 'DEGRADED',
      unavailable: 'UNAVAILABLE'
    },
    detailColumns: {
      model: '模型',
      latestStatus: '最新状态',
      latestLatency: '最新延迟 (ms)',
      availability7d: '7 天可用率',
      availability15d: '15 天可用率',
      availability30d: '30 天可用率',
      avgLatency7d: '7 天平均延迟 (ms)'
    },
    empty: {
      title: '暂无可显示的渠道',
      description: '管理员尚未配置可监控的渠道。'
    }
  }
}
