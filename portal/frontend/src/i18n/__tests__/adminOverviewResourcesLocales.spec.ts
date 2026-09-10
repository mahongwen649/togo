import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

function leafPaths(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return [prefix]
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    leafPaths(child, prefix ? `${prefix}.${key}` : key)
  )
}

function hasPath(value: unknown, path: string): boolean {
  let current = value
  for (const part of path.split('.')) {
    if (!current || typeof current !== 'object' || !(part in current)) return false
    current = (current as Record<string, unknown>)[part]
  }
  return true
}

describe('admin overview and resources English locale parity', () => {
  const sections = ['dashboard', 'users', 'groups', 'redeem'] as const
  const enAdmin = en.admin as unknown as Record<string, unknown>
  const zhAdmin = zh.admin as unknown as Record<string, unknown>

  it.each(sections)('covers admin.%s keys present in Chinese locale', (section) => {
    const missing = leafPaths(zhAdmin[section]).filter((path) => !hasPath(enAdmin[section], path))
    expect(missing).toEqual([])
  })
})
