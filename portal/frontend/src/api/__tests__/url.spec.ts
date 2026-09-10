import { describe, expect, it } from 'vitest'
import { getPublicGatewayBaseURL, normalizePublicGatewayBaseURL } from '../url'

describe('normalizePublicGatewayBaseURL', () => {
  it('trims whitespace and trailing slashes', () => {
    expect(normalizePublicGatewayBaseURL('  https://api.togoapi.com///  ')).toBe('https://api.togoapi.com')
  })

  it('keeps an empty configuration empty', () => {
    expect(normalizePublicGatewayBaseURL(undefined)).toBe('')
  })

  it('uses the public API default instead of the browser origin', () => {
    expect(getPublicGatewayBaseURL()).toBe('https://api.togoapi.com')
  })
})
