import { describe, expect, it } from 'vitest'
import { localizeApiMessage } from './api'

describe('API error localization', () => {
  it('maps lottery error codes to Chinese', () => {
    expect(localizeApiMessage('ACCOUNT_NOT_FOUND', 'Registered active account not found', 404))
      .toBe('未找到该注册账户，请确认邮箱或先完成注册。')
    expect(localizeApiMessage('RATE_LIMITED', 'Too many lottery requests', 429))
      .toBe('请求过于频繁，请稍后再试。')
  })

  it('does not expose unknown server errors', () => {
    expect(localizeApiMessage('UNKNOWN', 'internal service details', 500))
      .toBe('服务暂时繁忙，请稍后重试。')
  })
})
