import { readFileSync, readdirSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

type Messages = Record<string, unknown>

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      return entry.name === '__tests__' ? [] : sourceFiles(path)
    }
    return ['.ts', '.vue'].includes(extname(entry.name)) ? [path] : []
  })
}

function hasKey(messages: Messages, key: string): boolean {
  let current: unknown = messages
  for (const segment of key.split('.')) {
    if (!current || typeof current !== 'object' || !(segment in current)) return false
    current = (current as Messages)[segment]
  }
  return typeof current === 'string'
}

describe('i18n literal key coverage', () => {
  it('defines every literal translation key in Chinese and English', () => {
    const sourceRoot = resolve(process.cwd(), 'src')
    const missing: string[] = []
    const keyPattern = /(?<![\w$])(?:\$t|t)\(\s*['"]([^'"]+)['"]/g

    const files = sourceFiles(sourceRoot)
    for (const file of files) {
      const source = readFileSync(file, 'utf8')
      for (const match of source.matchAll(keyPattern)) {
        const key = match[1]
        if (key.endsWith('.')) continue
        if (!hasKey(zh as Messages, key)) missing.push(`zh:${key} (${relative(sourceRoot, file)})`)
        if (!hasKey(en as Messages, key)) missing.push(`en:${key} (${relative(sourceRoot, file)})`)
      }
    }

    expect([...new Set(missing)]).toEqual([])
  })
})
