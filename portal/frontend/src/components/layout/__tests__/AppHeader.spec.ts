import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader shortcuts', () => {
  it('keeps model market and docs while removing retired domain shortcuts', () => {
    expect(componentSource).toContain('to="/models"')
    expect(componentSource).toContain("t('nav.modelMarket')")
    expect(componentSource).toContain('to="/docs"')
    expect(componentSource).toContain("t('nav.docs')")
    expect(componentSource).not.toContain('to="/monitor"')
    expect(componentSource).not.toContain('SubscriptionProgressMini')
  })
})
