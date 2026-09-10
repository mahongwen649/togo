import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function read(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8')
}

describe('TogoAPI design system', () => {
  it('uses the Togo teal palette and restrained global effects', () => {
    const tailwind = read('tailwind.config.js')

    expect(tailwind).toContain("500: '#14b8a6'")
    expect(tailwind).toContain('relay: {')
    expect(tailwind).toContain("'mesh-gradient':")
    expect(tailwind).toContain("linear-gradient(180deg, #f8fafc 0%, #f0fdfa 50%, #f8fafc 100%)")
    expect(tailwind).not.toContain('radial-gradient(at 40% 20%')
  })

  it('keeps core controls and data surfaces at the eight-pixel radius', () => {
    const styles = read('src/style.css')

    expect(styles).toMatch(/\.btn\s*{[\s\S]*?rounded-lg/)
    expect(styles).toMatch(/\.input\s*{[\s\S]*?rounded-lg/)
    expect(styles).toMatch(/\.card\s*{[\s\S]*?rounded-lg/)
    expect(styles).toMatch(/\.table-container\s*{[\s\S]*?rounded-lg/)
  })

  it('removes decorative layout orbs and uses localized TogoAPI auth copy', () => {
    const appLayout = read('src/components/layout/AppLayout.vue')
    const authLayout = read('src/components/layout/AuthLayout.vue')
    const loginView = read('src/views/auth/LoginView.vue')

    expect(appLayout).not.toContain('Background Decoration')
    expect(authLayout).not.toContain('Gradient Orbs')
    expect(authLayout).toContain("t('auth.layout.gatewayEyebrow')")
    expect(authLayout).toContain("appStore.siteName || 'TogoAPI'")
    expect(loginView).toContain("t('auth.loginIdentifierLabel')")
    expect(loginView).toContain("t('auth.loginIdentifierPlaceholder')")
    expect(loginView).not.toContain("t('auth.accountLabel')")
  })

  it('keeps the approved home composition and login and registration on home', () => {
    const unifiedHome = read('src/views/HomeView.vue')
    const router = read('src/router/index.ts')

    for (const marker of ['home-shell', 'gateway-card', 'gateway-node', 'energy-pulse', '<HomeAuthPanel />']) {
      expect(unifiedHome).toContain(marker)
    }

    expect(unifiedHome).not.toContain('terminal-container')
    expect(router).toMatch(/path: '\/login',[\s\S]*?path: '\/home',[\s\S]*?mode: 'login'/)
    expect(router).toMatch(/path: '\/register',[\s\S]*?path: '\/home',[\s\S]*?mode: 'register'/)
    expect(router).not.toContain("component: () => import('@/views/auth/LoginView.vue')")
    expect(router).not.toContain("component: () => import('@/views/auth/RegisterView.vue')")
  })
})
