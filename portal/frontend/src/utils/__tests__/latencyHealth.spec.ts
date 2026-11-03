import { describe, expect, it } from 'vitest'

import { durationSeverity, firstTokenSeverity, formatUsageOutputRate } from '../latencyHealth'
import type { UsageLog } from '@/types'

describe('latencyHealth', () => {
  const textRow = { output_tokens: 104, duration_ms: 26350, image_count: 0, image_output_tokens: 0, billing_mode: 'token', request_type: 'stream' as const }

  it('matches Core output TPS using total duration including first-token wait', () => {
    expect(formatUsageOutputRate(textRow)).toBe('3.9 tok/s')
    expect(formatUsageOutputRate({ ...textRow, output_tokens: 265, duration_ms: 14260 })).toBe('18.6 tok/s')
    expect(formatUsageOutputRate({ ...textRow, request_type: undefined })).toBe('3.9 tok/s')
  })

  it.each<Partial<UsageLog>>([
    { output_tokens: 0 }, { output_tokens: -1 }, { output_tokens: Number.NaN },
    { duration_ms: null }, { duration_ms: 0 }, { duration_ms: -1 }, { duration_ms: Infinity },
    { image_count: 1 }, { image_output_tokens: 1 }, { billing_mode: 'image' },
    { request_type: 'live' }, { request_type: 'unknown' }
  ])('does not invent an output TPS for excluded or incomplete records: %j', (overrides) => {
    expect(formatUsageOutputRate({ ...textRow, ...overrides })).toBe('—')
  })
  it('classifies first-token latency at 10s/30s/60s boundaries', () => {
    expect(firstTokenSeverity(0)).toBe('good')
    expect(firstTokenSeverity(9_999)).toBe('good')
    expect(firstTokenSeverity(10_000)).toBe('warn')
    expect(firstTokenSeverity(29_999)).toBe('warn')
    expect(firstTokenSeverity(30_000)).toBe('slow')
    expect(firstTokenSeverity(59_999)).toBe('slow')
    expect(firstTokenSeverity(60_000)).toBe('critical')
  })

  it('classifies total duration at 1min/3min/5min boundaries', () => {
    expect(durationSeverity(0)).toBe('good')
    expect(durationSeverity(59_999)).toBe('good')
    expect(durationSeverity(60_000)).toBe('warn')
    expect(durationSeverity(179_999)).toBe('warn')
    expect(durationSeverity(180_000)).toBe('slow')
    expect(durationSeverity(299_999)).toBe('slow')
    expect(durationSeverity(300_000)).toBe('critical')
  })
})
