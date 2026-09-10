import { describe, expect, it } from 'vitest'
import { getSingleRouteQueryValue } from '../routeQuery'

describe('getSingleRouteQueryValue', () => {
  it('returns a trimmed scalar query value', () => {
    expect(getSingleRouteQueryValue('  order-1  ')).toBe('order-1')
  })

  it('uses the first non-empty value when a provider repeats a query parameter', () => {
    expect(getSingleRouteQueryValue(['', ' order-1 ', 'order-1'])).toBe('order-1')
  })

  it('returns an empty string when no usable value exists', () => {
    expect(getSingleRouteQueryValue([null, '  '])).toBe('')
  })
})
