import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../DashboardView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('admin DashboardView', () => {
  it('renders the retained statistics dashboard instead of only management shortcuts', () => {
    expect(viewSource).toContain('adminAPI.dashboard.getSnapshotV2')
    expect(viewSource).toContain('ModelDistributionChart')
    expect(viewSource).toContain('TokenUsageTrend')
    expect(viewSource).toContain('adminAPI.dashboard.getUserUsageTrend')
    expect(viewSource).toContain('adminAPI.dashboard.getUserSpendingRanking')
  })

  it('removes company and finance shortcuts from the overview', () => {
    expect(viewSource).not.toContain('path="/admin/companies"')
    expect(viewSource).not.toContain('path="/admin/finance"')
    expect(viewSource).not.toContain('DashboardQuickAction')
    expect(viewSource).not.toContain('/admin/groups')
    expect(viewSource).not.toContain('/admin/ops')
    expect(viewSource).not.toContain('/admin/settings')
  })

  it('shows today and all-time cache hit rates using read tokens over total input tokens', () => {
    expect(viewSource).toContain("t('admin.dashboard.cacheHitRate')")
    expect(viewSource).toContain('formatCacheHitRate(stats.today_input_tokens, stats.today_cache_read_tokens)')
    expect(viewSource).toContain('formatCacheHitRate(stats.total_input_tokens, stats.total_cache_read_tokens)')
    expect(viewSource).toContain('toFiniteNumber(inputTokens) + cacheRead')
  })

  it('uses shared device-aware dashboard components without changing desktop data flow', () => {
    expect(viewSource).toContain('const { isMobileH5 } = useDeviceMode()')
    expect(viewSource).toContain('<ChartRangeToolbar')
    expect(viewSource).toContain("isMobileH5 ? 'gap-2' : 'gap-4'")
    expect(viewSource).toContain('<ModelDistributionChart')
    expect(viewSource).toContain('<TokenUsageTrend')
    expect(viewSource).not.toContain('isMobileViewport')
  })
})
