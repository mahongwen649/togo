import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader shortcuts', () => {
  it('keeps model market while removing retired shortcuts', () => {
    expect(componentSource).toContain('to="/models"')
    expect(componentSource).toContain("t('nav.modelMarket')")
    expect(componentSource).not.toContain('to="/docs"')
    expect(componentSource).not.toContain("t('nav.docs')")
    expect(componentSource).not.toContain('to="/monitor"')
    expect(componentSource).not.toContain('SubscriptionProgressMini')
  })

  it('distinguishes relay and account-package balances with labels and icons', () => {
    expect(componentSource).toContain('data-wallet="relay"')
    expect(componentSource).toContain('data-wallet="account"')
    expect(componentSource).toContain('中转余额')
    expect(componentSource).toContain('套餐余额')
    expect(componentSource).toContain('name="link"')
    expect(componentSource).toContain('name="grid"')
    expect(componentSource).toContain('accountWalletStore.load(userID)')
    expect(componentSource).toContain('class="hidden xl:inline"')
  })
})
